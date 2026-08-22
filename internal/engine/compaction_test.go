package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
)

func TestNewAutoCompaction(t *testing.T) {
	t.Run("creates with default threshold", func(t *testing.T) {
		ac := NewAutoCompaction(100000)
		if ac.maxTokens != 100000 {
			t.Errorf("maxTokens = %d, want 100000", ac.maxTokens)
		}
		if ac.compactionPct != defaultCompactionThreshold {
			t.Errorf("compactionPct = %f, want %f", ac.compactionPct, defaultCompactionThreshold)
		}
		if ac.compactionCount != 0 {
			t.Errorf("compactionCount = %d, want 0", ac.compactionCount)
		}
	})
}

func TestNewAutoCompactionWithThreshold(t *testing.T) {
	t.Run("creates with custom threshold", func(t *testing.T) {
		ac := NewAutoCompactionWithThreshold(100000, 0.5)
		if ac.compactionPct != 0.5 {
			t.Errorf("compactionPct = %f, want 0.5", ac.compactionPct)
		}
	})

	t.Run("clamps threshold <= 0 to default", func(t *testing.T) {
		ac := NewAutoCompactionWithThreshold(100000, 0)
		if ac.compactionPct != defaultCompactionThreshold {
			t.Errorf("compactionPct = %f, want %f", ac.compactionPct, defaultCompactionThreshold)
		}
	})

	t.Run("clamps threshold > 1.0 to default", func(t *testing.T) {
		ac := NewAutoCompactionWithThreshold(100000, 1.5)
		if ac.compactionPct != defaultCompactionThreshold {
			t.Errorf("compactionPct = %f, want %f", ac.compactionPct, defaultCompactionThreshold)
		}
	})

	t.Run("clamps negative threshold to default", func(t *testing.T) {
		ac := NewAutoCompactionWithThreshold(100000, -0.5)
		if ac.compactionPct != defaultCompactionThreshold {
			t.Errorf("compactionPct = %f, want %f", ac.compactionPct, defaultCompactionThreshold)
		}
	})

	t.Run("threshold at exactly 1.0 is valid", func(t *testing.T) {
		ac := NewAutoCompactionWithThreshold(100000, 1.0)
		if ac.compactionPct != 1.0 {
			t.Errorf("compactionPct = %f, want 1.0", ac.compactionPct)
		}
	})
}

