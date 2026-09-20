//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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
	mu                    sync.Mutex
	requests              []string
	containerPolls        int
	instagramCreated      bool
	instagramUploaded     bool
	instagramPublished    bool
	errorContainerCreated bool
	pageVideoStarted      bool
	pageVideoOffset       int
	pageVideoFinished     bool
	accountsCalls         int
	template              map[string]any
	templateWasUpdated    bool
	templateWasDeleted    bool
	whatsAppMessageSent   bool
	whatsAppMediaUploaded bool
}

func (fake *fakeGraph) handler(writer http.ResponseWriter, request *http.Request) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	fake.requests = append(fake.requests, request.Method+" "+request.URL.RequestURI())
	writer.Header().Set("Content-Type", "application/json")
	if !fake.authorized(request) {
		graphError(writer, http.StatusUnauthorized, 190, "Invalid OAuth access token", "auth-trace")
		return
	}

	switch {
	case request.Method == http.MethodPost && request.URL.Path == "/v26.0/ig-1/media":
		body, ok := decodeJSON(writer, request)
		if !ok {
			return
		}
		if body["media_type"] != "REELS" {
			graphError(writer, http.StatusBadRequest, 100, "media_type must be REELS", "ig-create")
			return
		}
		if body["caption"] == "Error" {
			if !requiredString(body, "video_url") {
				graphError(writer, http.StatusBadRequest, 100, "video_url is required", "ig-create-error")
				return
			}
			fake.errorContainerCreated = true
			_, _ = io.WriteString(writer, `{"id":"container-error"}`)
			return
		}
		if body["upload_type"] != "resumable" || body["video_url"] != nil {
			graphError(writer, http.StatusBadRequest, 100, "upload_type must be resumable for a local reel", "ig-create")
			return
		}
		if body["cover_url"] != "https://cdn.example/cover.jpg" || body["caption"] != "Launch" {
			graphError(writer, http.StatusBadRequest, 100, "required reel fields are missing", "ig-create")
			return
		}
		fake.instagramCreated = true
		_, _ = io.WriteString(writer, `{"id":"container-1"}`)
	case request.Method == http.MethodPost && request.URL.Path == "/ig-api-upload/v26.0/container-1":
		if !fake.instagramCreated || fake.instagramUploaded {
			graphError(writer, http.StatusBadRequest, 100, "container must be created before upload", "ig-upload-order")
			return
		}
		if request.Header.Get("offset") != "0" || request.Header.Get("file_size") != "10" || request.Header.Get("Content-Type") != "application/octet-stream" {
			graphError(writer, http.StatusBadRequest, 100, "invalid resumable upload headers", "ig-upload")
			return
		}
		body, err := io.ReadAll(request.Body)
		if err != nil || string(body) != "fake-video" {
			graphError(writer, http.StatusBadRequest, 100, "upload body size does not match file_size", "ig-upload")
			return
		}
		fake.instagramUploaded = true
		_, _ = io.WriteString(writer, `{"success":true}`)
	case request.Method == http.MethodGet && request.URL.Path == "/v26.0/container-1":
		if !fake.instagramCreated || !fake.instagramUploaded {
			graphError(writer, http.StatusBadRequest, 100, "container upload is incomplete", "ig-status-order")
			return
		}
		fake.containerPolls++
		if fake.containerPolls == 1 {
			_, _ = io.WriteString(writer, `{"id":"container-1","status_code":"IN_PROGRESS"}`)
			return
		}
		_, _ = io.WriteString(writer, `{"id":"container-1","status_code":"FINISHED"}`)
	case request.Method == http.MethodGet && request.URL.Path == "/v26.0/container-error":
		if !fake.errorContainerCreated {
			graphError(writer, http.StatusBadRequest, 100, "container was not created", "ig-error-order")
			return
		}
		_, _ = io.WriteString(writer, `{"id":"container-error","status_code":"ERROR"}`)
	case request.Method == http.MethodPost && request.URL.Path == "/v26.0/ig-1/media_publish":
		body, ok := decodeJSON(writer, request)
		if !ok || body["creation_id"] != "container-1" || !fake.instagramUploaded || fake.containerPolls < 2 || fake.instagramPublished {
			graphError(writer, http.StatusBadRequest, 100, "container is not ready", "ig-publish")
			return
		}
		fake.instagramPublished = true
		_, _ = io.WriteString(writer, `{"id":"ig-media-1"}`)
	case request.Method == http.MethodPost && request.URL.Path == "/v26.0/ig-media-1/comments":
		body, ok := decodeJSON(writer, request)
		if !ok || !fake.instagramPublished || !requiredString(body, "message") || body["message"] != "First" {
			graphError(writer, http.StatusBadRequest, 100, "message is required", "ig-comment")
			return
		}
		graphError(writer, http.StatusForbidden, 10, "comment permission denied", "ig-comment-denied")
	case request.Method == http.MethodPost && request.URL.Path == "/v26.0/page-1/feed":
		body, ok := decodeJSON(writer, request)
		if !ok || body["published"] != false || body["scheduled_publish_time"] == nil || !requiredString(body, "message") {
			graphError(writer, http.StatusBadRequest, 100, "scheduled post fields are required", "page-post")
			return
		}
		_, _ = io.WriteString(writer, `{"id":"page-1_post-1"}`)
	case request.Method == http.MethodGet && request.URL.Path == "/v26.0/page-1/feed":
		if request.URL.Query().Get("fields") == "" {
			graphError(writer, http.StatusBadRequest, 100, "fields is required", "page-list")
			return
		}
		if request.URL.Query().Get("after") == "cursor-1" {
			_, _ = io.WriteString(writer, `{"data":[{"id":"post-2","message":"Second"}]}`)
		} else {
			_, _ = io.WriteString(writer, `{"data":[{"id":"post-1","message":"First"}],"paging":{"cursors":{"after":"cursor-1"},"next":"https://untrusted.invalid/page"}}`)
		}
	case request.Method == http.MethodPost && request.URL.Path == "/v26.0/page-1/videos" && request.URL.Query().Get("upload_phase") == "start":
		if request.URL.Query().Get("file_size") != "10" || fake.pageVideoStarted {
			graphError(writer, http.StatusBadRequest, 100, "invalid video start", "page-video-start")
			return
		}
		fake.pageVideoStarted = true
		_, _ = io.WriteString(writer, `{"upload_session_id":"session-1","video_id":"video-1","start_offset":"0","end_offset":"4"}`)
	case request.Method == http.MethodPost && request.URL.Path == "/v26.0/page-1/videos" && request.URL.Query().Get("upload_phase") == "transfer":
		fake.pageVideoTransfer(writer, request)
	case request.Method == http.MethodPost && request.URL.Path == "/v26.0/page-1/videos" && request.URL.Query().Get("upload_phase") == "finish":
		if !fake.pageVideoStarted || fake.pageVideoOffset != 10 || request.URL.Query().Get("upload_session_id") != "session-1" {
			graphError(writer, http.StatusBadRequest, 100, "video upload is incomplete", "page-video-finish")
			return
		}
		fake.pageVideoFinished = true
		_, _ = io.WriteString(writer, `{"success":true,"id":"video-1"}`)
	case request.Method == http.MethodPost && request.URL.Path == "/v26.0/video-1/thumbnails":
		if !fake.pageVideoFinished {
			graphError(writer, http.StatusBadRequest, 100, "video must be finished first", "page-thumbnail")
			return
		}
		fields, ok := decodeMultipart(writer, request)
		if !ok || fields["source"].value != "fake-image" || fields["source"].filename != "thumbnail.jpg" ||
			fields["source"].contentType != "image/jpeg" || fields["is_preferred"].value != "true" {
			graphError(writer, http.StatusBadRequest, 100, "source image file is required", "page-thumbnail")
			return
		}
		_, _ = io.WriteString(writer, `{"success":true}`)
	case request.Method == http.MethodGet && request.URL.Path == "/v26.0/me/accounts":
		if request.URL.Query().Get("fields") == "" {
			graphError(writer, http.StatusBadRequest, 100, "fields is required", "accounts")
			return
		}
		fake.accountsCalls++
		if fake.accountsCalls == 1 {
			writer.Header().Set("Retry-After", "0")
			writer.Header().Set("X-Business-Use-Case-Usage", `{"business-1":[{"call_count":100,"estimated_time_to_regain_access":0}]}`)
			graphError(writer, http.StatusBadRequest, 613, "Calls to this API have exceeded the rate limit", "rate-trace")
			return
		}
		writer.Header().Set("X-App-Usage", `{"call_count":80,"total_time":20,"total_cputime":10}`)
		_, _ = io.WriteString(writer, `{"data":[{"id":"page-1","name":"Example Page","tasks":["CREATE_CONTENT"]}]}`)
	case request.URL.Path == "/v26.0/waba-1/message_templates" && request.Method == http.MethodPost:
		body, ok := decodeJSON(writer, request)
		if !ok || !requiredString(body, "name") || !requiredString(body, "language") || !requiredString(body, "category") || body["components"] == nil {
			graphError(writer, http.StatusBadRequest, 100, "template fields are required", "wa-template")
			return
		}
		fake.template = body
		_, _ = io.WriteString(writer, `{"id":"template-1","status":"PENDING"}`)
	case request.URL.Path == "/v26.0/waba-1/message_templates" && request.Method == http.MethodGet:
		encoded, _ := json.Marshal(fake.template["components"])
		fmt.Fprintf(writer, `{"data":[{"id":"template-1","name":"%s","language":"en_US","category":"UTILITY","components":%s}]}`, fake.template["name"], encoded)
	case request.URL.Path == "/v26.0/template-1" && request.Method == http.MethodGet:
		_, _ = io.WriteString(writer, `{"id":"template-1","name":"order_ready","status":"APPROVED"}`)
	case request.URL.Path == "/v26.0/template-1" && request.Method == http.MethodPost:
		body, ok := decodeJSON(writer, request)
		if !ok {
			return
		}
		if fake.template == nil || (!requiredString(body, "category") && body["components"] == nil) {
			graphError(writer, http.StatusBadRequest, 100, "template must exist and include an update field", "wa-template-update")
			return
		}
		fake.templateWasUpdated = true
		_, _ = io.WriteString(writer, `{"success":true}`)
	case request.URL.Path == "/v26.0/waba-1/message_templates" && request.Method == http.MethodDelete:
		fake.templateWasDeleted = request.URL.Query().Get("name") == "order_ready" && request.URL.Query().Get("hsm_id") == "template-1"
		if !fake.templateWasDeleted {
			graphError(writer, http.StatusBadRequest, 100, "name and hsm_id are required", "wa-template-delete")
			return
		}
		_, _ = io.WriteString(writer, `{"success":true}`)
	case request.URL.Path == "/v26.0/phone-1/media" && request.Method == http.MethodPost:
		fields, ok := decodeMultipart(writer, request)
		if !ok || fields["messaging_product"].value != "whatsapp" || fields["type"].value != "image/jpeg" ||
			fields["file"].value != "fake-image" || fields["file"].filename != "thumbnail.jpg" || fields["file"].contentType != "image/jpeg" {
			graphError(writer, http.StatusBadRequest, 100, "file, type, and messaging_product are required", "wa-media")
			return
		}
		fake.whatsAppMediaUploaded = true
		_, _ = io.WriteString(writer, `{"id":"wa-media-1"}`)
	case request.URL.Path == "/v26.0/phone-1/messages" && request.Method == http.MethodPost:
		body, ok := decodeJSON(writer, request)
		text, _ := body["text"].(map[string]any)
		if !ok || body["messaging_product"] != "whatsapp" || body["recipient_type"] != "individual" || body["type"] != "text" || !requiredString(body, "to") || !requiredString(text, "body") {
			graphError(writer, http.StatusBadRequest, 100, "WhatsApp text fields are required", "wa-send")
			return
		}
		fake.whatsAppMessageSent = true
		_, _ = io.WriteString(writer, `{"messaging_product":"whatsapp","messages":[{"id":"wamid.1"}]}`)
	default:
		graphError(writer, http.StatusNotFound, 100, "Unsupported fake Graph request", "not-found")
	}
}

