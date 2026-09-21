package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	buildversion "github.com/jjuanrivvera/meta-cli/internal/version"
)

func init() {
	registerMeta(func(root *cobra.Command, options *globalOptions) { root.AddCommand(newVersionCmd(options)) })
}

func newVersionCmd(options *globalOptions) *cobra.Command {
	var asJSON, check bool
	command := &cobra.Command{Use: "version", Short: "Print build version information", RunE: func(command *cobra.Command, _ []string) error {
		info := buildversion.Current()
		result := map[string]any{"version": info.Version, "commit": info.Commit, "date": info.Date}
		if check {
			latest, err := latestVersion(command, options.deps.HTTPClient)
			if err != nil {
				return err
			}
			result["latest"] = latest
			result["update_available"] = latest != "" && latest != info.Version
		}
		if asJSON {
			return options.render(result, nil)
		}
		fmt.Fprintf(command.OutOrStdout(), "meta %s (%s, %s)\n", info.Version, info.Commit, info.Date)
		if latest, ok := result["latest"].(string); ok {
			fmt.Fprintf(command.OutOrStdout(), "latest %s\n", latest)
		}
		return nil
	}}
	command.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	command.Flags().BoolVar(&check, "check", false, "check the latest published version")
	markLocal(command)
	return command
}

func latestVersion(command *cobra.Command, client *http.Client) (string, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	request, err := http.NewRequestWithContext(command.Context(), http.MethodGet, "https://api.github.com/repos/jjuanrivvera/meta-cli/releases/latest", nil)
	if err != nil {
		return "", err
	}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("check latest version: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("check latest version: GitHub returned %s", response.Status)
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return "", err
	}
	return strings.TrimSpace(body.TagName), nil
}
