package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
)

func setupContextTest(t *testing.T) (*AgentRegistry, *ContextBuilder) {
	t.Helper()
	dir := t.TempDir()
	// Self-contained AGENTS.md project context: the builder walks up from the
	// project dir looking for AGENTS.md files, and the test must not depend on
	// whether one exists in the ambient working-directory tree (on Windows dev
	// boxes the repo root has none, so the chain would be empty).
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("Project instructions.\n"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}
	writeAgentFile(t, dir, "test-agent.md",
		"name: cosca-test\ncapabilities: [test]",
		"You are a test agent. Be helpful.")

	writeAgentFile(t, dir, "empty-agent.md",
		"name: cosca-empty",
		"")

	registry := NewAgentRegistry(dir)
	builder := NewContextBuilder(registry, 0).WithProjectDir(dir) // uses default
	return registry, builder
}

func TestNewContextBuilder(t *testing.T) {
	t.Run("creates with default max tokens", func(t *testing.T) {
		registry := NewAgentRegistry()
		b := NewContextBuilder(registry, 0)
		if b.maxTokens != DefaultMaxTokens {
			t.Errorf("maxTokens = %d, want %d", b.maxTokens, DefaultMaxTokens)
		}
	})

	t.Run("creates with custom max tokens", func(t *testing.T) {
		registry := NewAgentRegistry()
		b := NewContextBuilder(registry, 64000)
		if b.maxTokens != 64000 {
			t.Errorf("maxTokens = %d, want 64000", b.maxTokens)
		}
	})

	t.Run("clamps negative max tokens to default", func(t *testing.T) {
		registry := NewAgentRegistry()
		b := NewContextBuilder(registry, -1)
		if b.maxTokens != DefaultMaxTokens {
			t.Errorf("maxTokens = %d, want %d", b.maxTokens, DefaultMaxTokens)
		}
	})
}

