package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jjuanrivvera/meta-cli/internal/auth"
)

func TestResourceCommandsDryRun(t *testing.T) {
	mediaPath := filepath.Join(t.TempDir(), "media.mp4")
	require.NoError(t, os.WriteFile(mediaPath, []byte("fake-media"), 0o600))
	thumbnailPath := filepath.Join(t.TempDir(), "thumbnail.jpg")
	require.NoError(t, os.WriteFile(thumbnailPath, []byte("fake-image"), 0o600))
	test := &commandTest{
		store:      &memoryStore{values: map[string]auth.Credential{"default": {Token: "test-token", AppSecret: "test-secret"}}},
		configPath: filepath.Join(t.TempDir(), "config.yaml"),
	}
	common := []string{"--base-url", "http://127.0.0.1:18080", "--upload-url", "http://127.0.0.1:18081", "--page-id", "page-1", "--instagram-id", "ig-1", "--business-id", "business-1", "--waba-id", "waba-1", "--phone-id", "phone-1", "--app-id", "app-1", "--dry-run", "-o", "json"}
	cases := [][]string{
		{"auth", "debug"}, {"auth", "exchange"},
		{"instagram", "accounts", "list"},
		{"instagram", "containers", "create", "--type", "image", "--url", "https://cdn.example/image.jpg"},
		{"instagram", "containers", "create", "--type", "carousel", "--children", "one,two"},
		{"instagram", "containers", "status", "container-1"},
		{"instagram", "containers", "upload", "container-1", "--file", mediaPath},
		{"instagram", "containers", "publish", "container-1"},
		{"instagram", "publish", "reel", "--video", mediaPath, "--cover-url", "https://cdn.example/cover.jpg", "--first-comment", "First"},
		{"instagram", "media", "list", "--all"}, {"instagram", "media", "get", "media-1"},
		{"instagram", "comments", "list", "media-1"}, {"instagram", "comments", "create", "media-1", "--message", "Hello"},
		{"instagram", "comments", "replies", "comment-1"}, {"instagram", "comments", "reply", "comment-1", "--message", "Reply"},
		{"instagram", "comments", "hide", "comment-1"}, {"instagram", "comments", "delete", "comment-1"},
		{"instagram", "insights", "account", "--metrics", "reach"}, {"instagram", "insights", "media", "media-1", "--metrics", "reach"}, {"instagram", "limits", "publishing"},
		{"pages", "accounts", "list", "--all"},
		{"pages", "posts", "create", "--message", "Scheduled", "--published=false", "--scheduled-at", "1789900000"},
		{"pages", "posts", "list"}, {"pages", "posts", "get", "post-1"}, {"pages", "posts", "update", "post-1", "--message", "Updated"}, {"pages", "posts", "delete", "post-1"},
		{"pages", "photos", "create", "--url", "https://cdn.example/photo.jpg"}, {"pages", "photos", "list"}, {"pages", "photos", "get", "photo-1"},
		{"pages", "videos", "start", "--file-size", "10"}, {"pages", "videos", "upload", "session-1", "--file", mediaPath},
		{"pages", "videos", "status", "video-1"}, {"pages", "videos", "finish", "--session-id", "session-1"},
		{"pages", "videos", "thumbnail", "video-1", "--file", thumbnailPath},
		{"pages", "videos", "publish", "--file", mediaPath, "--thumbnail-file", thumbnailPath},
		{"pages", "reels", "start"}, {"pages", "reels", "upload", "video-1", "--file", mediaPath}, {"pages", "reels", "status", "video-1"},
		{"pages", "reels", "finish", "--video-id", "video-1"}, {"pages", "reels", "publish", "--video", mediaPath},
		{"pages", "comments", "list", "post-1"}, {"pages", "comments", "reply", "comment-1", "--message", "Reply"}, {"pages", "comments", "hide", "comment-1"}, {"pages", "comments", "delete", "comment-1"},
		{"pages", "insights", "page", "--metrics", "page_metric"}, {"pages", "insights", "post", "post-1", "--metrics", "post_metric"},
		{"whatsapp", "accounts", "list"}, {"whatsapp", "accounts", "get"}, {"whatsapp", "phones", "list"}, {"whatsapp", "phones", "get"},
		{"whatsapp", "templates", "list"}, {"whatsapp", "templates", "get", "template-1"},
		{"whatsapp", "templates", "create", "--name", "order_ready", "--components", `[{"type":"BODY","text":"Ready"}]`},
		{"whatsapp", "templates", "update", "template-1", "--category", "UTILITY"}, {"whatsapp", "templates", "delete", "--name", "order_ready"},
		{"whatsapp", "apps", "list"}, {"whatsapp", "apps", "subscribe"}, {"whatsapp", "apps", "unsubscribe"},
		{"whatsapp", "profile", "get"}, {"whatsapp", "profile", "update", "--about", "Example"},
		{"whatsapp", "media", "upload", "--file", mediaPath}, {"whatsapp", "media", "get", "media-1"}, {"whatsapp", "media", "delete", "media-1"},
		{"whatsapp", "send", "text", "--to", "15551234567", "--message", "Hello"},
		{"whatsapp", "send", "template", "--to", "15551234567", "--name", "order_ready"},
	}
	for _, args := range cases {
		name := strings.Join(args, " ")
		t.Run(name, func(t *testing.T) {
			err := test.run(append(common, args...)...)
			require.NoError(t, err)
			assert.Contains(t, test.output.String(), "curl -X")
			assert.NotContains(t, test.output.String(), "test-token")
		})
	}
}

func TestEveryAPICommandIsAnnotated(t *testing.T) {
	root := NewRootCmd(Dependencies{Store: &memoryStore{values: map[string]auth.Credential{}}, ConfigPath: filepath.Join(t.TempDir(), "config.yaml")})
	local := map[string]bool{"auth": true, "config": true, "init": true, "doctor": true, "completion": true, "alias": true, "version": true, "mcp": true, "agent": true, "update": true, "__surface": true}
	var walk func(*cobra.Command)
	walk = func(command *cobra.Command) {
		if command.HasSubCommands() {
			for _, child := range command.Commands() {
				walk(child)
			}
			return
		}
		top := command
		for top.Parent() != nil && top.Parent().Parent() != nil {
			top = top.Parent()
		}
		if local[top.Name()] {
			return
		}
		assert.NotEmpty(t, AnnotationKind(command), command.CommandPath())
	}
	walk(root)
}
