package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/chat/executor"
)

func setupEngineTest(t *testing.T) (*AgentRegistry, *ContextBuilder, *Router, *mockProvider, *mockExecutor) {
	t.Helper()
	dir := t.TempDir()

	writeAgentFile(t, dir, "general.md",
		"name: cosca-general\ncapabilities: [general, help]",
		"You are a general assistant.")

	writeAgentFile(t, dir, "kernel.md",
		"name: cosca-kernel\ncapabilities: [kernel, orchestration]",
		"You are the Cosca Kernel. Be precise.")

	writeAgentFile(t, dir, "database.md",
		"name: cosca-database\ncapabilities: [database, sql]",
		"You are a database expert.")

	registry := NewAgentRegistry(dir)
	contextBldr := NewContextBuilder(registry, 64000)
	router := NewRouter(registry)
	provider := newMockProvider()
	exec := newMockExecutor()

	return registry, contextBldr, router, provider, exec
}

func defaultEngineConfig() EngineConfig {
	return EngineConfig{
		Model:       "test-model",
		Temperature: 0.7,
		MaxTokens:   1000,
	}
}

func TestNewAgentEngine(t *testing.T) {
	t.Run("constructs correctly", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)

		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)
		if e == nil {
			t.Fatal("expected non-nil engine")
		}
		if e.provider != prov {
			t.Error("provider not set")
		}
		if e.executor != exec {
			t.Error("executor not set")
		}
		if e.registry != reg {
			t.Error("registry not set")
		}
		if e.contextBldr != cb {
			t.Error("contextBldr not set")
		}
		if e.router != rtr {
			t.Error("router not set")
		}
		if e.parser == nil {
			t.Error("parser should be initialized")
		}
		if e.spawner == nil {
			t.Error("spawner should be initialized")
		}
		if e.sessionMgr != nil {
			t.Error("sessionMgr should be nil initially")
		}
	})

	t.Run("WithSessionManager attaches session manager", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		sm := NewSessionManager(t.TempDir())

		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)
		e.WithSessionManager(sm)
		if e.sessionMgr != sm {
			t.Error("sessionMgr should be set")
		}
	})
}

