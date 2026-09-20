package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jjuanrivvera/meta-cli/internal/api"
	"github.com/jjuanrivvera/meta-cli/internal/auth"
)

func TestPageCredentialFlowAndSecretRedaction(t *testing.T) {
	var pageCalls int
	test, serverURL := newCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v26.0/me/accounts":
			assert.Equal(t, "Bearer test-token", request.Header.Get("Authorization"))
			_, _ = io.WriteString(writer, `{"data":[{"id":"page-1","name":"Page page-secret-token","access_token":"page-secret-token","tasks":["CREATE_CONTENT"]}]}`)
		case "/v26.0/page-1/feed":
			pageCalls++
			assert.Equal(t, "Bearer page-secret-token", request.Header.Get("Authorization"))
			_, _ = io.WriteString(writer, `{"data":[]}`)
		default:
			http.NotFound(writer, request)
		}
	})

	require.NoError(t, test.run("--base-url", serverURL, "--page-id", "page-1", "--show-token", "auth", "pages", "--save", "-o", "json"))
	assert.Equal(t, "page-secret-token", test.store.values["default"].PageToken)
	assert.NotContains(t, test.output.String(), "page-secret-token")

	require.NoError(t, test.run("--base-url", serverURL, "--page-id", "page-1", "--show-token", "pages", "posts", "list", "-o", "json"))
	assert.Equal(t, 1, pageCalls)
	assert.NotContains(t, test.output.String(), "page-secret-token")

	require.NoError(t, test.run("--base-url", serverURL, "--app-id", "app-1", "--dry-run", "--show-token", "auth", "exchange", "-o", "json"))
	assert.NotContains(t, test.output.String(), "test-secret")
	assert.NotContains(t, test.output.String(), "client_secret=test-secret")
}

func TestPageAccountDiscoveryRedactsNewTokensFromOrdinaryFields(t *testing.T) {
	const discovered = "new-page-credential-value"
	test, serverURL := newCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "/v26.0/me/accounts", request.URL.Path)
		assert.Equal(t, "Bearer test-token", request.Header.Get("Authorization"))
		_, _ = io.WriteString(writer, `{"data":[{"id":"page-1","name":"echo `+discovered+`","access_token":"`+discovered+`"}]}`)
	})
	require.NoError(t, test.run("--base-url", serverURL, "pages", "accounts", "list", "--fields", "id,name,access_token", "-o", "json"))
	assert.NotContains(t, test.output.String(), discovered)
	assert.Contains(t, test.output.String(), "redacted")
}

func TestDebugTokenUsesDocumentedAuthenticationShape(t *testing.T) {
	test, serverURL := newCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "/v26.0/debug_token", request.URL.Path)
		assert.Empty(t, request.Header.Get("Authorization"))
		assert.Empty(t, request.URL.Query().Get("appsecret_proof"))
		assert.Equal(t, "test-token", request.URL.Query().Get("input_token"))
		assert.Equal(t, "app-1|test-secret", request.URL.Query().Get("access_token"))
		_, _ = io.WriteString(writer, `{"data":{"is_valid":true}}`)
	})
	require.NoError(t, test.run("--base-url", serverURL, "--app-id", "app-1", "auth", "debug", "-o", "json"))
}

