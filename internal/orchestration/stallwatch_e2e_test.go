package orchestration

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/stallwatch"
)

// stallThenRespondProvider simulates the REAL stall from the OpenCode incident:
// the LLM provider stops responding (waits out the attempt deadline) for a
// configurable number of attempts, then responds. This mirrors "provider
// resolved not to answer" for a while, then recovering.
//
// Note: with non-cooperative execution the watchdog may invoke the provider
// CONCURRENTLY (a stalled goroutine from a previous attempt + the next
// attempt), so the counter MUST be atomic — matching the thread-safety
// contract of real providers (http.Client based).
type stallThenRespondProvider struct {
	name         string
	model        string
	stallFor     int // how many attempts stall before responding
	attempts     atomic.Int64
	neverRespond bool // if true, stalls forever
}

func (p *stallThenRespondProvider) Chat(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
	n := p.attempts.Add(1)
	if p.neverRespond || n <= int64(p.stallFor) {
		// Provider stops responding: no error, no data — just waits out the
		// attempt deadline (the "loading forever" behaviour).
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return &chat.ChatResponse{
		Model: p.model,
		Choices: []chat.Choice{{
			Index:        0,
			Message:      chat.Message{Role: chat.RoleAssistant, Content: "recovered after stall"},
			FinishReason: chat.FinishReasonStop,
		}},
	}, nil
}

func (p *stallThenRespondProvider) ChatStream(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (chat.ChatStream, error) {
	return nil, nil
}

func (p *stallThenRespondProvider) Model() string { return p.model }
func (p *stallThenRespondProvider) Name() string  { return p.name }
func (p *stallThenRespondProvider) Close() error  { return nil }

// formatReport renders the stallwatch Report for inspection.
func formatReport(r stallwatch.Report) string {
	return fmt.Sprintf(`┌─ STALLWATCH REPORT ─────────────────────────┐
│ TotalStalls : %d
│ Retries     : %d
│ Recovered   : %d
│ Failed      : %d
│ TotalWait   : %s
│ ByOperation : %v
└────────────────────────────────────────────────┘`, r.TotalStalls, r.TotalRetries, r.Recovered, r.Failed, r.TotalWait.Round(time.Millisecond), r.ByOperation)
}

// TestStallE2E_Recovered simulates the exact OpenCode incident path:
// chatWithRetry → collector → provider that stalls 2× then recovers.
// Expected: completes WITHOUT human intervention, Report shows 2 stalls,
// >= 1 retry, 1 recovery, 0 failures.
func TestStallE2E_Recovered(t *testing.T) {
	provider := &stallThenRespondProvider{name: "stub-llm", model: "gpt-4o", stallFor: 2}

	cfg := DefaultExecutorConfig()
	cfg.MaxRetries = 3
	cfg.RetryDelay = 10 * time.Millisecond
	cfg.Timeout = 50 * time.Millisecond

	collector := stallwatch.NewCollector()
	exec := NewExecutor(provider, cfg, nil)
	exec.SetStallCollector(collector)

	start := time.Now()
	result, err := exec.Execute(context.Background(), NewPipelineContext("req-e2e-1", "do the task"))
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Data.LLMResponse != "recovered after stall" {
		t.Fatalf("response: %q", result.Data.LLMResponse)
	}

	report := collector.Report()
	t.Logf("tempo total: %s (intervenção humana: 0)", elapsed.Round(time.Millisecond))
	t.Logf("%s", formatReport(report))

	if report.TotalStalls != 2 {
		t.Errorf("TotalStalls = %d, want 2", report.TotalStalls)
	}
	if report.TotalRetries < 1 {
		t.Errorf("Retries = %d, want >= 1", report.TotalRetries)
	}
	if report.Recovered != 1 {
		t.Errorf("Recovered = %d, want 1", report.Recovered)
	}
	if report.Failed != 0 {
		t.Errorf("Failed = %d, want 0", report.Failed)
	}
	if report.TotalWait <= 0 {
		t.Errorf("TotalWait = %v, want > 0", report.TotalWait)
	}
	if elapsed > 2*time.Second {
		t.Errorf("elapsed = %s, too slow (should be ~3 timeouts + backoff)", elapsed)
	}
}

// TestStallE2E_PersistentStall simulates the worst case: the provider NEVER
// responds. Expected: the call returns an error in FINITE time (no infinite
// loading), Report shows failures, and the executor did not hang.
func TestStallE2E_PersistentStall(t *testing.T) {
	provider := &stallThenRespondProvider{name: "dead-llm", model: "gpt-4o", neverRespond: true}

	cfg := DefaultExecutorConfig()
	cfg.MaxRetries = 2
	cfg.RetryDelay = 5 * time.Millisecond
	cfg.Timeout = 30 * time.Millisecond

	collector := stallwatch.NewCollector()
	exec := NewExecutor(provider, cfg, nil)
	exec.SetStallCollector(collector)

	start := time.Now()
	_, err := exec.Execute(context.Background(), NewPipelineContext("req-e2e-2", "x"))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error when provider never responds")
	}
	// 3 attempts × 30ms + 2 backoffs (~5ms + ~10ms) ≈ 115ms — must return
	// well under 5s (the OLD behaviour was "wait forever").
	if elapsed > 5*time.Second {
		t.Fatalf("elapsed = %s, still hanging", elapsed)
	}

	report := collector.Report()
	t.Logf("tempo total (provider morto): %s — retornou em tempo finito, sem travar", elapsed.Round(time.Millisecond))
	t.Logf("%s", formatReport(report))

	if report.Failed != 1 {
		t.Errorf("Failed = %d, want 1", report.Failed)
	}
	if report.TotalStalls < 3 { // 1 initial + 2 retries
		t.Errorf("TotalStalls = %d, want >= 3", report.TotalStalls)
	}
	if report.Recovered != 0 {
		t.Errorf("Recovered = %d, want 0", report.Recovered)
	}
}

// TestStallE2E_HealthyProvider verifies the baseline: a healthy provider
// produces ZERO stall events — the watchdog never fires when nothing is wrong.
func TestStallE2E_HealthyProvider(t *testing.T) {
	provider := newMockChatProvider("healthy", "gpt-4o")

	cfg := DefaultExecutorConfig()
	cfg.Timeout = time.Second

	collector := stallwatch.NewCollector()
	exec := NewExecutor(provider, cfg, nil)
	exec.SetStallCollector(collector)

	_, err := exec.Execute(context.Background(), NewPipelineContext("req-e2e-3", "x"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	report := collector.Report()
	t.Logf("provider saudável: %s", formatReport(report))
	if report.TotalStalls != 0 || report.Recovered != 0 || report.Failed != 0 {
		t.Fatalf("healthy provider must produce no stall events: %+v", report)
	}
}
