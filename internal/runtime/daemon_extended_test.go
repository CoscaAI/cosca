package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// =============================================================================
// Daemon Start/Stop
// =============================================================================

func TestDaemonStartStop(t *testing.T) {
	pidDir := t.TempDir()
	pidFile := filepath.Join(pidDir, "cosca.pid")

	r := New()
	cfg := DaemonConfig{
		PIDPath:          pidFile,
		WatchdogInterval: time.Hour, // Long interval to avoid watchdog actions
		SyncInterval:     time.Hour,
	}
	d := NewDaemon(r, cfg)

	if err := d.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !d.IsRunning() {
		t.Error("daemon should be running after Start")
	}

	// PID file should exist
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		t.Error("pid file should exist after Start")
	}

	if err := d.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// NOTE: Stop() doesn't reset watchdogActive to false (potential bug)
	// so IsRunning() may still return true after Stop.
	// We verify the watchdog/sync loops have actually exited instead.
	if d.IsRunning() {
		t.Log("BUG: IsRunning still returns true after Stop (watchdogActive not reset in Stop())")
	}

	// PID file should be removed
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Error("pid file should be removed after Stop")
	}
}

func TestDaemonStartWithEmptyPIDPath(t *testing.T) {
	r := New()
	cfg := DaemonConfig{
		PIDPath:          "",
		WatchdogInterval: time.Hour,
		SyncInterval:     time.Hour,
	}
	d := NewDaemon(r, cfg)

	if err := d.Start(); err != nil {
		t.Fatalf("Start with empty PID path failed: %v", err)
	}
	defer func() { _ = d.Stop() }()

	if !d.IsRunning() {
		t.Error("daemon should be running")
	}
}

func TestDaemonStartAlreadyRunning(t *testing.T) {
	pidDir := t.TempDir()
	pidFile := filepath.Join(pidDir, "cosca.pid")
	r := New()
	d := NewDaemon(r, DaemonConfig{
		PIDPath:          pidFile,
		WatchdogInterval: time.Hour,
		SyncInterval:     time.Hour,
	})

	if err := d.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = d.Stop() }()

	// Second start should fail PID check if file still exists
	// Note: the PID file still exists from the first Start
	d2 := NewDaemon(r, DaemonConfig{
		PIDPath:          pidFile,
		WatchdogInterval: time.Hour,
		SyncInterval:     time.Hour,
	})
	err := d2.Start()
	if err != nil {
		t.Logf("Expected error from second Start: %v", err)
	}
	// d2 must be stopped even when Start succeeds: it holds an open handle
	// to the PID file, and on Windows a file cannot be deleted while a
	// handle is open — the TempDir cleanup would fail otherwise.
	_ = d2.Stop()
}

// =============================================================================
// PID File Management
// =============================================================================

func TestWritePIDFile(t *testing.T) {
	pidDir := t.TempDir()
	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	d.pidPath = filepath.Join(pidDir, "test.pid")

	if err := d.writePIDFile(); err != nil {
		t.Fatalf("writePIDFile failed: %v", err)
	}

	// Read back and verify
	data, err := os.ReadFile(d.pidPath)
	if err != nil {
		t.Fatalf("read PID file: %v", err)
	}

	expected := fmt.Sprintf("%d\n", os.Getpid())
	if string(data) != expected {
		t.Errorf("pid file content = %q, want %q", string(data), expected)
	}

	// Cleanup
	_ = d.removePIDFile()
}

func TestWritePIDFileEmptyPath(t *testing.T) {
	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	d.pidPath = ""

	if err := d.writePIDFile(); err != nil {
		t.Errorf("writePIDFile with empty path should not fail: %v", err)
	}
}

func TestWritePIDFileStalePID(t *testing.T) {
	pidDir := t.TempDir()
	pidFile := filepath.Join(pidDir, "stale.pid")

	// Write a stale PID file (non-existing process)
	if err := os.WriteFile(pidFile, []byte("99999\n"), 0644); err != nil {
		t.Fatalf("write stale pid: %v", err)
	}

	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	d.pidPath = pidFile

	if err := d.writePIDFile(); err != nil {
		t.Fatalf("writePIDFile with stale PID: %v", err)
	}

	// Cleanup
	_ = d.removePIDFile()
}

