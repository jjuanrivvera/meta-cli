package config

import (
	"errors"
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
	value, err = Load("")
	require.NoError(t, err)
	assert.Empty(t, value.Accounts)
	require.NoError(t, Save("", &Config{Accounts: map[string]Account{"default": {}}}))
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", t.TempDir())
	path, err = Path()
	require.NoError(t, err)
	assert.Contains(t, path, filepath.Join(".config", "metactl", "config.yaml"))
}

func TestValidationAndPrecedence(t *testing.T) {
	for _, name := range []string{"", "../bad", "a/b", "a:b"} {
		assert.Error(t, ValidateAccount(name, Account{}))
	}
	assert.Error(t, ValidateAccount("ok", Account{BaseURL: "http://example.com"}))
	assert.Error(t, ValidateAccount("ok", Account{UploadURL: "://bad"}))
	assert.NoError(t, ValidateAccount("ok", Account{BaseURL: "http://127.0.0.1:8080"}))
	assert.NoError(t, ValidateAccount("ok", Account{BaseURL: "http://localhost:8080"}))
	assert.Equal(t, "flag", FirstNonEmpty("", "flag", "env"))
	assert.Empty(t, FirstNonEmpty("", " "))
}

func TestLoadInvalidYAML(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("accounts: ["), 0o600))
	_, err := Load(configPath)
	assert.Error(t, err)
	directory := t.TempDir()
	_, err = Load(directory)
	assert.ErrorContains(t, err, "read config")
}

func TestConfigAdditionalErrorBranches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "minimal.yaml")
	require.NoError(t, os.WriteFile(path, []byte("current: work\n"), 0o600))
	value, err := Load(path)
	require.NoError(t, err)
	assert.NotNil(t, value.Accounts)
	assert.NotNil(t, value.Aliases)

	invalid := &Config{Accounts: map[string]Account{"../bad": {}}}
	assert.Error(t, Save(filepath.Join(t.TempDir(), "config.yaml"), invalid))
	parentFile := filepath.Join(t.TempDir(), "parent")
	require.NoError(t, os.WriteFile(parentFile, []byte("x"), 0o600))
	assert.ErrorContains(t, Save(filepath.Join(parentFile, "config.yaml"), &Config{}), "create config directory")
	destinationDirectory := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.Mkdir(destinationDirectory, 0o700))
	assert.ErrorContains(t, Save(destinationDirectory, &Config{}), "replace config")

	originalCreateTemp := createTemp
	createTemp = func(string, string) (*os.File, error) { return nil, errors.New("denied") }
	t.Cleanup(func() { createTemp = originalCreateTemp })
	assert.ErrorContains(t, Save(filepath.Join(t.TempDir(), "config.yaml"), &Config{}), "create temporary config")

	originalUserConfigDir := userConfigDir
	userConfigDir = func() (string, error) { return "", errors.New("no config directory") }
	t.Cleanup(func() { userConfigDir = originalUserConfigDir })
	t.Setenv("XDG_CONFIG_HOME", "")
	_, err = Path()
	assert.ErrorContains(t, err, "resolve user config directory")
}
