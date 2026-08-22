package engine

import (
	"context"
	"fmt"
	"sort"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/safeerror"
)

// ─── Parse Event Types ─────────────────────────────────────────────────────────

// ParseEventType categorises a parse event from streaming response parsing.
type ParseEventType string

const (
	// ParseEventContent carries a text delta token from the stream.
	ParseEventContent ParseEventType = "content"
	// ParseEventToolCall carries a completed tool call after all deltas merged.
	ParseEventToolCall ParseEventType = "tool_call"
	// ParseEventDone signals that the stream has completed successfully.
	ParseEventDone ParseEventType = "done"
	// ParseEventError signals that the stream encountered an error.
	ParseEventError ParseEventType = "error"
)

// ParseEvent represents a single event emitted by ParseStream.
type ParseEvent struct {
	Type     ParseEventType
	Content  string         // text delta (for ParseEventContent)
	ToolCall *chat.ToolCall // completed tool call (for ParseEventToolCall)
	Error    error          // stream error (for ParseEventError)
}

// ToolCallDelta holds partial tool call data accumulated across streaming chunks.
// Multiple deltas with the same Index are merged into a single chat.ToolCall
// when the stream signals completion (FinishReason == "tool_calls").
type ToolCallDelta struct {
	Index     int
	ID        string
	Name      string
	Arguments string
}

// ─── ResponseParser ────────────────────────────────────────────────────────────

// ResponseParser parses streaming LLM responses into content deltas and tool
// call events. It handles both streaming (ParseStream) and non-streaming
// (ParseResponse) response formats.
type ResponseParser struct{}

// NewResponseParser creates a new ResponseParser.
func NewResponseParser() *ResponseParser {
	return &ResponseParser{}
}

// ParseStream consumes a streaming chat response and emits ParseEvents on the
// returned channel. The caller must read from the channel until it is closed.
//
// The parser accumulates tool call deltas across chunks by their Index field.
// When FinishReason == "tool_calls" is received, all accumulated deltas are
// merged into completed chat.ToolCall values and emitted as ParseEventToolCall
// events. Content deltas are emitted immediately as ParseEventContent.
func (p *ResponseParser) ParseStream(
	ctx context.Context,
	stream chat.ChatStream,
	toolDefs []chat.ToolDefinition,
) (<-chan ParseEvent, error) {
	events := make(chan ParseEvent)

	go func() {
		defer close(events)
		defer stream.Close()

		deltas := make(map[int]*ToolCallDelta)
		var contentBuilder contentAccumulator

		for {
			select {
			case <-ctx.Done():
				events <- ParseEvent{Type: ParseEventError, Error: safeerror.Error("stream_cancelled", ctx.Err())}
				return
			default:
			}

			chunk, err := stream.Recv()
			if err != nil {
				events <- ParseEvent{
					Type:  ParseEventError,
					Error: safeerror.Error("stream_receive_failed", err),
				}
				return
			}
			if chunk == nil {
				// Stream ended without an explicit finish reason.
				p.flushToolCalls(deltas, events)
				events <- ParseEvent{Type: ParseEventDone}
				return
			}

			for _, choice := range chunk.Choices {
				// Content delta.
				if choice.Delta.Content != "" {
					contentBuilder.Add(choice.Delta.Content)
					events <- ParseEvent{
						Type:    ParseEventContent,
						Content: choice.Delta.Content,
					}
				}

				// Tool call deltas — accumulate by index.
				for _, tc := range choice.Delta.ToolCalls {
					delta, exists := deltas[tc.Index]
					if !exists {
						delta = &ToolCallDelta{Index: tc.Index}
						deltas[tc.Index] = delta
					}
					if tc.ID != "" {
						delta.ID = tc.ID
					}
					if tc.Function.Name != "" {
						delta.Name = tc.Function.Name
					}
					if tc.Function.Arguments != "" {
						delta.Arguments += tc.Function.Arguments
					}
				}

				// Terminal finish reasons.
				switch choice.FinishReason {
				case chat.FinishReasonToolCalls:
					// Emit all accumulated tool calls.
					p.flushToolCalls(deltas, events)

				case chat.FinishReasonStop, chat.FinishReasonLength, chat.FinishReasonContentFilter:
					// Emit remaining tool calls (if any), then signal done.
					p.flushToolCalls(deltas, events)
					events <- ParseEvent{Type: ParseEventDone}
					return

				default:
					// Non-terminal chunk — continue accumulating.
				}
			}
		}
	}()

	return events, nil
}

// ParseResponse extracts the content and tool calls from a non-streaming
// chat response. It uses the first choice's message.
func (p *ResponseParser) ParseResponse(resp *chat.ChatResponse) (string, []chat.ToolCall, error) {
	if resp == nil {
		return "", nil, fmt.Errorf("response is nil")
	}
	if len(resp.Choices) == 0 {
		return "", nil, nil
	}
	choice := resp.Choices[0]
	return choice.Message.Content, choice.Message.ToolCalls, nil
}

// MergeToolCallDeltas merges a set of partial tool call deltas (for the same
// tool call index) into a complete chat.ToolCall. Arguments are concatenated
// across all deltas; the last non-empty ID and Name values win.
func (p *ResponseParser) MergeToolCallDeltas(deltas []ToolCallDelta) *chat.ToolCall {
	if len(deltas) == 0 {
		return nil
	}

	result := &chat.ToolCall{}
	for _, d := range deltas {
		result.Index = d.Index
		if d.ID != "" {
			result.ID = d.ID
		}
		if d.Name != "" {
			result.Function.Name = d.Name
		}
		if d.Arguments != "" {
			result.Function.Arguments += d.Arguments
		}
	}
	if result.Function.Name == "" && result.ID == "" && result.Function.Arguments == "" {
		return nil
	}
	return result
}

// ─── Internal Helpers ──────────────────────────────────────────────────────────

// flushToolCalls merges all accumulated ToolCallDelta values and emits finished
// ToolCall events. Deltas are emitted in index order and cleared from the map.
// The events channel parameter is named to avoid shadowing the field.
func (p *ResponseParser) flushToolCalls(deltas map[int]*ToolCallDelta, events chan<- ParseEvent) {
	if len(deltas) == 0 {
		return
	}

	// Collect and sort indices for deterministic emission order.
	indices := make([]int, 0, len(deltas))
	for idx := range deltas {
		indices = append(indices, idx)
	}
	sort.Ints(indices)

	for _, idx := range indices {
		delta := deltas[idx]
		merged := p.MergeToolCallDeltas([]ToolCallDelta{*delta})
		if merged != nil {
			events <- ParseEvent{
				Type:     ParseEventToolCall,
				ToolCall: merged,
			}
		}
		delete(deltas, idx)
	}
}

// contentAccumulator is a small helper to concatenate content deltas.
// It exists as a lightweight type to keep the ParseStream method clean;
// the accumulated content is typically discarded in favour of re-building
// from emitted ParseEventContent events when needed.
type contentAccumulator struct {
	parts []string
}

func (a *contentAccumulator) Add(s string) {
	a.parts = append(a.parts, s)
}

func (a *contentAccumulator) String() string {
	var result string
	for _, p := range a.parts {
		result += p
	}
	return result
}
