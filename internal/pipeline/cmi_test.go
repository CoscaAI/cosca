package pipeline

import (
	"math"
	"testing"
)

func TestNewCMITrackerDefaults(t *testing.T) {
	c := NewCMITracker()

	snap := c.Snapshot()
	if snap.TasksCount != 0 {
		t.Errorf("TasksCount = %d, want 0", snap.TasksCount)
	}
	if snap.Dimensions.Learning != 0.50 {
		t.Errorf("Learning = %.2f, want 0.50", snap.Dimensions.Learning)
	}
	if snap.Dimensions.Judgment != 0.50 {
		t.Errorf("Judgment = %.2f, want 0.50", snap.Dimensions.Judgment)
	}
	if snap.Dimensions.Consistency != 1.00 {
		t.Errorf("Consistency = %.2f, want 1.00", snap.Dimensions.Consistency)
	}
}

func TestRecordTaskSingleSuccess(t *testing.T) {
	c := NewCMITracker()

	c.RecordTask("success", 5, 2, 3)

	snap := c.Snapshot()
	if snap.TasksCount != 1 {
		t.Fatalf("TasksCount = %d, want 1", snap.TasksCount)
	}

	// Learning = 5 / (1 * 3) = 1.67 -> clamped to 1.0
	if snap.Dimensions.Learning != 1.0 {
		t.Errorf("Learning = %.2f, want 1.00", snap.Dimensions.Learning)
	}
	// Judgment = 1/1 = 1.0
	if snap.Dimensions.Judgment != 1.0 {
		t.Errorf("Judgment = %.2f, want 1.00", snap.Dimensions.Judgment)
	}
	// Planning = 1*1.0 + 0*0.5 / 1 = 1.0
	if snap.Dimensions.Planning != 1.0 {
		t.Errorf("Planning = %.2f, want 1.00", snap.Dimensions.Planning)
	}
	// SelfCritique = 2 / (1 * 2) = 1.0
	if snap.Dimensions.SelfCritique != 1.0 {
		t.Errorf("SelfCritique = %.2f, want 1.00", snap.Dimensions.SelfCritique)
	}
	// Transfer = 3 / (1 * 2) = 1.5 -> clamped to 1.0
	if snap.Dimensions.Transfer != 1.0 {
		t.Errorf("Transfer = %.2f, want 1.00", snap.Dimensions.Transfer)
	}
	// Consistency = 1.0 (single task, no pattern to compare)
	if snap.Dimensions.Consistency != 1.0 {
		t.Errorf("Consistency = %.2f, want 1.00", snap.Dimensions.Consistency)
	}
}

func TestRecordTaskMixedOutcomes(t *testing.T) {
	c := NewCMITracker()

	// 3 successes, 1 partial, 1 failure = 5 tasks
	c.RecordTask("success", 2, 1, 0)
	c.RecordTask("success", 3, 1, 2)
	c.RecordTask("partial", 1, 0, 1)
	c.RecordTask("failed", 0, 0, 0)
	c.RecordTask("success", 2, 2, 1)

	snap := c.Snapshot()
	if snap.TasksCount != 5 {
		t.Fatalf("TasksCount = %d, want 5", snap.TasksCount)
	}

	// Learning = (2+3+1+0+2) / (5 * 3) = 8/15 ≈ 0.53
	wantLearning := math.Round((8.0/15.0)*100) / 100
	if snap.Dimensions.Learning != wantLearning {
		t.Errorf("Learning = %.2f, want %.2f", snap.Dimensions.Learning, wantLearning)
	}

	// Judgment = 3/5 = 0.60
	if snap.Dimensions.Judgment != 0.60 {
		t.Errorf("Judgment = %.2f, want 0.60", snap.Dimensions.Judgment)
	}

	// Planning = (3*1.0 + 1*0.5) / 5 = 3.5/5 = 0.70
	if snap.Dimensions.Planning != 0.70 {
		t.Errorf("Planning = %.2f, want 0.70", snap.Dimensions.Planning)
	}

	// SelfCritique = (1+1+0+0+2) / (5 * 2) = 4/10 = 0.40
	if snap.Dimensions.SelfCritique != 0.40 {
		t.Errorf("SelfCritique = %.2f, want 0.40", snap.Dimensions.SelfCritique)
	}

	// Transfer = (0+2+1+0+1) / (5 * 2) = 4/10 = 0.40
	if snap.Dimensions.Transfer != 0.40 {
		t.Errorf("Transfer = %.2f, want 0.40", snap.Dimensions.Transfer)
	}
}

func TestOverallScoreCalculation(t *testing.T) {
	c := NewCMITracker()

	c.RecordTask("success", 5, 3, 2)
	c.RecordTask("success", 4, 2, 1)

	snap := c.Snapshot()

	// Learning = 9/6 = 1.5 -> clamped to 1.0
	// Judgment = 2/2 = 1.0
	// Planning = 2/2 = 1.0
	// SelfCritique = 5/4 = 1.25 -> clamped to 1.0
	// Transfer = 3/4 = 0.75
	// Consistency = 1.0 (all successes, no changes)

	expected := 1.0*0.25 + 1.0*0.20 + 1.0*0.20 + 1.0*0.15 + 0.75*0.10 + 1.0*0.10
	expected = math.Round(expected*100) / 100

	if snap.Overall != expected {
		t.Errorf("Overall = %.2f, want %.2f", snap.Overall, expected)
	}
}

