//go:build windows

package cli

import (
	"time"
)

// processUptime returns how long the process (PID) has been running.
// On Windows, this is a stub that returns 0 since /proc is not available.
// A proper implementation would use the Windows API (GetProcessTimes).
func processUptime(pid int) time.Duration {
	// TODO: Implement using Windows API (GetProcessTimes)
	// For now, return 0 to indicate unknown uptime.
	return 0
}