func TestPublishedReelReportsFirstCommentPartialFailure(t *testing.T) {
	videoPath := filepath.Join(t.TempDir(), "reel.mp4")
	require.NoError(t, os.WriteFile(videoPath, []byte("abcdefgh"), 0o600))
	test, serverURL := newCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v26.0/ig-1/media":
			var body map[string]any
			require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
			assert.Equal(t, "REELS", body["media_type"])
			assert.Equal(t, "resumable", body["upload_type"])
			_, _ = io.WriteString(writer, `{"id":"container-1"}`)
		case "/ig-api-upload/v26.0/container-1":
			assert.Equal(t, "OAuth test-token", request.Header.Get("Authorization"))
			assert.Equal(t, "0", request.Header.Get("offset"))
			body, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			assert.Equal(t, "abcdefgh", string(body))
			_, _ = io.WriteString(writer, `{"success":true}`)
		case "/v26.0/container-1":
			_, _ = io.WriteString(writer, `{"id":"container-1","status_code":"FINISHED"}`)
		case "/v26.0/ig-1/media_publish":
			_, _ = io.WriteString(writer, `{"id":"media-1"}`)
		case "/v26.0/media-1/comments":
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(writer, `{"error":{"message":"comment denied for test-token with test-secret","type":"OAuthException","code":10,"fbtrace_id":"comment-trace"}}`)
		default:
			http.NotFound(writer, request)
		}
	})
	err := test.run("--base-url", serverURL, "--upload-url", serverURL, "--instagram-id", "ig-1", "instagram", "publish", "reel", "--video", videoPath, "--first-comment", "First", "--poll-interval", "1ms", "-o", "json")
	require.Error(t, err)
	assert.Equal(t, 2, ExitCode(err))
	assert.Contains(t, test.output.String(), `"id": "media-1"`)
	assert.Contains(t, test.output.String(), `"first_comment_status": "failed"`)
	assert.Contains(t, test.output.String(), "was published")
	assert.NotContains(t, test.output.String(), "test-token")
	assert.NotContains(t, test.output.String(), "test-secret")
}

func TestInstagramUploadResumesFromReportedOffset(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "reel.mp4")
	require.NoError(t, os.WriteFile(filePath, []byte("abcdefgh"), 0o600))
	var uploads int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/ig-api-upload/v26.0/container-1":
			uploads++
			body, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			if uploads == 1 {
				assert.Equal(t, "0", request.Header.Get("offset"))
				assert.Equal(t, "abcdefgh", string(body))
				writer.WriteHeader(http.StatusInternalServerError)
				_, _ = io.WriteString(writer, `{"error":{"message":"connection interrupted","code":2}}`)
				return
			}
			assert.Equal(t, "4", request.Header.Get("offset"))
			assert.Equal(t, "efgh", string(body))
			_, _ = io.WriteString(writer, `{"success":true}`)
		case "/v26.0/container-1":
			_, _ = io.WriteString(writer, `{"video_status":{"uploading_phase":{"bytes_transferred":4}}}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(server.Close)
	client, err := api.New(api.Options{BaseURL: server.URL, UploadURL: server.URL, Token: "token", HTTPClient: server.Client(), RequestsPS: 100000})
	require.NoError(t, err)
	command := &cobra.Command{}
	command.SetContext(context.Background())
	result, err := uploadInstagramContainer(command, client, "container-1", filePath, "")
	require.NoError(t, err)
	assert.Equal(t, true, result.(map[string]any)["success"])
	assert.Equal(t, 2, uploads)
}

func TestPageReelUploadResumesFromReportedOffset(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "reel.mp4")
	require.NoError(t, os.WriteFile(filePath, []byte("abcdefgh"), 0o600))
	var uploads int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/video-upload/v26.0/video-1":
			uploads++
			body, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			if uploads == 1 {
				assert.Equal(t, "0", request.Header.Get("offset"))
				assert.Equal(t, "abcdefgh", string(body))
				writer.WriteHeader(http.StatusInternalServerError)
				_, _ = io.WriteString(writer, `{"error":{"message":"connection interrupted","code":2}}`)
				return
			}
			assert.Equal(t, "4", request.Header.Get("offset"))
			assert.Equal(t, "efgh", string(body))
			_, _ = io.WriteString(writer, `{"success":true}`)
		case "/v26.0/video-1":
			_, _ = io.WriteString(writer, `{"status":{"uploading_phase":{"`+legacyTransferredKey+`":4}}}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(server.Close)
	client, err := api.New(api.Options{BaseURL: server.URL, UploadURL: server.URL, Token: "page-token", HTTPClient: server.Client(), RequestsPS: 100000})
	require.NoError(t, err)
	command := &cobra.Command{}
	command.SetContext(context.Background())
	result, err := uploadPageReel(command, client, "video-1", filePath, "")
	require.NoError(t, err)
	assert.Equal(t, true, result.(map[string]any)["success"])
	assert.Equal(t, 2, uploads)
}

