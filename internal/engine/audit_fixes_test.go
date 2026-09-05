package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/chat/executor"
)

// ─── Fix 1: Stage 3 semantic matching ──────────────────────────────────────────

func TestRouteSemantic(t *testing.T) {
	t.Run("semantic match routes when keyword is insufficient", func(t *testing.T) {
		dir := t.TempDir()
		writeAgentFile(t, dir, "research.md",
			"name: cosca-research\ncapabilities: [physics, particle]\ndescription: focuses on quantum teleportation entanglement coherence protocols",
			"Research prompt")
		writeAgentFile(t, dir, "poet.md",
			"name: cosca-poet\ncapabilities: [poetry, writing]\ndescription: writes beautiful poetry about flowers and gardens",
			"Poet prompt")

		registry := NewAgentRegistry(dir)
		router := NewRouter(registry)
		ctx := context.Background()

		// No capability/name substring overlaps, so keyword matching must fail and
		// semantic matching must select the research agent.
		result := router.Route(ctx, "quantum teleportation entanglement coherence", nil)
		if result.Agent != "cosca-research" {
			t.Errorf("Agent = %q, want %q", result.Agent, "cosca-research")
		}
		if result.Method != methodSemantic {
			t.Errorf("Method = %q, want %q", result.Method, methodSemantic)
		}
		if result.Confidence < semanticMinScore || result.Confidence > 1.0 {
			t.Errorf("Confidence = %f, want within [%f, 1.0]", result.Confidence, semanticMinScore)
		}
	})

	t.Run("no semantic similarity falls back", func(t *testing.T) {
		dir := t.TempDir()
		writeAgentFile(t, dir, "research.md",
			"name: cosca-research\ncapabilities: [physics, particle]\ndescription: focuses on quantum teleportation entanglement coherence protocols",
			"Research prompt")

		registry := NewAgentRegistry(dir)
		router := NewRouter(registry)
		ctx := context.Background()

		result := router.Route(ctx, "please recommend a good book to read", nil)
		if result.Agent != defaultAgent {
			t.Errorf("Agent = %q, want %q", result.Agent, defaultAgent)
		}
		if result.Method != methodFallback {
			t.Errorf("Method = %q, want %q", result.Method, methodFallback)
		}
		if result.Confidence != 0.3 {
			t.Errorf("Confidence = %f, want 0.3", result.Confidence)
		}
	})
}

// ─── Fix 2: AGENTS.md inheritance chain ────────────────────────────────────────

