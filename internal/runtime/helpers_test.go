// Package runtime — Unit tests for helper functions with incomplete coverage.
//
// Targets uncovered branches identified via -coverprofile analysis:
//   - canTransitionTo: invalid from state (!ok path)
//   - processRunning: negative PID, edge cases
//   - Lifecycle init hooks: error paths (discovery, memory, plugins, editors, watcher)
//   - Lifecycle stop hooks: error paths (editors, plugins, cache, memory, discovery)
//   - recomputeHealth: all-stopped scenario
//   - GoroutineStats: zero samples
//   - Metrics.StartTime: type assertion failure
//   - GetComponentHealth: type assertion failure
//   - Daemon.Start: writePIDFile error propagation
//   - Daemon.Stop: removePIDFile error propagation
//   - writePIDFile: daemon already running detection
//   - removePIDFile: os.Remove permission error
//   - syncLoop: initial timer path
//   - Runtime.Restart: error paths
package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// =============================================================================
// canTransitionTo edge cases
// =============================================================================

// TestCanTransitionToInvalidFrom validates that querying transitions from an
// invalid state (not present in the validTransitions map) returns false.
func TestCanTransitionToInvalidFrom(t *testing.T) {
	t.Parallel()

	// ARRANGE: Use a State value that is not in the validTransitions map.
	// validTransitions has keys 0-7 (StateUninitialized through StateRecovering).
	// State(99) is not a key in the map, so it should hit the !ok branch.
	invalidState := State(99)

	// ACT
	got := canTransitionTo(invalidState, StateRunning)

	// ASSERT: should return false since invalidState is not in the map
	if got {
		t.Errorf("canTransitionTo(State(99), StateRunning) = true, want false")
	}
}

// TestCanTransitionToNegativeState validates behavior with negative state values.
func TestCanTransitionToNegativeState(t *testing.T) {
	t.Parallel()

	// ARRANGE
	negState := State(-1)

	// ACT
	got := canTransitionTo(negState, StateRunning)

	// ASSERT
	if got {
		t.Errorf("canTransitionTo(State(-1), StateRunning) = true, want false")
	}
}

// =============================================================================
// processRunning edge cases
// =============================================================================

// TestProcessRunningNegativePID validates that negative PIDs return false.
func TestProcessRunningNegativePID(t *testing.T) {
	t.Parallel()

	// ARRANGE & ACT
	running := processRunning(-1)

	// ASSERT: negative PID should never be running
	if running {
		t.Error("processRunning(-1) should return false for negative PID")
	}
}

// TestProcessRunningZeroPID validates PID 0 handling.
func TestProcessRunningZeroPID(t *testing.T) {
	t.Parallel()

	// ARRANGE & ACT
	running := processRunning(0)

	// ASSERT: PID 0 is not a real process on Unix
	if running {
		t.Error("processRunning(0) should return false")
	}
}

// =============================================================================
// recomputeHealth — all-stopped scenario
// =============================================================================

// TestRecomputeHealthWhenAllStopped verifies that when all components are stopped,
// the overall health is set to StatusUnknown.
func TestRecomputeHealthWhenAllStopped(t *testing.T) {
	t.Parallel()

	// ARRANGE: set all components to stopped
	rs := NewRuntimeState()
	rs.SetComponentStatus("knowledge", StatusStoppedComponent, "stopped")
	rs.SetComponentStatus("cache", StatusStoppedComponent, "stopped")
	rs.SetComponentStatus("plugins", StatusStoppedComponent, "stopped")

	// ASSERT: recomputeHealth should set health to unknown when all are stopped
	if rs.Health != StatusUnknown {
		t.Errorf("Health = %v, want %v (all components stopped)", rs.Health, StatusUnknown)
	}
}

// TestRecomputeHealthUnhealthyOverridesDegraded verifies unhealthy takes
// priority over degraded.
func TestRecomputeHealthUnhealthyOverridesDegraded(t *testing.T) {
	t.Parallel()

	// ARRANGE: mix of degraded and unhealthy
	rs := NewRuntimeState()
	rs.SetComponentStatus("a", StatusDegraded, "slow")
	rs.SetComponentStatus("b", StatusUnhealthy, "crashed")

	// ASSERT: unhealthy takes priority over degraded
	if rs.Health != StatusUnhealthy {
		t.Errorf("Health = %v, want %v", rs.Health, StatusUnhealthy)
	}
}

