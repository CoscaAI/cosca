// Package processutil provides a reusable process executor that:
//
//   - streams stdout/stderr chunk-by-chunk instead of buffering after the fact,
//   - detects idle output based on real activity and cancels after an
//     independent IdleTimeout,
//   - enforces a hard MaxRuntime bound that is independent of idle,
//   - terminates the WHOLE process tree (including grandchildren), so a
//     command that leaves a background daemon holding its output pipe open can
//     neither hang the caller nor leak orphaned processes.
//
// It is platform-aware: on Unix it puts the command in its own POSIX process
// group and SIGKILLs the group; on Windows it uses a Job Object with
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE and falls back to `taskkill /T /F /PID`.
//
// This package owns only the execution layer. It never touches UI concerns and
// preserves the security invariants (allowlist env, workspace rails, fail-closed
// sandbox gate) that stay in the sandbox layer above it.
package processutil

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"sync"
	"time"
)

// Default timeouts applied when the corresponding Config field is zero.
const (
	// DefaultIdleTimeout is how long a command may run without emitting any
	// stdout or stderr chunk before it is cancelled as idle.
	DefaultIdleTimeout = 120 * time.Second
	// DefaultMaxRuntime is the hard upper bound on a command's lifetime,
	// regardless of activity.
	DefaultMaxRuntime = 30 * time.Minute
)

// Config centralises the execution timing knobs. Both limits are independent:
// IdleTimeout cancels when no output chunk is observed for the duration;
// MaxRuntime is a hard bound regardless of activity.
type Config struct {
	// IdleTimeout is how long the process may run without producing any
	// stdout OR stderr chunk before it is cancelled as idle. Zero uses
	// DefaultIdleTimeout.
	IdleTimeout time.Duration
	// MaxRuntime is the hard upper bound on the process's lifetime. Zero uses
	// DefaultMaxRuntime.
	MaxRuntime time.Duration
}

// withDefaults returns a copy of c with zero fields filled in.
func (c Config) withDefaults() Config {
	if c.IdleTimeout <= 0 {
		c.IdleTimeout = DefaultIdleTimeout
	}
	if c.MaxRuntime <= 0 {
		c.MaxRuntime = DefaultMaxRuntime
	}
	return c
}

// Status is the semantic outcome of a run.
type Status string

const (
	// StatusSuccess: the process exited 0 and its output pipes closed cleanly.
	StatusSuccess Status = "success"
	// StatusCommandFailure: the process exited with a non-zero code.
	StatusCommandFailure Status = "command_failure"
	// StatusCancelled: the caller's context was cancelled.
	StatusCancelled Status = "cancelled"
	// StatusIdleTimeout: no output was produced for IdleTimeout.
	StatusIdleTimeout Status = "idle_timeout"
	// StatusHardTimeout: the command ran past MaxRuntime.
	StatusHardTimeout Status = "hard_timeout"
)

// Result is the outcome of running a command.
type Result struct {
	// Stdout is the captured standard output.
	Stdout string
	// Stderr is the captured standard error.
	Stderr string
	// ExitCode is the process exit code. Zero indicates success.
	ExitCode int
	// Duration is the wall-clock time the command took.
	Duration time.Duration
	// Status is the semantic outcome.
	Status Status
	// IdleFor is how long the command had been silent when it was cancelled as
	// idle. Populated only when Status == StatusIdleTimeout.
	IdleFor time.Duration
	// Err is non-nil when the process could not be reaped or failed to start
	// (an infrastructure error, not a normal exit code).
	Err error
}

// safeBuffer is a concurrency-safe output buffer that timestamps every write so
// the idle monitor can measure real output activity rather than buffering plus
// an end-of-run diff.
type safeBuffer struct {
	mu   sync.Mutex
	buf  bytes.Buffer
	last time.Time
}

func (b *safeBuffer) reset(start time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
	b.last = start
}

// Write implements io.Writer and records the activity timestamp.
func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.last = time.Now()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *safeBuffer) lastActivity() time.Time {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.last
}

// reason records the FIRST event that ended the run, so a timeout/cancel that
// races the process's own exit is reported truthfully instead of being mistaken
// for a success or a plain command failure.
type reason struct {
	mu      sync.Mutex
	status  Status
	idleFor time.Duration
	set     bool
}

// trigger records the reason only on the first call; it returns false if a
// reason was already recorded.
func (r *reason) trigger(s Status, idleFor time.Duration) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.set {
		return false
	}
	r.status = s
	r.idleFor = idleFor
	r.set = true
	return true
}

