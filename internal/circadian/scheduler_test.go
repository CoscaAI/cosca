package circadian

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// TestScheduler_StartStop verifies Start/Stop lifecycle: the scheduler runs
// after Start, stops after Stop, both calls are idempotent, and the loop
// goroutine does not leak.
func TestScheduler_StartStop(t *testing.T) {
	e := New()
	s := NewScheduler(e, t.TempDir())

	if s.interval != 30*time.Second {
		t.Errorf("default interval = %v, want 30s", s.interval)
	}
	if s.running {
		t.Error("expected not running before Start")
	}

	before := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	if !s.running {
		t.Error("expected running after Start")
	}
	s.Start(ctx) // idempotent: must not spawn a second loop
	time.Sleep(20 * time.Millisecond)

	s.Stop()
	if s.running {
		t.Error("expected not running after Stop")
	}
	s.Stop()                          // idempotent: must not panic or double-close
	time.Sleep(50 * time.Millisecond) // give the loop goroutine time to exit
	cancel()

	after := runtime.NumGoroutine()
	if after > before {
		t.Errorf("goroutine leak: %d before Start, %d after Stop", before, after)
	}
}

// TestScheduler_EvaluatesState drives the scheduler through a full window:
// the engine descends into the ORC window, the rest cycle runs exactly once,
// and returning to awake re-arms the gate for the next window.
func TestScheduler_EvaluatesState(t *testing.T) {
	e := New()
	reach(t, e, StateResting)

	// Make the idle clock demand the ORC window: resting -> sleeping is a
	// valid single step of the transition matrix.
	e.lastActivity = time.Now().Add(-20 * time.Minute)

	s := NewScheduler(e, t.TempDir())
	s.interval = 5 * time.Millisecond // fast ticks for the test

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)
	defer s.Stop()

	// Wait until the engine enters the ORC window and the rest cycle runs.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if e.State() == StateSleeping && s.ran() && s.LastORC() != nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if e.State() != StateSleeping {
		t.Fatalf("engine never reached the ORC window (state %s)", e.State())
	}
	if !s.ran() {
		t.Fatal("ORC did not run in the sleep window")
	}
	if s.LastORC() == nil {
		t.Fatal("LastORC() is nil after the cycle ran")
	}

	// The cycle must run at most once per window: further ticks in the same
	// window must not re-run it (the result pointer stays the same).
	first := s.LastORC()
	time.Sleep(40 * time.Millisecond) // several more ticks
	if got := s.LastORC(); got != first {
		t.Error("ORC reran inside the same sleep window")
	}

	// Simulate the Don interacting: RecordActivity resets the idle clock, the
	// scheduler proposes awake and re-arms the ORC gate for the next window.
	e.RecordActivity()
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if e.State() == StateAwake && !s.ran() {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if e.State() != StateAwake {
		t.Errorf("engine did not return to awake after activity (state %s)", e.State())
	}
	if s.ran() {
		t.Error("ranORC not reset after the engine returned to awake")
	}
}

// TestScheduler_TickGuardedRecoversPanic verifies the tick panic boundary:
// a panic inside a scheduler pass (Evaluate/RunORC) is recovered, logged and
// the next pass still works.
func TestScheduler_TickGuardedRecoversPanic(t *testing.T) {
	// A nil engine makes tick panic on the nil-receiver Evaluate() call.
	bad := NewScheduler(nil, t.TempDir())
	bad.tickGuarded(context.Background()) // must not propagate the panic

	// Subsequent passes with a healthy engine keep working: the engine must
	// still descend into the ORC window on a real tick.
	e := New()
	reach(t, e, StateResting)
	e.lastActivity = time.Now().Add(-20 * time.Minute)

	s := NewScheduler(e, t.TempDir())
	s.tickGuarded(context.Background())
	if e.State() != StateSleeping {
		t.Errorf("state = %s, want %s", e.State(), StateSleeping)
	}
}

// TestScheduler_LoopSurvivesPanickingTicks runs the real evaluation loop with
// an engine that panics on every tick: the panic boundary must keep the loop
// alive so the daemon does not crash.
func TestScheduler_LoopSurvivesPanickingTicks(t *testing.T) {
	s := NewScheduler(nil, t.TempDir()) // Evaluate() panics on nil receiver
	s.interval = 5 * time.Millisecond

	before := runtime.NumGoroutine()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)

	// Let several panicking ticks fire.
	time.Sleep(30 * time.Millisecond)

	if !s.running {
		t.Fatal("scheduler loop died after a panicking tick")
	}
	s.Stop()

	// The loop goroutine must still exit cleanly (no leak).
	time.Sleep(50 * time.Millisecond)
	if after := runtime.NumGoroutine(); after > before {
		t.Errorf("goroutine leak: %d before, %d after", before, after)
	}
}
