// Package circuitbreaker provides a generic circuit breaker implementation
// for external service calls, with automatic failover support.
//
// The circuit breaker has three states:
//   - Closed: normal operation, requests pass through
//   - Open: failures exceeded threshold, requests fail fast
//   - HalfOpen: probing if service recovered, limited requests allowed
//
// State transitions:
//
//	Closed → Open: when failure count reaches Threshold in a window
//	Open → HalfOpen: after Timeout duration
//	HalfOpen → Closed: when SuccessThreshold consecutive calls succeed
//	HalfOpen → Open: when any call fails in HalfOpen state
package circuitbreaker

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// State represents the circuit breaker state.
type State int

const (
	StateClosed   State = iota // Normal operation
	StateOpen                  // Failing fast
	StateHalfOpen              // Probing recovery
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Config configures a CircuitBreaker.
type Config struct {
	// FailureThreshold is the number of consecutive failures before opening.
	// Default: 5
	FailureThreshold int

	// SuccessThreshold is the number of consecutive successes in HalfOpen
	// state before closing. Default: 2
	SuccessThreshold int

	// Timeout is how long to wait before transitioning from Open to HalfOpen.
	// Default: 30 seconds
	Timeout time.Duration

	// OnStateChange is called when the circuit breaker state changes.
	OnStateChange func(from, to State)
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
	}
}

// CircuitBreaker implements the circuit breaker pattern.
// It is safe for concurrent use.
type CircuitBreaker struct {
	mu     sync.Mutex
	state  State
	config Config

	failures        int
	successes       int
	lastStateChange time.Time
	openSince       time.Time

	// Stats
	totalCalls     int64
	totalSuccesses int64
	totalFailures  int64
	totalTimeouts  int64
	totalRejects   int64
}

// New creates a new CircuitBreaker with the given config.
func New(config Config) *CircuitBreaker {
	if config.FailureThreshold <= 0 {
		config.FailureThreshold = 5
	}
	if config.SuccessThreshold <= 0 {
		config.SuccessThreshold = 2
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	return &CircuitBreaker{
		state:  StateClosed,
		config: config,
	}
}

// Execute runs the given function through the circuit breaker.
// If the circuit is open, it returns ErrCircuitOpen without calling fn.
// If the function succeeds, the success counter is incremented.
// If the function returns an error, the failure counter is incremented.
func (cb *CircuitBreaker) Execute(fn func() (interface{}, error)) (interface{}, error) {
	atomic.AddInt64(&cb.totalCalls, 1)

	if !cb.allowRequest() {
		atomic.AddInt64(&cb.totalRejects, 1)
		return nil, ErrCircuitOpen
	}

	result, err := fn()
	if err != nil {
		atomic.AddInt64(&cb.totalFailures, 1)
		cb.recordFailure()
		return result, err
	}

	atomic.AddInt64(&cb.totalSuccesses, 1)
	cb.recordSuccess()
	return result, nil
}

// State returns the current state of the circuit breaker.
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.evaluateTimeout()
	return cb.state
}

// Stats returns cumulative statistics.
func (cb *CircuitBreaker) Stats() Stats {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.evaluateTimeout()

	return Stats{
		State:                cb.state,
		Calls:                atomic.LoadInt64(&cb.totalCalls),
		Successes:            atomic.LoadInt64(&cb.totalSuccesses),
		Failures:             atomic.LoadInt64(&cb.totalFailures),
		Timeouts:             atomic.LoadInt64(&cb.totalTimeouts),
		Rejects:              atomic.LoadInt64(&cb.totalRejects),
		FailureCount:         cb.failures,
		ConsecutiveSuccesses: cb.successes,
	}
}

// Reset resets the circuit breaker to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	oldState := cb.state
	cb.state = StateClosed
	cb.failures = 0
	cb.successes = 0
	cb.lastStateChange = time.Now()
	if oldState != StateClosed && cb.config.OnStateChange != nil {
		cb.config.OnStateChange(oldState, StateClosed)
	}
}

// ── Internal ────────────────────────────────────────────────────────────────

// allowRequest checks whether a request should be allowed through.
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if cb.evaluateTimeout() {
			return true
		}
		return false
	case StateHalfOpen:
		// In HalfOpen, allow exactly SuccessThreshold requests.
		return cb.successes < cb.config.SuccessThreshold
	default:
		return false
	}
}

// evaluateTimeout checks if the timeout has elapsed for an Open circuit.
// If so, transitions to HalfOpen.
func (cb *CircuitBreaker) evaluateTimeout() bool {
	if cb.state != StateOpen {
		return false
	}
	if time.Since(cb.openSince) >= cb.config.Timeout {
		cb.setState(StateHalfOpen)
		return true
	}
	return false
}

// recordFailure handles a failed call.
func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		cb.failures++
		cb.successes = 0
		if cb.failures >= cb.config.FailureThreshold {
			cb.setState(StateOpen)
		}
	case StateHalfOpen:
		// Any failure in HalfOpen immediately goes back to Open.
		cb.setState(StateOpen)
	default:
		// Open: ignore (request was rejected at allowRequest level)
	}
}

// recordSuccess handles a successful call.
func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		cb.failures = 0
	case StateHalfOpen:
		cb.successes++
		if cb.successes >= cb.config.SuccessThreshold {
			cb.setState(StateClosed)
		}
	default:
		// Open: ignore (request was rejected at allowRequest level)
	}
}

// setState transitions to a new state, resetting counters.
func (cb *CircuitBreaker) setState(newState State) {
	oldState := cb.state
	cb.state = newState
	cb.failures = 0
	cb.successes = 0
	cb.lastStateChange = time.Now()
	if newState == StateOpen {
		cb.openSince = time.Now()
	}
	if oldState != newState && cb.config.OnStateChange != nil {
		cb.config.OnStateChange(oldState, newState)
	}
}

// Stats holds cumulative circuit breaker statistics.
type Stats struct {
	State                State `json:"state"`
	Calls                int64 `json:"calls"`
	Successes            int64 `json:"successes"`
	Failures             int64 `json:"failures"`
	Timeouts             int64 `json:"timeouts"`
	Rejects              int64 `json:"rejects"`
	FailureCount         int   `json:"failureCount"`
	ConsecutiveSuccesses int   `json:"consecutiveSuccesses"`
}

// ErrCircuitOpen is returned when the circuit breaker is open.
var ErrCircuitOpen = fmt.Errorf("circuit breaker: open (request rejected)")
