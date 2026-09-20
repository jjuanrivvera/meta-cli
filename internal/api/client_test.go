package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testClient(t *testing.T, handler http.HandlerFunc, options func(*Options)) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	config := Options{BaseURL: server.URL, UploadURL: server.URL, Token: "secret-token", Sleep: func(time.Duration) {}, Jitter: func(time.Duration) time.Duration { return 0 }}
	if options != nil {
		options(&config)
	}
	client, err := New(config)
	require.NoError(t, err)
	return client
}

func TestClientJSONAndProof(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "/v26.0/me", request.URL.Path)
		assert.Equal(t, "Bearer secret-token", request.Header.Get("Authorization"))
		assert.Len(t, request.URL.Query().Get("appsecret_proof"), 64)
		_, _ = io.WriteString(w, `{"id":"1"}`)
	}, func(options *Options) { options.AppSecret = "app-secret" })
	var result map[string]any
	require.NoError(t, client.JSON(context.TODO(), Request{Method: http.MethodGet, Path: "me"}, &result))
	assert.Equal(t, "1", result["id"])
}

func TestClientDryRunRedactsAndCanReveal(t *testing.T) {
	for _, show := range []bool{false, true} {
		t.Run(map[bool]string{false: "redacted", true: "shown"}[show], func(t *testing.T) {
			var output bytes.Buffer
			client, err := New(Options{BaseURL: "http://127.0.0.1:1234", UploadURL: "http://127.0.0.1:1234", Token: "secret-token", DryRun: true, ShowToken: show, Writer: &output})
			require.NoError(t, err)
			_, err = client.Do(context.TODO(), Request{Method: http.MethodPost, Path: "object", Body: []byte(`{"message":"it's ready"}`)})
			require.NoError(t, err)
			if show {
				assert.Contains(t, output.String(), "secret-token")
			} else {
				assert.NotContains(t, output.String(), "secret-token")
				assert.Contains(t, output.String(), "<redacted>")
			}
			assert.Contains(t, output.String(), "curl -X 'POST'")
		})
	}
}

func TestDryRunNeverPrintsSecretsOrFileBytes(t *testing.T) {
	var output bytes.Buffer
	client, err := New(Options{
		BaseURL: "http://127.0.0.1:1234", UploadURL: "http://127.0.0.1:1234",
		Token: "page-token-value", AppSecret: "app-secret-value", DryRun: true,
		ShowToken: true, AlwaysRedactToken: true, Redactions: []string{"unused-page-token-value"}, Writer: &output,
	})
	require.NoError(t, err)
	query := url.Values{
		"client_secret": {"client-secret-value"},
		"access_token":  {"app-id|app-secret-value"},
		"input_token":   {"user-token-value"},
	}
	_, err = client.Do(context.TODO(), Request{
		Method: http.MethodPost, Path: "upload", Query: query,
		Body: []byte("binary-secret-file-bytes"), BodyDescription: "@/safe/video.mp4",
		Headers: http.Header{"X-Description": {"contains page-token-value and app-secret-value"}},
	})
	require.NoError(t, err)
	for _, secret := range []string{"page-token-value", "app-secret-value", "client-secret-value", "user-token-value", "unused-page-token-value", "binary-secret-file-bytes"} {
		assert.NotContains(t, output.String(), secret)
	}
	assert.Contains(t, output.String(), "@/safe/video.mp4")

	output.Reset()
	_, err = client.Do(context.TODO(), Request{
		Method: http.MethodPost, Path: "post", Query: url.Values{"title": {"contains app-secret-value"}},
		Body: []byte(`{"app-secret-value-field":"ordinary","message":"contains page-token-value, unused-page-token-value, and app-secret-value"}`),
	})
	require.NoError(t, err)
	assert.NotContains(t, output.String(), "page-token-value")
	assert.NotContains(t, output.String(), "app-secret-value")
	assert.NotContains(t, output.String(), "unused-page-token-value")
	assert.Contains(t, output.String(), "<redacted>")

	output.Reset()
	_, err = client.Do(context.TODO(), Request{
		Method: http.MethodPost, Path: "scalar", Body: []byte(`"app-secret-value unused-page-token-value"`),
	})
	require.NoError(t, err)
	assert.NotContains(t, output.String(), "app-secret-value")
	assert.NotContains(t, output.String(), "unused-page-token-value")

	output.Reset()
	_, err = client.Do(context.TODO(), Request{
		Method: http.MethodPost, Path: "object/app-secret-value", BodyDescription: "@/safe/unused-page-token-value.bin",
		MultipartForm: []FormPart{
			{Name: "description", Value: "unused-page-token-value"},
			{Name: "file", File: "/safe/app-secret-value.bin", ContentType: "type/unused-page-token-value"},
		},
	})
	require.NoError(t, err)
	assert.NotContains(t, output.String(), "app-secret-value")
	assert.NotContains(t, output.String(), "unused-page-token-value")

	output.Reset()
	_, err = client.Do(context.TODO(), Request{
		Method: http.MethodPost, Path: "upload", BodyDescription: "@/safe/app-secret-value.bin",
	})
	require.NoError(t, err)
	assert.NotContains(t, output.String(), "app-secret-value")
}

