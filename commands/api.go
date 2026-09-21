package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	graph "github.com/jjuanrivvera/meta-cli/internal/api"
)

func init() {
	registerAPI(func(root *cobra.Command, options *globalOptions) { root.AddCommand(newAPICmd(options)) })
}

func newAPICmd(options *globalOptions) *cobra.Command {
	var data string
	var queryValues []string
	command := &cobra.Command{Use: "api METHOD PATH", Short: "Send a raw authenticated Graph request", Args: cobra.ExactArgs(2), Example: "  meta api GET me -q fields=id,name\n  meta api POST 123/feed -d '{\"message\":\"Hello\"}' --dry-run", RunE: func(command *cobra.Command, args []string) error {
		method := strings.ToUpper(args[0])
		if _, ok := map[string]bool{http.MethodGet: true, http.MethodPost: true, http.MethodPut: true, http.MethodPatch: true, http.MethodDelete: true}[method]; !ok {
			return fmt.Errorf("unsupported method %q", method)
		}
		query := make(url.Values)
		for _, pair := range queryValues {
			key, value, ok := strings.Cut(pair, "=")
			if !ok || key == "" {
				return fmt.Errorf("query value %q must be key=value", pair)
			}
			query.Add(key, value)
		}
		var body []byte
		if data != "" {
			if !json.Valid([]byte(data)) {
				return fmt.Errorf("--data must be valid JSON")
			}
			body = []byte(data)
		}
		client, _, _, err := options.clientFor()
		if err != nil {
			return err
		}
		response, err := client.Do(command.Context(), graph.Request{Method: method, Path: args[1], Query: query, Body: body})
		if err != nil {
			return err
		}
		var result any
		if len(response.Body) == 0 {
			result = map[string]any{"success": true}
		} else if err := json.Unmarshal(response.Body, &result); err != nil {
			result = map[string]any{"body": string(response.Body)}
		}
		return options.render(result, nil)
	}}
	command.Flags().StringVarP(&data, "data", "d", "", "JSON request body")
	command.Flags().StringArrayVarP(&queryValues, "query", "q", nil, "query parameter key=value; repeatable")
	annotate(command, kindDestructive)
	return command
}