func TestConsistencyAllSuccesses(t *testing.T) {
	c := NewCMITracker()

	for i := 0; i < 10; i++ {
		c.RecordTask("success", 1, 0, 0)
	}

	snap := c.Snapshot()
	if snap.Dimensions.Consistency != 1.0 {
		t.Errorf("Consistency for all successes = %.2f, want 1.00", snap.Dimensions.Consistency)
	}
}

func TestConsistencyAllFailures(t *testing.T) {
	c := NewCMITracker()

	for i := 0; i < 10; i++ {
		c.RecordTask("failed", 0, 0, 0)
	}

	snap := c.Snapshot()
	if snap.Dimensions.Consistency != 1.0 {
		t.Errorf("Consistency for all failures = %.2f, want 1.00", snap.Dimensions.Consistency)
	}
}

func TestConsistencyAlternating(t *testing.T) {
	c := NewCMITracker()

	// Alternating: success, fail, success, fail = 4 tasks, 3 changes
	outcomes := []string{"success", "failed", "success", "failed"}
	for _, o := range outcomes {
		c.RecordTask(o, 1, 0, 0)
	}

	snap := c.Snapshot()

	// 3 changes / 3 max = 1.0, consistency = 0.0
	if snap.Dimensions.Consistency != 0.0 {
		t.Errorf("Consistency for alternating = %.2f, want 0.00", snap.Dimensions.Consistency)
	}
}

func TestConsistencyTwoTasksSameOutcome(t *testing.T) {
	c := NewCMITracker()

	c.RecordTask("success", 1, 0, 0)
	c.RecordTask("success", 1, 0, 0)

	snap := c.Snapshot()
	if snap.Dimensions.Consistency != 1.0 {
		t.Errorf("Consistency for two same outcomes = %.2f, want 1.00", snap.Dimensions.Consistency)
	}
}

func TestZeroTasks(t *testing.T) {
	c := NewCMITracker()

	snap := c.Snapshot()
	if snap.TasksCount != 0 {
		t.Errorf("TasksCount = %d, want 0", snap.TasksCount)
	}

	// Overall with no tasks — uses initial defaults
	expected := 0.50*0.25 + 0.50*0.20 + 0.50*0.20 + 0.50*0.15 + 0.50*0.10 + 1.00*0.10
	expected = math.Round(expected*100) / 100

	if snap.Overall != expected {
		t.Errorf("Overall with 0 tasks = %.2f, want %.2f", snap.Overall, expected)
	}
}

func TestSnapshotTimestamp(t *testing.T) {
	c := NewCMITracker()
	c.RecordTask("success", 3, 1, 0)

	snap := c.Snapshot()
	if snap.Timestamp.IsZero() {
		t.Error("Snapshot timestamp should not be zero")
	}
	if snap.TasksCount != 1 {
		t.Errorf("TasksCount = %d, want 1", snap.TasksCount)
	}
}

func TestPartialOutcomePlanning(t *testing.T) {
	c := NewCMITracker()

	// partial gives 0.5 weight in planning vs 1.0 for success
	c.RecordTask("partial", 1, 0, 0)

	snap := c.Snapshot()
	// Planning = 0*1.0 + 1*0.5 / 1 = 0.5
	if snap.Dimensions.Planning != 0.50 {
		t.Errorf("Planning for single partial = %.2f, want 0.50", snap.Dimensions.Planning)
	}
}

func TestClampAtOne(t *testing.T) {
	c := NewCMITracker()

	// Massive knowledge gained should clamp Learning at 1.0
	c.RecordTask("success", 100, 50, 50)

	snap := c.Snapshot()
	if snap.Dimensions.Learning > 1.0 {
		t.Errorf("Learning = %.2f, should be clamped at 1.00", snap.Dimensions.Learning)
	}
	if snap.Dimensions.SelfCritique > 1.0 {
		t.Errorf("SelfCritique = %.2f, should be clamped at 1.00", snap.Dimensions.SelfCritique)
	}
	if snap.Dimensions.Transfer > 1.0 {
		t.Errorf("Transfer = %.2f, should be clamped at 1.00", snap.Dimensions.Transfer)
	}
}

func TestHistoryCapWindow(t *testing.T) {
	c := NewCMITracker()

	// Insert more tasks than historyCap (32)
	for i := 0; i < 40; i++ {
		outcome := "success"
		if i%2 == 0 {
			outcome = "failed"
		}
		c.RecordTask(outcome, 1, 0, 0)
	}

	// Internal history should not exceed 32 entries.
	// Consistency should be computed on last 32 entries only.
	// With alternating pattern on 32 entries: 31 changes / 31 = 1.0 → consistency = 0.0
	snap := c.Snapshot()
	if snap.Dimensions.Consistency > 0.1 {
		t.Errorf("Consistency with alternating over 40 tasks should be near 0, got %.2f", snap.Dimensions.Consistency)
	}
}
