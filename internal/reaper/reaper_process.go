package reaper

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

// Escalation windows for the process reaper, mirroring the openwork reaper's
// SIGTERM → wait → SIGKILL ladder.
const (
	// processGraceTimeout is how long to wait after SIGTERM before escalating.
	processGraceTimeout = 5 * time.Second
	// processKillTimeout is how long to wait after SIGKILL before giving up.
	processKillTimeout = 2 * time.Second
)

// processReaper disposes a process only after verifying it still matches the
// recorded identity marker (anti-hijack), then escalates SIGTERM → SIGKILL.
type processReaper struct{}

// Kind returns KindProcess.
func (processReaper) Kind() string { return KindProcess }

// ProcessReaper returns the built-in process reaper.
func ProcessReaper() Reaper { return processReaper{} }

// Reap validates the PID, verifies liveness, confirms the command line still
// contains the recorded match, and only then stops the process (and its group)
// gracefully, escalating to a forced kill. It never kills a process whose
// command line does not match — that is the anti-hijack guarantee.
func (processReaper) Reap(ctx context.Context, e Entry, _ ReapContext) (Outcome, error) {
	pid, err := parsePID(e.ID)
	if err != nil {
		return Outcome{Status: StatusSkipped, Reason: "invalid pid"}, nil
	}
	if !procAlive(pid) {
		return Outcome{Status: StatusMissing}, nil
	}
	if e.Match == "" {
		return Outcome{Status: StatusSkipped, Reason: "no identity marker"}, nil
	}
	cmdline, err := procCommandLine(pid)
	if err != nil || strings.TrimSpace(cmdline) == "" {
		// Fail closed: we cannot verify the person, so we do not kill it.
		return Outcome{Status: StatusSkipped, Reason: "identity unavailable"}, nil
	}
	if !identityMatch(cmdline, e.Match) {
		return Outcome{Status: StatusSkipped, Reason: "identity mismatch"}, nil
	}

	// Graceful stop (group + PID, best-effort), then wait, then escalate.
	_ = procStop(pid)
	if procWaitExit(pid, processGraceTimeout) {
		return Outcome{Status: StatusReaped}, nil
	}
	_ = procKill(pid)
	if procWaitExit(pid, processKillTimeout) {
		return Outcome{Status: StatusReaped}, nil
	}
	return Outcome{Status: StatusSkipped, Reason: "still alive"}, nil
}

// parsePID validates that s is a strictly-numeric, positive, in-range PID.
func parsePID(s string) (int, error) {
	if s == "" {
		return 0, errors.New("empty pid")
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, errors.New("non-numeric pid")
		}
	}
	pid, err := strconv.Atoi(s)
	if err != nil || pid <= 0 {
		return 0, errors.New("invalid pid")
	}
	return pid, nil
}

// identityMatch reports whether cmdline still carries the recorded match marker.
func identityMatch(cmdline, match string) bool {
	if match == "" {
		return false
	}
	return strings.Contains(cmdline, match)
}

// procWaitExit polls procAlive until pid exits or the timeout elapses.
func procWaitExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !procAlive(pid) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return !procAlive(pid)
}
