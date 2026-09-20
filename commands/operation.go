package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jjuanrivvera/meta-cli/internal/api"
	"github.com/jjuanrivvera/meta-cli/internal/config"
)

type operationSpec struct {
	Use     string
	Aliases []string
	Short   string
	Long    string
	Example string
	Kind    string
	Args    cobra.PositionalArgs
	Columns []string
	Flags   func(*cobra.Command)
	Confirm func() string
	Run     func(*cobra.Command, *globalOptions, *api.Client, config.Account, []string) (any, error)
}

func newOperationCommand(options *globalOptions, spec operationSpec) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use: spec.Use, Aliases: spec.Aliases, Short: spec.Short, Long: spec.Long,
		Example: spec.Example, Args: spec.Args,
		RunE: func(command *cobra.Command, args []string) (runErr error) {
			defer closeMCPConfinement(command.Context())
			defer func() { runErr = options.sanitizeError(runErr) }()
			client, _, account, err := options.clientForCommand(command)
			if err != nil {
				return err
			}
			if spec.Kind == kindDestructive && !yes && !options.dryRun {
				message := "This operation is destructive. Continue? [y/N] "
				if spec.Confirm != nil {
					message = spec.Confirm()
				}
				answer, err := promptLine(command, message)
				if err != nil {
					return err
				}
				if !strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes") {
					return fmt.Errorf("aborted")
				}
			}
			result, err := spec.Run(command, options, client, account, args)
			if result != nil {
				if renderErr := options.render(result, spec.Columns); renderErr != nil {
					return renderErr
				}
			}
			return err
		},
	}
	if spec.Kind == kindDestructive {
		command.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	}
	if spec.Flags != nil {
		spec.Flags(command)
	}
	annotate(command, spec.Kind)
	return command
}

type exitCoder interface{ ExitCode() int }

type partialFailureError struct{ message string }

func (err *partialFailureError) Error() string { return err.message }
func (err *partialFailureError) ExitCode() int { return 2 }

func ExitCode(err error) int {
	var coded exitCoder
	if errors.As(err, &coded) {
		return coded.ExitCode()
	}
	return 1
}

func newGroup(use, short string, aliases []string, options *globalOptions, specs ...operationSpec) *cobra.Command {
	group := &cobra.Command{Use: use, Short: short, Aliases: aliases}
	for _, spec := range specs {
		group.AddCommand(newOperationCommand(options, spec))
	}
	return group
}

func requireID(value, flag string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("%s is required; set it on the account or pass --%s", strings.ReplaceAll(flag, "-", " "), flag)
	}
	return value, nil
}

func listFlags(command *cobra.Command) {
	command.Flags().Bool("all", false, "fetch every cursor page")
	command.Flags().Int("limit", 0, "items per page")
	command.Flags().String("after", "", "start after this cursor")
	command.Flags().String("fields", "", "comma-separated field projection")
}

func listValues(command *cobra.Command) (bool, int, string, string) {
	all, _ := command.Flags().GetBool("all")
	limit, _ := command.Flags().GetInt("limit")
	after, _ := command.Flags().GetString("after")
	fields, _ := command.Flags().GetString("fields")
	return all, limit, after, fields
}

func decodeResponse(response *api.Response) (any, error) {
	if len(response.Body) == 0 {
		return map[string]any{"success": true}, nil
	}
	var value any
	if err := json.Unmarshal(response.Body, &value); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return value, nil
}

func uploadResponse(command *cobra.Command, client *api.Client, request api.Request) (any, error) {
	response, err := client.Do(command.Context(), request)
	if err != nil {
		return nil, err
	}
	return decodeResponse(response)
}

func stringField(value any, name string) string {
	object, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	text, _ := object[name].(string)
	return text
}
