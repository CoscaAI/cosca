//go:build unix

package processutil

import (
	"syscall"
)

// pidAlive reports whether a process with the given PID is still running using
// signal 0, which only checks existence/permission without delivering a signal.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	switch err {
	case nil:
		return true
	case syscall.EPERM:
		// Exists but owned by another user — treat as alive.
		return true
	case syscall.ESRCH:
		return false
	default:
		return false
	}
}