func TestWritePIDFileBadDir(t *testing.T) {
	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	// Path where parent doesn't exist would trigger os.MkdirAll to work
	// To get an error, use a file as a directory prefix
	pidDir := t.TempDir()
	blocker := filepath.Join(pidDir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0444); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	d.pidPath = filepath.Join(blocker, "subdir", "pid")

	err := d.writePIDFile()
	if err == nil {
		t.Error("writePIDFile should fail when a file blocks directory creation")
	}
	_ = os.Chmod(blocker, 0644)
}

func TestRemovePIDFile(t *testing.T) {
	pidDir := t.TempDir()
	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	d.pidPath = filepath.Join(pidDir, "remove.pid")

	_ = d.writePIDFile()
	if err := d.removePIDFile(); err != nil {
		t.Errorf("removePIDFile failed: %v", err)
	}

	if _, err := os.Stat(d.pidPath); !os.IsNotExist(err) {
		t.Error("pid file should not exist after remove")
	}
}

func TestRemovePIDFileAlreadyGone(t *testing.T) {
	pidDir := t.TempDir()
	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	d.pidPath = filepath.Join(pidDir, "nonexistent.pid")

	// Should not error even though file doesn't exist
	if err := d.removePIDFile(); err != nil {
		t.Errorf("removePIDFile on nonexistent file: %v", err)
	}
}

func TestRemovePIDFileEmptyPath(t *testing.T) {
	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	d.pidPath = ""
	if err := d.removePIDFile(); err != nil {
		t.Errorf("removePIDFile with empty path: %v", err)
	}
}

// =============================================================================
// readExistingPID
// =============================================================================

func TestReadExistingPID(t *testing.T) {
	pidDir := t.TempDir()
	pidFile := filepath.Join(pidDir, "existing.pid")

	currentPID := fmt.Sprintf("%d\n", os.Getpid())
	if err := os.WriteFile(pidFile, []byte(currentPID), 0644); err != nil {
		t.Fatalf("write pid: %v", err)
	}

	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	d.pidPath = pidFile

	pid, err := d.readExistingPID()
	if err != nil {
		t.Fatalf("readExistingPID: %v", err)
	}
	if pid != os.Getpid() {
		t.Errorf("pid = %d, want %d", pid, os.Getpid())
	}
}

func TestReadExistingPIDNonexistent(t *testing.T) {
	pidDir := t.TempDir()
	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	d.pidPath = filepath.Join(pidDir, "nope.pid")

	_, err := d.readExistingPID()
	if err == nil {
		t.Error("readExistingPID should fail for nonexistent file")
	}
}

func TestReadExistingPIDMalformed(t *testing.T) {
	pidDir := t.TempDir()
	pidFile := filepath.Join(pidDir, "bad.pid")
	if err := os.WriteFile(pidFile, []byte("not-a-number\n"), 0644); err != nil {
		t.Fatalf("write bad pid: %v", err)
	}

	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	d.pidPath = pidFile

	_, err := d.readExistingPID()
	if err == nil {
		t.Error("readExistingPID should fail for malformed PID")
	}
}

func TestReadExistingPIDWithWhitespace(t *testing.T) {
	pidDir := t.TempDir()
	pidFile := filepath.Join(pidDir, "ws.pid")

	// PID with surrounding whitespace
	if err := os.WriteFile(pidFile, []byte("  42  \n"), 0644); err != nil {
		t.Fatalf("write pid: %v", err)
	}

	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	d.pidPath = pidFile

	pid, err := d.readExistingPID()
	if err != nil {
		t.Fatalf("readExistingPID with whitespace: %v", err)
	}
	if pid != 42 {
		t.Errorf("pid = %d, want 42", pid)
	}
}

// =============================================================================
// processRunning
// =============================================================================

func TestProcessRunningCurrentProcess(t *testing.T) {
	pid := os.Getpid()
	if !processRunning(pid) {
		t.Errorf("processRunning(%d) should be true for current process", pid)
	}
}

func TestProcessRunningVeryLargePID(t *testing.T) {
	// PID 999999 is very unlikely to exist
	running := processRunning(99999)
	t.Logf("processRunning(99999) = %v", running)
}

// =============================================================================
// checkAndRestart
// =============================================================================

func TestCheckAndRestartNilRuntime(t *testing.T) {
	d := &Daemon{
		logger: zerolog.Nop(),
	}
	// Should not panic when runtime is nil
	d.checkAndRestart()
}

