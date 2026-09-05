package circadian

import (
	"testing"
	"time"
)

// mustTransition transitions e to the given state, failing the test on error.
func mustTransition(t *testing.T, e *Engine, to CircadianState, reason string) {
	t.Helper()
	if err := e.TransitionTo(to, reason); err != nil {
		t.Fatalf("transition to %s: %v", to, err)
	}
}

// reach walks the engine through the natural descent
// awake -> idle -> resting -> sleeping.
func reach(t *testing.T, e *Engine, target CircadianState) {
	t.Helper()
	steps := []CircadianState{StateIdle, StateResting, StateSleeping}
	for _, s := range steps {
		if e.State() == target {
			return
		}
		mustTransition(t, e, s, "descent")
		if e.State() == target {
			return
		}
	}
	t.Fatalf("could not reach %s (current %s)", target, e.State())
}

func TestNew_StartsAwake(t *testing.T) {
	e := New()
	if got := e.State(); got != StateAwake {
		t.Errorf("State() = %s, want %s", got, StateAwake)
	}
	if got := e.IdleDuration(); got > time.Second {
		t.Errorf("IdleDuration() = %v on fresh engine, want < 1s", got)
	}
	if got := len(e.Transitions()); got != 0 {
		t.Errorf("Transitions() = %d entries on fresh engine, want 0", got)
	}
}

func TestTransitionTo_Valid(t *testing.T) {
	e := New()

	if err := e.TransitionTo(StateFocused, "p0 task"); err != nil {
		t.Fatalf("TransitionTo(focused): %v", err)
	}
	if got := e.State(); got != StateFocused {
		t.Errorf("State() = %s, want %s", got, StateFocused)
	}

	tr := e.Transitions()
	if len(tr) != 1 {
		t.Fatalf("Transitions() = %d entries, want 1", len(tr))
	}
	if tr[0].From != StateAwake || tr[0].To != StateFocused {
		t.Errorf("transition = %s -> %s, want awake -> focused", tr[0].From, tr[0].To)
	}
	if tr[0].At.IsZero() {
		t.Error("transition timestamp is zero")
	}
	if tr[0].Reason != "p0 task" {
		t.Errorf("reason = %q, want %q", tr[0].Reason, "p0 task")
	}
}

func TestTransitionTo_Invalid(t *testing.T) {
	e := New()

	// awake -> sleeping directly must be rejected (must descend through
	// idle -> resting first).
	if err := e.TransitionTo(StateSleeping, "skip descent"); err == nil {
		t.Error("TransitionTo(sleeping) from awake: expected error, got nil")
	}
	if got := e.State(); got != StateAwake {
		t.Errorf("State() = %s after rejected transition, want %s", got, StateAwake)
	}
	if got := len(e.Transitions()); got != 0 {
		t.Errorf("Transitions() = %d entries after rejected transition, want 0", got)
	}

	// Unknown target state must be rejected.
	if err := e.TransitionTo(CircadianState("bogus"), "test"); err == nil {
		t.Error("TransitionTo(bogus): expected error, got nil")
	}

	// Self-transition must be rejected.
	if err := e.TransitionTo(StateAwake, "stay"); err == nil {
		t.Error("TransitionTo(awake) from awake: expected error, got nil")
	}
}

func TestTransitionTo_SleepingWakeViaWake(t *testing.T) {
	e := New()
	reach(t, e, StateSleeping)
	if got := e.State(); got != StateSleeping {
		t.Fatalf("State() = %s, want %s", got, StateSleeping)
	}

	// sleeping -> awake is valid via a wake event.
	if err := e.TransitionTo(StateAwake, "wake"); err != nil {
		t.Fatalf("TransitionTo(awake) from sleeping: %v", err)
	}
	if got := e.State(); got != StateAwake {
		t.Errorf("State() = %s, want %s", got, StateAwake)
	}
}

func TestCanRun_AwakeAllowsEverything(t *testing.T) {
	e := New()
	for _, act := range allActivities {
		if !e.CanRun(act) {
			t.Errorf("CanRun(%s) in awake = false, want true", act)
		}
	}
}

