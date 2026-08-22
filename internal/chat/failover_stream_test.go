package chat

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chunkWithContent returns a stream chunk carrying a content delta.
func chunkWithContent(id, content string) *ChatStreamChunk {
	return &ChatStreamChunk{
		ID:    id,
		Model: "mock-model",
		Choices: []StreamChoice{
			{Index: 0, Delta: Message{Role: RoleAssistant, Content: content}},
		},
	}
}

// finishChunk returns a stream chunk signaling a normal end via FinishReason.
func finishChunk() *ChatStreamChunk {
	return &ChatStreamChunk{
		ID:    "finish",
		Model: "mock-model",
		Choices: []StreamChoice{
			{Index: 0, FinishReason: FinishReasonStop},
		},
	}
}

// ─── failoverStream ──────────────────────────────────────────────────────────

// TestFailoverStream_ReconnectsMidStream verifies the core scenario: the
// primary stream delivers deltas, dies mid-flight with a Recv error, the
// wrapper reconnects to the fallback provider, and deltas keep flowing until a
// FinishReason chunk arrives.
func TestFailoverStream_ReconnectsMidStream(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()
	primary := registerMock(reg, "openai", "gpt-4o", 0)
	fallback := registerMock(reg, "anthropic", "claude-3-5-sonnet", 10)

	// Primary delivers two deltas then dies mid-flight.
	primary.streamResp = &mockStream{
		chunks: []*ChatStreamChunk{chunkWithContent("s1", "Hel"), chunkWithContent("s1", "lo")},
		err:    errFatal,
	}
	// Fallback completes the response and ends normally.
	fallback.streamResp = &mockStream{
		chunks: []*ChatStreamChunk{chunkWithContent("s2", " world"), chunkWithContent("s2", "!"), finishChunk()},
	}

	_ = reg.Select(context.Background(), ChatRegistryConfig{
		Primary:   "openai",
		Fallbacks: []string{"anthropic"},
	})

	stream, err := reg.ChatStream(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, ChatOptions{})
	require.NoError(t, err)
	require.NotNil(t, stream)

	var got strings.Builder
	var gotFinish bool
	for !gotFinish {
		chunk, err := stream.Recv()
		require.NoError(t, err)
		require.NotNil(t, chunk)
		for _, c := range chunk.Choices {
			got.WriteString(c.Delta.Content)
			if c.FinishReason != "" {
				gotFinish = true
			}
		}
	}

	assert.Equal(t, "Hello world!", got.String())

	// The wrapper reconnected exactly once, to the fallback provider.
	fs, ok := stream.(*failoverStream)
	require.True(t, ok, "ChatStream must be wrapped in failoverStream")
	assert.Equal(t, 1, fs.reconnects)
	assert.Equal(t, 1, fs.providerIdx)
	assert.Equal(t, int32(1), primary.streamCalls.Load())
	assert.Equal(t, int32(1), fallback.streamCalls.Load())
}

// TestFailoverStream_NoFallbackAvailable verifies that when every remaining
// provider fails to open during a reconnect, the original Recv error is
// returned with context about the failure.
func TestFailoverStream_NoFallbackAvailable(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()
	primary := registerMock(reg, "openai", "gpt-4o", 0)
	fb1 := registerMock(reg, "fb1", "fb1-model", 10)
	fb2 := registerMock(reg, "fb2", "fb2-model", 20)

	primary.streamResp = &mockStream{
		chunks: []*ChatStreamChunk{chunkWithContent("s1", "Hel")},
		err:    errFatal,
	}
	fb1.streamErr = errFatal
	fb2.streamErr = errFatal

	_ = reg.Select(context.Background(), ChatRegistryConfig{
		Primary:   "openai",
		Fallbacks: []string{"fb1", "fb2"},
	})

	stream, err := reg.ChatStream(context.Background(), nil, ChatOptions{})
	require.NoError(t, err)

	_, err = stream.Recv()
	require.NoError(t, err)

	_, err = stream.Recv()
	require.Error(t, err)
	assert.ErrorIs(t, err, errFatal, "original Recv error must be wrapped")
	assert.Contains(t, err.Error(), "stream failed after 1 chunks and 0 reconnects")

	// Both fallbacks were tried, in order.
	assert.Equal(t, int32(1), fb1.streamCalls.Load())
	assert.Equal(t, int32(1), fb2.streamCalls.Load())
}

