package pipeline

import (
	"strings"
	"testing"
	"time"
)

func TestNewBenchmark(t *testing.T) {
	b := NewBenchmark("build the platform")
	if !strings.HasPrefix(b.SessionID, "BENCH-") || b.Goal != "build the platform" {
		t.Fatalf("benchmark: %+v", b)
	}
	if b.StartTime.IsZero() {
		t.Fatal("StartTime must be set")
	}
}

func TestBenchmarkRecordPlan(t *testing.T) {
	b := NewBenchmark("x")
	b.RecordPlan(nil) // no-op
	if b.TasksPlanned != 0 {
		t.Fatal("nil plan must be ignored")
	}

	plan := &Plan{Tasks: []*TaskNode{
		{ID: "t1", Agent: "cosca-backend"},
		{ID: "t2", Agent: "cosca-testing"},
	}}
	b.RecordPlan(plan)
	if b.TasksPlanned != 2 || b.planTaskCount != 2 {
		t.Fatalf("planned = %d", b.TasksPlanned)
	}
	if len(b.AgentsUsed) != 2 {
		t.Fatalf("agents = %v", b.AgentsUsed)
	}
}

func TestBenchmarkRecordTaskCompletion(t *testing.T) {
	b := NewBenchmark("x")
	b.RecordTaskCompletion(&TaskNode{ID: "t1", Agent: "a", Result: &TaskResult{DurationMs: 100}}, true, "cosca-backend", "gpt-4o")
	b.RecordTaskCompletion(&TaskNode{ID: "t2", Status: TaskFailed}, false, "cosca-backend", "gpt-4o")

	if b.TasksCompleted != 1 || b.TasksFailed != 1 {
		t.Fatalf("completed/failed = %d/%d", b.TasksCompleted, b.TasksFailed)
	}
	// Agent/model dedup.
	if len(b.AgentsUsed) != 1 || len(b.ModelsUsed) != 1 {
		t.Fatalf("agents=%v models=%v", b.AgentsUsed, b.ModelsUsed)
	}
	if len(b.taskLatencies) != 1 || b.taskLatencies[0] != 100*time.Millisecond {
		t.Fatalf("latencies = %v", b.taskLatencies)
	}
}

func TestBenchmarkRecorders(t *testing.T) {
	b := NewBenchmark("x")
	b.RecordError(errSimulated, true)
	b.RecordError(errSimulated, false)
	b.RecordHumanIntervention("asking")
	b.RecordKnowledgeUse(3)
	b.RecordKnowledgeNeeded(true)
	b.RecordToolUse("read_file")
	b.RecordToolUse("read_file") // dedup
	b.RecordToolUse("write_file")
	b.RecordRoutingDecision("cosca-backend", true)
	b.RecordRoutingDecision("cosca-testing", false)
	b.RecordContextDelivery(true)
	b.RecordContextDelivery(false)
	b.RecordVerification(true)
	b.RecordVerification(false)
	b.RecordDelivery(true)

	if b.ErrorsEncountered != 2 || b.ErrorsRecovered != 1 || b.HumanInterventions != 1 {
		t.Fatalf("errors/interventions: %+v", b)
	}
	if b.KnowledgeItems != 3 || len(b.ToolsUsed) != 2 {
		t.Fatalf("knowledge/tools: %+v", b)
	}
	if b.routingDecisions != 2 || b.routingCorrect != 1 {
		t.Fatalf("routing: %d/%d", b.routingCorrect, b.routingDecisions)
	}
}

