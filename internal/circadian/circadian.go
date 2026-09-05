// Package circadian implements the Cosca Operational Rest Cycle (ORC): the
// activity/rest state machine that governs when the engine may run expensive
// work such as agent calls, benchmarks, audits, and deep maintenance.
//
// The ORC is a maintenance optimization model, not a biological metaphor. A
// quiet system progressively winds down expensive operations until it opens a
// maintenance window (the ORC window) where only deep, unattended housekeeping
// runs. When the Don (the operator) interacts again, the system wakes up and
// returns to full availability.
//
// The engine is deliberately single-threaded: state changes are driven by an
// external scheduler (added later). This package only provides the state, the
// activity gates, and the idle-based evaluation that the scheduler will use.
package circadian

import (
	"fmt"
	"sync"
	"time"
)

// CircadianState is the operational state of the engine.
type CircadianState string

// The six operational states of the ORC cycle.
const (
	// StateAwake means the engine is fully available. All activities are
	// permitted.
	StateAwake CircadianState = "awake"

	// StateFocused means the engine is executing priority work (P0/P1).
	// Agent calls, benchmarks, audits, and heavy indexing are allowed;
	// maintenance is deferred until the focus window ends.
	StateFocused CircadianState = "focused"

	// StateIdle means the engine is quiet but responsive: it monitors the
	// Don and answers quickly. Only light agent calls and maintenance run.
	StateIdle CircadianState = "idle"

	// StateResting means the engine has been idle for a while. Expensive
	// agents, benchmarks, audits, and heavy indexing are suspended; only
	// light maintenance is permitted.
	StateResting CircadianState = "resting"

	// StateSleeping means the engine entered its ORC window: deep, unattended
	// maintenance only. Normal operations resume only via a wake transition.
	StateSleeping CircadianState = "sleeping"

	// StateMaintenance is a forced maintenance state. It overrides the cycle:
	// light maintenance and the ORC run regardless of the idle clock.
	StateMaintenance CircadianState = "maintenance"
)

// Transition records a validated state change.
type Transition struct {
	From   CircadianState
	To     CircadianState
	At     time.Time
	Reason string
}

// Engine holds the ORC state machine. It is safe for concurrent use.
// A single Engine is intended to drive one workload (for example one
// workspace), so callers should create one instance per workload.
type Engine struct {
	mu           sync.RWMutex
	current      CircadianState
	lastActivity time.Time
	transitions  []Transition
	config       Config
}

// New returns an Engine in the awake state using the default thresholds.
func New() *Engine {
	return NewWithConfig(DefaultConfig())
}

// NewWithConfig returns an Engine in the awake state using the given thresholds.
func NewWithConfig(cfg Config) *Engine {
	return &Engine{
		current:      StateAwake,
		lastActivity: time.Now(),
		config:       cfg,
	}
}

// State returns the current operational state.
func (e *Engine) State() CircadianState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.current
}

// TransitionTo validates and applies a transition to the given state.
// The transition must be allowed by the transition matrix (see canTransition)
// and must differ from the current state. On success the transition is
// appended to the history and the current state is updated.
func (e *Engine) TransitionTo(to CircadianState, reason string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	from := e.current
	if from == to {
		return fmt.Errorf("circadian: already in state %q", to)
	}
	if !e.canTransition(to) {
		return fmt.Errorf("circadian: invalid transition %q -> %q", from, to)
	}

	e.transitions = append(e.transitions, Transition{
		From:   from,
		To:     to,
		At:     time.Now(),
		Reason: reason,
	})
	e.current = to
	return nil
}

// canTransition reports whether a transition from the current state to to is
// allowed by the transition matrix.
func (e *Engine) canTransition(to CircadianState) bool {
	return validTransitions[e.current][to]
}

// Transitions returns a copy of the recorded transition history.
func (e *Engine) Transitions() []Transition {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]Transition, len(e.transitions))
	copy(out, e.transitions)
	return out
}

// RecordActivity marks that the Don interacted with the engine, resetting the
// idle clock. It does not change the state: waking the engine (transitions
// back to awake) is the scheduler's job.
func (e *Engine) RecordActivity() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lastActivity = time.Now()
}

// IdleDuration reports how long the engine has been without activity.
func (e *Engine) IdleDuration() time.Duration {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return time.Since(e.lastActivity)
}

// CanRun reports whether the given activity is permitted in the current state.
// It is the gate the scheduler must consult before starting any activity.
func (e *Engine) CanRun(activity Activity) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.allowed(activity, e.current)
}

// Evaluate computes the state the engine should move to based on the current
// idle duration and the configured thresholds:
//
//	idle <  AwakeTimeout      -> awake
//	idle <  IdleToResting     -> idle
//	idle <  RestingToSleeping -> resting
//	otherwise                 -> sleeping
//
// Boundary semantics: the moment the idle clock reaches a threshold, the next
// state takes over (an engine idle for exactly IdleToResting is proposed as
// resting). Evaluate only proposes: it never mutates the engine. The scheduler
// walks the transition matrix (for example awake -> idle -> resting -> sleeping)
// using TransitionTo. While in the forced maintenance state the proposal should
// be ignored.
func (e *Engine) Evaluate() CircadianState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return evaluate(e.config, time.Since(e.lastActivity))
}

// evaluate is the pure, testable form of Evaluate.
func evaluate(cfg Config, idle time.Duration) CircadianState {
	switch {
	case idle < cfg.AwakeTimeout:
		return StateAwake
	case idle < cfg.IdleToResting:
		return StateIdle
	case idle < cfg.RestingToSleeping:
		return StateResting
	default:
		return StateSleeping
	}
}

// validTransitions is the transition matrix of the ORC cycle. A transition is
// valid only if listed; anything else is rejected (deny by default).
//
//	             awake focused idle resting sleeping maintenance
//	awake          .      x      x     .       .         x
//	focused        x      .      x     .       .         x
//	idle           x      x      .     x       .         x
//	resting        x      .      x     .       x         x
//	sleeping       x      .      .     .       .         x
//	maintenance    x      .      .     x       .         .
var validTransitions = map[CircadianState]map[CircadianState]bool{
	StateAwake: {
		StateFocused:     true,
		StateIdle:        true,
		StateMaintenance: true,
	},
	StateFocused: {
		StateAwake:       true,
		StateIdle:        true,
		StateMaintenance: true,
	},
	StateIdle: {
		StateAwake:       true,
		StateFocused:     true,
		StateResting:     true,
		StateMaintenance: true,
	},
	StateResting: {
		StateAwake:       true,
		StateIdle:        true,
		StateSleeping:    true,
		StateMaintenance: true,
	},
	StateSleeping: {
		StateAwake:       true,
		StateMaintenance: true,
	},
	StateMaintenance: {
		StateAwake:   true,
		StateResting: true,
	},
}
