package auth

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

func TestEncryptedFileStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.enc")
	store := NewFileStore(path, "correct horse battery staple")
	want := Credential{Token: "token-value", AppSecret: "app-secret"}
	require.NoError(t, store.Set("work", want))
	got, err := store.Get("work")
	require.NoError(t, err)
	assert.Equal(t, want, got)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "token-value")
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
	require.NoError(t, store.Delete("work"))
	_, err = store.Get("work")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestEncryptedFileStoreErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.enc")
	empty := NewFileStore(path, "")
	assert.Error(t, empty.Set("work", Credential{Token: "x"}))
	require.NoError(t, os.WriteFile(path, []byte("invalid"), 0o600))
	_, err := NewFileStore(path, "password").Get("work")
	assert.Error(t, err)
	assert.False(t, errors.Is(err, ErrNotFound))
	assert.Error(t, NewFileStore(path, "password").Set("work", Credential{Token: "x"}))
	assert.Error(t, NewFileStore(path, "password").Delete("work"))
}

func TestEncryptRejectsWrongPassword(t *testing.T) {
	raw, err := encrypt([]byte("secret"), "one")
	require.NoError(t, err)
	_, err = decrypt(raw, "two")
	assert.Error(t, err)
	_, err = decrypt([]byte("short"), "one")
	assert.Error(t, err)
	_, err = decrypt(append([]byte("MCTL1"), make([]byte, 16)...), "one")
	assert.Error(t, err)
}

func TestStoreSelectionAndHelpers(t *testing.T) {
	t.Setenv(backendEnv, "file")
	t.Setenv(passwordEnv, "password")
	configPath := filepath.Join(t.TempDir(), "nested", "config.yaml")
	store := NewStore(configPath)
	assert.Equal(t, "encrypted-file", store.Backend())
	path, err := credentialPath(configPath)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(filepath.Dir(configPath), encryptedFilename), path)
	assert.Equal(t, "account-work", key("work"))

	keyring := keyringStore{}
	assert.Equal(t, "os-keyring", keyring.Backend())
	assert.Equal(t, "encrypted-file", NewFileStore("path", "password").Backend())

	missing := NewFileStore(filepath.Join(t.TempDir(), "credentials.enc"), "password")
	_, err = missing.Get("missing")
	assert.ErrorIs(t, err, ErrNotFound)
	assert.ErrorIs(t, missing.Delete("missing"), ErrNotFound)
}

func TestDefaultFileStoreUsesConfigDirectory(t *testing.T) {
	configRoot := t.TempDir()
	workingDirectory := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv(backendEnv, "file")
	t.Setenv(passwordEnv, "correct horse battery staple")

	originalDirectory, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(workingDirectory))
	t.Cleanup(func() { require.NoError(t, os.Chdir(originalDirectory)) })

	store := NewStore("")
	require.NoError(t, store.Set("default", Credential{Token: "token-value"}))
	credentialFile := filepath.Join(configRoot, "meta", encryptedFilename)
	info, err := os.Stat(credentialFile)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
	_, err = os.Stat(filepath.Join(workingDirectory, encryptedFilename))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestOSKeyringBranches(t *testing.T) {
	originalGet, originalSet, originalDelete := keyringGet, keyringSet, keyringDelete
	t.Cleanup(func() { keyringGet, keyringSet, keyringDelete = originalGet, originalSet, originalDelete })
	store := keyringStore{}

	keyringGet = func(_, _ string) (string, error) { return `{"token":"value"}`, nil }
	credential, err := store.Get("work")
	require.NoError(t, err)
	assert.Equal(t, "value", credential.Token)
	keyringGet = func(_, _ string) (string, error) { return "", keyring.ErrNotFound }
	_, err = store.Get("work")
	assert.ErrorIs(t, err, ErrNotFound)
	keyringGet = func(_, _ string) (string, error) { return "", errors.New("backend") }
	_, err = store.Get("work")
	assert.ErrorContains(t, err, "read OS keyring")
	keyringGet = func(_, _ string) (string, error) { return "invalid", nil }
	_, err = store.Get("work")
	assert.ErrorContains(t, err, "decode credential")

	var stored string
	keyringSet = func(_, _, value string) error { stored = value; return nil }
	require.NoError(t, store.Set("work", Credential{Token: "value"}))
	assert.Contains(t, stored, "value")
	keyringSet = func(_, _, _ string) error { return errors.New("backend") }
	assert.ErrorContains(t, store.Set("work", Credential{}), "write OS keyring")

	keyringDelete = func(_, _ string) error { return keyring.ErrNotFound }
	assert.ErrorIs(t, store.Delete("work"), ErrNotFound)
	keyringDelete = func(_, _ string) error { return errors.New("backend") }
	assert.EqualError(t, store.Delete("work"), "backend")
	keyringDelete = func(_, _ string) error { return nil }
	require.NoError(t, store.Delete("work"))
}
