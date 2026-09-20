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

func TestValidateRejectsStaticOutputErrors(t *testing.T) {
	require.NoError(t, Validate(FormatJSON, ".id"))
	require.ErrorContains(t, Validate("xml", ""), "unsupported output format")
	require.ErrorContains(t, Validate(FormatJSON, ".["), "parse --jq")
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

func TestSensitiveFieldsAreRedactedBeforeJQ(t *testing.T) {
	value := map[string]any{
		"access_token": "page-token-value",
		"nested":       map[string]any{"app_secret": "app-secret-value", "token_type": "bearer"},
	}
	var output bytes.Buffer
	require.NoError(t, New(Options{Format: FormatJSON, Writer: &output, JQ: "."}).Render(value))
	assert.NotContains(t, output.String(), "page-token-value")
	assert.NotContains(t, output.String(), "app-secret-value")
	assert.Contains(t, output.String(), "redacted")
	assert.Contains(t, output.String(), "bearer")

	output.Reset()
	require.NoError(t, New(Options{Format: FormatJSON, Writer: &output, JQ: ".access_token"}).Render(value))
	assert.NotContains(t, output.String(), "page-token-value")

	output.Reset()
	require.NoError(t, New(Options{
		Format: FormatJSON, Writer: &output, JQ: `"app-secret-value"`, Secrets: []string{"app-secret-value"},
	}).Render(value))
	assert.NotContains(t, output.String(), "app-secret-value")
	assert.Contains(t, output.String(), "redacted")
}

func TestPageTokensAreRedactedInHumanAndMachineFormats(t *testing.T) {
	for _, format := range []string{FormatTable, FormatJSON, FormatYAML, FormatCSV} {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			require.NoError(t, New(Options{Format: format, Writer: &output}).Render(map[string]any{"id": "page-1", "access_token": "page-secret-token"}))
			assert.NotContains(t, output.String(), "page-secret-token")
		})
	}
}

func TestNewSensitiveFieldValuesAreRedactedFromOrdinaryFields(t *testing.T) {
	const discovered = "newly-discovered-page-credential"
	var output bytes.Buffer
	require.NoError(t, New(Options{Format: FormatJSON, Writer: &output}).Render(map[string]any{
		"access_token": discovered,
		"name":         "echo " + discovered,
	}))
	assert.NotContains(t, output.String(), discovered)
	assert.Contains(t, output.String(), "redacted")
}

func TestCredentialValuesAndSensitiveURLQueriesAreRedactedBeforeJQ(t *testing.T) {
	value := map[string]any{
		"app-secret-value-field": "ordinary",
		"message":                "echo page-secret-token and app-secret-value",
		"paging": map[string]any{
			"next": "https://graph.example/items?after=cursor&access_token=" + "unknown-response-token" + "&appsecret_proof=proof-value",
		},
	}
	var output bytes.Buffer
	require.NoError(t, New(Options{
		Format: FormatJSON, Writer: &output, JQ: ".", Secrets: []string{"page-secret-token", "app-secret-value"},
	}).Render(value))
	for _, secret := range []string{"page-secret-token", "app-secret-value", "unknown-response-token", "proof-value"} {
		assert.NotContains(t, output.String(), secret)
	}
	assert.Contains(t, output.String(), "redacted")

	output.Reset()
	require.NoError(t, New(Options{
		Format: FormatCSV, Writer: &output,
		Columns: []string{"page-secret-token", "app-secret-value"},
		Secrets: []string{"page-secret-token", "app-secret-value"},
	}).Render(map[string]any{"id": "1"}))
	assert.NotContains(t, output.String(), "page-secret-token")
	assert.NotContains(t, output.String(), "app-secret-value")
}