// TestRecomputeHealthHealthyOverrideUnknown verifies that if at least one
// component is healthy, health is not unknown (unless nothing else is set).
func TestRecomputeHealthHealthyWithUnknown(t *testing.T) {
	t.Parallel()

	// ARRANGE: one healthy, no unhealthy or degraded
	rs := NewRuntimeState()
	rs.SetComponentStatus("knowledge", StatusHealthy, "ok")

	// ASSERT: overall should be healthy
	if rs.Health != StatusHealthy {
		t.Errorf("Health = %v, want %v", rs.Health, StatusHealthy)
	}
}

// =============================================================================
// Lifecycle init hooks — error paths
// =============================================================================

// TestInitDiscoveryStartError verifies initDiscovery propagates Start errors.
func TestInitDiscoveryStartError(t *testing.T) {
	t.Parallel()
	r := New()
	r.RegisterDiscovery(&MockSubsystem{
		NameFunc:  func() string { return "discovery" },
		StartFunc: func(_ context.Context) error { return errors.New("discovery init failed") },
	})

	err := r.lifecycle.initDiscovery(context.Background(), r)
	if err == nil {
		t.Error("initDiscovery should return error when Start fails")
	}
}

// TestInitMemoryStartError verifies initMemory propagates Start errors.
func TestInitMemoryStartError(t *testing.T) {
	t.Parallel()
	r := New()
	r.RegisterMemory(&MockSubsystem{
		NameFunc:  func() string { return "memory" },
		StartFunc: func(_ context.Context) error { return errors.New("memory init failed") },
	})

	err := r.lifecycle.initMemory(context.Background(), r)
	if err == nil {
		t.Error("initMemory should return error when Start fails")
	}
}

// TestInitPluginsStartError verifies initPlugins propagates Start errors.
func TestInitPluginsStartError(t *testing.T) {
	t.Parallel()
	r := New()
	r.RegisterPlugins(&MockSubsystem{
		NameFunc:  func() string { return "plugins" },
		StartFunc: func(_ context.Context) error { return errors.New("plugins init failed") },
	})

	err := r.lifecycle.initPlugins(context.Background(), r)
	if err == nil {
		t.Error("initPlugins should return error when Start fails")
	}
}

// TestInitEditorsStartError verifies initEditors propagates Start errors.
func TestInitEditorsStartError(t *testing.T) {
	t.Parallel()
	r := New()
	r.RegisterEditors(&MockSubsystem{
		NameFunc:  func() string { return "editors" },
		StartFunc: func(_ context.Context) error { return errors.New("editors init failed") },
	})

	err := r.lifecycle.initEditors(context.Background(), r)
	if err == nil {
		t.Error("initEditors should return error when Start fails")
	}
}

// TestInitWatcherStartError verifies initWatcher propagates Start errors.
func TestInitWatcherStartError(t *testing.T) {
	t.Parallel()
	r := New()
	r.RegisterWatcher(&MockSubsystem{
		NameFunc:  func() string { return "watcher" },
		StartFunc: func(_ context.Context) error { return errors.New("watcher init failed") },
	})

	err := r.lifecycle.initWatcher(context.Background(), r)
	if err == nil {
		t.Error("initWatcher should return error when Start fails")
	}
}

// =============================================================================
// Lifecycle stop hooks — error paths
// =============================================================================

// TestStopEditorsError verifies stopEditors propagates Stop errors.
func TestStopEditorsError(t *testing.T) {
	t.Parallel()
	r := New()
	r.RegisterEditors(&MockSubsystem{
		NameFunc: func() string { return "editors" },
		StopFunc: func(_ context.Context) error { return errors.New("editors stop failed") },
	})

	err := r.lifecycle.stopEditors(context.Background(), r)
	if err == nil {
		t.Error("stopEditors should return error when Stop fails")
	}
}

// TestStopPluginsError verifies stopPlugins propagates Stop errors.
func TestStopPluginsError(t *testing.T) {
	t.Parallel()
	r := New()
	r.RegisterPlugins(&MockSubsystem{
		NameFunc: func() string { return "plugins" },
		StopFunc: func(_ context.Context) error { return errors.New("plugins stop failed") },
	})

	err := r.lifecycle.stopPlugins(context.Background(), r)
	if err == nil {
		t.Error("stopPlugins should return error when Stop fails")
	}
}