func (fake *fakeGraph) authorized(request *http.Request) bool {
	expected := "Bearer fake-token"
	if request.URL.Path == "/v26.0/page-1/feed" || request.URL.Path == "/v26.0/page-1/videos" || request.URL.Path == "/v26.0/video-1/thumbnails" {
		expected = "Bearer page-token"
	}
	if strings.HasPrefix(request.URL.Path, "/ig-api-upload/") || strings.HasPrefix(request.URL.Path, "/video-upload/") {
		expected = "OAuth fake-token"
	}
	return request.Header.Get("Authorization") == expected
}

func (fake *fakeGraph) pageVideoTransfer(writer http.ResponseWriter, request *http.Request) {
	if !fake.pageVideoStarted || fake.pageVideoFinished || request.URL.Query().Get("upload_session_id") != "session-1" || request.URL.Query().Get("start_offset") != strconv.Itoa(fake.pageVideoOffset) {
		graphError(writer, http.StatusBadRequest, 100, "video transfer step is out of order", "page-video-transfer")
		return
	}
	fields, ok := decodeMultipart(writer, request)
	if !ok {
		return
	}
	want := []string{"fake", "-vid", "eo"}
	if fake.pageVideoOffset == 4 {
		want = want[1:]
	} else if fake.pageVideoOffset == 8 {
		want = want[2:]
	}
	chunk := fields["video_file_chunk"]
	if len(want) == 0 || chunk.value != want[0] || chunk.filename != "video.mp4" || chunk.contentType != "application/octet-stream" {
		graphError(writer, http.StatusBadRequest, 100, "video chunk has the wrong size or bytes", "page-video-transfer")
		return
	}
	fake.pageVideoOffset += len(want[0])
	nextEnd := fake.pageVideoOffset + 4
	if nextEnd > 10 {
		nextEnd = 10
	}
	fmt.Fprintf(writer, `{"start_offset":"%d","end_offset":"%d"}`, fake.pageVideoOffset, nextEnd)
}

