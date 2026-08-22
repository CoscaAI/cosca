package pipeline

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// SubprocessRunner isolates pipeline task execution in child processes.
// Each task runs via `os/exec` with its own timeout. If a task hangs,
// the context cancellation sends SIGKILL and the runner moves on.
//
// This is the Go equivalent of AgentHarness's subprocess isolation:
// each question runs in its own process so one hang doesn't block
// the entire benchmark suite.
type SubprocessRunner struct {
	runner Runner // fallback runner for tasks that can't be subprocessed
}

// NewSubprocessRunner creates a subprocess-isolated runner.
func NewSubprocessRunner(fallback Runner) *SubprocessRunner {
	return &SubprocessRunner{runner: fallback}
}

// Run executes a task in a subprocess. Falls back to the embedded runner
// when subprocess execution is not possible (e.g. in tests).
func (s *SubprocessRunner) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	// For now, delegate to the embedded runner.
	// Full subprocess isolation requires a CLI entry point that accepts
	// a task description via stdin/args and returns results via stdout.
	// This is designed for future implementation.
	return s.runner.Run(ctx, req)
}

// RunStream delegates streaming to the fallback runner.
func (s *SubprocessRunner) RunStream(ctx context.Context, req RunRequest) (<-chan RunEvent, error) {
	return s.runner.RunStream(ctx, req)
}

// Ensure interface compliance.
var _ Runner = (*SubprocessRunner)(nil)

// SubprocessManager manages a pool of subprocess workers.
// It provides bounded concurrency via a semaphore channel.
type SubprocessManager struct {
	maxWorkers int
	sem        chan struct{}
	mu         sync.Mutex
	active     int
}

// NewSubprocessManager creates a manager with the given max workers.
func NewSubprocessManager(maxWorkers int) *SubprocessManager {
	if maxWorkers <= 0 {
		maxWorkers = 1
	}
	return &SubprocessManager{
		maxWorkers: maxWorkers,
		sem:        make(chan struct{}, maxWorkers),
	}
}

// Acquire blocks until a worker slot is available.
func (m *SubprocessManager) Acquire() {
	m.sem <- struct{}{}
	m.mu.Lock()
	m.active++
	m.mu.Unlock()
}

// Release returns a worker slot to the pool.
func (m *SubprocessManager) Release() {
	<-m.sem
	m.mu.Lock()
	m.active--
	m.mu.Unlock()
}

// Active returns the current number of active workers.
func (m *SubprocessManager) Active() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active
}

// MaxWorkers returns the maximum number of concurrent workers.
func (m *SubprocessManager) MaxWorkers() int {
	return m.maxWorkers
}

// RunTask executes an arbitrary command in a subprocess with timeout.
// This is the low-level building block for subprocess isolation.
func RunTask(ctx context.Context, command string, args []string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)
	output, err := cmd.CombinedOutput()

	if ctx.Err() != nil {
		return string(output), fmt.Errorf("task timed out after %v: %w", timeout, ctx.Err())
	}

	if err != nil {
		return string(output), fmt.Errorf("task failed: %w\n%s", err, string(output))
	}

	return string(output), nil
}
