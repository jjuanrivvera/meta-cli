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
	BaseURL           string
	UploadURL         string
	Version           string
	Token             string
	AppSecret         string
	DryRun            bool
	ShowToken         bool
	AlwaysRedactToken bool
	Redactions        []string
	Verbose           bool
	Writer            io.Writer
	Diagnostics       io.Writer
	HTTPClient        *http.Client
	MaxRetries        int
	RequestsPS        float64
	Sleep             func(time.Duration)
	Jitter            func(time.Duration) time.Duration
	Now               func() time.Time
}

type Client struct {
	baseURL           *url.URL
	uploadURL         *url.URL
	version           string
	token             string
	appSecret         string
	dryRun            bool
	showToken         bool
	alwaysRedactToken bool
	redactions        []string
	verbose           bool
	writer            io.Writer
	diagnostics       io.Writer
	httpClient        *http.Client
	maxRetries        int
	limiter           *rateLimiter
	wait              func(context.Context, time.Duration) error
	jitter            func(time.Duration) time.Duration
	now               func() time.Time
}

type Request struct {
	Method             string
	Path               string
	Query              url.Values
	Body               []byte
	BodyFactory        func() (io.ReadCloser, error)
	BodyDescription    string
	MultipartForm      []FormPart
	ContentLength      int64
	Headers            http.Header
	Idempotent         bool
	Upload             bool
	LongRunning        bool
	SkipAuthorization  bool
	SkipAppSecretProof bool
}

// FormPart describes a multipart field for an equivalent, copy-pasteable
// dry-run command. BodyFactory remains responsible for the streamed request.
type FormPart struct {
	Name        string
	Value       string
	File        string
	ContentType string
	Offset      int64
	Length      int64
}

type Response struct {
	Body   []byte
	Header http.Header
	Status int
}

