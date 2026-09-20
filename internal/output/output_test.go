package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatsAndTransforms(t *testing.T) {
	value := []map[string]any{{"id": "2", "name": "=SUM(1,1)", "status": "off"}, {"id": "1", "name": "Alpha", "status": "on"}}
	for _, format := range []string{FormatTable, FormatJSON, FormatYAML, FormatCSV, FormatID} {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			renderer := New(Options{Format: format, Writer: &output, Warnings: &output, Columns: []string{"id", "name"}, Sort: "id"})
			require.NoError(t, renderer.Render(value))
			assert.NotEmpty(t, output.String())
			if format == FormatCSV {
				assert.Contains(t, output.String(), "'=SUM")
			}
			if format == FormatID {
				assert.Equal(t, "1\n2\n", output.String())
			}
		})
	}
}

func TestJQFilterColumnsAndSanitization(t *testing.T) {
	value := map[string]any{"data": []any{map[string]any{"id": "1", "status": "on", "message": "hello\x1b[31m"}, map[string]any{"id": "2", "status": "off"}}}
	var output bytes.Buffer
	renderer := New(Options{Format: FormatTable, Writer: &output, Warnings: &output, Filter: "status=on", JQ: ".data", Columns: []string{"id", "message"}})
	require.NoError(t, renderer.Render(value))
	assert.Contains(t, output.String(), "hello")
	assert.NotContains(t, output.String(), "\x1b")
	assert.NotContains(t, output.String(), "2")
}

func TestOutputErrorsAndWideValue(t *testing.T) {
	var output bytes.Buffer
	assert.Error(t, New(Options{Format: "xml", Writer: &output}).Render(map[string]any{"id": "1"}))
	assert.Error(t, New(Options{Format: FormatJSON, Writer: &output, JQ: ".["}).Render([]any{}))
	wide := strings.Repeat("x", 60)
	require.NoError(t, New(Options{Format: FormatTable, Writer: &output, Warnings: &output}).Render(map[string]any{"id": "1", "value": wide}))
	assert.Contains(t, output.String(), "truncated")
}

func TestCSVInjectionRules(t *testing.T) {
	assert.Equal(t, "'=1+1", safeCSV("=1+1"))
	assert.Equal(t, "-12", safeCSV("-12"))
	assert.Equal(t, "'-cmd", safeCSV("-cmd"))
}
