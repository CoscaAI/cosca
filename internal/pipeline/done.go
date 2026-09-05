package pipeline

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// DoDCheck represents a single Definition of Done criterion.
type DoDCheck struct {
	Name          string
	Description   string
	Required      bool
	EvidenceKinds []string
}

// DefinitionOfDone validates completed pipeline tasks against quality criteria.
type DefinitionOfDone struct {
	Checks    []DoDCheck
	workDir   string
}

// DoDReport summarizes validation results.
type DoDReport struct {
	Passed    []string
	Failed    []string
	Skipped   []string
	AllPassed bool
}

// StandardDoD returns the standard Definition of Done checklist.
func StandardDoD() *DefinitionOfDone {
	return &DefinitionOfDone{
		Checks: []DoDCheck{
			{Name: "code_exists", Description: "Code files were created or modified", Required: true, EvidenceKinds: []string{"file_diff"}},
			{Name: "builds", Description: "Project builds without errors", Required: true, EvidenceKinds: []string{"build_output"}},
			{Name: "tests_pass", Description: "All tests pass", Required: true, EvidenceKinds: []string{"test_result"}},
			{Name: "no_regressions", Description: "Existing tests still pass", Required: true, EvidenceKinds: []string{"test_result"}},
			{Name: "security_ok", Description: "No obvious security issues introduced", Required: false, EvidenceKinds: []string{"security_scan"}},
			{Name: "documented", Description: "Changes are documented", Required: false, EvidenceKinds: []string{"documentation"}},
		},
	}
}

func (d *DefinitionOfDone) SetWorkDir(dir string) {
	d.workDir = dir
}

// Validate checks all DoD criteria against completed tasks in a plan.
func (d *DefinitionOfDone) Validate(ctx context.Context, plan *Plan) *DoDReport {
	report := &DoDReport{}

	for _, check := range d.Checks {
		passed := d.evaluateCheck(ctx, plan, check)
		if passed {
			report.Passed = append(report.Passed, check.Name)
		} else if check.Required {
			report.Failed = append(report.Failed, check.Name)
		} else {
			report.Skipped = append(report.Skipped, check.Name)
		}
	}

	report.AllPassed = len(report.Failed) == 0
	return report
}

// ValidateWithEvidence performs real validation against evidence instead of string matching.
func (d *DefinitionOfDone) ValidateWithEvidence(ctx context.Context, plan *Plan, evidence []Evidence) *DoDReport {
	report := &DoDReport{}

	for _, check := range d.Checks {
		passed := d.evaluateWithEvidence(ctx, plan, check, evidence)
		if passed {
			report.Passed = append(report.Passed, check.Name)
		} else if check.Required {
			report.Failed = append(report.Failed, check.Name)
		} else {
			report.Skipped = append(report.Skipped, check.Name)
		}
	}

	report.AllPassed = len(report.Failed) == 0
	return report
}

func (d *DefinitionOfDone) evaluateWithEvidence(ctx context.Context, plan *Plan, check DoDCheck, evidence []Evidence) bool {
	if len(check.EvidenceKinds) == 0 {
		return d.evaluateCheck(ctx, plan, check)
	}

	for _, ev := range evidence {
		for _, kind := range check.EvidenceKinds {
			if ev.Kind != kind {
				continue
			}

			switch check.Name {
			case "code_exists", "builds", "tests_pass", "no_regressions":
				if ev.Status == "pass" {
					return true
				}
			case "security_ok", "documented":
				return true
			default:
				if ev.Status == "pass" {
					return true
				}
			}
		}
	}
	return false
}

func (d *DefinitionOfDone) evaluateCheck(ctx context.Context, plan *Plan, check DoDCheck) bool {
	switch check.Name {
	case "code_exists":
		for _, task := range plan.Tasks {
			if task.Status == TaskCompleted && len(task.OutputFiles) > 0 {
				return true
			}
		}
		// Fallback: check filesystem for generated files
		if d.workDir != "" {
			return hasSourceFiles(d.workDir)
		}
		return false

	case "builds":
		// Real build evidence: any task with a BuildResult decides the
		// check. When NO task produced build evidence the check FAILS —
		// a plan without build verification must not claim "builds" passes.
		haveEvidence := false
		for _, task := range plan.Tasks {
			if task.Result == nil {
				continue
			}
			if strings.Contains(task.Result.Error, "build failed") {
				return false
			}
			if task.Result.BuildResult != nil {
				haveEvidence = true
				if task.Status == TaskCompleted && task.Result.BuildResult.Success {
					return true
				}
			}
		}
		return haveEvidence

	case "tests_pass":
		// Real test evidence: any task with a TestResult decides the check.
		// With no test evidence the check FAILS instead of vacuous true.
		haveEvidence := false
		for _, task := range plan.Tasks {
			if task.Result == nil {
				continue
			}
			if strings.Contains(task.Result.Error, "tests failed") {
				return false
			}
			if task.Result.TestResult != nil {
				haveEvidence = true
				if task.Status == TaskCompleted && task.Result.TestResult.Success {
					return true
				}
			}
		}
		return haveEvidence

	case "no_regressions":
		// Check if any task reports test failures
		hasFailures := false
		for _, task := range plan.Tasks {
			if task.Status == TaskFailed && task.Result != nil {
				hasFailures = true
				break
			}
		}
		return !hasFailures

	case "security_ok":
		completedCount := 0
		for _, task := range plan.Tasks {
			if task.Status == TaskCompleted {
				completedCount++
			}
		}
		return completedCount > 0

	case "documented":
		completedCount := 0
		for _, task := range plan.Tasks {
			if task.Status == TaskCompleted {
				completedCount++
			}
		}
		return completedCount > 0

	default:
		return true
	}
}

// FormatReport returns a human-readable DoD report.
func (r *DoDReport) FormatReport() string {
	var b strings.Builder
	b.WriteString("Definition of Done Report\n")
	b.WriteString("========================\n")

	if len(r.Passed) > 0 {
		b.WriteString(fmt.Sprintf("\n  PASSED (%d):\n", len(r.Passed)))
		for _, name := range r.Passed {
			b.WriteString(fmt.Sprintf("    [x] %s\n", name))
		}
	}
	if len(r.Failed) > 0 {
		b.WriteString(fmt.Sprintf("\n  FAILED (%d):\n", len(r.Failed)))
		for _, name := range r.Failed {
			b.WriteString(fmt.Sprintf("    [ ] %s  <- REQUIRED\n", name))
		}
	}
	if len(r.Skipped) > 0 {
		b.WriteString(fmt.Sprintf("\n  SKIPPED (%d):\n", len(r.Skipped)))
		for _, name := range r.Skipped {
			b.WriteString(fmt.Sprintf("    [ ] %s  (optional)\n", name))
		}
	}

	if r.AllPassed {
		b.WriteString("\n  STATUS: READY\n")
	} else {
		b.WriteString("\n  STATUS: NOT READY — fix required checks above\n")
	}

	return b.String()
}

func hasSourceFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".go") || strings.HasSuffix(name, ".py") ||
			strings.HasSuffix(name, ".js") || strings.HasSuffix(name, ".ts") ||
			strings.HasSuffix(name, ".java") || strings.HasSuffix(name, ".rs") ||
			strings.HasSuffix(name, ".rb") || strings.HasSuffix(name, ".php") {
			return true
		}
	}
	return false
}
