//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	binaryPath string
	buildDir   string
)

func TestMain(main *testing.M) {
	var err error
	buildDir, err = os.MkdirTemp("", "metactl-e2e-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	binaryName := "metactl"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath = filepath.Join(buildDir, binaryName)
	command := exec.Command("go", "build", "-o", binaryPath, "../cmd/metactl") // #nosec G204 -- fixed build command and package
	command.Stdout = os.Stderr
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		_ = os.RemoveAll(buildDir)
		os.Exit(1)
	}
	code := main.Run()
	_ = os.RemoveAll(buildDir)
	os.Exit(code)
}

type fakeGraph struct {
	mu                 sync.Mutex
	requests           []string
	videoTransferCalls int
	accountsCalls      int
	template           map[string]any
	templateWasUpdated bool
	templateWasDeleted bool
}

func (fake *fakeGraph) handler(writer http.ResponseWriter, request *http.Request) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.requests = append(fake.requests, request.Method+" "+request.URL.RequestURI())
	writer.Header().Set("Content-Type", "application/json")
	switch {
	case request.Method == http.MethodPost && request.URL.Path == "/v26.0/ig-1/media":
		var body map[string]any
		_ = json.NewDecoder(request.Body).Decode(&body)
		if body["cover_url"] != "https://cdn.example/cover.jpg" || body["caption"] != "Launch" {
			http.Error(writer, `{"error":{"message":"missing reel fields","code":100}}`, http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(writer, `{"id":"container-1"}`)
	case request.Method == http.MethodPost && request.URL.Path == "/ig-api-upload/v26.0/container-1":
		if request.Header.Get("offset") != "0" || request.Header.Get("file_size") == "" {
			http.Error(writer, `{"error":{"message":"bad upload headers","code":100}}`, http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(writer, `{"success":true}`)
	case request.Method == http.MethodGet && request.URL.Path == "/v26.0/container-1":
		_, _ = io.WriteString(writer, `{"id":"container-1","status_code":"FINISHED"}`)
	case request.Method == http.MethodPost && request.URL.Path == "/v26.0/ig-1/media_publish":
		_, _ = io.WriteString(writer, `{"id":"ig-media-1"}`)
	case request.Method == http.MethodPost && request.URL.Path == "/v26.0/ig-media-1/comments":
		var body map[string]any
		_ = json.NewDecoder(request.Body).Decode(&body)
		if body["message"] != "First" {
			http.Error(writer, `{"error":{"message":"first comment missing","code":100}}`, http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(writer, `{"id":"comment-1"}`)
	case request.Method == http.MethodPost && request.URL.Path == "/v26.0/page-1/feed":
		var body map[string]any
		_ = json.NewDecoder(request.Body).Decode(&body)
		if body["published"] != false || body["scheduled_publish_time"] == nil {
			http.Error(writer, `{"error":{"message":"post was not scheduled","code":100}}`, http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(writer, `{"id":"page-1_post-1"}`)
	case request.Method == http.MethodGet && request.URL.Path == "/v26.0/page-1/feed":
		if request.URL.Query().Get("after") == "cursor-1" {
			_, _ = io.WriteString(writer, `{"data":[{"id":"post-2","message":"Second"}]}`)
		} else {
			_, _ = io.WriteString(writer, `{"data":[{"id":"post-1","message":"First"}],"paging":{"cursors":{"after":"cursor-1"},"next":"https://untrusted.invalid/page"}}`)
		}
	case request.Method == http.MethodPost && request.URL.Path == "/v26.0/page-1/videos" && request.URL.Query().Get("upload_phase") == "start":
		_, _ = io.WriteString(writer, `{"upload_session_id":"session-1","video_id":"video-1","start_offset":"0","end_offset":"10"}`)
	case request.Method == http.MethodPost && request.URL.Path == "/v26.0/page-1/videos" && request.URL.Query().Get("upload_phase") == "transfer":
		fake.videoTransferCalls++
		if fake.videoTransferCalls == 1 {
			writer.Header().Set("Retry-After", "0")
			writer.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(writer, `{"error":{"message":"temporary","code":2}}`)
			return
		}
		_, _ = io.WriteString(writer, `{"start_offset":"10","end_offset":"10"}`)
	case request.Method == http.MethodPost && request.URL.Path == "/v26.0/page-1/videos" && request.URL.Query().Get("upload_phase") == "finish":
		_, _ = io.WriteString(writer, `{"success":true,"id":"video-1"}`)
	case request.Method == http.MethodPost && request.URL.Path == "/v26.0/video-1/thumbnails":
		_, _ = io.WriteString(writer, `{"success":true}`)
	case request.Method == http.MethodGet && request.URL.Path == "/v26.0/me/accounts":
		fake.accountsCalls++
		if fake.accountsCalls == 1 {
			writer.Header().Set("Retry-After", "0")
			writer.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(writer, `{"error":{"message":"rate limited","code":4}}`)
			return
		}
		writer.Header().Set("X-App-Usage", `{"call_count":80,"total_time":20,"total_cputime":10}`)
		_, _ = io.WriteString(writer, `{"data":[{"id":"page-1","name":"Example Page"}]}`)
	case request.URL.Path == "/v26.0/waba-1/message_templates" && request.Method == http.MethodPost:
		_ = json.NewDecoder(request.Body).Decode(&fake.template)
		_, _ = io.WriteString(writer, `{"id":"template-1","status":"PENDING"}`)
	case request.URL.Path == "/v26.0/waba-1/message_templates" && request.Method == http.MethodGet:
		encoded, _ := json.Marshal(fake.template)
		fmt.Fprintf(writer, `{"data":[{"id":"template-1","name":"%s","language":"en_US","category":"UTILITY","components":%s}]}`, fake.template["name"], encoded)
	case request.URL.Path == "/v26.0/template-1" && request.Method == http.MethodGet:
		_, _ = io.WriteString(writer, `{"id":"template-1","name":"order_ready","status":"APPROVED"}`)
	case request.URL.Path == "/v26.0/template-1" && request.Method == http.MethodPost:
		fake.templateWasUpdated = true
		_, _ = io.WriteString(writer, `{"success":true}`)
	case request.URL.Path == "/v26.0/waba-1/message_templates" && request.Method == http.MethodDelete:
		fake.templateWasDeleted = request.URL.Query().Get("name") == "order_ready"
		_, _ = io.WriteString(writer, `{"success":true}`)
	default:
		http.Error(writer, `{"error":{"message":"not found","code":100}}`, http.StatusNotFound)
	}
}

func TestEndToEndFakeGraph(t *testing.T) {
	fake := &fakeGraph{}
	server := httptest.NewServer(http.HandlerFunc(fake.handler))
	t.Cleanup(server.Close)
	mediaPath := filepath.Join(t.TempDir(), "video.mp4")
	require.NoError(t, os.WriteFile(mediaPath, []byte("fake-video"), 0o600))
	environment := []string{
		"METACTL_TOKEN=fake-token", "METACTL_BASE_URL=" + server.URL, "METACTL_UPLOAD_URL=" + server.URL,
		"METACTL_INSTAGRAM_ID=ig-1", "METACTL_PAGE_ID=page-1", "METACTL_WABA_ID=waba-1",
		"METACTL_PHONE_ID=phone-1", "METACTL_BUSINESS_ID=business-1", "NO_COLOR=1",
		"XDG_CONFIG_HOME=" + t.TempDir(),
	}

	output := runCLI(t, environment, "instagram", "publish", "reel", "--video", mediaPath, "--cover-url", "https://cdn.example/cover.jpg", "--caption", "Launch", "--first-comment", "First", "--poll-interval", "1ms", "-o", "json")
	assert.Contains(t, output, "ig-media-1")

	output = runCLI(t, environment, "pages", "posts", "create", "--message", "Scheduled", "--published=false", "--scheduled-at", "1789900000", "-o", "json")
	assert.Contains(t, output, "page-1_post-1")

	output = runCLI(t, environment, "pages", "videos", "publish", "--file", mediaPath, "--title", "Video", "--thumbnail-url", "https://cdn.example/thumb.jpg", "-o", "json")
	assert.Contains(t, output, "video-1")
	assert.Equal(t, 2, fake.videoTransferCalls)

	output = runCLI(t, environment, "pages", "posts", "list", "--all", "-o", "json")
	assert.Contains(t, output, "post-1")
	assert.Contains(t, output, "post-2")

	output = runCLI(t, environment, "pages", "accounts", "list", "-o", "json")
	assert.Contains(t, output, "Example Page")
	assert.Equal(t, 2, fake.accountsCalls)

	runCLI(t, environment, "whatsapp", "templates", "create", "--name", "order_ready", "--language", "en_US", "--category", "UTILITY", "--components", `[{"type":"BODY","text":"Order {{1}} is ready"}]`, "-o", "json")
	output = runCLI(t, environment, "whatsapp", "templates", "list", "-o", "json")
	assert.Contains(t, output, "order_ready")
	output = runCLI(t, environment, "whatsapp", "templates", "get", "template-1", "-o", "json")
	assert.Contains(t, output, "APPROVED")
	runCLI(t, environment, "whatsapp", "templates", "update", "template-1", "--category", "MARKETING", "-o", "json")
	runCLI(t, environment, "whatsapp", "templates", "delete", "--name", "order_ready", "--yes", "-o", "json")
	assert.True(t, fake.templateWasUpdated)
	assert.True(t, fake.templateWasDeleted)

	joined := strings.Join(fake.requests, "\n")
	assert.NotContains(t, joined, "graph.facebook.com")
}

func runCLI(t *testing.T, environment []string, args ...string) string {
	t.Helper()
	command := exec.Command(binaryPath, args...) // #nosec G204 -- binary path is built by TestMain and arguments are fixed test data
	command.Env = append(os.Environ(), environment...)
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
	return string(output)
}
