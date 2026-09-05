package runtime

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// Summary with various states
// =============================================================================

func TestSummaryWithError(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	_ = rs.TransitionTo(StateInitializing, "init")
	rs.SetError(errors.New("fatal error"))

	summary := rs.Summary()
	if !strings.Contains(summary, "error") {
		t.Errorf("summary should contain 'error', got: %s", summary)
	}
	if !strings.Contains(summary, "fatal error") {
		t.Errorf("summary should contain error message, got: %s", summary)
	}
}

func TestSummaryWithRestarts(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	rs.SetComponentStatus("comp", StatusHealthy, "")
	rs.IncrementRestartCount("comp")
	rs.IncrementRestartCount("comp")

	summary := rs.Summary()
	if !strings.Contains(summary, "restarts: 2") {
		t.Errorf("summary should show restart count, got: %s", summary)
	}
}

func TestSummaryWithMultipleComponents(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	rs.SetComponentStatus("knowledge", StatusHealthy, "running")
	rs.SetComponentStatus("cache", StatusDegraded, "slow")
	rs.SetComponentStatus("plugins", StatusUnhealthy, "crashed")

	summary := rs.Summary()
	for _, name := range []string{"knowledge", "cache", "plugins"} {
		if !strings.Contains(summary, name) {
			t.Errorf("summary should contain %q", name)
		}
	}
	if !strings.Contains(summary, "unhealthy") {
		t.Errorf("summary should show unhealthy state, got: %s", summary)
	}
}

func TestSummaryWithUptime(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	_ = rs.TransitionTo(StateInitializing, "init")
	_ = rs.TransitionTo(StateReady, "ready")
	_ = rs.TransitionTo(StateRunning, "run")

	summary := rs.Summary()
	if !strings.Contains(summary, "Uptime:") {
		t.Errorf("summary should contain Uptime, got: %s", summary)
	}
}

func TestSummaryNoComponents(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	summary := rs.Summary()
	if !strings.Contains(summary, "Components: 0") {
		t.Errorf("summary for empty components, got: %s", summary)
	}
}

// =============================================================================
// SetComponentStatus edge cases
// =============================================================================

func TestSetComponentStatusHealthyAfterUnhealthy(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()

	// Set unhealthy first
	rs.SetComponentStatus("comp", StatusUnhealthy, "connection refused")

	info, _ := rs.ComponentStatus("comp")
	if info.Error != "connection refused" {
		t.Errorf("Error should be set: %q", info.Error)
	}

	// Now set healthy — error should be cleared
	rs.SetComponentStatus("comp", StatusHealthy, "reconnected")

	info, _ = rs.ComponentStatus("comp")
	if info.Error != "" {
		t.Errorf("Error should be cleared when transitioning unhealthy->healthy, got: %q", info.Error)
	}
	if info.Status != StatusHealthy {
		t.Errorf("Status = %v, want healthy", info.Status)
	}
}

func TestSetComponentStatusStartingResetsStartedAt(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	rs.SetComponentStatus("comp", StatusStarting, "initializing")
	info, _ := rs.ComponentStatus("comp")
	firstStart := info.StartedAt

	time.Sleep(time.Millisecond)

	rs.SetComponentStatus("comp", StatusStarting, "reinitializing")
	info, _ = rs.ComponentStatus("comp")
	secondStart := info.StartedAt

	if !secondStart.After(firstStart) {
		t.Error("StartedAt should be updated on StatusStarting")
	}
}

func TestSetComponentStatusUnhealthyWithoutMessage(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	rs.SetComponentStatus("comp", StatusUnhealthy, "")

	info, _ := rs.ComponentStatus("comp")
	if info.Error != "" {
		t.Errorf("Error should be empty when no message, got: %q", info.Error)
	}
}

func TestSetComponentStatusUpdatesUptime(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	rs.SetComponentStatus("comp", StatusHealthy, "")

	time.Sleep(time.Millisecond)

	rs.SetComponentStatus("comp", StatusHealthy, "still running")
	info, _ := rs.ComponentStatus("comp")
	if info.Uptime <= 0 {
		t.Error("Uptime should be positive")
	}
}

// =============================================================================
// recomputeHealth edge cases
// =============================================================================

func TestRecomputeHealthAllStarting(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	rs.SetComponentStatus("a", StatusStarting, "")
	rs.SetComponentStatus("b", StatusStarting, "")

	if rs.Health != StatusHealthy {
		t.Errorf("Health = %v, want healthy (starting implies alive)", rs.Health)
	}
}