func TestPageVideoUsesServerChunkOffsetsAndMultipartThumbnail(t *testing.T) {
	videoPath := filepath.Join(t.TempDir(), "video.mp4")
	thumbnailPath := filepath.Join(t.TempDir(), "thumbnail.jpg")
	require.NoError(t, os.WriteFile(videoPath, []byte("abcdefgh"), 0o600))
	require.NoError(t, os.WriteFile(thumbnailPath, []byte("jpeg-data"), 0o600))
	var transfers int
	test, serverURL := newCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "Bearer page-token", request.Header.Get("Authorization"))
		switch {
		case request.URL.Path == "/v26.0/page-1/videos" && request.URL.Query().Get("upload_phase") == "start":
			_, _ = io.WriteString(writer, `{"upload_session_id":"session-1","video_id":"video-1","start_offset":"0","end_offset":"4"}`)
		case request.URL.Path == "/v26.0/page-1/videos" && request.URL.Query().Get("upload_phase") == "transfer":
			transfers++
			require.NoError(t, request.ParseMultipartForm(1024))
			part, _, err := request.FormFile("video_file_chunk")
			require.NoError(t, err)
			chunk, err := io.ReadAll(part)
			require.NoError(t, err)
			_ = part.Close()
			assert.Len(t, chunk, 4)
			if transfers == 1 {
				assert.Equal(t, "abcd", string(chunk))
				_, _ = io.WriteString(writer, `{"start_offset":"4","end_offset":"8"}`)
			} else {
				assert.Equal(t, "efgh", string(chunk))
				_, _ = io.WriteString(writer, `{"start_offset":"8","end_offset":"8"}`)
			}
		case request.URL.Path == "/v26.0/page-1/videos" && request.URL.Query().Get("upload_phase") == "finish":
			_, _ = io.WriteString(writer, `{"success":true,"id":"video-1"}`)
		case request.URL.Path == "/v26.0/video-1/thumbnails":
			reader, err := request.MultipartReader()
			require.NoError(t, err)
			fields := multipartValues(t, reader)
			assert.Equal(t, "jpeg-data", fields["source"])
			assert.Equal(t, "true", fields["is_preferred"])
			_, _ = io.WriteString(writer, `{"success":true}`)
		default:
			http.NotFound(writer, request)
		}
	})
	test.store.values["default"] = auth.Credential{Token: "user-token", PageToken: "page-token"}
	require.NoError(t, test.run("--base-url", serverURL, "--page-id", "page-1", "pages", "videos", "publish", "--file", videoPath, "--thumbnail-file", thumbnailPath, "-o", "json"))
	assert.Equal(t, 2, transfers)
}

func TestOutOfRangeUploadOffsetsCannotCompleteOrPublish(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "video.mp4")
	require.NoError(t, os.WriteFile(filePath, []byte("abcdefgh"), 0o600))
	for _, testCase := range []struct {
		name       string
		uploadPath string
		statusPath string
		upload     func(*cobra.Command, *api.Client, string, string, string) (any, error)
	}{
		{name: "instagram", uploadPath: "/ig-api-upload/v26.0/media-1", statusPath: "/v26.0/media-1", upload: uploadInstagramContainer},
		{name: "page reel", uploadPath: "/video-upload/v26.0/media-1", statusPath: "/v26.0/media-1", upload: uploadPageReel},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case testCase.uploadPath:
					writer.WriteHeader(http.StatusBadRequest)
					_, _ = io.WriteString(writer, `{"error":{"message":"interrupted","code":100}}`)
				case testCase.statusPath:
					_, _ = io.WriteString(writer, `{"status":{"uploading_phase":{"bytes_transferred":9}}}`)
				default:
					http.NotFound(writer, request)
				}
			}))
			t.Cleanup(server.Close)
			client, err := api.New(api.Options{BaseURL: server.URL, UploadURL: server.URL, Token: "token", HTTPClient: server.Client(), RequestsPS: 100000})
			require.NoError(t, err)
			command := &cobra.Command{}
			command.SetContext(context.Background())
			result, err := testCase.upload(command, client, "media-1", filePath, "")
			require.ErrorContains(t, err, "beyond file size")
			assert.Nil(t, result)
		})
	}

	finished := false
	commandTest, serverURL := newCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Query().Get("upload_phase") {
		case "start":
			_, _ = io.WriteString(writer, `{"upload_session_id":"session-1","video_id":"video-1","start_offset":"0","end_offset":"4"}`)
		case "transfer":
			_, _ = io.WriteString(writer, `{"start_offset":"9","end_offset":"9"}`)
		case "finish":
			finished = true
			_, _ = io.WriteString(writer, `{"success":true}`)
		default:
			http.NotFound(writer, request)
		}
	})
	commandTest.store.values["default"] = auth.Credential{Token: "user-token", PageToken: "page-token"}
	err := commandTest.run("--base-url", serverURL, "--page-id", "page-1", "pages", "videos", "publish", "--file", filePath, "-o", "json")
	require.ErrorContains(t, err, "invalid upload offsets")
	assert.False(t, finished)
}

