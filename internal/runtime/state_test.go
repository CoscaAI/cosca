package runtime

import (
	"encoding"
	"errors"
	"testing"
)

func TestStateString(t *testing.T) {
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
		{State(99), "unknown(99)"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("State.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStateMarshalText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		state State
		want  string
	}{
		{StateUninitialized, "uninitialized"},
		{StateRunning, "running"},
		{StateStopped, "stopped"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got, err := tt.state.MarshalText()
			if err != nil {
				t.Fatalf("MarshalText error: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("MarshalText = %q, want %q", string(got), tt.want)
			}
		})
	}
}

func TestStateUnmarshalText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text    string
		want    State
		wantErr bool
	}{
		{"uninitialized", StateUninitialized, false},
		{"initializing", StateInitializing, false},
		{"ready", StateReady, false},
		{"running", StateRunning, false},
		{"stopping", StateStopping, false},
		{"stopped", StateStopped, false},
		{"error", StateError, false},
		{"recovering", StateRecovering, false},
		{"unknown", State(0), true},
		{"", State(0), true},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			var s State
			sPtr := &s
			um, ok := interface{}(sPtr).(encoding.TextUnmarshaler)
			if !ok {
				t.Fatal("State does not implement TextUnmarshaler")
			}
			err := um.UnmarshalText([]byte(tt.text))
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalText error = %v, wantErr = %v", err, tt.wantErr)
			}
			if !tt.wantErr && s != tt.want {
				t.Errorf("UnmarshalText = %v, want %v", s, tt.want)
			}
		})
	}
}

func TestValidTransitions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		from  State
		to    State
		valid bool
	}{
		{"uninitialized->initializing", StateUninitialized, StateInitializing, true},
		{"uninitialized->error", StateUninitialized, StateError, true},
		{"uninitialized->running", StateUninitialized, StateRunning, false},
		{"uninitialized->stopped", StateUninitialized, StateStopped, false},
		{"initializing->ready", StateInitializing, StateReady, true},
		{"initializing->error", StateInitializing, StateError, true},
		{"initializing->stopping", StateInitializing, StateStopping, true},
		{"initializing->running", StateInitializing, StateRunning, false},
		{"ready->running", StateReady, StateRunning, true},
		{"ready->stopping", StateReady, StateStopping, true},
		{"ready->error", StateReady, StateError, true},
		{"ready->initializing", StateReady, StateInitializing, false},
		{"running->stopping", StateRunning, StateStopping, true},
		{"running->error", StateRunning, StateError, true},
		{"running->recovering", StateRunning, StateRecovering, true},
		{"running->ready", StateRunning, StateReady, true},
		{"running->initializing", StateRunning, StateInitializing, false},
		{"stopping->stopped", StateStopping, StateStopped, true},
		{"stopping->error", StateStopping, StateError, true},
		{"stopping->running", StateStopping, StateRunning, false},
		{"stopped->any", StateStopped, StateRunning, false},
		{"stopped->stopped", StateStopped, StateStopped, false},
		{"error->recovering", StateError, StateRecovering, true},
		{"error->stopping", StateError, StateStopping, true},
		{"error->uninitialized", StateError, StateUninitialized, true},
		{"error->ready", StateError, StateReady, false},
		{"recovering->ready", StateRecovering, StateReady, true},
		{"recovering->error", StateRecovering, StateError, true},
		{"recovering->stopping", StateRecovering, StateStopping, true},
		{"recovering->running", StateRecovering, StateRunning, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := canTransitionTo(tt.from, tt.to)
			if got != tt.valid {
				t.Errorf("canTransitionTo(%v, %v) = %v, want %v", tt.from, tt.to, got, tt.valid)
			}
		})
	}
}

func TestComponentStatusConstants(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status ComponentStatus
		want   string
	}{
		{StatusUnknown, "unknown"},
		{StatusHealthy, "healthy"},
		{StatusDegraded, "degraded"},
		{StatusUnhealthy, "unhealthy"},
		{StatusStarting, "starting"},
		{StatusStopping, "stopping"},
		{StatusStoppedComponent, "stopped"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.status) != tt.want {
				t.Errorf("ComponentStatus = %q, want %q", string(tt.status), tt.want)
			}
		})
	}
}

func TestNewRuntimeState(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	if rs.CurrentState != StateUninitialized {
		t.Errorf("CurrentState = %v, want %v", rs.CurrentState, StateUninitialized)
	}
	if rs.Health != StatusUnknown {
		t.Errorf("Health = %v, want %v", rs.Health, StatusUnknown)
	}
	if rs.Components == nil {
		t.Error("Components map should not be nil")
	}
	if len(rs.Components) != 0 {
		t.Errorf("Components should be empty, got %d", len(rs.Components))
	}
}