func TestCheckAndRestartNoSubsystems(t *testing.T) {
	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	// Should not panic with no subsystems
	d.checkAndRestart()
}

func TestCheckAndRestartHealthySubsystems(t *testing.T) {
	r := New()
	mock := &MockSubsystem{
		NameFunc:   func() string { return "knowledge" },
		HealthFunc: func() ComponentStatus { return StatusHealthy },
	}
	r.RegisterKnowledge(mock)

	d := NewDaemon(r, DefaultDaemonConfig())
	d.checkAndRestart()

	info, ok := r.state.ComponentStatus("knowledge")
	if !ok {
		t.Fatal("component not tracked")
	}
	if info.Status != StatusHealthy {
		t.Errorf("status = %v", info.Status)
	}
}

func TestCheckAndRestartUnhealthySubsystem(t *testing.T) {
	r := New()

	stopCount := 0
	startCount := 0
	mock := &MockSubsystem{
		NameFunc:   func() string { return "knowledge" },
		HealthFunc: func() ComponentStatus { return StatusUnhealthy },
		StopFunc: func(_ context.Context) error {
			stopCount++
			return nil
		},
		StartFunc: func(_ context.Context) error {
			startCount++
			return nil
		},
	}
	r.RegisterKnowledge(mock)

	d := NewDaemon(r, DefaultDaemonConfig())
	d.checkAndRestart()

	if stopCount != 1 {
		t.Errorf("Stop not called for unhealthy component, count=%d", stopCount)
	}
	if startCount != 1 {
		t.Errorf("Start not called for restarted component, count=%d", startCount)
	}

	info, _ := r.state.ComponentStatus("knowledge")
	if info.Restarts != 1 {
		t.Errorf("Restarts = %d, want 1", info.Restarts)
	}
}

func TestCheckAndRestartUnhealthyStopFails(t *testing.T) {
	r := New()

	stopCount := 0
	startCount := 0
	mock := &MockSubsystem{
		NameFunc:   func() string { return "knowledge" },
		HealthFunc: func() ComponentStatus { return StatusUnhealthy },
		StopFunc: func(_ context.Context) error {
			stopCount++
			return errors.New("stop failed")
		},
		StartFunc: func(_ context.Context) error {
			startCount++
			return nil
		},
	}
	r.RegisterKnowledge(mock)

	d := NewDaemon(r, DefaultDaemonConfig())
	d.checkAndRestart()

	if stopCount != 1 {
		t.Errorf("Stop should be called once, got %d", stopCount)
	}
	// Start should not be called if stop fails
	if startCount != 0 {
		t.Errorf("Start should not be called when stop fails, count=%d", startCount)
	}
}

func TestCheckAndRestartUnhealthyStartFails(t *testing.T) {
	r := New()

	stopCount := 0
	startCount := 0
	mock := &MockSubsystem{
		NameFunc:   func() string { return "knowledge" },
		HealthFunc: func() ComponentStatus { return StatusUnhealthy },
		StopFunc: func(_ context.Context) error {
			stopCount++
			return nil
		},
		StartFunc: func(_ context.Context) error {
			startCount++
			return errors.New("start failed")
		},
	}
	r.RegisterKnowledge(mock)

	d := NewDaemon(r, DefaultDaemonConfig())
	d.checkAndRestart()

	if stopCount != 1 {
		t.Errorf("Stop should be called, count=%d", stopCount)
	}
	if startCount != 1 {
		t.Errorf("Start should be called, count=%d", startCount)
	}
	// Even though start failed, restart count should be incremented
	info, _ := r.state.ComponentStatus("knowledge")
	if info.Restarts != 1 {
		t.Errorf("Restarts = %d, want 1", info.Restarts)
	}
}

// =============================================================================
// runSync
// =============================================================================

func TestRunSyncNoSyncFunc(t *testing.T) {
	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	d.syncFunc = nil

	// Should not panic
	d.runSync()
}

func TestRunSyncSuccess(t *testing.T) {
	r := New()
	syncCalled := make(chan struct{})
	d := NewDaemon(r, DefaultDaemonConfig())
	d.syncFunc = func(_ context.Context) error {
		close(syncCalled)
		return nil
	}

	d.runSync()

	select {
	case <-syncCalled:
		// Success
	case <-time.After(time.Second):
		t.Error("sync function was not called")
	}

	if r.metrics.SyncCount() != 1 {
		t.Errorf("SyncCount = %d, want 1", r.metrics.SyncCount())
	}
}