func TestPublishedPageVideoReportsThumbnailPartialFailure(t *testing.T) {
	videoPath := filepath.Join(t.TempDir(), "video.mp4")
	thumbnailPath := filepath.Join(t.TempDir(), "thumbnail.jpg")
	require.NoError(t, os.WriteFile(videoPath, []byte("data"), 0o600))
	require.NoError(t, os.WriteFile(thumbnailPath, []byte("image"), 0o600))
	test, serverURL := newCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/v26.0/page-1/videos" && request.URL.Query().Get("upload_phase") == "start":
			_, _ = io.WriteString(writer, `{"upload_session_id":"session-1","video_id":"video-1","start_offset":"0","end_offset":"4"}`)
		case request.URL.Path == "/v26.0/page-1/videos" && request.URL.Query().Get("upload_phase") == "transfer":
			_, _ = io.WriteString(writer, `{"start_offset":"4","end_offset":"4"}`)
		case request.URL.Path == "/v26.0/page-1/videos" && request.URL.Query().Get("upload_phase") == "finish":
			_, _ = io.WriteString(writer, `{"success":true,"id":"video-1"}`)
		case request.URL.Path == "/v26.0/video-1/thumbnails":
			writer.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(writer, `{"error":{"message":"thumbnail denied","code":10,"fbtrace_id":"thumb-trace"}}`)
		default:
			http.NotFound(writer, request)
		}
	})
	test.store.values["default"] = auth.Credential{Token: "user-token", PageToken: "page-token"}
	err := test.run("--base-url", serverURL, "--page-id", "page-1", "pages", "videos", "publish", "--file", videoPath, "--thumbnail-file", thumbnailPath, "-o", "json")
	require.Error(t, err)
	assert.Equal(t, 2, ExitCode(err))
	assert.Contains(t, test.output.String(), `"id": "video-1"`)
	assert.Contains(t, test.output.String(), `"thumbnail_status": "failed"`)
}

func TestPublishedPageReelReportsProcessingPartialFailure(t *testing.T) {
	videoPath := filepath.Join(t.TempDir(), "reel.mp4")
	require.NoError(t, os.WriteFile(videoPath, []byte("data"), 0o600))
	test, serverURL := newCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/v26.0/page-1/video_reels" && request.Method == http.MethodPost:
			body, err := io.ReadAll(request.Body)
			require.NoError(t, err)
			if strings.Contains(string(body), `"upload_phase":"start"`) {
				_, _ = io.WriteString(writer, `{"video_id":"video-1"}`)
			} else {
				_, _ = io.WriteString(writer, `{"success":true,"id":"video-1"}`)
			}
		case request.URL.Path == "/video-upload/v26.0/video-1":
			_, _ = io.WriteString(writer, `{"success":true}`)
		case request.URL.Path == "/v26.0/video-1":
			_, _ = io.WriteString(writer, `{"id":"video-1","status":{"processing_phase":{"status":"ERROR"}}}`)
		default:
			http.NotFound(writer, request)
		}
	})
	test.store.values["default"] = auth.Credential{Token: "user-token", PageToken: "page-token"}
	err := test.run("--base-url", serverURL, "--upload-url", serverURL, "--page-id", "page-1", "pages", "reels", "publish", "--video", videoPath, "--poll-interval", "1ms", "-o", "json")
	require.Error(t, err)
	assert.Equal(t, 2, ExitCode(err))
	assert.Contains(t, test.output.String(), `"id": "video-1"`)
	assert.Contains(t, test.output.String(), `"processing_status": "failed"`)
}

