package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/shlex"
	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/meta-cli/internal/config"
)

var builtInCommands = map[string]bool{"auth": true, "config": true, "init": true, "setup": true, "doctor": true, "completion": true, "alias": true, "api": true, "version": true, "mcp": true, "agent": true, "update": true, "instagram": true, "pages": true, "whatsapp": true}

func init() {
	registerMeta(func(root *cobra.Command, options *globalOptions) { root.AddCommand(newAliasCmd(options)) })
}

func newAliasCmd(options *globalOptions) *cobra.Command {
	command := &cobra.Command{Use: "alias", Short: "Manage command aliases"}
	command.AddCommand(newAliasSetCmd(options), newAliasListCmd(options), newAliasRemoveCmd(options))
	markLocal(command)
	return command
}

func newAliasSetCmd(options *globalOptions) *cobra.Command {
	command := &cobra.Command{Use: "set NAME EXPANSION", Short: "Create or replace an alias", Args: cobra.ExactArgs(2), Example: "  metactl alias set scheduled 'pages posts list --filter is_published=false'", RunE: func(_ *cobra.Command, args []string) error {
		if builtInCommands[args[0]] || strings.HasPrefix(args[0], "-") {
			return fmt.Errorf("alias %q would shadow a built-in or flag", args[0])
		}
		if _, err := shlex.Split(args[1]); err != nil {
			return fmt.Errorf("parse expansion: %w", err)
		}
		value, err := config.Load(options.deps.ConfigPath)
		if err != nil {
			return err
		}
		value.Aliases[args[0]] = args[1]
		return config.Save(options.deps.ConfigPath, value)
	}}
	markLocal(command)
	return command
}

func newAliasListCmd(options *globalOptions) *cobra.Command {
	command := &cobra.Command{Use: "list", Short: "List aliases", RunE: func(_ *cobra.Command, _ []string) error {
		value, err := config.Load(options.deps.ConfigPath)
		if err != nil {
			return err
		}
		names := make([]string, 0, len(value.Aliases))
		for name := range value.Aliases {
			names = append(names, name)
		}
		sort.Strings(names)
		rows := make([]map[string]any, 0, len(names))
		for _, name := range names {
			rows = append(rows, map[string]any{"name": name, "expansion": value.Aliases[name]})
		}
		return options.render(rows, []string{"name", "expansion"})
	}}
	markLocal(command)
	return command
}

func newAliasRemoveCmd(options *globalOptions) *cobra.Command {
	command := &cobra.Command{Use: "remove NAME", Short: "Remove an alias", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		value, err := config.Load(options.deps.ConfigPath)
		if err != nil {
			return err
		}
		delete(value.Aliases, args[0])
		return config.Save(options.deps.ConfigPath, value)
	}}
	markLocal(command)
	return command
}

func ExpandAliases(args []string, configPath string) ([]string, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return args, nil
	}
	value, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for depth := 0; depth < 8; depth++ {
		expansion, ok := value.Aliases[args[0]]
		if !ok {
			return args, nil
		}
		if seen[args[0]] {
			return nil, fmt.Errorf("alias cycle at %q", args[0])
		}
		seen[args[0]] = true
		words, err := shlex.Split(expansion)
		if err != nil {
			return nil, fmt.Errorf("parse alias %q: %w", args[0], err)
		}
		args = append(words, args[1:]...)
		if len(args) == 0 {
			return nil, fmt.Errorf("alias expands to an empty command")
		}
	}
	return nil, fmt.Errorf("alias expansion exceeded 8 levels")
}
