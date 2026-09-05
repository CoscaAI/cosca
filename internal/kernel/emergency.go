package kernel

import (
	"errors"
	"strings"
	"sync"
	"time"
)

// EmergencyState represents the kill-switch state of the Kernel.
//
// The kill switch is the emergency mechanism that lets the Don stop the
// daemon and block execution remotely, BEFORE the Kernel "wakes up" in
// serve. It is wired into the REST API behind admin-only auth and into
// the serve command's shutdown path.
type EmergencyState string

const (
	// EmergencyNone is the normal operating state. The daemon is free to run.
	EmergencyNone EmergencyState = "none"
	// EmergencyStop means an emergency stop has been ordered — the daemon
	// must shut down gracefully as soon as possible.
	EmergencyStop EmergencyState = "stop"
	// EmergencyHalted is the stronger variant — everything stops, no
	// graceful drain, no question asked.
	EmergencyHalted EmergencyState = "halted"
)

// ErrEmergencyAlreadyTriggered is returned by TriggerStop when the kill
// switch has already been pulled and the new trigger does not escalate it
// (e.g. a second "stop" while already in EmergencyStop). Callers may treat
// it as an idempotent acknowledgement that the emergency is already active.
var ErrEmergencyAlreadyTriggered = errors.New("emergency already triggered")

// EmergencyManager tracks the Kernel's kill-switch state and invokes a
// shutdown function when the switch is pulled.
//
// The manager is goroutine-safe: the HTTP handler goroutine calls
// TriggerStop while the serve event loop concurrently observes the state.
type EmergencyManager struct {
	mu         sync.RWMutex
	state      EmergencyState
	haltedAt   time.Time // UTC time the emergency was triggered
	reason     string    // human-readable reason for the trigger
	shutdownFn func(reason string)
}

// NewEmergencyManager creates an EmergencyManager in the EmergencyNone state.
func NewEmergencyManager() *EmergencyManager {
	return &EmergencyManager{state: EmergencyNone}
}

// TriggerStop pulls the kill switch. It transitions the manager into an
// emergency state, records when it happened, and invokes the registered
// shutdown function (if any) with the given reason.
//
// The target state depends on the reason: the reserved reason "halted"
// (case-insensitive) produces the stronger EmergencyHalted state — used by
// POST /v1/kernel/emergency/halt — while any other reason produces
// EmergencyStop. A halt also escalates an already-stopped manager, since
// "halt everything" is strictly stronger than "stop".
//
// The shutdown function is invoked synchronously but AFTER the manager's
// lock is released, so it may safely signal channels without deadlocking.
// It is invoked at most once per distinct transition; duplicate triggers
// of the same state return ErrEmergencyAlreadyTriggered.
func (m *EmergencyManager) TriggerStop(reason string) error {
	target := EmergencyStop
	if strings.EqualFold(strings.TrimSpace(reason), "halted") {
		target = EmergencyHalted
	}

	m.mu.Lock()
	switch m.state {
	case EmergencyNone:
		m.state = target
	case EmergencyStop:
		if target != EmergencyHalted {
			m.mu.Unlock()
			return ErrEmergencyAlreadyTriggered
		}
		m.state = EmergencyHalted // escalate stop → halt
	default: // EmergencyHalted
		m.mu.Unlock()
		return ErrEmergencyAlreadyTriggered
	}
	m.haltedAt = time.Now().UTC()
	m.reason = reason
	fn := m.shutdownFn
	m.mu.Unlock()

	if fn != nil {
		fn(reason)
	}
	return nil
}

// State returns the current emergency state.
func (m *EmergencyManager) State() EmergencyState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// Reason returns the reason recorded with the most recent trigger.
func (m *EmergencyManager) Reason() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.reason
}

// HaltedAt returns the UTC time the emergency was triggered.
// The zero time is returned when no emergency has ever been triggered.
func (m *EmergencyManager) HaltedAt() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.haltedAt
}

// IsHalted reports whether the kill switch has been pulled, i.e. the state
// is anything other than EmergencyNone. This is the flag the Kernel must
// check before "waking up" in serve: when true, execution is blocked.
func (m *EmergencyManager) IsHalted() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state != EmergencyNone
}

// SetShutdownFn registers the function invoked by TriggerStop. A nil
// function disables the callback. Intended to be wired by the serve command
// to the daemon shutdown path before the REST server starts.
func (m *EmergencyManager) SetShutdownFn(fn func(reason string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shutdownFn = fn
}

// Clear resets the manager to EmergencyNone, clearing the reason and the
// halted timestamp. Used after a daemon restart to release the kill switch.
func (m *EmergencyManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = EmergencyNone
	m.haltedAt = time.Time{}
	m.reason = ""
}
