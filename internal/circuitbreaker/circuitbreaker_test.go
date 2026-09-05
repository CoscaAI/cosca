package circuitbreaker

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestInitialState(t *testing.T) {
	cb := New(DefaultConfig())
	if cb.State() != StateClosed {
		t.Errorf("expected closed, got %s", cb.State())
	}
}

func TestOpenAfterFailures(t *testing.T) {
	cb := New(Config{FailureThreshold: 3, SuccessThreshold: 1, Timeout: time.Minute})

	for i := 0; i < 3; i++ {
		_, err := cb.Execute(func() (interface{}, error) {
			return nil, errors.New("fail")
		})
		if err == nil {
			t.Fatal("expected error")
		}
	}

	if cb.State() != StateOpen {
		t.Errorf("expected open, got %s", cb.State())
	}
}

func TestRejectWhenOpen(t *testing.T) {
	cb := New(Config{FailureThreshold: 1, SuccessThreshold: 1, Timeout: time.Minute})

	// Trigger open state
	cb.Execute(func() (interface{}, error) {
		return nil, errors.New("fail")
	})

	// Should be rejected fast
	_, err := cb.Execute(func() (interface{}, error) {
		return "should not run", nil
	})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestHalfOpenToClosed(t *testing.T) {
	cb := New(Config{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	})

	// Open the circuit
	cb.Execute(func() (interface{}, error) { return nil, errors.New("fail") })
	cb.Execute(func() (interface{}, error) { return nil, errors.New("fail") })

	if cb.State() != StateOpen {
		t.Fatalf("expected open, got %s", cb.State())
	}

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Should be half-open now
	state := cb.State()
	if state != StateHalfOpen {
		t.Fatalf("expected half-open, got %s", state)
	}

	// Succeed twice
	for i := 0; i < 2; i++ {
		_, err := cb.Execute(func() (interface{}, error) {
			return "success", nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if cb.State() != StateClosed {
		t.Errorf("expected closed, got %s", cb.State())
	}
}

func TestHalfOpenToOpenOnFailure(t *testing.T) {
	cb := New(Config{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	})

	// Open the circuit
	cb.Execute(func() (interface{}, error) { return nil, errors.New("fail") })
	cb.Execute(func() (interface{}, error) { return nil, errors.New("fail") })

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Should be half-open, fail once -> back to open
	_, err := cb.Execute(func() (interface{}, error) {
		return nil, errors.New("fail in half-open")
	})
	if err == nil {
		t.Fatal("expected error")
	}

	if cb.State() != StateOpen {
		t.Errorf("expected open after half-open failure, got %s", cb.State())
	}
}

func TestStateChangeCallback(t *testing.T) {
	var changeCount int32
	cb := New(Config{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          50 * time.Millisecond,
		OnStateChange: func(from, to State) {
			atomic.AddInt32(&changeCount, 1)
		},
	})

	// Closed -> Open
	cb.Execute(func() (interface{}, error) { return nil, errors.New("fail") })

	time.Sleep(100 * time.Millisecond)

	// Open -> HalfOpen (triggered by allowRequest check)
	cb.State()

	// HalfOpen -> Closed
	cb.Execute(func() (interface{}, error) { return "ok", nil })

	if n := atomic.LoadInt32(&changeCount); n < 2 {
		t.Errorf("expected at least 2 callbacks, got %d", n)
	}
}

func TestReset(t *testing.T) {
	cb := New(Config{FailureThreshold: 1, SuccessThreshold: 1, Timeout: time.Minute})

	cb.Execute(func() (interface{}, error) { return nil, errors.New("fail") })
	if cb.State() != StateOpen {
		t.Fatalf("expected open, got %s", cb.State())
	}

	cb.Reset()
	if cb.State() != StateClosed {
		t.Errorf("expected closed after reset, got %s", cb.State())
	}

	// Should accept requests again
	_, err := cb.Execute(func() (interface{}, error) {
		return "ok", nil
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConcurrentAccess(t *testing.T) {
	cb := New(Config{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
	})

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				cb.Execute(func() (interface{}, error) {
					time.Sleep(time.Microsecond)
					return "ok", nil
				})
				cb.State()
				_ = cb.Stats()
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestStats(t *testing.T) {
	cb := New(DefaultConfig())

	cb.Execute(func() (interface{}, error) { return "ok", nil })
	cb.Execute(func() (interface{}, error) { return nil, errors.New("fail") })

	stats := cb.Stats()
	if stats.Calls != 2 {
		t.Errorf("expected 2 calls, got %d", stats.Calls)
	}
	if stats.Successes != 1 {
		t.Errorf("expected 1 success, got %d", stats.Successes)
	}
	if stats.Failures != 1 {
		t.Errorf("expected 1 failure, got %d", stats.Failures)
	}
}
