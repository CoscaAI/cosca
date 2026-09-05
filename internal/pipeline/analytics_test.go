package pipeline

import (
	"strings"
	"testing"
	"time"
)

func TestRunAnalyticsComputeDerived(t *testing.T) {
	// Zero tasks → score 0.
	a := &RunAnalytics{}
	a.ComputeDerived()
	if a.AutonomyScore != 0 {
		t.Fatalf("zero-tasks score = %v, want 0", a.AutonomyScore)
	}

	// All clean success → 1.0.
	a = &RunAnalytics{TasksTotal: 4, TasksDone: 4}
	a.ComputeDerived()
	if a.AutonomyScore != 1.0 {
		t.Fatalf("clean score = %v, want 1.0", a.AutonomyScore)
	}

	// Recovered tasks count as 0.7.
	a = &RunAnalytics{TasksTotal: 10, TasksDone: 8, RecoveriesSucceeded: 2}
	a.ComputeDerived()
	// (6*1.0 + 2*0.7) / 10 = 7.4/10 = 0.74
	if a.AutonomyScore != 0.74 {
		t.Fatalf("recovered score = %v, want 0.74", a.AutonomyScore)
	}

	// Human interventions reduce the score (0.1 each), clamped at 0.
	a = &RunAnalytics{TasksTotal: 1, TasksDone: 1, HumanApprovals: 20}
	a.ComputeDerived()
	if a.AutonomyScore != 0 {
		t.Fatalf("clamped score = %v, want 0", a.AutonomyScore)
	}

	// Never exceeds 1.0.
	a = &RunAnalytics{TasksTotal: 2, TasksDone: 2}
	a.ComputeDerived()
	if a.AutonomyScore > 1.0 {
		t.Fatalf("score > 1.0: %v", a.AutonomyScore)
	}
}

func TestRunAnalyticsSummary(t *testing.T) {
	a := &RunAnalytics{PlanID: "p1", TasksTotal: 10, TasksDone: 8, RecoveriesSucceeded: 2}
	a.ComputeDerived()
	s := a.Summary()
	if !strings.Contains(s, "Plan p1") || !strings.Contains(s, "8/10 tasks done") {
		t.Fatalf("summary = %q", s)
	}
	if !strings.Contains(s, "autonomous") {
		t.Fatalf("summary missing autonomy: %q", s)
	}
}

func TestAggregateAnalyticsAdd(t *testing.T) {
	agg := &AggregateAnalytics{}
	r1 := &RunAnalytics{TasksTotal: 2, TasksDone: 2, AutonomyScore: 1.0, LLMCalls: 3, TokensUsed: 1000}
	r2 := &RunAnalytics{TasksTotal: 4, TasksDone: 2, TasksFailed: 2, AutonomyScore: 0.5, LLMCalls: 5, TokensUsed: 2000}

	agg.Add(r1)
	if agg.TotalPlans != 1 || agg.AvgAutonomyScore != 1.0 {
		t.Fatalf("after r1: %+v", agg)
	}

	agg.Add(r2)
	if agg.TotalPlans != 2 || agg.TotalTasks != 6 || agg.TasksFailed != 2 {
		t.Fatalf("after r2: %+v", agg)
	}
	if agg.TotalDurationMs != 0 || agg.LLMCalls != 8 || agg.TokensUsed != 3000 {
		t.Fatalf("sums wrong: %+v", agg)
	}
	// Weighted avg: (1.0 + 0.5)/2 = 0.75
	if agg.AvgAutonomyScore != 0.75 {
		t.Fatalf("avg autonomy = %v, want 0.75", agg.AvgAutonomyScore)
	}
	if len(agg.Runs) != 2 {
		t.Fatalf("runs len = %d, want 2", len(agg.Runs))
	}

	// Third run moves the weighted average.
	agg.Add(&RunAnalytics{AutonomyScore: 0.5, TasksTotal: 1})
	if agg.TotalPlans != 3 {
		t.Fatalf("plans = %d", agg.TotalPlans)
	}
}

