package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jjuanrivvera/meta-cli/internal/auth"
	"github.com/jjuanrivvera/meta-cli/internal/config"
)

func TestClassifyCommandsAndAliases(t *testing.T) {
	root := NewRootCmd(Dependencies{Store: &memoryStore{values: map[string]auth.Credential{}}, ConfigPath: filepath.Join(t.TempDir(), "config.yaml")})
	commands := classifyCommands(root)
	kinds := map[string]string{}
	for _, command := range commands {
		kinds[command.Path] = command.Kind
	}
	assert.Equal(t, guardRead, kinds["pages posts list"])
	assert.Equal(t, guardApproval, kinds["pages posts create"])
	assert.Equal(t, guardIrreversible, kinds["pages posts delete"])
	assert.Equal(t, guardIrreversible, kinds["pages post delete"])
	assert.Equal(t, guardIrreversible, kinds["wa templates delete"])
	assert.Equal(t, guardDynamic, kinds["api"])
	assert.NotContains(t, kinds, "__surface list")
	assert.NotContains(t, kinds, "__surface resolve")
	assert.NotContains(t, kinds, "help")
}

func TestHostConfigSchemas(t *testing.T) {
	input := guardInput{Binary: "meta", Commands: []guardCommand{{Path: "pages posts list", Tool: "meta_pages_posts_list", Kind: guardRead}, {Path: "pages posts create", Tool: "meta_pages_posts_create", Kind: guardApproval}, {Path: "pages posts delete", Tool: "meta_pages_posts_delete", Kind: guardIrreversible}}}
	files, err := renderHostConfig("claude-code", input)
	require.NoError(t, err)
	assert.Len(t, files, 2)
	assert.Contains(t, files[1].Content, "PreToolUse")
	assert.Contains(t, files[0].Content, "([^[:space:]]*/)?")
	assert.Contains(t, files[0].Content, "tr '\\n{}:,'")
	var settings map[string]any
	require.NoError(t, json.Unmarshal([]byte(files[1].Content), &settings))
	assert.Contains(t, settings, "permissions")

	files, err = renderHostConfig("codex", input)
	require.NoError(t, err)
	assert.Contains(t, files[0].Content, "sandbox_mode")
	assert.NotContains(t, files[0].Content, "[sandbox]")

	files, err = renderHostConfig("opencode", input)
	require.NoError(t, err)
	var openCode map[string]any
	require.NoError(t, json.Unmarshal([]byte(files[0].Content), &openCode))
	assert.Contains(t, openCode, "permission")
	assert.NotContains(t, openCode, "permissions")
	_, err = renderHostConfig("unknown", input)
	assert.Error(t, err)
}

func TestWriteGuardFilesNeverOverwrites(t *testing.T) {
	directory := t.TempDir()
	command := &cobra.Command{}
	files := []guardFile{{Path: ".guard/hook.sh", Content: "#!/bin/sh\n", Executable: true}}
	require.NoError(t, writeGuardFiles(command, directory, files))
	info, err := os.Stat(filepath.Join(directory, ".guard", "hook.sh"))
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&0o100)
	assert.Error(t, writeGuardFiles(command, directory, files))
}

func TestUserAliasClassificationAndGuardCommand(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, config.Save(configPath, &config.Config{Aliases: map[string]string{
		"read-posts": "pages posts list --all",
		"broken":     "does-not-exist",
	}}))
	root := NewRootCmd(Dependencies{Store: &memoryStore{values: map[string]auth.Credential{}}, ConfigPath: configPath})
	classified := addUserAliases(classifyCommands(root), configPath)
	var found bool
	for _, item := range classified {
		if item.Path == "read-posts" {
			found = true
			assert.Equal(t, guardRead, item.Kind)
		}
	}
	assert.True(t, found)

	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"agent", "guard", "--host", "codex"})
	require.NoError(t, root.Execute())
	assert.Contains(t, output.String(), "sandbox_mode")

	output.Reset()
	root = NewRootCmd(Dependencies{Out: &output, Err: &output, Store: &memoryStore{values: map[string]auth.Credential{}}, ConfigPath: configPath})
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"agent", "classify", "-o", "json"})
	require.NoError(t, root.Execute())
	assert.Contains(t, output.String(), "pages posts delete")
}
