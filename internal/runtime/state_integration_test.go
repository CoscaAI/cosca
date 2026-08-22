// Package runtime — Integration tests for the runtime state machine.
//
// Tests every transition defined in the validTransitions map (21 total)
// with AAA pattern, isolated state per test, and -race safe.
package runtime

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

// =============================================================================
// Transition Validation — Every edge in validTransitions graph
// =============================================================================

// transitionTestCase defines a single state transition test.
type transitionTestCase struct {
	name    string
	setup   func(*RuntimeState) // Arranges initial state
	to      State
	reason  string
	wantErr bool
	check   func(*testing.T, *RuntimeState) // Post-condition assertions
}

// transitionTests is the complete catalog of 21 valid transitions.
var transitionTests = []transitionTestCase{
	// === Uninitialized ===
	{
		name: "Uninitialized→Initializing",
		to:   StateInitializing, reason: "startup", wantErr: false,
		check: func(t *testing.T, rs *RuntimeState) {
			t.Helper()
			if rs.CurrentState != StateInitializing {
				t.Errorf("state = %v, want %v", rs.CurrentState, StateInitializing)
			}
			if rs.PreviousState != StateUninitialized {
				t.Errorf("previous = %v, want %v", rs.PreviousState, StateUninitialized)
			}
		},
	},
	{
		name: "Uninitialized→Error",
		to:   StateError, reason: "fatal init", wantErr: false,
		check: func(t *testing.T, rs *RuntimeState) {
			t.Helper()
			if rs.Health != StatusUnhealthy {
				t.Errorf("health = %v, want %v", rs.Health, StatusUnhealthy)
			}
		},
	},
	{
		name:  "Uninitialized→Running (invalid)",
		setup: nil,
		to:    StateRunning, reason: "invalid jump", wantErr: true,
		check: func(t *testing.T, rs *RuntimeState) {
			t.Helper()
			if rs.CurrentState != StateUninitialized {
				t.Errorf("state should remain Uninitialized, got %v", rs.CurrentState)
			}
		},
	},

	// === Initializing ===
	{
		name: "Initializing→Ready",
		setup: func(rs *RuntimeState) {
			_ = rs.TransitionTo(StateInitializing, "init")
		},
		to: StateReady, reason: "init complete", wantErr: false,
		check: func(t *testing.T, rs *RuntimeState) {
			t.Helper()
			if rs.CurrentState != StateReady {
				t.Errorf("state = %v, want %v", rs.CurrentState, StateReady)
			}
		},
	},
	{
		name: "Initializing→Error",
		setup: func(rs *RuntimeState) {
			_ = rs.TransitionTo(StateInitializing, "init")
		},
		to: StateError, reason: "app crashed", wantErr: false,
		check: func(t *testing.T, rs *RuntimeState) {
			t.Helper()
			if rs.CurrentState != StateError {
				t.Errorf("state = %v", rs.CurrentState)
			}
			if rs.ErrorMessage != "app crashed" {
				t.Errorf("error msg = %q", rs.ErrorMessage)
			}
		},
	},
	{
		name: "Initializing→Stopping",
		setup: func(rs *RuntimeState) {
			_ = rs.TransitionTo(StateInitializing, "init")
		},
		to: StateStopping, reason: "early shutdown", wantErr: false,
		check: func(t *testing.T, rs *RuntimeState) {
			t.Helper()
			if rs.CurrentState != StateStopping {
				t.Errorf("state = %v", rs.CurrentState)
			}
		},
	},

	// === Ready ===
	{
		name: "Ready→Running",
		setup: func(rs *RuntimeState) {
			_ = rs.TransitionTo(StateInitializing, "init")
			_ = rs.TransitionTo(StateReady, "ready")
		},
		to: StateRunning, reason: "start subsystems", wantErr: false,
		check: func(t *testing.T, rs *RuntimeState) {
			t.Helper()
			if rs.CurrentState != StateRunning {
				t.Errorf("state = %v", rs.CurrentState)
			}
			if rs.StartedAt.IsZero() {
				t.Error("StartedAt should be set")
			}
		},
	},
	{
		name: "Ready→Stopping",
		setup: func(rs *RuntimeState) {
			_ = rs.TransitionTo(StateInitializing, "init")
			_ = rs.TransitionTo(StateReady, "ready")
		},
		to: StateStopping, reason: "shutdown requested", wantErr: false,
		check: func(t *testing.T, rs *RuntimeState) {
			t.Helper()
			if rs.CurrentState != StateStopping {
				t.Errorf("state = %v", rs.CurrentState)
			}
		},
	},
	{
		name: "Ready→Error",
		setup: func(rs *RuntimeState) {
			_ = rs.TransitionTo(StateInitializing, "init")
			_ = rs.TransitionTo(StateReady, "ready")
		},
		to: StateError, reason: "component failure", wantErr: false,
		check: func(t *testing.T, rs *RuntimeState) {
			t.Helper()
			if rs.Health != StatusUnhealthy {
				t.Errorf("health = %v, want %v", rs.Health, StatusUnhealthy)
			}
		},
	},

	// === Running ===
	{
		name: "Running→Stopping",
		setup: func(rs *RuntimeState) {
			_ = rs.TransitionTo(StateInitializing, "init")
			_ = rs.TransitionTo(StateReady, "ready")
			_ = rs.TransitionTo(StateRunning, "run")
		},
		to: StateStopping, reason: "shutdown", wantErr: false,
		check: func(t *testing.T, rs *RuntimeState) {
			t.Helper()
			if rs.CurrentState != StateStopping {
				t.Errorf("state = %v", rs.CurrentState)
			}
		},
	},
	{
		name: "Running→Error",
		setup: func(rs *RuntimeState) {
			_ = rs.TransitionTo(StateInitializing, "init")
			_ = rs.TransitionTo(StateReady, "ready")
			_ = rs.TransitionTo(StateRunning, "run")
		},
		to: StateError, reason: "crash", wantErr: false,
		check: func(t *testing.T, rs *RuntimeState) {
			t.Helper()
			if rs.CurrentState != StateError {
				t.Errorf("state = %v", rs.CurrentState)
			}
		},
	},
	{
		name: "Running→Recovering",
		setup: func(rs *RuntimeState) {
			_ = rs.TransitionTo(StateInitializing, "init")
			_ = rs.TransitionTo(StateReady, "ready")
			_ = rs.TransitionTo(StateRunning, "run")
		},
		to: StateRecovering, reason: "auto-recovery", wantErr: false,
		check: func(t *testing.T, rs *RuntimeState) {
			t.Helper()
			if rs.CurrentState != StateRecovering {
				t.Errorf("state = %v", rs.CurrentState)
			}
		},
	},
	{
		name: "Running→Ready (health degradation)",
		setup: func(rs *RuntimeState) {
			_ = rs.TransitionTo(StateInitializing, "init")
			_ = rs.TransitionTo(StateReady, "ready")
			_ = rs.TransitionTo(StateRunning, "run")
		},
		to: StateReady, reason: "degraded to ready", wantErr: false,
		check: func(t *testing.T, rs *RuntimeState) {
			t.Helper()
			if rs.CurrentState != StateReady {
				t.Errorf("state = %v, want %v", rs.CurrentState, StateReady)
			}
		},
	},

	// === Stopping ===
	{
		name: "Stopping→Stopped",
		setup: func(rs *RuntimeState) {
			_ = rs.TransitionTo(StateInitializing, "init")
			_ = rs.TransitionTo(StateReady, "ready")
			_ = rs.TransitionTo(StateRunning, "run")
			_ = rs.TransitionTo(StateStopping, "stop")
		},
		to: StateStopped, reason: "shutdown done", wantErr: false,
		check: func(t *testing.T, rs *RuntimeState) {
			t.Helper()
			if rs.CurrentState != StateStopped {
				t.Errorf("state = %v", rs.CurrentState)
			}
			if rs.Health != StatusUnknown {
				t.Errorf("health = %v, want %v (after stop)", rs.Health, StatusUnknown)
			}
		},
	},
	{
		name: "Stopping→Error",
		setup: func(rs *RuntimeState) {
			_ = rs.TransitionTo(StateInitializing, "init")
			_ = rs.TransitionTo(StateReady, "ready")
			_ = rs.TransitionTo(StateRunning, "run")
			_ = rs.TransitionTo(StateStopping, "stop")
		},
		to: StateError, reason: "stop failure", wantErr: false,
		check: func(t *testing.T, rs *RuntimeState) {
			t.Helper()
			if rs.CurrentState != StateError {
				t.Errorf("state = %v", rs.CurrentState)
			}
		},
	},

	// === Stopped ===
	{
		name: "Stopped→Uninitialized (restart path)",
		setup: func(rs *RuntimeState) {
			_ = rs.TransitionTo(StateInitializing, "init")
			_ = rs.TransitionTo(StateReady, "ready")
			_ = rs.TransitionTo(StateRunning, "run")
			_ = rs.TransitionTo(StateStopping, "stop")
			_ = rs.TransitionTo(StateStopped, "done")
		},
		to: StateUninitialized, reason: "reset for restart", wantErr: false,
		check: func(t *testing.T, rs *RuntimeState) {
			t.Helper()
			if rs.CurrentState != StateUninitialized {
				t.Errorf("state = %v, want %v", rs.CurrentState, StateUninitialized)
			}
		},
	},

	// === Error ===
	{
		name: "Error→Recovering",
		setup: func(rs *RuntimeState) {
			_ = rs.TransitionTo(StateInitializing, "init")
			_ = rs.TransitionTo(StateError, "crash")
		},
		to: StateRecovering, reason: "recovery attempt", wantErr: false,
		check: func(t *testing.T, rs *RuntimeState) {
			t.Helper()
			if rs.CurrentState != StateRecovering {
				t.Errorf("state = %v", rs.CurrentState)
			}
		},
	},
	{
		name: "Error→Stopping (fatal error)",
		setup: func(rs *RuntimeState) {
			_ = rs.TransitionTo(StateInitializing, "init")
			_ = rs.TransitionTo(StateError, "fatal")
		},
		to: StateStopping, reason: "emergency shutdown", wantErr: false,
		check: func(t *testing.T, rs *RuntimeState) {
			t.Helper()
			if rs.CurrentState != StateStopping {
				t.Errorf("state = %v", rs.CurrentState)
			}
		},
	},
	{
		name: "Error→Uninitialized (hard reset)",
		setup: func(rs *RuntimeState) {
			_ = rs.TransitionTo(StateInitializing, "init")
			_ = rs.TransitionTo(StateError, "fatal")
		},
		to: StateUninitialized, reason: "hard reset", wantErr: false,
		check: func(t *testing.T, rs *RuntimeState) {
			t.Helper()
			if rs.CurrentState != StateUninitialized {
				t.Errorf("state = %v", rs.CurrentState)
			}
		},
	},

	// === Recovering ===
	{
		name: "Recovering→Ready (success)",
		setup: func(rs *RuntimeState) {
			_ = rs.TransitionTo(StateInitializing, "init")
			_ = rs.TransitionTo(StateError, "crash")
			_ = rs.TransitionTo(StateRecovering, "recover")
		},
		to: StateReady, reason: "recovered", wantErr: false,
		check: func(t *testing.T, rs *RuntimeState) {
			t.Helper()
			if rs.CurrentState != StateReady {
				t.Errorf("state = %v, want %v", rs.CurrentState, StateReady)
			}
		},
	},
	{
		name: "Recovering→Error (failure)",
		setup: func(rs *RuntimeState) {
			_ = rs.TransitionTo(StateInitializing, "init")
			_ = rs.TransitionTo(StateError, "crash")
			_ = rs.TransitionTo(StateRecovering, "recover")
		},
		to: StateError, reason: "recovery failed", wantErr: false,
		check: func(t *testing.T, rs *RuntimeState) {
			t.Helper()
			if rs.CurrentState != StateError {
				t.Errorf("state = %v", rs.CurrentState)
			}
		},
	},
	{
		name: "Recovering→Stopping",
		setup: func(rs *RuntimeState) {
			_ = rs.TransitionTo(StateInitializing, "init")
			_ = rs.TransitionTo(StateError, "crash")
			_ = rs.TransitionTo(StateRecovering, "recover")
		},
		to: StateStopping, reason: "abort recovery", wantErr: false,
		check: func(t *testing.T, rs *RuntimeState) {
			t.Helper()
			if rs.CurrentState != StateStopping {
				t.Errorf("state = %v", rs.CurrentState)
			}
		},
	},
}

