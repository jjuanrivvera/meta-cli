package commands

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jjuanrivvera/meta-cli/internal/auth"
)

type memoryStore struct{ values map[string]auth.Credential }

func (store *memoryStore) Get(account string) (auth.Credential, error) {
	credential, ok := store.values[account]
	if !ok {
		return auth.Credential{}, auth.ErrNotFound
	}
	return credential, nil
}
func (store *memoryStore) Set(account string, credential auth.Credential) error {
	store.values[account] = credential
	return nil
}
func (store *memoryStore) Delete(account string) error {
	delete(store.values, account)
	return nil
}
func (store *memoryStore) Backend() string { return "memory" }

func testRoot(t *testing.T) (*bytes.Buffer, *cobraHarness) {
	t.Helper()
	var output bytes.Buffer
	store := &memoryStore{values: map[string]auth.Credential{"default": {Token: "test-token"}}}
	root := NewRootCmd(Dependencies{Out: &output, Err: &output, In: bytes.NewBuffer(nil), ConfigPath: filepath.Join(t.TempDir(), "config.yaml"), Store: store})
	return &output, &cobraHarness{root: root, store: store}
}

type cobraHarness struct {
	root interface {
		SetArgs([]string)
		Execute() error
	}
	store *memoryStore
}

func TestRootHelpAndFlags(t *testing.T) {
	output, harness := testRoot(t)
	harness.root.SetArgs([]string{"--help"})
	require.NoError(t, harness.root.Execute())
	assert.Contains(t, output.String(), "Instagram publishing")
}

func TestSplitComma(t *testing.T) {
	assert.Equal(t, []string{"id", "name"}, splitComma("id, name,"))
}
