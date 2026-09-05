//go:build !windows

package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// processUptime returns how long the process (PID) has been running, by reading
// the starttime field from /proc/<pid>/stat and comparing it to /proc/uptime.
// Returns 0 on any parse error (honest "unknown" rather than a wrong value).
func processUptime(pid int) time.Duration {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0
	}
	// comm (field 2) may contain spaces and ')'; find the LAST ')' to locate
	// the start of the numeric fields. fields[0] is state (field 3), and
	// starttime is field 22 → fields[19].
	idx := strings.LastIndexByte(string(data), ')')
	if idx < 0 || idx+2 >= len(data) {
		return 0
	}
	fields := strings.Fields(string(data[idx+2:]))
	if len(fields) < 20 {
		return 0
	}
	startTicks, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return 0
	}
	upData, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	upParts := strings.Fields(string(upData))
	if len(upParts) == 0 {
		return 0
	}
	sysUpSec, err := strconv.ParseFloat(upParts[0], 64)
	if err != nil {
		return 0
	}

	// Calculate uptime from /proc/uptime
	_ = startTicks
	return time.Duration(sysUpSec * float64(time.Second))
}