func TestContextBuild(t *testing.T) {
	t.Run("assembles system prompt from agent", func(t *testing.T) {
		_, builder := setupContextTest(t)
		ctx := context.Background()

		bc := builder.Build(ctx, "cosca-test", nil, nil, nil, nil)
		// O sistema prompt começa com a persona do agente e ganha o contexto
		// de projeto (cadeia AGENTS.md) — feature desenhada em context.go
		// ("Project Context — AGENTS.md chain of inheritance").
		if !strings.HasPrefix(bc.SystemPrompt, "You are a test agent. Be helpful.") {
			t.Errorf("SystemPrompt = %q", bc.SystemPrompt)
		}
		// AGENTS.md content is now wrapped in content trust envelopes for security.
		if !strings.Contains(bc.SystemPrompt, "cosca-untrusted-data-v1") {
			t.Errorf("SystemPrompt deveria incluir o contexto AGENTS.md (content trust envelope): %q", bc.SystemPrompt)
		}
		if bc.ContextLimit != DefaultMaxTokens {
			t.Errorf("ContextLimit = %d", bc.ContextLimit)
		}
	})

	t.Run("handles empty agent name gracefully", func(t *testing.T) {
		_, builder := setupContextTest(t)
		ctx := context.Background()

		bc := builder.Build(ctx, "", nil, nil, nil, nil)
		// Sem agente, não há persona — mas o contexto de projeto (AGENTS.md)
		// ainda é injetado pelo builder (feature de context.go).
		if strings.Contains(bc.SystemPrompt, "You are a test agent") {
			t.Errorf("expected no agent persona for empty name, got %q", bc.SystemPrompt)
		}
		// AGENTS.md content is now wrapped in content trust envelopes for security.
		if !strings.Contains(bc.SystemPrompt, "cosca-untrusted-data-v1") {
			t.Errorf("expected AGENTS.md project context (content trust envelope), got %q", bc.SystemPrompt)
		}
	})

	t.Run("handles unknown agent name", func(t *testing.T) {
		_, builder := setupContextTest(t)
		ctx := context.Background()

		bc := builder.Build(ctx, "nonexistent-agent", nil, nil, nil, nil)
		if strings.Contains(bc.SystemPrompt, "You are a test agent") {
			t.Errorf("expected no agent persona for unknown agent, got %q", bc.SystemPrompt)
		}
		// AGENTS.md content is now wrapped in content trust envelopes for security.
		if !strings.Contains(bc.SystemPrompt, "cosca-untrusted-data-v1") {
			t.Errorf("expected AGENTS.md project context (content trust envelope), got %q", bc.SystemPrompt)
		}
	})

	t.Run("includes conversation history", func(t *testing.T) {
		_, builder := setupContextTest(t)
		ctx := context.Background()

		messages := []chat.Message{
			{Role: chat.RoleUser, Content: "Hello"},
			{Role: chat.RoleAssistant, Content: "Hi there"},
		}
		bc := builder.Build(ctx, "cosca-test", messages, nil, nil, nil)
		if len(bc.Messages) != 2 {
			t.Errorf("expected 2 messages, got %d", len(bc.Messages))
		}
	})

	t.Run("includes memories", func(t *testing.T) {
		_, builder := setupContextTest(t)
		ctx := context.Background()

		memories := []string{"User likes Go programming", "User has worked on CLI tools"}
		bc := builder.Build(ctx, "cosca-test", nil, nil, memories, nil)
		if !contains(bc.SystemPrompt, "RELEVANT MEMORIES") {
			t.Errorf("system prompt should contain memories section, got: %s", bc.SystemPrompt)
		}
		if !contains(bc.SystemPrompt, "User likes Go programming") {
			t.Errorf("system prompt should contain memory content")
		}
	})

	t.Run("includes knowledge", func(t *testing.T) {
		_, builder := setupContextTest(t)
		ctx := context.Background()

		knowledge := []string{"Cosca uses hexagonal architecture", "Agents are defined in Markdown"}
		bc := builder.Build(ctx, "cosca-test", nil, nil, nil, knowledge)
		if !contains(bc.SystemPrompt, "RELEVANT KNOWLEDGE") {
			t.Errorf("system prompt should contain knowledge section")
		}
	})

	t.Run("includes both memories and knowledge", func(t *testing.T) {
		_, builder := setupContextTest(t)
		ctx := context.Background()

		bc := builder.Build(ctx, "cosca-test", nil, nil,
			[]string{"memory1"}, []string{"knowledge1"})
		if !contains(bc.SystemPrompt, "RELEVANT MEMORIES") {
			t.Error("missing memories section")
		}
		if !contains(bc.SystemPrompt, "RELEVANT KNOWLEDGE") {
			t.Error("missing knowledge section")
		}
	})

	t.Run("converts tools to ToolDefinitions", func(t *testing.T) {
		_, builder := setupContextTest(t)
		ctx := context.Background()

		tools := []chat.Tool{
			newMockTool("read_file", "Read a file"),
			newMockTool("write_file", "Write a file"),
		}
		bc := builder.Build(ctx, "cosca-test", nil, tools, nil, nil)
		if len(bc.ToolDefinitions) != 2 {
			t.Errorf("expected 2 tool definitions, got %d", len(bc.ToolDefinitions))
		}
		if bc.ToolDefinitions[0].Function.Name != "read_file" {
			t.Errorf("ToolDef[0].Name = %q", bc.ToolDefinitions[0].Function.Name)
		}
		if bc.ToolDefinitions[1].Function.Name != "write_file" {
			t.Errorf("ToolDef[1].Name = %q", bc.ToolDefinitions[1].Function.Name)
		}
	})

	t.Run("tool with schema is converted to parameters", func(t *testing.T) {
		_, builder := setupContextTest(t)
		ctx := context.Background()

		tool := newMockTool("search", "Search tool")
		tool.schema = json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`)

		bc := builder.Build(ctx, "cosca-test", nil, []chat.Tool{tool}, nil, nil)
		if len(bc.ToolDefinitions) != 1 {
			t.Fatal("expected 1 tool definition")
		}
		params := bc.ToolDefinitions[0].Function.Parameters
		if params == nil {
			t.Fatal("expected non-nil parameters")
		}
		if params["type"] != "object" {
			t.Errorf("params type = %v", params["type"])
		}
	})

	t.Run("tool with invalid schema is handled gracefully", func(t *testing.T) {
		_, builder := setupContextTest(t)
		ctx := context.Background()

		tool := newMockTool("broken", "Broken schema tool")
		tool.schema = json.RawMessage(`{invalid json}`)

		bc := builder.Build(ctx, "cosca-test", nil, []chat.Tool{tool}, nil, nil)
		if len(bc.ToolDefinitions) != 1 {
			t.Fatal("expected 1 tool definition")
		}
		// Should have nil parameters (schema parse error silently ignored)
		if bc.ToolDefinitions[0].Function.Parameters != nil {
			t.Errorf("expected nil parameters for invalid schema, got %v", bc.ToolDefinitions[0].Function.Parameters)
		}
	})

	t.Run("token estimate includes system prompt and messages", func(t *testing.T) {
		_, builder := setupContextTest(t)
		ctx := context.Background()

		messages := []chat.Message{
			{Role: chat.RoleUser, Content: "Hello, world!"},
			{Role: chat.RoleAssistant, Content: "Hi, how can I help?"},
		}
		bc := builder.Build(ctx, "cosca-test", messages, nil, nil, nil)
		if bc.TokenEstimate <= 0 {
			t.Errorf("expected positive token estimate, got %d", bc.TokenEstimate)
		}
	})

	t.Run("usage percentage is calculated", func(t *testing.T) {
		_, builder := setupContextTest(t)
		ctx := context.Background()

		bc := builder.Build(ctx, "cosca-test", nil, nil, nil, nil)
		if bc.UsagePct <= 0 {
			t.Errorf("expected positive usage percentage, got %f", bc.UsagePct)
		}
	})

	t.Run("includes ContentParts in token estimate", func(t *testing.T) {
		_, builder := setupContextTest(t)
		ctx := context.Background()

		messages := []chat.Message{
			{
				Role:    chat.RoleUser,
				Content: "Hello",
				ContentParts: []chat.ContentPart{
					{Type: "text", Text: " with image description"},
				},
			},
		}
		bc := builder.Build(ctx, "cosca-test", messages, nil, nil, nil)
		if bc.TokenEstimate <= 0 {
			t.Errorf("expected positive token estimate with ContentParts, got %d", bc.TokenEstimate)
		}
	})
}

func TestEstimateTokens(t *testing.T) {
	builder := NewContextBuilder(NewAgentRegistry(), 0)

	t.Run("returns 0 for empty string", func(t *testing.T) {
		if got := builder.EstimateTokens(""); got != 0 {
			t.Errorf("EstimateTokens('') = %d, want 0", got)
		}
	})

	t.Run("returns at least 1 for very short strings", func(t *testing.T) {
		if got := builder.EstimateTokens("a"); got != 1 {
			t.Errorf("EstimateTokens('a') = %d, want 1", got)
		}
	})

	t.Run("estimates approximately 4 chars per token", func(t *testing.T) {
		// 100 chars should give ~25 tokens + ~1 overhead = ~26
		input := "Hello, world! This is a test of the token estimation function."
		got := builder.EstimateTokens(input)
		expected := len(input)/4 + len(input)/100
		if got != expected {
			t.Errorf("EstimateTokens = %d, want %d", got, expected)
		}
	})

	t.Run("works with long text", func(t *testing.T) {
		input := string(make([]byte, 10000))
		got := builder.EstimateTokens(input)
		if got <= 0 {
			t.Error("expected positive estimate")
		}
	})
}

// ─── Helpers ────────────────────────────────────────────────────────────────────

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