func New(options Options) (*Client, error) {
	configuredSleep := options.Sleep
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
	if options.Diagnostics == nil {
		options.Diagnostics = io.Discard
	}
	if options.HTTPClient == nil {
		options.HTTPClient = &http.Client{}
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
	wait := waitForContext
	if configuredSleep != nil {
		wait = func(ctx context.Context, delay time.Duration) error {
			configuredSleep(delay)
			return ctx.Err()
		}
	}
	return &Client{
		baseURL: baseURL, uploadURL: uploadURL, version: options.Version, token: options.Token,
		appSecret: options.AppSecret, dryRun: options.DryRun, showToken: options.ShowToken,
		alwaysRedactToken: options.AlwaysRedactToken, verbose: options.Verbose,
		redactions: append([]string(nil), options.Redactions...),
		writer:     options.Writer, diagnostics: options.Diagnostics,
		httpClient: options.HTTPClient, maxRetries: options.MaxRetries,
		limiter: newRateLimiter(options.RequestsPS, options.Sleep), wait: wait,
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
	if !spec.SkipAppSecretProof && client.appSecret != "" && client.token != "" {
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
		requestContext := ctx
		cancel := func() {}
		if !spec.LongRunning {
			requestContext, cancel = context.WithTimeout(ctx, 60*time.Second)
		}
		requestBodyReader, err := requestBody(spec)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("open request body: %w", err)
		}
		req, err := http.NewRequestWithContext(requestContext, spec.Method, requestURL.String(), requestBodyReader)
		if err != nil {
			if closer, ok := requestBodyReader.(io.Closer); ok {
				_ = closer.Close()
			}
			cancel()
			return nil, fmt.Errorf("build request: %w", err)
		}
		if spec.ContentLength > 0 {
			req.ContentLength = spec.ContentLength
		}
		for name, values := range spec.Headers {
			for _, value := range values {
				req.Header.Add(name, value)
			}
		}
		if len(spec.Body) > 0 && req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if client.token != "" && !spec.SkipAuthorization {
			scheme := "Bearer"
			if spec.Upload {
				scheme = "OAuth"
			}
			req.Header.Set("Authorization", scheme+" "+client.token)
		}
		resp, requestErr := client.httpClient.Do(req)
		if requestErr != nil {
			cancel()
			if canRetry && attempt < client.maxRetries && transientNetworkError(requestErr) {
				if waitErr := client.wait(ctx, client.jitter(time.Second<<attempt)); waitErr != nil {
					return nil, waitErr
				}
				continue
			}
			message := sanitizedTransportError(requestErr, requestURL)
			message = redactKnownSecrets(message, client.credentialValues(query))
			return nil, fmt.Errorf("send request: %s", message)
		}
		responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		closeErr := resp.Body.Close()
		cancel()
		if readErr != nil {
			return nil, fmt.Errorf("read response: %w", readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close response: %w", closeErr)
		}
		client.limiter.Observe(resp.Header, resp.StatusCode)
		if client.verbose {
			diagnosticPath := redactKnownSecrets(requestURL.Path, client.credentialValues(query))
			fmt.Fprintf(client.diagnostics, "%s %s -> %d\n", spec.Method, diagnosticPath, resp.StatusCode)
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return &Response{Body: responseBody, Header: resp.Header.Clone(), Status: resp.StatusCode}, nil
		}
		responseBody = redactCredentialValues(responseBody, client.credentialValues(query))
		apiErr := decodeAPIError(resp.StatusCode, responseBody)
		throttled := isThrottled(resp.Header, apiErr)
		if canRetry && attempt < client.maxRetries && (retryableStatus(resp.StatusCode) || throttled) {
			delay, ok := throttleDelay(resp.Header, client.now())
			if !ok {
				delay = client.jitter(time.Second << attempt)
			}
			if waitErr := client.wait(ctx, delay); waitErr != nil {
				return nil, waitErr
			}
			continue
		}
		return nil, apiErr
	}
}

func waitForContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func requestBody(spec Request) (io.Reader, error) {
	if spec.BodyFactory != nil {
		return spec.BodyFactory()
	}
	return bytes.NewReader(spec.Body), nil
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
	showToken := client.showToken && !client.alwaysRedactToken
	if showToken {
		token = client.token
	}
	displayURL := *requestURL
	query := displayURL.Query()
	secrets := client.credentialValues(query)
	displayURL.Path = redactKnownSecrets(displayURL.Path, secrets)
	displayURL.RawPath = ""
	for key, values := range query {
		if sensitiveName(key) {
			if showToken && key == "input_token" {
				continue
			}
			query.Set(key, "<redacted>")
			continue
		}
		for index, value := range values {
			values[index] = redactKnownSecrets(value, secrets)
		}
		query[key] = values
	}
	displayURL.RawQuery = query.Encode()
	parts := []string{"curl", "-X", shellQuote(spec.Method), shellQuote(redactKnownSecrets(displayURL.String(), secrets))}
	prefix := ""
	if client.token != "" && !spec.SkipAuthorization {
		scheme := "Bearer"
		if spec.Upload {
			scheme = "OAuth"
		}
		parts = append(parts, "-H", shellQuote("Authorization: "+scheme+" "+token))
	}
	for name, values := range spec.Headers {
		if len(spec.MultipartForm) > 0 && strings.EqualFold(name, "Content-Type") {
			continue
		}
		for _, value := range values {
			if sensitiveName(name) {
				value = "<redacted>"
			} else {
				value = redactKnownSecrets(value, secrets)
			}
			parts = append(parts, "-H", shellQuote(name+": "+value))
		}
	}
	for _, form := range spec.MultipartForm {
		if form.File == "" {
			value := form.Value
			if sensitiveName(form.Name) {
				value = "<redacted>"
			} else {
				value = redactKnownSecrets(value, secrets)
			}
			parts = append(parts, "--form-string", shellQuote(form.Name+"="+value))
			continue
		}
		filePath := redactKnownSecrets(form.File, secrets)
		contentType := redactKnownSecrets(form.ContentType, secrets)
		fileSource := "@" + filePath
		if form.Length > 0 {
			prefix = "dd if=" + shellQuote(filePath) + " bs=1 skip=" + strconv.FormatInt(form.Offset, 10) +
				" count=" + strconv.FormatInt(form.Length, 10) + " 2>/dev/null | "
			fileSource = "@-;filename=" + path.Base(filePath)
		}
		if contentType != "" {
			fileSource += ";type=" + contentType
		}
		parts = append(parts, "--form", shellQuote(form.Name+"="+fileSource))
	}
	if len(spec.MultipartForm) > 0 {
		return prefix + strings.Join(parts, " ")
	}
	if spec.BodyDescription != "" {
		parts = append(parts, "--data-binary", shellQuote(redactKnownSecrets(spec.BodyDescription, secrets)))
	} else if len(spec.Body) > 0 {
		parts = append(parts, "--data-binary", shellQuote(redactBody(spec.Body, secrets)))
	}
	return strings.Join(parts, " ")
}

func (client *Client) credentialValues(query url.Values) []string {
	values := []string{client.token, client.appSecret}
	values = append(values, client.redactions...)
	for name, entries := range query {
		if sensitiveName(name) {
			values = append(values, entries...)
		}
	}
	return values
}

func redactCredentialValues(body []byte, secrets []string) []byte {
	var value any
	if json.Unmarshal(body, &value) == nil {
		if text, ok := value.(string); ok {
			for _, secret := range secrets {
				if secret != "" {
					text = strings.ReplaceAll(text, secret, "<redacted>")
				}
			}
			value = text
		} else {
			redactStrings(value, secrets)
		}
		if redacted, err := json.Marshal(value); err == nil {
			return redacted
		}
	}
	text := string(body)
	for _, secret := range secrets {
		if secret != "" {
			text = strings.ReplaceAll(text, secret, "<redacted>")
		}
	}
	return []byte(text)
}

func redactStrings(value any, secrets []string) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			redactedKey := redactKnownSecrets(key, secrets)
			if text, ok := child.(string); ok {
				for _, secret := range secrets {
					if secret != "" {
						text = strings.ReplaceAll(text, secret, "<redacted>")
					}
				}
				if redactedKey != key {
					delete(typed, key)
				}
				typed[redactedKey] = text
				continue
			}
			redactStrings(child, secrets)
			if redactedKey != key {
				delete(typed, key)
				typed[redactedKey] = child
			}
		}
	case []any:
		for index, child := range typed {
			if text, ok := child.(string); ok {
				for _, secret := range secrets {
					if secret != "" {
						text = strings.ReplaceAll(text, secret, "<redacted>")
					}
				}
				typed[index] = text
				continue
			}
			redactStrings(child, secrets)
		}
	}
}

