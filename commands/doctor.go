package commands

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/meta-cli/internal/api"
)

func init() {
	registerMeta(func(root *cobra.Command, options *globalOptions) { root.AddCommand(newDoctorCmd(options)) })
}

func newDoctorCmd(options *globalOptions) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{Use: "doctor", Short: "Check configuration, credentials, and Graph connectivity", Example: "  metactl doctor --json", RunE: func(command *cobra.Command, _ []string) error {
		if jsonOutput {
			options.output = "json"
		}
		client, name, account, err := options.clientFor()
		if err != nil {
			return err
		}
		var identity map[string]any
		connectivity := "ok"
		if err := client.JSON(command.Context(), api.Request{Method: http.MethodGet, Path: "me", Query: url.Values{"fields": {"id,name"}}}, &identity); err != nil {
			connectivity = err.Error()
		}
		result := map[string]any{
			"account": name, "graph_version": account.GraphVersion, "base_url": account.BaseURL,
			"credential_backend": options.deps.Store.Backend(), "connectivity": connectivity,
			"clock_utc": time.Now().UTC().Format(time.RFC3339),
		}
		if identity != nil {
			result["identity"] = identity
		}
		if renderErr := options.render(result, []string{"account", "graph_version", "credential_backend", "connectivity", "clock_utc"}); renderErr != nil {
			return renderErr
		}
		if connectivity != "ok" {
			return fmt.Errorf("doctor found a connectivity or authentication failure")
		}
		return nil
	}}
	command.Flags().BoolVar(&jsonOutput, "json", false, "emit JSON diagnostics")
	markLocal(command)
	return command
}