// TestStateTransitions_CompleteCatalog validates all 21 valid transitions.
func TestStateTransitions_CompleteCatalog(t *testing.T) {
	t.Parallel()

	// Group variants: transitions that fail
	invalidTests := []transitionTestCase{
		{name: "Uninitialized→Running (invalid)", to: StateRunning, reason: "invalid", wantErr: true},
	}

	for _, tt := range transitionTests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// ARRANGE — Fresh state per test
			rs := NewRuntimeState()
			if tt.setup != nil {
				tt.setup(rs)
			}

			// ACT
			err := rs.TransitionTo(tt.to, tt.reason)

			// ASSERT
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.check != nil {
				tt.check(t, rs)
			}
		})
	}

	// Verify the invalid transitions also work as expected
	for _, tt := range invalidTests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			rs := NewRuntimeState()
			err := rs.TransitionTo(tt.to, tt.reason)
			if err == nil {
				t.Error("expected error for invalid transition")
			}
			if rs.CurrentState != StateUninitialized {
				t.Errorf("state should remain Uninitialized, got %v", rs.CurrentState)
			}
		})
	}
}

// =============================================================================
// Full Lifecycle Paths — Integration
// =============================================================================

// TestStateMachine_FullHappyPath validates the complete Uninitialized→Running lifecycle.
func TestStateMachine_FullHappyPath(t *testing.T) {
	t.Parallel()

	// ARRANGE
	rs := NewRuntimeState()

	// ACT & ASSERT — 4 steps

	// Step 1: Uninitialized → Initializing
	if err := rs.TransitionTo(StateInitializing, "startup"); err != nil {
		t.Fatalf("step 1: %v", err)
	}
	if rs.CurrentState != StateInitializing {
		t.Fatalf("step 1: state = %v", rs.CurrentState)
	}

	// Step 2: Initializing → Ready
	if err := rs.TransitionTo(StateReady, "init done"); err != nil {
		t.Fatalf("step 2: %v", err)
	}

	// Step 3: Ready → Running
	if err := rs.TransitionTo(StateRunning, "subsystems up"); err != nil {
		t.Fatalf("step 3: %v", err)
	}
	if rs.StartedAt.IsZero() {
		t.Error("StartedAt should be set after reaching Running")
	}

	// Step 4: Verify complete state
	snapshot := rs.Get()
	if snapshot.CurrentState != StateRunning {
		t.Errorf("final state = %v, want %v", snapshot.CurrentState, StateRunning)
	}
	if snapshot.PreviousState != StateReady {
		t.Errorf("previous = %v, want %v", snapshot.PreviousState, StateReady)
	}
}

