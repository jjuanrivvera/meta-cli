package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/itchyny/gojq"
	"gopkg.in/yaml.v3"
)

const (
	FormatTable = "table"
	FormatJSON  = "json"
	FormatYAML  = "yaml"
	FormatCSV   = "csv"
	FormatID    = "id"
)

type Options struct {
	Format   string
	Columns  []string
	IDField  string
	JQ       string
	Sort     string
	Filter   string
	NoColor  bool
	Writer   io.Writer
	Warnings io.Writer
	Secrets  []string
}

type Renderer struct{ options Options }

func New(options Options) *Renderer {
	if options.Format == "" {
		options.Format = FormatTable
	}
	if options.IDField == "" {
		options.IDField = "id"
	}
	if options.Writer == nil {
		options.Writer = io.Discard
	}
	if options.Warnings == nil {
		options.Warnings = io.Discard
	}
	return &Renderer{options: options}
}

func (renderer *Renderer) Render(value any) error {
	normalized, err := normalize(value)
	if err != nil {
		return err
	}
	secrets := append([]string(nil), renderer.options.Secrets...)
	secrets = append(secrets, collectSensitiveValues(normalized)...)
	renderer.options.Secrets = secrets
	normalized = redact(normalized, secrets)
	if renderer.options.JQ != "" {
		normalized, err = applyJQ(renderer.options.JQ, normalized)
		if err != nil {
			return err
		}
		normalized = redact(normalized, secrets)
	}
	rows := rowsFrom(normalized)
	rows = filterRows(rows, renderer.options.Filter)
	sortRows(rows, renderer.options.Sort)
	switch renderer.options.Format {
	case FormatJSON:
		encoder := json.NewEncoder(renderer.options.Writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(normalizedRowsOrValue(normalized, rows, renderer.options.Filter, renderer.options.Sort))
	case FormatYAML:
		return yaml.NewEncoder(renderer.options.Writer).Encode(normalizedRowsOrValue(normalized, rows, renderer.options.Filter, renderer.options.Sort))
	case FormatCSV:
		return renderer.csv(rows)
	case FormatID:
		for _, row := range rows {
			if value, ok := lookup(row, renderer.options.IDField); ok {
				fmt.Fprintln(renderer.options.Writer, scalar(value))
			}
		}
		return nil
	case FormatTable:
		return renderer.table(rows)
	default:
		return fmt.Errorf("unsupported output format %q; use table, json, yaml, csv, or id", renderer.options.Format)
	}
}

func collectSensitiveValues(value any) []string {
	var secrets []string
	var collectStrings func(any)
	collectStrings = func(current any) {
		switch typed := current.(type) {
		case string:
			if typed != "" {
				secrets = append(secrets, typed)
			}
		case []any:
			for _, item := range typed {
				collectStrings(item)
			}
		case map[string]any:
			for _, item := range typed {
				collectStrings(item)
			}
		}
	}
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			for key, child := range typed {
				if sensitiveName(key) {
					collectStrings(child)
				}
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	return secrets
}

func redact(value any, secrets []string) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			redactedKey := redactString(key, secrets)
			if sensitiveName(key) {
				child = "<redacted>"
			} else {
				child = redact(child, secrets)
			}
			if redactedKey != key {
				delete(typed, key)
			}
			typed[redactedKey] = child
		}
	case []any:
		for index, child := range typed {
			typed[index] = redact(child, secrets)
		}
	case string:
		return redactString(typed, secrets)
	}
	return value
}

func redactString(value string, secrets []string) string {
	for _, secret := range secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "<redacted>")
		}
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return value
	}
	parsed.Path = redactKnownValue(parsed.Path, secrets)
	parsed.RawPath = ""
	query := parsed.Query()
	for key := range query {
		if sensitiveName(key) {
			query.Set(key, "<redacted>")
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func redactKnownValue(value string, secrets []string) string {
	for _, secret := range secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "<redacted>")
		}
	}
	return value
}

func sensitiveName(name string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(name, "-", "_"))
	return normalized == "token" || strings.HasSuffix(normalized, "_token") || strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "password") || strings.Contains(normalized, "authorization") ||
		strings.Contains(normalized, "proof")
}

func (renderer *Renderer) table(rows []map[string]any) error {
	if len(rows) == 0 {
		return nil
	}
	columns := renderer.columns(rows)
	displayColumns := renderer.displayColumns(columns)
	widths := make([]int, len(columns))
	for index := range columns {
		widths[index] = utf8.RuneCountInString(strings.ToUpper(displayColumns[index]))
	}
	data := make([][]string, len(rows))
	truncated := false
	for rowIndex, row := range rows {
		data[rowIndex] = make([]string, len(columns))
		for columnIndex, column := range columns {
			value, _ := lookup(row, column)
			cell := sanitizeTerminal(scalar(value))
			if utf8.RuneCountInString(cell) > 48 {
				cell = truncateRunes(cell, 47) + "…"
				truncated = true
			}
			data[rowIndex][columnIndex] = cell
			if size := utf8.RuneCountInString(cell); size > widths[columnIndex] {
				widths[columnIndex] = size
			}
		}
	}
	printRow(renderer.options.Writer, uppercase(displayColumns), widths)
	for _, row := range data {
		printRow(renderer.options.Writer, row, widths)
	}
	if truncated {
		fmt.Fprintln(renderer.options.Warnings, "note: wide values truncated; use -o json for complete data")
	}
	return nil
}