func TestMultipartDryRunUsesEquivalentCurlForms(t *testing.T) {
	var output bytes.Buffer
	client, err := New(Options{
		BaseURL: "http://127.0.0.1:1234", UploadURL: "http://127.0.0.1:1234",
		Token: "page-token", DryRun: true, Writer: &output,
	})
	require.NoError(t, err)
	_, err = client.Do(context.Background(), Request{
		Method: http.MethodPost, Path: "page/videos",
		Headers: http.Header{"Content-Type": {"multipart/form-data; boundary=runtime-boundary"}},
		MultipartForm: []FormPart{
			{Name: "is_preferred", Value: "true"},
			{Name: "video_file_chunk", File: "/safe/video.mp4", ContentType: "application/octet-stream", Offset: 4, Length: 8},
		},
	})
	require.NoError(t, err)
	command := output.String()
	assert.Contains(t, command, "dd if='/safe/video.mp4' bs=1 skip=4 count=8")
	assert.Contains(t, command, "--form-string 'is_preferred=true'")
	assert.Contains(t, command, "--form 'video_file_chunk=@-;filename=video.mp4;type=application/octet-stream'")
	assert.NotContains(t, command, "runtime-boundary")
	assert.NotContains(t, command, "--data-binary")
}

func TestRequestCanSuppressAuthorizationAndProof(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, request *http.Request) {
		assert.Empty(t, request.Header.Get("Authorization"))
		assert.Empty(t, request.URL.Query().Get("appsecret_proof"))
		_, _ = io.WriteString(w, `{}`)
	}, func(options *Options) { options.AppSecret = "app-secret" })
	_, err := client.Do(context.TODO(), Request{Method: http.MethodGet, Path: "debug_token", SkipAuthorization: true, SkipAppSecretProof: true})
	require.NoError(t, err)
}

func TestRetryRulesAndRetryAfter(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			calls := 0
			var delays []time.Duration
			client := testClient(t, func(w http.ResponseWriter, request *http.Request) {
				calls++
				if calls == 1 {
					w.Header().Set("Retry-After", "2")
					w.WriteHeader(http.StatusTooManyRequests)
					_, _ = io.WriteString(w, `{"error":{"message":"slow down","code":4}}`)
					return
				}
				_, _ = io.WriteString(w, `{"ok":true}`)
			}, func(options *Options) { options.Sleep = func(delay time.Duration) { delays = append(delays, delay) } })
			_, err := client.Do(context.TODO(), Request{Method: method, Path: "retry"})
			if method == http.MethodGet {
				require.NoError(t, err)
				assert.Equal(t, 2, calls)
				assert.Contains(t, delays, 2*time.Second)
			} else {
				assert.Error(t, err)
				assert.Equal(t, 1, calls)
			}
		})
	}
}

func TestBusinessUsageHeaderRetriesAndUsesEstimatedDelay(t *testing.T) {
	calls := 0
	var delays []time.Duration
	client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("X-Business-Use-Case-Usage", `{"business":[{"call_count":100,"estimated_time_to_regain_access":7}]}`)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":{"message":"usage exhausted","code":100,"fbtrace_id":"trace-1"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true}`)
	}, func(options *Options) { options.Sleep = func(delay time.Duration) { delays = append(delays, delay) } })
	_, err := client.Do(context.TODO(), Request{Method: http.MethodGet, Path: "throttled"})
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
	assert.Contains(t, delays, 7*time.Minute)
}