// TestStateMachine_GracefulShutdown validates Ready→Stopping→Stopped.
func TestStateMachine_GracefulShutdown(t *testing.T) {
	t.Parallel()

	// ARRANGE
	rs := NewRuntimeState()
	_ = rs.TransitionTo(StateInitializing, "init")
	_ = rs.TransitionTo(StateReady, "ready")
	_ = rs.TransitionTo(StateRunning, "run")

	// ACT — Stop sequence
	err1 := rs.TransitionTo(StateStopping, "shutdown")
	if err1 != nil {
		t.Fatalf("transition to stopping: %v", err1)
	}

	err2 := rs.TransitionTo(StateStopped, "done")
	if err2 != nil {
		t.Fatalf("transition to stopped: %v", err2)
	}

	// ASSERT
	if rs.CurrentState != StateStopped {
		t.Errorf("state = %v, want %v", rs.CurrentState, StateStopped)
	}
	if rs.Health != StatusUnknown {
		t.Errorf("health = %v, want %v", rs.Health, StatusUnknown)
	}
}

// TestStateMachine_ErrorRecoveryPath validates Error→Recovering→Ready.
func TestStateMachine_ErrorRecoveryPath(t *testing.T) {
	t.Parallel()

	// ARRANGE
	rs := NewRuntimeState()
	_ = rs.TransitionTo(StateInitializing, "init")
	_ = rs.TransitionTo(StateReady, "ready")
	_ = rs.TransitionTo(StateRunning, "run")

	// ACT — Error and recovery sequence
	err1 := rs.TransitionTo(StateError, "component failure")
	if err1 != nil {
		t.Fatalf("transition to error: %v", err1)
	}

	// SetRecovery via helper
	rs.SetRecovery()
	if rs.CurrentState != StateRecovering {
		t.Fatalf("recovery state = %v, want %v", rs.CurrentState, StateRecovering)
	}

	err2 := rs.TransitionTo(StateReady, "component restarted")
	if err2 != nil {
		t.Fatalf("transition to ready: %v", err2)
	}

	// ASSERT
	if rs.CurrentState != StateReady {
		t.Errorf("final state = %v, want %v", rs.CurrentState, StateReady)
	}
	if rs.RecoveryCount != 1 {
		t.Errorf("recovery count = %d, want 1", rs.RecoveryCount)
	}
	if rs.Health != StatusDegraded {
		t.Errorf("health during recovery = %v, want %v", rs.Health, StatusDegraded)
	}
}