func TestOutputFailuresCannotHideOrPrecedePublicationState(t *testing.T) {
	t.Run("static validation precedes Graph request", func(t *testing.T) {
		calls := 0
		test, serverURL := newCommandTest(t, func(writer http.ResponseWriter, _ *http.Request) {
			calls++
			_, _ = io.WriteString(writer, `{"id":"post-1"}`)
		})
		test.store.values["default"] = auth.Credential{Token: "user-token", PageToken: "page-token"}

		err := test.run("--base-url", serverURL, "--page-id", "page-1", "-o", "invalid", "pages", "posts", "create", "--message", "hello")
		require.ErrorContains(t, err, "unsupported output format")
		assert.Zero(t, calls)
	})

	t.Run("runtime jq failure preserves partial publication", func(t *testing.T) {
		videoPath := filepath.Join(t.TempDir(), "reel.mp4")
		require.NoError(t, os.WriteFile(videoPath, []byte("data"), 0o600))
		test, serverURL := newCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
			switch {
			case request.URL.Path == "/v26.0/page-1/video_reels" && request.Method == http.MethodPost:
				body, err := io.ReadAll(request.Body)
				require.NoError(t, err)
				if strings.Contains(string(body), `"upload_phase":"start"`) {
					_, _ = io.WriteString(writer, `{"video_id":"video-1"}`)
				} else {
					_, _ = io.WriteString(writer, `{"success":true,"id":"video-1"}`)
				}
			case request.URL.Path == "/video-upload/v26.0/video-1":
				_, _ = io.WriteString(writer, `{"success":true}`)
			case request.URL.Path == "/v26.0/video-1":
				_, _ = io.WriteString(writer, `{"id":"video-1","status":{"processing_phase":{"status":"ERROR"}}}`)
			default:
				http.NotFound(writer, request)
			}
		})
		test.store.values["default"] = auth.Credential{Token: "user-token", PageToken: "page-token"}

		err := test.run("--base-url", serverURL, "--upload-url", serverURL, "--page-id", "page-1", "pages", "reels", "publish", "--video", videoPath, "--poll-interval", "1ms", "-o", "json", "--jq", ".id | tonumber")
		require.Error(t, err)
		assert.Equal(t, 2, ExitCode(err))
		assert.Contains(t, err.Error(), "emitted unfiltered JSON")
		assert.Contains(t, test.output.String(), `"id": "video-1"`)
		assert.Contains(t, test.output.String(), `"processing_status": "failed"`)
	})
}

func TestWhatsAppMediaSendsRequiredMultipartFields(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "image.jpg")
	require.NoError(t, os.WriteFile(filePath, []byte("image-data"), 0o600))
	test, serverURL := newCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		reader, err := request.MultipartReader()
		require.NoError(t, err)
		fields := multipartValues(t, reader)
		assert.Equal(t, "whatsapp", fields["messaging_product"])
		assert.Equal(t, "image/jpeg", fields["type"])
		assert.Equal(t, "image-data", fields["file"])
		_, _ = io.WriteString(writer, `{"id":"media-1"}`)
	})
	require.NoError(t, test.run("--base-url", serverURL, "--phone-id", "phone-1", "whatsapp", "media", "upload", "--file", filePath, "-o", "json"))
}

func multipartValues(t *testing.T, reader *multipart.Reader) map[string]string {
	t.Helper()
	values := map[string]string{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		body, err := io.ReadAll(part)
		require.NoError(t, err)
		values[part.FormName()] = string(body)
		_ = part.Close()
	}
	return values
}

func TestPageReelFinishOmitsUndocumentedThumbOffset(t *testing.T) {
	body := pageReelFinishBody("video-1", "Title", "Description", 12345)
	assert.NotContains(t, string(body), "thumb_offset")
	assert.Contains(t, string(body), `"video_state":"SCHEDULED"`)
	assert.Contains(t, string(body), strconv.FormatInt(12345, 10))
}