// TestFailoverStream_NormalEndDoesNotReconnect verifies that a normal stream
// end — a chunk with a non-empty FinishReason and no error — is NOT treated as
// a failure, so no reconnect is attempted even after content was delivered.
func TestFailoverStream_NormalEndDoesNotReconnect(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()
	primary := registerMock(reg, "openai", "gpt-4o", 0)
	fallback := registerMock(reg, "anthropic", "claude-3-5-sonnet", 10)

	// Primary ends normally: content delta followed by a FinishReason chunk
	// with no error. The err field is never reached by a well-behaved consumer.
	primary.streamResp = &mockStream{
		chunks: []*ChatStreamChunk{chunkWithContent("s1", "Hello"), finishChunk()},
		err:    errFatal,
	}

	_ = reg.Select(context.Background(), ChatRegistryConfig{
		Primary:   "openai",
		Fallbacks: []string{"anthropic"},
	})

	stream, err := reg.ChatStream(context.Background(), nil, ChatOptions{})
	require.NoError(t, err)

	chunk, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, "Hello", chunk.Choices[0].Delta.Content)

	chunk, err = stream.Recv()
	require.NoError(t, err)
	require.NotNil(t, chunk)
	require.Len(t, chunk.Choices, 1)
	assert.Equal(t, FinishReasonStop, chunk.Choices[0].FinishReason)

	// The finish chunk ended the stream normally — no reconnect happened.
	fs, ok := stream.(*failoverStream)
	require.True(t, ok, "ChatStream must be wrapped in failoverStream")
	assert.Equal(t, 0, fs.reconnects)
	assert.Equal(t, int32(1), primary.streamCalls.Load())
	assert.Equal(t, int32(0), fallback.streamCalls.Load())
}

// TestFailoverStream_MaxReconnectsCap verifies the reconnect budget: with a
// chain of primary + two fallbacks, each dying after a single delta, the
// wrapper reconnects at most maxReconnects times and then surfaces the error
// with the final reconnect count.
func TestFailoverStream_MaxReconnectsCap(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()
	primary := registerMock(reg, "openai", "gpt-4o", 0)
	fb1 := registerMock(reg, "fb1", "fb1-model", 10)
	fb2 := registerMock(reg, "fb2", "fb2-model", 20)

	dyingStream := func(id string) ChatStream {
		return &mockStream{
			chunks: []*ChatStreamChunk{chunkWithContent(id, id)},
			err:    errFatal,
		}
	}
	primary.streamResp = dyingStream("a")
	fb1.streamResp = dyingStream("b")
	fb2.streamResp = dyingStream("c")

	_ = reg.Select(context.Background(), ChatRegistryConfig{
		Primary:   "openai",
		Fallbacks: []string{"fb1", "fb2"},
	})

	stream, err := reg.ChatStream(context.Background(), nil, ChatOptions{})
	require.NoError(t, err)

	// Every provider delivers one delta then dies, so each Recv after the
	// first triggers exactly one reconnect until the budget is exhausted.
	var got strings.Builder
	for {
		chunk, err := stream.Recv()
		if err != nil {
			require.ErrorIs(t, err, errFatal)
			assert.Contains(t, err.Error(), "stream failed after 3 chunks and 2 reconnects")
			break
		}
		got.WriteString(chunk.Choices[0].Delta.Content)
	}

	assert.Equal(t, "abc", got.String())

	fs, ok := stream.(*failoverStream)
	require.True(t, ok)
	assert.Equal(t, 2, fs.reconnects)
	assert.Equal(t, 2, fs.providerIdx)
}