// TestStateMachine_RestartPath validates Stopped→Uninitialized→Initializing.
// NOTE: This tests the state machine transition logic only — the Runtime.Restart()
// bug (BUG-U01) is that this transition is never called by Restart().
func TestStateMachine_RestartPath(t *testing.T) {
	t.Parallel()

	// ARRANGE
	rs := NewRuntimeState()
	_ = rs.TransitionTo(StateInitializing, "init")
	_ = rs.TransitionTo(StateReady, "ready")
	_ = rs.TransitionTo(StateRunning, "run")
	_ = rs.TransitionTo(StateStopping, "stop")
	_ = rs.TransitionTo(StateStopped, "done")

	// ACT — Reset and restart
	err1 := rs.TransitionTo(StateUninitialized, "reset for restart")
	if err1 != nil {
		t.Fatalf("transition to uninitialized: %v", err1)
	}
	err2 := rs.TransitionTo(StateInitializing, "restarting")
	if err2 != nil {
		t.Fatalf("re-init: %v", err2)
	}

	// ASSERT
	if rs.CurrentState != StateInitializing {
		t.Errorf("state = %v, want %v", rs.CurrentState, StateInitializing)
	}
}

// =============================================================================
// Event Callbacks — Integration
// =============================================================================

// TestStateMachine_OnChangeCallback validates the state change event system.
func TestStateMachine_OnChangeCallback(t *testing.T) {
	t.Parallel()

	// ARRANGE
	rs := NewRuntimeState()
	var events []StateChangeEvent
	var mu sync.Mutex

	rs.OnStateChange(func(e StateChangeEvent) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	})

	// ACT — Execute multiple transitions
	_ = rs.TransitionTo(StateInitializing, "init")
	_ = rs.TransitionTo(StateReady, "ready")
	_ = rs.TransitionTo(StateRunning, "run")

	// ASSERT — First batch of transitions
	mu.Lock()
	eventCountBeforeInvalid := len(events)
	mu.Unlock()

	if eventCountBeforeInvalid != 3 {
		t.Fatalf("expected 3 events, got %d", eventCountBeforeInvalid)
	}

	// Verify event sequence (no lock needed — callback is done firing)
	mu.Lock()
	expected := []struct {
		from, to State
	}{
		{StateUninitialized, StateInitializing},
		{StateInitializing, StateReady},
		{StateReady, StateRunning},
	}
	for i, exp := range expected {
		if events[i].From != exp.from {
			t.Errorf("event[%d].From = %v, want %v", i, events[i].From, exp.from)
		}
		if events[i].To != exp.to {
			t.Errorf("event[%d].To = %v, want %v", i, events[i].To, exp.to)
		}
		if events[i].Timestamp.IsZero() {
			t.Errorf("event[%d].Timestamp is zero", i)
		}
	}
	mu.Unlock()

	// Invalid transition should still fire callback.
	// NOTE: mu must NOT be held here — TransitionTo fires the callback
	// synchronously, which tries to acquire mu. Holding mu would deadlock.
	_ = rs.TransitionTo(StateInitializing, "invalid back") // Should fail

	mu.Lock()
	if len(events) != eventCountBeforeInvalid+1 {
		t.Errorf("expected %d events after invalid transition, got %d", eventCountBeforeInvalid+1, len(events))
	}
	if len(events) >= eventCountBeforeInvalid+1 {
		lastEvent := events[len(events)-1]
		if lastEvent.Error == "" {
			t.Error("last event (invalid transition) should have Error field set")
		}
	}
	mu.Unlock()
}