func TestGraphThrottleCodeRetriesWithoutUsageHeaders(t *testing.T) {
	calls := 0
	client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":{"message":"throttled","code":613,"fbtrace_id":"trace-code"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true}`)
	}, nil)
	_, err := client.Do(context.Background(), Request{Method: http.MethodGet, Path: "throttled"})
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
	assert.True(t, isThrottleCode(80014))
	assert.False(t, isThrottleCode(80010))
}

func TestUsageObservationSlowsTheNextRequest(t *testing.T) {
	var delays []time.Duration
	limiter := newRateLimiter(10, func(delay time.Duration) { delays = append(delays, delay) })
	limiter.Wait()
	limiter.Observe(http.Header{"X-App-Usage": {`{"call_count":90}`}}, http.StatusOK)
	limiter.Wait()
	require.NotEmpty(t, delays)
	assert.Greater(t, delays[len(delays)-1], 300*time.Millisecond)
}

func TestThrottleWaitStopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error":{"message":"slow down","code":4}}`)
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()
	}))
	t.Cleanup(server.Close)
	client, err := New(Options{
		BaseURL: server.URL, UploadURL: server.URL, Token: "token", HTTPClient: server.Client(),
		MaxRetries: 1, RequestsPS: 100000,
	})
	require.NoError(t, err)
	started := time.Now()
	_, err = client.Do(ctx, Request{Method: http.MethodGet, Path: "throttled"})
	require.ErrorIs(t, err, context.Canceled)
	assert.Less(t, time.Since(started), time.Second)
	assert.Equal(t, 1, calls)
}

func TestStreamingBodyReopensForRetry(t *testing.T) {
	calls, opens := 0, 0
	client := testClient(t, func(w http.ResponseWriter, request *http.Request) {
		calls++
		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		assert.Equal(t, "streamed", string(body))
		if calls == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"error":{"message":"retry","code":2}}`)
			return
		}
		_, _ = io.WriteString(w, `{}`)
	}, nil)
	_, err := client.Do(context.TODO(), Request{
		Method: http.MethodPost, Path: "upload", Idempotent: true, LongRunning: true,
		BodyFactory: func() (io.ReadCloser, error) {
			opens++
			return io.NopCloser(bytes.NewBufferString("streamed")), nil
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 2, opens)
}

func TestCursorPaginationRebuildsQuery(t *testing.T) {
	calls := 0
	client := testClient(t, func(w http.ResponseWriter, request *http.Request) {
		calls++
		if calls == 1 {
			assert.Empty(t, request.URL.Query().Get("after"))
			_, _ = io.WriteString(w, `{"data":[{"id":"1"}],"paging":{"cursors":{"after":"next"},"next":"https://evil.invalid/leak"}}`)
			return
		}
		assert.Equal(t, "next", request.URL.Query().Get("after"))
		_, _ = io.WriteString(w, `{"data":[{"id":"2"}]}`)
	}, nil)
	items, err := client.List(context.TODO(), "items", url.Values{"fields": {"id"}}, true, 10)
	require.NoError(t, err)
	assert.Len(t, items, 2)
	assert.Equal(t, 2, calls)
}

func TestClientErrorsAndValidation(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, request *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"expired","code":190,"fbtrace_id":"trace-auth"}}`)
	}, nil)
	_, err := client.Do(context.TODO(), Request{Method: http.MethodGet, Path: "me"})
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Contains(t, err.Error(), "auth login")
	assert.Equal(t, "trace-auth", apiErr.TraceID)
	assert.Contains(t, err.Error(), "trace-auth")

	for _, raw := range []string{"", "ftp://example.com", "http://example.com", "relative"} {
		_, err := New(Options{BaseURL: raw, UploadURL: "http://127.0.0.1:1"})
		if raw == "" {
			assert.NoError(t, err)
		} else {
			assert.Error(t, err)
		}
	}
	_, err = New(Options{BaseURL: "http://127.0.0.1:1", UploadURL: "http://127.0.0.1:1", Version: "26/0"})
	assert.Error(t, err)
}

