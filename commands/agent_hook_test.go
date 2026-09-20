package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratedHookExecutionBattery(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the generated hook targets POSIX shells")
	}
	input := guardInput{Binary: "metactl", Commands: []guardCommand{
		{Path: "pages posts list", Tool: "metactl_pages_posts_list", Kind: guardRead},
		{Path: "pages posts create", Tool: "metactl_pages_posts_create", Kind: guardApproval},
		{Path: "pages posts delete", Tool: "metactl_pages_posts_delete", Kind: guardIrreversible},
	}}
	hookPath := filepath.Join(t.TempDir(), "guard.sh")
	require.NoError(t, os.WriteFile(hookPath, []byte(buildPreToolUseHook(input)), 0o700))
	cases := []struct {
		name    string
		tool    string
		command string
		deny    bool
	}{
		{"blocked", "Bash", "metactl pages posts delete 1", true},
		{"path prefix", "Bash", "/usr/local/bin/metactl pages posts delete 1", true},
		{"glued separator", "Bash", "metactl pages posts delete;true", true},
		{"quote split", "Bash", `metactl pages posts de""lete 1`, true},
		{"backslash split", "Bash", `metactl pages posts de\lete 1`, true},
		{"newline", "Bash", "echo ok\nmetactl pages posts delete 1", true},
		{"semicolon chain", "Bash", "echo ok; metactl pages posts delete 1", true},
		{"pipe chain", "Bash", "echo ok | metactl pages posts delete 1", true},
		{"and chain", "Bash", "echo ok && metactl pages posts delete 1", true},
		{"env prefix", "Bash", "env X=1 metactl pages posts delete 1", true},
		{"read", "Bash", "metactl pages posts list", false},
		{"verb in argument", "Bash", "metactl pages posts list --filter message=delete", false},
		{"source filename", "Bash", "cat pages_posts_delete.go", false},
		{"raw read", "Bash", "metactl api GET posts/delete", false},
		{"raw write", "Bash", "metactl api POST posts", true},
		{"different binary", "Bash", "mymetactl pages posts delete 1", false},
		{"mcp blocked", "mcp__metactl__pages_posts_delete", "", true},
		{"mcp read", "mcp__metactl__pages_posts_list", "", false},
		{"mcp near miss", "mcp__metactl__pages_posts_delete2", "", false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			denied := runGuardHook(t, hookPath, test.tool, test.command, "")
			assert.Equal(t, test.deny, denied)
		})
	}

	strictPath := strictToolPath(t)
	assert.True(t, runGuardHook(t, hookPath, "Bash", "metactl pages posts delete 1", strictPath))
	assert.False(t, runGuardHook(t, hookPath, "Bash", "metactl pages posts list", strictPath))
}

func runGuardHook(t *testing.T, hookPath, tool, commandText, strictPath string) bool {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"tool_name": tool, "tool_input": map[string]any{"command": commandText}})
	require.NoError(t, err)
	command := exec.Command("/bin/bash", hookPath) // #nosec G204 -- the test executes its own generated hook at a controlled path
	command.Stdin = bytes.NewReader(payload)
	if strictPath != "" {
		command.Env = append(os.Environ(), "PATH="+strictPath)
	}
	err = command.Run()
	if err == nil {
		return false
	}
	var exitError *exec.ExitError
	if !assert.ErrorAs(t, err, &exitError) {
		return false
	}
	return exitError.ExitCode() == 2
}

func strictToolPath(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	for _, name := range []string{"cat", "tr", "grep"} {
		path, err := exec.LookPath(name)
		require.NoError(t, err)
		require.NoError(t, os.Symlink(path, filepath.Join(directory, name)))
	}
	return directory
}
