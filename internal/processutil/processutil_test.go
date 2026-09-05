package processutil

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// init runs helper child modes when the test binary is re-executed as a
// controlled fixture. This gives deterministic, fast, cross-platform timing
// tests without depending on Gradle, `sleep`, or slow interpreter startup.
func init() {
	if mode := os.Getenv("PROCESSUTIL_HELPER"); mode != "" {
		runHelper(mode)
	}
}

// runHelper is the fixture entry point for child processes. It must not return:
// every branch either completes and exits, or sits in a controlled long/short
// wait to exercise idle/hard timeouts.
func runHelper(mode string) {
	switch mode {
	case "periodic":
		// Emits output faster than IdleTimeout so it must NOT be idle-cancelled.
		for i := 0; i < 6; i++ {
			fmt.Fprintf(os.Stdout, "tick %d\n", i)
			time.Sleep(60 * time.Millisecond)
		}
		os.Exit(0)
	case "stderr":
		// Proves stderr activity also refreshes lastActivity.
		for i := 0; i < 6; i++ {
			fmt.Fprintf(os.Stderr, "err %d\n", i)
			time.Sleep(50 * time.Millisecond)
		}
		os.Exit(0)
	case "silent":
		// No output at all; only ends via idle/hard/cancel.
		time.Sleep(60 * time.Second)
		os.Exit(0)
	case "failfast":
		os.Exit(7)
	case "orphan":
		runOrphanHelper()
	case "orphanchild":
		pid := os.Getpid()
		if pf := os.Getenv("PROCESSUTIL_PIDFILE"); pf != "" {
			_ = os.WriteFile(pf, []byte(strconv.Itoa(pid)), 0o644)
		}
		// Sleep holding the inherited output pipe so the parent command appears
		// to have "completed" but its tree still holds the pipe open.
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	os.Exit(1)
}

// runOrphanHelper spawns a grandchild that outlives the parent and keeps the
// output pipe open, then the parent exits. It reproduces the daemonised-child
// bug: the command's direct process finishes, but a descendant holds the pipe.
func runOrphanHelper() {
	// Let the parent-command get assigned to the process-tree unit (Job Object
	// on Windows / group on Unix) before spawning, so the descendant is inside it.
	time.Sleep(500 * time.Millisecond)
	child := exec.Command(os.Args[0], "-test.run=^$")
	child.Env = append(os.Environ(), "PROCESSUTIL_HELPER=orphanchild")
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	if err := child.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "orphan spawn failed: %v\n", err)
		os.Exit(2)
	}
	_ = child.Process.Release()
	fmt.Println("SPAWNED")
	os.Exit(0)
}

// helperCmd builds an exec.Cmd that re-executes the test binary in the given
// helper mode.
func helperCmd(mode string, extraEnv ...string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), append([]string{
		"PROCESSUTIL_HELPER=" + mode,
	}, extraEnv...)...)
	return cmd
}

// ─── success / failure / cancel ──────────────────────────────────────────────

func TestRunSuccess(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^$", "-test.count=1")
	cmd.Env = append(os.Environ(), "PROCESSUTIL_HELPER=periodic")
	res, err := Run(context.Background(), cmd, Config{IdleTimeout: 2 * time.Second, MaxRuntime: 10 * time.Second})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("expected success, got %s (stderr=%q)", res.Status, res.Stderr)
	}
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", res.ExitCode)
	}
	if res.Stdout == "" {
		t.Fatalf("expected captured stdout, got empty")
	}
}

func TestRunCommandFailure(t *testing.T) {
	cmd := helperCmd("failfast")
	res, err := Run(context.Background(), cmd, Config{IdleTimeout: 2 * time.Second, MaxRuntime: 10 * time.Second})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if res.Status != StatusCommandFailure {
		t.Fatalf("expected command_failure, got %s", res.Status)
	}
	if res.ExitCode != 7 {
		t.Fatalf("expected exit 7, got %d", res.ExitCode)
	}
}

func TestRunCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := helperCmd("silent")
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()
	res, err := Run(ctx, cmd, Config{IdleTimeout: 60 * time.Second, MaxRuntime: 60 * time.Second})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if res.Status != StatusCancelled {
		t.Fatalf("expected cancelled, got %s", res.Status)
	}
	if res.Duration < 100*time.Millisecond {
		t.Fatalf("expected duration >= ~150ms, got %v", res.Duration)
	}
}

