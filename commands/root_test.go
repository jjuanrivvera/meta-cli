package commands

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jjuanrivvera/meta-cli/internal/auth"
)

type memoryStore struct {
	values map[string]auth.Credential
	gets   int
}

func (store *memoryStore) Get(account string) (auth.Credential, error) {
	store.gets++
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

func TestSurfaceInspectionDoesNotReadCredentials(t *testing.T) {
	var output bytes.Buffer
	store := &memoryStore{values: map[string]auth.Credential{"default": {Token: "stored-token"}}}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	root := NewRootCmd(Dependencies{Out: &output, Err: &output, Store: store, ConfigPath: configPath})
	root.SetArgs([]string{"__surface", "resolve", "auth"})
	require.NoError(t, root.Execute())
	assert.Equal(t, "auth\n", output.String())
	assert.Zero(t, store.gets)

	_ = commandCredentialSecrets([]string{"__surface", "resolve", "auth"}, Dependencies{Store: store, ConfigPath: configPath})
	assert.Zero(t, store.gets)

	t.Chdir(t.TempDir())
	root = NewRootCmd(Dependencies{Out: &output, Err: &output, Store: store, ConfigPath: configPath})
	root.SetArgs([]string{"mcp", "tools"})
	require.NoError(t, root.Execute())
	assert.Zero(t, store.gets)

	_ = commandCredentialSecrets([]string{"mcp", "tools"}, Dependencies{Store: store, ConfigPath: configPath})
	assert.Zero(t, store.gets)
}

func TestSplitComma(t *testing.T) {
	assert.Equal(t, []string{"id", "name"}, splitComma("id, name,"))
}

func TestEveryCommandRendersHelp(t *testing.T) {
	root := NewRootCmd(Dependencies{Out: io.Discard, Err: io.Discard, Store: &memoryStore{values: map[string]auth.Credential{}}, ConfigPath: filepath.Join(t.TempDir(), "config.yaml")})
	var walk func(*cobra.Command)
	walk = func(command *cobra.Command) {
		t.Run(command.CommandPath(), func(t *testing.T) {
			command.SetOut(io.Discard)
			command.SetErr(io.Discard)
			require.NoError(t, command.Help())
		})
		for _, child := range command.Commands() {
			walk(child)
		}
	}
	walk(root)
}
