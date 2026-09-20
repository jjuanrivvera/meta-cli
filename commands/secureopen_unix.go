//go:build darwin || freebsd || openbsd || netbsd

package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func secureOpenUnderRoot(confinement *mcpFileConfinement, filePath string) (*os.File, error) {
	relative, err := filepath.Rel(confinement.root, filePath)
	if err != nil || relative == "." || relative == ".." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("MCP file path %q is outside METACTL_MCP_ROOT", filePath)
	}
	current, err := unix.Dup(int(confinement.directory.Fd()))
	if err != nil {
		return nil, fmt.Errorf("duplicate MCP root descriptor: %w", err)
	}
	components := strings.Split(relative, string(filepath.Separator))
	for index, component := range components {
		flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW
		if index < len(components)-1 {
			flags |= unix.O_DIRECTORY
		}
		next, openErr := unix.Openat(current, component, flags, 0)
		_ = unix.Close(current)
		if openErr != nil {
			return nil, fmt.Errorf("open confined MCP file %q: %w", filePath, openErr)
		}
		current = next
	}
	return os.NewFile(uintptr(current), filePath), nil
}
