package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/njayp/ophis"
	"github.com/spf13/cobra"
)

var mcpExcludedGroups = []string{"agent", "alias", "api", "auth", "completion", "config", "doctor", "init", "mcp", "update", "version"}

var mcpExcludedFlags = []string{
	"show-token", ProfileFlag, "profile", "base-url", "upload-url", "graph-version",
	"page-id", "instagram-id", "business-id", "waba-id", "phone-id", "app-id",
}

type mcpFileConfinement struct {
	root      string
	directory *os.File
}

type mcpConfinementContextKey struct{}

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
				LocalFlagSelector:     ophis.ExcludeFlags("yes"),
				InheritedFlagSelector: ophis.ExcludeFlags(mcpExcludedFlags...),
				Middleware:            confineMCPFiles,
			}},
		}))
	})
}

func confineMCPFiles(ctx context.Context, request *mcp.CallToolRequest, input ophis.ToolInput, next ophis.ExecuteFunc) (result *mcp.CallToolResult, output ophis.ToolOutput, err error) {
	defer func() { err = SanitizeError(err, nil, Dependencies{}) }()
	for _, argument := range input.Args {
		if strings.HasPrefix(argument, "-") {
			return nil, ophis.ToolOutput{}, fmt.Errorf("MCP positional arguments may not inject command flags")
		}
	}
	root := os.Getenv("METACTL_MCP_ROOT")
	if root == "" {
		root, err = os.Getwd()
		if err != nil {
			return nil, ophis.ToolOutput{}, fmt.Errorf("resolve MCP file root: %w", err)
		}
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, ophis.ToolOutput{}, fmt.Errorf("resolve MCP file root: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, ophis.ToolOutput{}, fmt.Errorf("resolve MCP file root: %w", err)
	}
	for _, flag := range []string{"file", "thumbnail-file"} {
		value, ok := input.Flags[flag].(string)
		if !ok || value == "" {
			continue
		}
		resolved, err := pathWithinRoot(root, value)
		if err != nil {
			return nil, ophis.ToolOutput{}, err
		}
		input.Flags[flag] = resolved
	}
	if value, ok := input.Flags["video"].(string); ok && value != "" &&
		!strings.HasPrefix(value, "https://") && !strings.HasPrefix(value, "http://") {
		resolved, err := pathWithinRoot(root, value)
		if err != nil {
			return nil, ophis.ToolOutput{}, err
		}
		input.Flags["video"] = resolved
	}
	// Ophis omits false booleans, which can invert flags whose Cobra default is
	// true. Encoding the value in the flag name preserves an explicit false.
	for name, value := range input.Flags {
		boolean, ok := value.(bool)
		if !ok || boolean {
			continue
		}
		delete(input.Flags, name)
		input.Flags[name+"=false"] = true
	}
	input.Flags["mcp-root-internal"] = root
	result, output, err = next(ctx, request, input)
	secrets := commandCredentialSecrets(nil, Dependencies{})
	output.StdOut = redactCommandText(output.StdOut, secrets)
	output.StdErr = redactCommandText(output.StdErr, secrets)
	return result, output, err
}

func confineCommandFiles(command *cobra.Command, root string) error {
	if root == "" {
		return nil
	}
	directory, err := os.Open(root) // #nosec G304 -- root was canonicalized and supplied by the MCP middleware
	if err != nil {
		return fmt.Errorf("open MCP file root: %w", err)
	}
	info, err := directory.Stat()
	if err != nil || !info.IsDir() {
		_ = directory.Close()
		return fmt.Errorf("MCP file root must be a directory")
	}
	for _, flagName := range []string{"file", "thumbnail-file", "video"} {
		flag := command.Flags().Lookup(flagName)
		if flag == nil || !flag.Changed {
			continue
		}
		selected := flag.Value.String()
		if flagName == "video" && (strings.HasPrefix(selected, "https://") || strings.HasPrefix(selected, "http://")) {
			continue
		}
		resolved, err := pathWithinRoot(root, selected)
		if err != nil {
			_ = directory.Close()
			return err
		}
		if err := command.Flags().Set(flagName, resolved); err != nil {
			_ = directory.Close()
			return fmt.Errorf("set confined MCP file path: %w", err)
		}
	}
	currentRoot, err := os.Stat(root)
	if err != nil || !os.SameFile(info, currentRoot) {
		_ = directory.Close()
		return fmt.Errorf("MCP file root changed while validating the request")
	}
	command.SetContext(context.WithValue(command.Context(), mcpConfinementContextKey{}, &mcpFileConfinement{
		root: root, directory: directory,
	}))
	return nil
}

func closeMCPConfinement(ctx context.Context) {
	confinement, _ := ctx.Value(mcpConfinementContextKey{}).(*mcpFileConfinement)
	if confinement != nil && confinement.directory != nil {
		_ = confinement.directory.Close()
	}
}

func pathWithinRoot(root, selected string) (string, error) {
	if selected == "-" {
		return "", fmt.Errorf("MCP file input must be a regular file under METACTL_MCP_ROOT")
	}
	absolute := selected
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(root, absolute)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve MCP file path: %w", err)
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("MCP file path %q is outside METACTL_MCP_ROOT", selected)
	}
	info, err := os.Stat(resolved) // #nosec G703 -- EvalSymlinks plus the Rel check above confines this path to the MCP root
	if err != nil {
		return "", fmt.Errorf("inspect MCP file path: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("MCP file path %q is not a regular file", selected)
	}
	return resolved, nil
}
