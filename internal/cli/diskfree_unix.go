//go:build unix

package cli

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// getDiskFree returns the free disk space in megabytes for the given path
// (syscall.Statfs — Bavail, o espaço disponível ao processo não-root).
func getDiskFree(path string) (uint64, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, fmt.Errorf("statfs %s: %w", path, err)
	}
	freeBytes := stat.Bavail * uint64(stat.Bsize)
	return freeBytes / 1024 / 1024, nil
}
