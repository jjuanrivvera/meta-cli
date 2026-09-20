package commands

import "github.com/spf13/cobra"

const (
	kindRead        = "read"
	kindWrite       = "write"
	kindDestructive = "destructive"
)

func annotate(command *cobra.Command, kind string) {
	if command.Annotations == nil {
		command.Annotations = map[string]string{}
	}
	switch kind {
	case kindRead:
		command.Annotations["readOnlyHint"] = "true"
		command.Annotations["openWorldHint"] = "true"
	case kindWrite:
		command.Annotations["openWorldHint"] = "true"
		command.Annotations["idempotentHint"] = "false"
	case kindDestructive:
		command.Annotations["destructiveHint"] = "true"
		command.Annotations["openWorldHint"] = "true"
	}
	command.Annotations["metactlKind"] = kind
}

func AnnotationKind(command *cobra.Command) string {
	if command.Annotations == nil {
		return ""
	}
	return command.Annotations["metactlKind"]
}

func markLocal(command *cobra.Command) {
	if command.Annotations == nil {
		command.Annotations = map[string]string{}
	}
	command.Annotations["metactlLocal"] = "true"
}
