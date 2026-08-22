package engine

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/chat/executor"
)

// ─── Mock Provider ─────────────────────────────────────────────────────────────

// mockProvider implements ProviderChat for testing.
type mockProvider struct {
	chatFunc       func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error)
	chatStreamFunc func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (chat.ChatStream, error)
	name           string
}

func newMockProvider() *mockProvider {
	return &mockProvider{
		name: "test-provider",
	}
}

func (m *mockProvider) Chat(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
	if m.chatFunc != nil {
		return m.chatFunc(ctx, messages, opts)
	}
	return &chat.ChatResponse{
		Choices: []chat.Choice{
			{
				Message: chat.Message{
					Role:    chat.RoleAssistant,
					Content: "mock response",
				},
			},
		},
		Usage: chat.Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
	}, nil
}

func (m *mockProvider) ChatStream(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (chat.ChatStream, error) {
	if m.chatStreamFunc != nil {
		return m.chatStreamFunc(ctx, messages, opts)
	}
	return newMockStream("mock stream response", nil), nil
}

func (m *mockProvider) Name() string {
	return m.name
}

// ─── Mock Stream ───────────────────────────────────────────────────────────────

// mockStream implements chat.ChatStream for testing.
type mockStream struct {
	chunks []chat.ChatStreamChunk
	idx    int
	closed bool
	err    error
}

func newMockStream(content string, toolCalls []chat.ToolCall) *mockStream {
	chunks := []chat.ChatStreamChunk{
		{
			Choices: []chat.StreamChoice{
				{
					Delta: chat.Message{
						Content:   content,
						ToolCalls: toolCalls,
					},
					FinishReason: chat.FinishReasonStop,
				},
			},
		},
	}
	return &mockStream{chunks: chunks}
}

func newMockStreamWithChunks(chunks []chat.ChatStreamChunk) *mockStream {
	return &mockStream{chunks: chunks}
}

func (s *mockStream) Recv() (*chat.ChatStreamChunk, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.idx >= len(s.chunks) {
		return nil, nil // stream ended
	}
	chunk := s.chunks[s.idx]
	s.idx++
	return &chunk, nil
}

func (s *mockStream) Close() error {
	s.closed = true
	return nil
}

// ─── Mock Executor ─────────────────────────────────────────────────────────────

// mockExecutor implements ToolExecutor for testing.
type mockExecutor struct {
	executeFunc      func(ctx context.Context, toolCall executor.ToolCall) (*executor.ToolResult, error)
	executeBatchFunc func(ctx context.Context, toolCalls []executor.ToolCall, opts ...executor.BatchOption) []*executor.ToolResult
	listToolsFunc    func(ctx context.Context) ([]chat.ToolDefinition, error)
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{}
}

func (m *mockExecutor) Execute(ctx context.Context, toolCall executor.ToolCall) (*executor.ToolResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, toolCall)
	}
	return &executor.ToolResult{
		ToolCallID: toolCall.ID,
		Status:     executor.StatusSuccess,
		Output:     fmt.Sprintf("executed %s", toolCall.Name),
	}, nil
}

func (m *mockExecutor) ExecuteBatch(ctx context.Context, toolCalls []executor.ToolCall, opts ...executor.BatchOption) []*executor.ToolResult {
	if m.executeBatchFunc != nil {
		return m.executeBatchFunc(ctx, toolCalls, opts...)
	}
	results := make([]*executor.ToolResult, len(toolCalls))
	for i, tc := range toolCalls {
		results[i] = &executor.ToolResult{
			ToolCallID: tc.ID,
			Status:     executor.StatusSuccess,
			Output:     fmt.Sprintf("executed %s", tc.Name),
		}
	}
	return results
}

func (m *mockExecutor) ListTools(ctx context.Context) ([]chat.ToolDefinition, error) {
	if m.listToolsFunc != nil {
		return m.listToolsFunc(ctx)
	}
	return []chat.ToolDefinition{}, nil
}

// ─── Mock Tool ─────────────────────────────────────────────────────────────────

// mockTool implements chat.Tool for testing.
type mockTool struct {
	name        string
	description string
	schema      json.RawMessage
	executeFunc func(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error)
}

func newMockTool(name, description string) *mockTool {
	return &mockTool{
		name:        name,
		description: description,
		schema:      json.RawMessage(`{"type":"object","properties":{}}`),
	}
}

func (m *mockTool) Name() string                          { return m.name }
func (m *mockTool) Description() string                   { return m.description }
func (m *mockTool) Schema() json.RawMessage               { return m.schema }
func (m *mockTool) Validate(params json.RawMessage) error { return nil }
func (m *mockTool) Execute(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, params)
	}
	return &chat.ToolResult{Output: m.name + " executed"}, nil
}
