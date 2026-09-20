package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://graph.facebook.com"

type Options struct {
	BaseURL    string
	UploadURL  string
	Version    string
	Token      string
	AppSecret  string
	DryRun     bool
	ShowToken  bool
	Writer     io.Writer
	HTTPClient *http.Client
	MaxRetries int
	RequestsPS float64
	Sleep      func(time.Duration)
	Jitter     func(time.Duration) time.Duration
	Now        func() time.Time
}

type Client struct {
	baseURL    *url.URL
	uploadURL  *url.URL
	version    string
	token      string
	appSecret  string
	dryRun     bool
	showToken  bool
	writer     io.Writer
	httpClient *http.Client
	maxRetries int
	limiter    *rateLimiter
	sleep      func(time.Duration)
	jitter     func(time.Duration) time.Duration
	now        func() time.Time
}

type Request struct {
	Method     string
	Path       string
	Query      url.Values
	Body       []byte
	Headers    http.Header
	Idempotent bool
	Upload     bool
}

type Response struct {
	Body   []byte
	Header http.Header
	Status int
}

func New(options Options) (*Client, error) {
	if options.BaseURL == "" {
		options.BaseURL = defaultBaseURL
	}
	baseURL, err := validateBaseURL(options.BaseURL)
	if err != nil {
		return nil, err
	}
	if options.UploadURL == "" {
		options.UploadURL = "https://rupload.facebook.com"
	}
	uploadURL, err := validateBaseURL(options.UploadURL)
	if err != nil {
		return nil, fmt.Errorf("upload URL: %w", err)
	}
	if options.Version == "" {
		options.Version = "v26.0"
	}
	if !strings.HasPrefix(options.Version, "v") || strings.Contains(options.Version, "/") {
		return nil, fmt.Errorf("invalid Graph version %q", options.Version)
	}
	if options.Writer == nil {
		options.Writer = io.Discard
	}
	if options.HTTPClient == nil {
		options.HTTPClient = &http.Client{Timeout: 60 * time.Second}
	}
	if options.MaxRetries <= 0 {
		options.MaxRetries = 3
	}
	if options.Sleep == nil {
		options.Sleep = time.Sleep
	}
	if options.Jitter == nil {
		options.Jitter = fullJitter
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Client{
		baseURL: baseURL, uploadURL: uploadURL, version: options.Version, token: options.Token,
		appSecret: options.AppSecret, dryRun: options.DryRun, showToken: options.ShowToken,
		writer: options.Writer, httpClient: options.HTTPClient, maxRetries: options.MaxRetries,
		limiter: newRateLimiter(options.RequestsPS, options.Sleep), sleep: options.Sleep,
		jitter: options.Jitter, now: options.Now,
	}, nil
}

func validateBaseURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("base URL must be an absolute HTTP or HTTPS URL")
	}
	host := parsed.Hostname()
	if parsed.Scheme == "http" && host != "localhost" && net.ParseIP(host) == nil {
		return nil, fmt.Errorf("plain HTTP is allowed only for loopback test servers")
	}
	if parsed.Scheme == "http" {
		ip := net.ParseIP(host)
		if ip != nil && !ip.IsLoopback() {
			return nil, fmt.Errorf("plain HTTP is allowed only for loopback test servers")
		}
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	return parsed, nil
}

