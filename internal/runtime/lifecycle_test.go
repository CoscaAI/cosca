package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestNewLifecycle(t *testing.T) {
	t.Parallel()
	lc := NewLifecycle()
	if lc == nil {
		t.Fatal("NewLifecycle returned nil")
	}
	lc.mu.RLock()
	if len(lc.initHooks) != 0 {
		t.Errorf("initHooks should be empty, got %d", len(lc.initHooks))
	}
	if len(lc.startHooks) != 0 {
		t.Errorf("startHooks should be empty, got %d", len(lc.startHooks))
	}
	if len(lc.stopHooks) != 0 {
		t.Errorf("stopHooks should be empty, got %d", len(lc.stopHooks))
	}
	lc.mu.RUnlock()
}

func TestAddInitHook(t *testing.T) {
	t.Parallel()
	lc := NewLifecycle()
	lc.AddInitHook("test", func(_ context.Context, _ *Runtime) error { return nil }, time.Second, false)
	lc.mu.RLock()
	if len(lc.initHooks) != 1 {
		t.Errorf("expected 1 init hook, got %d", len(lc.initHooks))
	}
	if lc.initHooks[0].Name != "test" {
		t.Errorf("hook name = %q, want %q", lc.initHooks[0].Name, "test")
	}
	if lc.initHooks[0].Phase != PhaseInit {
		t.Errorf("hook phase = %v, want %v", lc.initHooks[0].Phase, PhaseInit)
	}
	if lc.initHooks[0].Timeout != time.Second {
		t.Errorf("hook timeout = %v, want %v", lc.initHooks[0].Timeout, time.Second)
	}
	lc.mu.RUnlock()
}

func TestAddStartHook(t *testing.T) {
	t.Parallel()
	lc := NewLifecycle()
	lc.AddStartHook("start-test", func(_ context.Context, _ *Runtime) error { return nil }, 0, true)
	lc.mu.RLock()
	if len(lc.startHooks) != 1 {
		t.Errorf("expected 1 start hook, got %d", len(lc.startHooks))
	}
	if lc.startHooks[0].Name != "start-test" {
		t.Errorf("hook name = %q", lc.startHooks[0].Name)
	}
	if lc.startHooks[0].Required != true {
		t.Error("hook should be required")
	}
	lc.mu.RUnlock()
}

func TestAddStopHook(t *testing.T) {
	t.Parallel()
	lc := NewLifecycle()
	lc.AddStopHook("stop-test", func(_ context.Context, _ *Runtime) error { return nil }, 5*time.Second, false)
	lc.mu.RLock()
	if len(lc.stopHooks) != 1 {
		t.Errorf("expected 1 stop hook, got %d", len(lc.stopHooks))
	}
	lc.mu.RUnlock()
}

