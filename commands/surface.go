package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	registerMeta(func(root *cobra.Command, _ *globalOptions) {
		surface := &cobra.Command{Use: "__surface", Hidden: true}
		resolve := &cobra.Command{
			Use:                "resolve COMMAND...",
			Hidden:             true,
			DisableFlagParsing: true,
			Args:               cobra.MinimumNArgs(1),
			RunE: func(command *cobra.Command, args []string) error {
				found, remaining, err := command.Root().Find(args)
				if err != nil || len(remaining) != 0 {
					return fmt.Errorf("command path %q does not resolve exactly", strings.Join(args, " "))
				}
				fmt.Fprintln(command.OutOrStdout(), strings.TrimPrefix(found.CommandPath(), command.Root().Name()+" "))
				return nil
			},
		}
		list := &cobra.Command{Use: "list", Hidden: true, Run: func(command *cobra.Command, _ []string) {
			var paths []string
			walkRunnable(command.Root(), func(candidate *cobra.Command) {
				if manifestCommand(candidate) {
					paths = append(paths, strings.TrimPrefix(candidate.CommandPath(), command.Root().Name()+" "))
				}
			})
			sort.Strings(paths)
			for _, path := range paths {
				fmt.Fprintln(command.OutOrStdout(), path)
			}
		}}
		surface.AddCommand(resolve, list)
		root.AddCommand(surface)
	})
}

func walkRunnable(command *cobra.Command, visit func(*cobra.Command)) {
	if command.Run != nil || command.RunE != nil {
		visit(command)
	}
	for _, child := range command.Commands() {
		walkRunnable(child, visit)
	}
}

func manifestCommand(command *cobra.Command) bool {
	path := strings.TrimPrefix(command.CommandPath(), command.Root().Name()+" ")
	return strings.HasPrefix(path, "instagram ") || strings.HasPrefix(path, "pages ") ||
		strings.HasPrefix(path, "whatsapp ") || path == "auth debug" || path == "auth exchange"
}
