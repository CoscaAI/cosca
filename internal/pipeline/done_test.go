package pipeline

import (
	"context"
	"testing"
)

func TestDoD_BuildsWithoutEvidenceFails(t *testing.T) {
	// A plan whose completed tasks carry NO build evidence must FAIL the
	// "builds" check (previously vacuous `len(plan.Tasks) > 0`).
	dod := StandardDoD()
	plan := &Plan{
		Tasks: []*TaskNode{
			{ID: "a", Status: TaskCompleted, Result: &TaskResult{Success: true, Output: "ok"}},
		},
	}
	report := dod.Validate(context.Background(), plan)
	for _, failed := range report.Failed {
		if failed == "builds" {
			return
		}
	}
	t.Fatalf("expected 'builds' to fail without evidence; passed=%v failed=%v", report.Passed, report.Failed)
}

func TestDoD_TestsWithoutEvidenceFails(t *testing.T) {
	dod := StandardDoD()
	plan := &Plan{
		Tasks: []*TaskNode{
			{ID: "a", Status: TaskCompleted, Result: &TaskResult{Success: true}},
		},
	}
	report := dod.Validate(context.Background(), plan)
	for _, failed := range report.Failed {
		if failed == "tests_pass" {
			return
		}
	}
	t.Fatalf("expected 'tests_pass' to fail without evidence; passed=%v failed=%v", report.Passed, report.Failed)
}

func TestDoD_BuildAndTestEvidencePasses(t *testing.T) {
	dod := StandardDoD()
	dod.SetWorkDir(t.TempDir())
	plan := &Plan{
		Tasks: []*TaskNode{
			{
				ID:          "a",
				Status:      TaskCompleted,
				OutputFiles: []string{"main.go"},
				Result: &TaskResult{
					Success:     true,
					BuildResult: &BuildResult{Success: true},
					TestResult:  &TestResult{Success: true},
				},
			},
		},
	}
	report := dod.Validate(context.Background(), plan)
	if !report.AllPassed {
		t.Fatalf("expected DoD to pass with real evidence; failed=%v", report.Failed)
	}
}

func TestDoD_FailedBuildFailsBuildsCheck(t *testing.T) {
	dod := StandardDoD()
	plan := &Plan{
		Tasks: []*TaskNode{
			{
				ID: "a", Status: TaskFailed,
				Result: &TaskResult{
					Success:     false,
					Error:       "build failed: cannot compile",
					BuildResult: &BuildResult{Success: false},
				},
			},
		},
	}
	report := dod.Validate(context.Background(), plan)
	for _, failed := range report.Failed {
		if failed == "builds" {
			return
		}
	}
	t.Fatalf("expected 'builds' to fail on failed build; passed=%v failed=%v", report.Passed, report.Failed)
}