func TestGraphErrorBodiesRedactCredentialValues(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, request *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"message":"echo secret-token app-secret-value client-secret-value","code":100,"fbtrace_id":"trace"}}`)
	}, func(options *Options) { options.AppSecret = "app-secret-value" })
	_, err := client.Do(context.Background(), Request{
		Method: http.MethodPost, Path: "oauth/access_token", Query: url.Values{"client_secret": {"client-secret-value"}},
	})
	require.Error(t, err)
	for _, secret := range []string{"secret-token", "app-secret-value", "client-secret-value"} {
		assert.NotContains(t, err.Error(), secret)
	}
	assert.Contains(t, err.Error(), "<redacted>")
}

func TestScalarGraphErrorBodyRedactsCredentialValues(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, request *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `"secret-token app-secret-value"`)
	}, func(options *Options) { options.AppSecret = "app-secret-value" })
	_, err := client.Do(context.Background(), Request{Method: http.MethodGet, Path: "me"})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret-token")
	assert.NotContains(t, err.Error(), "app-secret-value")
}

type temporaryError struct{}

func (temporaryError) Error() string   { return "temporary" }
func (temporaryError) Timeout() bool   { return true }
func (temporaryError) Temporary() bool { return true }

type failingTransport struct{ calls int }

func (transport *failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	transport.calls++
	if transport.calls == 1 {
		return nil, temporaryError{}
	}
	return nil, errors.New("permanent")
}

type roundTripFunction func(*http.Request) (*http.Response, error)

func (function roundTripFunction) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestTransientNetworkRetry(t *testing.T) {
	transport := &failingTransport{}
	client, err := New(Options{BaseURL: "http://127.0.0.1:1", UploadURL: "http://127.0.0.1:1", HTTPClient: &http.Client{Transport: transport}, Sleep: func(time.Duration) {}, Jitter: func(time.Duration) time.Duration { return 0 }, MaxRetries: 1})
	require.NoError(t, err)
	_, err = client.Do(context.TODO(), Request{Method: http.MethodGet, Path: "me"})
	assert.Error(t, err)
	assert.Equal(t, 2, transport.calls)
	var netErr net.Error = temporaryError{}
	assert.True(t, transientNetworkError(netErr))
}

func TestTransportErrorsRedactSecretQueryValues(t *testing.T) {
	transport := roundTripFunction(func(request *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("dial failed for %s", request.URL.String())
	})
	client, err := New(Options{
		BaseURL: "http://127.0.0.1:1", UploadURL: "http://127.0.0.1:1",
		Token: "user-token-secret", AppSecret: "app-secret-value",
		HTTPClient: &http.Client{Transport: transport}, MaxRetries: 1,
	})
	require.NoError(t, err)
	_, err = client.Do(context.Background(), Request{Method: http.MethodPost, Path: "oauth/access_token", Query: url.Values{
		"client_secret":     {"client-secret-value"},
		"fb_exchange_token": {"exchange-token-value"},
	}})
	require.Error(t, err)
	for _, secret := range []string{"user-token-secret", "app-secret-value", "client-secret-value", "exchange-token-value"} {
		assert.NotContains(t, err.Error(), secret)
	}
	assert.Contains(t, err.Error(), "redacted")
}

func TestHelpers(t *testing.T) {
	assert.Equal(t, 97, maxUsage(`{"call_count":12,"nested":[{"total_time":97}]}`))
	assert.Zero(t, maxUsage("bad"))
	assert.True(t, methodIsIdempotent(http.MethodDelete))
	assert.False(t, methodIsIdempotent(http.MethodPatch))
	assert.True(t, retryableStatus(503))
	assert.False(t, retryableStatus(400))
	assert.Equal(t, "'a'\\''b'", shellQuote("a'b"))
	delay, ok := retryAfter("3", time.Now())
	assert.True(t, ok)
	assert.Equal(t, 3*time.Second, delay)
	_, ok = retryAfter("invalid", time.Now())
	assert.False(t, ok)
	assert.Zero(t, fullJitter(0))
	assert.LessOrEqual(t, fullJitter(time.Second), time.Second)
}

func TestAPIErrorHints(t *testing.T) {
	tests := []struct {
		error *APIError
		hint  string
	}{
		{&APIError{StatusCode: http.StatusForbidden}, "permissions"},
		{&APIError{StatusCode: http.StatusNotFound}, "object id"},
		{&APIError{StatusCode: http.StatusTooManyRequests}, "rate limited"},
		{&APIError{StatusCode: http.StatusBadGateway}, "server error"},
		{&APIError{StatusCode: http.StatusBadRequest, Body: "bad body"}, "--verbose"},
	}
	for _, test := range tests {
		assert.Contains(t, test.error.Hint(), test.hint)
		assert.Contains(t, test.error.Error(), test.hint)
	}
	assert.Equal(t, "abcd…", truncate("abcdefgh", 4))
}