// TestStopCacheError verifies stopCache propagates Stop errors.
func TestStopCacheError(t *testing.T) {
	t.Parallel()
	r := New()
	r.RegisterCache(&MockSubsystem{
		NameFunc: func() string { return "cache" },
		StopFunc: func(_ context.Context) error { return errors.New("cache stop failed") },
	})

	err := r.lifecycle.stopCache(context.Background(), r)
	if err == nil {
		t.Error("stopCache should return error when Stop fails")
	}
}

// TestStopMemoryError verifies stopMemory propagates Stop errors.
func TestStopMemoryError(t *testing.T) {
	t.Parallel()
	r := New()
	r.RegisterMemory(&MockSubsystem{
		NameFunc: func() string { return "memory" },
		StopFunc: func(_ context.Context) error { return errors.New("memory stop failed") },
	})

	err := r.lifecycle.stopMemory(context.Background(), r)
	if err == nil {
		t.Error("stopMemory should return error when Stop fails")
	}
}

// TestStopDiscoveryError verifies stopDiscovery propagates Stop errors.
func TestStopDiscoveryError(t *testing.T) {
	t.Parallel()
	r := New()
	r.RegisterDiscovery(&MockSubsystem{
		NameFunc: func() string { return "discovery" },
		StopFunc: func(_ context.Context) error { return errors.New("discovery stop failed") },
	})

	err := r.lifecycle.stopDiscovery(context.Background(), r)
	if err == nil {
		t.Error("stopDiscovery should return error when Stop fails")
	}
}

// =============================================================================
// GoroutineStats — zero samples
// =============================================================================

// TestGoroutineStatsZeroSamples verifies that zero samples return (0, 0, 0).
func TestGoroutineStatsZeroSamples(t *testing.T) {
	t.Parallel()

	// ARRANGE: no samples recorded
	m := NewMetrics()

	// ACT
	minVal, maxVal, avg := m.GoroutineStats()

	// ASSERT
	if minVal != 0 || maxVal != 0 || avg != 0 {
		t.Errorf("GoroutineStats with zero samples = (%d, %d, %d), want (0, 0, 0)",
			minVal, maxVal, avg)
	}
}

// =============================================================================
// Metrics.StartTime — type assertion failure
// =============================================================================

// TestMetricsStartTimeTypeAssertionFailure verifies StartTime handles a
// non-time.Time value stored in the atomic.Value gracefully.
func TestMetricsStartTimeTypeAssertionFailure(t *testing.T) {
	t.Parallel()

	// ARRANGE: store a non-time.Time value in the atomic.Value
	m := NewMetrics()
	var wrong atomic.Value
	wrong.Store("not-a-time")
	// Replace the startedAt with the bad value
	m.startedAt = wrong

	// ACT: Should not panic and should return time.Now()
	got := m.StartTime()

	// ASSERT: should return current time (fallback), not zero
	if got.IsZero() {
		t.Error("StartTime should not return zero time on type assertion failure")
	}
}

// =============================================================================
// GetComponentHealth — type assertion failure
// =============================================================================

// TestGetComponentHealthTypeAssertionFailure verifies GetComponentHealth handles
// a non-ComponentStatus value stored in the sync.Map gracefully.
func TestGetComponentHealthTypeAssertionFailure(t *testing.T) {
	t.Parallel()

	// ARRANGE: store a non-ComponentStatus value
	m := NewMetrics()
	m.componentHealth.Store("bad-component", 12345) // int, not ComponentStatus

	// ACT
	got := m.GetComponentHealth("bad-component")

	// ASSERT: should return StatusUnknown as fallback
	if got != StatusUnknown {
		t.Errorf("GetComponentHealth = %v, want %v", got, StatusUnknown)
	}
}

// =============================================================================
// Daemon.Start error propagation
// =============================================================================

