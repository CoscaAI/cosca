package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
)

func TestNewResponseParser(t *testing.T) {
	p := NewResponseParser()
	if p == nil {
		t.Fatal("NewResponseParser returned nil")
	}
}

func TestParseResponse(t *testing.T) {
	p := NewResponseParser()

	t.Run("extracts content correctly", func(t *testing.T) {
		resp := &chat.ChatResponse{
			Choices: []chat.Choice{
				{
					Message: chat.Message{
						Content: "Hello, world!",
					},
				},
			},
		}
		content, toolCalls, err := p.ParseResponse(resp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if content != "Hello, world!" {
			t.Errorf("content = %q, want %q", content, "Hello, world!")
		}
		if len(toolCalls) != 0 {
			t.Errorf("expected 0 tool calls, got %d", len(toolCalls))
		}
	})

	t.Run("extracts tool calls correctly", func(t *testing.T) {
		resp := &chat.ChatResponse{
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
		}
		content, toolCalls, err := p.ParseResponse(resp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if content != "" {
			t.Errorf("content = %q, want empty", content)
		}
		if len(toolCalls) != 1 {
			t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
		}
		if toolCalls[0].Function.Name != "read_file" {
			t.Errorf("tool name = %q", toolCalls[0].Function.Name)
		}
	})

	t.Run("handles empty response", func(t *testing.T) {
		resp := &chat.ChatResponse{
			Choices: []chat.Choice{
				{Message: chat.Message{}},
			},
		}
		content, toolCalls, err := p.ParseResponse(resp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if content != "" {
			t.Errorf("content = %q, want empty", content)
		}
		if len(toolCalls) != 0 {
			t.Errorf("expected 0 tool calls, got %d", len(toolCalls))
		}
	})

	t.Run("handles nil response", func(t *testing.T) {
		_, _, err := p.ParseResponse(nil)
		if err == nil {
			t.Error("expected error for nil response")
		}
	})

	t.Run("handles response with no choices", func(t *testing.T) {
		resp := &chat.ChatResponse{Choices: []chat.Choice{}}
		content, toolCalls, err := p.ParseResponse(resp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if content != "" || len(toolCalls) != 0 {
			t.Errorf("expected empty content and tool calls for no choices")
		}
	})

	t.Run("uses first choice only", func(t *testing.T) {
		resp := &chat.ChatResponse{
			Choices: []chat.Choice{
				{Message: chat.Message{Content: "first"}},
				{Message: chat.Message{Content: "second"}},
			},
		}
		content, _, err := p.ParseResponse(resp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if content != "first" {
			t.Errorf("content = %q, want %q", content, "first")
		}
	})

	t.Run("response with both content and tool calls", func(t *testing.T) {
		resp := &chat.ChatResponse{
			Choices: []chat.Choice{
				{
					Message: chat.Message{
						Content: "I'll look that up.",
						ToolCalls: []chat.ToolCall{
							{
								ID:   "call_1",
								Type: "function",
								Function: chat.FunctionCall{
									Name:      "search",
									Arguments: `{}`,
								},
							},
						},
					},
				},
			},
		}
		content, toolCalls, err := p.ParseResponse(resp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if content != "I'll look that up." {
			t.Errorf("content = %q", content)
		}
		if len(toolCalls) != 1 {
			t.Errorf("expected 1 tool call, got %d", len(toolCalls))
		}
	})
}

func TestMergeToolCallDeltas(t *testing.T) {
	p := NewResponseParser()

	t.Run("returns nil for empty deltas", func(t *testing.T) {
		result := p.MergeToolCallDeltas([]ToolCallDelta{})
		if result != nil {
			t.Error("expected nil for empty deltas")
		}
	})

	t.Run("handles single delta", func(t *testing.T) {
		deltas := []ToolCallDelta{
			{
				Index:     0,
				ID:        "call_1",
				Name:      "read_file",
				Arguments: `{"path":"test.txt"}`,
			},
		}
		result := p.MergeToolCallDeltas(deltas)
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.ID != "call_1" {
			t.Errorf("ID = %q", result.ID)
		}
		if result.Function.Name != "read_file" {
			t.Errorf("Name = %q", result.Function.Name)
		}
		if result.Function.Arguments != `{"path":"test.txt"}` {
			t.Errorf("Arguments = %q", result.Function.Arguments)
		}
	})

	t.Run("merges multiple fields across deltas", func(t *testing.T) {
		deltas := []ToolCallDelta{
			{Index: 0, ID: "call_1"},
			{Index: 0, Name: "read_file"},
			{Index: 0, Arguments: `{"path"`},
			{Index: 0, Arguments: `:"test.txt"}`},
		}
		result := p.MergeToolCallDeltas(deltas)
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.ID != "call_1" {
			t.Errorf("ID = %q", result.ID)
		}
		if result.Function.Name != "read_file" {
			t.Errorf("Name = %q", result.Function.Name)
		}
		if result.Function.Arguments != `{"path":"test.txt"}` {
			t.Errorf("Arguments = %q", result.Function.Arguments)
		}
	})

	t.Run("merges multiple deltas with different indices separately", func(t *testing.T) {
		// MergeToolCallDeltas should only be called with same-index deltas.
		// But if multiple are passed, the last index wins for Index field
		// and arguments accumulate.
		deltas := []ToolCallDelta{
			{Index: 0, ID: "call_1", Name: "tool_a", Arguments: `{"a":1}`},
			{Index: 1, ID: "call_2", Name: "tool_b", Arguments: `{"b":2}`},
		}
		result := p.MergeToolCallDeltas(deltas)
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		// The Index from the last delta with data wins
		if result.Function.Arguments != `{"a":1}{"b":2}` {
			t.Errorf("Arguments = %q (accumulated)", result.Function.Arguments)
		}
	})

	t.Run("returns nil when all fields empty", func(t *testing.T) {
		deltas := []ToolCallDelta{
			{Index: 0},
		}
		result := p.MergeToolCallDeltas(deltas)
		if result != nil {
			t.Error("expected nil when all fields are empty")
		}
	})

	t.Run("last non-empty ID and Name win", func(t *testing.T) {
		deltas := []ToolCallDelta{
			{Index: 0, ID: "call_old", Name: "old_name"},
			{Index: 0, ID: "call_new", Name: "new_name"},
		}
		result := p.MergeToolCallDeltas(deltas)
		if result.ID != "call_new" {
			t.Errorf("ID = %q (last non-empty should win)", result.ID)
		}
		if result.Function.Name != "new_name" {
			t.Errorf("Name = %q (last non-empty should win)", result.Function.Name)
		}
	})
}

func TestParseStream(t *testing.T) {
	p := NewResponseParser()

	t.Run("streams content deltas", func(t *testing.T) {
		chunks := []chat.ChatStreamChunk{
			{Choices: []chat.StreamChoice{{Delta: chat.Message{Content: "Hello "}, FinishReason: ""}}},
			{Choices: []chat.StreamChoice{{Delta: chat.Message{Content: "World!"}, FinishReason: chat.FinishReasonStop}}},
		}
		stream := newMockStreamWithChunks(chunks)
		ctx := context.Background()

		events, err := p.ParseStream(ctx, stream, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var contents []string
		for e := range events {
			switch e.Type {
			case ParseEventContent:
				contents = append(contents, e.Content)
			case ParseEventDone:
				// OK
			case ParseEventError:
				t.Fatalf("unexpected error event: %v", e.Error)
			}
		}

		expected := []string{"Hello ", "World!"}
		if len(contents) != len(expected) {
			t.Errorf("got %d content events, want %d", len(contents), len(expected))
		}
		for i, c := range contents {
			if c != expected[i] {
				t.Errorf("content[%d] = %q, want %q", i, c, expected[i])
			}
		}
	})

	t.Run("streams tool calls", func(t *testing.T) {
		chunks := []chat.ChatStreamChunk{
			{
				Choices: []chat.StreamChoice{
					{
						Delta: chat.Message{
							ToolCalls: []chat.ToolCall{
								{Index: 0, ID: "call_1", Function: chat.FunctionCall{Name: "read_file", Arguments: `{"path"`}},
							},
						},
						FinishReason: "",
					},
				},
			},
			{
				Choices: []chat.StreamChoice{
					{
						Delta: chat.Message{
							ToolCalls: []chat.ToolCall{
								{Index: 0, Function: chat.FunctionCall{Arguments: `:"test.txt"}`}},
							},
						},
						FinishReason: chat.FinishReasonToolCalls,
					},
				},
			},
		}
		stream := newMockStreamWithChunks(chunks)
		ctx := context.Background()

		events, err := p.ParseStream(ctx, stream, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var toolCalls []chat.ToolCall
		for e := range events {
			switch e.Type {
			case ParseEventToolCall:
				if e.ToolCall != nil {
					toolCalls = append(toolCalls, *e.ToolCall)
				}
			case ParseEventDone:
				// OK
			case ParseEventError:
				t.Fatalf("unexpected error: %v", e.Error)
			}
		}

		if len(toolCalls) != 1 {
			t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
		}
		if toolCalls[0].ID != "call_1" {
			t.Errorf("tool call ID = %q", toolCalls[0].ID)
		}
		if toolCalls[0].Function.Name != "read_file" {
			t.Errorf("tool call name = %q", toolCalls[0].Function.Name)
		}
		if toolCalls[0].Function.Arguments != `{"path":"test.txt"}` {
			t.Errorf("tool call args = %q", toolCalls[0].Function.Arguments)
		}
	})

	t.Run("handles ctx cancellation", func(t *testing.T) {
		// Use an infinite stream to test cancellation.
		chunks := []chat.ChatStreamChunk{
			{Choices: []chat.StreamChoice{{Delta: chat.Message{Content: "streaming..."}, FinishReason: ""}}},
		}
		stream := newMockStreamWithChunks(chunks)
		// Make the stream never end by emitting the same chunk repeatedly
		stream.idx = 0                                   // reset
		stream.chunks = append(stream.chunks, chunks...) // repeat
		// Actually, let's use a simpler approach: create a canceled context.
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		events, err := p.ParseStream(ctx, stream, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		gotError := false
		for e := range events {
			if e.Type == ParseEventError {
				gotError = true
			}
		}
		if !gotError {
			t.Error("expected error event for cancelled context")
		}
	})

	t.Run("handles stream recv error", func(t *testing.T) {
		stream := &mockStream{err: errors.New("stream error")}
		ctx := context.Background()

		events, err := p.ParseStream(ctx, stream, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		gotError := false
		for e := range events {
			if e.Type == ParseEventError {
				gotError = true
				if e.Error == nil {
					t.Error("error event should have non-nil error")
				}
			}
		}
		if !gotError {
			t.Error("expected error event for stream recv error")
		}
	})

	t.Run("stream ending without finish reason", func(t *testing.T) {
		chunks := []chat.ChatStreamChunk{
			{Choices: []chat.StreamChoice{{Delta: chat.Message{Content: "some content"}, FinishReason: ""}}},
		}
		stream := newMockStreamWithChunks(chunks)
		ctx := context.Background()

		events, err := p.ParseStream(ctx, stream, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		gotDone := false
		for e := range events {
			if e.Type == ParseEventDone {
				gotDone = true
			}
			if e.Type == ParseEventError {
				t.Fatalf("unexpected error: %v", e.Error)
			}
		}
		if !gotDone {
			t.Error("expected Done event when stream ends without finish reason")
		}
	})

	t.Run("stream with multiple choices", func(t *testing.T) {
		// Multiple choices in a single chunk
		chunks := []chat.ChatStreamChunk{
			{
				Choices: []chat.StreamChoice{
					{Index: 0, Delta: chat.Message{Content: "Hello"}, FinishReason: ""},
					{Index: 1, Delta: chat.Message{Content: "World"}, FinishReason: chat.FinishReasonStop},
				},
			},
		}
		stream := newMockStreamWithChunks(chunks)
		ctx := context.Background()

		events, err := p.ParseStream(ctx, stream, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		contents := make(map[int]string)
		for e := range events {
			if e.Type == ParseEventContent {
				// This depends on iteration order, just check we got content
				contents[0] = e.Content
			}
		}
		// The stream with FinishReasonStop on choice 1 will trigger done
		// Content from choice 0 should still be emitted
		if len(contents) == 0 {
			t.Error("expected at least one content event")
		}
	})

	t.Run("finish reason length triggers done", func(t *testing.T) {
		chunks := []chat.ChatStreamChunk{
			{
				Choices: []chat.StreamChoice{
					{Delta: chat.Message{Content: "truncated"}, FinishReason: chat.FinishReasonLength},
				},
			},
		}
		stream := newMockStreamWithChunks(chunks)
		ctx := context.Background()

		events, err := p.ParseStream(ctx, stream, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		gotDone := false
		for e := range events {
			if e.Type == ParseEventDone {
				gotDone = true
			}
			if e.Type == ParseEventError {
				t.Fatalf("unexpected error: %v", e.Error)
			}
		}
		if !gotDone {
			t.Error("expected Done for length finish reason")
		}
	})

	t.Run("finish reason content filter triggers done", func(t *testing.T) {
		chunks := []chat.ChatStreamChunk{
			{
				Choices: []chat.StreamChoice{
					{Delta: chat.Message{Content: "filtered"}, FinishReason: chat.FinishReasonContentFilter},
				},
			},
		}
		stream := newMockStreamWithChunks(chunks)
		ctx := context.Background()

		events, err := p.ParseStream(ctx, stream, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		gotDone := false
		for e := range events {
			if e.Type == ParseEventDone {
				gotDone = true
			}
		}
		if !gotDone {
			t.Error("expected Done for content_filter finish reason")
		}
	})
}

func TestContentAccumulator(t *testing.T) {
	t.Run("accumulates and returns concatenated string", func(t *testing.T) {
		var ca contentAccumulator
		ca.Add("Hello ")
		ca.Add("World")
		ca.Add("!")
		if s := ca.String(); s != "Hello World!" {
			t.Errorf("String() = %q, want %q", s, "Hello World!")
		}
	})

	t.Run("empty accumulator returns empty string", func(t *testing.T) {
		var ca contentAccumulator
		if s := ca.String(); s != "" {
			t.Errorf("String() = %q, want empty", s)
		}
	})
}
