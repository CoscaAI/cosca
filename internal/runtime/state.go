//
// Package runtime provides the central runtime engine for the Cosca platform.
// It manages the application lifecycle, subsystem coordination, health reporting,
// and event propagation.

package runtime

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// RuntimeState Enum
// =============================================================================

// State represents the runtime lifecycle state.
type State int

const (
	// StateUninitialized is the initial state before any initialization.
	StateUninitialized State = iota
	// StateInitializing indicates the runtime is starting up.
	StateInitializing
	// StateReady indicates all subsystems are initialized and ready.
	StateReady
	// StateRunning indicates the runtime is actively running.
	StateRunning
	// StateStopping indicates the runtime is shutting down.
	StateStopping
	// StateStopped indicates the runtime has shut down.
	StateStopped
	// StateError indicates the runtime encountered a non-recoverable error.
	StateError
	// StateRecovering indicates the runtime is attempting to recover from an error.
	StateRecovering
)

// String returns the human-readable name of the state.
func (s State) String() string {
	switch s {
	case StateUninitialized:
		return "uninitialized"
	case StateInitializing:
		return "initializing"
	case StateReady:
		return "ready"
	case StateRunning:
		return "running"
	case StateStopping:
		return "stopping"
	case StateStopped:
		return "stopped"
	case StateError:
		return "error"
	case StateRecovering:
		return "recovering"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// MarshalText implements the encoding.TextMarshaler interface.
func (s State) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (s *State) UnmarshalText(text []byte) error {
	switch string(text) {
	case "uninitialized":
		*s = StateUninitialized
	case "initializing":
		*s = StateInitializing
	case "ready":
		*s = StateReady
	case "running":
		*s = StateRunning
	case "stopping":
		*s = StateStopping
	case "stopped":
		*s = StateStopped
	case "error":
		*s = StateError
	case "recovering":
		*s = StateRecovering
	default:
		return fmt.Errorf("unknown runtime state: %s", string(text))
	}
	return nil
}

// =============================================================================
// State Transitions
// =============================================================================

// validTransitions defines the allowed state transitions.
var validTransitions = map[State][]State{
	StateUninitialized: {StateInitializing, StateError},
	StateInitializing:  {StateReady, StateError, StateStopping},
	StateReady:         {StateRunning, StateStopping, StateError},
	StateRunning:       {StateStopping, StateError, StateRecovering, StateReady},
	StateStopping:      {StateStopped, StateError},
	StateStopped:       {StateUninitialized}, // Allow restart via Reset/Initialize
	StateError:         {StateRecovering, StateStopping, StateUninitialized},
	StateRecovering:    {StateReady, StateError, StateStopping},
}

// canTransitionTo checks if a transition from `from` to `to` is valid.
func canTransitionTo(from, to State) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// =============================================================================
// ComponentStatus
// =============================================================================

// ComponentStatus represents the health status of a runtime component.
type ComponentStatus string

const (
	// StatusUnknown means the component status has not been determined.
	StatusUnknown ComponentStatus = "unknown"
	// StatusHealthy means the component is operating normally.
	StatusHealthy ComponentStatus = "healthy"
	// StatusDegraded means the component is operating with reduced capabilities.
	StatusDegraded ComponentStatus = "degraded"
	// StatusUnhealthy means the component is not functioning.
	StatusUnhealthy ComponentStatus = "unhealthy"
	// StatusStarting means the component is initializing.
	StatusStarting ComponentStatus = "starting"
	// StatusStopping means the component is shutting down.
	StatusStopping ComponentStatus = "stopping"
	// StatusStoppedComponent means the component has shut down.
	StatusStoppedComponent ComponentStatus = "stopped"
)

// ComponentInfo holds the status and metadata of a runtime component.
type ComponentInfo struct {
	Name      string          `json:"name" yaml:"name"`
	Status    ComponentStatus `json:"status" yaml:"status"`
	Message   string          `json:"message,omitempty" yaml:"message,omitempty"`
	Error     string          `json:"error,omitempty" yaml:"error,omitempty"`
	Uptime    time.Duration   `json:"uptime" yaml:"uptime"`
	StartedAt time.Time       `json:"started_at" yaml:"started_at"`
	Restarts  int             `json:"restarts" yaml:"restarts"`
}

// =============================================================================
// RuntimeState
// =============================================================================

// RuntimeState holds the complete runtime state.
//
//nolint:revive // Stutter name preserved for API compatibility — used as runtime.RuntimeState externally.
type RuntimeState struct {
	mu            sync.RWMutex
	CurrentState  State                    `json:"current_state" yaml:"current_state"`
	PreviousState State                    `json:"previous_state" yaml:"previous_state"`
	Health        ComponentStatus          `json:"health" yaml:"health"`
	ErrorMessage  string                   `json:"error_message,omitempty" yaml:"error_message,omitempty"`
	StartedAt     time.Time                `json:"started_at" yaml:"started_at"`
	Uptime        time.Duration            `json:"uptime" yaml:"uptime"`
	Components    map[string]ComponentInfo `json:"components" yaml:"components"`
	ConfigVersion string                   `json:"config_version,omitempty" yaml:"config_version,omitempty"`
	RecoveryCount int                      `json:"recovery_count" yaml:"recovery_count"`
	onChange      func(StateChangeEvent)
}

// StateChangeEvent is emitted when the runtime state changes.
type StateChangeEvent struct {
	From      State     `json:"from" yaml:"from"`
	To        State     `json:"to" yaml:"to"`
	Timestamp time.Time `json:"timestamp" yaml:"timestamp"`
	Reason    string    `json:"reason,omitempty" yaml:"reason,omitempty"`
	Error     string    `json:"error,omitempty" yaml:"error,omitempty"`
}

// NewRuntimeState creates a new runtime state tracker.
func NewRuntimeState() *RuntimeState {
	return &RuntimeState{
		CurrentState:  StateUninitialized,
		PreviousState: StateUninitialized,
		Health:        StatusUnknown,
		Components:    make(map[string]ComponentInfo),
	}
}

// Get returns a snapshot of the current runtime state.
func (rs *RuntimeState) Get() RuntimeState {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	// Compute uptime
	var uptime time.Duration
	if !rs.StartedAt.IsZero() {
		uptime = time.Since(rs.StartedAt)
	}

	// Create a copy of components
	components := make(map[string]ComponentInfo, len(rs.Components))
	for k, v := range rs.Components {
		components[k] = v
	}

	return RuntimeState{
		CurrentState:  rs.CurrentState,
		PreviousState: rs.PreviousState,
		Health:        rs.Health,
		ErrorMessage:  rs.ErrorMessage,
		StartedAt:     rs.StartedAt,
		Uptime:        uptime,
		Components:    components,
		ConfigVersion: rs.ConfigVersion,
		RecoveryCount: rs.RecoveryCount,
	}
}

// Current returns the current state value.
func (rs *RuntimeState) Current() State {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.CurrentState
}

// SetHealth sets the overall runtime health.
func (rs *RuntimeState) SetHealth(health ComponentStatus) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.Health = health
}

// HealthStatus returns the current health status.
func (rs *RuntimeState) HealthStatus() ComponentStatus {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.Health
}

// =============================================================================
// State Transitions
// =============================================================================

// TransitionTo attempts to transition the runtime to a new state.
// Returns an error if the transition is not allowed.
func (rs *RuntimeState) TransitionTo(newState State, reason string) error {
	event := StateChangeEvent{
		To:        newState,
		Timestamp: time.Now(),
		Reason:    reason,
	}

	rs.mu.Lock()
	defer rs.mu.Unlock()

	event.From = rs.CurrentState

	if !canTransitionTo(rs.CurrentState, newState) {
		event.Error = fmt.Sprintf(
			"invalid transition: %s -> %s",
			rs.CurrentState, newState,
		)
		// Fire error event regardless
		if rs.onChange != nil {
			rs.onChange(event)
		}
		return fmt.Errorf("invalid state transition: %s -> %s", rs.CurrentState, newState)
	}

	rs.PreviousState = rs.CurrentState
	rs.CurrentState = newState

	if newState == StateRunning || newState == StateReady {
		if rs.StartedAt.IsZero() {
			rs.StartedAt = time.Now()
		}
	}

	if newState == StateError {
		rs.ErrorMessage = reason
		rs.Health = StatusUnhealthy
	}

	if newState == StateStopped {
		rs.Health = StatusUnknown
	}

	event.Timestamp = time.Now()

	if rs.onChange != nil {
		rs.onChange(event)
	}

	return nil
}

// OnStateChange registers a callback for state change events.
func (rs *RuntimeState) OnStateChange(fn func(StateChangeEvent)) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.onChange = fn
}

