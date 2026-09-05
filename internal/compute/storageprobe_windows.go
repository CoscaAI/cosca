//go:build windows

package compute

import "golang.org/x/sys/windows"

// defaultStatfs devolve a capacidade de um path em bytes via
// GetDiskFreeSpaceEx (Win32): TotalBytes = total do volume,
// FreeBytes = total de bytes livres no volume, AvailableBytes = bytes
// disponíveis ao chamador (equivalente ao Bavail do statfs). false quando o
// path não é um caminho de volume resolvível. Best-effort — o probeStorage
// já registra limitação quando a capacidade não é observável.
func defaultStatfs(path string) (statfsInfo, bool) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return statfsInfo{}, false
	}
	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	if err := windows.GetDiskFreeSpaceEx(pathPtr, &freeBytesAvailable, &totalBytes, &totalFreeBytes); err != nil {
		return statfsInfo{}, false
	}
	return statfsInfo{
		TotalBytes:     totalBytes,
		FreeBytes:      totalFreeBytes,
		AvailableBytes: freeBytesAvailable,
	}, true
}