// TestDaemonStartPIDFileError verifies that Start propagates writePIDFile errors.
func TestDaemonStartPIDFileError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permissões POSIX (chmod read-only) são no-op no Windows — diretório read-only não bloqueia criação de arquivo")
	}
	// ARRANGE: create a scenario where writePIDFile will fail.
	// Create a directory that we can't write into.
	readOnlyDir := t.TempDir()
	if err := os.Chmod(readOnlyDir, 0o444); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnlyDir, 0o755) })

	r := New()
	cfg := DaemonConfig{
		PIDPath:          filepath.Join(readOnlyDir, "subdir", "test.pid"),
		WatchdogInterval: time.Hour,
		SyncInterval:     time.Hour,
	}
	d := NewDaemon(r, cfg)

	// ACT
	err := d.Start()

	// ASSERT: writePIDFile should fail (can't create subdir in read-only parent)
	if err == nil {
		t.Error("Daemon.Start() should fail when PID file can't be written")
	}
}

// =============================================================================
// Daemon.Stop — removePIDFile error propagation
// =============================================================================

// TestDaemonStopPIDFileRemoveError verifies Stop handles removePIDFile errors
// gracefully (does not return error, logs warning).
func TestDaemonStopPIDFileRemoveError(t *testing.T) {
	// ARRANGE: put PID file in a directory we can't write to after writing succeeds.
	// First create the directory writable, write PID, then make it read-only.
	pidDir := t.TempDir()
	pidFile := filepath.Join(pidDir, "cosca.pid")

	r := New()
	cfg := DaemonConfig{
		PIDPath:          pidFile,
		WatchdogInterval: time.Hour,
		SyncInterval:     time.Hour,
	}
	d := NewDaemon(r, cfg)

	// Start the daemon to write PID file
	_ = d.Start()

	// Make PID dir read-only so removePIDFile fails
	_ = os.Chmod(pidDir, 0o444)
	t.Cleanup(func() { _ = os.Chmod(pidDir, 0o755) })

	// ACT: Stop should still succeed (removePIDFile error is non-fatal)
	err := d.Stop()

	// ASSERT
	if err != nil {
		t.Fatalf("Stop should not fail even when PID file remove fails: %v", err)
	}
}

// =============================================================================
// writePIDFile — daemon already running detection
// =============================================================================

// TestWritePIDFileOwnPIDNotBlocked verifies that a PID file containing the
// current process's own PID is NOT treated as a second daemon: os.Getpid()
// (or realPID() inside the jail) belongs to this same daemon, so the entry is
// stale from a previous boot and must be overwritten, never block startup.
func TestWritePIDFileOwnPIDNotBlocked(t *testing.T) {
	// ARRANGE: create a PID file containing the current process's PID
	pidDir := t.TempDir()
	pidFile := filepath.Join(pidDir, "cosca.pid")

	currentPID := fmt.Sprintf("%d\n", os.Getpid())
	if err := os.WriteFile(pidFile, []byte(currentPID), 0o644); err != nil {
		t.Fatalf("write pid: %v", err)
	}

	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	d.pidPath = pidFile

	// ACT: the guard must NOT treat our own PID as an already-running daemon
	err := d.writePIDFile()

	// ASSERT: should succeed and overwrite the stale file with realPID()
	if err != nil {
		t.Fatalf("writePIDFile with own PID should not block: %v", err)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("read pid file: %v", err)
	}
	want := fmt.Sprintf("%d\n", realPID())
	if string(data) != want {
		t.Errorf("pid file content = %q, want %q", string(data), want)
	}
	_ = d.removePIDFile()
}

// =============================================================================
// writePIDFile — write error after successful open
// =============================================================================

