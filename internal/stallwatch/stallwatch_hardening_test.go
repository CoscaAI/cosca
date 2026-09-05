package stallwatch

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// nonCooperativeFn ignores the context deadline entirely and sleeps for a
// fixed duration — the worst-case provider that never respects cancellation
// (the true "loading forever" behaviour). The watchdog must reclaim control
// at the deadline, not wait for the sleep to finish.
func nonCooperativeFn(sleep time.Duration) func(context.Context) error {
	return func(ctx context.Context) error {
		time.Sleep(sleep) // deliberately ignores ctx
		return nil
	}
}

func TestWatchReclaimsControlFromNonCooperativeOp(t *testing.T) {
	c := NewCollector()
	w := NewWatchdog(c)
	spec := WatchSpec{Name: "stubborn", Timeout: 20 * time.Millisecond, MaxRetries: 1, RetryDelay: time.Millisecond}

	// The operation sleeps 5s while the deadline is 20ms. Without the
	// goroutine+select approach the watchdog would hang for 5s; it must
	// reclaim control at ~20ms.
	start := time.Now()
	err := w.Watch(context.Background(), spec, nonCooperativeFn(5*time.Second))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error: non-cooperative op never completed within retries")
	}
	if elapsed > time.Second {
		t.Fatalf("watchdog did not reclaim control: elapsed = %s (sleep was 5s)", elapsed)
	}
	if n := countAction(c.Events(), ActionStall); n != 2 {
		t.Fatalf("stalls = %d, want 2 (non-cooperative stall must be recorded)", n)
	}
	if countAction(c.Events(), ActionFailed) != 1 {
		t.Fatal("failed event missing")
	}
}

func TestWatchNonCooperativeThenRecovers(t *testing.T) {
	c := NewCollector()
	w := NewWatchdog(c)
	spec := WatchSpec{Name: "stubborn-ok", Timeout: 20 * time.Millisecond, MaxRetries: 2, RetryDelay: time.Millisecond}

	// attempts is shared with the watchdog's goroutine (a non-cooperative
	// attempt keeps running after the watchdog reclaims control), so it must
	// be atomic to avoid a data race.
	var attempts atomic.Int64
	err := w.Watch(context.Background(), spec, func(ctx context.Context) error {
		n := attempts.Add(1)
		if n <= 2 {
			time.Sleep(5 * time.Second) // ignore ctx, stall
			return nil
		}
		return nil // cooperative on the last attempt
	})
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	if attempts.Load() != 3 {
		t.Fatalf("attempts = %d, want 3", attempts.Load())
	}
	ev := c.Events()
	if countAction(ev, ActionStall) != 2 || countAction(ev, ActionRecovered) != 1 {
		t.Fatalf("stall/recover: stalls=%d recovered=%d", countAction(ev, ActionStall), countAction(ev, ActionRecovered))
	}
}

func TestWatchNonRetryableFailsImmediately(t *testing.T) {
	c := NewCollector()
	w := NewWatchdog(c)
	permErr := errors.New("permanent: bad api key")
	spec := WatchSpec{
		Name: "auth", Timeout: time.Second, MaxRetries: 5, RetryDelay: time.Millisecond,
		Retryable: func(err error) bool { return !errors.Is(err, permErr) },
	}

	attempts := 0
	err := w.Watch(context.Background(), spec, func(ctx context.Context) error {
		attempts++
		return permErr
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1 (non-retryable must not retry)", attempts)
	}
	if n := countAction(c.Events(), ActionRetry); n != 0 {
		t.Fatalf("retries = %d, want 0", n)
	}
	if countAction(c.Events(), ActionFailed) != 1 {
		t.Fatal("failed event missing")
	}
}

func TestWatchTransientErrorRetries(t *testing.T) {
	c := NewCollector()
	w := NewWatchdog(c)
	transient := errors.New("connection reset")
	spec := WatchSpec{
		Name: "net", Timeout: time.Second, MaxRetries: 2, RetryDelay: time.Millisecond,
		Retryable: func(err error) bool { return true },
	}

	attempts := 0
	err := w.Watch(context.Background(), spec, func(ctx context.Context) error {
		attempts++
		if attempts <= 2 {
			return transient
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if countAction(c.Events(), ActionRecovered) != 1 {
		t.Fatal("recovered event missing")
	}
}

func TestBackoffWithJitterBounded(t *testing.T) {
	// With a small cap, even attempt 10 must not exceed the cap.
	for i := 0; i < 200; i++ {
		d := backoffWithJitter(time.Second, 30*time.Second, 10)
		if d < 0 || d > 30*time.Second {
			t.Fatalf("backoff out of bounds: %v", d)
		}
	}
	// Jitter actually varies (not always the full value).
	var min, max time.Duration
	seen := map[time.Duration]bool{}
	for i := 0; i < 100; i++ {
		d := backoffWithJitter(time.Second, 30*time.Second, 2) // exp=4s
		seen[d] = true
		if i == 0 {
			min, max = d, d
		}
		if d < min {
			min = d
		}
		if d > max {
			max = d
		}
	}
	if len(seen) < 2 {
		t.Fatalf("jitter produced a single value (%v) — not randomized", min)
	}
	if max > 4*time.Second {
		t.Fatalf("jitter exceeded exponential: %v", max)
	}
	_ = min
}

func TestWatchZeroMaxRetriesStillAttemptsOnce(t *testing.T) {
	c := NewCollector()
	w := NewWatchdog(c)
	spec := WatchSpec{Name: "once", Timeout: time.Second, MaxRetries: 0}

	var called atomic.Bool
	if err := w.Watch(context.Background(), spec, func(ctx context.Context) error {
		called.Store(true)
		return nil
	}); err != nil {
		t.Fatalf("Watch: %v", err)
	}
	if !called.Load() {
		t.Fatal("operation never ran")
	}
}
