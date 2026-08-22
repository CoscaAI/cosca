//
// Tests for the cognitive budget (internal/engine/budget.go) and its opt-in
// integration in the agent loop (EngineConfig.Budget).
//
// Covers:
//   - DefaultCognitiveBudget values (Tokens: 8k, Tempo: 20s, Custo: $0.05)
//   - Record accumulates tokens/duration/cost and increments AICalls
//   - Exceeded triggers per dimension (tokens/tempo/custo) and not on boundary
//   - CanCall estimate logic (fresh tracker + average-per-call)
//   - Summary string (0 calls → "resolvido sem IA")
//   - Engine integration: budget nil → loop unchanged (no regression); budget
//     set → CanCall=false returns clear error WITHOUT calling the provider
//     (fake provider counting calls); budget set within limits → runs and
//     attaches the Budget snapshot to the EngineResult
//

package engine

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/chat/executor"
)

// =============================================================================
// Defaults
// =============================================================================

func TestDefaultCognitiveBudget(t *testing.T) {
	b := DefaultCognitiveBudget()
	if b.MaxTokens != 8000 {
		t.Errorf("MaxTokens = %d, want 8000", b.MaxTokens)
	}
	if b.MaxDuration != 20*time.Second {
		t.Errorf("MaxDuration = %s, want 20s", b.MaxDuration)
	}
	if b.MaxCost != 0.05 {
		t.Errorf("MaxCost = %f, want 0.05", b.MaxCost)
	}
}

// =============================================================================
// Record / Spent accumulation
// =============================================================================

func TestBudgetTracker_RecordAccumulates(t *testing.T) {
	tr := NewBudgetTracker(DefaultCognitiveBudget())

	tr.Record(100, 2*time.Second, 0.01)
	tr.Record(50, 1*time.Second, 0)
	tr.Record(0, 0, 0)

	spent := tr.Spent()
	if spent.Tokens != 150 {
		t.Errorf("Tokens = %d, want 150", spent.Tokens)
	}
	if spent.Duration != 3*time.Second {
		t.Errorf("Duration = %s, want 3s", spent.Duration)
	}
	if spent.Cost != 0.01 {
		t.Errorf("Cost = %f, want 0.01", spent.Cost)
	}
	// AICalls increments only when tokens > 0.
	if spent.AICalls != 2 {
		t.Errorf("AICalls = %d, want 2", spent.AICalls)
	}
}

func TestBudgetTracker_RecordZeroTokensKeepsAICalls(t *testing.T) {
	tr := NewBudgetTracker(DefaultCognitiveBudget())
	tr.Record(0, 5*time.Second, 0)
	if tr.Spent().AICalls != 0 {
		t.Errorf("AICalls = %d, want 0 (no tokens consumed)", tr.Spent().AICalls)
	}
}

// =============================================================================
// Exceeded
// =============================================================================

func TestBudgetTracker_ExceededPerDimension(t *testing.T) {
	// Tokens.
	tr := NewBudgetTracker(CognitiveBudget{MaxTokens: 100, MaxDuration: time.Second, MaxCost: 0.05})
	tr.Record(101, 0, 0)
	if !tr.Exceeded() {
		t.Error("Exceeded should be true when tokens over budget")
	}

	// Tempo (duration) — tokens 0 keeps AICalls 0 but duration still triggers.
	tr = NewBudgetTracker(CognitiveBudget{MaxTokens: 100, MaxDuration: time.Second, MaxCost: 0.05})
	tr.Record(0, 2*time.Second, 0)
	if !tr.Exceeded() {
		t.Error("Exceeded should be true when duration over budget")
	}

	// Custo.
	tr = NewBudgetTracker(CognitiveBudget{MaxTokens: 100, MaxDuration: time.Second, MaxCost: 0.05})
	tr.Record(0, 0, 0.06)
	if !tr.Exceeded() {
		t.Error("Exceeded should be true when cost over budget")
	}
}

func TestBudgetTracker_ExceededBoundaryNotExceeded(t *testing.T) {
	tr := NewBudgetTracker(CognitiveBudget{MaxTokens: 100, MaxDuration: time.Second, MaxCost: 0.05})
	tr.Record(100, time.Second, 0.05)
	if tr.Exceeded() {
		t.Error("Exceeded should be false at the exact boundary (equal is allowed)")
	}
}

// =============================================================================
// CanCall
// =============================================================================

func TestBudgetTracker_CanCallFreshAlwaysTrue(t *testing.T) {
	tr := NewBudgetTracker(CognitiveBudget{MaxTokens: 100, MaxDuration: time.Second, MaxCost: 0.05})
	if !tr.CanCall() {
		t.Error("CanCall should be true for a fresh tracker")
	}
}