func TestRuntimeStateGet(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	snapshot := rs.Get()
	if snapshot.CurrentState != StateUninitialized {
		t.Errorf("snapshot CurrentState = %v", snapshot.CurrentState)
	}
	if snapshot.Health != StatusUnknown {
		t.Errorf("snapshot Health = %v", snapshot.Health)
	}
}

func TestTransitionToValid(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	err := rs.TransitionTo(StateInitializing, "test startup")
	if err != nil {
		t.Fatalf("TransitionTo failed: %v", err)
	}
	if rs.CurrentState != StateInitializing {
		t.Errorf("CurrentState = %v, want %v", rs.CurrentState, StateInitializing)
	}
	if rs.PreviousState != StateUninitialized {
		t.Errorf("PreviousState = %v, want %v", rs.PreviousState, StateUninitialized)
	}
}

func TestTransitionToInvalid(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	err := rs.TransitionTo(StateRunning, "invalid jump")
	if err == nil {
		t.Fatal("Expected error for invalid transition")
	}
	if rs.CurrentState != StateUninitialized {
		t.Errorf("CurrentState should remain uninitialized, got %v", rs.CurrentState)
	}
}

func TestTransitionToRunningSetsStartedAt(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	_ = rs.TransitionTo(StateInitializing, "init")
	_ = rs.TransitionTo(StateReady, "ready")
	_ = rs.TransitionTo(StateRunning, "running")
	if rs.StartedAt.IsZero() {
		t.Error("StartedAt should be set when transitioning to running")
	}
}

func TestTransitionToErrorSetsHealth(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	_ = rs.TransitionTo(StateInitializing, "init")
	err := rs.TransitionTo(StateError, "something failed")
	if err != nil {
		t.Fatalf("TransitionTo error failed: %v", err)
	}
	if rs.Health != StatusUnhealthy {
		t.Errorf("Health = %v, want %v", rs.Health, StatusUnhealthy)
	}
	if rs.ErrorMessage != "something failed" {
		t.Errorf("ErrorMessage = %q, want %q", rs.ErrorMessage, "something failed")
	}
}

func TestTransitionToStoppedResetsHealth(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	_ = rs.TransitionTo(StateInitializing, "init")
	_ = rs.TransitionTo(StateReady, "ready")
	_ = rs.TransitionTo(StateRunning, "run")
	_ = rs.TransitionTo(StateStopping, "stop")
	_ = rs.TransitionTo(StateStopped, "done")
	if rs.Health != StatusUnknown {
		t.Errorf("Health after stop = %v, want %v", rs.Health, StatusUnknown)
	}
}

func TestHealthStatus(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	if h := rs.HealthStatus(); h != StatusUnknown {
		t.Errorf("HealthStatus = %v, want %v", h, StatusUnknown)
	}
	rs.SetHealth(StatusDegraded)
	if h := rs.HealthStatus(); h != StatusDegraded {
		t.Errorf("HealthStatus = %v, want %v", h, StatusDegraded)
	}
}

func TestSetComponentStatusAddsComponent(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	rs.SetComponentStatus("test-comp", StatusHealthy, "running")
	info, ok := rs.ComponentStatus("test-comp")
	if !ok {
		t.Fatal("ComponentStatus should return ok for existing component")
	}
	if info.Status != StatusHealthy {
		t.Errorf("Status = %v, want %v", info.Status, StatusHealthy)
	}
	if info.Name != "test-comp" {
		t.Errorf("Name = %q, want %q", info.Name, "test-comp")
	}
}

func TestSetComponentStatusStarting(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	rs.SetComponentStatus("comp", StatusStarting, "initializing")
	info, ok := rs.ComponentStatus("comp")
	if !ok {
		t.Fatal("Component not found")
	}
	if info.Status != StatusStarting {
		t.Errorf("Status = %v, want %v", info.Status, StatusStarting)
	}
	if info.StartedAt.IsZero() {
		t.Error("StartedAt should be set for StatusStarting")
	}
}

func TestSetComponentStatusUnhealthySetsError(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	rs.SetComponentStatus("comp", StatusUnhealthy, "connection refused")
	info, ok := rs.ComponentStatus("comp")
	if !ok {
		t.Fatal("Component not found")
	}
	if info.Error != "connection refused" {
		t.Errorf("Error = %q, want %q", info.Error, "connection refused")
	}
}

func TestComponentStatusNotFound(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	_, ok := rs.ComponentStatus("nonexistent")
	if ok {
		t.Error("Expected ok=false for nonexistent component")
	}
}

func TestAllComponentStatuses(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	rs.SetComponentStatus("a", StatusHealthy, "")
	rs.SetComponentStatus("b", StatusDegraded, "")
	all := rs.AllComponentStatuses()
	if len(all) != 2 {
		t.Errorf("Expected 2 components, got %d", len(all))
	}
}

