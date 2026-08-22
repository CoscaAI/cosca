package pipeline

import (
	"context"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/diagnostics"
)

type mockRunner struct {
	shouldSucceed bool
	response      string
	callCount     int
}

func (m *mockRunner) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	m.callCount++
	success := m.shouldSucceed
	return &RunResult{
		Response: m.response,
		BuildResult: &BuildResult{
			Success: success,
			Output:  m.response,
		},
	}, nil
}

func (m *mockRunner) RunStream(ctx context.Context, req RunRequest) (<-chan RunEvent, error) {
	ch := make(chan RunEvent, 1)
	ch <- RunEvent{Type: EventDone}
	close(ch)
	return ch, nil
}

func TestRecoveryCompilationErrorFixPrompt(t *testing.T) {
	classifier := diagnostics.NewClassifier()
	loop := NewRecoveryLoop(classifier)
	loop.MaxRetries = 1

	runner := &mockRunner{shouldSucceed: true, response: "fixed"}
	task := &TaskNode{ID: "fix-task", Agent: "cosca-backend"}

	failure := "pkg/handler.go:42:3: undefined: UserModel"

	result, err := loop.Recover(context.Background(), task, failure, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ErrorCategory != diagnostics.CatCompilation {
		t.Errorf("category = %s, want compilation", result.ErrorCategory)
	}
	if !strings.Contains(result.FixDescription, "Fix compilation error") {
		t.Errorf("fix description = %q, should mention compilation", result.FixDescription)
	}
	if !strings.Contains(result.FixDescription, "pkg/handler.go") {
		t.Errorf("fix description = %q, should contain file path", result.FixDescription)
	}
	if !result.RetrySuccessful {
		t.Error("retry should be successful")
	}
}

func TestRecoveryDependencyErrorTriggersKnowledgeGap(t *testing.T) {
	classifier := diagnostics.NewClassifier()
	loop := NewRecoveryLoop(classifier)
	loop.MaxRetries = 1

	runner := &mockRunner{shouldSucceed: true, response: "installed"}
	task := &TaskNode{ID: "dep-task", Agent: "cosca-backend"}

	failure := `cannot find package "github.com/example/missing-lib"`

	result, err := loop.Recover(context.Background(), task, failure, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ErrorCategory != diagnostics.CatDependency {
		t.Errorf("category = %s, want dependency", result.ErrorCategory)
	}
	if !result.KnowledgeGap {
		t.Error("KnowledgeGap should be true for dependency errors")
	}
	if !strings.Contains(result.FixDescription, "Install missing dependency") {
		t.Errorf("fix description = %q, should mention dependency", result.FixDescription)
	}
}

func TestRecoveryMaxRetriesLimitsAttempts(t *testing.T) {
	classifier := diagnostics.NewClassifier()
	loop := NewRecoveryLoop(classifier)
	loop.MaxRetries = 3
	loop.RetryDelay = 0

	runner := &mockRunner{shouldSucceed: false, response: "pkg/app.go:10:5: syntax error: unexpected newline"}
	task := &TaskNode{ID: "retry-task", Agent: "cosca-backend"}

	failure := "pkg/app.go:10:5: syntax error: unexpected newline"

	result, err := loop.Recover(context.Background(), task, failure, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.RetrySuccessful {
		t.Error("retry should not be successful when runner always fails")
	}
	if result.AttemptsUsed != 3 {
		t.Errorf("attempts used = %d, want 3", result.AttemptsUsed)
	}
	if runner.callCount != 3 {
		t.Errorf("runner call count = %d, want 3", runner.callCount)
	}
}