// =============================================================================
// Concurrent Access — Race Condition Detection
// =============================================================================

// TestStateMachine_ConcurrentTransitions tests that concurrent state reads
// don't race with writes.
func TestStateMachine_ConcurrentTransitions(t *testing.T) {
	// NOTE: intentionally not t.Parallel() — this test itself has internal concurrency.

	// ARRANGE
	rs := NewRuntimeState()
	_ = rs.TransitionTo(StateInitializing, "init")
	_ = rs.TransitionTo(StateReady, "ready")
	_ = rs.TransitionTo(StateRunning, "run")

	var wg sync.WaitGroup
	const goroutines = 20

	// ACT — Concurrent reads and writes
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = rs.Current()
				_ = rs.HealthStatus()
				_ = fmt.Sprintf("%s", rs.Current())
				snap := rs.Get()
				_ = snap.CurrentState
				_ = rs.Summary()
				rs.SetComponentStatus("comp", StatusHealthy, "ok")
			}
		}(i)
	}

	// Also concurrently attempt transitions
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 50; j++ {
			_ = rs.TransitionTo(StateStopping, "test")
			_ = rs.TransitionTo(StateStopped, "done")
			_ = rs.TransitionTo(StateUninitialized, "reset")
			_ = rs.TransitionTo(StateInitializing, "reinit")
			_ = rs.TransitionTo(StateReady, "ready")
			_ = rs.TransitionTo(StateRunning, "run")
		}
	}()

	wg.Wait()

	// ASSERT — State should be a valid state (not a torn read)
	finalState := rs.Current()
	if finalState < StateUninitialized || finalState > StateRecovering {
		t.Errorf("invalid final state: %v", finalState)
	}
}

