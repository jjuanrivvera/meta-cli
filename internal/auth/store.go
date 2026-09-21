package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jjuanrivvera/meta-cli/internal/config"
	"github.com/zalando/go-keyring"
)

const (
	serviceName = "meta"
	// #nosec G101 -- this is an environment variable name, not a credential value.
	passwordEnv       = "META_KEYRING_PASSWORD"
	backendEnv        = "META_KEYRING_BACKEND"
	encryptedFilename = "credentials.enc"
)

var ErrNotFound = errors.New("no stored credential")

var (
	keyringGet    = keyring.Get
	keyringSet    = keyring.Set
	keyringDelete = keyring.Delete
)

type Credential struct {
	Token     string `json:"token,omitempty"`
	PageToken string `json:"page_token,omitempty"`
	AppSecret string `json:"app_secret,omitempty"`
}

type Store interface {
	Get(account string) (Credential, error)
	Set(account string, credential Credential) error
	Delete(account string) error
	Backend() string
}

type keyringStore struct{}

func (keyringStore) Backend() string { return "os-keyring" }

func (keyringStore) Get(account string) (Credential, error) {
	raw, err := keyringGet(serviceName, key(account))
	if errors.Is(err, keyring.ErrNotFound) {
		return Credential{}, ErrNotFound
	}
	if err != nil {
		return Credential{}, fmt.Errorf("read OS keyring: %w", err)
	}
	var credential Credential
	if err := json.Unmarshal([]byte(raw), &credential); err != nil {
		return Credential{}, fmt.Errorf("decode credential: %w", err)
	}
	return credential, nil
}

func (keyringStore) Set(account string, credential Credential) error {
	raw, err := json.Marshal(credential)
	if err != nil {
		return err
	}
	if err := keyringSet(serviceName, key(account), string(raw)); err != nil {
		return fmt.Errorf("write OS keyring: %w", err)
	}
	return nil
}

func (keyringStore) Delete(account string) error {
	err := keyringDelete(serviceName, key(account))
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

func key(account string) string { return "account-" + account }

type fileStore struct {
	path     string
	password string
	pathErr  error
}

func NewStore(configPath string) Store {
	if strings.EqualFold(os.Getenv(backendEnv), "file") {
		path, err := credentialPath(configPath)
		return &fileStore{path: path, password: os.Getenv(passwordEnv), pathErr: err}
	}
	return keyringStore{}
}

func NewFileStore(path, password string) Store { return &fileStore{path: path, password: password} }

func (store *fileStore) Backend() string { return "encrypted-file" }

func (store *fileStore) Get(account string) (Credential, error) {
	values, err := store.load()
	if err != nil {
		return Credential{}, err
	}
	credential, ok := values[account]
	if !ok {
		return Credential{}, ErrNotFound
	}
	return credential, nil
}

func (store *fileStore) Set(account string, credential Credential) error {
	values, err := store.load()
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if values == nil {
		values = map[string]Credential{}
	}
	values[account] = credential
	return store.save(values)
}

func (store *fileStore) Delete(account string) error {
	values, err := store.load()
	if err != nil {
		return err
	}
	if _, ok := values[account]; !ok {
		return ErrNotFound
	}
	delete(values, account)
	return store.save(values)
}

func (store *fileStore) load() (map[string]Credential, error) {
	if store.pathErr != nil {
		return nil, store.pathErr
	}
	if store.password == "" {
		return nil, fmt.Errorf("%s is required for the encrypted credential store", passwordEnv)
	}
	raw, err := os.ReadFile(store.path) // #nosec G304 -- path is the selected credential file
	if errors.Is(err, os.ErrNotExist) {
		return map[string]Credential{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read encrypted credentials: %w", err)
	}
	plain, err := decrypt(raw, store.password)
	if err != nil {
		return nil, err
	}
	var values map[string]Credential
	if err := json.Unmarshal(plain, &values); err != nil {
		return nil, fmt.Errorf("decode credentials: %w", err)
	}
	return values, nil
}

func (store *fileStore) save(values map[string]Credential) error {
	if store.pathErr != nil {
		return store.pathErr
	}
	if store.password == "" {
		return fmt.Errorf("%s is required for the encrypted credential store", passwordEnv)
	}
	plain, err := json.Marshal(values)
	if err != nil {
		return err
	}
	raw, err := encrypt(plain, store.password)
	if err != nil {
		return err
	}
	dir := filepath.Dir(store.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".credentials-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(raw); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, store.path)
}

func encrypt(plain []byte, password string) ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, 120000, 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	sealed := aead.Seal(nil, nonce, plain, nil)
	return append(append([]byte("MCTL1"), salt...), append(nonce, sealed...)...), nil
}

func decrypt(raw []byte, password string) ([]byte, error) {
	if len(raw) < 5+16 || string(raw[:5]) != "MCTL1" {
		return nil, fmt.Errorf("invalid encrypted credential file")
	}
	salt := raw[5:21]
	key, err := pbkdf2.Key(sha256.New, password, salt, 120000, 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < 21+aead.NonceSize() {
		return nil, fmt.Errorf("invalid encrypted credential file")
	}
	nonce := raw[21 : 21+aead.NonceSize()]
	plain, err := aead.Open(nil, nonce, raw[21+aead.NonceSize():], nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt credentials: wrong password or modified file")
	}
	return plain, nil
}

func credentialPath(configPath string) (string, error) {
	if configPath == "" {
		var err error
		configPath, err = config.Path()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(filepath.Dir(configPath), encryptedFilename), nil
}
