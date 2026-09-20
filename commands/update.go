package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	updaterpkg "github.com/jjuanrivvera/meta-cli/internal/update"
	buildversion "github.com/jjuanrivvera/meta-cli/internal/version"
)

var newUpdater = updaterpkg.New

func init() {
	registerMeta(func(root *cobra.Command, options *globalOptions) { root.AddCommand(newUpdateCmd(options)) })
}

func newUpdateCmd(options *globalOptions) *cobra.Command {
	var check bool
	command := &cobra.Command{Use: "update", Short: "Install the latest checksum-verified release", Example: "  metactl update --check\n  metactl update", RunE: func(command *cobra.Command, _ []string) error {
		updater := newUpdater(options.deps.HTTPClient)
		if check || options.dryRun {
			release, err := updater.Latest(command.Context())
			if err != nil {
				return err
			}
			return options.render(map[string]any{"current": buildversion.Version, "latest": release.TagName, "update_available": release.TagName != buildversion.Version}, []string{"current", "latest", "update_available"})
		}
		version, err := updater.Apply(command.Context(), "")
		if err != nil {
			return err
		}
		fmt.Fprintf(command.OutOrStdout(), "updated metactl to %s; backup saved beside the executable\n", version)
		return nil
	}}
	command.Flags().BoolVar(&check, "check", false, "check without replacing the executable")
	markLocal(command)
	return command
}
