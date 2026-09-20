package commands

import (
	"slices"

	"github.com/njayp/ophis"
	"github.com/spf13/cobra"
)

var mcpExcludedGroups = []string{"agent", "alias", "api", "auth", "completion", "config", "doctor", "init", "mcp", "update", "version"}

var mcpExcludedFlags = []string{
	"show-token", ProfileFlag, "profile", "base-url", "upload-url", "graph-version",
	"page-id", "instagram-id", "business-id", "waba-id", "phone-id", "app-id",
}

func mcpCommandSelector(command *cobra.Command) bool {
	top := command
	for top.HasParent() && top.Parent().HasParent() {
		top = top.Parent()
	}
	return top.HasParent() && !slices.Contains(mcpExcludedGroups, top.Name())
}

func init() {
	registerMeta(func(root *cobra.Command, _ *globalOptions) {
		root.AddCommand(ophis.Command(&ophis.Config{
			ToolNamePrefix: "metactl",
			Selectors: []ophis.Selector{{
				CmdSelector:           mcpCommandSelector,
				InheritedFlagSelector: ophis.ExcludeFlags(mcpExcludedFlags...),
			}},
		}))
	})
}
