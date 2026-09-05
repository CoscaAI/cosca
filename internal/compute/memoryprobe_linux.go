//go:build linux

package compute

import "syscall"

// defaultRlimitAS devolve o limite atual de endereço (RLIMIT_AS) em bytes e
// true quando o limite é finito (cur != RLIM_INFINITY). false = sem limite
// observável ou getrlimit indisponível.
func defaultRlimitAS() (uint64, bool) {
	var lim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_AS, &lim); err != nil {
		return 0, false
	}
	if lim.Cur == ^uint64(0) { // RLIM_INFINITY — sem limite
		return 0, false
	}
	return lim.Cur, true
}
