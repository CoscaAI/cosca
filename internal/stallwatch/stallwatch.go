// Package stallwatch implements a stall watchdog for external calls (LLM
// providers, subprocesses, network operations). Its purpose is to guarantee
// the family rule: "no external call may wait indefinitely."
//
// A Watchdog wraps an operation: if the operation exceeds its deadline
// without responding (a stall), the watchdog records a structured event,
// retries with exponential backoff, optionally falls back to an alternative
// operation, and reports metrics so a stalled run can be compared with a
// healthy one (timeouts, retries, recoveries, total wait).
package stallwatch

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"runtime/debug"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Action classifies what the watchdog did in response to a slow operation.
type Action string

const (
	// ActionStall records that the operation exceeded its deadline (the
	// provider stopped responding — the "loading forever" case).
	ActionStall Action = "stall"
	// ActionRetry records a retry scheduled after a failure.
	ActionRetry Action = "retry"
	// ActionFallback records a switch to the fallback operation.
	ActionFallback Action = "fallback"
	// ActionRecovered records a successful completion after one or more stalls.
	ActionRecovered Action = "recovered"
	// ActionFailed records a final failure after retries/fallback were exhausted.
	ActionFailed Action = "failed"
)

// Event is a single structured record of a stall, retry, or recovery.
type Event struct {
	Timestamp time.Time     `json:"timestamp"`
	Operation string        `json:"operation"`
	Provider  string        `json:"provider,omitempty"`
	Model     string        `json:"model,omitempty"`
	Waited    time.Duration `json:"waited_ms"`
	Attempt   int           `json:"attempt"`
	Action    Action        `json:"action"`
	Error     string        `json:"error,omitempty"`
}

// Collector accumulates stall events for a run. It is safe for concurrent use.
type Collector struct {
	mu     sync.Mutex
	events []Event
}

// NewCollector creates an empty collector.
func NewCollector() *Collector {
	return &Collector{}
}

// Add appends an event.
func (c *Collector) Add(e Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
}

// Events returns a copy of all recorded events.
func (c *Collector) Events() []Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Event, len(c.events))
	copy(out, c.events)
	return out
}

// Clear resets the collector (e.g. between runs).
func (c *Collector) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = nil
}

// Report summarizes the collected events — the "before vs after" metrics for
// comparing a stalled run against a healthy one.
type Report struct {
	TotalStalls  int
	TotalRetries int
	Recovered    int
	Failed       int
	TotalWait    time.Duration
	ByOperation  map[string]int
}

// Report computes the summary from the collected events.
func (c *Collector) Report() Report {
	r := Report{ByOperation: make(map[string]int)}
	for _, e := range c.Events() {
		r.TotalWait += e.Waited
		switch e.Action {
		case ActionStall:
			r.TotalStalls++
			r.ByOperation[e.Operation]++
		case ActionRetry:
			r.TotalRetries++
		case ActionRecovered:
			r.Recovered++
		case ActionFailed:
			r.Failed++
		}
	}
	return r
}

// WatchSpec configures a single watchdog run.
type WatchSpec struct {
	// Name identifies the operation (e.g. "llm.chat" or "task:build").
	Name string
	// Provider is the external system being called (e.g. "openai").
	Provider string
	// Model is the model/endpoint (e.g. "gpt-4o").
	Model string
	// Timeout is the per-attempt deadline. A deadline exceeded is a stall.
	Timeout time.Duration
	// MaxRetries is how many additional attempts happen after the first.
	MaxRetries int
	// RetryDelay is the base backoff; each retry delays with bounded jitter.
	RetryDelay time.Duration
	// MaxBackoff caps the per-retry delay (default 30s).
	MaxBackoff time.Duration
	// Retryable reports whether an error (other than a stall) is worth
	// retrying. When it returns false for a non-stall error, the operation
	// fails immediately instead of burning retries. Nil means "always retry".
	Retryable func(err error) bool
	// Fallback is an alternative operation tried after retries are exhausted.
	Fallback func(ctx context.Context) error
}

// ErrAttemptTimedOut is returned by runAttempt (and used by integrations such
// as the executor) when the deadline fired before fn returned — the
// non-cooperative stall case, where fn ignores the context.
var ErrAttemptTimedOut = errors.New("stallwatch: attempt exceeded deadline (operation did not respect context cancellation)")

