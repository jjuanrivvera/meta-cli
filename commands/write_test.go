package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteOptionsBody(t *testing.T) {
	tests := []struct {
		name    string
		options writeOptions
		stdin   string
		want    string
		wantErr string
	}{
		{name: "defaults and set", options: writeOptions{set: []string{"count=2", "label=value"}}, want: `{"count":2,"label":"value","name":"default"}`},
		{name: "inline JSON", options: writeOptions{data: `{"name":"override","active":true}`}, want: `{"active":true,"name":"override"}`},
		{name: "stdin", options: writeOptions{file: "-"}, stdin: `{"stdin":true}`, want: `{"name":"default","stdin":true}`},
		{name: "conflict", options: writeOptions{data: `{}`, file: "body.json"}, wantErr: "only one"},
		{name: "invalid JSON", options: writeOptions{data: `{`}, wantErr: "decode JSON"},
		{name: "invalid set", options: writeOptions{set: []string{"broken"}}, wantErr: "key=value"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := &cobra.Command{}
			command.SetIn(bytes.NewBufferString(test.stdin))
			body, err := test.options.body(command, map[string]any{"name": "default", "empty": "", "none": nil, "list": []string{}})
			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			assert.JSONEq(t, test.want, string(body))
		})
	}
}

func TestWriteOptionsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "body.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"file":true}`), 0o600))
	body, err := (&writeOptions{file: path}).body(&cobra.Command{}, nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"file":true}`, string(body))

	_, err = (&writeOptions{file: filepath.Join(t.TempDir(), "missing")}).body(&cobra.Command{}, nil)
	assert.ErrorContains(t, err, "read --file")
}