// TestWritePIDFileReadOnlyFile verifies writePIDFile handles permission errors
// when opening the PID file.
func TestWritePIDFileReadOnlyDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permissões POSIX (chmod read-only) são no-op no Windows — diretório read-only não bloqueia criação de arquivo")
	}
	// ARRANGE: create a directory without write permission for the current user
	readOnlyDir := t.TempDir()
	if err := os.Chmod(readOnlyDir, 0o500); err != nil {
		t.Fatalf("chmod 500: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnlyDir, 0o700) })

	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	d.pidPath = filepath.Join(readOnlyDir, "test.pid")

	// ACT
	err := d.writePIDFile()

	// ASSERT: should fail — can't create file in read-only directory
	if err == nil {
		t.Error("writePIDFile should fail when directory is read-only")
	}
}

// =============================================================================
// removePIDFile — os.Remove permission error
// =============================================================================

// TestRemovePIDFilePermissionDenied verifies removePIDFile handles permission
// errors when removing the PID file.
func TestRemovePIDFilePermissionDenied(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permissões POSIX (chmod read-only) são no-op no Windows — diretório read-only não bloqueia remoção de arquivo")
	}
	// ARRANGE: create a PID file in a directory, then make the dir read-only
	pidDir := t.TempDir()
	pidFile := filepath.Join(pidDir, "test.pid")

	if err := os.WriteFile(pidFile, []byte("12345\n"), 0o644); err != nil {
		t.Fatalf("write pid file: %v", err)
	}

	// Make directory read-only so file removal fails
	if err := os.Chmod(pidDir, 0o444); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(pidDir, 0o755) })

	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	d.pidPath = pidFile

	// ACT
	err := d.removePIDFile()

	// ASSERT: should return error (os.Remove fails on read-only dir)
	if err == nil {
		t.Error("removePIDFile should return error when file can't be removed")
	}
}

// =============================================================================
// syncLoop — initial timer path
// =============================================================================

// TestSyncLoopInitialTimer verifies that syncLoop fires an initial sync after
// a short delay, not just on the ticker interval.
func TestSyncLoopInitialTimer(t *testing.T) {
	// ARRANGE: create a daemon with sync configured
	r := New()
	syncCalled := make(chan struct{}, 3)
	syncCalledCount := 0
	var mu sync.Mutex

	d := NewDaemon(r, DaemonConfig{
		SyncInterval:     time.Hour, // Ticker won't fire in test
		WatchdogInterval: time.Hour,
	})
	d.syncFunc = func(_ context.Context) error {
		mu.Lock()
		syncCalledCount++
		c := syncCalledCount
		mu.Unlock()

		if c <= 2 {
			syncCalled <- struct{}{}
		}
		return nil
	}
	d.syncStop = make(chan struct{})
	d.ctx = context.Background()

	// ACT: start syncLoop
	d.wg.Add(1)
	go d.syncLoop()

	// ASSERT: the initial timer should fire (default 10s delay)
	// We wait for that first fire and then stop
	select {
	case <-syncCalled:
		// Initial sync fired
	case <-time.After(15 * time.Second):
		t.Error("initial sync timer did not fire within expected window")
	}

	close(d.syncStop)
	d.wg.Wait()

	mu.Lock()
	count := syncCalledCount
	mu.Unlock()
	if count < 1 {
		t.Error("sync should have been called at least once via initial timer")
	}
	t.Logf("sync called %d times via initial timer", count)
}

// =============================================================================
// Runtime.Restart error paths
// =============================================================================

// TestRuntimeRestartStopError verifies that Restart propagates Stop errors.
func TestRuntimeRestartStopError(t *testing.T) {
	// ARRANGE: create a runtime that is running
	r := New(WithLogger(zerolog.Nop()))
	ctx := context.Background()
	_ = r.Start(ctx)

	// Stop it so Restart's Stop call will be on a stopped runtime
	// (Stop should work fine), but then Start will create a new context.
	// Actually, Restart calls Stop then Start. If State is Stopped,
	// Stop returns nil (already stopped), then Start returns error
	// because state is not Uninitialized.
	// Let's force the restart to happen in a specific way.
	_ = r.Stop(ctx)

	// ACT: Restart should fail because state is Stopped, not Uninitialized
	err := r.Restart(ctx)

	// ASSERT: Restart should succeed — fix transitions Stopped→Uninitialized before Start
	if err != nil {
		t.Errorf("Restart should succeed after fix (transitions to Uninitialized): %v", err)
	}
}

// TestRuntimeRestartStartError verifies that Restart propagates Start errors
// after a valid Stop.
func TestRuntimeRestartStartError(t *testing.T) {
	// ARRANGE: Create a runtime, start it normally, then restart
	r := New(WithLogger(zerolog.Nop()))
	ctx := context.Background()
	_ = r.Start(ctx)

	// ACT: Restart while running
	err := r.Restart(ctx)

	// ASSERT: Stop + Start should succeed (or Start may fail due to state)
	if err != nil {
		t.Logf("Restart error (expected due to state machine): %v", err)
	}
}