func TestCanRun_RestingBlocksAgent(t *testing.T) {
	e := New()
	reach(t, e, StateResting)

	if got := e.CanRun(ActivityRunAgent); got {
		t.Error("CanRun(run_agent) in resting = true, want false")
	}
	if got := e.CanRun(ActivityBenchmark); got {
		t.Error("CanRun(benchmark) in resting = true, want false")
	}
	if got := e.CanRun(ActivityAudit); got {
		t.Error("CanRun(audit) in resting = true, want false")
	}
	if got := e.CanRun(ActivityHeavyIndex); got {
		t.Error("CanRun(heavy_index) in resting = true, want false")
	}
	if got := e.CanRun(ActivityLightMaintenance); !got {
		t.Error("CanRun(light_maintenance) in resting = false, want true")
	}
}

func TestCanRun_SleepingOnlyORC(t *testing.T) {
	e := New()
	reach(t, e, StateSleeping)

	if got := e.CanRun(ActivityORC); !got {
		t.Error("CanRun(orc) in sleeping = false, want true")
	}
	if got := e.CanRun(ActivityRunAgent); got {
		t.Error("CanRun(run_agent) in sleeping = true, want false")
	}
	if got := e.CanRun(ActivityBenchmark); got {
		t.Error("CanRun(benchmark) in sleeping = true, want false")
	}
	// The ORC window still allows cheap housekeeping.
	if got := e.CanRun(ActivityLightMaintenance); !got {
		t.Error("CanRun(light_maintenance) in sleeping = false, want true")
	}
}

func TestCanRun_MaintenanceForced(t *testing.T) {
	e := New()
	mustTransition(t, e, StateMaintenance, "forced")

	if got := e.CanRun(ActivityORC); !got {
		t.Error("CanRun(orc) in maintenance = false, want true")
	}
	if got := e.CanRun(ActivityLightMaintenance); !got {
		t.Error("CanRun(light_maintenance) in maintenance = false, want true")
	}
	if got := e.CanRun(ActivityRunAgent); got {
		t.Error("CanRun(run_agent) in maintenance = true, want false")
	}
}

func TestEvaluate_StateProgression(t *testing.T) {
	cfg := DefaultConfig()
	cases := []struct {
		name string
		idle time.Duration
		want CircadianState
	}{
		{"fresh", time.Second, StateAwake},
		{"just under awake timeout", cfg.AwakeTimeout - time.Second, StateAwake},
		{"at awake timeout boundary", cfg.AwakeTimeout, StateIdle},
		{"mid idle window", 2 * time.Minute, StateIdle},
		{"at idle-to-resting boundary", cfg.IdleToResting, StateResting},
		{"just past idle-to-resting", cfg.IdleToResting + time.Second, StateResting},
		{"mid resting window", 10 * time.Minute, StateResting},
		{"at resting-to-sleeping boundary", cfg.RestingToSleeping, StateSleeping},
		{"just past resting-to-sleeping", cfg.RestingToSleeping + time.Second, StateSleeping},
		{"long orc window", 30 * time.Minute, StateSleeping},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := evaluate(cfg, tc.idle); got != tc.want {
				t.Errorf("evaluate(idle=%v) = %s, want %s", tc.idle, got, tc.want)
			}
		})
	}
}

func TestEvaluate_MethodUsesIdleClock(t *testing.T) {
	e := New()

	// < 30s idle -> awake
	e.lastActivity = time.Now()
	if got := e.Evaluate(); got != StateAwake {
		t.Errorf("Evaluate() = %s, want %s", got, StateAwake)
	}

	// > 5min idle -> resting
	e.lastActivity = time.Now().Add(-10 * time.Minute)
	if got := e.Evaluate(); got != StateResting {
		t.Errorf("Evaluate() = %s, want %s", got, StateResting)
	}

	// > 15min idle -> sleeping
	e.lastActivity = time.Now().Add(-20 * time.Minute)
	if got := e.Evaluate(); got != StateSleeping {
		t.Errorf("Evaluate() = %s, want %s", got, StateSleeping)
	}
}

