package commands

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jjuanrivvera/meta-cli/internal/api"
)

func TestInstagramContainerBodyVariants(t *testing.T) {
	tests := []struct {
		mediaType string
		url       string
		children  string
		wantField string
	}{
		{"image", "https://example.test/image.jpg", "", "image_url"},
		{"video", "https://example.test/video.mp4", "", "video_url"},
		{"reel", "", "", "upload_type"},
		{"story", "https://example.test/story.mp4", "", "media_type"},
		{"carousel-item", "https://example.test/item.mp4", "", "is_carousel_item"},
		{"carousel-item", "https://example.test/item.jpg", "", "image_url"},
		{"carousel", "", "one,two", "children"},
	}
	for _, test := range tests {
		body, err := instagramContainerBody(test.mediaType, test.url, "caption", test.children, "https://example.test/cover.jpg", 100, true)
		require.NoError(t, err)
		var value map[string]any
		require.NoError(t, json.Unmarshal(body, &value))
		assert.Contains(t, value, test.wantField)
	}
	_, err := instagramContainerBody("image", "", "", "", "", 0, false)
	assert.ErrorContains(t, err, "--url")
	_, err = instagramContainerBody("carousel", "", "", "", "", 0, false)
	assert.ErrorContains(t, err, "--children")
	_, err = instagramContainerBody("unknown", "", "", "", "", 0, false)
	assert.ErrorContains(t, err, "unsupported")
}

func TestWaitForInstagramContainerStates(t *testing.T) {
	for _, status := range []string{"FINISHED", "ERROR", "IN_PROGRESS"} {
		t.Run(status, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(writer, `{"status_code":"`+status+`"}`)
			}))
			defer server.Close()
			client, err := api.New(api.Options{BaseURL: server.URL, UploadURL: server.URL, Token: "token", HTTPClient: server.Client()})
			require.NoError(t, err)
			command := &cobra.Command{}
			command.SetContext(context.Background())
			_, err = waitForInstagramContainer(command, &globalOptions{}, client, "container", 0, 1)
			switch status {
			case "FINISHED":
				assert.NoError(t, err)
			case "ERROR":
				assert.ErrorContains(t, err, "entered ERROR")
			default:
				assert.ErrorContains(t, err, "did not finish")
			}
		})
	}
}
