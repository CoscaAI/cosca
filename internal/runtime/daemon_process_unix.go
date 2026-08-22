//go:build !windows

package runtime

import (
	"os"
	"syscall"
)

// processRunningPlatform checks if a process with the given PID is running.
// On Unix, FindProcess always succeeds (never returns error), so the error
// check is intentionally omitted. Use signal 0 to verify process existence.
func processRunningPlatform(pid int) bool {
	process, _ := os.FindProcess(pid)
	// Send signal 0 to check if the process is alive.
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	return true
}
