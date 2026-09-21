package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jjuanrivvera/meta-cli/commands"
)

func main() {
	ctx, stop := signal.NotifyContext(context.TODO(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	dependencies := commands.Dependencies{}
	originalArgs := os.Args[1:]
	args, err := commands.ExpandAliases(originalArgs, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", commands.SanitizeError(err, originalArgs, dependencies))
		os.Exit(1)
	}
	root := commands.NewRootCmd(dependencies)
	root.SetArgs(args)
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", commands.SanitizeError(err, args, dependencies))
		os.Exit(commands.ExitCode(err))
	}
}
