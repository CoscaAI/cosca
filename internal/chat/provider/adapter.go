// Package provider provides chat.Provider adapters for various LLM backends.
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/CoscaAI/cosca/internal/chat"
)

// ─── ProviderAdapter ──────────────────────────────────────────────────────────

// ProviderAdapter wraps a chat.Provider (from ports.go) to implement the
// chat.ChatProvider interface (from types.go) that the engine requires.
//
// The adapter bridges the two provider interfaces:
//   - chat.Provider.Chat takes ChatRequest and returns (<-chan ChatEvent, error)
//   - chat.ChatProvider.Chat takes messages + opts and returns (*ChatResponse, error)
//   - chat.ChatProvider.ChatStream takes messages + opts and returns (ChatStream, error)
type ProviderAdapter struct {
	provider chat.Provider
	model    string
}

// NewProviderAdapter creates a new adapter that wraps the given provider.
// If model is empty, the provider's default model is used.
func NewProviderAdapter(p chat.Provider, model string) *ProviderAdapter {
	return &ProviderAdapter{
		provider: p,
		model:    model,
	}
}

// Name returns the underlying provider's name.
func (a *ProviderAdapter) Name() string {
	return a.provider.Name()
}

// Model returns the model identifier.
func (a *ProviderAdapter) Model() string {
	models := a.provider.Models()
	if a.model != "" {
		return a.model
	}
	if len(models) > 0 {
		return models[0]
	}
	return "unknown"
}

// Close releases provider resources (no-op for HTTP-based providers).
func (a *ProviderAdapter) Close() error {
	return nil
}

// Chat sends a synchronous chat completion request and returns the full response.
// It collects streaming events into a single ChatResponse.
func (a *ProviderAdapter) Chat(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
	tools := convertToolDefs(opts.Tools)

	req := chat.ChatRequest{
		Model:    a.model,
		Messages: messages,
		Tools:    tools,
		Stream:   false,
	}

	eventCh, err := a.provider.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("provider %q: %w", a.provider.Name(), err)
	}

	resp, collectErr := collectNonStreamResponse(eventCh)
	if collectErr != nil {
		return nil, fmt.Errorf("provider %q: %w", a.provider.Name(), collectErr)
	}
	return resp, nil
}

// ChatStream initiates a streaming chat completion and returns a ChatStream
// that the caller can iterate over.
func (a *ProviderAdapter) ChatStream(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (chat.ChatStream, error) {
	// Convert tool definitions to Tool interface values.
	tools := convertToolDefs(opts.Tools)

	req := chat.ChatRequest{
		Model:    a.model,
		Messages: messages,
		Tools:    tools,
		Stream:   true,
	}

	eventCh, err := a.provider.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("provider %q: %w", a.provider.Name(), err)
	}

	return &eventChatStream{
		ch:    eventCh,
		mu:    sync.Mutex{},
		done:  false,
		model: a.model,
	}, nil
}

// ─── ChatStream adapter ───────────────────────────────────────────────────────

// eventChatStream wraps a <-chan chat.ChatEvent as a chat.ChatStream.
type eventChatStream struct {
	ch    <-chan chat.ChatEvent
	mu    sync.Mutex
	done  bool
	model string
}