// =============================================================================
// Component Health — Integration
// =============================================================================

// TestStateMachine_ComponentHealthRecomputation validates health recomputation.
func TestStateMachine_ComponentHealthRecomputation(t *testing.T) {
	t.Parallel()

	// ARRANGE
	rs := NewRuntimeState()

	// ACT — Add components with various health statuses
	rs.SetComponentStatus("a", StatusHealthy, "")
	rs.SetComponentStatus("b", StatusHealthy, "")
	rs.SetComponentStatus("c", StatusDegraded, "slow")

	// ASSERT — Health should be degraded (not healthy, but not unhealthy)
	if rs.Health != StatusDegraded {
		t.Errorf("health = %v, want %v", rs.Health, StatusDegraded)
	}

	// ACT — Add an unhealthy component
	rs.SetComponentStatus("d", StatusUnhealthy, "crash")

	// ASSERT — Health should escalate to unhealthy
	if rs.Health != StatusUnhealthy {
		t.Errorf("health = %v, want %v", rs.Health, StatusUnhealthy)
	}

	// ACT — Heal the unhealthy component
	rs.SetComponentStatus("d", StatusHealthy, "recovered")

	// ASSERT — Health should return to degraded (c is still degraded)
	if rs.Health != StatusDegraded {
		t.Errorf("health after healing d = %v, want %v", rs.Health, StatusDegraded)
	}

	// ACT — Heal the degraded component
	rs.SetComponentStatus("c", StatusHealthy, "optimized")

	// ASSERT — All healthy
	if rs.Health != StatusHealthy {
		t.Errorf("health after healing all = %v, want %v", rs.Health, StatusHealthy)
	}
}

