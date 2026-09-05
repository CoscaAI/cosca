// Package runtime — Coverage gap tests targeting 100% statement coverage.
//
// Covers the 10 functions below 100% identified in the 2026-07-30 audit.
// Some blocks are intentionally unreachable (dead code / OS-specific paths).
// Testes que exigem syscalls Unix (kill/sinal e RLIMIT_FSIZE) vivem em
// coverage_gap_unix_test.go (build tag unix).
package runtime

import (
	"context"
	"errors"
	"testing"
	"time"
)

// =============================================================================
// runtime.Start — 78.3% → target 90%+
// =============================================================================

// TestRuntimeStart_AlreadyStarted covers the "runtime already started" branch.
func TestRuntimeStart_AlreadyStarted(t *testing.T) {
	r := newTestRuntime()
	ctx := context.Background()

	if err := r.Start(ctx); err != nil {
		t.Fatalf("first Start failed: %v", err)
	}
	defer func() { _ = r.Stop(ctx) }()

	// Second Start must fail — state is Running, not Uninitialized.
	if err := r.Start(ctx); err == nil {
		t.Fatal("second Start should fail")
	}
}

// TestRuntimeStart_ExecuteInitFails covers the ExecuteInit error path.
// Knowledge subsystem is required=true; registering a failing MockSubsystem
// causes ExecuteInit → initKnowledge → Start error → hook failure → return error.
func TestRuntimeStart_ExecuteInitFails(t *testing.T) {
	r := newTestRuntime()
	ctx := context.Background()

	mock := &MockSubsystem{
		NameFunc:  func() string { return "knowledge" },
		StartFunc: func(ctx context.Context) error { return errors.New("init failure") },
	}
	r.RegisterKnowledge(mock)

	if err := r.Start(ctx); err == nil {
		t.Fatal("Start should fail when knowledge init fails")
	}
}

// TestRuntimeStart_ExecuteStartFails covers the ExecuteStart error path.
// We add a required start hook that fails. Init hooks (including knowledge)
// return nil (no subsystems registered → default hooks return nil).
// ExecuteStart runs all hooks; our failing required hook causes the error.
func TestRuntimeStart_ExecuteStartFails(t *testing.T) {
	r := newTestRuntime()
	ctx := context.Background()

	// Add a required start hook that will fail.
	r.lifecycle.AddStartHook("will-fail", func(ctx context.Context, rt *Runtime) error {
		return errors.New("start hook failure")
	}, time.Second, true)

	if err := r.Start(ctx); err == nil {
		t.Fatal("Start should fail when required start hook fails")
	}
}

// =============================================================================
// runtime.Restart — 78.6% → target 90%+
// =============================================================================

// TestRuntimeRestart_StopFails covers runtime.go:417-419.
// Restart calls Stop internally. If the state machine is in Error,
// TransitionTo(Stopping) is invalid → Stop returns error → Restart fails.
func TestRuntimeRestart_StopFails(t *testing.T) {
	r := newTestRuntime()
	ctx := context.Background()

	// Start to reach Running state.
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Corrupt state directly (same package access).
	// StateUninitialized → Stopping is the ONLY state where
	// Stop's TransitionTo(Stopping) fails.
	// (StateStopped is caught by the early-return check in Stop.)
	r.state.mu.Lock()
	r.state.CurrentState = StateUninitialized
	r.state.mu.Unlock()

	if err := r.Restart(ctx); err == nil {
		t.Fatal("Restart should fail when Stop fails")
	}
}

// TestRuntimeRestart_StartFails covers runtime.go:434-436.
// After restart's Stop+TransitionTo(Uninitialized) succeed,
// Start is called internally. If a required subsystem init fails, Start errors.
func TestRuntimeRestart_StartFails(t *testing.T) {
	r := newTestRuntime()
	ctx := context.Background()

	// First Start succeeds normally.
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Now register a failing knowledge subsystem.
	// The next Start (called internally by Restart) will fail at ExecuteInit.
	mock := &MockSubsystem{
		NameFunc:  func() string { return "knowledge" },
		StartFunc: func(ctx context.Context) error { return errors.New("knowledge failure") },
	}
	r.RegisterKnowledge(mock)

	if err := r.Restart(ctx); err == nil {
		t.Fatal("Restart should fail when internal Start fails")
	}
}

// =============================================================================
// daemon.Start — 88.2% → target 100%
// =============================================================================