func TestRecordActivity_ResetsIdle(t *testing.T) {
	e := New()
	e.lastActivity = time.Now().Add(-10 * time.Minute)

	if got := e.IdleDuration(); got < 9*time.Minute {
		t.Errorf("IdleDuration() = %v before RecordActivity, want >= 9m", got)
	}

	e.RecordActivity()
	if got := e.IdleDuration(); got >= time.Second {
		t.Errorf("IdleDuration() = %v after RecordActivity, want < 1s", got)
	}
}

func TestShouldRest(t *testing.T) {
	e := New()
	if got := e.ShouldRest(); got {
		t.Error("ShouldRest() on fresh engine = true, want false")
	}

	e.lastActivity = time.Now().Add(-6 * time.Minute)
	if got := e.ShouldRest(); !got {
		t.Error("ShouldRest() after 6min idle = false, want true")
	}
}

// TestGateMatrix locks the full activity gate table.
func TestGateMatrix(t *testing.T) {
	want := map[CircadianState]map[Activity]bool{
		StateAwake: {
			ActivityRunAgent:         true,
			ActivityBenchmark:        true,
			ActivityAudit:            true,
			ActivityMaintenance:      true,
			ActivityORC:              true,
			ActivityHeavyIndex:       true,
			ActivityLightMaintenance: true,
		},
		StateFocused: {
			ActivityRunAgent:   true,
			ActivityBenchmark:  true,
			ActivityAudit:      true,
			ActivityHeavyIndex: true,
		},
		StateIdle: {
			ActivityRunAgent:    true,
			ActivityMaintenance: true,
		},
		StateResting: {
			ActivityLightMaintenance: true,
		},
		StateSleeping: {
			ActivityLightMaintenance: true,
			ActivityORC:              true,
		},
		StateMaintenance: {
			ActivityLightMaintenance: true,
			ActivityORC:              true,
		},
	}

	e := New()
	states := []CircadianState{StateAwake, StateFocused, StateIdle, StateResting, StateSleeping, StateMaintenance}
	for _, state := range states {
		for _, act := range allActivities {
			wantVal := want[state][act]
			if got := e.allowed(act, state); got != wantVal {
				t.Errorf("allowed(%s, %s) = %v, want %v", act, state, got, wantVal)
			}
		}
	}
}

// TestTransitionMatrix locks the full transition matrix.
func TestTransitionMatrix(t *testing.T) {
	want := map[CircadianState]map[CircadianState]bool{
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

	states := []CircadianState{StateAwake, StateFocused, StateIdle, StateResting, StateSleeping, StateMaintenance}
	for _, from := range states {
		for _, to := range states {
			wantVal := want[from][to]
			if got := validTransitions[from][to]; got != wantVal {
				t.Errorf("validTransitions[%s][%s] = %v, want %v", from, to, got, wantVal)
			}
		}
	}
}

func TestTransitions_HistorySnapshot(t *testing.T) {
	e := New()
	mustTransition(t, e, StateIdle, "timeout")
	mustTransition(t, e, StateResting, "quiet")
	mustTransition(t, e, StateAwake, "wake")

	tr := e.Transitions()
	if len(tr) != 3 {
		t.Fatalf("Transitions() = %d entries, want 3", len(tr))
	}
	if tr[0].From != StateAwake || tr[0].To != StateIdle {
		t.Errorf("tr[0] = %s -> %s, want awake -> idle", tr[0].From, tr[0].To)
	}
	if tr[1].From != StateIdle || tr[1].To != StateResting {
		t.Errorf("tr[1] = %s -> %s, want idle -> resting", tr[1].From, tr[1].To)
	}
	if tr[2].From != StateResting || tr[2].To != StateAwake {
		t.Errorf("tr[2] = %s -> %s, want resting -> awake", tr[2].From, tr[2].To)
	}

	// The returned slice is a snapshot: mutating it must not touch the engine.
	tr[0].Reason = "tampered"
	got := e.Transitions()
	if got[0].Reason == "tampered" {
		t.Error("Transitions() snapshot leaks into engine history")
	}
}
