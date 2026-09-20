//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd

package commands

import (
	"fmt"
	"os"
)

func secureOpenUnderRoot(_ *mcpFileConfinement, filePath string) (*os.File, error) {
	return nil, fmt.Errorf("atomic MCP file confinement is unavailable on this platform; refusing to open %q", filePath)
}
