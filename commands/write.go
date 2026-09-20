package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

type writeOptions struct {
	data string
	file string
	set  []string
}

func (options *writeOptions) addFlags(command *cobra.Command) {
	command.Flags().StringVarP(&options.data, "data", "d", "", "JSON object body")
	command.Flags().StringVarP(&options.file, "file", "f", "", "read the JSON object body from this explicit path, or - for stdin")
	command.Flags().StringArrayVar(&options.set, "set", nil, "set body field as key=value; repeatable")
}

func (options *writeOptions) body(command *cobra.Command, defaults map[string]any) ([]byte, error) {
	value := make(map[string]any, len(defaults)+len(options.set))
	for key, item := range defaults {
		if !emptyValue(item) {
			value[key] = item
		}
	}
	if options.data != "" && options.file != "" {
		return nil, fmt.Errorf("use only one of --data or --file")
	}
	raw := []byte(options.data)
	if options.file == "-" {
		var err error
		raw, err = io.ReadAll(command.InOrStdin())
		if err != nil {
			return nil, err
		}
	} else if options.file != "" {
		var err error
		raw, err = os.ReadFile(options.file) // #nosec G304 -- the user explicitly selected this input path
		if err != nil {
			return nil, fmt.Errorf("read --file: %w", err)
		}
	}
	if len(raw) > 0 {
		var fromInput map[string]any
		if err := json.Unmarshal(raw, &fromInput); err != nil {
			return nil, fmt.Errorf("decode JSON body: %w", err)
		}
		for key, item := range fromInput {
			value[key] = item
		}
	}
	for _, pair := range options.set {
		key, text, ok := strings.Cut(pair, "=")
		if !ok || key == "" {
			return nil, fmt.Errorf("--set value %q must be key=value", pair)
		}
		var item any
		if json.Unmarshal([]byte(text), &item) != nil {
			item = text
		}
		value[key] = item
	}
	return json.Marshal(value)
}

func emptyValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return typed == ""
	case []string:
		return len(typed) == 0
	default:
		return false
	}
}
