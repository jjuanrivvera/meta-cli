package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type guardInput struct {
	Binary      string
	Commands    []guardCommand
	BlockWrites bool
}

type guardFile struct {
	Path       string
	Content    string
	Executable bool
}

func renderHostConfig(host string, input guardInput) ([]guardFile, error) {
	switch host {
	case "claude-code":
		return claudeCodeConfig(input)
	case "codex":
		return codexConfig(input), nil
	case "opencode":
		return opencodeConfig(input)
	default:
		return nil, fmt.Errorf("unsupported host %q; use claude-code, codex, or opencode", host)
	}
}

func deniedCommands(input guardInput) []guardCommand {
	var denied []guardCommand
	for _, command := range input.Commands {
		if command.Kind == guardIrreversible || input.BlockWrites && command.Kind == guardApproval {
			denied = append(denied, command)
		}
	}
	return denied
}

func approvalCommands(input guardInput) []guardCommand {
	if input.BlockWrites {
		return nil
	}
	var approval []guardCommand
	for _, command := range input.Commands {
		if command.Kind == guardApproval {
			approval = append(approval, command)
		}
	}
	return approval
}

func claudeCodeConfig(input guardInput) ([]guardFile, error) {
	deny := deniedCommands(input)
	ask := approvalCommands(input)
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Bash|mcp__metactl__.*",
					"hooks": []any{
						map[string]any{"type": "command", "command": ".claude/hooks/metactl-guard.sh"},
					},
				},
			},
		},
		"permissions": map[string]any{"deny": exactPermissionRules(deny), "ask": exactPermissionRules(ask)},
	}
	raw, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, err
	}
	return []guardFile{
		{Path: ".claude/hooks/metactl-guard.sh", Content: buildPreToolUseHook(input), Executable: true},
		{Path: ".claude/settings.local.json", Content: string(raw) + "\n"},
	}, nil
}

func exactPermissionRules(commands []guardCommand) []string {
	rules := make([]string, 0, len(commands)*2)
	for _, command := range commands {
		rules = append(rules, "Bash(metactl "+command.Path+"*)")
		if command.Tool != "" {
			rules = append(rules, "mcp__metactl__"+strings.TrimPrefix(command.Tool, "metactl_")+"*")
		}
	}
	sort.Strings(rules)
	return rules
}

func buildPreToolUseHook(input guardInput) string {
	deny := deniedCommands(input)
	var shell strings.Builder
	shell.WriteString("#!/usr/bin/env bash\nset -euo pipefail\npayload=$(cat)\n")
	shell.WriteString("if command -v jq >/dev/null 2>&1; then\n")
	shell.WriteString("  command_text=$(printf '%s' \"$payload\" | jq -r '.tool_input.command // .tool_input.args // .command // empty' 2>/dev/null || true)\n")
	shell.WriteString("  tool_name=$(printf '%s' \"$payload\" | jq -r '.tool_name // empty' 2>/dev/null || true)\n")
	shell.WriteString("else\n")
	shell.WriteString("  command_text=$(printf '%s' \"$payload\" | tr '\\n{}:,' '     ')\n")
	shell.WriteString("  tool_name=$command_text\nfi\n")
	shell.WriteString("clean=$(printf '%s' \"$command_text\" | tr -d '\\042\\047\\134' | tr '\\n' ' ')\n")
	shell.WriteString("deny() { printf '%s\\n' '{\"decision\":\"block\",\"reason\":\"metactl guard blocked a state-changing command\"}'; exit 2; }\n")
	for _, command := range deny {
		pathPattern := strings.ReplaceAll(command.Path, " ", "[[:space:]]+")
		fmt.Fprintf(&shell, "printf '%%s' \"$clean\" | grep -Eq %s && deny\n", shellQuoteSingle("(^|[;&|([:space:]]+)([^[:space:]]*/)?"+input.Binary+"[[:space:]]+"+pathPattern+"([[:space:];&|)]|$)"))
		if command.Tool != "" {
			hostedTool := "mcp__metactl__" + strings.TrimPrefix(command.Tool, "metactl_")
			fmt.Fprintf(&shell, "printf '%%s' \"$tool_name\" | grep -Eq %s && deny\n", shellQuoteSingle("(^|[[:space:]\"'])("+command.Tool+"|"+hostedTool+")([[:space:]\"']|$)"))
		}
	}
	fmt.Fprintf(&shell, "printf '%%s' \"$clean\" | grep -Eqi %s && deny\n", shellQuoteSingle("(^|[;&|([:space:]]+)([^[:space:]]*/)?"+input.Binary+"[[:space:]]+api[[:space:]]+(DELETE|PUT|POST|PATCH)([[:space:];&|)]|$)"))
	shell.WriteString("exit 0\n")
	return shell.String()
}

func shellQuoteSingle(value string) string { return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'" }

func codexConfig(input guardInput) []guardFile {
	mode := "workspace-write"
	policy := "on-request"
	if input.BlockWrites {
		mode = "read-only"
		policy = "never"
	}
	content := fmt.Sprintf("sandbox_mode = %q\napproval_policy = %q\n", mode, policy)
	return []guardFile{{Path: ".codex/config.toml", Content: content}}
}

func opencodeConfig(input guardInput) ([]guardFile, error) {
	bashPermissions := map[string]string{"*": "allow"}
	for _, command := range deniedCommands(input) {
		bashPermissions[input.Binary+" "+command.Path+" *"] = "deny"
	}
	for _, command := range approvalCommands(input) {
		bashPermissions[input.Binary+" "+command.Path+" *"] = "ask"
	}
	value := map[string]any{"$schema": "https://opencode.ai/config.json", "permission": map[string]any{"bash": bashPermissions}}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return []guardFile{{Path: "opencode.json", Content: string(raw) + "\n"}}, nil
}

func writeGuardFiles(command *cobra.Command, directory string, files []guardFile) error {
	for _, file := range files {
		target := filepath.Join(directory, filepath.FromSlash(file.Path))
		if _, err := os.Stat(target); err == nil {
			return fmt.Errorf("refusing to overwrite %s", target)
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		mode := os.FileMode(0o600)
		if file.Executable {
			mode = 0o700
		}
		if err := os.WriteFile(target, []byte(file.Content), mode); err != nil { // #nosec G703 -- target is confined beneath the explicit --dir
			return err
		}
		fmt.Fprintln(command.ErrOrStderr(), "wrote", target)
	}
	return nil
}