// TestDaemonStart_WithSyncFunc covers daemon.go:147-150.
// The syncFunc != nil branch was uncovered because all existing tests
// created daemons without a sync function.
func TestDaemonStart_WithSyncFunc(t *testing.T) {
	r := newTestRuntime()

	syncCalled := make(chan struct{}, 1)
	cfg := DaemonConfig{
		PIDPath:          "", // Empty → skip PID file
		WatchdogInterval: time.Hour,
		SyncInterval:     10 * time.Millisecond,
		SyncFunc: func(ctx context.Context) error {
			select {
			case syncCalled <- struct{}{}:
			default:
			}
			return nil
		},
	}
	d := NewDaemon(r, cfg)

	if err := d.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = d.Stop() }()

	// Verify syncFunc was actually called.
	select {
	case <-syncCalled:
		// Good — sync loop started.
	case <-time.After(500 * time.Millisecond):
		t.Error("sync function was never called")
	}
}

// =============================================================================
// metrics.GoroutineStats — 92.9% → target 100%
// =============================================================================

// TestGoroutineStats_MaxBranch covers metrics.go:277-279 (v > maxVal branch).
// The existing tests use real goroutine counts which are monotonic;
// the maxVal update path was never exercised.
func TestGoroutineStats_MaxBranch(t *testing.T) {
	m := NewMetrics()

	// Directly set samples (same package) to exercise both min AND max branches.
	m.goroutineSamples = []int{5, 1, 10, 3, 8}

	minVal, maxVal, avg := m.GoroutineStats()

	if minVal != 1 {
		t.Errorf("min = %d, want 1", minVal)
	}
	if maxVal != 10 {
		t.Errorf("max = %d, want 10", maxVal)
	}
	// avg = (5+1+10+3+8) / 5 = 27/5 = 5
	if avg != 5 {
		t.Errorf("avg = %d, want 5", avg)
	}
}

// =============================================================================
// metrics.percentileIndex — 87.5% → target 100%
// =============================================================================

// TestPercentileIndex_NegativeIdx covers metrics.go:443-445 (idx < 0).
// This branch is reachable with a negative percentile value.
func TestPercentileIndex_NegativeIdx(t *testing.T) {
	// With n=1 and p=-100: (1 * -100) / 100 = -1. idx < 0 → return 0.
	got := percentileIndex(1, -100)
	if got != 0 {
		t.Errorf("percentileIndex(1, -100) = %d, want 0", got)
	}
}

// =============================================================================
// state.SetRecovery — 90.0% → target 100%
// =============================================================================

// TestSetRecovery_WithOnChange covers state.go:426-433 (onChange callback path).
func TestSetRecovery_WithOnChange(t *testing.T) {
	rs := NewRuntimeState()

	// Transition to a state that allows recovery (Running → Recovering is valid).
	_ = rs.TransitionTo(StateInitializing, "init")
	_ = rs.TransitionTo(StateReady, "ready")
	_ = rs.TransitionTo(StateRunning, "run")

	// Register onChange callback.
	eventReceived := make(chan StateChangeEvent, 1)
	rs.OnStateChange(func(e StateChangeEvent) {
		select {
		case eventReceived <- e:
		default:
		}
	})

	rs.SetRecovery()

	// Verify onChange was called.
	select {
	case evt := <-eventReceived:
		if evt.To != StateRecovering {
			t.Errorf("event.To = %v, want %v", evt.To, StateRecovering)
		}
	case <-time.After(time.Second):
		t.Fatal("onChange was not called")
	}
}

// TestSetRecovery_InvalidTransition covers state.go:422 (canTransitionTo false path).
// From StateUninitialized, recovery is not valid.
func TestSetRecovery_InvalidTransition(t *testing.T) {
	rs := NewRuntimeState()
	// State is Uninitialized. Recovery is not a valid transition from here.

	rs.SetRecovery()

	// State should remain Uninitialized (transition was rejected).
	if rs.Current() != StateUninitialized {
		t.Errorf("state = %v, want %v", rs.Current(), StateUninitialized)
	}
	// RecoveryCount should still be incremented.
	if rs.RecoveryCount != 1 {
		t.Errorf("RecoveryCount = %d, want 1", rs.RecoveryCount)
	}
}

// =============================================================================
// state.recomputeHealth — 95.0% → 100% after dead code removal
// =============================================================================

// TestRecomputeHealth_AllStopped exercises the allStopped case with unknown components.
func TestRecomputeHealth_AllStopped(t *testing.T) {
	rs := NewRuntimeState()

	// StatusUnknown falls through to default (no flags set).
	// Result: hasUnhealthy=false, hasDegraded=false, allStopped=true
	// → allStopped fires → Health = StatusUnknown.
	rs.SetComponentStatus("a", StatusUnknown, "")
	rs.SetComponentStatus("b", StatusUnknown, "")

	rs.recomputeHealth()

	if rs.Health != StatusUnknown {
		t.Errorf("Health = %v, want %v (allStopped)", rs.Health, StatusUnknown)
	}
}