func decodeJSON(writer http.ResponseWriter, request *http.Request) (map[string]any, bool) {
	if !strings.HasPrefix(request.Header.Get("Content-Type"), "application/json") {
		graphError(writer, http.StatusBadRequest, 100, "application/json is required", "content-type")
		return nil, false
	}
	var body map[string]any
	decoder := json.NewDecoder(request.Body)
	if err := decoder.Decode(&body); err != nil {
		graphError(writer, http.StatusBadRequest, 100, "invalid JSON body", "json")
		return nil, false
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		graphError(writer, http.StatusBadRequest, 100, "JSON body must contain exactly one bounded object", "json")
		return nil, false
	}
	return body, true
}

type multipartPart struct {
	value       string
	filename    string
	contentType string
}

func decodeMultipart(writer http.ResponseWriter, request *http.Request) (map[string]multipartPart, bool) {
	reader, err := request.MultipartReader()
	if err != nil {
		graphError(writer, http.StatusBadRequest, 100, "multipart/form-data is required", "multipart")
		return nil, false
	}
	values := map[string]multipartPart{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			graphError(writer, http.StatusBadRequest, 100, "invalid multipart body", "multipart")
			return nil, false
		}
		body, err := io.ReadAll(io.LimitReader(part, 64<<10))
		_ = part.Close()
		if err != nil {
			graphError(writer, http.StatusBadRequest, 100, "invalid multipart part", "multipart")
			return nil, false
		}
		values[part.FormName()] = multipartPart{value: string(body), filename: part.FileName(), contentType: part.Header.Get("Content-Type")}
	}
	return values, true
}