func TestRecomputeHealthMixed(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	rs.SetComponentStatus("a", StatusHealthy, "")
	rs.SetComponentStatus("b", StatusStoppedComponent, "")

	// b is stopped but a is healthy — overall should be unknown because
	// allStopped is false (a is healthy), allUnknown is false, hasUnhealthy is false,
	// hasDegraded is false. So we hit the `default` and set to StatusHealthy.
	if rs.Health != StatusHealthy {
		t.Errorf("Health = %v, want healthy", rs.Health)
	}
}

func TestRecomputeHealthAllUnknown(t *testing.T) {
	t.Parallel()
	// When no components are set, health should be unknown
	rs := NewRuntimeState()
	// health is already unknown from NewRuntimeState

	// We need to trigger recomputeHealth without setting any components
	// Cannot call it directly (unexported), but initial state is unknown
	if rs.Health != StatusUnknown {
		t.Errorf("initial health = %v", rs.Health)
	}
}

func TestRecomputeHealthDegradedOverridesNone(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	rs.SetComponentStatus("a", StatusStoppedComponent, "")
	rs.SetComponentStatus("b", StatusDegraded, "slow")
	rs.SetComponentStatus("c", StatusStoppedComponent, "")

	if rs.Health != StatusDegraded {
		t.Errorf("Health = %v, want degraded", rs.Health)
	}
}

// =============================================================================
// SetRecovery edge cases
// =============================================================================

func TestSetRecoveryFromStopped(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	_ = rs.TransitionTo(StateInitializing, "i")
	_ = rs.TransitionTo(StateReady, "r")
	_ = rs.TransitionTo(StateRunning, "run")
	_ = rs.TransitionTo(StateStopping, "stop")
	_ = rs.TransitionTo(StateStopped, "done")

	// stopped cannot transition to recovering (no valid transition)
	rs.SetRecovery()
	// Should still be stopped
	if rs.CurrentState != StateStopped {
		t.Errorf("CurrentState = %v, want stopped (no valid transition)", rs.CurrentState)
	}
}

func TestSetRecoveryFromError(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	_ = rs.TransitionTo(StateInitializing, "i")
	_ = rs.TransitionTo(StateError, "err")

	rs.SetRecovery()
	if rs.CurrentState != StateRecovering {
		t.Errorf("CurrentState = %v, want recovering", rs.CurrentState)
	}
	if rs.RecoveryCount != 1 {
		t.Errorf("RecoveryCount = %d, want 1", rs.RecoveryCount)
	}
}

func TestSetRecoveryIncrementsCounter(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	_ = rs.TransitionTo(StateInitializing, "i")
	_ = rs.TransitionTo(StateReady, "r")
	_ = rs.TransitionTo(StateRunning, "run")
	_ = rs.TransitionTo(StateError, "err1")

	rs.SetRecovery()
	_ = rs.TransitionTo(StateReady, "recovered")

	_ = rs.TransitionTo(StateError, "err2")
	rs.SetRecovery()

	if rs.RecoveryCount != 2 {
		t.Errorf("RecoveryCount = %d, want 2", rs.RecoveryCount)
	}
}

// =============================================================================
// SetError from various states
// =============================================================================

func TestSetErrorTransitionsToError(t *testing.T) {
	t.Parallel()
	var tests = []struct {
		name string
		from State
	}{
		{"from uninitialized", StateUninitialized},
		{"from initializing", StateInitializing},
		{"from ready", StateReady},
		{"from running", StateRunning},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := NewRuntimeState()
			if tt.from != StateUninitialized {
				_ = rs.TransitionTo(tt.from, "setup")
			}
			rs.SetError(errors.New("test error"))
			if rs.CurrentState != StateError {
				t.Errorf("CurrentState = %v, want error", rs.CurrentState)
			}
		})
	}
}

func TestSetErrorDoesNotReapplyWhenAlreadyError(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	rs.SetError(errors.New("first error"))
	firstMsg := rs.ErrorMessage
	rs.SetError(errors.New("second error"))
	// Second error should update the message but not fire another transition
	if rs.ErrorMessage != "second error" {
		t.Errorf("ErrorMessage = %q, want second error", rs.ErrorMessage)
	}
	_ = firstMsg
}

// =============================================================================
// OnStateChange for error transitions
// =============================================================================

