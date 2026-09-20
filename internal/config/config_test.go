package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveLoadAndPermissions(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "nested", "config.yaml")
	want := &Config{Current: "work", Accounts: map[string]Account{"work": {BaseURL: "https://graph.facebook.com", GraphVersion: "v26.0", PageID: "123"}}, Aliases: map[string]string{}}
	require.NoError(t, Save(configPath, want))
	got, err := Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, want, got)
	if runtime.GOOS != "windows" {
		info, err := os.Stat(configPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
}

func TestLoadMissingAndPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.yaml")
	value, err := Load(missing)
	require.NoError(t, err)
	assert.Empty(t, value.Accounts)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path, err := Path()
	require.NoError(t, err)
	assert.Contains(t, path, filepath.Join("metactl", "config.yaml"))
}

func TestValidationAndPrecedence(t *testing.T) {
	for _, name := range []string{"", "../bad", "a/b", "a:b"} {
		assert.Error(t, ValidateAccount(name, Account{}))
	}
	assert.Error(t, ValidateAccount("ok", Account{BaseURL: "http://example.com"}))
	assert.NoError(t, ValidateAccount("ok", Account{BaseURL: "http://127.0.0.1:8080"}))
	assert.Equal(t, "flag", FirstNonEmpty("", "flag", "env"))
}

func TestLoadInvalidYAML(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("accounts: ["), 0o600))
	_, err := Load(configPath)
	assert.Error(t, err)
}