func requiredString(value map[string]any, key string) bool {
	text, ok := value[key].(string)
	return ok && strings.TrimSpace(text) != ""
}

func graphError(writer http.ResponseWriter, status, code int, message, trace string) {
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]any{"error": map[string]any{
		"message": message, "type": "OAuthException", "code": code, "fbtrace_id": trace,
	}})
}

func TestStrictFakeRejectsInvalidRequests(t *testing.T) {
	fake := &fakeGraph{}
	server := httptest.NewServer(http.HandlerFunc(fake.handler))
	t.Cleanup(server.Close)

	assertGraphFailure(t, rawRequest(t, server.URL+"/v26.0/container-1", http.MethodGet, "", "", nil), http.StatusUnauthorized)
	assertGraphFailure(t, rawRequest(t, server.URL+"/v26.0/container-1", http.MethodGet, "Bearer fake-token", "", nil), http.StatusBadRequest)
	assertGraphFailure(t, rawRequest(t, server.URL+"/v26.0/ig-1/media", http.MethodPost, "Bearer fake-token", "application/json", strings.NewReader(`{"caption":"missing required fields"}`)), http.StatusBadRequest)
	assertGraphFailure(t, rawRequest(t, server.URL+"/v26.0/page-1/videos?upload_phase=transfer&upload_session_id=session-1&start_offset=0", http.MethodPost, "Bearer page-token", "multipart/form-data", strings.NewReader("invalid")), http.StatusBadRequest)
	assertGraphFailure(t, rawRequest(t, server.URL+"/v26.0/template-1", http.MethodPost, "Bearer fake-token", "application/json", strings.NewReader(`{"category":"MARKETING"}`)), http.StatusBadRequest)
	assertGraphFailure(t, rawRequest(t, server.URL+"/v26.0/ig-media-1/comments", http.MethodPost, "Bearer fake-token", "application/json", strings.NewReader(`{"message":"First"}`)), http.StatusBadRequest)
	oversizedJSON := `{"media_type":"REELS","upload_type":"resumable"}` + strings.Repeat(" ", (1<<20)+1)
	assertGraphFailure(t, rawRequest(t, server.URL+"/v26.0/ig-1/media", http.MethodPost, "Bearer fake-token", "application/json", strings.NewReader(oversizedJSON)), http.StatusBadRequest)

	validCreate := `{"media_type":"REELS","upload_type":"resumable","caption":"Launch","cover_url":"https://cdn.example/cover.jpg"}`
	response := rawRequest(t, server.URL+"/v26.0/ig-1/media", http.MethodPost, "Bearer fake-token", "application/json", strings.NewReader(validCreate))
	require.Equal(t, http.StatusOK, response.StatusCode)
	_ = response.Body.Close()
	request, err := http.NewRequest(http.MethodPost, server.URL+"/ig-api-upload/v26.0/container-1", strings.NewReader("short"))
	require.NoError(t, err)
	request.Header.Set("Authorization", "OAuth fake-token")
	request.Header.Set("offset", "0")
	request.Header.Set("file_size", "10")
	request.Header.Set("Content-Type", "application/octet-stream")
	assertGraphFailure(t, executeRequest(t, request), http.StatusBadRequest)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("messaging_product", "whatsapp"))
	require.NoError(t, writer.WriteField("type", "image/jpeg"))
	require.NoError(t, writer.WriteField("file", "fake-image"))
	require.NoError(t, writer.Close())
	assertGraphFailure(t, rawRequest(t, server.URL+"/v26.0/phone-1/media", http.MethodPost, "Bearer fake-token", writer.FormDataContentType(), &body), http.StatusBadRequest)

	templateCreate := `{"name":"order_ready","language":"en_US","category":"UTILITY","components":[{"type":"BODY","text":"Ready"}]}`
	response = rawRequest(t, server.URL+"/v26.0/waba-1/message_templates", http.MethodPost, "Bearer fake-token", "application/json", strings.NewReader(templateCreate))
	require.Equal(t, http.StatusOK, response.StatusCode)
	_ = response.Body.Close()
	assertGraphFailure(t, rawRequest(t, server.URL+"/v26.0/template-1", http.MethodPost, "Bearer fake-token", "application/json", strings.NewReader(`{}`)), http.StatusBadRequest)
}