// runAttempt executes fn in a goroutine and selects on the deadline, so the
// watchdog reclaims control even when the operation IGNORES context
// cancellation (the real "loading forever" provider). The done channel is
// buffered so a late-returning goroutine never blocks on send; its lifetime
// is bounded by the operation itself.
func (w *Watchdog) runAttempt(ctx context.Context, spec WatchSpec, fn func(ctx context.Context) error) (error, time.Duration) {
	attemptCtx, cancel := context.WithTimeout(ctx, spec.Timeout)
	defer cancel()

	done := make(chan error, 1)
	start := w.now()
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error().
					Interface("panic", rec).
					Str("stack", string(debug.Stack())).
					Str("operation", spec.Name).
					Str("provider", spec.Provider).
					Msg("stallwatch operation panicked; watchdog continues watching")
				done <- fmt.Errorf("stallwatch: operation %q panicked: %v", spec.Name, rec)
			}
		}()
		done <- fn(attemptCtx)
	}()

	select {
	case err := <-done:
		return err, w.now().Sub(start)
	case <-attemptCtx.Done():
		// Deadline fired while fn was still running: the provider stopped
		// responding and does not respect cancellation. Take control now.
		return ErrAttemptTimedOut, w.now().Sub(start)
	case <-ctx.Done():
		// Parent cancelled — not a stall.
		return ctx.Err(), w.now().Sub(start)
	}
}

// backoffWithJitter returns RetryDelay * 2^attempt, bounded by maxBackoff and
// with full jitter in [0, delay] to avoid thundering herds.
func backoffWithJitter(base, max time.Duration, attempt int) time.Duration {
	exp := base * time.Duration(1<<uint(attempt))
	if exp > max {
		exp = max
	}
	if exp <= 0 {
		return 0
	}
	// Full jitter: random in [0, exp).
	return time.Duration(rand.Int63n(int64(exp)))
}

// Watchdog detects and recovers from stalled operations.
type Watchdog struct {
	collector *Collector // may be nil (no recording)
	now       func() time.Time
}

// NewWatchdog creates a watchdog. collector may be nil to skip recording.
func NewWatchdog(collector *Collector) *Watchdog {
	return &Watchdog{collector: collector, now: time.Now}
}

// SetCollector attaches (or detaches, with nil) the event collector.
func (w *Watchdog) SetCollector(c *Collector) {
	w.collector = c
}

// Watch runs fn with a per-attempt deadline. On a stall (deadline exceeded) it
// records the event and retries with exponential backoff; after MaxRetries it
// tries the fallback. Returns nil once the operation (or fallback) succeeds.
//
// The parent context is always respected: cancellation aborts immediately and
// is never classified as a stall.
func (w *Watchdog) Watch(ctx context.Context, spec WatchSpec, fn func(ctx context.Context) error) error {
	if spec.Timeout <= 0 {
		spec.Timeout = 5 * time.Minute
	}
	if spec.MaxRetries < 0 {
		spec.MaxRetries = 0
	}
	if spec.RetryDelay <= 0 {
		spec.RetryDelay = time.Second
	}
	if spec.MaxBackoff <= 0 {
		spec.MaxBackoff = 30 * time.Second
	}

	for attempt := 0; attempt <= spec.MaxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		err, waited := w.runAttempt(ctx, spec, fn)

		if err == nil {
			if attempt > 0 {
				w.record(spec, ActionRecovered, waited, attempt, "")
			}
			return nil
		}

		// Parent cancelled while the attempt was running — not a stall.
		if ctx.Err() != nil {
			return err
		}

		isStall := errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrAttemptTimedOut)
		if isStall {
			w.record(spec, ActionStall, waited, attempt, err.Error())
		} else if spec.Retryable != nil && !spec.Retryable(err) {
			// Non-retryable permanent failure (bad config, auth, 404): fail
			// immediately instead of burning retries.
			w.record(spec, ActionFailed, waited, attempt, err.Error())
			return err
		}

		if attempt == spec.MaxRetries {
			break
		}

		// Backoff before the next attempt (even non-stall failures wait, so
		// the provider gets time to recover). Bounded with jitter: no
		// exponential explosion, no thundering herd.
		w.record(spec, ActionRetry, waited, attempt, "")
		delay := backoffWithJitter(spec.RetryDelay, spec.MaxBackoff, attempt)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	// Retries exhausted — try the fallback if configured.
	if spec.Fallback != nil {
		w.record(spec, ActionFallback, 0, spec.MaxRetries+1, "")
		if ferr := spec.Fallback(ctx); ferr == nil {
			w.record(spec, ActionRecovered, 0, spec.MaxRetries+1, "")
			return nil
		}
	}

	w.record(spec, ActionFailed, 0, spec.MaxRetries+1, "operation stalled past retries")
	return fmt.Errorf("stallwatch: operation %q did not complete after %d attempt(s)",
		spec.Name, spec.MaxRetries+1)
}

func (w *Watchdog) record(spec WatchSpec, action Action, waited time.Duration, attempt int, errMsg string) {
	if w.collector == nil {
		return
	}
	w.collector.Add(Event{
		Timestamp: w.now().UTC(),
		Operation: spec.Name,
		Provider:  spec.Provider,
		Model:     spec.Model,
		Waited:    waited,
		Attempt:   attempt,
		Action:    action,
		Error:     errMsg,
	})
}
