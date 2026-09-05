package pipeline

import (
	"context"
	"fmt"
	"time"
)

// MockRunner simulates pipeline execution without LLM calls.
// Used for testing the full durable execution infrastructure.
type MockRunner struct {
	// BuildResult simulates go build outcome.
	BuildSuccess bool
	// TestResult simulates go test outcome.
	TestSuccess bool
	// TestPassed simulates number of passed tests.
	TestPassed int
	// TestFailed simulates number of failed tests.
	TestFailed int
	// Delay simulates execution time per step.
	Delay time.Duration
}

// NewMockRunner creates a MockRunner with all-success defaults.
func NewMockRunner() *MockRunner {
	return &MockRunner{
		BuildSuccess: true,
		TestSuccess:  true,
		TestPassed:   10,
		TestFailed:   0,
		Delay:        50 * time.Millisecond,
	}
}

// NewFailingMockRunner creates a MockRunner that simulates build failure.
func NewFailingMockRunner() *MockRunner {
	return &MockRunner{
		BuildSuccess: false,
		TestSuccess:  false,
		TestPassed:   0,
		TestFailed:   3,
		Delay:        50 * time.Millisecond,
	}
}

func (m *MockRunner) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(m.Delay):
	}

	result := &RunResult{
		Response:  fmt.Sprintf("[SIMULATED] Task: %s | Agent: %s", req.Prompt, req.Agent),
		Agent:     req.Agent,
		TraceID:   fmt.Sprintf("mock-%d", time.Now().UnixNano()),
		TurnCount: 1,
		TokenUsage: TokenUsage{
			Input:  len(req.Prompt) / 4,
			Output: 50,
		},
	}

	if req.Options.EnableBuild {
		if m.BuildSuccess {
			result.BuildResult = &BuildResult{
				Success:    true,
				Output:     "go build: OK",
				DurationMs: m.Delay.Milliseconds(),
			}
		} else {
			result.BuildResult = &BuildResult{
				Success:    false,
				Output:     "go build: compilation error in mock.go:10",
				DurationMs: m.Delay.Milliseconds(),
			}
		}
	}

	if req.Options.EnableTest && (result.BuildResult == nil || result.BuildResult.Success) {
		result.TestResult = &TestResult{
			Success:    m.TestSuccess,
			Passed:     m.TestPassed,
			Failed:     m.TestFailed,
			Output:     fmt.Sprintf("go test: %d passed, %d failed", m.TestPassed, m.TestFailed),
			DurationMs: m.Delay.Milliseconds(),
		}
	}

	return result, nil
}

func (m *MockRunner) RunStream(ctx context.Context, req RunRequest) (<-chan RunEvent, error) {
	ch := make(chan RunEvent, 10)

	go func() {
		defer close(ch)
		ch <- RunEvent{Type: EventContent, Data: fmt.Sprintf("[SIMULATED] Starting: %s", req.Prompt)}
		time.Sleep(m.Delay / 2)
		ch <- RunEvent{Type: EventBuildStart}
		time.Sleep(m.Delay / 2)
		if m.BuildSuccess {
			ch <- RunEvent{Type: EventBuildEnd, Data: "OK"}
		} else {
			ch <- RunEvent{Type: EventBuildEnd, Data: "FAILED"}
		}
		ch <- RunEvent{Type: EventDone}
	}()

	return ch, nil
}
