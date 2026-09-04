//go:build windows

package reaper

import (
	"os/exec"
	"strconv"
	"strings"
)

// procAlive reports whether pid exists. On Windows there is no signal 0, so it
// queries tasklist for the PID and checks it appears in the CSV line.
func procAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	out, err := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/FO", "CSV", "/NH").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "\""+strconv.Itoa(pid)+"\"")
}

// procCommandLine returns the command line of pid via PowerShell (Win32_Process).
// This is the identity source for the anti-hijack check on Windows. Windows is
// best-effort: if the query is unavailable the reaper fails closed and does not
// kill the process.
func procCommandLine(pid int) (string, error) {
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		"Get-CimInstance Win32_Process -Filter \"ProcessId = "+strconv.Itoa(pid)+"\" | Select-Object -ExpandProperty CommandLine").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// procStop issues a best-effort graceful tree kill (taskkill /T without /F). The
// distinction between SIGTERM and SIGKILL is blurred on Windows, so the forced
// step always follows and is the authoritative cleanup.
func procStop(pid int) error {
	return exec.Command("taskkill", "/T", "/PID", strconv.Itoa(pid)).Run()
}

// procKill issues a forced tree kill (taskkill /T /F /PID). This mirrors the
// processutil fallback and closes the grandchild race.
func procKill(pid int) error {
	return exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run()
}