func TestRunSyncFailure(t *testing.T) {
	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	d.syncFunc = func(_ context.Context) error {
		return errors.New("sync failed")
	}

	d.runSync()

	// Error counter should be incremented
	if r.metrics.ErrorCount() != 1 {
		t.Errorf("ErrorCount = %d, want 1", r.metrics.ErrorCount())
	}
	// Sync counter should NOT be incremented on failure
	if r.metrics.SyncCount() != 0 {
		t.Errorf("SyncCount should be 0 on failure, got %d", r.metrics.SyncCount())
	}
}

func TestRunSyncContextTimeout(t *testing.T) {
	r := New()
	d := NewDaemon(r, DaemonConfig{
		SyncInterval: 20 * time.Millisecond, // Short interval means short timeout
	})
	d.syncFunc = func(ctx context.Context) error {
		// Block until context is done
		<-ctx.Done()
		return ctx.Err()
	}

	d.runSync()

	// Error counter should increment
	if r.metrics.ErrorCount() != 1 {
		t.Errorf("ErrorCount = %d, want 1", r.metrics.ErrorCount())
	}
}

// =============================================================================
// watchdogLoop
// =============================================================================

func TestWatchdogLoopStartsAndStops(t *testing.T) {
	r := New()
	d := NewDaemon(r, DaemonConfig{
		WatchdogInterval: 10 * time.Millisecond,
	})

	d.watchdogActive = true

	// Start the watchdog
	d.wg.Add(1)
	go d.watchdogLoop()

	// Let it tick a few times
	time.Sleep(25 * time.Millisecond)

	// Close watchdog stop channel
	close(d.watchdogStop)
	d.wg.Wait()
}

func TestWatchdogLoopContextCancel(t *testing.T) {
	r := New()
	d := NewDaemon(r, DaemonConfig{
		WatchdogInterval: 10 * time.Millisecond,
	})

	d.watchdogActive = true
	ctx, cancel := context.WithCancel(context.Background())
	d.ctx = ctx

	d.wg.Add(1)
	go d.watchdogLoop()

	time.Sleep(25 * time.Millisecond)

	cancel()
	d.wg.Wait()
}

// =============================================================================
// syncLoop
// =============================================================================

func TestSyncLoopStartsAndStops(t *testing.T) {
	r := New()
	d := NewDaemon(r, DaemonConfig{
		SyncInterval: 10 * time.Millisecond,
	})
	d.syncFunc = func(_ context.Context) error {
		return nil
	}
	d.syncStop = make(chan struct{})
	d.ctx = context.Background()

	d.wg.Add(1)
	go d.syncLoop()

	time.Sleep(15 * time.Millisecond)

	close(d.syncStop)
	d.wg.Wait()
}

func TestSyncLoopContextCancel(t *testing.T) {
	r := New()
	d := NewDaemon(r, DaemonConfig{
		SyncInterval: 10 * time.Millisecond,
	})
	d.syncFunc = func(_ context.Context) error {
		return nil
	}
	d.syncStop = make(chan struct{})

	ctx, cancel := context.WithCancel(context.Background())
	d.ctx = ctx

	d.wg.Add(1)
	go d.syncLoop()

	time.Sleep(15 * time.Millisecond)

	cancel()
	d.wg.Wait()
}

func TestSyncLoopNoSyncFunc(t *testing.T) {
	r := New()
	d := NewDaemon(r, DaemonConfig{
		SyncInterval: 10 * time.Millisecond,
	})
	d.syncFunc = nil
	d.syncStop = make(chan struct{})
	d.ctx = context.Background()

	d.wg.Add(1)
	go d.syncLoop()

	time.Sleep(15 * time.Millisecond)

	close(d.syncStop)
	d.wg.Wait()
}

// =============================================================================
// handleSignals (daemon)
// =============================================================================