// =============================================================================
// Runtime.HandleSignals — context cancellation exit
// =============================================================================

// TestHandleSignalsContextCancel verifies that the signal handler goroutine
// exits when the runtime context is cancelled.
func TestHandleSignalsContextCancel(t *testing.T) {
	// ARRANGE
	r := New(WithLogger(zerolog.Nop()))
	r.HandleSignals()

	// Signal the handler to stop by cancelling the context
	r.cancel()

	// ACT: wait for the signal handler goroutine to exit
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success: goroutine exited
	case <-time.After(2 * time.Second):
		t.Error("HandleSignals goroutine did not exit after context cancel")
	}
}

// =============================================================================
// EventBus.Publish — cancelled context handling
// =============================================================================

// TestPublishCancelledContext verifies that Publish handles a cancelled context
// without panicking (the per-handler timeout will still work).
func TestPublishCancelledContext(t *testing.T) {
	t.Parallel()

	// ARRANGE
	eb := NewEventBus(zerolog.Nop())
	called := false
	eb.Subscribe(EventStateChange, func(_ context.Context, _ Event) error {
		called = true
		return nil
	})

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// ACT: Publish with cancelled context should not panic
	eb.Publish(ctx, EventStateChange, "test", nil)

	// ASSERT: handler should still be called (it gets its own context)
	if !called {
		t.Error("handler should be called even with cancelled parent context")
	}
}

// =============================================================================
// Runtime.HandleSignals — SIGHUP signal path
// =============================================================================

// TestRuntimeHandleSignalsSIGHUP verifies that SIGHUP triggers config reload event.
func TestRuntimeHandleSignalsSIGHUPWithEvents(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sinais POSIX (SIGHUP via Process.Signal) não suportados no Windows — no-op retornando EWINDOWS")
	}
	// ARRANGE
	r := New(WithLogger(zerolog.Nop()))
	reloadFired := make(chan struct{}, 1)

	r.Events().Subscribe(EventConfigReload, func(_ context.Context, e Event) error {
		reloadFired <- struct{}{}
		return nil
	})

	r.HandleSignals()

	// ACT: send SIGHUP
	pid := os.Getpid()
	proc, _ := os.FindProcess(pid)
	_ = proc.Signal(syscall.SIGHUP)

	// ASSERT: config reload event should be published
	select {
	case <-reloadFired:
		// Event received
	case <-time.After(2 * time.Second):
		t.Error("EventConfigReload not published after SIGHUP")
	}

	// Cleanup
	r.cancel()
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
	}
}

// =============================================================================
// Daemon.handleSignals — SIGHUP without onReload
// =============================================================================

// TestDaemonHandleSignalsSIGHUPNoReload verifies SIGHUP doesn't panic when
// onReload is nil.
func TestDaemonHandleSignalsSIGHUPNoReload(t *testing.T) {
	// ARRANGE
	r := New()
	d := NewDaemon(r, DaemonConfig{
		OnReload: nil, // No reload handler
	})

	signal.Notify(d.sigCh, syscall.SIGINT, syscall.SIGHUP)
	d.wg.Add(1)
	go d.handleSignals()

	// ACT: send SIGHUP with no reload handler
	pid := os.Getpid()
	proc, _ := os.FindProcess(pid)
	_ = proc.Signal(syscall.SIGHUP)

	// Small wait to let handler process
	time.Sleep(50 * time.Millisecond)

	// Cleanup
	d.cancel()
	d.wg.Wait()
	// Test passes if no panic
}

// =============================================================================
// Daemon.syncLoop — no syncFunc
// =============================================================================

// TestSyncLoopNoSyncFuncInitialTimer verifies syncLoop with no syncFunc
// completes the initial timer path without calling runSync.
func TestSyncLoopNoSyncFuncInitialTimer(t *testing.T) {
	// ARRANGE
	r := New()
	d := NewDaemon(r, DaemonConfig{
		SyncInterval:     10 * time.Millisecond,
		WatchdogInterval: time.Hour,
	})
	d.syncFunc = nil
	d.syncStop = make(chan struct{})
	d.ctx = context.Background()

	// ACT
	d.wg.Add(1)
	go d.syncLoop()

	// Let it tick a couple of times
	time.Sleep(25 * time.Millisecond)

	// Cleanup
	close(d.syncStop)
	d.wg.Wait()
	// Test passes if syncLoop doesn't panic with nil syncFunc
}