// Recv returns the next chunk from the stream. It blocks until a new event
// arrives or the stream is closed. When the stream ends (done or error event),
// Recv returns a chunk with a non-empty FinishReason and no error. The second
// call returns nil, nil to signal stream exhaustion.
func (s *eventChatStream) Recv() (*chat.ChatStreamChunk, error) {
	s.mu.Lock()
	if s.done {
		s.mu.Unlock()
		return nil, nil
	}
	s.mu.Unlock()

	event, ok := <-s.ch
	if !ok {
		s.mu.Lock()
		s.done = true
		s.mu.Unlock()
		return nil, nil
	}

	switch event.Type {
	case chat.ChatEventDelta:
		return &chat.ChatStreamChunk{
			ID:    "",
			Model: s.model,
			Choices: []chat.StreamChoice{
				{
					Delta: chat.Message{
						Content: event.Delta,
					},
				},
			},
		}, nil

	case chat.ChatEventToolCall:
		// Forward tool-call parts as a stream delta so the executor's
		// tool-loop can merge and execute them (Vercel principle: never drop).
		return &chat.ChatStreamChunk{
			ID:    "",
			Model: s.model,
			Choices: []chat.StreamChoice{
				{
					Delta: chat.Message{
						ToolCalls: event.ToolCalls,
					},
				},
			},
		}, nil

	case chat.ChatEventDone:
		s.mu.Lock()
		s.done = true
		s.mu.Unlock()

		usage := event.Usage
		if usage == nil {
			usage = &chat.Usage{}
		}
		return &chat.ChatStreamChunk{
			ID:    "",
			Model: s.model,
			Choices: []chat.StreamChoice{
				{
					FinishReason: chat.FinishReasonStop,
				},
			},
			Usage: usage,
		}, nil

	case chat.ChatEventError:
		s.mu.Lock()
		s.done = true
		s.mu.Unlock()

		err := event.Error
		if err == nil {
			err = fmt.Errorf("unknown stream error")
		}
		return nil, err

	default:
		return s.Recv() // skip unknown event types
	}
}

// Close terminates the stream (no-op for channel-based streams).
func (s *eventChatStream) Close() error {
	s.mu.Lock()
	s.done = true
	s.mu.Unlock()
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// collectNonStreamResponse reads all events from a non-streaming provider call
// and assembles a single ChatResponse.
func collectNonStreamResponse(eventCh <-chan chat.ChatEvent) (*chat.ChatResponse, error) {
	resp := &chat.ChatResponse{
		Choices: []chat.Choice{
			{
				Message: chat.Message{},
			},
		},
		Usage: chat.Usage{},
	}

	var contentBuilder string
	var usage *chat.Usage
	var toolCalls []chat.ToolCall

	for event := range eventCh {
		switch event.Type {
		case chat.ChatEventDelta:
			contentBuilder += event.Delta
		case chat.ChatEventToolCall:
			// Tool calls are first-class parts (Vercel principle). Never drop
			// them — the orchestration tool-loop depends on them.
			toolCalls = append(toolCalls, event.ToolCalls...)
		case chat.ChatEventDone:
			usage = event.Usage
		case chat.ChatEventError:
			if event.Error != nil {
				return nil, fmt.Errorf("provider returned error event: %w", event.Error)
			}
		}
	}

	resp.Choices[0].Message.Content = contentBuilder
	if len(toolCalls) > 0 {
		resp.Choices[0].Message.ToolCalls = toolCalls
		resp.Choices[0].FinishReason = chat.FinishReasonToolCalls
	}
	if usage != nil {
		resp.Usage = *usage
	}
	return resp, nil
}

// convertToolDefs converts []chat.ToolDefinition to []chat.Tool for use with
// the chat.Provider interface. Each definition is wrapped in a toolDefAdapter.
func convertToolDefs(defs []chat.ToolDefinition) []chat.Tool {
	if len(defs) == 0 {
		return nil
	}
	tools := make([]chat.Tool, 0, len(defs))
	for _, d := range defs {
		tools = append(tools, &toolDefAdapter{def: d})
	}
	return tools
}

// toolDefAdapter wraps a chat.ToolDefinition to implement the chat.Tool interface.
// It provides the Name, Description, and Schema methods needed by providers to
// build the LLM function-calling request body. Execute and Validate return errors
// since tool definitions are schemas, not executable tool instances.
type toolDefAdapter struct {
	def chat.ToolDefinition
}

func (t *toolDefAdapter) Name() string        { return t.def.Function.Name }
func (t *toolDefAdapter) Description() string { return t.def.Function.Description }

func (t *toolDefAdapter) Schema() json.RawMessage {
	data, err := json.Marshal(t.def.Function.Parameters)
	if err != nil {
		return json.RawMessage(`{"type":"object"}`)
	}
	return data
}

func (t *toolDefAdapter) Execute(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
	return nil, fmt.Errorf("tool definition %q is not executable", t.def.Function.Name)
}

func (t *toolDefAdapter) Validate(params json.RawMessage) error {
	return fmt.Errorf("tool definition %q is not validatable", t.def.Function.Name)
}
