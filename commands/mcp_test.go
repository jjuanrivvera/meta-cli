package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/njayp/ophis"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jjuanrivvera/meta-cli/internal/auth"
)

func TestMCPExcludesSetupCommands(t *testing.T) {
	root := NewRootCmd(Dependencies{Store: &memoryStore{values: map[string]auth.Credential{}}, ConfigPath: filepath.Join(t.TempDir(), "config.yaml")})
	for _, name := range mcpExcludedGroups {
		command, _, err := root.Find([]string{name})
		if err != nil {
			continue
		}
		assert.False(t, mcpCommandSelector(command), name)
	}
	command, _, err := root.Find([]string{"pages", "posts", "list"})
	require.NoError(t, err)
	assert.True(t, mcpCommandSelector(command))
}

func TestMCPFileConfinementRejectsEscapesAndSymlinks(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("META_KEYRING_BACKEND", "file")
	t.Setenv("META_KEYRING_PASSWORD", "test")
	inside := filepath.Join(root, "inside.mp4")
	require.NoError(t, os.WriteFile(inside, []byte("video"), 0o600))
	outside := filepath.Join(t.TempDir(), "outside.mp4")
	require.NoError(t, os.WriteFile(outside, []byte("video"), 0o600))
	symlink := filepath.Join(root, "escape.mp4")
	require.NoError(t, os.Symlink(outside, symlink))
	t.Setenv("META_MCP_ROOT", root)

	called := false
	var received ophis.ToolInput
	next := func(_ context.Context, _ *mcp.CallToolRequest, input ophis.ToolInput) (*mcp.CallToolResult, ophis.ToolOutput, error) {
		called = true
		received = input
		return nil, ophis.ToolOutput{ExitCode: 0}, nil
	}
	_, _, err := confineMCPFiles(context.Background(), nil, ophis.ToolInput{Flags: map[string]any{"file": inside}}, next)
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, inside, received.Flags["file"])

	workingDirectory := t.TempDir()
	originalWorkingDirectory, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(workingDirectory))
	t.Cleanup(func() { _ = os.Chdir(originalWorkingDirectory) })
	_, _, err = confineMCPFiles(context.Background(), nil, ophis.ToolInput{Flags: map[string]any{"file": "inside.mp4"}}, next)
	require.NoError(t, err)
	assert.Equal(t, inside, received.Flags["file"])
	assert.Equal(t, root, received.Flags["mcp-root-internal"])

	var output bytes.Buffer
	cli := NewRootCmd(Dependencies{Out: &output, Err: &output, In: bytes.NewBuffer(nil), Store: &memoryStore{values: map[string]auth.Credential{}}, ConfigPath: filepath.Join(t.TempDir(), "config.yaml")})
	cli.SetArgs([]string{"--mcp-root-internal", root, "--base-url", "http://127.0.0.1:1", "--upload-url", "http://127.0.0.1:1", "--dry-run", "instagram", "containers", "upload", "container-1", "--file", "inside.mp4"})
	require.NoError(t, cli.Execute())
	assert.Contains(t, output.String(), "@"+inside)

	for _, selected := range []string{outside, symlink, "-"} {
		called = false
		_, _, err = confineMCPFiles(context.Background(), nil, ophis.ToolInput{Flags: map[string]any{"file": selected}}, next)
		assert.Error(t, err, selected)
		assert.False(t, called, selected)
	}

	_, _, err = confineMCPFiles(context.Background(), nil, ophis.ToolInput{Flags: map[string]any{"video": "https://cdn.example/video.mp4"}}, next)
	require.NoError(t, err)
	_, _, err = confineMCPFiles(context.Background(), nil, ophis.ToolInput{Flags: map[string]any{"published": false}}, next)
	require.NoError(t, err)
	assert.NotContains(t, received.Flags, "published")
	assert.Equal(t, true, received.Flags["published=false"])
	_, _, err = confineMCPFiles(context.Background(), nil, ophis.ToolInput{Args: []string{"object-id", "--yes"}, Flags: map[string]any{}}, next)
	assert.ErrorContains(t, err, "may not inject command flags")
}

func TestMCPConfinedOpenRejectsFileReplacedAfterValidation(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "inside.mp4")
	require.NoError(t, os.WriteFile(inside, []byte("inside"), 0o600))
	outside := filepath.Join(t.TempDir(), "outside.mp4")
	require.NoError(t, os.WriteFile(outside, []byte("outside"), 0o600))
	resolved, err := pathWithinRoot(root, inside)
	require.NoError(t, err)
	directory, err := os.Open(root) // #nosec G304 -- test-owned temporary directory
	require.NoError(t, err)
	t.Cleanup(func() { _ = directory.Close() })

	require.NoError(t, os.Remove(inside))
	require.NoError(t, os.Symlink(outside, inside))
	ctx := context.WithValue(context.Background(), mcpConfinementContextKey{}, &mcpFileConfinement{
		root: root, directory: directory,
	})
	_, _, err = fileBody(ctx, resolved, 0, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "confined MCP file")
	command := &cobra.Command{}
	command.SetContext(ctx)
	_, err = (&writeOptions{file: resolved}).body(command, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "confined MCP file")
}
