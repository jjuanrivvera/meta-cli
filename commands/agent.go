package commands

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/meta-cli/internal/config"
)

const (
	guardRead         = "read"
	guardApproval     = "approval"
	guardIrreversible = "irreversible"
	guardDynamic      = "dynamic"
)

type guardCommand struct {
	Path string `json:"path" yaml:"path"`
	Tool string `json:"tool" yaml:"tool"`
	Kind string `json:"kind" yaml:"kind"`
}

var guardLocalGroups = map[string]bool{"__surface": true, "agent": true, "alias": true, "auth": true, "completion": true, "config": true, "doctor": true, "help": true, "init": true, "mcp": true, "update": true, "version": true}

var alwaysIrreversible = map[string]bool{"delete": true, "remove": true, "unsubscribe": true, "logout": true}

func classifyCommands(root *cobra.Command) []guardCommand {
	var result []guardCommand
	var walk func(*cobra.Command, [][]string)
	walk = func(command *cobra.Command, prefixes [][]string) {
		if command == root {
			prefixes = [][]string{{}}
		} else {
			names := append([]string{command.Name()}, command.Aliases...)
			var expanded [][]string
			for _, prefix := range prefixes {
				for _, name := range names {
					expanded = append(expanded, append(append([]string(nil), prefix...), name))
				}
			}
			prefixes = expanded
		}
		if command.HasSubCommands() {
			for _, child := range command.Commands() {
				walk(child, prefixes)
			}
			return
		}
		for _, path := range prefixes {
			if len(path) == 0 || guardLocalGroups[path[0]] {
				continue
			}
			kind := guardKind(command, path)
			joined := strings.Join(path, " ")
			result = append(result, guardCommand{Path: joined, Tool: "meta_" + strings.ReplaceAll(joined, " ", "_"), Kind: kind})
		}
	}
	walk(root, nil)
	sort.Slice(result, func(left, right int) bool { return result[left].Path < result[right].Path })
	return result
}

func guardKind(command *cobra.Command, path []string) string {
	if path[0] == "api" {
		return guardDynamic
	}
	if alwaysIrreversible[path[len(path)-1]] || AnnotationKind(command) == kindDestructive {
		return guardIrreversible
	}
	if AnnotationKind(command) == kindRead {
		return guardRead
	}
	return guardApproval
}

func addUserAliases(commands []guardCommand, configPath string) []guardCommand {
	value, err := config.Load(configPath)
	if err != nil {
		return commands
	}
	byPath := make(map[string]string, len(commands))
	for _, command := range commands {
		byPath[command.Path] = command.Kind
	}
	for name := range value.Aliases {
		expanded, err := ExpandAliases([]string{name}, configPath)
		if err != nil {
			continue
		}
		for length := len(expanded); length > 0; length-- {
			if kind, ok := byPath[strings.Join(expanded[:length], " ")]; ok {
				commands = append(commands, guardCommand{Path: name, Tool: "", Kind: kind})
				break
			}
		}
	}
	sort.Slice(commands, func(left, right int) bool { return commands[left].Path < commands[right].Path })
	return commands
}

func init() {
	registerMeta(func(root *cobra.Command, options *globalOptions) { root.AddCommand(newAgentCmd(root, options)) })
}

func newAgentCmd(root *cobra.Command, options *globalOptions) *cobra.Command {
	command := &cobra.Command{Use: "agent", Short: "Generate automation safety policy from command annotations"}
	command.AddCommand(newAgentGuardCmd(root, options), newAgentClassifyCmd(root, options))
	markLocal(command)
	return command
}

func newAgentGuardCmd(root *cobra.Command, options *globalOptions) *cobra.Command {
	var host, directory string
	var blockWrites, write bool
	command := &cobra.Command{
		Use:   "guard",
		Short: "Render host policy that blocks irreversible operations",
		Example: "  meta agent guard --host codex\n" +
			"  meta agent guard --host opencode --write --dir .",
		RunE: func(command *cobra.Command, _ []string) error {
			commands := addUserAliases(classifyCommands(root), options.deps.ConfigPath)
			files, err := renderHostConfig(host, guardInput{Binary: "meta", Commands: commands, BlockWrites: blockWrites})
			if err != nil {
				return err
			}
			if write {
				return writeGuardFiles(command, directory, files)
			}
			for _, file := range files {
				fmt.Fprintf(command.OutOrStdout(), "--- %s ---\n%s", filepath.ToSlash(file.Path), file.Content)
				if !strings.HasSuffix(file.Content, "\n") {
					fmt.Fprintln(command.OutOrStdout())
				}
			}
			return nil
		},
	}
	command.Flags().StringVar(&host, "host", "", "target host: claude-code, codex, or opencode")
	command.Flags().StringVar(&directory, "dir", ".", "project directory for --write")
	command.Flags().BoolVar(&blockWrites, "all-writes", false, "block ordinary writes as well as irreversible operations")
	command.Flags().BoolVar(&write, "write", false, "write files without replacing existing files")
	_ = command.MarkFlagRequired("host")
	markLocal(command)
	return command
}

func newAgentClassifyCmd(root *cobra.Command, options *globalOptions) *cobra.Command {
	command := &cobra.Command{Use: "classify", Short: "List command safety classifications", RunE: func(_ *cobra.Command, _ []string) error {
		return options.render(classifyCommands(root), []string{"path", "kind", "tool"})
	}}
	markLocal(command)
	return command
}