func (client *Client) Do(ctx context.Context, spec Request) (*Response, error) {
	if spec.Method == "" {
		spec.Method = http.MethodGet
	}
	requestURL := client.resolveURL(spec.Path, spec.Upload)
	query := cloneValues(spec.Query)
	if client.appSecret != "" && client.token != "" {
		query.Set("appsecret_proof", appSecretProof(client.token, client.appSecret))
	}
	requestURL.RawQuery = query.Encode()
	if client.dryRun {
		fmt.Fprintln(client.writer, client.curl(spec, requestURL))
		return &Response{Body: []byte(`{"dry_run":true}`), Header: make(http.Header), Status: 0}, nil
	}
	canRetry := spec.Idempotent || methodIsIdempotent(spec.Method)
	for attempt := 0; ; attempt++ {
		client.limiter.Wait()
		req, err := http.NewRequestWithContext(ctx, spec.Method, requestURL.String(), bytes.NewReader(spec.Body))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		for name, values := range spec.Headers {
			for _, value := range values {
				req.Header.Add(name, value)
			}
		}
		if len(spec.Body) > 0 && req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if client.token != "" {
			scheme := "Bearer"
			if spec.Upload {
				scheme = "OAuth"
			}
			req.Header.Set("Authorization", scheme+" "+client.token)
		}
		resp, requestErr := client.httpClient.Do(req)
		if requestErr != nil {
			if canRetry && attempt < client.maxRetries && transientNetworkError(requestErr) {
				client.sleep(client.jitter(time.Second << attempt))
				continue
			}
			return nil, fmt.Errorf("send request: %w", requestErr)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		closeErr := resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read response: %w", readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close response: %w", closeErr)
		}
		client.limiter.Observe(resp.Header, resp.StatusCode)
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return &Response{Body: body, Header: resp.Header.Clone(), Status: resp.StatusCode}, nil
		}
		if canRetry && attempt < client.maxRetries && retryableStatus(resp.StatusCode) {
			delay, ok := retryAfter(resp.Header.Get("Retry-After"), client.now())
			if !ok {
				delay = client.jitter(time.Second << attempt)
			}
			client.sleep(delay)
			continue
		}
		return nil, decodeAPIError(resp.StatusCode, body)
	}
}

func (client *Client) JSON(ctx context.Context, spec Request, target any) error {
	response, err := client.Do(ctx, spec)
	if err != nil {
		return err
	}
	if target == nil || len(response.Body) == 0 {
		return nil
	}
	if err := json.Unmarshal(response.Body, target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (client *Client) List(ctx context.Context, requestPath string, query url.Values, all bool, limit int) ([]any, error) {
	query = cloneValues(query)
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	var result []any
	for {
		response, err := client.Do(ctx, Request{Method: http.MethodGet, Path: requestPath, Query: query})
		if err != nil {
			return nil, err
		}
		items, after, err := decodePage(response.Body)
		if err != nil {
			return nil, fmt.Errorf("decode page: %w", err)
		}
		result = append(result, items...)
		if !all || after == "" {
			return result, nil
		}
		query.Set("after", after)
	}
}

func (client *Client) resolveURL(requestPath string, upload bool) *url.URL {
	base := client.baseURL
	if upload {
		base = client.uploadURL
	}
	copyURL := *base
	clean := strings.TrimPrefix(requestPath, "/")
	copyURL.Path = path.Join(copyURL.Path, client.version, clean)
	if upload && strings.HasPrefix(clean, "ig-api-upload/") {
		copyURL.Path = path.Join(base.Path, "ig-api-upload", client.version, strings.TrimPrefix(clean, "ig-api-upload/"))
	}
	if upload && strings.HasPrefix(clean, "video-upload/") {
		copyURL.Path = path.Join(base.Path, "video-upload", client.version, strings.TrimPrefix(clean, "video-upload/"))
	}
	return &copyURL
}

func (client *Client) curl(spec Request, requestURL *url.URL) string {
	token := "<redacted>"
	if client.showToken {
		token = client.token
	}
	parts := []string{"curl", "-X", shellQuote(spec.Method), shellQuote(requestURL.String())}
	if client.token != "" {
		scheme := "Bearer"
		if spec.Upload {
			scheme = "OAuth"
		}
		parts = append(parts, "-H", shellQuote("Authorization: "+scheme+" "+token))
	}
	for name, values := range spec.Headers {
		for _, value := range values {
			parts = append(parts, "-H", shellQuote(name+": "+value))
		}
	}
	if len(spec.Body) > 0 {
		parts = append(parts, "--data-binary", shellQuote(string(spec.Body)))
	}
	return strings.Join(parts, " ")
}

func appSecretProof(token, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'" }

func cloneValues(values url.Values) url.Values {
	clone := make(url.Values, len(values))
	for key, entries := range values {
		clone[key] = append([]string(nil), entries...)
	}
	return clone
}

func transientNetworkError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	var operationError *net.OpError
	return errors.As(err, &operationError)
}