func sensitiveName(name string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(name, "-", "_"))
	return normalized == "token" || strings.HasSuffix(normalized, "_token") || strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "password") || strings.Contains(normalized, "authorization") ||
		strings.Contains(normalized, "proof")
}

func sanitizedTransportError(err error, requestURL *url.URL) string {
	displayURL := *requestURL
	query := displayURL.Query()
	for key := range query {
		if sensitiveName(key) {
			query.Set(key, "<redacted>")
		}
	}
	displayURL.RawQuery = query.Encode()
	message := strings.ReplaceAll(err.Error(), requestURL.String(), displayURL.String())
	for key, values := range requestURL.Query() {
		if !sensitiveName(key) {
			continue
		}
		for _, value := range values {
			if value != "" {
				message = strings.ReplaceAll(message, value, "<redacted>")
				message = strings.ReplaceAll(message, url.QueryEscape(value), url.QueryEscape("<redacted>"))
			}
		}
	}
	return message
}

func redactBody(body []byte, secrets []string) string {
	var value any
	if json.Unmarshal(body, &value) != nil {
		return redactKnownSecrets(string(body), secrets)
	}
	value = redactValue(value, secrets)
	redacted, err := json.Marshal(value)
	if err != nil {
		return "<redacted body>"
	}
	return string(redacted)
}

func redactValue(value any, secrets []string) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			redactedKey := redactKnownSecrets(key, secrets)
			if sensitiveName(key) {
				child = "<redacted>"
			} else {
				child = redactValue(child, secrets)
			}
			if redactedKey != key {
				delete(typed, key)
			}
			typed[redactedKey] = child
		}
	case []any:
		for index, child := range typed {
			typed[index] = redactValue(child, secrets)
		}
	case string:
		return redactKnownSecrets(typed, secrets)
	}
	return value
}

func redactKnownSecrets(value string, secrets []string) string {
	for _, secret := range secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "<redacted>")
		}
	}
	return value
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
