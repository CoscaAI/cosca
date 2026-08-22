//go:build windows

package runtime

import "syscall"

// processRunningPlatform checks if a process with the given PID is running.
//
// Windows has no "signal 0" probe and os.Process.Signal is only implemented
// for Kill. Worse, os.FindProcess returns (nil, error) for a PID that does
// not exist, so the Unix pattern (FindProcess + Signal(0)) would nil-deref.
// Process existence is probed directly via OpenProcess:
//   - success        → the process exists;
//   - ACCESS_DENIED  → the process exists but belongs to another user /
//     integrity level (still "running" for our purposes);
//   - any other error (e.g. INVALID_PARAMETER) → no such process.
func processRunningPlatform(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		return err == syscall.ERROR_ACCESS_DENIED
	}
	_ = syscall.CloseHandle(handle)
	return true
}
