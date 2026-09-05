//go:build linux

package compute

import "syscall"

// defaultStatfs devolve a capacidade de um path em bytes (total/free/available)
// via syscall.Statfs e false quando o mount point não é statfs-ável NESTE
// ambiente. Todos os campos são convertidos para BYTES.
func defaultStatfs(path string) (statfsInfo, bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return statfsInfo{}, false
	}
	if st.Bsize <= 0 {
		return statfsInfo{}, false
	}
	bs := uint64(st.Bsize)
	return statfsInfo{
		TotalBytes:     st.Blocks * bs,
		FreeBytes:      st.Bfree * bs,
		AvailableBytes: st.Bavail * bs,
	}, true
}