// TestFailoverStream_NoReconnectBeforeContent verifies the sawContent gate: a
// stream that dies before delivering any content is reported as an error
// without burning reconnect budget or touching the fallbacks.
func TestFailoverStream_NoReconnectBeforeContent(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()
	primary := registerMock(reg, "openai", "gpt-4o", 0)
	fallback := registerMock(reg, "anthropic", "claude-3-5-sonnet", 10)

	// Primary dies on the very first Recv — no content was delivered.
	primary.streamResp = &mockStream{err: errFatal}

	_ = reg.Select(context.Background(), ChatRegistryConfig{
		Primary:   "openai",
		Fallbacks: []string{"anthropic"},
	})

	stream, err := reg.ChatStream(context.Background(), nil, ChatOptions{})
	require.NoError(t, err)

	_, err = stream.Recv()
	require.Error(t, err)
	assert.ErrorIs(t, err, errFatal)
	assert.Contains(t, err.Error(), "stream failed after 0 chunks and 0 reconnects")

	// No reconnect attempt happened — the fallback was never asked to open.
	assert.Equal(t, int32(0), fallback.streamCalls.Load())
}

// TestFailoverStream_ReconnectThatFailsImmediately verifies the reconnected
// stream itself failing on its first Recv does not surface the error: the
// wrapper advances to the next provider in the same Recv call.
func TestFailoverStream_ReconnectThatFailsImmediately(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()
	primary := registerMock(reg, "openai", "gpt-4o", 0)
	fb1 := registerMock(reg, "fb1", "fb1-model", 10)
	fb2 := registerMock(reg, "fb2", "fb2-model", 20)

	primary.streamResp = &mockStream{
		chunks: []*ChatStreamChunk{chunkWithContent("s1", "Hel")},
		err:    errFatal,
	}
	// fb1 opens successfully but fails on its very first Recv.
	fb1.streamResp = &mockStream{err: errFatal}
	// fb2 completes the response.
	fb2.streamResp = &mockStream{
		chunks: []*ChatStreamChunk{chunkWithContent("s2", "lo"), finishChunk()},
	}

	_ = reg.Select(context.Background(), ChatRegistryConfig{
		Primary:   "openai",
		Fallbacks: []string{"fb1", "fb2"},
	})

	stream, err := reg.ChatStream(context.Background(), nil, ChatOptions{})
	require.NoError(t, err)

	_, err = stream.Recv()
	require.NoError(t, err)

	chunk, err := stream.Recv() // primary dies → fb1 opens but fails → fb2
	require.NoError(t, err)
	assert.Equal(t, "lo", chunk.Choices[0].Delta.Content)

	// Both reconnects were consumed (fb1 then fb2), ending on the last provider.
	fs, ok := stream.(*failoverStream)
	require.True(t, ok)
	assert.Equal(t, 2, fs.reconnects)
	assert.Equal(t, 2, fs.providerIdx)
	assert.Equal(t, int32(1), fb1.streamCalls.Load())
	assert.Equal(t, int32(1), fb2.streamCalls.Load())
}

// TestFailoverStream_Close verifies Close only touches the active stream.
func TestFailoverStream_Close(t *testing.T) {
	t.Parallel()

	primary := registerMock(newTestRegistry(), "openai", "gpt-4o", 0)
	fallback := registerMock(newTestRegistry(), "anthropic", "claude-3-5-sonnet", 10)

	primaryStream := &mockStream{chunks: []*ChatStreamChunk{chunkWithContent("s1", "Hel")}, err: errFatal}
	fallbackStream := &mockStream{chunks: []*ChatStreamChunk{chunkWithContent("s2", "lo")}}
	fallback.streamResp = fallbackStream

	fs := &failoverStream{
		ctx:           context.Background(),
		messages:      nil,
		opts:          ChatOptions{Stream: true},
		providers:     []ChatProvider{primary, fallback},
		current:       primaryStream,
		providerIdx:   0,
		maxReconnects: defaultMaxStreamReconnects,
	}

	_, err := fs.Recv()
	require.NoError(t, err)
	_, err = fs.Recv() // triggers reconnect to fallback
	require.NoError(t, err)

	require.NoError(t, fs.Close())
	assert.True(t, fallbackStream.closed)
	assert.False(t, primaryStream.closed, "dead streams are not closed")
}