func rawRequest(t *testing.T, target, method, authorization, contentType string, body io.Reader) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, target, body)
	require.NoError(t, err)
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	return executeRequest(t, request)
}

func executeRequest(t *testing.T, request *http.Request) *http.Response {
	t.Helper()
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	return response
}

func assertGraphFailure(t *testing.T, response *http.Response, status int) {
	t.Helper()
	defer func() { _ = response.Body.Close() }()
	assert.Equal(t, status, response.StatusCode)
	var envelope struct {
		Error struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
			TraceID string `json:"fbtrace_id"`
		} `json:"error"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&envelope))
	assert.NotEmpty(t, envelope.Error.Message)
	assert.NotZero(t, envelope.Error.Code)
	assert.NotEmpty(t, envelope.Error.TraceID)
}

func TestEndToEndStrictFakeGraph(t *testing.T) {
	fake := &fakeGraph{}
	server := httptest.NewServer(http.HandlerFunc(fake.handler))
	t.Cleanup(server.Close)
	mediaPath := filepath.Join(t.TempDir(), "video.mp4")
	thumbnailPath := filepath.Join(t.TempDir(), "thumbnail.jpg")
	require.NoError(t, os.WriteFile(mediaPath, []byte("fake-video"), 0o600))
	require.NoError(t, os.WriteFile(thumbnailPath, []byte("fake-image"), 0o600))
	environment := []string{
		"METACTL_TOKEN=fake-token", "METACTL_PAGE_TOKEN=page-token",
		"METACTL_BASE_URL=" + server.URL, "METACTL_UPLOAD_URL=" + server.URL,
		"METACTL_INSTAGRAM_ID=ig-1", "METACTL_PAGE_ID=page-1", "METACTL_WABA_ID=waba-1",
		"METACTL_PHONE_ID=phone-1", "METACTL_BUSINESS_ID=business-1", "NO_COLOR=1",
		"XDG_CONFIG_HOME=" + t.TempDir(),
	}

	output, exitCode := runCLIExit(t, environment, "instagram", "publish", "reel", "--video", mediaPath, "--cover-url", "https://cdn.example/cover.jpg", "--caption", "Launch", "--first-comment", "First", "--poll-interval", "1ms", "-o", "json")
	assert.Equal(t, 2, exitCode, output)
	assert.Contains(t, output, `"id": "ig-media-1"`)
	assert.Contains(t, output, `"first_comment_status": "failed"`)
	assertGraphFailure(t, rawRequest(t, server.URL+"/v26.0/ig-1/media_publish", http.MethodPost, "Bearer fake-token", "application/json", strings.NewReader(`{"creation_id":"container-1"}`)), http.StatusBadRequest)

	output, exitCode = runCLIExit(t, environment, "instagram", "publish", "reel", "--video", "https://cdn.example/error.mp4", "--caption", "Error", "--poll-interval", "1ms", "-o", "json")
	assert.Equal(t, 1, exitCode, output)
	assert.Contains(t, output, "entered ERROR status")

	output = runCLI(t, environment, "pages", "posts", "create", "--message", "Scheduled", "--published=false", "--scheduled-at", "1789900000", "-o", "json")
	assert.Contains(t, output, "page-1_post-1")

	output = runCLI(t, environment, "pages", "videos", "publish", "--file", mediaPath, "--title", "Video", "--thumbnail-file", thumbnailPath, "-o", "json")
	assert.Contains(t, output, "video-1")
	assert.Equal(t, 10, fake.pageVideoOffset)

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
	runCLI(t, environment, "whatsapp", "templates", "delete", "--name", "order_ready", "--id", "template-1", "--yes", "-o", "json")
	assert.True(t, fake.templateWasUpdated)
	assert.True(t, fake.templateWasDeleted)

	output = runCLI(t, environment, "whatsapp", "media", "upload", "--file", thumbnailPath, "-o", "json")
	assert.Contains(t, output, "wa-media-1")
	output = runCLI(t, environment, "whatsapp", "send", "text", "--to", "15551234567", "--message", "Hello", "-o", "json")
	assert.Contains(t, output, "wamid.1")
	assert.True(t, fake.whatsAppMediaUploaded)
	assert.True(t, fake.whatsAppMessageSent)

	joined := strings.Join(fake.requests, "\n")
	assert.NotContains(t, joined, "graph.facebook.com")
}

func TestMCPExecutesConfinedSubprocessesWithoutCredentialLeaks(t *testing.T) {
	const (
		userToken      = "mcp-user-token-secret"
		pageToken      = "mcp-page-token-secret"
		appSecret      = "mcp-app-secret-value"
		discoveredPage = "newly-discovered-page-credential"
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/v26.0/me/accounts" {
			_ = json.NewEncoder(writer).Encode(map[string]any{"data": []any{map[string]any{
				"id": "page-1", "name": "echo " + discoveredPage, "access_token": discoveredPage,
			}}})
			return
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"data": []any{map[string]any{
			"id":         "mcp-post-1",
			"message":    "echo " + pageToken + " " + appSecret,
			"paging_url": "https://graph.example/next?access_token=" + pageToken + "&appsecret_proof=proof-value",
		}}})
	}))
	t.Cleanup(server.Close)

	mcpRoot := t.TempDir()
	workingDirectory := t.TempDir()
	inside := filepath.Join(mcpRoot, "inside.mp4")
	require.NoError(t, os.WriteFile(inside, []byte("fake-video"), 0o600))
	outside := filepath.Join(t.TempDir(), "outside.mp4")
	require.NoError(t, os.WriteFile(outside, []byte("outside"), 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(mcpRoot, "escape.mp4")))

	process := exec.Command(binaryPath, "mcp", "start") // #nosec G204 -- binaryPath is produced by this test suite
	process.Dir = workingDirectory
	process.Env = append(os.Environ(),
		"METACTL_MCP_ROOT="+mcpRoot,
		"METACTL_TOKEN="+userToken,
		"METACTL_PAGE_TOKEN="+pageToken,
		"METACTL_APP_SECRET="+appSecret,
		"METACTL_BASE_URL="+server.URL,
		"METACTL_UPLOAD_URL="+server.URL,
		"METACTL_PAGE_ID=page-1",
		"METACTL_INSTAGRAM_ID=ig-1",
		"METACTL_WABA_ID=waba-1",
		"XDG_CONFIG_HOME="+t.TempDir(),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "metactl-e2e", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: process}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, session.Close()) })

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "metactl_instagram_containers_upload",
		Arguments: map[string]any{
			"args":  []string{"container-1"},
			"flags": map[string]any{"file": "inside.mp4", "dry-run": true},
		},
	})
	require.NoError(t, err)
	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), inside)
	for _, secret := range []string{userToken, pageToken, appSecret} {
		assert.NotContains(t, string(encoded), secret)
	}

	result, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "metactl_instagram_containers_upload",
		Arguments: map[string]any{
			"args":  []string{"container-1"},
			"flags": map[string]any{"file": "escape.mp4", "dry-run": true},
		},
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	encoded, err = json.Marshal(result)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), "outside METACTL_MCP_ROOT")

	result, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "metactl_instagram_containers_upload",
		Arguments: map[string]any{
			"args":  []string{"container-1"},
			"flags": map[string]any{"file": appSecret + ".mp4", "dry-run": true},
		},
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	encoded, err = json.Marshal(result)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), appSecret)
	assert.Contains(t, string(encoded), "redacted")

	result, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "metactl_pages_posts_list",
		Arguments: map[string]any{"flags": map[string]any{"output": "json"}},
	})
	require.NoError(t, err)
	encoded, err = json.Marshal(result)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), "mcp-post-1")
	for _, secret := range []string{userToken, pageToken, appSecret} {
		assert.NotContains(t, string(encoded), secret)
	}
	assert.NotContains(t, string(encoded), "proof-value")

	result, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "metactl_pages_posts_list",
		Arguments: map[string]any{"flags": map[string]any{
			"output": "json", "jq": `"` + appSecret + `"`,
		}},
	})
	require.NoError(t, err)
	encoded, err = json.Marshal(result)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), appSecret)
	assert.Contains(t, string(encoded), "redacted")

	result, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "metactl_instagram_media_get",
		Arguments: map[string]any{
			"args":  []string{appSecret},
			"flags": map[string]any{"output": "json", "verbose": true},
		},
	})
	require.NoError(t, err)
	encoded, err = json.Marshal(result)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), "redacted")
	for _, secret := range []string{userToken, pageToken, appSecret} {
		assert.NotContains(t, string(encoded), secret)
	}

	result, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "metactl_instagram_containers_create",
		Arguments: map[string]any{
			"flags": map[string]any{"type": appSecret, "dry-run": true},
		},
	})
	require.NoError(t, err)
	encoded, err = json.Marshal(result)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), appSecret)
	assert.Contains(t, string(encoded), "redacted")

	result, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "metactl_pages_posts_create",
		Arguments: map[string]any{"flags": map[string]any{
			"message": "safe", "dry-run": true, "output": "csv", "columns": appSecret + "," + pageToken,
		}},
	})
	require.NoError(t, err)
	encoded, err = json.Marshal(result)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), appSecret)
	assert.NotContains(t, string(encoded), pageToken)
	assert.Contains(t, string(encoded), "redacted")

	result, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "metactl_pages_accounts_list",
		Arguments: map[string]any{"flags": map[string]any{
			"fields": "id,name,access_token", "output": "json",
		}},
	})
	require.NoError(t, err)
	encoded, err = json.Marshal(result)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), discoveredPage)
	assert.Contains(t, string(encoded), "redacted")

	result, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "metactl_pages_posts_create",
		Arguments: map[string]any{"flags": map[string]any{
			"message": "draft", "published": false, "dry-run": true, "output": "json",
		}},
	})
	require.NoError(t, err)
	structured, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	var toolOutput struct {
		StdErr string `json:"stderr"`
	}
	require.NoError(t, json.Unmarshal(structured, &toolOutput))
	assert.Contains(t, toolOutput.StdErr, `"published":false`)
	assert.NotContains(t, toolOutput.StdErr, `"published":true`)

	result, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "metactl_whatsapp_templates_delete",
		Arguments: map[string]any{"flags": map[string]any{
			"name": appSecret, "dry-run": true, "output": "json",
		}},
	})
	require.NoError(t, err)
	encoded, err = json.Marshal(result)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), appSecret)
	assert.Contains(t, string(encoded), "redacted")
}

func runCLI(t *testing.T, environment []string, args ...string) string {
	t.Helper()
	output, exitCode := runCLIExit(t, environment, args...)
	require.Equal(t, 0, exitCode, output)
	return output
}

func runCLIExit(t *testing.T, environment []string, args ...string) (string, int) {
	t.Helper()
	command := exec.Command(binaryPath, args...) // #nosec G204 -- binary path is built by TestMain and arguments are fixed test data
	command.Env = append(os.Environ(), environment...)
	output, err := command.CombinedOutput()
	if err == nil {
		return string(output), 0
	}
	var exitError *exec.ExitError
	require.ErrorAs(t, err, &exitError, string(output))
	return string(output), exitError.ExitCode()
}