func (renderer *Renderer) csv(rows []map[string]any) error {
	writer := csv.NewWriter(renderer.options.Writer)
	columns := renderer.columns(rows)
	if err := writer.Write(renderer.displayColumns(columns)); err != nil {
		return err
	}
	for _, row := range rows {
		record := make([]string, len(columns))
		for index, column := range columns {
			value, _ := lookup(row, column)
			record[index] = safeCSV(scalar(value))
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func (renderer *Renderer) displayColumns(columns []string) []string {
	display := make([]string, len(columns))
	for index, column := range columns {
		display[index] = redactString(column, renderer.options.Secrets)
	}
	return display
}

func (renderer *Renderer) columns(rows []map[string]any) []string {
	if len(renderer.options.Columns) > 0 {
		return renderer.options.Columns
	}
	seen := map[string]bool{}
	for _, row := range rows {
		for key := range row {
			seen[key] = true
		}
	}
	preferred := []string{"id", "name", "username", "status", "message", "created_time", "updated_time"}
	columns := make([]string, 0, len(seen))
	for _, key := range preferred {
		if seen[key] {
			columns = append(columns, key)
			delete(seen, key)
		}
	}
	rest := make([]string, 0, len(seen))
	for key := range seen {
		rest = append(rest, key)
	}
	sort.Strings(rest)
	columns = append(columns, rest...)
	if len(columns) > 10 {
		fmt.Fprintln(renderer.options.Warnings, "note: showing the first 10 fields; use --columns or -o json")
		columns = columns[:10]
	}
	return columns
}

func normalize(value any) (any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("normalize output: %w", err)
	}
	var normalized any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&normalized); err != nil {
		return nil, fmt.Errorf("normalize output: %w", err)
	}
	return normalized, nil
}

func rowsFrom(value any) []map[string]any {
	switch typed := value.(type) {
	case []any:
		rows := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if row, ok := item.(map[string]any); ok {
				rows = append(rows, row)
			} else {
				rows = append(rows, map[string]any{"value": item})
			}
		}
		return rows
	case map[string]any:
		if data, ok := typed["data"].([]any); ok {
			return rowsFrom(data)
		}
		return []map[string]any{typed}
	default:
		return []map[string]any{{"value": typed}}
	}
}

func applyJQ(expression string, value any) (any, error) {
	query, err := gojq.Parse(expression)
	if err != nil {
		return nil, fmt.Errorf("parse --jq: %w", err)
	}
	iterator := query.Run(value)
	var results []any
	for {
		item, ok := iterator.Next()
		if !ok {
			break
		}
		if err, ok := item.(error); ok {
			return nil, fmt.Errorf("run --jq: %w", err)
		}
		results = append(results, item)
	}
	if len(results) == 1 {
		return results[0], nil
	}
	return results, nil
}

func filterRows(rows []map[string]any, filter string) []map[string]any {
	if filter == "" {
		return rows
	}
	key, value, ok := strings.Cut(filter, "=")
	if !ok {
		return rows
	}
	result := rows[:0]
	for _, row := range rows {
		candidate, found := lookup(row, key)
		if found && scalar(candidate) == value {
			result = append(result, row)
		}
	}
	return result
}

func sortRows(rows []map[string]any, field string) {
	if field == "" {
		return
	}
	sort.SliceStable(rows, func(left, right int) bool {
		leftValue, _ := lookup(rows[left], field)
		rightValue, _ := lookup(rows[right], field)
		return scalar(leftValue) < scalar(rightValue)
	})
}

func lookup(row map[string]any, path string) (any, bool) {
	var current any = row
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func scalar(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case json.Number:
		return typed.String()
	case bool:
		return strconv.FormatBool(typed)
	default:
		raw, _ := json.Marshal(typed)
		return string(raw)
	}
}

func safeCSV(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if trimmed == "" {
		return value
	}
	first := trimmed[0]
	if first == '=' || first == '+' || first == '@' || (first == '-' && (len(trimmed) == 1 || trimmed[1] < '0' || trimmed[1] > '9')) {
		return "'" + value
	}
	return value
}

func sanitizeTerminal(value string) string {
	var output strings.Builder
	for _, character := range value {
		if character == '\n' || character == '\t' || character >= 0x20 && character != 0x7f {
			output.WriteRune(character)
		}
	}
	clean := output.String()
	for {
		start := strings.IndexByte(clean, 0x1b)
		if start < 0 {
			break
		}
		end := start + 1
		for end < len(clean) && (clean[end] < '@' || clean[end] > '~') {
			end++
		}
		if end < len(clean) {
			end++
		}
		clean = clean[:start] + clean[end:]
	}
	return clean
}

func truncateRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func uppercase(values []string) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = strings.ToUpper(value)
	}
	return result
}

func printRow(writer io.Writer, values []string, widths []int) {
	for index, value := range values {
		if index > 0 {
			fmt.Fprint(writer, "  ")
		}
		fmt.Fprint(writer, value)
		if index < len(values)-1 {
			fmt.Fprint(writer, strings.Repeat(" ", widths[index]-utf8.RuneCountInString(value)))
		}
	}
	fmt.Fprintln(writer)
}

func normalizedRowsOrValue(original any, rows []map[string]any, filter, sortField string) any {
	if filter != "" || sortField != "" {
		return rows
	}
	return original
}