// =============================================================================
// Component Management
// =============================================================================

// SetComponentStatus updates the status of a runtime component.
func (rs *RuntimeState) SetComponentStatus(name string, status ComponentStatus, message string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	existing, ok := rs.Components[name]
	if !ok {
		existing = ComponentInfo{
			Name:      name,
			StartedAt: time.Now(),
		}
	}

	oldStatus := existing.Status
	existing.Status = status
	existing.Message = message
	if status == StatusUnhealthy && message != "" {
		existing.Error = message
	}
	if status == StatusStarting {
		existing.StartedAt = time.Now()
	}
	if status == StatusHealthy && oldStatus == StatusUnhealthy {
		existing.Error = ""
	}

	existing.Uptime = time.Since(existing.StartedAt)
	rs.Components[name] = existing

	// Update overall health based on component statuses
	rs.recomputeHealth()
}

// ComponentStatus returns the status of a specific component.
func (rs *RuntimeState) ComponentStatus(name string) (ComponentInfo, bool) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	info, ok := rs.Components[name]
	return info, ok
}

// AllComponentStatuses returns the status of all components.
func (rs *RuntimeState) AllComponentStatuses() map[string]ComponentInfo {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	result := make(map[string]ComponentInfo, len(rs.Components))
	for k, v := range rs.Components {
		result[k] = v
	}
	return result
}