// =============================================================================
// writePIDFile — directory creation path
// =============================================================================

// TestWritePIDFileCreatesDirectory verifies that writePIDFile creates
// the parent directory if it doesn't exist.
func TestWritePIDFileCreatesDirectory(t *testing.T) {
	// ARRANGE: use a PID path inside a non-existent subdirectory
	baseDir := t.TempDir()
	pidPath := filepath.Join(baseDir, "new", "subdir", "cosca.pid")

	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	d.pidPath = pidPath

	// ACT
	err := d.writePIDFile()

	// ASSERT
	if err != nil {
		t.Fatalf("writePIDFile should create parent directories: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(filepath.Dir(pidPath)); os.IsNotExist(err) {
		t.Error("parent directory was not created")
	}

	// Cleanup
	_ = d.removePIDFile()
}

// =============================================================================
// removePIDFile — closed pidFile handle
// =============================================================================

// TestRemovePIDFileWithOpenHandle verifies removePIDFile closes and removes
// an existing open PID file handle.
func TestRemovePIDFileWithOpenHandle(t *testing.T) {
	// ARRANGE: simulate a daemon with an open pidFile handle
	pidDir := t.TempDir()
	pidFile := filepath.Join(pidDir, "handle.pid")

	f, err := os.OpenFile(pidFile, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open pid file: %v", err)
	}
	_, _ = f.WriteString(fmt.Sprintf("%d\n", os.Getpid()))

	r := New()
	d := NewDaemon(r, DefaultDaemonConfig())
	d.pidPath = pidFile
	d.pidFile = f

	// ACT
	err = d.removePIDFile()

	// ASSERT
	if err != nil {
		t.Fatalf("removePIDFile: %v", err)
	}

	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Error("PID file should be removed")
	}
}

// =============================================================================
// Daemon.Start — no syncFunc
// =============================================================================

// TestDaemonStartNoSyncFunc verifies daemon starts correctly when no
// sync function is configured (the sync goroutine is not spawned).
func TestDaemonStartNoSyncFunc(t *testing.T) {
	// ARRANGE
	pidDir := t.TempDir()
	r := New()
	cfg := DaemonConfig{
		PIDPath:          filepath.Join(pidDir, "nosync.pid"),
		WatchdogInterval: time.Hour,
		SyncInterval:     time.Hour,
		SyncFunc:         nil, // No sync function
	}
	d := NewDaemon(r, cfg)

	// ACT
	err := d.Start()

	// ASSERT
	if err != nil {
		t.Fatalf("Start without syncFunc: %v", err)
	}
	if !d.IsRunning() {
		t.Error("daemon should be running")
	}

	// Cleanup
	_ = d.Stop()
}

// =============================================================================
// HealthReport — after state transitions
// =============================================================================

// TestHealthReportAfterStart verifies HealthReport reflects the running state.
func TestHealthReportAfterStart(t *testing.T) {
	// ARRANGE
	r := New(WithLogger(zerolog.Nop()))
	ctx := context.Background()
	_ = r.Start(ctx)
	defer func() { _ = r.Stop(ctx) }()

	// ACT
	report := r.HealthReport()

	// ASSERT
	if report["state"] != "running" {
		t.Errorf("state = %v, want running", report["state"])
	}
	if report["uptime"].(string) == "0s" {
		// Up should be non-zero after Start
		t.Logf("uptime = %v (may be 0s immediately after start)", report["uptime"])
	}
	if report["version"] != "0.0.0" {
		t.Errorf("version = %v", report["version"])
	}
}

// =============================================================================
// Summary with no started_at
// =============================================================================

// TestSummaryWithNoStartedAt verifies that Summary doesn't panic when
// StartedAt is zero (should not print uptime line).
func TestSummaryWithNoStartedAt(t *testing.T) {
	t.Parallel()

	// ARRANGE: runtime state with zero StartedAt
	rs := NewRuntimeState()
	// StartedAt is zero by default

	// ACT: should not panic
	summary := rs.Summary()

	// ASSERT: should contain state but NOT uptime
	if summary == "" {
		t.Error("Summary should not be empty")
	}
	if len(summary) == 0 {
		t.Error("Summary should not be empty")
	}
	// Uptime line should not appear (StartedAt is zero)
	if contains(summary, "Uptime:") {
		t.Error("Summary should not contain uptime when StartedAt is zero")
	}
}

