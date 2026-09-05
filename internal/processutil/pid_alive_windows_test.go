//go:build windows

package processutil

import "golang.org/x/sys/windows"

// pidAlive reports whether a process with the given PID is still running.
//
// On Windows a terminated process (with a not-yet-reused PID) fails to open
// with ERROR_INVALID_PARAMETER, so it is reported as dead. ERROR_ACCESS_DENIED
// means the process exists but we lack the rights to query it — that is treated
// as alive.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		// ERROR_INVALID_PARAMETER returned when the process does not exist.
		if err == windows.ERROR_INVALID_PARAMETER {
			return false
		}
		// ERROR_ACCESS_DENIED returned when the process exists but we cannot
		// query it.
		if err == windows.ERROR_ACCESS_DENIED {
			return true
		}
		return false
	}
	_ = windows.CloseHandle(h)
	return true
}