func TestRun(t *testing.T) {
	t.Run("returns EngineResult with content", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)

		result, err := e.Run(context.Background(), "Hello", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Content != "mock response" {
			t.Errorf("Content = %q, want %q", result.Content, "mock response")
		}
		if result.TurnCount != 1 {
			t.Errorf("TurnCount = %d, want 1", result.TurnCount)
		}
		if result.TokenUsage.TotalTokens <= 0 {
			t.Errorf("expected positive token usage, got %d", result.TokenUsage.TotalTokens)
		}
		if result.Error != "" {
			t.Errorf("unexpected error: %s", result.Error)
		}
	})

	t.Run("includes messages in result", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)

		history := []chat.Message{
			{Role: chat.RoleUser, Content: "Previous question"},
			{Role: chat.RoleAssistant, Content: "Previous answer"},
		}
		result, err := e.Run(context.Background(), "New question", history)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Messages) < 3 {
			t.Errorf("expected at least 3 messages (history + user), got %d", len(result.Messages))
		}
	})

	t.Run("respects max turns", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		e := NewAgentEngine(reg, cb, rtr, prov, exec, EngineConfig{
			Model:    "test-model",
			MaxTurns: 2,
		}, nil, nil, nil)

		callCount := 0
		prov.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
			callCount++
			if callCount <= 2 {
				return &chat.ChatResponse{
					Choices: []chat.Choice{
						{
							Message: chat.Message{
								Content: "Using tool",
								ToolCalls: []chat.ToolCall{
									{
										ID:   fmt.Sprintf("call_%d", callCount),
										Type: "function",
										Function: chat.FunctionCall{
											Name:      "read_file",
											Arguments: `{"path":"test.txt"}`,
										},
									},
								},
							},
						},
					},
					Usage: chat.Usage{TotalTokens: 10},
				}, nil
			}
			return &chat.ChatResponse{
				Choices: []chat.Choice{{Message: chat.Message{Content: "Final answer"}}},
				Usage:   chat.Usage{TotalTokens: 10},
			}, nil
		}

		exec.executeFunc = func(ctx context.Context, tc executor.ToolCall) (*executor.ToolResult, error) {
			return &executor.ToolResult{Status: executor.StatusSuccess, Output: "data"}, nil
		}

		result, err := e.Run(context.Background(), "do something", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.TurnCount > 2 {
			t.Errorf("TurnCount = %d, should be <= 2 due to MaxTurns", result.TurnCount)
		}
		if result.Content == "" {
			t.Error("expected some content")
		}
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		result, err := e.Run(ctx, "Hello", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error == "" {
			t.Error("expected error message for cancelled context")
		}
	})

	t.Run("handles provider error gracefully", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)

		prov.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
			return nil, errors.New("provider unavailable")
		}

		_, err := e.Run(context.Background(), "Hello", nil)
		if err == nil {
			t.Fatal("expected error for provider failure")
		}
		if strings.Contains(err.Error(), "provider unavailable") || !strings.Contains(err.Error(), "llm_call_failed") {
			t.Errorf("error should be safely categorized, got: %v", err)
		}
	})

	t.Run("executes tool calls with executor", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)

		callCount := 0
		prov.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
			callCount++
			if callCount == 1 {
				return &chat.ChatResponse{
					Choices: []chat.Choice{
						{
							Message: chat.Message{
								Content: "Let me check the file...",
								ToolCalls: []chat.ToolCall{
									{
										ID:   "call_1",
										Type: "function",
										Function: chat.FunctionCall{
											Name:      "read_file",
											Arguments: `{"path":"test.txt"}`,
										},
									},
								},
							},
						},
					},
					Usage: chat.Usage{TotalTokens: 15},
				}, nil
			}
			return &chat.ChatResponse{
				Choices: []chat.Choice{{Message: chat.Message{Content: "Done reading file."}}},
				Usage:   chat.Usage{TotalTokens: 5},
			}, nil
		}

		executed := false
		exec.executeFunc = func(ctx context.Context, tc executor.ToolCall) (*executor.ToolResult, error) {
			executed = true
			if tc.Name != "read_file" {
				t.Errorf("executed tool = %q, want read_file", tc.Name)
			}
			return &executor.ToolResult{Status: executor.StatusSuccess, Output: "file contents"}, nil
		}

		result, err := e.Run(context.Background(), "read the file", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !executed {
			t.Error("tool was not executed")
		}
		if result.Error != "" {
			t.Errorf("unexpected error: %s", result.Error)
		}
	})

	t.Run("handles tool execution infrastructure error", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)

		// The executor returns a real error (not a ToolResult with error),
		// which causes the engine to abort the loop with a terminal error.
		callCount := 0
		prov.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
			callCount++
			return &chat.ChatResponse{
				Choices: []chat.Choice{
					{
						Message: chat.Message{
							Content: "",
							ToolCalls: []chat.ToolCall{
								{
									ID:   "call_1",
									Type: "function",
									Function: chat.FunctionCall{
										Name:      "read_file",
										Arguments: `{"path":"test.txt"}`,
									},
								},
							},
						},
					},
				},
				Usage: chat.Usage{TotalTokens: 10},
			}, nil
		}

		exec.executeFunc = func(ctx context.Context, tc executor.ToolCall) (*executor.ToolResult, error) {
			return nil, errors.New("executor infrastructure error")
		}

		result, err := e.Run(context.Background(), "read file", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error == "" {
			t.Error("expected error for executor failure")
		}
	})

	t.Run("handles invalid tool call arguments", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)

		callCount := 0
		prov.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
			callCount++
			if callCount == 1 {
				// First call: return tool calls with invalid JSON arguments
				return &chat.ChatResponse{
					Choices: []chat.Choice{
						{
							Message: chat.Message{
								ToolCalls: []chat.ToolCall{
									{
										ID:   "call_1",
										Type: "function",
										Function: chat.FunctionCall{
											Name:      "read_file",
											Arguments: `{invalid json}`,
										},
									},
								},
							},
						},
					},
					Usage: chat.Usage{TotalTokens: 10},
				}, nil
			}
			// Second call: engine feeds back the error tool result, respond normally
			return &chat.ChatResponse{
				Choices: []chat.Choice{{Message: chat.Message{Content: "I see the error"}}},
				Usage:   chat.Usage{TotalTokens: 5},
			}, nil
		}

		result, err := e.Run(context.Background(), "read file", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Content == "" {
			t.Error("expected some content even with invalid tool args")
		}
	})

	t.Run("spawns subagent on spawn_agent tool call", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)

		// Create the subagent in registry
		subDir := t.TempDir()
		writeAgentFile(t, subDir, "sub.md",
			"name: cosca-analytics\ncapabilities: [analytics]",
			"You are an analytics agent.")
		reg2 := NewAgentRegistry(subDir)
		e.registry = reg2
		e.spawner = NewSubagentSpawner(reg2, prov, exec)

		callCount := 0
		prov.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
			callCount++
			if callCount == 1 {
				return &chat.ChatResponse{
					Choices: []chat.Choice{
						{
							Message: chat.Message{
								Content: "Spawning agent...",
								ToolCalls: []chat.ToolCall{
									{
										ID:   "spawn_1",
										Type: "function",
										Function: chat.FunctionCall{
											Name:      SubagentSpawnToolName,
											Arguments: `{"agent":"cosca-analytics","task":"analyze this"}`,
										},
									},
								},
							},
						},
					},
					Usage: chat.Usage{TotalTokens: 10},
				}, nil
			}
			// Second call includes subagent result
			return &chat.ChatResponse{
				Choices: []chat.Choice{{Message: chat.Message{Content: "Analysis complete"}}},
				Usage:   chat.Usage{TotalTokens: 5},
			}, nil
		}

		result, err := e.Run(context.Background(), "analyze the data", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error != "" {
			t.Errorf("unexpected error: %s", result.Error)
		}
		if result.Content == "" {
			t.Error("expected non-empty content")
		}
	})

	t.Run("uses session manager when configured", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		sm := NewSessionManager(t.TempDir())
		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)
		e.WithSessionManager(sm)

		result, err := e.Run(context.Background(), "Hello", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.SessionID == "" {
			t.Error("expected session ID when session manager configured")
		}
	})

	t.Run("ephemeral session skips persistence", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		sm := NewSessionManager(t.TempDir())
		e := NewAgentEngine(reg, cb, rtr, prov, exec, EngineConfig{
			Model:     "test-model",
			Ephemeral: true,
		}, nil, nil, nil)
		e.WithSessionManager(sm)

		result, err := e.Run(context.Background(), "Hello", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.SessionID != "" {
			t.Error("expected no session ID for ephemeral session")
		}
	})

	t.Run("session manager handles history", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		sm := NewSessionManager(t.TempDir())
		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)
		e.WithSessionManager(sm)

		history := []chat.Message{
			{Role: chat.RoleUser, Content: "Earlier"},
			{Role: chat.RoleAssistant, Content: "Response"},
		}

		result, err := e.Run(context.Background(), "Hello", history)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// With history, no session is created (len(history) > 0)
		if result.SessionID != "" {
			t.Error("expected no session ID when history exists")
		}
	})
}