func (r *reason) get() (Status, time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status, r.idleFor
}

// Run executes cmd with streaming stdout/stderr capture, idle detection, an
// independent hard timeout, and whole-tree termination.
//
// Run owns cancellation through its idle/hard monitor: the caller's context is
// tracked and, when done, the monitor kills the whole tree. A deadline on the
// context is reported as hard_timeout, a manual cancel as cancelled.
//
// Run does not set cmd.Cancel — the monitor is the single cancellation owner,
// so cmd may be created with either exec.Command or exec.CommandContext. (When
// the caller supplies a CommandContext command, exec may ALSO fire its own
// default direct-process kill on cancel; the monitor's tree kill still runs and
// the status is reconciled from ctx.Err().)
//
// Run does not return until the process tree has released its output pipes, so
// it cannot leak orphaned grandchildren. A non-nil Err in the returned Result
// (or a non-nil error) is reserved for start/wait infrastructure failures. A
// normal non-zero exit code is delivered through ExitCode with Status ==
// command_failure, not through the error.
func Run(ctx context.Context, cmd *exec.Cmd, cfg Config) (*Result, error) {
	cfg = cfg.withDefaults()
	start := time.Now()

	tk := newTreeKiller(cmd)
	kill := func() error {
		tk.attach()
		return tk.kill()
	}

	var stdout, stderr safeBuffer
	stdout.reset(start)
	stderr.reset(start)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	idleFor := func() time.Duration {
		so := stdout.lastActivity()
		se := stderr.lastActivity()
		if se.After(so) {
			return time.Since(se)
		}
		return time.Since(so)
	}

	if err := cmd.Start(); err != nil {
		return &Result{
			Status:   StatusCommandFailure,
			Duration: time.Since(start),
			Err:      err,
		}, err
	}
	// Bind the running process to the tree-kill unit (e.g. the Windows Job
	// Object) as soon as possible to shrink the assignment race.
	tk.attach()

	var rs reason
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				idle := idleFor()
				if ctx.Err() == context.DeadlineExceeded {
					rs.trigger(StatusHardTimeout, idle)
				} else {
					rs.trigger(StatusCancelled, idle)
				}
				kill()
				return
			case <-ticker.C:
				idle := idleFor()
				if cfg.IdleTimeout > 0 && idle > cfg.IdleTimeout {
					if rs.trigger(StatusIdleTimeout, idle) {
						kill()
					}
					return
				}
				if cfg.MaxRuntime > 0 && time.Since(start) > cfg.MaxRuntime {
					if rs.trigger(StatusHardTimeout, idle) {
						kill()
					}
					return
				}
			}
		}
	}()

	err := cmd.Wait()
	close(done)
	<-finished

	duration := time.Since(start)

	exitCode := 0
	var waitErr error
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			waitErr = err
		}
	}

	res := &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Duration: duration,
		Err:      waitErr,
	}

	status, idle := rs.get()
	switch {
	case status != "":
		res.Status = status
		res.IdleFor = idle
	case ctx.Err() == context.DeadlineExceeded:
		// exec's watchCtx may have driven the kill (via cmd.Cancel) before the
		// monitor recorded a reason; a done deadline is still a hard timeout.
		res.Status = StatusHardTimeout
		res.IdleFor = idle
	case ctx.Err() == context.Canceled:
		res.Status = StatusCancelled
		res.IdleFor = idle
	case waitErr != nil:
		res.Status = StatusCommandFailure
	case exitCode == 0:
		res.Status = StatusSuccess
	default:
		res.Status = StatusCommandFailure
	}

	return res, waitErr
}

// ConfigureProcessGroup wires cmd so that the whole process tree can be killed
// as a unit. It installs the platform-specific SysProcAttr, sets cmd.Cancel to
// a function that terminates the whole tree, and returns a release function
// that frees any OS resources (e.g. the Windows Job Object handle). The release
// function must be called once the command has finished to avoid resource leaks.
//
// Because cmd.Cancel is set, cmd must be created via exec.CommandContext (as the
// evals verify runner does); exec.Cmd.Start rejects a non-nil Cancel on a
// command not created with a context.
func ConfigureProcessGroup(cmd *exec.Cmd) (kill func() error, release func()) {
	tk := newTreeKiller(cmd)
	kill = func() error {
		tk.attach()
		return tk.kill()
	}
	cmd.Cancel = kill
	return kill, tk.release
}