// IncrementRestartCount increments the restart counter for a component.
func (rs *RuntimeState) IncrementRestartCount(name string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if existing, ok := rs.Components[name]; ok {
		existing.Restarts++
		rs.Components[name] = existing
	}
}

// =============================================================================
// Error State Management
// =============================================================================

// SetError sets the runtime into an error state with a message.
func (rs *RuntimeState) SetError(err error) {
	if err == nil {
		return
	}
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.ErrorMessage = err.Error()
	rs.Health = StatusUnhealthy

	// Try to transition to error state if not already there
	if rs.CurrentState != StateError {
		from := rs.CurrentState
		if canTransitionTo(from, StateError) {
			rs.PreviousState = from
			rs.CurrentState = StateError
			if rs.onChange != nil {
				rs.onChange(StateChangeEvent{
					From:      from,
					To:        StateError,
					Timestamp: time.Now(),
					Reason:    err.Error(),
					Error:     err.Error(),
				})
			}
		}
	}
}

// SetRecovery increments the recovery counter and transitions to recovering.
func (rs *RuntimeState) SetRecovery() {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.RecoveryCount++
	from := rs.CurrentState
	if canTransitionTo(from, StateRecovering) {
		rs.PreviousState = from
		rs.CurrentState = StateRecovering
		rs.Health = StatusDegraded
		if rs.onChange != nil {
			rs.onChange(StateChangeEvent{
				From:      from,
				To:        StateRecovering,
				Timestamp: time.Now(),
				Reason:    "automatic recovery initiated",
			})
		}
	}
}

// =============================================================================
// Internal Helpers
// =============================================================================

// recomputeHealth updates the overall health based on component statuses.
func (rs *RuntimeState) recomputeHealth() {
	hasUnhealthy := false
	hasDegraded := false
	allStopped := true

	for _, s := range rs.Components {
		switch s.Status {
		case StatusUnhealthy:
			hasUnhealthy = true
			allStopped = false
		case StatusDegraded:
			hasDegraded = true
			allStopped = false
		case StatusHealthy, StatusStarting:
			allStopped = false
		default:
			// StatusUnknown, StatusStopping, StatusStoppedComponent, etc.
			// Don't change allStopped — these are not "running" components.
		}
	}

	switch {
	case hasUnhealthy:
		rs.Health = StatusUnhealthy
	case hasDegraded:
		rs.Health = StatusDegraded
	case allStopped:
		rs.Health = StatusUnknown
	default:
		rs.Health = StatusHealthy
	}
}

// =============================================================================
// Summary
// =============================================================================

// Summary returns a concise, human-readable summary of the runtime state.
func (rs *RuntimeState) Summary() string {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	var b strings.Builder
	fmt.Fprintf(&b, "State: %s\n", rs.CurrentState)
	fmt.Fprintf(&b, "Health: %s\n", rs.Health)
	if !rs.StartedAt.IsZero() {
		fmt.Fprintf(&b, "Uptime: %s\n", time.Since(rs.StartedAt).Round(time.Second))
	}
	if rs.ErrorMessage != "" {
		fmt.Fprintf(&b, "Error: %s\n", rs.ErrorMessage)
	}
	fmt.Fprintf(&b, "Components: %d\n", len(rs.Components))

	// List component statuses
	for name, info := range rs.Components {
		fmt.Fprintf(&b, "  - %s: %s", name, info.Status)
		if info.Restarts > 0 {
			fmt.Fprintf(&b, " (restarts: %d)", info.Restarts)
		}
		b.WriteString("\n")
	}

	return b.String()
}