func TestBudgetTracker_CanCallAverageEstimate(t *testing.T) {
	budget := CognitiveBudget{MaxTokens: 100, MaxDuration: 10 * time.Second, MaxCost: 0.05}
	tr := NewBudgetTracker(budget)

	// One call of 50 tokens → average 50; 50+50 <= 100 → still fits.
	tr.Record(50, 2*time.Second, 0)
	if !tr.CanCall() {
		t.Error("CanCall should be true when spent(50) + avg(50) <= 100")
	}

	// One call of 60 tokens → average 60; 60+60 > 100 → would exceed.
	tr = NewBudgetTracker(budget)
	tr.Record(60, 2*time.Second, 0)
	if tr.CanCall() {
		t.Error("CanCall should be false when spent(60) + avg(60) > 100")
	}

	// Duration dimension: one call of 7s → average 7s; 7+7 > 10 → would exceed.
	tr = NewBudgetTracker(budget)
	tr.Record(10, 7*time.Second, 0)
	if tr.CanCall() {
		t.Error("CanCall should be false when spent(7s) + avg(7s) > 10s")
	}
}

// =============================================================================
// Summary
// =============================================================================

func TestBudgetTracker_SummaryNoAI(t *testing.T) {
	tr := NewBudgetTracker(DefaultCognitiveBudget())
	s := tr.Summary()
	for _, want := range []string{
		"AI calls: 0",
		"Tokens: 0",
		"Custo: $0",
		"✓ resolvido sem IA (AI calls: 0, Cost: $0)",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("Summary missing %q: %q", want, s)
		}
	}
}

func TestBudgetTracker_SummaryWithAICalls(t *testing.T) {
	tr := NewBudgetTracker(DefaultCognitiveBudget())
	tr.Record(500, 3*time.Second, 0)
	s := tr.Summary()
	for _, want := range []string{
		"AI calls: 1",
		"Tokens: 500",
		"Tempo: 3s",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("Summary missing %q: %q", want, s)
		}
	}
	if strings.Contains(s, "resolvido sem IA") {
		t.Errorf("Summary should not claim 'resolvido sem IA' when AI was called: %q", s)
	}
}

// =============================================================================
// Engine integration — budget nil (no regression)
// =============================================================================

func TestRun_BudgetNil_Unchanged(t *testing.T) {
	reg, cb, rtr, prov, exec := setupEngineTest(t)
	e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)

	callCount := 0
	prov.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
		callCount++
		return newMockChatResponse("mock response"), nil
	}

	result, err := e.Run(context.Background(), "Hello", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "mock response" {
		t.Errorf("Content = %q, want %q (loop behavior must be unchanged)", result.Content, "mock response")
	}
	if result.Budget != nil {
		t.Error("Budget should be nil when EngineConfig.Budget is nil")
	}
	if callCount != 1 {
		t.Errorf("provider calls = %d, want 1", callCount)
	}
}

// =============================================================================
// Engine integration — budget set
// =============================================================================

// newMockChatResponse is a helper for a plain assistant response.
func newMockChatResponse(content string) *chat.ChatResponse {
	return &chat.ChatResponse{
		Choices: []chat.Choice{{Message: chat.Message{Role: chat.RoleAssistant, Content: content}}},
		Usage:   chat.Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
	}
}

// TestRun_BudgetExceeded_BlocksSecondCall verifies that once the budget is
// exceeded, CanCall returns false and the engine returns the clear "budget
// cognitivo estourado" error WITHOUT calling the provider again.
func TestRun_BudgetExceeded_BlocksSecondCall(t *testing.T) {
	reg, cb, rtr, prov, exec := setupEngineTest(t)
	cfg := defaultEngineConfig()
	// First call (30 tokens from the mock) already blows a 10-token budget.
	cfg.Budget = &CognitiveBudget{MaxTokens: 10, MaxDuration: time.Minute, MaxCost: 1.0}
	e := NewAgentEngine(reg, cb, rtr, prov, exec, cfg, nil, nil, nil)

	callCount := 0
	prov.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
		callCount++
		if callCount == 1 {
			// First call demands a tool, forcing the loop into a second turn.
			return &chat.ChatResponse{
				Choices: []chat.Choice{{
					Message: chat.Message{
						Content: "Using tool",
						ToolCalls: []chat.ToolCall{{
							ID:   "call_1",
							Type: "function",
							Function: chat.FunctionCall{
								Name:      "read_file",
								Arguments: `{"path":"test.txt"}`,
							},
						}},
					},
				}},
				Usage: chat.Usage{TotalTokens: 30},
			}, nil
		}
		return newMockChatResponse("should never be reached"), nil
	}

	exec.executeFunc = func(ctx context.Context, tc executor.ToolCall) (*executor.ToolResult, error) {
		return &executor.ToolResult{Status: executor.StatusSuccess, Output: "data"}, nil
	}

	result, err := e.Run(context.Background(), "do something", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The second LLM call was blocked: the provider ran only once.
	if callCount != 1 {
		t.Errorf("provider calls = %d, want 1 (second call must be blocked)", callCount)
	}
	if !strings.Contains(result.Error, "budget cognitivo estourado") {
		t.Errorf("expected clear budget error, got: %q", result.Error)
	}
	if result.Budget == nil {
		t.Fatal("Budget snapshot should be attached when a budget is configured")
	}
	if result.Budget.AICalls != 1 {
		t.Errorf("Budget.AICalls = %d, want 1", result.Budget.AICalls)
	}
	if result.Budget.Tokens != 30 {
		t.Errorf("Budget.Tokens = %d, want 30", result.Budget.Tokens)
	}
}