func TestPublishedContainerIsNotRepublished(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, `{"id":"container-1","status_code":"PUBLISHED"}`)
	}))
	t.Cleanup(server.Close)
	client, err := api.New(api.Options{BaseURL: server.URL, UploadURL: server.URL, Token: "token", HTTPClient: server.Client(), RequestsPS: 100000})
	require.NoError(t, err)
	command := &cobra.Command{}
	command.SetContext(context.Background())
	_, err = waitForInstagramContainer(command, &globalOptions{}, client, "container-1", 0, 1)
	assert.ErrorContains(t, err, "refusing a duplicate publish")
}

func TestWhatsAppRejectsGenericBinaryType(t *testing.T) {
	_, err := uploadWhatsAppMedia(&cobra.Command{}, nil, "phone", "file", "application/octet-stream")
	assert.ErrorContains(t, err, "not a supported WhatsApp media type")
}

func TestPartialFailureExitCode(t *testing.T) {
	err := &partialFailureError{message: "partial"}
	assert.Equal(t, 2, ExitCode(err))
	assert.Equal(t, 1, ExitCode(io.EOF))
}

func TestUploadOffsetAcceptsBothGraphSpellings(t *testing.T) {
	assert.Equal(t, int64(7), uploadOffset(map[string]any{"video_status": map[string]any{"uploading_phase": map[string]any{"bytes_transferred": float64(7)}}}))
	assert.Equal(t, int64(8), uploadOffset(map[string]any{"status": map[string]any{"uploading_phase": map[string]any{legacyTransferredKey: "8"}}}))
}

func TestDryRunFileDescriptionDoesNotContainContent(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "video.mp4")
	require.NoError(t, os.WriteFile(filePath, []byte("sensitive-media-bytes"), 0o600))
	request, _, err := rawUploadRequest(context.Background(), "video-upload/id", filePath, 0)
	require.NoError(t, err)
	var output bytes.Buffer
	client, err := api.New(api.Options{BaseURL: "http://127.0.0.1:1", UploadURL: "http://127.0.0.1:1", Token: "token", DryRun: true, Writer: &output})
	require.NoError(t, err)
	_, err = client.Do(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, strings.Contains(output.String(), "@"+filePath))
	assert.NotContains(t, output.String(), "sensitive-media-bytes")
}

func TestDryRunKeepsMachineOutputSeparateFromCurl(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := NewRootCmd(Dependencies{
		Out: &stdout, Err: &stderr, In: bytes.NewBuffer(nil),
		ConfigPath: filepath.Join(t.TempDir(), "config.yaml"),
		Store:      &memoryStore{values: map[string]auth.Credential{}},
	})
	root.SetArgs([]string{"--base-url", "http://127.0.0.1:1", "--page-id", "page-1", "--dry-run", "-o", "json", "pages", "posts", "create", "--message", "Preview"})
	require.NoError(t, root.Execute())
	assert.JSONEq(t, `{"dry_run":true}`, stdout.String())
	assert.NotContains(t, stdout.String(), "curl")
	assert.Contains(t, stderr.String(), "curl -X")
}

