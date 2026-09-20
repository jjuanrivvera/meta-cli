//go:build linux

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
	how := &unix.OpenHow{
		Flags:   uint64(unix.O_RDONLY | unix.O_CLOEXEC),
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS,
	}
	descriptor, err := unix.Openat2(int(confinement.directory.Fd()), relative, how)
	if err != nil {
		return nil, fmt.Errorf("open confined MCP file %q: %w", filePath, err)
	}
	return os.NewFile(uintptr(descriptor), filePath), nil
}