// TestRun_BudgetWithinLimits_RunsFully verifies that a budget that fits allows
// the loop to complete and attaches the accumulated Budget snapshot.
func TestRun_BudgetWithinLimits_RunsFully(t *testing.T) {
	reg, cb, rtr, prov, exec := setupEngineTest(t)
	cfg := defaultEngineConfig()
	cfg.Budget = &CognitiveBudget{MaxTokens: 10000, MaxDuration: time.Minute, MaxCost: 1.0}
	e := NewAgentEngine(reg, cb, rtr, prov, exec, cfg, nil, nil, nil)

	callCount := 0
	prov.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
		callCount++
		if callCount == 1 {
			return &chat.ChatResponse{
				Choices: []chat.Choice{{
					Message: chat.Message{
						Content: "Using tool",
						ToolCalls: []chat.ToolCall{{
							ID:   "call_1",
							Type: "function",
							Function: chat.FunctionCall{
								Name:      "read_file",
								Arguments: `{"path":"test.txt"}`,
							},
						}},
					},
				}},
				Usage: chat.Usage{TotalTokens: 30},
			}, nil
		}
		return newMockChatResponse("final answer"), nil
	}

	exec.executeFunc = func(ctx context.Context, tc executor.ToolCall) (*executor.ToolResult, error) {
		return &executor.ToolResult{Status: executor.StatusSuccess, Output: "data"}, nil
	}

	result, err := e.Run(context.Background(), "do something", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 2 {
		t.Errorf("provider calls = %d, want 2", callCount)
	}
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if result.Content != "final answer" {
		t.Errorf("Content = %q, want %q", result.Content, "final answer")
	}
	if result.Budget == nil {
		t.Fatal("Budget snapshot should be attached when a budget is configured")
	}
	if result.Budget.AICalls != 2 {
		t.Errorf("Budget.AICalls = %d, want 2", result.Budget.AICalls)
	}
	if result.Budget.Tokens != 60 {
		t.Errorf("Budget.Tokens = %d, want 60", result.Budget.Tokens)
	}
}

// TestRunStream_BudgetExhausted_EmitsError verifies the streaming path also
// guards the LLM call with the cognitive budget.
func TestRunStream_BudgetExhausted_EmitsError(t *testing.T) {
	reg, cb, rtr, prov, exec := setupEngineTest(t)
	cfg := defaultEngineConfig()
	cfg.Budget = &CognitiveBudget{MaxTokens: 1, MaxDuration: time.Minute, MaxCost: 1.0}
	e := NewAgentEngine(reg, cb, rtr, prov, exec, cfg, nil, nil, nil)

	// First stream call returns content + a tool call chunk → forces a second
	// turn AND consumes budget. The streaming path estimates tokens from the
	// generated content (streams report no usage), so a tool-call-only chunk
	// (empty content) would record 0 tokens and CanCall would still allow the
	// second call — the budget guard would never trip.
	toolCallChunks := []chat.ChatStreamChunk{
		{
			Choices: []chat.StreamChoice{
				{
					Delta: chat.Message{
						Content: "I'll read the file.",
						ToolCalls: []chat.ToolCall{
							{Index: 0, ID: "call_1", Type: "function", Function: chat.FunctionCall{Name: "read_file", Arguments: `{"path":"test.txt"}`}},
						},
					},
					FinishReason: chat.FinishReasonToolCalls,
				},
			},
		},
	}
	doneChunks := []chat.ChatStreamChunk{
		{Choices: []chat.StreamChoice{{Delta: chat.Message{Content: "Done"}, FinishReason: chat.FinishReasonStop}}},
	}

	callCount := 0
	prov.chatStreamFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (chat.ChatStream, error) {
		callCount++
		if callCount == 1 {
			return newMockStreamWithChunks(toolCallChunks), nil
		}
		return newMockStreamWithChunks(doneChunks), nil
	}

	exec.executeFunc = func(ctx context.Context, tc executor.ToolCall) (*executor.ToolResult, error) {
		return &executor.ToolResult{Status: executor.StatusSuccess, Output: "data"}, nil
	}

	events, err := e.RunStream(context.Background(), "read the file", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotBudgetErr := false
	for evt := range events {
		if evt.Type == EngineEventError && strings.Contains(evt.Error.Error(), "budget cognitivo estourado") {
			gotBudgetErr = true
		}
	}
	if !gotBudgetErr {
		t.Error("expected budget error event when streaming budget is exhausted")
	}
	if callCount != 1 {
		t.Errorf("provider stream calls = %d, want 1 (second call must be blocked)", callCount)
	}
}
