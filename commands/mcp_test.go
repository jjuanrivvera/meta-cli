package commands

import (
	"path/filepath"
	"testing"

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