func TestContextAgentsMDChain(t *testing.T) {
	t.Run("includes AGENTS.md chain root to subdir", func(t *testing.T) {
		root := t.TempDir()
		sub := filepath.Join(root, "sub")
		if err := os.MkdirAll(sub, 0755); err != nil {
			t.Fatal(err)
		}
		rootAgents := filepath.Join(root, "AGENTS.md")
		subAgents := filepath.Join(sub, "AGENTS.md")
		if err := os.WriteFile(rootAgents, []byte("root instructions"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(subAgents, []byte("sub instructions"), 0644); err != nil {
			t.Fatal(err)
		}

		dir := t.TempDir()
		writeAgentFile(t, dir, "test-agent.md",
			"name: cosca-test\ncapabilities: [test]",
			"You are a test agent.")

		builder := NewContextBuilder(NewAgentRegistry(dir), 0)
		builder.WithProjectDir(sub)

		bc := builder.Build(context.Background(), "cosca-test", nil, nil, nil, nil)

		// AGENTS.md content is now wrapped in content trust envelopes for security.
		// Check that both root and sub content appear in the system prompt.
		if !strings.Contains(bc.SystemPrompt, "root instructions") {
			t.Errorf("system prompt should contain root AGENTS.md content, got:\n%s", bc.SystemPrompt)
		}
		if !strings.Contains(bc.SystemPrompt, "sub instructions") {
			t.Errorf("system prompt should contain sub AGENTS.md content, got:\n%s", bc.SystemPrompt)
		}
		// Verify content trust envelope is present
		if !strings.Contains(bc.SystemPrompt, "cosca-untrusted-data-v1") {
			t.Errorf("system prompt should contain content trust envelope, got:\n%s", bc.SystemPrompt)
		}

		// Inheritance order: root content must precede sub content.
		rootIdx := strings.Index(bc.SystemPrompt, "root instructions")
		subIdx := strings.Index(bc.SystemPrompt, "sub instructions")
		if rootIdx < 0 || subIdx < 0 || rootIdx > subIdx {
			t.Errorf("root AGENTS.md content should precede sub content (root at %d, sub at %d)", rootIdx, subIdx)
		}
	})

	t.Run("no AGENTS.md adds no sections and no error", func(t *testing.T) {
		dir := t.TempDir()
		writeAgentFile(t, dir, "test-agent.md",
			"name: cosca-test\ncapabilities: [test]",
			"You are a test agent.")

		builder := NewContextBuilder(NewAgentRegistry(dir), 0)
		builder.WithProjectDir(t.TempDir())

		bc := builder.Build(context.Background(), "cosca-test", nil, nil, nil, nil)
		if bc.SystemPrompt != "You are a test agent." {
			t.Errorf("SystemPrompt = %q, want unchanged agent prompt", bc.SystemPrompt)
		}
		if bc.UsagePct <= 0 {
			t.Errorf("expected positive usage percentage, got %f", bc.UsagePct)
		}
	})
}

func TestLoadAgentsMDChainOrdering(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("root"), 0644)
	os.WriteFile(filepath.Join(sub, "AGENTS.md"), []byte("deep"), 0644)

	chain := loadAgentsMDChain(sub)
	if len(chain) != 2 {
		t.Fatalf("expected 2 AGENTS.md entries, got %d", len(chain))
	}
	if !strings.HasSuffix(chain[0].Path, filepath.Join(root, "AGENTS.md")) {
		t.Errorf("chain[0] should be root AGENTS.md, got %q", chain[0].Path)
	}
	if !strings.HasSuffix(chain[1].Path, filepath.Join(sub, "AGENTS.md")) {
		t.Errorf("chain[1] should be deep AGENTS.md, got %q", chain[1].Path)
	}
	if chain[0].Content != "root" || chain[1].Content != "deep" {
		t.Errorf("unexpected chain contents: %+v", chain)
	}
}

// ─── Fix 3: Engine auto-compaction ─────────────────────────────────────────────

func TestCompactMessagesHelper(t *testing.T) {
	t.Run("compacts conversation but preserves system prompt and tail", func(t *testing.T) {
		e := &AgentEngine{autoCompaction: NewAutoCompaction(1000)}

		msgs := []chat.Message{
			{Role: chat.RoleSystem, Content: "SYSTEM PROMPT"},
			{Role: chat.RoleUser, Content: "Question 1"},
			{Role: chat.RoleAssistant, Content: "Answer 1"},
			{Role: chat.RoleUser, Content: "Question 2"},
			{Role: chat.RoleAssistant, Content: "Answer 2"},
			{Role: chat.RoleUser, Content: "Question 3"},
			{Role: chat.RoleAssistant, Content: "Answer 3"},
			{Role: chat.RoleUser, Content: "Question 4"},
			{Role: chat.RoleAssistant, Content: "Answer 4"},
			{Role: chat.RoleUser, Content: "Question 5"},
			{Role: chat.RoleAssistant, Content: "Answer 5"},
		}

		compacted, ok := e.compactMessages(msgs)
		if !ok {
			t.Fatal("expected compaction to happen")
		}
		if compacted[0].Role != chat.RoleSystem || compacted[0].Content != "SYSTEM PROMPT" {
			t.Errorf("system prompt must be preserved, got first message: %+v", compacted[0])
		}
		if len(compacted) >= len(msgs) {
			t.Errorf("compacted should be shorter, got %d >= %d", len(compacted), len(msgs))
		}
		last := compacted[len(compacted)-1]
		if last.Role != chat.RoleAssistant || last.Content != "Answer 5" {
			t.Errorf("last response should be preserved, got %+v", last)
		}
	})

	t.Run("nil autoCompaction returns original unchanged", func(t *testing.T) {
		e := &AgentEngine{}
		msgs := []chat.Message{{Role: chat.RoleUser, Content: "hi"}}
		out, ok := e.compactMessages(msgs)
		if ok {
			t.Error("expected no compaction when autoCompaction is nil")
		}
		if len(out) != len(msgs) {
			t.Errorf("expected unchanged messages, got %d", len(out))
		}
	})
}

func TestEngineAutoCompaction(t *testing.T) {
	t.Run("large history over threshold reduces turns", func(t *testing.T) {
		reg, _, rtr, prov, exec := setupEngineTest(t)

		// Small context window so a large history pushes UsagePct over the threshold.
		cb := NewContextBuilder(reg, 200)
		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)

		history := make([]chat.Message, 0, 40)
		for i := 0; i < 20; i++ {
			history = append(history,
				chat.Message{Role: chat.RoleUser, Content: fmt.Sprintf("User turn %d with some padding to consume context window tokens here", i)},
				chat.Message{Role: chat.RoleAssistant, Content: fmt.Sprintf("Assistant turn %d with some padding to consume context window tokens here", i)},
			)
		}

		var firstLen, secondLen int
		callCount := 0
		prov.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
			callCount++
			if callCount == 1 {
				firstLen = len(messages)
				return &chat.ChatResponse{
					Choices: []chat.Choice{{
						Message: chat.Message{
							Content: "using tool",
							ToolCalls: []chat.ToolCall{{
								ID: "call_1", Type: "function",
								Function: chat.FunctionCall{Name: "read_file", Arguments: `{"path":"x"}`},
							}},
						},
					}},
					Usage: chat.Usage{TotalTokens: 10},
				}, nil
			}
			secondLen = len(messages)
			return &chat.ChatResponse{
				Choices: []chat.Choice{{Message: chat.Message{Content: "final answer"}}},
				Usage:   chat.Usage{TotalTokens: 10},
			}, nil
		}
		exec.executeFunc = func(ctx context.Context, tc executor.ToolCall) (*executor.ToolResult, error) {
			return &executor.ToolResult{Status: executor.StatusSuccess, Output: "data"}, nil
		}

		result, err := e.Run(context.Background(), "big question", history)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if callCount < 2 {
			t.Fatal("expected at least 2 LLM calls")
		}
		if secondLen >= firstLen {
			t.Errorf("second call should have fewer messages after compaction, got %d >= %d", secondLen, firstLen)
		}
		// The system prompt must still be the first message after compaction.
		if result.Messages[0].Role != chat.RoleSystem {
			t.Errorf("first result message should be the system prompt, got %+v", result.Messages[0])
		}
	})

	t.Run("below threshold nothing changes", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)

		history := []chat.Message{
			{Role: chat.RoleUser, Content: "Question"},
			{Role: chat.RoleAssistant, Content: "Answer"},
		}

		var firstLen, secondLen int
		callCount := 0
		prov.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
			callCount++
			if callCount == 1 {
				firstLen = len(messages)
				return &chat.ChatResponse{
					Choices: []chat.Choice{{
						Message: chat.Message{
							Content: "using tool",
							ToolCalls: []chat.ToolCall{{
								ID: "call_1", Type: "function",
								Function: chat.FunctionCall{Name: "read_file", Arguments: `{"path":"x"}`},
							}},
						},
					}},
					Usage: chat.Usage{TotalTokens: 10},
				}, nil
			}
			secondLen = len(messages)
			return &chat.ChatResponse{
				Choices: []chat.Choice{{Message: chat.Message{Content: "final answer"}}},
				Usage:   chat.Usage{TotalTokens: 10},
			}, nil
		}
		exec.executeFunc = func(ctx context.Context, tc executor.ToolCall) (*executor.ToolResult, error) {
			return &executor.ToolResult{Status: executor.StatusSuccess, Output: "data"}, nil
		}

		_, err := e.Run(context.Background(), "hello", history)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// No compaction: the second call simply appends the assistant tool-call
		// message plus the tool result on top of the first call's messages.
		if secondLen != firstLen+2 {
			t.Errorf("expected second call to be firstLen+2 (no compaction), got %d vs %d", secondLen, firstLen)
		}
	})
}
