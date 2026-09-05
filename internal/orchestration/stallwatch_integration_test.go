package orchestration

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/stallwatch"
)

// TestStallCollectorRecordsProviderStall verifies the executor records stall
// events when the provider stops responding (deadline exceeded) and recovers
// on a later attempt — the "loading forever" case, now observable.
func TestStallCollectorRecordsProviderStall(t *testing.T) {
	attempts := atomic.Int32{}
	provider := newMockChatProvider("test", "test-model")
	provider.chatFn = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
		n := attempts.Add(1)
		if n <= 2 {
			// Provider stops responding: wait for the attempt deadline.
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return &chat.ChatResponse{
			Model: "test-model",
			Choices: []chat.Choice{{
				Index: 0,
				Message: chat.Message{
					Role:    chat.RoleAssistant,
					Content: "recovered",
				},
				FinishReason: chat.FinishReasonStop,
			}},
		}, nil
	}

	cfg := DefaultExecutorConfig()
	cfg.MaxRetries = 3
	cfg.RetryDelay = time.Millisecond
	cfg.Timeout = 20 * time.Millisecond

	exec := NewExecutor(provider, cfg, nil)
	collector := stallwatch.NewCollector()
	exec.SetStallCollector(collector)

	pc := NewPipelineContext("req-stall", "do the thing")
	_, err := exec.Execute(context.Background(), pc)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	events := collector.Events()
	if len(events) == 0 {
		t.Fatal("no stall events recorded")
	}

	var stalls, retries, recovered int
	for _, e := range events {
		switch e.Action {
		case stallwatch.ActionStall:
			stalls++
		case stallwatch.ActionRetry:
			retries++
		case stallwatch.ActionRecovered:
			recovered++
		}
	}
	if stalls != 2 {
		t.Fatalf("stalls = %d, want 2 (events: %+v)", stalls, events)
	}
	if retries < 1 {
		t.Fatalf("retries = %d, want >= 1", retries)
	}
	if recovered != 1 {
		t.Fatalf("recovered = %d, want 1", recovered)
	}

	report := collector.Report()
	if report.TotalStalls != 2 || report.Recovered != 1 || report.TotalWait <= 0 {
		t.Fatalf("report: %+v", report)
	}
}

// TestStallCollectorRecordsFailure verifies a failed chat (retries exhausted)
// records a final failure event.
func TestStallCollectorRecordsFailure(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	provider.chatFn = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	cfg := DefaultExecutorConfig()
	cfg.MaxRetries = 1
	cfg.RetryDelay = time.Millisecond
	cfg.Timeout = 10 * time.Millisecond

	exec := NewExecutor(provider, cfg, nil)
	collector := stallwatch.NewCollector()
	exec.SetStallCollector(collector)

	_, err := exec.Execute(context.Background(), NewPipelineContext("req-fail", "x"))
	if err == nil {
		t.Fatal("expected error when provider never responds")
	}

	report := collector.Report()
	if report.Failed != 1 {
		t.Fatalf("failed = %d, want 1 (%+v)", report.Failed, report)
	}
	if report.TotalStalls < 1 {
		t.Fatalf("stalls = %d, want >= 1", report.TotalStalls)
	}
}

// TestStallCollectorNilIsNoop verifies that without a collector the executor
// behaves exactly as before (no events, no panics).
func TestStallCollectorNilIsNoop(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	exec := NewExecutor(provider, DefaultExecutorConfig(), nil)
	// No SetStallCollector call → nil collector.
	_, err := exec.Execute(context.Background(), NewPipelineContext("req", "x"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
}