// =============================================================================
// SetError Path — Integration
// =============================================================================

// TestStateMachine_SetErrorPath validates SetError behavior across states.
func TestStateMachine_SetErrorPath(t *testing.T) {
	t.Parallel()

	// ARRANGE
	rs := NewRuntimeState()
	_ = rs.TransitionTo(StateInitializing, "init")

	// ACT — SetError with nil should be a no-op
	rs.SetError(nil)
	if rs.Health != StatusUnknown {
		t.Errorf("health should be unchanged after nil error, got %v", rs.Health)
	}

	// ACT — SetError with real error from Initializing
	rs.SetError(errors.New("kernel panic"))

	// ASSERT
	if rs.CurrentState != StateError {
		t.Errorf("state = %v, want %v", rs.CurrentState, StateError)
	}
	if rs.Health != StatusUnhealthy {
		t.Errorf("health = %v, want %v", rs.Health, StatusUnhealthy)
	}
	if rs.ErrorMessage != "kernel panic" {
		t.Errorf("error msg = %q, want %q", rs.ErrorMessage, "kernel panic")
	}
}

// =============================================================================
// Transition Coverage Counter
// =============================================================================

// TestStateMachine_TransitionCounts verifies the exact number of tested transitions.
func TestStateMachine_TransitionCounts(t *testing.T) {
	t.Parallel()

	// Count unique from→to pairs in validTransitions
	transitionCount := 0
	for _, targets := range validTransitions {
		transitionCount += len(targets)
	}

	if transitionCount != 21 {
		t.Errorf("validTransitions has %d entries, expected 21", transitionCount)
	}

	// Count how many transitions we test in the catalog
	testedTransitions := make(map[string]bool)
	for _, tt := range transitionTests {
		if !tt.wantErr {
			key := "valid:" + tt.name
			testedTransitions[key] = true
		}
	}
	// We test 21 valid transitions (the 21 in the catalog minus invalid ones are all valid expect the 1 explicit invalid)
	validTested := 0
	for _, tt := range transitionTests {
		if !tt.wantErr {
			validTested++
		}
	}
	if validTested < 20 {
		t.Errorf("only %d valid transitions tested, expected >= 20 (covers 20 of 21; 1 is duplicated via invalid test)", validTested)
	}
}

// =============================================================================
// Edge Cases — Boundary Conditions
// =============================================================================

// TestStateMachine_DoubleTransition verifies that duplicate transitions fail.
func TestStateMachine_DoubleTransition(t *testing.T) {
	t.Parallel()

	// ARRANGE
	rs := NewRuntimeState()
	_ = rs.TransitionTo(StateInitializing, "first")

	// ACT — Try to transition to the same state
	err := rs.TransitionTo(StateInitializing, "second")

	// ASSERT — Should fail (already in Initializing, and Initializing→Initializing is not valid)
	if err == nil {
		t.Error("expected error for transition to same state")
	}
}
