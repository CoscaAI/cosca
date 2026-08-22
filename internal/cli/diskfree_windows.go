//go:build windows

package cli

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// getDiskFree returns the free disk space in megabytes for the given path.
// Win32: GetDiskFreeSpaceEx — lpFreeBytesAvailableToCaller (espaço disponível
// ao chamador, equivalente ao Bavail do statfs).
func getDiskFree(path string) (uint64, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, fmt.Errorf("statfs %s: %w", path, err)
	}
	var freeBytesAvailable uint64
	if err := windows.GetDiskFreeSpaceEx(pathPtr, &freeBytesAvailable, nil, nil); err != nil {
		return 0, fmt.Errorf("statfs %s: %w", path, err)
	}
	return freeBytesAvailable / 1024 / 1024, nil
}