func TestIncrementRestartCount(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	rs.SetComponentStatus("comp", StatusUnhealthy, "")
	info, _ := rs.ComponentStatus("comp")
	if info.Restarts != 0 {
		t.Errorf("Initial Restarts = %d, want 0", info.Restarts)
	}
	rs.IncrementRestartCount("comp")
	info, _ = rs.ComponentStatus("comp")
	if info.Restarts != 1 {
		t.Errorf("After increment Restarts = %d, want 1", info.Restarts)
	}
	rs.IncrementRestartCount("comp")
	info, _ = rs.ComponentStatus("comp")
	if info.Restarts != 2 {
		t.Errorf("After second increment Restarts = %d, want 2", info.Restarts)
	}
}

func TestSetErrorNil(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	rs.SetError(nil)
	if rs.Health != StatusUnknown {
		t.Errorf("Health should remain unchanged, got %v", rs.Health)
	}
}

func TestSetErrorNonNil(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	_ = rs.TransitionTo(StateInitializing, "init")
	rs.SetError(errors.New("plugin not found"))
	if rs.Health != StatusUnhealthy {
		t.Errorf("Health = %v, want %v", rs.Health, StatusUnhealthy)
	}
	if rs.ErrorMessage == "" {
		t.Error("ErrorMessage should not be empty")
	}
}

func TestSetRecovery(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	_ = rs.TransitionTo(StateInitializing, "init")
	_ = rs.TransitionTo(StateReady, "ready")
	_ = rs.TransitionTo(StateRunning, "run")
	_ = rs.TransitionTo(StateError, "err")
	rs.SetRecovery()
	if rs.CurrentState != StateRecovering {
		t.Errorf("CurrentState = %v, want %v", rs.CurrentState, StateRecovering)
	}
	if rs.Health != StatusDegraded {
		t.Errorf("Health = %v, want %v", rs.Health, StatusDegraded)
	}
	if rs.RecoveryCount != 1 {
		t.Errorf("RecoveryCount = %d, want 1", rs.RecoveryCount)
	}
}

func TestRecomputeHealthUnhealthy(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	rs.SetComponentStatus("a", StatusHealthy, "")
	rs.SetComponentStatus("b", StatusUnhealthy, "fail")
	if rs.Health != StatusUnhealthy {
		t.Errorf("Health = %v, want %v", rs.Health, StatusUnhealthy)
	}
}

func TestRecomputeHealthDegraded(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	rs.SetComponentStatus("a", StatusHealthy, "")
	rs.SetComponentStatus("b", StatusDegraded, "slow")
	if rs.Health != StatusDegraded {
		t.Errorf("Health = %v, want %v", rs.Health, StatusDegraded)
	}
}

func TestRecomputeHealthAllStopped(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	rs.SetComponentStatus("a", StatusStoppedComponent, "")
	if rs.Health != StatusUnknown {
		t.Errorf("Health = %v, want %v", rs.Health, StatusUnknown)
	}
}

func TestRecomputeHealthAllHealthy(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	rs.SetComponentStatus("a", StatusHealthy, "")
	rs.SetComponentStatus("b", StatusHealthy, "")
	if rs.Health != StatusHealthy {
		t.Errorf("Health = %v, want %v", rs.Health, StatusHealthy)
	}
}

func TestSummary(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	summary := rs.Summary()
	if summary == "" {
		t.Error("Summary should not be empty")
	}
}

func TestStateChangeEvent(t *testing.T) {
	t.Parallel()
	event := StateChangeEvent{
		From:   StateUninitialized,
		To:     StateRunning,
		Reason: "test",
	}
	if event.From != StateUninitialized {
		t.Errorf("From = %v", event.From)
	}
	if event.To != StateRunning {
		t.Errorf("To = %v", event.To)
	}
}

func TestComponentInfoDefaults(t *testing.T) {
	t.Parallel()
	info := ComponentInfo{Name: "test"}
	if info.Status != "" {
		t.Errorf("Default Status should be empty, got %q", info.Status)
	}
	if info.Restarts != 0 {
		t.Errorf("Default Restarts = %d, want 0", info.Restarts)
	}
}

func TestOnStateChange(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	called := false
	rs.OnStateChange(func(e StateChangeEvent) {
		called = true
		if e.From != StateUninitialized {
			t.Errorf("From = %v", e.From)
		}
		if e.To != StateInitializing {
			t.Errorf("To = %v", e.To)
		}
	})
	_ = rs.TransitionTo(StateInitializing, "test")
	if !called {
		t.Error("OnStateChange callback was not called")
	}
}

func TestCurrentState(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	if c := rs.Current(); c != StateUninitialized {
		t.Errorf("Current = %v", c)
	}
}

func TestUptimeInGet(t *testing.T) {
	t.Parallel()
	rs := NewRuntimeState()
	_ = rs.TransitionTo(StateInitializing, "init")
	_ = rs.TransitionTo(StateReady, "ready")
	snapshot := rs.Get()
	if snapshot.Uptime < 0 {
		t.Error("Uptime should be >= 0")
	}
}