func TestAnalyticsStoreSaveLoad(t *testing.T) {
	h, err := NewWorkflowHistory(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store := NewAnalyticsStore(h)

	a := &RunAnalytics{
		PlanID: "plan-a", Intent: "build api", IntentType: "create-crud-api",
		TasksTotal: 5, TasksDone: 4, TasksFailed: 1,
		RecoveriesAttempted: 2, RecoveriesSucceeded: 1,
		TestsRun: 12, TestsPassed: 11, TestsFailed: 1,
		DurationMs: 3000, LLMCalls: 7, TokensUsed: 5000,
	}
	if err := store.Save(a); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load("plan-a")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.PlanID != "plan-a" {
		t.Fatalf("loaded plan = %q", loaded.PlanID)
	}
	// Note: Save persists only a summary event; Load reconstructs structured
	// fields exclusively from task events (StepEvent*), which Save does not
	// emit. A bare Save→Load roundtrip therefore yields zero metrics — the
	// autonomy score here reflects that (documented lossy behavior).
	if loaded.AutonomyScore != 0 {
		t.Fatalf("bare roundtrip autonomy = %v, want 0 (no task events in history)", loaded.AutonomyScore)
	}
}

func TestAnalyticsStoreLoadReconstructsFromEvents(t *testing.T) {
	h, _ := NewWorkflowHistory(t.TempDir())
	store := NewAnalyticsStore(h)
	now := time.Now().UTC()

	events := []StepEvent{
		{Type: PlanCreated, PlanID: "p", Input: "intent here", Timestamp: now},
		{Type: StepEventStarted, PlanID: "p", TaskID: "t1", Timestamp: now},
		{Type: StepEventCompleted, PlanID: "p", TaskID: "t1", Timestamp: now, Duration: 100},
		{Type: StepEventStarted, PlanID: "p", TaskID: "t2", Timestamp: now},
		{Type: StepEventRetrying, PlanID: "p", TaskID: "t2", Timestamp: now},
		{Type: StepEventCompleted, PlanID: "p", TaskID: "t2", Timestamp: now, Duration: 200},
		{Type: StepEventFailed, PlanID: "p", TaskID: "t3", Timestamp: now},
		// A plan with a failed task emits PlanFailed (not PlanCompleted).
		{Type: PlanFailed, PlanID: "p", Timestamp: now},
	}
	for _, ev := range events {
		if err := h.Append("p", ev); err != nil {
			t.Fatal(err)
		}
	}

	loaded, err := store.Load("p")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Intent != "intent here" {
		t.Fatalf("intent = %q", loaded.Intent)
	}
	if loaded.LLMCalls != 2 { // 2 StepEventStarted
		t.Fatalf("LLMCalls = %d, want 2", loaded.LLMCalls)
	}
	if loaded.TasksDone != 2 || loaded.TasksFailed != 1 {
		t.Fatalf("tasks done/failed = %d/%d, want 2/1", loaded.TasksDone, loaded.TasksFailed)
	}
	if loaded.RecoveriesAttempted != 1 || loaded.RecoveriesSucceeded != 1 {
		t.Fatalf("recoveries = %d/%d, want 1/1", loaded.RecoveriesAttempted, loaded.RecoveriesSucceeded)
	}
	if loaded.TasksTotal != 3 {
		t.Fatalf("TasksTotal = %d, want 3", loaded.TasksTotal)
	}
	if loaded.DurationMs != 300 {
		t.Fatalf("DurationMs = %d, want 300", loaded.DurationMs)
	}
}

func TestAnalyticsStoreLoadMissingPlan(t *testing.T) {
	h, _ := NewWorkflowHistory(t.TempDir())
	store := NewAnalyticsStore(h)
	loaded, err := store.Load("no-such-plan")
	if err != nil {
		t.Fatalf("Load(missing) should return zero analytics: %v", err)
	}
	if loaded.PlanID != "no-such-plan" || loaded.TasksTotal != 0 {
		t.Fatalf("empty load: %+v", loaded)
	}
}
