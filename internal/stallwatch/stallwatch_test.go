package stallwatch

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// blockingFn never returns until the context deadline fires — it simulates a
// provider that stopped responding (the "loading forever" case). It must
// actually block: returning context.DeadlineExceeded immediately makes the
// measured wait ~0 and the Waited>0 assertion flaky (and always 0 on Windows).
func blockingFn(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

// slowFn sleeps longer than the attempt timeout, then returns the error.
func slowFn(err error) func(context.Context) error {
	return func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
			return err
		}
	}
}

func TestWatchSucceedsImmediately(t *testing.T) {
	c := NewCollector()
	w := NewWatchdog(c)
	spec := WatchSpec{Name: "ok", Timeout: time.Second, MaxRetries: 1}
	if err := w.Watch(context.Background(), spec, func(ctx context.Context) error { return nil }); err != nil {
		t.Fatalf("Watch: %v", err)
	}
	// No stall, no events.
	if n := len(c.Events()); n != 0 {
		t.Fatalf("events = %d, want 0", n)
	}
	r := c.Report()
	if r.TotalStalls != 0 || r.Recovered != 0 || r.Failed != 0 {
		t.Fatalf("report: %+v", r)
	}
}

func TestWatchDetectsStallAndRetries(t *testing.T) {
	c := NewCollector()
	w := NewWatchdog(c)
	spec := WatchSpec{Name: "llm.chat", Provider: "openai", Model: "gpt-4o",
		Timeout: 10 * time.Millisecond, MaxRetries: 2, RetryDelay: time.Millisecond}

	// First two attempts stall, third succeeds.
	var attempts atomic.Int32
	err := w.Watch(context.Background(), spec, func(ctx context.Context) error {
		n := attempts.Add(1)
		if n <= 2 {
			return blockingFn(ctx)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	if attempts.Load() != 3 {
		t.Fatalf("attempts = %d, want 3", attempts.Load())
	}

	events := c.Events()
	stalls := countAction(events, ActionStall)
	retries := countAction(events, ActionRetry)
	recovered := countAction(events, ActionRecovered)
	if stalls != 2 {
		t.Fatalf("stalls = %d, want 2 (%+v)", stalls, events)
	}
	if retries != 2 {
		t.Fatalf("retries = %d, want 2", retries)
	}
	if recovered != 1 {
		t.Fatalf("recovered = %d, want 1", recovered)
	}

	r := c.Report()
	if r.TotalStalls != 2 || r.TotalRetries != 2 || r.Recovered != 1 {
		t.Fatalf("report: %+v", r)
	}
	if r.ByOperation["llm.chat"] != 2 {
		t.Fatalf("by operation: %+v", r.ByOperation)
	}
	// Stall events must carry the provider/model and a positive wait.
	if events[0].Provider != "openai" || events[0].Model != "gpt-4o" || events[0].Waited <= 0 {
		t.Fatalf("stall event: %+v", events[0])
	}
}

func TestWatchFailsAfterMaxRetries(t *testing.T) {
	c := NewCollector()
	w := NewWatchdog(c)
	spec := WatchSpec{Name: "dead", Timeout: 5 * time.Millisecond, MaxRetries: 2, RetryDelay: time.Millisecond}

	err := w.Watch(context.Background(), spec, blockingFn)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	events := c.Events()
	if countAction(events, ActionStall) != 3 { // 1 initial + 2 retries
		t.Fatalf("stalls = %d, want 3", countAction(events, ActionStall))
	}
	if countAction(events, ActionFailed) != 1 {
		t.Fatalf("failed = %d, want 1", countAction(events, ActionFailed))
	}
	if r := c.Report(); r.Failed != 1 || r.Recovered != 0 {
		t.Fatalf("report: %+v", r)
	}
}

func TestWatchFallback(t *testing.T) {
	c := NewCollector()
	w := NewWatchdog(c)
	spec := WatchSpec{
		Name: "primary", Timeout: 5 * time.Millisecond, MaxRetries: 1, RetryDelay: time.Millisecond,
		Fallback: func(ctx context.Context) error { return nil },
	}

	if err := w.Watch(context.Background(), spec, blockingFn); err != nil {
		t.Fatalf("Watch with fallback: %v", err)
	}
	events := c.Events()
	if countAction(events, ActionFallback) != 1 {
		t.Fatalf("fallback = %d, want 1", countAction(events, ActionFallback))
	}
	if countAction(events, ActionRecovered) != 1 {
		t.Fatalf("recovered after fallback = %d, want 1", countAction(events, ActionRecovered))
	}
}

func TestWatchFallbackAlsoFails(t *testing.T) {
	c := NewCollector()
	w := NewWatchdog(c)
	spec := WatchSpec{
		Name: "primary", Timeout: 5 * time.Millisecond, MaxRetries: 0, RetryDelay: time.Millisecond,
		Fallback: func(ctx context.Context) error { return errors.New("fallback also down") },
	}
	if err := w.Watch(context.Background(), spec, blockingFn); err == nil {
		t.Fatal("expected error when fallback fails")
	}
	if countAction(c.Events(), ActionFailed) != 1 {
		t.Fatal("failed event missing")
	}
}

func TestWatchRespectsParentCancel(t *testing.T) {
	c := NewCollector()
	w := NewWatchdog(c)
	spec := WatchSpec{Name: "cancel", Timeout: 5 * time.Second, MaxRetries: 3, RetryDelay: time.Hour}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	err := w.Watch(ctx, spec, func(ctx context.Context) error { return nil })
	if err == nil {
		t.Fatal("cancelled parent must abort")
	}
	// No stall events for a parent cancellation.
	if n := len(c.Events()); n != 0 {
		t.Fatalf("events = %d, want 0", n)
	}
}

func TestWatchParentCancelDuringBackoff(t *testing.T) {
	c := NewCollector()
	w := NewWatchdog(c)
	spec := WatchSpec{Name: "x", Timeout: 5 * time.Millisecond, MaxRetries: 5, RetryDelay: time.Hour}

	ctx, cancel := context.WithCancel(context.Background())
	var started atomic.Bool
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()
	_ = w.Watch(ctx, spec, func(ctx context.Context) error {
		started.Store(true)
		return blockingFn(ctx)
	})
	if !started.Load() {
		t.Fatal("fn never ran")
	}
	// Backoff is interrupted by cancellation — Watch returns before the hour.
}

func TestCollectorClear(t *testing.T) {
	c := NewCollector()
	c.Add(Event{Action: ActionStall})
	c.Clear()
	if n := len(c.Events()); n != 0 {
		t.Fatalf("events after clear = %d", n)
	}
}

func TestWatchDefaults(t *testing.T) {
	c := NewCollector()
	w := NewWatchdog(c)
	// Zero spec: defaults applied, no panic, succeeds quickly.
	if err := w.Watch(context.Background(), WatchSpec{}, func(ctx context.Context) error { return nil }); err != nil {
		t.Fatalf("Watch with empty spec: %v", err)
	}
}

func countAction(events []Event, a Action) int {
	n := 0
	for _, e := range events {
		if e.Action == a {
			n++
		}
	}
	return n
}

// panickingFn panics inside the attempt goroutine — the worst case for the
// watchdog, since a panic is neither a returned error nor a stall. It must
// be recovered so the watchdog keeps watching.
func panickingFn(panicUntilAttempt int) func(context.Context) error {
	var n int
	return func(ctx context.Context) error {
		n++
		if n <= panicUntilAttempt {
			panic("provider exploded")
		}
		return nil
	}
}

// TestWatchPanickingFnContinuesWatching verifies that a panicking operation
// does not kill the watchdog: the panic is recovered inside the attempt
// goroutine, the attempt counts as a failure, and the retry/fallback logic
// keeps running so the next (healthy) attempt succeeds.
func TestWatchPanickingFnContinuesWatching(t *testing.T) {
	c := NewCollector()
	w := NewWatchdog(c)
	spec := WatchSpec{Name: "explode", Timeout: time.Second, MaxRetries: 1, RetryDelay: time.Millisecond}

	if err := w.Watch(context.Background(), spec, panickingFn(1)); err != nil {
		t.Fatalf("Watch: %v", err)
	}
	// The panic was converted into a retry: exactly one retry then recovery.
	events := c.Events()
	if n := countAction(events, ActionRetry); n != 1 {
		t.Fatalf("retries = %d, want 1 (%+v)", n, events)
	}
	if n := countAction(events, ActionRecovered); n != 1 {
		t.Fatalf("recovered = %d, want 1 (%+v)", n, events)
	}
}

// TestWatchPanickingFnDoesNotCrash verifies that a permanently panicking
// operation still fails through the normal path (no crash, no hang): the
// watchdog records the failure and returns the panic-derived error.
func TestWatchPanickingFnDoesNotCrash(t *testing.T) {
	c := NewCollector()
	w := NewWatchdog(c)
	spec := WatchSpec{Name: "always-panic", Timeout: time.Second, MaxRetries: 1, RetryDelay: time.Millisecond}

	err := w.Watch(context.Background(), spec, panickingFn(1<<30))
	if err == nil {
		t.Fatal("expected error after permanently panicking operation")
	}
	if n := countAction(c.Events(), ActionFailed); n != 1 {
		t.Fatalf("failed = %d, want 1 (%+v)", n, c.Events())
	}
}
