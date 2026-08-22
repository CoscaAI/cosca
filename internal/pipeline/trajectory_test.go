package pipeline

import (
	"strings"
	"testing"
	"time"
)

func TestNewTrajectory(t *testing.T) {
	tr := NewTrajectory("task-1", "cosca-backend")
	if tr.TaskID != "task-1" || tr.Agent != "cosca-backend" {
		t.Fatalf("trajectory: %+v", tr)
	}
	if !strings.HasPrefix(tr.ID, "TRJ-") {
		t.Fatalf("ID = %q", tr.ID)
	}
	if tr.Started.IsZero() {
		t.Fatal("Started must be set")
	}
	if tr.StepCount() != 0 {
		t.Fatalf("initial steps = %d", tr.StepCount())
	}
}

func TestTrajectoryRecord(t *testing.T) {
	tr := NewTrajectory("t1", "a1")
	tr.Record(TrajThought, "thinking about the problem")
	tr.Record(TrajPlan, "step 1: do x")

	if tr.StepCount() != 2 {
		t.Fatalf("steps = %d", tr.StepCount())
	}
	if tr.Events[0].Step != 1 || tr.Events[0].Type != TrajThought {
		t.Fatalf("event0: %+v", tr.Events[0])
	}
	if tr.Events[1].Step != 2 || tr.Events[1].Type != TrajPlan {
		t.Fatalf("event1: %+v", tr.Events[1])
	}
}

func TestTrajectoryRecordTool(t *testing.T) {
	tr := NewTrajectory("t1", "a1")
	tr.RecordTool("read_file", "path=/etc/hosts", "success", 42)

	if tr.StepCount() != 2 {
		t.Fatalf("steps = %d (action+observation)", tr.StepCount())
	}
	action := tr.Events[0]
	if action.Type != TrajAction || !strings.Contains(action.ToolCall, "read_file") {
		t.Fatalf("action: %+v", action)
	}
	if action.DurationMs != 42 {
		t.Fatalf("action duration = %d", action.DurationMs)
	}
	obs := tr.Events[1]
	if obs.Type != TrajObservation || obs.ToolResult != "success" {
		t.Fatalf("observation: %+v", obs)
	}
}

func TestTrajectoryCompleteAndDuration(t *testing.T) {
	tr := NewTrajectory("t1", "a1")
	tr.Record(TrajThought, "x")

	// Before completion, Duration is wall-clock (>= 0).
	if tr.Duration() < 0 {
		t.Fatal("negative duration")
	}

	tr.Complete("success")
	if tr.Outcome != "success" || tr.Finished.IsZero() {
		t.Fatalf("complete: %+v", tr)
	}
	if tr.Events[len(tr.Events)-1].Type != TrajComplete {
		t.Fatal("last event must be complete")
	}
	if tr.Duration() != tr.Finished.Sub(tr.Started) {
		t.Fatalf("duration mismatch: %v", tr.Duration())
	}
}

func TestTrajectorySummary(t *testing.T) {
	tr := NewTrajectory("t1", "cosca-testing")
	tr.Record(TrajThought, "first thought")
	tr.RecordTool("run_tests", "all", "pass", 10)
	tr.Complete("success")

	out := tr.Summary()
	for _, want := range []string{"Trajectory:", "Task:       t1", "Agent:      cosca-testing", "Steps:", "Timeline:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("summary missing %q:\n%s", want, out)
		}
	}
}

func TestTruncate(t *testing.T) {
	if truncate("short", 10) != "short" {
		t.Fatal("short string must be unchanged")
	}
	if got := truncate("1234567890abc", 10); got != "1234567890..." {
		t.Fatalf("truncate = %q", got)
	}
}

func TestRandomHex(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 5; i++ {
		h := randomHex(8)
		if len(h) != 8 {
			t.Fatalf("randomHex len = %d", len(h))
		}
		if seen[h] {
			t.Fatal("duplicate random hex (collision)")
		}
		seen[h] = true
	}
	_ = time.Now()
}
