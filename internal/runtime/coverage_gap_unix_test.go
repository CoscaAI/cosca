//go:build unix

// Package runtime — testes de coverage que exigem syscalls Unix
// (kill/sinal para o próprio processo e RLIMIT_FSIZE). Separados do
// coverage_gap_test.go (portátil) para o pacote compilar no Windows.
package runtime

import (
	"context"
	"syscall"
	"testing"
	"time"
)

// =============================================================================
// runtime.HandleSignals — 95.0% → target 100%
// =============================================================================

// TestHandleSignals_StopError covers runtime.go:468-470.
// Sends SIGINT while the runtime is in Error state.
// The signal handler goroutine calls r.Stop() which fails because
// Error → Stopping is an invalid transition.
func TestHandleSignals_StopError(t *testing.T) {
	r := newTestRuntime()
	ctx := context.Background()

	// Start to reach Running state.
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Register signal handlers.
	r.HandleSignals()

	// Corrupt state to Uninitialized — Stop's TransitionTo(Stopping) will fail.
	// StateUninitialized → Stopping is the only invalid transition that Stop doesn't
	// catch with its early-return check (StateStopped/Stopping return nil early).
	r.state.mu.Lock()
	r.state.CurrentState = StateUninitialized
	r.state.mu.Unlock()

	// Send SIGINT to the current process.
	// The handler goroutine receives it and launches r.Stop() which errors.
	syscall.Kill(syscall.Getpid(), syscall.SIGINT)

	// Wait for the goroutine to process the signal.
	time.Sleep(200 * time.Millisecond)

	// Cleanup: cancel the runtime context to stop the signal handler goroutine.
	r.cancel()
	time.Sleep(50 * time.Millisecond)
}

// =============================================================================
// daemon.writePIDFile — 89.5% → 100% via RLIMIT_FSIZE
// =============================================================================

// TestWritePIDFile_FprintfError covers daemon.go:218-221.
// Uses RLIMIT_FSIZE=1 to trigger EFBIG on Fprintf.
func TestWritePIDFile_FprintfError(t *testing.T) {
	// Save and restore the file size limit.
	var oldLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &oldLimit); err != nil {
		t.Skipf("getrlimit failed: %v", err)
	}
	defer syscall.Setrlimit(syscall.RLIMIT_FSIZE, &oldLimit)

	// Set file size limit to 1 byte — Fprintf("%d\n", pid) will fail with EFBIG.
	newLimit := syscall.Rlimit{Cur: 1, Max: oldLimit.Max}
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &newLimit); err != nil {
		t.Skipf("setrlimit failed: %v", err)
	}

	pidDir := t.TempDir()
	pidFile := pidDir + "/cosca.pid"

	r := newTestRuntime()
	d := NewDaemon(r, DaemonConfig{PIDPath: pidFile})

	// Call writePIDFile directly (same package). With RLIMIT_FSIZE=1,
	// Fprintf will fail with EFBIG after writing ~1 byte of the PID string.
	err := d.writePIDFile()
	if err == nil {
		t.Fatal("writePIDFile should fail with RLIMIT_FSIZE=1")
	}
}
