//go:build docsgen

// Command gendocs rebuilds the command reference from the live Cobra tree.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra/doc"

	"github.com/jjuanrivvera/meta-cli/commands"
)

func main() {
	outputDirectory := "docs/commands"
	if len(os.Args) > 1 {
		outputDirectory = os.Args[1]
	}
	pages, err := generate(outputDirectory)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gendocs: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("gendocs: wrote %d pages to %s\n", pages, outputDirectory)
}

func generate(outputDirectory string) (int, error) {
	if err := os.MkdirAll(outputDirectory, 0o750); err != nil { // #nosec G703 -- the operator selects the dedicated documentation directory
		return 0, err
	}
	entries, err := os.ReadDir(outputDirectory)
	if err != nil {
		return 0, err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			if err := os.Remove(filepath.Join(outputDirectory, entry.Name())); err != nil { // #nosec G703 -- only generated Markdown in the selected directory is removed
				return 0, err
			}
		}
	}

	root := commands.NewRootCmd(commands.Dependencies{})
	// Date stamps would make the generated reference drift without a command change.
	root.DisableAutoGenTag = true
	if err := doc.GenMarkdownTree(root, outputDirectory); err != nil {
		return 0, err
	}
	return countPages(outputDirectory)
}

func countPages(directory string) (int, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		count++
	}
	return count, nil
}