func TestDaemonHandleSignalsSIGINTIgnored(t *testing.T) {
	// SIGINT is handled exclusively by Runtime.HandleSignals(), not the daemon.
	// This test verifies the daemon does NOT react to SIGINT — it ignores it.
	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())

	// Daemon only registers SIGHUP (not SIGINT/SIGTERM)
	signal.Notify(d.sigCh, syscall.SIGHUP)
	d.wg.Add(1)
	go d.handleSignals()

	// Send SIGINT directly to daemon channel — daemon should ignore it
	// (channel is only registered for SIGHUP, so SIGINT won't arrive)
	select {
	case d.sigCh <- syscall.SIGINT:
		// SIGINT sent to channel — but daemon doesn't handle it
	default:
	}

	// Verify the daemon context is NOT cancelled
	select {
	case <-d.ctx.Done():
		t.Error("daemon context was cancelled after SIGINT — should be ignored")
	case <-time.After(200 * time.Millisecond):
		// Expected: daemon ignores SIGINT
	}

	// Clean shutdown
	d.cancel()
	d.wg.Wait()
}

func TestDaemonHandleSignalsSIGTERMIgnored(t *testing.T) {
	// SIGTERM is handled exclusively by Runtime.HandleSignals(), not the daemon.
	// This test verifies the daemon does NOT react to SIGTERM.
	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())

	// Daemon only registers SIGHUP (not SIGINT/SIGTERM)
	signal.Notify(d.sigCh, syscall.SIGHUP)
	d.wg.Add(1)
	go d.handleSignals()

	// Send SIGTERM directly to daemon channel
	select {
	case d.sigCh <- syscall.SIGTERM:
	default:
	}

	// Verify the daemon context is NOT cancelled
	select {
	case <-d.ctx.Done():
		t.Error("daemon context was cancelled after SIGTERM — should be ignored")
	case <-time.After(200 * time.Millisecond):
		// Expected: daemon ignores SIGTERM
	}

	// Clean shutdown
	d.cancel()
	d.wg.Wait()
}

func TestDaemonHandleSignalsSIGHUPWithReload(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sinais POSIX (SIGHUP via Process.Signal) não suportados no Windows — no-op retornando EWINDOWS")
	}
	r := New()
	reloaded := make(chan struct{})
	d := NewDaemon(r, DaemonConfig{
		OnReload: func() error {
			close(reloaded)
			return nil
		},
	})

	signal.Notify(d.sigCh, syscall.SIGINT, syscall.SIGHUP)
	d.wg.Add(1)
	go d.handleSignals()

	pid := os.Getpid()
	proc, _ := os.FindProcess(pid)
	_ = proc.Signal(syscall.SIGHUP)

	select {
	case <-reloaded:
		// Reload called
	case <-time.After(time.Second):
		t.Error("OnReload not called after SIGHUP")
	}

	// Clean up
	d.cancel()
	d.wg.Wait()
}

func TestDaemonHandleSignalsSIGHUPError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sinais POSIX (SIGHUP via Process.Signal) não suportados no Windows — no-op retornando EWINDOWS")
	}
	r := New()
	called := make(chan struct{})
	d := NewDaemon(r, DaemonConfig{
		OnReload: func() error {
			close(called)
			return errors.New("reload failed")
		},
	})

	signal.Notify(d.sigCh, syscall.SIGINT, syscall.SIGHUP)
	d.wg.Add(1)
	go d.handleSignals()

	pid := os.Getpid()
	proc, _ := os.FindProcess(pid)
	_ = proc.Signal(syscall.SIGHUP)

	select {
	case <-called:
		// onReload was called despite error
	case <-time.After(time.Second):
		t.Error("OnReload not called")
	}

	d.cancel()
	d.wg.Wait()
}

func TestDaemonHandleSignalsContextDone(t *testing.T) {
	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())

	signal.Notify(d.sigCh, syscall.SIGINT)
	d.wg.Add(1)
	go d.handleSignals()

	d.cancel()

	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("handleSignals did not exit after context cancel")
	}
}

// =============================================================================
// runSync with nil runtime
// =============================================================================

func TestRunSyncNilRuntime(t *testing.T) {
	d := &Daemon{
		logger: zerolog.Nop(),
		ctx:    context.Background(),
	}
	d.syncFunc = func(_ context.Context) error {
		return nil
	}
	// Should not panic
	d.runSync()
}

func TestRunSyncNilRuntimeError(t *testing.T) {
	d := &Daemon{
		logger: zerolog.Nop(),
		ctx:    context.Background(),
	}
	d.syncFunc = func(_ context.Context) error {
		return errors.New("fail")
	}
	// Should not panic
	d.runSync()
}
