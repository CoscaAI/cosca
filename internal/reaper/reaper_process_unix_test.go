//go:build unix

package reaper

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

// The helper marker must survive on the helper's command line so identity-match
// has a real, distinctive fragment to look for.
const reaperHelperMarker = "reaper-antihijack-marker-8f3d19c2"

// init implements the controlled child fixture: when the test binary is
// re-executed with REAPER_HELPER set, it blocks in a long sleep so the reaper
// has a real, identifiable process to verify and (if it matches) dispose.
func init() {
	if os.Getenv("REAPER_HELPER") != "" {
		// A Go test binary with no signal.Notify for SIGTERM/SIGKILL terminates
		// by default, so the reaper's stop→kill escalation can actually end it.
		time.Sleep(60 * time.Second)
		os.Exit(0)
	}
}

// startHelper spawns a long-lived child whose command line carries
// reaperHelperMarker and returns the child cmd (so the test can wait/reap it).
func startHelper(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$", "--", reaperHelperMarker)
	cmd.Env = append(os.Environ(), "REAPER_HELPER=1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("spawn helper: %v", err)
	}
	return cmd
}

func pidStr(cmd *exec.Cmd) string { return strconv.Itoa(cmd.Process.Pid) }

func TestProcessReaperReapsMatchedPid(t *testing.T) {
	cmd := startHelper(t)
	pid := pidStr(cmd)
	// Reap it as a side effect of the test to avoid a leaked sleeping process.
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	rp := ProcessReaper()
	out, err := rp.Reap(context.Background(), Entry{Kind: KindProcess, ID: pid, Match: reaperHelperMarker}, ReapContext{})
	if err != nil {
		t.Fatalf("Reap error: %v", err)
	}
	if out.Status != StatusReaped {
		t.Fatalf("status = %s, want reaped (%s)", out.Status, out.Reason)
	}
	// The child must actually be gone now.
	if err := cmd.Wait(); err == nil {
		t.Fatalf("expected a non-nil (signal) error from Wait — process should have been terminated")
	}
	if procAlive(cmd.Process.Pid) {
		t.Fatalf("matched process %d still alive after reaping", cmd.Process.Pid)
	}
}

func TestProcessReaperAntiHijack(t *testing.T) {
	cmd := startHelper(t)
	pid := pidStr(cmd)
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	// The recorded match does NOT belong to this command line → the reaper must
	// refuse to kill it (anti-hijack: never kill a process that is not its own).
	rp := ProcessReaper()
	out, err := rp.Reap(context.Background(), Entry{Kind: KindProcess, ID: pid, Match: "some-other-process-signature"}, ReapContext{})
	if err != nil {
		t.Fatalf("Reap error: %v", err)
	}
	if out.Status != StatusSkipped || out.Reason != "identity mismatch" {
		t.Fatalf("status = %s, reason = %q, want skipped/identity mismatch", out.Status, out.Reason)
	}
	if !procAlive(cmd.Process.Pid) {
		t.Fatalf("process %d was killed despite the identity mismatch", cmd.Process.Pid)
	}
}

func TestProcessReaperMissingPid(t *testing.T) {
	// A PID that does not exist and cannot be claimed → missing.
	rp := ProcessReaper()
	out, err := rp.Reap(context.Background(), Entry{Kind: KindProcess, ID: "99999999", Match: "x"}, ReapContext{})
	if err != nil {
		t.Fatalf("Reap error: %v", err)
	}
	if out.Status != StatusMissing {
		t.Fatalf("status = %s, want missing", out.Status)
	}
}