func TestDryRunRedactsCredentialsEmbeddedInOrdinaryValues(t *testing.T) {
	test, _ := newCommandTest(t, func(http.ResponseWriter, *http.Request) {})
	credential := test.store.values["default"]
	credential.PageToken = "page-secret-token"
	test.store.values["default"] = credential
	common := []string{
		"--base-url", "http://127.0.0.1:1", "--upload-url", "http://127.0.0.1:1",
		"--page-id", "page-1", "--instagram-id", "ig-1", "--waba-id", "waba-1", "--phone-id", "phone-1", "--dry-run", "-o", "json",
	}
	mediaPath := filepath.Join(t.TempDir(), "image.jpg")
	require.NoError(t, os.WriteFile(mediaPath, []byte("fake-image"), 0o600))
	cases := [][]string{
		{"pages", "videos", "finish", "--session-id", "session-1", "--title", "test-secret"},
		{"whatsapp", "send", "text", "--to", "15551234567", "--message", "page-secret-token"},
		{"whatsapp", "media", "upload", "--file", mediaPath, "--content-type", "test-secret"},
		{"instagram", "media", "get", "test-secret"},
		{"whatsapp", "templates", "delete", "--name", "test-secret"},
		{"api", "POST", "object", "--data", `"test-secret page-secret-token"`},
	}
	for _, arguments := range cases {
		require.NoError(t, test.run(append(common, arguments...)...))
		assert.NotContains(t, test.output.String(), "test-secret")
		assert.NotContains(t, test.output.String(), "page-secret-token")
		assert.Contains(t, test.output.String(), "redacted")
	}
	err := test.run(append(common, "instagram", "containers", "create", "--type", "test-secret")...)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "test-secret")
	assert.Contains(t, err.Error(), "redacted")
	require.NoError(t, test.run(append(common, "--jq", `"test-secret"`, "pages", "posts", "create", "--message", "safe")...))
	assert.NotContains(t, test.output.String(), "test-secret")
	assert.Contains(t, test.output.String(), "redacted")
	require.NoError(t, test.run(append(common, "--columns", "test-secret,page-secret-token", "-o", "csv", "pages", "posts", "create", "--message", "safe")...))
	assert.NotContains(t, test.output.String(), "test-secret")
	assert.NotContains(t, test.output.String(), "page-secret-token")
	assert.Contains(t, test.output.String(), "redacted")

	err = test.run("--waba-id", "waba-1", "whatsapp", "templates", "delete", "--name", "test-secret")
	require.Error(t, err)
	assert.NotContains(t, test.output.String(), "test-secret")
	assert.Contains(t, test.output.String(), "redacted")
}

func TestAuthLoginRedactsStoredSecretsFromSuccessfulIdentity(t *testing.T) {
	const (
		storedPage      = "stored-page-credential"
		environmentPage = "environment-page-credential"
	)
	t.Setenv("METACTL_PAGE_TOKEN", environmentPage)
	test, serverURL := newCommandTest(t, func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "Bearer fresh-user-token", request.Header.Get("Authorization"))
		_, _ = io.WriteString(writer, `{"id":"user-1","name":"echo test-secret, `+storedPage+`, and `+environmentPage+`"}`)
	})
	credential := test.store.values["default"]
	credential.PageToken = storedPage
	test.store.values["default"] = credential
	require.NoError(t, test.run("--base-url", serverURL, "auth", "login", "--token", "fresh-user-token", "-o", "json"))
	assert.NotContains(t, test.output.String(), "test-secret")
	assert.NotContains(t, test.output.String(), storedPage)
	assert.NotContains(t, test.output.String(), environmentPage)
	assert.Contains(t, test.output.String(), "redacted")
}

func TestUploadBodiesStreamFromDiskInsteadOfSnapshottingFiles(t *testing.T) {
	const size = 2 << 20
	filePath := filepath.Join(t.TempDir(), "large.mp4")
	require.NoError(t, os.WriteFile(filePath, make([]byte, size), 0o600))

	rawFactory, length, err := fileBody(context.Background(), filePath, 0, 0)
	require.NoError(t, err)
	assert.EqualValues(t, size, length)
	raw, err := rawFactory()
	require.NoError(t, err)
	require.NoError(t, writeLastByte(filePath, 'R'))
	rawBytes, err := io.ReadAll(raw)
	require.NoError(t, err)
	require.NoError(t, raw.Close())
	assert.Equal(t, byte('R'), rawBytes[len(rawBytes)-1])

	multipartFactory, contentType, err := multipartFileBody(context.Background(), filePath, "video_file_chunk", "application/octet-stream", nil, 0, size)
	require.NoError(t, err)
	multipartBody, err := multipartFactory()
	require.NoError(t, err)
	require.NoError(t, writeLastByte(filePath, 'M'))
	_, parameters, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	fields := multipartValues(t, multipart.NewReader(multipartBody, parameters["boundary"]))
	require.NoError(t, multipartBody.Close())
	assert.Equal(t, byte('M'), fields["video_file_chunk"][len(fields["video_file_chunk"])-1])
}

func writeLastByte(filePath string, value byte) error {
	file, err := os.OpenFile(filePath, os.O_WRONLY, 0) // #nosec G304 -- test path is isolated in t.TempDir
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	_, err = file.WriteAt([]byte{value}, info.Size()-1)
	return err
}