func TestPhaseString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		phase Phase
		want  string
	}{
		{PhaseInit, "init"},
		{PhaseStart, "start"},
		{PhaseStop, "stop"},
		{PhaseRestart, "restart"},
		{Phase(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.phase.String(); got != tt.want {
				t.Errorf("Phase.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSetLogger(t *testing.T) {
	t.Parallel()
	lc := NewLifecycle()
	if lc == nil {
		t.Fatal("NewLifecycle returned nil")
	}
	lc.SetLogger(zerolog.Nop())
	// Should not panic — SetLogger accepts any zerolog.Logger as the field
	// is a value type and cannot be nil.
}

func TestExecuteInitWithNilSubsystems(t *testing.T) {
	t.Parallel()
	r := New()
	ctx := context.Background()
	// All subsystems are nil - init should succeed
	err := r.lifecycle.ExecuteInit(ctx, r)
	if err != nil {
		t.Fatalf("ExecuteInit with nil subsystems failed: %v", err)
	}
}

func TestExecuteStartWithNilSubsystems(t *testing.T) {
	t.Parallel()
	r := New()
	ctx := context.Background()
	err := r.lifecycle.ExecuteStart(ctx, r)
	if err != nil {
		t.Fatalf("ExecuteStart with nil subsystems failed: %v", err)
	}
}

func TestExecuteStopWithNilSubsystems(t *testing.T) {
	t.Parallel()
	r := New()
	ctx := context.Background()
	err := r.lifecycle.ExecuteStop(ctx, r)
	if err != nil {
		t.Fatalf("ExecuteStop with nil subsystems failed: %v", err)
	}
}

func TestHookExecutionOrder(t *testing.T) {
	t.Parallel()
	lc := NewLifecycle()
	var order []string
	lc.AddInitHook("first", func(_ context.Context, _ *Runtime) error {
		order = append(order, "first")
		return nil
	}, time.Second, false)
	lc.AddInitHook("second", func(_ context.Context, _ *Runtime) error {
		order = append(order, "second")
		return nil
	}, time.Second, false)

	r := New()
	err := lc.executeHooks(context.Background(), r, lc.initHooks, PhaseInit)
	if err != nil {
		t.Fatalf("executeHooks failed: %v", err)
	}
	if len(order) != 2 {
		t.Fatalf("expected 2 hooks executed, got %d", len(order))
	}
	if order[0] != "first" || order[1] != "second" {
		t.Errorf("order = %v, want [first second]", order)
	}
}

func TestNonRequiredHookFailure(t *testing.T) {
	t.Parallel()
	lc := NewLifecycle()
	lc.AddInitHook("failing", func(_ context.Context, _ *Runtime) error {
		return errors.New("hook failed")
	}, time.Second, false)

	r := New()
	err := lc.executeHooks(context.Background(), r, lc.initHooks, PhaseInit)
	if err != nil {
		t.Fatalf("Non-required hook failure should not cause phase failure, got: %v", err)
	}
}

func TestRequiredHookFailure(t *testing.T) {
	t.Parallel()
	lc := NewLifecycle()
	lc.AddInitHook("critical", func(_ context.Context, _ *Runtime) error {
		return errors.New("critical failure")
	}, time.Second, true)

	r := New()
	err := lc.executeHooks(context.Background(), r, lc.initHooks, PhaseInit)
	if err == nil {
		t.Fatal("Required hook failure should cause phase failure")
	}
}

func TestHookTimeout(t *testing.T) {
	t.Parallel()
	lc := NewLifecycle()
	lc.AddInitHook("slow", func(ctx context.Context, _ *Runtime) error {
		// Use a very short deadline to test timeout
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return nil
		}
	}, time.Millisecond, false)

	r := New()
	ctx := context.Background()
	err := lc.executeHooks(ctx, r, lc.initHooks, PhaseInit)
	if err != nil {
		t.Fatalf("Non-required timeout should not fail phase: %v", err)
	}
}

func TestPhaseDuration(t *testing.T) {
	t.Parallel()
	lc := NewLifecycle()
	// Register an init hook that actually takes some time: the phase duration
	// must be measurable even on platforms with coarse clock granularity
	// (Windows: time.Now() can jump in ~0.5ms steps, so a phase of empty hooks
	// records 0s there).
	lc.AddInitHook("slow-init", func(_ context.Context, _ *Runtime) error {
		time.Sleep(2 * time.Millisecond)
		return nil
	}, time.Second, true)
	r := New()
	_ = lc.executeHooks(context.Background(), r, lc.initHooks, PhaseInit)
	d, ok := lc.PhaseDuration(PhaseInit)
	if !ok {
		t.Error("PhaseDuration should return ok for executed phase")
	}
	if d <= 0 {
		t.Errorf("Duration should be > 0, got %v", d)
	}
}

func TestPhaseDurationNotExecuted(t *testing.T) {
	t.Parallel()
	lc := NewLifecycle()
	_, ok := lc.PhaseDuration(PhaseRestart)
	if ok {
		t.Error("PhaseDuration should return false for unexecuted phase")
	}
}

func TestDefaultHooksRegisteredOnExecute(t *testing.T) {
	t.Parallel()
	r := New()
	ctx := context.Background()

	_ = r.lifecycle.ExecuteInit(ctx, r)
	_ = r.lifecycle.ExecuteStart(ctx, r)
	_ = r.lifecycle.ExecuteStop(ctx, r)

	r.lifecycle.mu.RLock()
	if len(r.lifecycle.initHooks) < 7 {
		t.Errorf("expected at least 7 init hooks, got %d", len(r.lifecycle.initHooks))
	}
	if len(r.lifecycle.startHooks) < 7 {
		t.Errorf("expected at least 7 start hooks, got %d", len(r.lifecycle.startHooks))
	}
	if len(r.lifecycle.stopHooks) < 7 {
		t.Errorf("expected at least 7 stop hooks, got %d", len(r.lifecycle.stopHooks))
	}
	r.lifecycle.mu.RUnlock()
}
