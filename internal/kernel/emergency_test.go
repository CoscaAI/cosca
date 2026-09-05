package kernel

import (
	"errors"
	"sync"
	"testing"
)

// TestEmergencyManager_TriggerStop verifies that TriggerStop transitions the
// state to EmergencyStop, records the halt time and reason, and invokes the
// registered shutdown function with the given reason.
func TestEmergencyManager_TriggerStop(t *testing.T) {
	m := NewEmergencyManager()

	var (
		mu        sync.Mutex
		called    bool
		gotReason string
	)
	m.SetShutdownFn(func(reason string) {
		mu.Lock()
		defer mu.Unlock()
		called = true
		gotReason = reason
	})

	if err := m.TriggerStop("daemon compromised"); err != nil {
		t.Fatalf("TriggerStop returned error: %v", err)
	}

	if got := m.State(); got != EmergencyStop {
		t.Errorf("State() = %q, want %q", got, EmergencyStop)
	}
	if got := m.Reason(); got != "daemon compromised" {
		t.Errorf("Reason() = %q, want %q", got, "daemon compromised")
	}
	if m.HaltedAt().IsZero() {
		t.Error("HaltedAt() must be set after TriggerStop")
	}

	mu.Lock()
	defer mu.Unlock()
	if !called {
		t.Error("shutdown function was not called")
	}
	if gotReason != "daemon compromised" {
		t.Errorf("shutdown function got reason %q, want %q", gotReason, "daemon compromised")
	}
}

// TestEmergencyManager_IsHalted verifies IsHalted is false before any trigger
// and true once the kill switch has been pulled.
func TestEmergencyManager_IsHalted(t *testing.T) {
	m := NewEmergencyManager()

	if m.IsHalted() {
		t.Error("IsHalted() = true before any trigger")
	}

	if err := m.TriggerStop("halted"); err != nil {
		t.Fatalf("TriggerStop returned error: %v", err)
	}
	if !m.IsHalted() {
		t.Error("IsHalted() = false after trigger")
	}
}

// TestEmergencyManager_EscalationStopToHalt verifies that triggering a halt
// after a plain stop escalates the state to EmergencyHalted without error.
func TestEmergencyManager_EscalationStopToHalt(t *testing.T) {
	m := NewEmergencyManager()

	if err := m.TriggerStop("first stop"); err != nil {
		t.Fatalf("TriggerStop(plain) returned error: %v", err)
	}
	if got := m.State(); got != EmergencyStop {
		t.Fatalf("State() = %q, want %q", got, EmergencyStop)
	}

	if err := m.TriggerStop("halted"); err != nil {
		t.Fatalf("TriggerStop(halt) should escalate without error, got: %v", err)
	}
	if got := m.State(); got != EmergencyHalted {
		t.Errorf("State() = %q, want %q (escalated)", got, EmergencyHalted)
	}
	if got := m.Reason(); got != "halted" {
		t.Errorf("Reason() = %q, want %q", got, "halted")
	}
}

// TestEmergencyManager_DuplicateError verifies that a second trigger of the
// same state returns ErrEmergencyAlreadyTriggered and preserves the state.
func TestEmergencyManager_DuplicateError(t *testing.T) {
	m := NewEmergencyManager()

	if err := m.TriggerStop("one"); err != nil {
		t.Fatalf("first TriggerStop returned error: %v", err)
	}

	err := m.TriggerStop("two")
	if err == nil {
		t.Fatal("second TriggerStop returned nil, want ErrEmergencyAlreadyTriggered")
	}
	if !errors.Is(err, ErrEmergencyAlreadyTriggered) {
		t.Errorf("got error %v, want ErrEmergencyAlreadyTriggered", err)
	}
	if got := m.State(); got != EmergencyStop {
		t.Errorf("State() = %q, want %q preserved", got, EmergencyStop)
	}
	if got := m.Reason(); got != "one" {
		t.Errorf("Reason() = %q, want first reason %q preserved", got, "one")
	}
}

// TestEmergencyManager_HaltThenHalt verifies that a second halt (already
// halted) returns ErrEmergencyAlreadyTriggered.
func TestEmergencyManager_HaltThenHalt(t *testing.T) {
	m := NewEmergencyManager()

	if err := m.TriggerStop("halted"); err != nil {
		t.Fatalf("first halt returned error: %v", err)
	}
	if err := m.TriggerStop("halted"); err == nil {
		t.Fatal("second halt returned nil, want ErrEmergencyAlreadyTriggered")
	}
}

// TestEmergencyManager_Clear verifies Clear resets the manager to
// EmergencyNone, clearing reason and halted time.
func TestEmergencyManager_Clear(t *testing.T) {
	m := NewEmergencyManager()

	if err := m.TriggerStop("done"); err != nil {
		t.Fatalf("TriggerStop returned error: %v", err)
	}
	m.Clear()

	if got := m.State(); got != EmergencyNone {
		t.Errorf("State() = %q, want %q", got, EmergencyNone)
	}
	if m.IsHalted() {
		t.Error("IsHalted() = true after Clear")
	}
	if got := m.Reason(); got != "" {
		t.Errorf("Reason() = %q, want empty after Clear", got)
	}
	if !m.HaltedAt().IsZero() {
		t.Error("HaltedAt() must be zero after Clear")
	}

	// The kill switch can be pulled again after Clear (daemon restart).
	if err := m.TriggerStop("again"); err != nil {
		t.Errorf("TriggerStop after Clear returned error: %v", err)
	}
	if got := m.State(); got != EmergencyStop {
		t.Errorf("State() = %q, want %q after re-trigger", got, EmergencyStop)
	}
}

// TestEmergencyManager_ShutdownFnNotCalledWhenNil verifies a nil shutdown
// function is tolerated (no panic).
func TestEmergencyManager_ShutdownFnNotCalledWhenNil(t *testing.T) {
	m := NewEmergencyManager()
	if err := m.TriggerStop("silent"); err != nil {
		t.Fatalf("TriggerStop with nil shutdown function returned error: %v", err)
	}
	if !m.IsHalted() {
		t.Error("IsHalted() = false after trigger")
	}
}

// TestEmergencyManager_TriggerStopHaltedReason verifies the reserved reason
// "halted" (case-insensitive) produces the EmergencyHalted state.
func TestEmergencyManager_TriggerStopHaltedReason(t *testing.T) {
	m := NewEmergencyManager()
	if err := m.TriggerStop("HALTED"); err != nil {
		t.Fatalf("TriggerStop returned error: %v", err)
	}
	if got := m.State(); got != EmergencyHalted {
		t.Errorf("State() = %q, want %q", got, EmergencyHalted)
	}
}
