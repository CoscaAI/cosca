package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/chat/executor"
)

func setupSubagentTest(t *testing.T) (*AgentRegistry, *mockProvider, *mockExecutor) {
	t.Helper()
	dir := t.TempDir()

	writeAgentFile(t, dir, "sub-agent.md",
		"name: cosca-subagent\ncapabilities: [analysis]\ndescription: A test subagent\ntemperature: 0.5",
		"You are a subagent. Analyze the task carefully.")

	writeAgentFile(t, dir, "no-temp-agent.md",
		"name: cosca-notemp\ncapabilities: [test]",
		"You are an agent with default temperature.")

	registry := NewAgentRegistry(dir)
	provider := newMockProvider()
	executor := newMockExecutor()
	return registry, provider, executor
}

func TestSpawn(t *testing.T) {
	t.Run("returns subagent result with summary", func(t *testing.T) {
		registry, provider, exec := setupSubagentTest(t)
		spawner := NewSubagentSpawner(registry, provider, exec)

		provider.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
			// Verify the messages include system prompt and user task
			if len(messages) < 2 {
				t.Fatalf("expected at least 2 messages, got %d", len(messages))
			}
			if messages[0].Role != chat.RoleSystem {
				t.Errorf("first message role = %q, want system", messages[0].Role)
			}
			if !contains(messages[0].Content, "subagent") {
				t.Errorf("system prompt should mention subagent, got %q", messages[0].Content)
			}
			if messages[1].Role != chat.RoleUser {
				t.Errorf("second message role = %q, want user", messages[1].Role)
			}
			if messages[1].Content != "Analyze this code" {
				t.Errorf("task = %q, want %q", messages[1].Content, "Analyze this code")
			}

			return &chat.ChatResponse{
				Choices: []chat.Choice{
					{Message: chat.Message{Content: "Analysis complete: this code is clean."}},
				},
				Usage: chat.Usage{PromptTokens: 50, CompletionTokens: 30, TotalTokens: 80},
			}, nil
		}

		result, err := spawner.Spawn(context.Background(), SubagentRequest{
			Agent: "cosca-subagent",
			Task:  "Analyze this code",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error != "" {
			t.Fatalf("unexpected error in result: %s", result.Error)
		}
		if result.Summary != "Analysis complete: this code is clean." {
			t.Errorf("Summary = %q", result.Summary)
		}
		if result.TokenUsage.TotalTokens != 80 {
			t.Errorf("TokenUsage.TotalTokens = %d, want 80", result.TokenUsage.TotalTokens)
		}
	})

	t.Run("includes context in task message", func(t *testing.T) {
		registry, provider, exec := setupSubagentTest(t)
		spawner := NewSubagentSpawner(registry, provider, exec)

		provider.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
			if len(messages) < 2 {
				t.Fatalf("expected at least 2 messages")
			}
			userMsg := messages[1]
			if !contains(userMsg.Content, "## Context") {
				t.Error("expected context section in user message")
			}
			if !contains(userMsg.Content, "file.go") {
				t.Error("expected context key in user message")
			}
			return &chat.ChatResponse{
				Choices: []chat.Choice{{Message: chat.Message{Content: "OK"}}},
			}, nil
		}

		result, err := spawner.Spawn(context.Background(), SubagentRequest{
			Agent: "cosca-subagent",
			Task:  "Review this file",
			Context: map[string]string{
				"file":    "file.go",
				"project": "cosca",
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error != "" {
			t.Errorf("unexpected error: %s", result.Error)
		}
	})

	t.Run("handles unknown agent", func(t *testing.T) {
		registry, provider, exec := setupSubagentTest(t)
		spawner := NewSubagentSpawner(registry, provider, exec)

		result, err := spawner.Spawn(context.Background(), SubagentRequest{
			Agent: "nonexistent-agent",
			Task:  "Do something",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error == "" {
			t.Error("expected error for unknown agent")
		}
		if !contains(result.Error, "not found") {
			t.Errorf("error message should mention 'not found', got: %s", result.Error)
		}
	})

	t.Run("handles provider error gracefully", func(t *testing.T) {
		registry, provider, exec := setupSubagentTest(t)
		spawner := NewSubagentSpawner(registry, provider, exec)

		provider.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
			return nil, errors.New("provider unavailable")
		}

		result, err := spawner.Spawn(context.Background(), SubagentRequest{
			Agent: "cosca-subagent",
			Task:  "Do something",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error == "" {
			t.Error("expected error in result for provider failure")
		}
		if contains(result.Error, "provider unavailable") || !contains(result.Error, "subagent_call_failed") {
			t.Errorf("error should be safely categorized, got: %s", result.Error)
		}
	})

	t.Run("handles empty response choices", func(t *testing.T) {
		registry, provider, exec := setupSubagentTest(t)
		spawner := NewSubagentSpawner(registry, provider, exec)

		provider.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
			return &chat.ChatResponse{Choices: []chat.Choice{}}, nil
		}

		result, err := spawner.Spawn(context.Background(), SubagentRequest{
			Agent: "cosca-subagent",
			Task:  "Do something",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Summary != "" {
			t.Errorf("expected empty summary for no choices, got %q", result.Summary)
		}
	})

	t.Run("uses agent temperature when set", func(t *testing.T) {
		registry, provider, exec := setupSubagentTest(t)
		spawner := NewSubagentSpawner(registry, provider, exec)

		provider.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
			if opts.Temperature != 0.5 {
				t.Errorf("Temperature = %f, want 0.5", opts.Temperature)
			}
			return &chat.ChatResponse{
				Choices: []chat.Choice{{Message: chat.Message{Content: "done"}}},
			}, nil
		}

		_, err := spawner.Spawn(context.Background(), SubagentRequest{
			Agent: "cosca-subagent",
			Task:  "test",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("uses default temperature when agent has none", func(t *testing.T) {
		registry, provider, exec := setupSubagentTest(t)
		spawner := NewSubagentSpawner(registry, provider, exec)

		provider.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
			if opts.Temperature != 0.7 {
				t.Errorf("Temperature = %f, want 0.7", opts.Temperature)
			}
			return &chat.ChatResponse{
				Choices: []chat.Choice{{Message: chat.Message{Content: "done"}}},
			}, nil
		}

		_, err := spawner.Spawn(context.Background(), SubagentRequest{
			Agent: "cosca-notemp",
			Task:  "test",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestSpawnWithTools(t *testing.T) {
	t.Run("spawns with limited tool access", func(t *testing.T) {
		registry, provider, exec := setupSubagentTest(t)
		spawner := NewSubagentSpawner(registry, provider, exec)

		provider.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
			if len(opts.Tools) > 0 {
				// First call has tools
				return &chat.ChatResponse{
					Choices: []chat.Choice{
						{
							Message: chat.Message{
								Content: "Let me read the file...",
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
					Usage: chat.Usage{PromptTokens: 100, CompletionTokens: 20, TotalTokens: 120},
				}, nil
			}
			// Follow-up call without tools
			return &chat.ChatResponse{
				Choices: []chat.Choice{
					{Message: chat.Message{Content: "Final analysis after tool use."}},
				},
				Usage: chat.Usage{PromptTokens: 50, CompletionTokens: 30, TotalTokens: 80},
			}, nil
		}

		exec.executeFunc = func(ctx context.Context, tc executor.ToolCall) (*executor.ToolResult, error) {
			if tc.Name == "read_file" {
				return &executor.ToolResult{
					ToolCallID: tc.ID,
					Status:     executor.StatusSuccess,
					Output:     "file contents",
				}, nil
			}
			return &executor.ToolResult{
				Status: executor.StatusError,
				Error:  "unknown tool",
			}, nil
		}

		tools := []chat.Tool{
			newMockTool("read_file", "Read a file"),
		}

		result, err := spawner.SpawnWithTools(context.Background(), SubagentRequest{
			Agent: "cosca-subagent",
			Task:  "Read the test file and analyze",
		}, tools)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error != "" {
			t.Fatalf("unexpected error in result: %s", result.Error)
		}
		if result.Summary != "Final analysis after tool use." {
			t.Errorf("Summary = %q, want %q", result.Summary, "Final analysis after tool use.")
		}
		// Total tokens should include both calls: 120 + 80 = 200
		if result.TokenUsage.TotalTokens != 200 {
			t.Errorf("TotalTokens = %d, want 200", result.TokenUsage.TotalTokens)
		}
	})

	t.Run("handles tool execution error", func(t *testing.T) {
		registry, provider, exec := setupSubagentTest(t)
		spawner := NewSubagentSpawner(registry, provider, exec)

		provider.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
			if len(opts.Tools) > 0 {
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
											Arguments: `{"path":"test.txt"}`,
										},
									},
								},
							},
						},
					},
				}, nil
			}
			return &chat.ChatResponse{
				Choices: []chat.Choice{{Message: chat.Message{Content: "Final response"}}},
			}, nil
		}

		exec.executeFunc = func(ctx context.Context, tc executor.ToolCall) (*executor.ToolResult, error) {
			return &executor.ToolResult{
				ToolCallID: tc.ID,
				Status:     executor.StatusError,
				Error:      "file not found",
			}, nil
		}

		result, err := spawner.SpawnWithTools(context.Background(), SubagentRequest{
			Agent: "cosca-subagent",
			Task:  "Read file",
		}, []chat.Tool{newMockTool("read_file", "Read file")})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error != "" {
			t.Errorf("unexpected error: %s", result.Error)
		}
	})

	t.Run("handles provider error gracefully", func(t *testing.T) {
		registry, provider, exec := setupSubagentTest(t)
		spawner := NewSubagentSpawner(registry, provider, exec)

		provider.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
			return nil, errors.New("provider error")
		}

		result, err := spawner.SpawnWithTools(context.Background(), SubagentRequest{
			Agent: "cosca-subagent",
			Task:  "test",
		}, []chat.Tool{newMockTool("read_file", "Read file")})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error == "" {
			t.Error("expected error in result for provider failure")
		}
	})

	t.Run("handles unknown agent", func(t *testing.T) {
		registry, provider, exec := setupSubagentTest(t)
		spawner := NewSubagentSpawner(registry, provider, exec)

		result, err := spawner.SpawnWithTools(context.Background(), SubagentRequest{
			Agent: "nonexistent",
			Task:  "test",
		}, []chat.Tool{newMockTool("read_file", "Read file")})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error == "" {
			t.Error("expected error for unknown agent")
		}
	})

	t.Run("no tool calls, no follow-up", func(t *testing.T) {
		registry, provider, exec := setupSubagentTest(t)
		spawner := NewSubagentSpawner(registry, provider, exec)

		callCount := 0
		provider.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
			callCount++
			if callCount > 1 {
				t.Error("should not make follow-up call when no tool calls")
			}
			return &chat.ChatResponse{
				Choices: []chat.Choice{
					{Message: chat.Message{Content: "Direct analysis without tools."}},
				},
				Usage: chat.Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
			}, nil
		}

		result, err := spawner.SpawnWithTools(context.Background(), SubagentRequest{
			Agent: "cosca-subagent",
			Task:  "Analyze directly",
		}, []chat.Tool{newMockTool("read_file", "Read file")})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error != "" {
			t.Errorf("unexpected error: %s", result.Error)
		}
		if result.Summary != "Direct analysis without tools." {
			t.Errorf("Summary = %q", result.Summary)
		}
		if result.TokenUsage.TotalTokens != 30 {
			t.Errorf("TotalTokens = %d, want 30", result.TokenUsage.TotalTokens)
		}
	})

	t.Run("schemaToMap handles empty and invalid schema", func(t *testing.T) {
		registry, provider, exec := setupSubagentTest(t)
		spawner := NewSubagentSpawner(registry, provider, exec)

		// Empty schema
		emptyTool := newMockTool("empty_tool", "Tool with empty schema")
		emptyTool.schema = nil

		// Invalid schema
		invalidTool := newMockTool("invalid_tool", "Tool with invalid schema")
		invalidTool.schema = []byte(`{invalid json}`)

		provider.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
			// Just check tool defs were created correctly
			return &chat.ChatResponse{
				Choices: []chat.Choice{{Message: chat.Message{Content: "done"}}},
				Usage:   chat.Usage{TotalTokens: 10},
			}, nil
		}

		result, err := spawner.SpawnWithTools(context.Background(), SubagentRequest{
			Agent: "cosca-subagent",
			Task:  "test",
		}, []chat.Tool{emptyTool, invalidTool})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error != "" {
			t.Errorf("unexpected error: %s", result.Error)
		}
	})

	t.Run("formatToolOutput handles nil result and nil output", func(t *testing.T) {
		spawner := &SubagentSpawner{}

		// nil result
		if s := spawner.formatToolOutput(nil); s != "No result returned." {
			t.Errorf("nil result: %q", s)
		}

		// result with nil output
		nilOutput := &executor.ToolResult{Status: executor.StatusSuccess}
		if s := spawner.formatToolOutput(nilOutput); s != "OK" {
			t.Errorf("nil output: %q", s)
		}

		// result with non-string, non-map output (e.g. int)
		intOutput := &executor.ToolResult{Status: executor.StatusSuccess, Output: 42}
		if s := spawner.formatToolOutput(intOutput); s != "42" {
			t.Errorf("int output: %q", s)
		}
	})

	t.Run("chatToolCallToExec handles invalid JSON", func(t *testing.T) {
		spawner := &SubagentSpawner{}
		tc := chat.ToolCall{
			ID:   "call_1",
			Type: "function",
			Function: chat.FunctionCall{
				Name:      "read_file",
				Arguments: `{invalid json}`,
			},
		}
		_, err := spawner.chatToolCallToExec(tc)
		if err == nil {
			t.Error("expected error for invalid JSON arguments")
		}
	})

	t.Run("follow-up call failure returns partial result", func(t *testing.T) {
		registry, provider, exec := setupSubagentTest(t)
		spawner := NewSubagentSpawner(registry, provider, exec)

		callIdx := 0
		provider.chatFunc = func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
			callIdx++
			if callIdx == 1 {
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
											Arguments: `{"path":"test.txt"}`,
										},
									},
								},
							},
						},
					},
					Usage: chat.Usage{TotalTokens: 50},
				}, nil
			}
			return nil, errors.New("follow-up failed")
		}

		exec.executeFunc = func(ctx context.Context, tc executor.ToolCall) (*executor.ToolResult, error) {
			return &executor.ToolResult{Status: executor.StatusSuccess, Output: "data"}, nil
		}

		result, err := spawner.SpawnWithTools(context.Background(), SubagentRequest{
			Agent: "cosca-subagent",
			Task:  "test",
		}, []chat.Tool{newMockTool("read_file", "Read file")})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error == "" {
			t.Error("expected error for follow-up failure")
		}
		if contains(result.Error, "follow-up failed") || !contains(result.Error, "subagent_followup_failed") {
			t.Errorf("error should be safely categorized, got: %s", result.Error)
		}
	})
}