func TestBenchmarkCalculateScores(t *testing.T) {
	b := NewBenchmark("goal")
	b.RecordPlan(&Plan{Tasks: []*TaskNode{
		{ID: "t1", Agent: "cosca-backend"},
		{ID: "t2", Agent: "cosca-backend"},
		{ID: "t3", Agent: "cosca-backend"},
	}})
	b.RecordTaskCompletion(&TaskNode{ID: "t1"}, true, "cosca-backend", "gpt-4o")
	b.RecordTaskCompletion(&TaskNode{ID: "t2"}, true, "cosca-backend", "gpt-4o")
	b.RecordTaskCompletion(&TaskNode{ID: "t3", Status: TaskFailed}, false, "cosca-backend", "gpt-4o")
	b.RecordKnowledgeUse(1)
	b.RecordKnowledgeNeeded(true)
	b.RecordVerification(true)
	b.RecordVerification(true)
	b.RecordDelivery(true)
	// Let measurable session time elapse: Windows time.Now() can jump in
	// ~0.5ms steps, so an instant CalculateScores() may read Duration = 0.
	time.Sleep(2 * time.Millisecond)
	b.CalculateScores()

	if b.Duration <= 0 || b.EndTime.IsZero() {
		t.Fatal("timing not set")
	}
	if b.Cognitive.PlanQuality != 2.0/3.0 {
		t.Fatalf("plan quality = %v", b.Cognitive.PlanQuality)
	}
	if b.Cognitive.GoalComprehension != 0.7 {
		t.Fatalf("goal comprehension = %v", b.Cognitive.GoalComprehension)
	}
	if b.Cognitive.VerificationRate != 1.0 {
		t.Fatalf("verification = %v", b.Cognitive.VerificationRate)
	}
	if b.Cognitive.VADScore <= 0 || b.Cognitive.VADScore > 100 {
		t.Fatalf("VAD = %v", b.Cognitive.VADScore)
	}
	// No errors → recovery = 1.0.
	if b.Cognitive.RecoveryRate != 1.0 {
		t.Fatalf("recovery = %v", b.Cognitive.RecoveryRate)
	}
}

func TestBenchmarkEmptyScores(t *testing.T) {
	b := NewBenchmark("nothing")
	b.CalculateScores()
	if b.Cognitive.GoalComprehension != 0.1 || b.Cognitive.PlanQuality != 0.3 {
		t.Fatalf("empty scores: %+v", b.Cognitive)
	}
	if b.Cognitive.VerificationRate != 0 || b.Cognitive.DeliveryRate != 0 {
		t.Fatalf("empty verification/delivery: %+v", b.Cognitive)
	}
}

func TestBenchmarkArchitectureAssessment(t *testing.T) {
	b := NewBenchmark("x")
	b.RecordPlan(&Plan{Tasks: []*TaskNode{{ID: "t1", Agent: "cosca-backend"}}})
	b.RecordTaskCompletion(&TaskNode{ID: "t1"}, true, "cosca-backend", "gpt-4o")
	b.RecordVerification(true)
	b.RecordKnowledgeUse(1)
	b.CalculateScores()

	if len(b.ArchitectureSufficient) == 0 {
		t.Fatalf("no strengths: %+v", b.ArchitectureSufficient)
	}
	// Gaps: empty benchmark.
	b2 := NewBenchmark("empty")
	b2.CalculateScores()
	if len(b2.ArchitectureGaps) == 0 {
		t.Fatalf("empty benchmark should have gaps: %+v", b2)
	}
}

func TestBenchmarkVADHelpers(t *testing.T) {
	b := NewBenchmark("x")
	b.Cognitive.VADScore = 85
	bar := b.vadBar()
	if !strings.Contains(bar, "85/100") || !strings.Contains(bar, "STRONG") {
		t.Fatalf("bar: %s", bar)
	}

	grades := map[int]string{
		95: "EXCEPTIONAL", 85: "STRONG", 70: "CAPABLE",
		55: "DEVELOPING", 35: "WEAK", 10: "CRITICAL",
	}
	for score, want := range grades {
		if got := b.vadGrade(score); !strings.Contains(got, want) {
			t.Errorf("vadGrade(%d) = %q, want %q", score, got, want)
		}
	}
}

func TestBenchmarkGenerateReport(t *testing.T) {
	b := NewBenchmark("the goal")
	b.RecordPlan(&Plan{Tasks: []*TaskNode{{ID: "t1", Agent: "cosca-backend"}}})
	b.RecordTaskCompletion(&TaskNode{ID: "t1"}, true, "cosca-backend", "gpt-4o")
	out := b.GenerateReport()

	for _, want := range []string{"BENCHMARK REPORT", "VAD SCORE", "the goal", "Planned 1 tasks, completed 1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("report missing %q", want)
		}
	}
}

func TestBenchmarkPlannedVsActual(t *testing.T) {
	b := NewBenchmark("x")
	b.CalculateScores()
	if b.PlannedVsActual != "No plan produced — execution never started." {
		t.Fatalf("no-plan message: %q", b.PlannedVsActual)
	}

	b2 := NewBenchmark("x")
	b2.RecordPlan(&Plan{Tasks: []*TaskNode{{ID: "t1"}}})
	b2.CalculateScores()
	if !strings.Contains(b2.PlannedVsActual, "none completed") {
		t.Fatalf("planned-not-completed: %q", b2.PlannedVsActual)
	}
}
