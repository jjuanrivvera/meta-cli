package commands

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jjuanrivvera/meta-cli/internal/auth"
	"github.com/jjuanrivvera/meta-cli/internal/config"
)

type commandTest struct {
	output     bytes.Buffer
	store      *memoryStore
	configPath string
	client     *http.Client
}

func newCommandTest(t *testing.T, handler http.HandlerFunc) (*commandTest, string) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &commandTest{
		store:      &memoryStore{values: map[string]auth.Credential{"default": {Token: "test-token", AppSecret: "test-secret"}}},
		configPath: filepath.Join(t.TempDir(), "config.yaml"),
		client:     server.Client(),
	}, server.URL
}

func (test *commandTest) run(args ...string) error {
	test.output.Reset()
	root := NewRootCmd(Dependencies{In: bytes.NewBuffer(nil), Out: &test.output, Err: &test.output, HTTPClient: test.client, ConfigPath: test.configPath, Store: test.store})
	root.SetArgs(args)
	return root.Execute()
}

func TestAuthCommands(t *testing.T) {
	test, serverURL := newCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v26.0/me":
			_, _ = io.WriteString(writer, `{"id":"user-1","name":"Example User"}`)
		case "/v26.0/debug_token":
			assert.Equal(t, "test-token", request.URL.Query().Get("input_token"))
			_, _ = io.WriteString(writer, `{"data":{"is_valid":true}}`)
		case "/v26.0/oauth/access_token":
			_, _ = io.WriteString(writer, `{"access_token":"long-token","token_type":"bearer","expires_in":5000}`)
		case "/v26.0/me/accounts":
			_, _ = io.WriteString(writer, `{"data":[{"id":"page-1","name":"Example Page","access_token":"page-token"}]}`)
		default:
			http.NotFound(writer, request)
		}
	})
	require.NoError(t, test.run("--base-url", serverURL, "--upload-url", serverURL, "--app-id", "app-1", "auth", "login", "--token", "test-token", "--app-secret", "test-secret"))
	assert.Contains(t, test.output.String(), "credential stored")
	value, err := config.Load(test.configPath)
	require.NoError(t, err)
	assert.Equal(t, "default", value.Current)

	for _, args := range [][]string{
		{"--base-url", serverURL, "auth", "status", "-o", "json"},
		{"--base-url", serverURL, "--app-id", "app-1", "auth", "debug", "-o", "json"},
		{"--base-url", serverURL, "--app-id", "app-1", "auth", "exchange", "--save", "-o", "json"},
		{"--base-url", serverURL, "auth", "pages", "-o", "json"},
	} {
		require.NoError(t, test.run(args...))
		assert.NotEmpty(t, test.output.String())
	}
	assert.Equal(t, "long-token", test.store.values["default"].Token)
	require.NoError(t, test.run("auth", "logout"))
	_, ok := test.store.values["default"]
	assert.False(t, ok)
}

func TestConfigAndAliasCommands(t *testing.T) {
	test, _ := newCommandTest(t, func(writer http.ResponseWriter, request *http.Request) { http.NotFound(writer, request) })
	require.NoError(t, test.run("config", "set", "work", "--base-url", "https://graph.facebook.com", "--graph-version", "v26.0", "--page-id", "123"))
	require.NoError(t, test.run("config", "use", "work"))
	require.NoError(t, test.run("config", "list-accounts", "-o", "json"))
	assert.Contains(t, test.output.String(), "work")
	require.NoError(t, test.run("config", "view", "-o", "yaml"))
	assert.NotContains(t, test.output.String(), "test-token")
	require.NoError(t, test.run("config", "path"))
	assert.Contains(t, test.output.String(), "config.yaml")

	require.NoError(t, test.run("alias", "set", "mine", "pages posts list --all"))
	expanded, err := ExpandAliases([]string{"mine", "-o", "json"}, test.configPath)
	require.NoError(t, err)
	assert.Equal(t, []string{"pages", "posts", "list", "--all", "-o", "json"}, expanded)
	require.NoError(t, test.run("alias", "list", "-o", "json"))
	assert.Contains(t, test.output.String(), "mine")
	assert.Error(t, test.run("alias", "set", "auth", "pages posts list"))
	require.NoError(t, test.run("alias", "remove", "mine"))
	require.NoError(t, test.run("config", "remove", "work"))
}

func TestRawAPIDoctorVersionAndCompletion(t *testing.T) {
	test, serverURL := newCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/v26.0/me" {
			_, _ = io.WriteString(writer, `{"id":"user-1","name":"Example"}`)
			return
		}
		if request.URL.Path == "/v26.0/object" {
			assert.Equal(t, "one", request.URL.Query().Get("key"))
			_, _ = io.WriteString(writer, `{"id":"object-1"}`)
			return
		}
		http.NotFound(writer, request)
	})
	require.NoError(t, test.run("--base-url", serverURL, "api", "GET", "object", "-q", "key=one", "-o", "json"))
	assert.Contains(t, test.output.String(), "object-1")
	require.NoError(t, test.run("--base-url", serverURL, "doctor", "--json"))
	assert.Contains(t, test.output.String(), "connectivity")
	require.NoError(t, test.run("version", "--json", "-o", "json"))
	assert.Contains(t, test.output.String(), "version")
	require.NoError(t, test.run("completion", "bash"))
	assert.Contains(t, test.output.String(), "bash completion")
}

func TestInitAndPromptHelpers(t *testing.T) {
	test, serverURL := newCommandTest(t, func(writer http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(writer, `{"id":"user-1"}`) })
	delete(test.store.values, "default")
	require.NoError(t, test.run("--account", "new", "init", "--token", "new-token", "--graph-url", serverURL, "--resumable-url", serverURL, "--page", "123"))
	assert.Equal(t, "new-token", test.store.values["new"].Token)
	assert.Equal(t, "secret", sanitizeSecret("\x1b[200~ secret \x1b[201~"))

	var output bytes.Buffer
	root := NewRootCmd(Dependencies{In: strings.NewReader("line value\n"), Out: &output, Err: &output, ConfigPath: filepath.Join(t.TempDir(), "config.yaml"), Store: test.store})
	line, err := promptLine(root, "Prompt: ")
	require.NoError(t, err)
	assert.Equal(t, "line value", line)

	root.SetIn(strings.NewReader(" pasted-secret\n"))
	secret, err := promptSecret(root, "Secret: ")
	require.NoError(t, err)
	assert.Equal(t, "pasted-secret", secret)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestVersionCheck(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		assert.Equal(t, "api.github.com", request.URL.Host)
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`{"tag_name":"v9.9.9"}`)),
			Header:     make(http.Header),
		}, nil
	})}
	var output bytes.Buffer
	root := NewRootCmd(Dependencies{Out: &output, Err: &output, HTTPClient: client, ConfigPath: filepath.Join(t.TempDir(), "config.yaml"), Store: &memoryStore{values: map[string]auth.Credential{}}})
	root.SetArgs([]string{"version", "--check", "--json", "-o", "json"})
	require.NoError(t, root.Execute())
	assert.Contains(t, output.String(), "v9.9.9")
}
