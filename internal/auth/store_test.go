package auth

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
}

func TestEncryptRejectsWrongPassword(t *testing.T) {
	raw, err := encrypt([]byte("secret"), "one")
	require.NoError(t, err)
	_, err = decrypt(raw, "two")
	assert.Error(t, err)
}

func TestStoreSelectionAndHelpers(t *testing.T) {
	t.Setenv(backendEnv, "file")
	t.Setenv(passwordEnv, "password")
	configPath := filepath.Join(t.TempDir(), "nested", "config.yaml")
	store := NewStore(configPath)
	assert.Equal(t, "encrypted-file", store.Backend())
	assert.Equal(t, filepath.Join(filepath.Dir(configPath), encryptedFilename), credentialPath(configPath))
	assert.Equal(t, encryptedFilename, credentialPath(""))
	assert.Equal(t, "account-work", key("work"))

	keyring := keyringStore{}
	assert.Equal(t, "os-keyring", keyring.Backend())
	assert.Equal(t, "encrypted-file", NewFileStore("path", "password").Backend())

	missing := NewFileStore(filepath.Join(t.TempDir(), "credentials.enc"), "password")
	_, err := missing.Get("missing")
	assert.ErrorIs(t, err, ErrNotFound)
}