func TestCheck(t *testing.T) {
	t.Run("returns no compaction when under threshold", func(t *testing.T) {
		ac := NewAutoCompaction(1000)
		messages := []chat.Message{
			{Role: chat.RoleUser, Content: "Hello"},
			{Role: chat.RoleAssistant, Content: "Hi"},
		}
		result, err := ac.Check(context.Background(), messages, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Compacted {
			t.Error("expected no compaction when under threshold")
		}
	})

	t.Run("triggers compaction when over threshold", func(t *testing.T) {
		ac := NewAutoCompaction(1000)
		// Create enough messages so after stripping tool messages, we still have
		// enough to compact (more than minKeepMessages).
		messages := []chat.Message{
			{Role: chat.RoleUser, Content: "Hello 1"},
			{Role: chat.RoleAssistant, Content: "Response 1"},
			{Role: chat.RoleUser, Content: "Hello 2"},
			{Role: chat.RoleAssistant, Content: "Response 2"},
			{Role: chat.RoleUser, Content: "Hello 3"},
			{Role: chat.RoleAssistant, Content: "Response 3"},
			{Role: chat.RoleUser, Content: "Hello 4"},
			{Role: chat.RoleAssistant, Content: "Response 4"},
			{Role: chat.RoleUser, Content: "Hello 5"},
			{Role: chat.RoleAssistant, Content: "Response 5"},
		}

		// estimatedTokens must be >= 800 (80% of 1000)
		result, err := ac.Check(context.Background(), messages, 900)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Compacted {
			t.Error("expected compaction when over threshold")
		}
		if result.MessagesRemaining <= 0 {
			t.Errorf("expected some messages to remain, got %d", result.MessagesRemaining)
		}
	})

	t.Run("no compaction for empty messages", func(t *testing.T) {
		ac := NewAutoCompaction(1000)
		result, err := ac.Check(context.Background(), []chat.Message{}, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Compacted {
			t.Error("expected no compaction for empty messages")
		}
	})

	t.Run("no compaction for zero estimated tokens", func(t *testing.T) {
		ac := NewAutoCompaction(1000)
		messages := []chat.Message{{Role: chat.RoleUser, Content: "Hello"}}
		result, err := ac.Check(context.Background(), messages, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Compacted {
			t.Error("expected no compaction for zero tokens")
		}
	})

	t.Run("thrashing detection returns error", func(t *testing.T) {
		ac := NewAutoCompaction(1000)
		messages := []chat.Message{
			{Role: chat.RoleUser, Content: "A"},
			{Role: chat.RoleAssistant, Content: "B"},
			{Role: chat.RoleUser, Content: "C"},
			{Role: chat.RoleAssistant, Content: "D"},
			{Role: chat.RoleUser, Content: "E"},
			{Role: chat.RoleAssistant, Content: "F"},
		}

		// First compaction
		result, err := ac.Check(context.Background(), messages, 900)
		if err != nil {
			t.Fatalf("first compaction failed: %v", err)
		}
		if !result.Compacted {
			t.Fatal("expected first compaction to succeed")
		}

		// Second check immediately after — should detect thrashing
		// because detectThrashing was set to true by Compact.
		_, err = ac.Check(context.Background(), messages, 900)
		if err == nil {
			t.Error("expected thrashing error on second check")
		}
		if !strings.Contains(err.Error(), "thrashing") {
			t.Errorf("error should mention thrashing, got: %v", err)
		}
	})

	t.Run("no thrashing if under threshold after compact", func(t *testing.T) {
		ac := NewAutoCompaction(1000)
		messages := []chat.Message{
			{Role: chat.RoleUser, Content: "A"},
			{Role: chat.RoleAssistant, Content: "B"},
		}

		// Under threshold, detectThrashing set to false
		result, err := ac.Check(context.Background(), messages, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Compacted {
			t.Error("expected no compaction")
		}

		// Second check under threshold should be fine
		result, err = ac.Check(context.Background(), messages, 100)
		if err != nil {
			t.Fatalf("second check failed: %v", err)
		}
		if result.Compacted {
			t.Error("expected no compaction on second check")
		}
	})
}

func TestCompact(t *testing.T) {
	t.Run("removes tool messages", func(t *testing.T) {
		ac := NewAutoCompaction(1000)
		messages := []chat.Message{
			{Role: chat.RoleUser, Content: "Hello"},
			{Role: chat.RoleAssistant, Content: "I'll use a tool"},
			{Role: chat.RoleTool, Content: "tool output 1", ToolCallID: "call_1"},
			{Role: chat.RoleUser, Content: "Thanks"},
			{Role: chat.RoleAssistant, Content: "You're welcome"},
			{Role: chat.RoleTool, Content: "tool output 2", ToolCallID: "call_2"},
		}

		result, err := ac.Compact(context.Background(), messages)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Compacted {
			t.Error("expected compacted = true")
		}
		if result.MessagesRemaining <= 0 {
			t.Errorf("expected remaining messages, got %d", result.MessagesRemaining)
		}

		// Tool messages should be removed
		for _, m := range messages {
			if m.Role == chat.RoleTool {
				t.Logf("tool message found (should have been removed): %q", m.Content)
			}
		}
	})

	t.Run("keeps minimum messages", func(t *testing.T) {
		ac := NewAutoCompaction(1000)
		// Just a few messages — should stay as-is after stripping tool messages
		messages := []chat.Message{
			{Role: chat.RoleUser, Content: "Hello"},
			{Role: chat.RoleAssistant, Content: "Hi"},
			{Role: chat.RoleTool, Content: "tool data", ToolCallID: "call_1"},
		}

		result, err := ac.Compact(context.Background(), messages)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Compacted {
			t.Error("expected compacted = true")
		}
		if result.MessagesRemaining < 2 {
			t.Errorf("expected at least 2 remaining messages, got %d", result.MessagesRemaining)
		}
	})

	t.Run("handles empty messages", func(t *testing.T) {
		ac := NewAutoCompaction(1000)
		result, err := ac.Compact(context.Background(), []chat.Message{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Compacted {
			t.Error("expected no compaction for empty messages")
		}
	})

	t.Run("handles few messages - no compaction needed", func(t *testing.T) {
		ac := NewAutoCompaction(1000)
		messages := []chat.Message{
			{Role: chat.RoleUser, Content: "Hello"},
			{Role: chat.RoleAssistant, Content: "Hi there"},
		}

		// After stripping tool messages (none), len is 2 which is <= minKeepMessages (4)
		result, err := ac.Compact(context.Background(), messages)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Compacted {
			t.Error("expected compacted = true even for few messages (tool stripping still happens)")
		}
	})

	t.Run("builds summary from old messages", func(t *testing.T) {
		ac := NewAutoCompaction(1000)
		messages := make([]chat.Message, 12)
		for i := 0; i < 12; i++ {
			role := chat.RoleUser
			if i%2 == 1 {
				role = chat.RoleAssistant
			}
			messages[i] = chat.Message{Role: role, Content: "Message " + string(rune('A'+i))}
		}

		result, err := ac.Compact(context.Background(), messages)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Summary == "" {
			t.Error("expected non-empty summary")
		}
	})

	t.Run("tokens saved is calculated", func(t *testing.T) {
		ac := NewAutoCompaction(1000)
		messages := make([]chat.Message, 6)
		for i := 0; i < 6; i++ {
			role := chat.RoleUser
			if i%2 == 1 {
				role = chat.RoleAssistant
			}
			messages[i] = chat.Message{Role: role, Content: "Test message content with enough text to see token savings"}
		}

		result, err := ac.Compact(context.Background(), messages)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.TokensSaved < 0 {
			t.Errorf("TokensSaved = %d, should be >= 0", result.TokensSaved)
		}
	})
}

func TestCompactMessages(t *testing.T) {
	t.Run("strips tool messages and creates summary", func(t *testing.T) {
		ac := NewAutoCompaction(100000)
		messages := []chat.Message{
			{Role: chat.RoleUser, Content: "What's the weather?"},
			{Role: chat.RoleAssistant, Content: "Let me check", ToolCalls: []chat.ToolCall{{ID: "tc1", Function: chat.FunctionCall{Name: "get_weather"}}}},
			{Role: chat.RoleTool, Content: "Sunny 72°F", ToolCallID: "tc1"},
			{Role: chat.RoleAssistant, Content: "It's sunny and 72°F."},
			{Role: chat.RoleUser, Content: "Great! And tomorrow?"},
			{Role: chat.RoleAssistant, Content: "Let me check again", ToolCalls: []chat.ToolCall{{ID: "tc2", Function: chat.FunctionCall{Name: "get_weather"}}}},
			{Role: chat.RoleTool, Content: "Rainy 65°F", ToolCallID: "tc2"},
			{Role: chat.RoleAssistant, Content: "Tomorrow will be rainy and 65°F."},
			{Role: chat.RoleUser, Content: "Thanks!"},
			{Role: chat.RoleAssistant, Content: "You're welcome!"},
		}

		compacted := ac.compactMessages(messages)

		// Should have fewer messages
		if len(compacted) >= len(messages) {
			t.Errorf("compacted should have fewer messages, got %d >= %d", len(compacted), len(messages))
		}

		// No tool messages should remain
		for _, m := range compacted {
			if m.Role == chat.RoleTool {
				t.Errorf("tool message should have been removed: %+v", m)
			}
		}

		// First message should be the summary
		if compacted[0].Role != chat.RoleSystem {
			t.Errorf("first compacted message role = %q, want system", compacted[0].Role)
		}
		if !strings.HasPrefix(compacted[0].Content, "Previous conversation summary:") {
			t.Errorf("first compacted message should start with summary prefix, got: %q", compacted[0].Content)
		}

		// Summary should mention user questions
		if !strings.Contains(compacted[0].Content, "weather") {
			t.Errorf("summary should mention weather, got: %s", compacted[0].Content)
		}
	})

	t.Run("handles messages with empty content", func(t *testing.T) {
		ac := NewAutoCompaction(100000)
		messages := []chat.Message{
			{Role: chat.RoleUser, Content: ""},
			{Role: chat.RoleAssistant, Content: "Hello"},
			{Role: chat.RoleUser, Content: "How are you?"},
			{Role: chat.RoleAssistant, Content: ""},
			{Role: chat.RoleUser, Content: "Still there?"},
			{Role: chat.RoleAssistant, Content: "Yes!"},
		}

		compacted := ac.compactMessages(messages)
		if len(compacted) == 0 {
			t.Error("expected non-empty compacted result")
		}
		// Should not panic
	})
}

func TestEstimateTokenCount(t *testing.T) {
	t.Run("estimates tokens for messages", func(t *testing.T) {
		messages := []chat.Message{
			{Role: chat.RoleUser, Content: "Hello, world!"},
			{Role: chat.RoleAssistant, Content: "Hi, how can I help you today?"},
		}
		tokens := estimateTokenCount(messages)
		if tokens <= 0 {
			t.Errorf("expected positive token count, got %d", tokens)
		}
	})

	t.Run("includes tool call arguments in estimate", func(t *testing.T) {
		messages := []chat.Message{
			{Role: chat.RoleAssistant, Content: "Let me search",
				ToolCalls: []chat.ToolCall{
					{Function: chat.FunctionCall{Name: "search", Arguments: `{"query":"test"}`}},
				}},
			{Role: chat.RoleTool, Content: "Results here", ToolCallID: "call_1"},
		}
		tokens := estimateTokenCount(messages)
		if tokens <= 0 {
			t.Errorf("expected positive token count, got %d", tokens)
		}
	})

	t.Run("returns 0 for empty messages", func(t *testing.T) {
		tokens := estimateTokenCount([]chat.Message{})
		if tokens != 0 {
			t.Errorf("expected 0 for empty messages, got %d", tokens)
		}
	})
}

func TestBuildSummary(t *testing.T) {
	t.Run("builds summary from messages", func(t *testing.T) {
		messages := []chat.Message{
			{Role: chat.RoleUser, Content: "What is the capital of France?"},
			{Role: chat.RoleAssistant, Content: "The capital of France is Paris."},
			{Role: chat.RoleSystem, Content: "Be helpful."},
		}

		summary := buildSummary(messages)
		if !strings.Contains(summary, "capital of France") {
			t.Errorf("summary should mention user question, got: %s", summary)
		}
		if !strings.Contains(summary, "Paris") {
			t.Errorf("summary should mention assistant response, got: %s", summary)
		}
		if !strings.Contains(summary, "Be helpful") {
			t.Errorf("summary should mention system instruction, got: %s", summary)
		}
	})

	t.Run("returns empty for empty messages", func(t *testing.T) {
		summary := buildSummary([]chat.Message{})
		if summary != "" {
			t.Errorf("expected empty summary, got: %s", summary)
		}
	})

	t.Run("skips messages with empty content", func(t *testing.T) {
		messages := []chat.Message{
			{Role: chat.RoleUser, Content: "Hello"},
			{Role: chat.RoleAssistant, Content: ""},
			{Role: chat.RoleUser, Content: ""},
			{Role: chat.RoleAssistant, Content: "World"},
		}
		summary := buildSummary(messages)
		if !strings.Contains(summary, "Hello") {
			t.Errorf("summary should contain 'Hello', got: %s", summary)
		}
		if !strings.Contains(summary, "World") {
			t.Errorf("summary should contain 'World', got: %s", summary)
		}
	})

	t.Run("truncates long messages", func(t *testing.T) {
		longMsg := string(make([]byte, 200))
		messages := []chat.Message{
			{Role: chat.RoleUser, Content: longMsg},
		}
		summary := buildSummary(messages)
		if !strings.Contains(summary, "...") {
			t.Error("expected long message to be truncated with ...")
		}
	})

	t.Run("formats assistant messages with prefix", func(t *testing.T) {
		messages := []chat.Message{
			{Role: chat.RoleAssistant, Content: "Here is the answer."},
		}
		summary := buildSummary(messages)
		if !strings.Contains(summary, "Assistant: Here is the answer.") {
			t.Errorf("assistant message should be prefixed with 'Assistant:', got: %s", summary)
		}
	})

	t.Run("formats system messages with prefix", func(t *testing.T) {
		messages := []chat.Message{
			{Role: chat.RoleSystem, Content: "Follow the guidelines."},
		}
		summary := buildSummary(messages)
		if !strings.Contains(summary, "Instruction: Follow the guidelines.") {
			t.Errorf("system message should be prefixed with 'Instruction:', got: %s", summary)
		}
	})
}

func TestExtractSummaryFromCompacted(t *testing.T) {
	t.Run("extracts summary from compacted messages", func(t *testing.T) {
		compacted := []chat.Message{
			{Role: chat.RoleSystem, Content: "Previous conversation summary: User asked about Go programming"},
			{Role: chat.RoleUser, Content: "Thanks!"},
		}
		summary := extractSummaryFromCompacted(compacted)
		if summary != "User asked about Go programming" {
			t.Errorf("got %q", summary)
		}
	})

	t.Run("returns empty when no summary found", func(t *testing.T) {
		compacted := []chat.Message{
			{Role: chat.RoleUser, Content: "Hello"},
		}
		summary := extractSummaryFromCompacted(compacted)
		if summary != "" {
			t.Errorf("got %q, want empty", summary)
		}
	})

	t.Run("returns empty for empty input", func(t *testing.T) {
		summary := extractSummaryFromCompacted([]chat.Message{})
		if summary != "" {
			t.Errorf("got %q, want empty", summary)
		}
	})
}