// ─── idle vs hard timeouts ───────────────────────────────────────────────────

func TestRunIdleTimeoutGivenSilence(t *testing.T) {
	cmd := helperCmd("silent")
	res, err := Run(context.Background(), cmd, Config{IdleTimeout: 300 * time.Millisecond, MaxRuntime: 60 * time.Second})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if res.Status != StatusIdleTimeout {
		t.Fatalf("expected idle_timeout, got %s", res.Status)
	}
	if res.IdleFor <= 0 {
		t.Fatalf("expected IdleFor > 0, got %v", res.IdleFor)
	}
}

func TestRunPeriodicOutputNotCancelled(t *testing.T) {
	cmd := helperCmd("periodic")
	// IdleTimeout is deliberately generous: under `-race` the test binary is
	// instrumented and the gap between fixture ticks can grow well past the raw
	// 60ms sleep, but real output still arrives long before 5s.
	res, err := Run(context.Background(), cmd, Config{IdleTimeout: 5 * time.Second, MaxRuntime: 10 * time.Second})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("expected success (output kept it alive), got %s", res.Status)
	}
}

func TestRunStderrActivityRefreshesIdle(t *testing.T) {
	cmd := helperCmd("stderr")
	res, err := Run(context.Background(), cmd, Config{IdleTimeout: 5 * time.Second, MaxRuntime: 10 * time.Second})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("expected success (stderr kept it alive), got %s", res.Status)
	}
	if res.Stderr == "" {
		t.Fatalf("expected captured stderr, got empty")
	}
}

func TestRunHardTimeoutGivenMaxRuntime(t *testing.T) {
	cmd := helperCmd("silent")
	res, err := Run(context.Background(), cmd, Config{IdleTimeout: 30 * time.Second, MaxRuntime: 400 * time.Millisecond})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if res.Status != StatusHardTimeout {
		t.Fatalf("expected hard_timeout, got %s", res.Status)
	}
	if res.Duration < 400*time.Millisecond {
		t.Fatalf("expected duration >= MaxRuntime(~400ms), got %v", res.Duration)
	}
}

func TestRunTimeoutConfigDefaults(t *testing.T) {
	// Zero config must be filled with the documented defaults.
	c := Config{}.withDefaults()
	if c.IdleTimeout != DefaultIdleTimeout {
		t.Fatalf("expected default idle %v, got %v", DefaultIdleTimeout, c.IdleTimeout)
	}
	if c.MaxRuntime != DefaultMaxRuntime {
		t.Fatalf("expected default max %v, got %v", DefaultMaxRuntime, c.MaxRuntime)
	}
}

// ─── whole-tree termination (no orphaned descendants) ────────────────────────

func TestRunKillsTreeWithSurvivingChild(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	cmd := helperCmd("orphan", "PROCESSUTIL_PIDFILE="+pidFile)

	res, err := Run(context.Background(), cmd, Config{IdleTimeout: 2 * time.Second, MaxRuntime: 15 * time.Second})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// The parent's direct process exits with 0, but a descendant keeps the pipe
	// open in silence, so the run must be cut short by the idle monitor, not
	// reported as a clean success (which would mean we returned before the tree
	// closed the pipe).
	if res.Status != StatusIdleTimeout {
		t.Fatalf("expected idle_timeout (hang cut short), got %s (stdout=%q)", res.Status, res.Stdout)
	}
	// CRLF / LF safe: Windows Go may or may not translate '\n' to '\r\n'.
	if strings.TrimSpace(res.Stdout) != "SPAWNED" {
		t.Fatalf("expected captured 'SPAWNED', got %q", res.Stdout)
	}

	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("child did not write pid file: %v", err)
	}
	childPid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatalf("invalid child pid %q: %v", string(data), err)
	}
	if pidAlive(childPid) {
		t.Fatalf("child pid %d still alive — the process tree leaked an orphan", childPid)
	}
}

// pidAlive reports whether a process with the given PID is still running.
// Implementations are platform-specific (see pid_alive_*_test.go).

func TestConfigureProcessGroupWiresCancel(t *testing.T) {
	cmd := helperCmd("silent")
	kill, release := ConfigureProcessGroup(cmd)
	defer release()
	if kill == nil {
		t.Fatalf("expected a non-nil kill function")
	}
	if cmd.Cancel == nil {
		t.Fatalf("ConfigureProcessGroup should leave cmd.Cancel set")
	}
}
