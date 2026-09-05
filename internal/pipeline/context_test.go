package pipeline

import (
	"strings"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/diagnostics"
)

func TestNewSessionContext(t *testing.T) {
	plan := &Plan{ID: "p1", Intent: "build"}
	sc := NewSessionContext(plan)
	if sc.SessionID != "p1" || sc.Plan != plan {
		t.Fatalf("context: %+v", sc)
	}
	if sc.Metrics.StartedAt.IsZero() {
		t.Fatal("StartedAt must be set")
	}
	if sc.Handoffs == nil || sc.Knowledge == nil || sc.Errors == nil {
		t.Fatal("slices must be initialized (non-nil)")
	}
}

func TestSessionContextRecordHandoff(t *testing.T) {
	sc := NewSessionContext(&Plan{ID: "p"})
	sc.RecordHandoff(&HandoffArtifact{ID: "HO-1", FromAgent: "a", ToAgent: "b"})
	sc.RecordHandoff(&HandoffArtifact{ID: "HO-2"})
	if len(sc.Handoffs) != 2 {
		t.Fatalf("handoffs = %d", len(sc.Handoffs))
	}
	if sc.Handoffs[0].FromAgent != "a" {
		t.Fatalf("handoff[0]: %+v", sc.Handoffs[0])
	}
}

func TestSessionContextErrorsAndMetrics(t *testing.T) {
	sc := NewSessionContext(&Plan{ID: "p"})
	sc.RecordError(diagnostics.ClassifiedError{Category: "compilation", RawMessage: "x"})
	if len(sc.Errors) != 1 {
		t.Fatalf("errors = %d", len(sc.Errors))
	}

	sc.IncrementTasksCompleted()
	sc.IncrementTasksCompleted()
	sc.IncrementTasksFailed()
	sc.AddTokens(100, 50)
	sc.AddTokens(10, 5)

	if sc.Metrics.TasksCompleted != 2 || sc.Metrics.TasksFailed != 1 {
		t.Fatalf("metrics: %+v", sc.Metrics)
	}
	if sc.Metrics.TokensUsed.Input != 110 || sc.Metrics.TokensUsed.Output != 55 {
		t.Fatalf("tokens: %+v", sc.Metrics.TokensUsed)
	}
}

func TestSessionContextSummary(t *testing.T) {
	sc := NewSessionContext(&Plan{ID: "p9"})
	sc.IncrementTasksCompleted()
	sc.Knowledge = append(sc.Knowledge, "patterns/retry")
	out := sc.Summary()

	for _, want := range []string{"Session: p9", "1 completed", "Knowledge used: patterns/retry", "Tokens: 0 in / 0 out"} {
		if !strings.Contains(out, want) {
			t.Fatalf("summary missing %q:\n%s", want, out)
		}
	}
	_ = time.Now()
}

func TestPlanProgress(t *testing.T) {
	plan := &Plan{Tasks: []*TaskNode{
		{ID: "a", Status: TaskCompleted},
		{ID: "b", Status: TaskPending},
		{ID: "c", Status: TaskCompleted},
	}}
	done, total := plan.Progress()
	if done != 2 || total != 3 {
		t.Fatalf("progress = %d/%d, want 2/3", done, total)
	}
}
