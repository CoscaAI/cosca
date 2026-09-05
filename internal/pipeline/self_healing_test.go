package pipeline

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/diagnostics"
)

// alwaysFailRunner never succeeds — used to exercise self-healing failure paths.
type alwaysFailRunner struct{}

func (r *alwaysFailRunner) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	return nil, errSimulated
}

func (r *alwaysFailRunner) RunStream(ctx context.Context, req RunRequest) (<-chan RunEvent, error) {
	ch := make(chan RunEvent, 1)
	ch <- RunEvent{Type: EventDone}
	close(ch)
	return ch, nil
}

var errSimulated = &pipelineError{"simulated failure"}

type pipelineError struct{ msg string }

func (e *pipelineError) Error() string { return e.msg }

func TestNewSelfHealerDefaults(t *testing.T) {
	s := NewSelfHealer(nil, nil, &alwaysFailRunner{}, nil)
	if s.classifier == nil || s.recoveryLoop == nil {
		t.Fatal("defaults must construct classifier and recovery loop")
	}
	if s.maxAttempts != 5 {
		t.Fatalf("maxAttempts = %d", s.maxAttempts)
	}
}

func TestSelfHealerCanAutoFix(t *testing.T) {
	s := NewSelfHealer(nil, nil, &alwaysFailRunner{}, nil)
	if s.CanAutoFix(nil) {
		t.Fatal("nil error must not be auto-fixable")
	}
	// A real Go compile error is auto-fixable.
	if !s.CanAutoFix(&pipelineError{"main.go:12:5: undefined: foo"}) {
		t.Fatal("compilation error should be auto-fixable")
	}
	// Unrecognized error is not.
	if s.CanAutoFix(&pipelineError{"the flux capacitor is out of alignment"}) {
		t.Fatal("unknown error should not be auto-fixable")
	}
}

func TestSelfHealerBuildDiagnosis(t *testing.T) {
	s := NewSelfHealer(nil, nil, &alwaysFailRunner{}, nil)
	classified := diagnostics.ClassifiedError{
		Category:   diagnostics.CatCompilation,
		Language:   "go",
		Confidence: 0.95,
		File:       "main.go",
		Line:       12,
		Symbol:     "foo",
		Suggestion: "check the type",
		RawMessage: "cannot compile",
	}
	diag := s.buildDiagnosis(classified)
	for _, want := range []string{"compilation", "go", "0.95", "main.go", "12", "foo", "check the type"} {
		if !strings.Contains(diag, want) {
			t.Fatalf("diagnosis missing %q: %s", want, diag)
		}
	}
}

func TestSelfHealerBuildFixPrompt(t *testing.T) {
	s := NewSelfHealer(nil, nil, &alwaysFailRunner{}, nil)
	task := &TaskNode{ID: "t1", Description: "build the module"}
	prompt := s.buildFixPrompt(task, diagnostics.ClassifiedError{
		Category:   diagnostics.CatCompilation,
		File:       "main.go",
		Line:       5,
		RawMessage: "syntax error",
	})
	if !strings.Contains(prompt, "Task t1 (build the module)") || !strings.Contains(prompt, "Fix compilation error") {
		t.Fatalf("prompt: %s", prompt)
	}
}

func TestSelfHealerHealAppendsHistory(t *testing.T) {
	classifier := diagnostics.NewClassifier()
	recovery := NewRecoveryLoop(classifier)
	recovery.RetryDelay = time.Millisecond // keep the test fast
	runner := &alwaysFailRunner{}
	s := NewSelfHealer(classifier, recovery, runner, nil)

	attempt, err := s.Heal(context.Background(), &TaskNode{ID: "t", Description: "x"}, errSimulated)
	// Heal returns (attempt, nil) even when recovery fails gracefully — the
	// attempt records Success=false instead of surfacing a hard error.
	if err != nil {
		t.Fatalf("Heal must not hard-error on graceful recovery failure: %v", err)
	}
	if attempt == nil || attempt.Success {
		t.Fatalf("attempt must record failure: %+v", attempt)
	}
	if attempt.Error != "simulated failure" || attempt.Diagnosis == "" {
		t.Fatalf("attempt fields: %+v", attempt)
	}

	report := s.Report()
	if report.TotalAttempts != 1 || report.Failed != 1 || report.SuccessRate != 0 {
		t.Fatalf("report: %+v", report)
	}
	if len(report.Attempts) != 1 {
		t.Fatalf("report attempts = %d", len(report.Attempts))
	}
}

func TestSelfHealerHealWithRetryExhausts(t *testing.T) {
	classifier := diagnostics.NewClassifier()
	recovery := NewRecoveryLoop(classifier)
	recovery.RetryDelay = time.Millisecond
	s := NewSelfHealer(classifier, recovery, &alwaysFailRunner{}, nil)
	s.maxAttempts = 2

	err := s.HealWithRetry(context.Background(), &TaskNode{ID: "t", Description: "x"}, errSimulated)
	if err == nil {
		t.Fatal("HealWithRetry with always-failing runner must error")
	}
	if !strings.Contains(err.Error(), "self-healing failed") {
		t.Fatalf("error = %v", err)
	}
	if got := s.Report().TotalAttempts; got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
}

func TestSelfHealerReportEmpty(t *testing.T) {
	s := NewSelfHealer(nil, nil, &alwaysFailRunner{}, nil)
	report := s.Report()
	if report.TotalAttempts != 0 || report.SuccessRate != 0 {
		t.Fatalf("empty report: %+v", report)
	}
}