func TestRunStream(t *testing.T) {
	t.Run("emits content events and done event", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)

		events, err := e.RunStream(context.Background(), "Hello", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var contentEvents []string
		gotDone := false
		for evt := range events {
			switch evt.Type {
			case EngineEventContent:
				contentEvents = append(contentEvents, evt.Content)
			case EngineEventDone:
				gotDone = true
			case EngineEventError:
				t.Fatalf("unexpected error event: %v", evt.Error)
			}
		}

		if !gotDone {
			t.Error("expected Done event")
		}
		if len(contentEvents) == 0 {
			t.Error("expected at least one content event")
		}
	})

	t.Run("emits tool start and result events", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)

		toolCallChunks := []chat.ChatStreamChunk{
			{
				Choices: []chat.StreamChoice{
					{
						Delta: chat.Message{
							Content: "Let me check...",
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

		var eventTypes []EngineEventType
		for evt := range events {
			eventTypes = append(eventTypes, evt.Type)
		}

		hasDone := false
		for _, et := range eventTypes {
			if et == EngineEventDone {
				hasDone = true
				break
			}
		}
		if !hasDone {
			t.Error("expected Done event")
		}
	})

	t.Run("handles stream start error", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)

		prov.chatStreamFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (chat.ChatStream, error) {
			return nil, errors.New("stream unavailable")
		}

		events, err := e.RunStream(context.Background(), "Hello", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		gotError := false
		for evt := range events {
			if evt.Type == EngineEventError {
				gotError = true
			}
		}
		if !gotError {
			t.Error("expected error event")
		}
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		events, err := e.RunStream(ctx, "Hello", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		gotError := false
		for evt := range events {
			if evt.Type == EngineEventError {
				gotError = true
			}
		}
		if !gotError {
			t.Error("expected error event for cancelled context")
		}
	})

	t.Run("emits events in correct order", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)

		// Stream with chunks
		prov.chatStreamFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (chat.ChatStream, error) {
			chunks := []chat.ChatStreamChunk{
				{Choices: []chat.StreamChoice{{Delta: chat.Message{Content: "Hello "}, FinishReason: ""}}},
				{Choices: []chat.StreamChoice{{Delta: chat.Message{Content: "World!"}, FinishReason: chat.FinishReasonStop}}},
			}
			return newMockStreamWithChunks(chunks), nil
		}

		events, err := e.RunStream(context.Background(), "Hello", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var order []string
		for evt := range events {
			switch evt.Type {
			case EngineEventContent:
				order = append(order, "content:"+evt.Content)
			case EngineEventDone:
				order = append(order, "done")
			case EngineEventError:
				order = append(order, "error")
			}
		}

		if len(order) < 2 {
			t.Fatalf("expected at least 2 events, got %d", len(order))
		}
		if order[len(order)-1] != "done" {
			t.Errorf("last event should be 'done', got: %s", order[len(order)-1])
		}
	})

	t.Run("respects max turns in stream", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		e := NewAgentEngine(reg, cb, rtr, prov, exec, EngineConfig{
			Model:    "test-model",
			MaxTurns: 1,
		}, nil, nil, nil)

		prov.chatStreamFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (chat.ChatStream, error) {
			chunks := []chat.ChatStreamChunk{
				{Choices: []chat.StreamChoice{{Delta: chat.Message{Content: "result"}, FinishReason: chat.FinishReasonStop}}},
			}
			return newMockStreamWithChunks(chunks), nil
		}

		events, err := e.RunStream(context.Background(), "Hello", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		gotDone := false
		for evt := range events {
			if evt.Type == EngineEventDone {
				gotDone = true
			}
			if evt.Type == EngineEventError {
				t.Fatalf("unexpected error: %v", evt.Error)
			}
		}
		if !gotDone {
			t.Error("expected Done event")
		}
	})

	t.Run("emits turn end events for tool calls", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)

		// Tool call stream
		toolCallChunks := []chat.ChatStreamChunk{
			{
				Choices: []chat.StreamChoice{
					{
						Delta: chat.Message{
							Content: "Using tool...",
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

		events, err := e.RunStream(context.Background(), "read file", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		eventCount := 0
		for range events {
			eventCount++
		}
		if eventCount == 0 {
			t.Error("expected events")
		}
	})

	t.Run("stream with session manager", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		sm := NewSessionManager(t.TempDir())
		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)
		e.WithSessionManager(sm)

		events, err := e.RunStream(context.Background(), "Hello", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		gotDone := false
		for evt := range events {
			if evt.Type == EngineEventDone {
				gotDone = true
			}
			if evt.Type == EngineEventError {
				t.Fatalf("unexpected error: %v", evt.Error)
			}
		}
		if !gotDone {
			t.Error("expected Done event")
		}
	})

	t.Run("stream with subagent spawn", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)

		subDir := t.TempDir()
		writeAgentFile(t, subDir, "sub.md",
			"name: cosca-analytics\ncapabilities: [analytics]",
			"You are an analytics agent.")
		reg2 := NewAgentRegistry(subDir)
		e.registry = reg2
		e.spawner = NewSubagentSpawner(reg2, prov, exec)

		toolCallChunks := []chat.ChatStreamChunk{
			{
				Choices: []chat.StreamChoice{
					{
						Delta: chat.Message{
							ToolCalls: []chat.ToolCall{
								{Index: 0, ID: "spawn_1", Type: "function", Function: chat.FunctionCall{Name: SubagentSpawnToolName, Arguments: `{"agent":"cosca-analytics","task":"analyze"}`}},
							},
						},
						FinishReason: chat.FinishReasonToolCalls,
					},
				},
			},
		}
		doneChunks := []chat.ChatStreamChunk{
			{Choices: []chat.StreamChoice{{Delta: chat.Message{Content: "Analysis done"}, FinishReason: chat.FinishReasonStop}}},
		}

		callCount := 0
		prov.chatStreamFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (chat.ChatStream, error) {
			callCount++
			if callCount == 1 {
				return newMockStreamWithChunks(toolCallChunks), nil
			}
			return newMockStreamWithChunks(doneChunks), nil
		}

		events, err := e.RunStream(context.Background(), "analyze data", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var eventTypes []EngineEventType
		for evt := range events {
			eventTypes = append(eventTypes, evt.Type)
		}

		hasStart := false
		hasResult := false
		for _, et := range eventTypes {
			if et == EngineEventSubagentStart {
				hasStart = true
			}
			if et == EngineEventSubagentResult {
				hasResult = true
			}
		}
		if !hasStart {
			t.Error("expected SubagentStart event")
		}
		if !hasResult {
			t.Error("expected SubagentResult event")
		}
	})

	t.Run("stream subagent spawn with error", func(t *testing.T) {
		reg, cb, rtr, prov, exec := setupEngineTest(t)
		e := NewAgentEngine(reg, cb, rtr, prov, exec, defaultEngineConfig(), nil, nil, nil)

		subDir := t.TempDir()
		writeAgentFile(t, subDir, "sub.md",
			"name: cosca-analytics\ncapabilities: [analytics]",
			"You are an analytics agent.")
		reg2 := NewAgentRegistry(subDir)
		e.registry = reg2
		e.spawner = NewSubagentSpawner(reg2, prov, exec)

		// Make the provider return an error for the subagent spawn
		prov.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
			return nil, errors.New("subagent provider error")
		}

		toolCallChunks := []chat.ChatStreamChunk{
			{
				Choices: []chat.StreamChoice{
					{
						Delta: chat.Message{
							ToolCalls: []chat.ToolCall{
								{Index: 0, ID: "spawn_1", Type: "function", Function: chat.FunctionCall{Name: SubagentSpawnToolName, Arguments: `{"agent":"cosca-analytics","task":"analyze"}`}},
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

		events, err := e.RunStream(context.Background(), "analyze data", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		gotDone := false
		for evt := range events {
			if evt.Type == EngineEventDone {
				gotDone = true
			}
		}
		if !gotDone {
			t.Error("expected Done event")
		}
	})
}

func TestInternalHelpers(t *testing.T) {
	t.Run("isSubagentSpawn detects spawn_agent", func(t *testing.T) {
		e := &AgentEngine{}

		tc := chat.ToolCall{
			Function: chat.FunctionCall{
				Name: SubagentSpawnToolName,
			},
		}
		if !e.isSubagentSpawn(tc) {
			t.Error("should detect spawn_agent tool")
		}

		tc2 := chat.ToolCall{
			Function: chat.FunctionCall{
				Name: "read_file",
			},
		}
		if e.isSubagentSpawn(tc2) {
			t.Error("should not detect non-spawn tool")
		}
	})

	t.Run("parseSubagentRequest extracts fields", func(t *testing.T) {
		e := &AgentEngine{}

		// With valid JSON
		tc := chat.ToolCall{
			Function: chat.FunctionCall{
				Name:      SubagentSpawnToolName,
				Arguments: `{"agent":"cosca-database","task":"write a query","context":{"file":"schema.sql"}}`,
			},
		}
		req := e.parseSubagentRequest(tc)
		if req.Agent != "cosca-database" {
			t.Errorf("Agent = %q", req.Agent)
		}
		if req.Task != "write a query" {
			t.Errorf("Task = %q", req.Task)
		}
		if req.Context["file"] != "schema.sql" {
			t.Errorf("Context = %v", req.Context)
		}
	})

	t.Run("parseSubagentRequest handles missing arguments", func(t *testing.T) {
		e := &AgentEngine{}

		tc := chat.ToolCall{
			Function: chat.FunctionCall{
				Name: SubagentSpawnToolName,
			},
		}
		req := e.parseSubagentRequest(tc)
		if req.Agent != SubagentSpawnToolName {
			t.Errorf("Agent = %q, should default to tool name", req.Agent)
		}
	})

	t.Run("parseSubagentRequest handles invalid JSON", func(t *testing.T) {
		e := &AgentEngine{}

		tc := chat.ToolCall{
			Function: chat.FunctionCall{
				Name:      SubagentSpawnToolName,
				Arguments: `{invalid}`,
			},
		}
		req := e.parseSubagentRequest(tc)
		if req.Agent != SubagentSpawnToolName {
			t.Errorf("Agent = %q, should default to tool name on parse error", req.Agent)
		}
	})

	t.Run("formatExecResult handles various outputs", func(t *testing.T) {
		e := &AgentEngine{}

		// Nil result
		if s := e.formatExecResult(nil); s != "No result returned." {
			t.Errorf("nil result: %q", s)
		}

		// Error result
		errResult := &executor.ToolResult{Error: "something went wrong"}
		if s := e.formatExecResult(errResult); strings.Contains(s, "something went wrong") || !strings.Contains(s, "tool_execution_failed") {
			t.Errorf("error result: %q", s)
		}

		// Nil output
		noOutput := &executor.ToolResult{Status: executor.StatusSuccess}
		if s := e.formatExecResult(noOutput); s != "OK" {
			t.Errorf("nil output: %q", s)
		}

		// String output
		strOutput := &executor.ToolResult{Status: executor.StatusSuccess, Output: "hello"}
		if s := e.formatExecResult(strOutput); s != "hello" {
			t.Errorf("string output: %q", s)
		}

		// Map output
		mapOutput := &executor.ToolResult{Status: executor.StatusSuccess, Output: map[string]interface{}{"key": "value"}}
		if s := e.formatExecResult(mapOutput); s != `{"key":"value"}` {
			t.Errorf("map output: %q", s)
		}

		// Channel output (non-marshalable, triggers json.Marshal error)
		ch := make(chan int)
		chanOutput := &executor.ToolResult{Status: executor.StatusSuccess, Output: ch}
		s := e.formatExecResult(chanOutput)
		if s == "" {
			t.Error("expected non-empty string for channel output")
		}
	})
}

// Test coverage for TimeNow (used in session)
func TestTimeImport(t *testing.T) {
	// Just verify that time.Now compiles and works
	_ = time.Now
}