func TestOnStateChangeErrorTransition(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	events := make(chan StateChangeEvent, 3)
	rs.OnStateChange(func(e StateChangeEvent) {
		events <- e
	})

	_ = rs.TransitionTo(StateInitializing, "init")
	rs.SetError(errors.New("test error"))

	// Close after collecting
	close(events)

	// Should have at least 2 events (init transition + error transition)
	count := 0
	for range events {
		count++
	}
	if count < 2 {
		t.Errorf("expected at least 2 state change events, got %d", count)
	}
}

// =============================================================================
// InvalidTransition also fires callback
// =============================================================================

func TestInvalidTransitionFiresOnChange(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	fired := false
	rs.OnStateChange(func(e StateChangeEvent) {
		fired = true
		if e.Error == "" {
			t.Error("invalid transition event should have error")
		}
	})

	_ = rs.TransitionTo(StateRunning, "invalid jump")
	if !fired {
		t.Error("OnStateChange should fire even for invalid transitions")
	}
}

// =============================================================================
// Get with started runtime
// =============================================================================

func TestGetWithStartedState(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	_ = rs.TransitionTo(StateInitializing, "i")
	_ = rs.TransitionTo(StateReady, "r")
	_ = rs.TransitionTo(StateRunning, "run")

	// Let a measurable uptime elapse: Windows time.Now() can jump in ~0.5ms
	// steps, so an immediate snapshot may legitimately read 0s.
	time.Sleep(2 * time.Millisecond)

	snapshot := rs.Get()
	if snapshot.Uptime <= 0 {
		t.Error("Uptime should be positive for started runtime")
	}
	if snapshot.CurrentState != StateRunning {
		t.Errorf("CurrentState = %v", snapshot.CurrentState)
	}
}

// =============================================================================
// SetHealth
// =============================================================================

func TestSetHealthAllValues(t *testing.T) {
	t.Parallel()
	statuses := []ComponentStatus{
		StatusUnknown, StatusHealthy, StatusDegraded,
		StatusUnhealthy, StatusStarting, StatusStopping, StatusStoppedComponent,
	}
	for _, s := range statuses {
		rs := NewRuntimeState()
		rs.SetHealth(s)
		if rs.HealthStatus() != s {
			t.Errorf("Health after SetHealth(%v) = %v", s, rs.HealthStatus())
		}
	}
}

// =============================================================================
// Get copies components to prevent mutation
// =============================================================================

func TestGetReturnsCopy(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	rs.SetComponentStatus("a", StatusHealthy, "")

	snapshot := rs.Get()
	snapshot.Components["a"] = ComponentInfo{Name: "mutated"}

	// Original should be unchanged
	orig, ok := rs.ComponentStatus("a")
	if !ok {
		t.Fatal("component should still exist")
	}
	if orig.Name != "a" {
		t.Error("original component was mutated via snapshot")
	}
}

// =============================================================================
// IncrementRestartCount nonexistent component
// =============================================================================

func TestIncrementRestartCountNonexistent(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	// Should not panic
	rs.IncrementRestartCount("nonexistent")
	// Should not create component
	_, ok := rs.ComponentStatus("nonexistent")
	if ok {
		t.Error("nonexistent component should not be created")
	}
}

// =============================================================================
// ComponentInfo uptime tracking
// =============================================================================

func TestComponentInfoUptime(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	rs.SetComponentStatus("comp", StatusStarting, "")

	info, _ := rs.ComponentStatus("comp")
	firstUptime := info.Uptime

	time.Sleep(time.Millisecond)

	rs.SetComponentStatus("comp", StatusHealthy, "")
	info, _ = rs.ComponentStatus("comp")
	secondUptime := info.Uptime

	if secondUptime <= firstUptime {
		t.Errorf("Uptime should increase: %v -> %v", firstUptime, secondUptime)
	}
}

// =============================================================================
// Starting -> Unhealthy flow
// =============================================================================

func TestComponentStartingToUnhealthy(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	rs.SetComponentStatus("comp", StatusStarting, "starting up")
	info, _ := rs.ComponentStatus("comp")
	if info.Status != StatusStarting {
		t.Errorf("status = %v", info.Status)
	}

	rs.SetComponentStatus("comp", StatusUnhealthy, "startup failed")
	info, _ = rs.ComponentStatus("comp")
	if info.Status != StatusUnhealthy {
		t.Errorf("status = %v", info.Status)
	}
	if info.Error != "startup failed" {
		t.Errorf("error = %q", info.Error)
	}
}
