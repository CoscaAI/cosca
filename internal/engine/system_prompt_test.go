package engine

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
)

// ─── Stubs for memory/knowledge ports ────────────────────────────────────────

type mockMemoryRetriever struct {
	results []MemoryRecord
	err     error
}

func (m *mockMemoryRetriever) Search(ctx context.Context, query string, opts MemorySearchOptions) ([]MemoryRecord, error) {
	return m.results, m.err
}

func (m *mockMemoryRetriever) Retrieve(ctx context.Context, id, layer string) (*MemoryRecord, error) {
	return nil, nil
}

type mockKnowledgeSearcher struct {
	results *KnowledgeSearchResults
	err     error
}

func (m *mockKnowledgeSearcher) Search(ctx context.Context, params KnowledgeSearchParams) (*KnowledgeSearchResults, error) {
	return m.results, m.err
}

// ─── System prompt injection ──────────────────────────────────────────────────

// TestRunInjectsSystemPrompt proves that Run prepends the built system prompt
// (agent identity) as the first system message sent to the provider — the fix
// for the asymmetry where only builtCtx.Messages was copied.
func TestRunInjectsSystemPrompt(t *testing.T) {
	t.Run("first message is system containing the agent identity", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)

		var captured []chat.Message
		prov.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
			captured = messages
			return &chat.ChatResponse{
				Choices: []chat.Choice{{Message: chat.Message{Role: chat.RoleAssistant, Content: "ok"}}},
				Usage:   chat.Usage{TotalTokens: 10},
			}, nil
		}

		_, err := e.Run(context.Background(), "Hello", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(captured) == 0 {
			t.Fatal("no messages captured from provider")
		}
		first := captured[0]
		if first.Role != chat.RoleSystem {
			t.Errorf("first message role = %q, want %q", first.Role, chat.RoleSystem)
		}
		if strings.TrimSpace(first.Content) == "" {
			t.Error("system prompt must be non-empty when the agent exists")
		}
		if !strings.Contains(first.Content, "Cosca Kernel") {
			t.Errorf("system prompt should contain agent identity, got %q", first.Content)
		}
	})

	t.Run("system prompt includes memories and knowledge", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)
		e.memoryRetriever = &mockMemoryRetriever{
			results: []MemoryRecord{{ID: "m1", Layer: "session", Content: "User prefers Go over Rust"}},
		}
		e.knowledge = &mockKnowledgeSearcher{
			results: &KnowledgeSearchResults{
				Results: []KnowledgeSearchResult{{Title: "K1", Content: "Cosca uses hexagonal architecture"}},
			},
		}

		var captured []chat.Message
		prov.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
			captured = messages
			return &chat.ChatResponse{
				Choices: []chat.Choice{{Message: chat.Message{Role: chat.RoleAssistant, Content: "ok"}}},
				Usage:   chat.Usage{TotalTokens: 10},
			}, nil
		}

		_, err := e.Run(context.Background(), "Hello", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(captured) == 0 || captured[0].Role != chat.RoleSystem {
			t.Fatalf("expected first message to be system, got %+v", captured)
		}
		sys := captured[0].Content
		if !strings.Contains(sys, "RELEVANT MEMORIES") || !strings.Contains(sys, "User prefers Go") {
			t.Errorf("system prompt should embed memories, got: %s", sys)
		}
		if !strings.Contains(sys, "RELEVANT KNOWLEDGE") || !strings.Contains(sys, "hexagonal architecture") {
			t.Errorf("system prompt should embed knowledge, got: %s", sys)
		}
	})

	t.Run("does not duplicate an existing leading system message", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)

		history := []chat.Message{
			{Role: chat.RoleSystem, Content: "custom identity already in history"},
			{Role: chat.RoleUser, Content: "earlier question"},
		}
		var captured []chat.Message
		prov.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
			captured = messages
			return &chat.ChatResponse{
				Choices: []chat.Choice{{Message: chat.Message{Role: chat.RoleAssistant, Content: "ok"}}},
				Usage:   chat.Usage{TotalTokens: 10},
			}, nil
		}

		_, err := e.Run(context.Background(), "Hello", history)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		count := 0
		for _, m := range captured {
			if m.Role == chat.RoleSystem {
				count++
			}
		}
		if count != 1 {
			t.Errorf("expected exactly 1 system message (no duplication), got %d", count)
		}
		if captured[0].Role != chat.RoleSystem || captured[0].Content != "custom identity already in history" {
			t.Errorf("history system message should be preserved as-is, got %+v", captured[0])
		}
	})
}

// TestRunStreamInjectsSystemPrompt mirrors the Run assertions on the streaming
// path — the one used by the TUI.
func TestRunStreamInjectsSystemPrompt(t *testing.T) {
	reg, cb, rtr, prov, exec := setupEngineTest(t)
	e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)

	var captured []chat.Message
	prov.chatStreamFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (chat.ChatStream, error) {
		captured = messages
		return newMockStream("hello", nil), nil
	}

	events, err := e.RunStream(context.Background(), "Hello", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range events {
	}

	if len(captured) == 0 {
		t.Fatal("no messages captured from provider")
	}
	first := captured[0]
	if first.Role != chat.RoleSystem {
		t.Errorf("first message role = %q, want %q", first.Role, chat.RoleSystem)
	}
	if strings.TrimSpace(first.Content) == "" {
		t.Error("system prompt must be non-empty when the agent exists")
	}
	if !strings.Contains(first.Content, "Cosca Kernel") {
		t.Errorf("system prompt should contain agent identity, got %q", first.Content)
	}
}

// ─── Router fallback ──────────────────────────────────────────────────────────

// TestRouteFallbackToKernel proves the router fallback points at an agent that
// actually exists (cosca-kernel), not the removed cosca-general.
func TestRouteFallbackToKernel(t *testing.T) {
	_, router := setupRouterTest(t)
	ctx := context.Background()

	result := router.Route(ctx, "zzz flurbo garblex", nil)
	if result.Method != methodFallback {
		t.Fatalf("Method = %q, want %q", result.Method, methodFallback)
	}
	if result.Agent != "cosca-kernel" {
		t.Errorf("fallback Agent = %q, want %q", result.Agent, "cosca-kernel")
	}
	if result.Agent == "cosca-general" {
		t.Error("fallback must not return cosca-general (it does not exist in .cosca/framework/agents/)")
	}
}

// TestDefaultAgentLoadsFromRealFallback verifies the real .cosca/framework/agents
// directory registers cosca-kernel with a non-empty system prompt. It is skipped
// when running from a checkout without that directory.
func TestDefaultAgentLoadsFromRealFallback(t *testing.T) {
	dir := "../../.cosca/framework/agents"
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("fallback agents dir not present: %v", err)
	}

	r := NewAgentRegistry(dir)
	def := r.Get(defaultAgent)
	if def == nil {
		t.Fatalf("default agent %q not loaded from %s", defaultAgent, dir)
	}
	if strings.TrimSpace(def.SystemPrompt) == "" {
		t.Error("default agent system prompt must be non-empty")
	}
}
