//go:build unix

package reaper

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// procAlive reports whether pid exists (or is owned by another user) via signal
// 0, which only probes liveness without delivering a signal.
func procAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	switch err {
	case nil:
		return true
	case syscall.EPERM:
		// Exists but not ours; treat as alive so the reaper does not mistake it.
		return true
	case syscall.ESRCH:
		return false
	default:
		return false
	}
}

// procCommandLine returns the full command line of pid via `ps`. It is the
// identity source for the anti-hijack check: the reaper compares the recorded
// match against this.
func procCommandLine(pid int) (string, error) {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}

// procStop sends SIGTERM to the process group (if pid is a group leader) and to
// the PID itself, best-effort. Killing the group reaps children that were set
// into their own group by processutil; a non-leader PID safely hits ESRCH.
func procStop(pid int) error {
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	return syscall.Kill(pid, syscall.SIGTERM)
}

// procKill sends SIGKILL to the process group and the PID itself, best-effort.
func procKill(pid int) error {
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	return syscall.Kill(pid, syscall.SIGKILL)
}