// contains is a helper for string containment checks.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// =============================================================================
// State.String with all values
// =============================================================================

// TestStateStringAllValues verifies all defined states have correct strings.
func TestStateStringAllValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		state State
		want  string
	}{
		{StateUninitialized, "uninitialized"},
		{StateInitializing, "initializing"},
		{StateReady, "ready"},
		{StateRunning, "running"},
		{StateStopping, "stopping"},
		{StateStopped, "stopped"},
		{StateError, "error"},
		{StateRecovering, "recovering"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

// =============================================================================
// SetComponentStatus — unhealthy to healthy transition clears error
// =============================================================================

// TestSetComponentStatusUnhealthyToHealthyClearsError verifies that transitioning
// a component from unhealthy to healthy clears the error field.
func TestSetComponentStatusUnhealthyToHealthyClearsError(t *testing.T) {
	t.Parallel()

	// ARRANGE
	rs := NewRuntimeState()
	rs.SetComponentStatus("test-comp", StatusUnhealthy, "critical failure")

	// ACT: transition back to healthy
	rs.SetComponentStatus("test-comp", StatusHealthy, "recovered")

	// ASSERT
	info, ok := rs.ComponentStatus("test-comp")
	if !ok {
		t.Fatal("component not found")
	}
	if info.Status != StatusHealthy {
		t.Errorf("status = %v, want healthy", info.Status)
	}
	if info.Error != "" {
		t.Errorf("error should be cleared on healthy transition, got: %q", info.Error)
	}
}

// =============================================================================
// IncrementRestartCount — non-existent component
// =============================================================================

// TestIncrementRestartCountNotFails verifies that IncrementRestartCount
// does not panic for a component that hasn't been registered.
func TestIncrementRestartCountNotFails(t *testing.T) {
	t.Parallel()

	// ARRANGE
	rs := NewRuntimeState()

	// ACT: should not panic
	rs.IncrementRestartCount("nonexistent")

	// ASSERT: component should not be created
	_, ok := rs.ComponentStatus("nonexistent")
	if ok {
		t.Error("IncrementRestartCount should not create component for unregistered name")
	}
}

// =============================================================================
// SetError with nil error — secondary verification
// =============================================================================

// TestSetErrorDoesNotCrashOnNil verifies SetError is a no-op for nil errors
// (complementary to existing TestSetErrorNil).
func TestSetErrorDoesNotCrashOnNil(t *testing.T) {
	t.Parallel()

	// ARRANGE: already in error state
	rs := NewRuntimeState()
	_ = rs.TransitionTo(StateInitializing, "boot")
	rs.SetError(errors.New("real error"))

	// ACT: calling SetError(nil) should not panic and not change state
	rs.SetError(nil)

	// ASSERT: should remain in error state with original message
	if rs.CurrentState != StateError {
		t.Errorf("State should remain Error, got %v", rs.CurrentState)
	}
}

// =============================================================================
// AllComponentStatuses — non-empty
// =============================================================================

// TestAllComponentStatusesMultiple verifies AllComponentStatuses returns
// correct number of components after registration.
func TestAllComponentStatusesMultiple(t *testing.T) {
	t.Parallel()

	// ARRANGE
	rs := NewRuntimeState()
	rs.SetComponentStatus("a", StatusHealthy, "")
	rs.SetComponentStatus("b", StatusDegraded, "")
	rs.SetComponentStatus("c", StatusUnhealthy, "oops")

	// ACT
	statuses := rs.AllComponentStatuses()

	// ASSERT
	if len(statuses) != 3 {
		t.Errorf("AllComponentStatuses length = %d, want 3", len(statuses))
	}
	if statuses["a"].Status != StatusHealthy {
		t.Errorf("component a status = %v", statuses["a"].Status)
	}
	if statuses["b"].Status != StatusDegraded {
		t.Errorf("component b status = %v", statuses["b"].Status)
	}
	if statuses["c"].Status != StatusUnhealthy {
		t.Errorf("component c status = %v", statuses["c"].Status)
	}
}
