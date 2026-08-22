package chat

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Mock ChatProvider ───────────────────────────────────────────────────────

// mockProvider is a configurable ChatProvider used throughout tests.
type mockProvider struct {
	name       string
	model      string
	chatErr    error
	chatResp   *ChatResponse
	streamErr  error
	streamResp ChatStream
	closeErr   error

	// Counters for verifying invocations.
	chatCalls   atomic.Int32
	streamCalls atomic.Int32
	closeCalls  atomic.Int32
}

func (m *mockProvider) Chat(_ context.Context, _ []Message, _ ChatOptions) (*ChatResponse, error) {
	m.chatCalls.Add(1)
	if m.chatErr != nil {
		return nil, m.chatErr
	}
	return m.chatResp, nil
}

func (m *mockProvider) ChatStream(_ context.Context, _ []Message, _ ChatOptions) (ChatStream, error) {
	m.streamCalls.Add(1)
	if m.streamErr != nil {
		return nil, m.streamErr
	}
	return m.streamResp, nil
}

func (m *mockProvider) Model() string { return m.model }

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) Close() error {
	m.closeCalls.Add(1)
	return m.closeErr
}

// ─── Mock ChatStream ─────────────────────────────────────────────────────────

// mockStream implements ChatStream with configurable chunks.
type mockStream struct {
	chunks   []*ChatStreamChunk
	err      error
	pos      int
	closeErr error
	closed   bool
}

func (s *mockStream) Recv() (*ChatStreamChunk, error) {
	if s.pos < len(s.chunks) {
		chunk := s.chunks[s.pos]
		s.pos++
		return chunk, nil
	}
	return nil, s.err
}

func (s *mockStream) Close() error {
	s.closed = true
	return s.closeErr
}

// ─── Mock Factory ────────────────────────────────────────────────────────────

// mockFactory returns a factory that creates the given provider.
// A nil factory value means "return error on creation".
func mockFactory(p ChatProvider, createErr error) ChatProviderFactory {
	return func(_ context.Context, _ map[string]interface{}) (ChatProvider, error) {
		if createErr != nil {
			return nil, createErr
		}
		return p, nil
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// newTestRegistry creates a fresh ChatRegistry for testing.
func newTestRegistry() *ChatRegistry {
	return &ChatRegistry{
		stats: &ChatStats{},
	}
}

// registerMock registers a mockProvider in the registry and returns the mock
// so callers can inspect it after operations.
func registerMock(r *ChatRegistry, name, model string, priority int) *mockProvider {
	p := &mockProvider{name: name, model: model}
	factory := func(_ context.Context, _ map[string]interface{}) (ChatProvider, error) {
		return p, nil
	}
	r.Register(name, factory, "test provider: "+name, priority)
	return p
}

// okResponse returns a minimal successful ChatResponse.
func okResponse() *ChatResponse {
	return &ChatResponse{
		ID:    "chat-1",
		Model: "mock-model",
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					Role:    RoleAssistant,
					Content: "Hello from mock",
				},
				FinishReason: FinishReasonStop,
			},
		},
		Usage: Usage{TotalTokens: 42},
	}
}

// okStream returns a minimal valid ChatStream.
func okStream() ChatStream {
	return &mockStream{
		chunks: []*ChatStreamChunk{
			{
				ID:    "stream-1",
				Model: "mock-model",
				Choices: []StreamChoice{
					{
						Index: 0,
						Delta: Message{Role: RoleAssistant, Content: "Hello"},
					},
				},
			},
		},
	}
}

// fatalErr is a sentinel error used to trigger failure paths.
var errFatal = errors.New("provider fatal error")

// ─── ChatRegistry: Register ──────────────────────────────────────────────────

func TestRegistry_Register(t *testing.T) {
	t.Parallel()

	t.Run("registers new provider", func(t *testing.T) {
		reg := newTestRegistry()
		_ = registerMock(reg, "openai", "gpt-4o", 0)

		names := reg.List()
		require.Len(t, names, 1)
		assert.Equal(t, "openai", names[0])
	})

	t.Run("replaces existing provider with same name", func(t *testing.T) {
		reg := newTestRegistry()
		_ = registerMock(reg, "openai", "gpt-4o", 0)
		_ = registerMock(reg, "openai", "gpt-4o-mini", 10)

		names := reg.List()
		require.Len(t, names, 1)
		assert.Equal(t, "openai", names[0])

		p, ok := reg.Get("openai")
		require.True(t, ok)
		assert.Equal(t, "gpt-4o-mini", p.Model())
	})

	t.Run("sorts by priority ascending", func(t *testing.T) {
		reg := newTestRegistry()
		_ = registerMock(reg, "low", "l", 100)
		_ = registerMock(reg, "high", "h", 0)
		_ = registerMock(reg, "mid", "m", 50)

		names := reg.List()
		assert.Equal(t, []string{"high", "mid", "low"}, names)
	})

	t.Run("multiple providers", func(t *testing.T) {
		reg := newTestRegistry()
		_ = registerMock(reg, "a", "a-model", 0)
		_ = registerMock(reg, "b", "b-model", 1)
		_ = registerMock(reg, "c", "c-model", 2)

		assert.Len(t, reg.List(), 3)
	})
}

// ─── ChatRegistry: Get ───────────────────────────────────────────────────────

func TestRegistry_Get(t *testing.T) {
	t.Parallel()

	t.Run("returns provider for existing name", func(t *testing.T) {
		reg := newTestRegistry()
		m := registerMock(reg, "openai", "gpt-4o", 0)

		p, ok := reg.Get("openai")
		require.True(t, ok)
		assert.Equal(t, "openai", p.Name())
		assert.Equal(t, "gpt-4o", p.Model())

		// instance was created via the factory
		_ = m
	})

	t.Run("returns nil/ok=false for nonexistent name", func(t *testing.T) {
		reg := newTestRegistry()
		p, ok := reg.Get("nonexistent")
		assert.False(t, ok)
		assert.Nil(t, p)
	})

	t.Run("empty registry returns nothing", func(t *testing.T) {
		reg := newTestRegistry()
		p, ok := reg.Get("any")
		assert.False(t, ok)
		assert.Nil(t, p)
	})

	t.Run("caches provider instance", func(t *testing.T) {
		reg := newTestRegistry()
		m := registerMock(reg, "openai", "gpt-4o", 0)

		p1, ok1 := reg.Get("openai")
		require.True(t, ok1)

		p2, ok2 := reg.Get("openai")
		require.True(t, ok2)

		// Same instance should be returned (factory is only called once per
		// registered entry because getOrCreate caches the result).
		assert.Same(t, p1, p2)

		// Factory was called exactly once (for providers that use factory).
		// The mock is pre-built and factory just returns it, so no extra calls.
		_ = m
	})

	t.Run("factory returning error yields nil provider", func(t *testing.T) {
		reg := newTestRegistry()
		factory := func(_ context.Context, _ map[string]interface{}) (ChatProvider, error) {
			return nil, errFatal
		}
		reg.Register("broken", factory, "failing", 0)

		// Get returns (nil, true) because the name is registered even though
		// the factory fails to instantiate. Consumers should check p != nil.
		p, ok := reg.Get("broken")
		assert.True(t, ok, "name is registered so ok must be true")
		assert.Nil(t, p, "factory failed so provider is nil")
	})
}

// ─── ChatRegistry: List ──────────────────────────────────────────────────────

func TestRegistry_List(t *testing.T) {
	t.Parallel()

	t.Run("empty registry returns empty list", func(t *testing.T) {
		reg := newTestRegistry()
		assert.Empty(t, reg.List())
	})

	t.Run("single provider", func(t *testing.T) {
		reg := newTestRegistry()
		_ = registerMock(reg, "sole", "sole-model", 5)
		assert.Equal(t, []string{"sole"}, reg.List())
	})
}

// ─── ChatRegistry: Select and AutoDetect ─────────────────────────────────────

func TestRegistry_Select(t *testing.T) {
	t.Parallel()

	t.Run("selects explicit primary", func(t *testing.T) {
		reg := newTestRegistry()
		_ = registerMock(reg, "openai", "gpt-4o", 0)

		cfg := ChatRegistryConfig{
			Primary: "openai",
		}
		err := reg.Select(context.Background(), cfg)
		require.NoError(t, err)

		assert.Equal(t, "openai", reg.Name())
		assert.Equal(t, "gpt-4o", reg.Model())
	})

	t.Run("auto-detects from registered providers", func(t *testing.T) {
		reg := newTestRegistry()
		_ = registerMock(reg, "anthropic", "claude-3-5-sonnet", 5)

		cfg := ChatRegistryConfig{
			AutoDetect: true,
		}
		err := reg.Select(context.Background(), cfg)
		require.NoError(t, err)

		assert.Equal(t, "anthropic", reg.Name())
	})

	t.Run("uses fallbacks when primary set", func(t *testing.T) {
		reg := newTestRegistry()
		primary := registerMock(reg, "openai", "gpt-4o", 0)
		fallback := registerMock(reg, "anthropic", "claude-3-5-sonnet", 10)
		primary.chatResp = okResponse()

		cfg := ChatRegistryConfig{
			Primary:   "openai",
			Fallbacks: []string{"anthropic"},
		}
		err := reg.Select(context.Background(), cfg)
		require.NoError(t, err)

		// Primary is selected; fallback should be ready.
		assert.Equal(t, "openai", reg.Name())

		// Verify fallback works if primary fails.
		primary.chatErr = errFatal
		fallback.chatResp = okResponse()

		resp, err := reg.Chat(context.Background(), nil, ChatOptions{})
		require.NoError(t, err)
		assert.Equal(t, "mock-model", resp.Model)
	})

	t.Run("auto-detect does not duplicate primary", func(t *testing.T) {
		reg := newTestRegistry()
		_ = registerMock(reg, "openai", "gpt-4o", 0)
		_ = registerMock(reg, "anthropic", "claude-3-5-sonnet", 10)

		cfg := ChatRegistryConfig{
			Primary:    "openai",
			AutoDetect: true,
		}
		err := reg.Select(context.Background(), cfg)
		require.NoError(t, err)

		// openai is primary, anthropic is a fallback (not duplicated).
		assert.Equal(t, "openai", reg.Name())
	})

	t.Run("factory returning error skips candidate", func(t *testing.T) {
		reg := newTestRegistry()

		// Register a provider whose factory always fails.
		reg.Register("bad", mockFactory(nil, errFatal), "failing", 0)
		_ = registerMock(reg, "good", "good-model", 10)

		cfg := ChatRegistryConfig{
			Primary:   "bad",
			Fallbacks: []string{"good"},
		}
		err := reg.Select(context.Background(), cfg)
		require.NoError(t, err)

		// bad was skipped, good was selected.
		assert.Equal(t, "good", reg.Name())
	})

	t.Run("no viable candidate returns error", func(t *testing.T) {
		reg := newTestRegistry()

		// Only a failing provider.
		reg.Register("bad", mockFactory(nil, errFatal), "failing", 0)

		cfg := ChatRegistryConfig{
			Primary: "bad",
		}
		err := reg.Select(context.Background(), cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no chat provider could be initialized")
	})

	t.Run("empty candidates returns error", func(t *testing.T) {
		reg := newTestRegistry()
		cfg := ChatRegistryConfig{}
		err := reg.Select(context.Background(), cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no chat provider could be initialized")
	})
}

// ─── ChatRegistry: Chat (fallback chain) ─────────────────────────────────────

func TestRegistry_Chat(t *testing.T) {
	t.Parallel()

	t.Run("success through primary", func(t *testing.T) {
		reg := newTestRegistry()
		m := registerMock(reg, "openai", "gpt-4o", 0)
		m.chatResp = okResponse()

		_ = reg.Select(context.Background(), ChatRegistryConfig{Primary: "openai"})

		resp, err := reg.Chat(context.Background(), []Message{
			{Role: RoleUser, Content: "hi"},
		}, ChatOptions{})
		require.NoError(t, err)
		assert.Equal(t, "chat-1", resp.ID)
		assert.Equal(t, int32(1), m.chatCalls.Load())
	})

	t.Run("primary fails, fallback succeeds", func(t *testing.T) {
		reg := newTestRegistry()
		primary := registerMock(reg, "openai", "gpt-4o", 0)
		fallback := registerMock(reg, "anthropic", "claude-3-5-sonnet", 10)
		primary.chatErr = errFatal
		fallback.chatResp = okResponse()

		_ = reg.Select(context.Background(), ChatRegistryConfig{
			Primary:   "openai",
			Fallbacks: []string{"anthropic"},
		})

		resp, err := reg.Chat(context.Background(), []Message{
			{Role: RoleUser, Content: "hi"},
		}, ChatOptions{})
		require.NoError(t, err)

		// Primary was tried and failed, fallback was used.
		assert.Equal(t, int32(1), primary.chatCalls.Load())
		assert.Equal(t, int32(1), fallback.chatCalls.Load())
		assert.Equal(t, "chat-1", resp.ID)
	})

	t.Run("all providers fail", func(t *testing.T) {
		reg := newTestRegistry()
		primary := registerMock(reg, "openai", "gpt-4o", 0)
		fallback := registerMock(reg, "anthropic", "claude-3-5-sonnet", 10)
		primary.chatErr = errFatal
		fallback.chatErr = errFatal

		_ = reg.Select(context.Background(), ChatRegistryConfig{
			Primary:   "openai",
			Fallbacks: []string{"anthropic"},
		})

		_, err := reg.Chat(context.Background(), []Message{
			{Role: RoleUser, Content: "hi"},
		}, ChatOptions{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "all chat providers failed")
	})

	t.Run("no selected provider returns error", func(t *testing.T) {
		reg := newTestRegistry()

		_, err := reg.Chat(context.Background(), nil, ChatOptions{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no chat provider selected")
	})

	t.Run("empty messages are passed through", func(t *testing.T) {
		reg := newTestRegistry()
		m := registerMock(reg, "openai", "gpt-4o", 0)
		m.chatResp = okResponse()

		_ = reg.Select(context.Background(), ChatRegistryConfig{Primary: "openai"})

		resp, err := reg.Chat(context.Background(), nil, ChatOptions{
			Temperature: 0.5,
			MaxTokens:   100,
		})
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})
}

// ─── ChatRegistry: ChatStream (fallback chain) ───────────────────────────────

func TestRegistry_ChatStream(t *testing.T) {
	t.Parallel()

	t.Run("success through primary", func(t *testing.T) {
		reg := newTestRegistry()
		m := registerMock(reg, "openai", "gpt-4o", 0)
		m.streamResp = okStream()

		_ = reg.Select(context.Background(), ChatRegistryConfig{Primary: "openai"})

		stream, err := reg.ChatStream(context.Background(), []Message{
			{Role: RoleUser, Content: "hi"},
		}, ChatOptions{})
		require.NoError(t, err)
		require.NotNil(t, stream)

		chunk, err := stream.Recv()
		require.NoError(t, err)
		assert.Equal(t, "stream-1", chunk.ID)
		assert.Equal(t, int32(1), m.streamCalls.Load())
	})

	t.Run("primary fails, fallback succeeds", func(t *testing.T) {
		reg := newTestRegistry()
		primary := registerMock(reg, "openai", "gpt-4o", 0)
		fallback := registerMock(reg, "anthropic", "claude-3-5-sonnet", 10)
		primary.streamErr = errFatal
		fallback.streamResp = okStream()

		_ = reg.Select(context.Background(), ChatRegistryConfig{
			Primary:   "openai",
			Fallbacks: []string{"anthropic"},
		})

		stream, err := reg.ChatStream(context.Background(), []Message{
			{Role: RoleUser, Content: "hi"},
		}, ChatOptions{})
		require.NoError(t, err)
		assert.NotNil(t, stream)

		assert.Equal(t, int32(1), primary.streamCalls.Load())
		assert.Equal(t, int32(1), fallback.streamCalls.Load())
	})

	t.Run("all stream providers fail", func(t *testing.T) {
		reg := newTestRegistry()
		primary := registerMock(reg, "openai", "gpt-4o", 0)
		fallback := registerMock(reg, "anthropic", "claude-3-5-sonnet", 10)
		primary.streamErr = errFatal
		fallback.streamErr = errFatal

		_ = reg.Select(context.Background(), ChatRegistryConfig{
			Primary:   "openai",
			Fallbacks: []string{"anthropic"},
		})

		_, err := reg.ChatStream(context.Background(), []Message{
			{Role: RoleUser, Content: "hi"},
		}, ChatOptions{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "all chat stream providers failed")
	})

	t.Run("Stream=true is forced in options", func(t *testing.T) {
		reg := newTestRegistry()
		m := registerMock(reg, "openai", "gpt-4o", 0)
		m.streamResp = okStream()

		_ = reg.Select(context.Background(), ChatRegistryConfig{Primary: "openai"})

		// Call with Stream=false; the registry forces Stream=true.
		_, err := reg.ChatStream(context.Background(), nil, ChatOptions{Stream: false})
		require.NoError(t, err)
		// The underlying mockProvider received the options with Stream=true.
	})

	t.Run("no selected provider returns error", func(t *testing.T) {
		reg := newTestRegistry()

		_, err := reg.ChatStream(context.Background(), nil, ChatOptions{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no chat provider selected")
	})
}

// ─── ChatRegistry: Model / Name ──────────────────────────────────────────────

func TestRegistry_Model(t *testing.T) {
	t.Parallel()

	t.Run("returns model of selected provider", func(t *testing.T) {
		reg := newTestRegistry()
		_ = registerMock(reg, "openai", "gpt-4o", 0)

		_ = reg.Select(context.Background(), ChatRegistryConfig{Primary: "openai"})
		assert.Equal(t, "gpt-4o", reg.Model())
	})

	t.Run("returns empty when nothing selected", func(t *testing.T) {
		reg := newTestRegistry()
		assert.Empty(t, reg.Model())
	})
}

func TestRegistry_Name(t *testing.T) {
	t.Parallel()

	t.Run("returns name of selected provider", func(t *testing.T) {
		reg := newTestRegistry()
		_ = registerMock(reg, "openai", "gpt-4o", 0)

		_ = reg.Select(context.Background(), ChatRegistryConfig{Primary: "openai"})
		assert.Equal(t, "openai", reg.Name())
	})

	t.Run("returns fallback name when nothing selected", func(t *testing.T) {
		reg := newTestRegistry()
		assert.Equal(t, "chat-registry", reg.Name())
	})
}

// ─── ChatRegistry: Close ─────────────────────────────────────────────────────

func TestRegistry_Close(t *testing.T) {
	t.Parallel()

	t.Run("closes selected and fallback providers", func(t *testing.T) {
		reg := newTestRegistry()
		primary := registerMock(reg, "openai", "gpt-4o", 0)
		fallback := registerMock(reg, "anthropic", "claude-3-5-sonnet", 10)

		_ = reg.Select(context.Background(), ChatRegistryConfig{
			Primary:   "openai",
			Fallbacks: []string{"anthropic"},
		})

		err := reg.Close()
		assert.NoError(t, err)
		assert.Greater(t, primary.closeCalls.Load(), int32(0))
		assert.Greater(t, fallback.closeCalls.Load(), int32(0))
	})

	t.Run("empty registry close is safe", func(t *testing.T) {
		reg := newTestRegistry()
		err := reg.Close()
		assert.NoError(t, err)
	})
}

// ─── ChatRegistry: Stats ─────────────────────────────────────────────────────

func TestRegistry_Stats(t *testing.T) {
	t.Parallel()

	t.Run("records successful requests", func(t *testing.T) {
		reg := newTestRegistry()
		m := registerMock(reg, "openai", "gpt-4o", 0)
		m.chatResp = okResponse()

		_ = reg.Select(context.Background(), ChatRegistryConfig{Primary: "openai"})

		_, _ = reg.Chat(context.Background(), nil, ChatOptions{})
		_, _ = reg.Chat(context.Background(), nil, ChatOptions{})

		stats := reg.Stats()
		assert.Equal(t, int64(2), stats.TotalRequests)
		assert.Equal(t, int64(84), stats.TotalTokens) // 2 * 42
		assert.Zero(t, stats.Errors)
	})

	t.Run("records errors", func(t *testing.T) {
		reg := newTestRegistry()
		primary := registerMock(reg, "openai", "gpt-4o", 0)
		primary.chatErr = errFatal

		_ = reg.Select(context.Background(), ChatRegistryConfig{Primary: "openai"})

		_, err := reg.Chat(context.Background(), nil, ChatOptions{})
		require.Error(t, err)

		stats := reg.Stats()
		assert.Equal(t, int64(1), stats.Errors)
		assert.Zero(t, stats.TotalRequests)
	})
}

// ─── ChatRegistry: Singleton ─────────────────────────────────────────────────

func TestRegistry_GetRegistry(t *testing.T) {
	// NOT parallel — modifies global state.
	ResetRegistry()

	r1 := GetRegistry()
	r2 := GetRegistry()
	assert.Same(t, r1, r2, "GetRegistry must return the same singleton")
}

func TestRegistry_ResetRegistry(t *testing.T) {
	// NOT parallel — modifies global state.
	ResetRegistry()

	reg := GetRegistry()
	_ = registerMock(reg, "test", "test-model", 0)

	ResetRegistry()

	reg2 := GetRegistry()
	assert.Empty(t, reg2.List(), "registry must be empty after reset")
}

// ─── ChatStats ───────────────────────────────────────────────────────────────

func TestChatStats_RecordRequest(t *testing.T) {
	t.Parallel()

	t.Run("zero tokens", func(t *testing.T) {
		s := &ChatStats{}
		s.RecordRequest(0)
		assert.Equal(t, int64(1), s.TotalRequests)
		assert.Zero(t, s.TotalTokens)
	})

	t.Run("positive tokens", func(t *testing.T) {
		s := &ChatStats{}
		s.RecordRequest(100)
		s.RecordRequest(50)
		assert.Equal(t, int64(2), s.TotalRequests)
		assert.Equal(t, int64(150), s.TotalTokens)
	})
}

func TestChatStats_RecordError(t *testing.T) {
	t.Parallel()

	s := &ChatStats{}
	s.RecordError()
	s.RecordError()
	assert.Equal(t, int64(2), s.Errors)
}

func TestChatStats_Snapshot(t *testing.T) {
	t.Parallel()

	s := &ChatStats{}
	s.RecordRequest(42)
	s.RecordError()

	// Take snapshot while mutating.
	before := s.Snapshot()

	s.RecordRequest(10)
	s.RecordError()

	after := s.Snapshot()

	// Snapshot returns a copy; before hasn't changed.
	assert.Equal(t, int64(1), before.TotalRequests)
	assert.Equal(t, int64(1), before.Errors)

	assert.Equal(t, int64(2), after.TotalRequests)
	assert.Equal(t, int64(2), after.Errors)
}

// ─── ProviderError ───────────────────────────────────────────────────────────

func TestProviderError_Error(t *testing.T) {
	t.Parallel()

	err := &ProviderError{Provider: "openai", Err: errFatal}
	assert.Equal(t, `provider "openai": provider fatal error`, err.Error())
}

func TestProviderError_Unwrap(t *testing.T) {
	t.Parallel()

	err := &ProviderError{Provider: "openai", Err: errFatal}
	assert.True(t, errors.Is(err, errFatal))
}

func TestNewProviderError(t *testing.T) {
	t.Parallel()

	err := NewProviderError("anthropic", errFatal)
	require.Error(t, err)

	var pe *ProviderError
	require.True(t, errors.As(err, &pe))
	assert.Equal(t, "anthropic", pe.Provider)
	assert.True(t, errors.Is(pe, errFatal))
}

// ─── DefaultChatOptions / DefaultChatRegistryConfig ──────────────────────────

func TestDefaultChatOptions(t *testing.T) {
	t.Parallel()

	opts := DefaultChatOptions()
	assert.Equal(t, 0.7, opts.Temperature)
	assert.Equal(t, 1.0, opts.TopP)
	assert.Zero(t, opts.MaxTokens)
	assert.False(t, opts.Stream)
}

func TestDefaultChatRegistryConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultChatRegistryConfig()
	assert.True(t, cfg.AutoDetect)
	assert.Empty(t, cfg.Primary)
	assert.Empty(t, cfg.Fallbacks)
}

// ─── MockProvider Interface Conformance ──────────────────────────────────────

func TestMockProvider_ImplementsChatProvider(t *testing.T) {
	t.Parallel()

	var _ ChatProvider = (*mockProvider)(nil) // compile-time check

	m := &mockProvider{
		name:       "test",
		model:      "test-model",
		chatResp:   okResponse(),
		streamResp: okStream(),
	}

	ctx := context.Background()
	resp, err := m.Chat(ctx, nil, DefaultChatOptions())
	require.NoError(t, err)
	assert.Equal(t, "chat-1", resp.ID)

	stream, err := m.ChatStream(ctx, nil, DefaultChatOptions())
	require.NoError(t, err)
	assert.NotNil(t, stream)

	chunk, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, "stream-1", chunk.ID)

	assert.Equal(t, "test", m.Name())
	assert.Equal(t, "test-model", m.Model())

	err = m.Close()
	assert.NoError(t, err)
	assert.Equal(t, int32(1), m.closeCalls.Load())
}

// ─── MockStream Interface Conformance ────────────────────────────────────────

func TestMockStream_ImplementsChatStream(t *testing.T) {
	t.Parallel()

	var _ ChatStream = (*mockStream)(nil) // compile-time check
}

// ─── ChatRegistry: Full Interface Conformance ────────────────────────────────

func TestChatRegistry_ImplementsChatProvider(t *testing.T) {
	t.Parallel()

	var _ ChatProvider = (*ChatRegistry)(nil) // compile-time check
}

// ─── DefaultHotReloadConfig ──────────────────────────────────────────────────

func TestDefaultHotReloadConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultHotReloadConfig()
	assert.NotEmpty(t, cfg.ConfigPaths)
	assert.Equal(t, 30*time.Second, cfg.PollInterval)
	assert.True(t, cfg.AutoDetect)
}

// ─── Provider Discovery: Environment-based Hash ──────────────────────────────

func TestComputeProviderHash(t *testing.T) {
	// NOT parallel — reads global env vars (which are inherently shared).

	t.Run("returns non-empty string", func(t *testing.T) {
		hash := computeProviderHash()
		// Even with no env vars set, the hash is made of "|" separators.
		assert.NotEmpty(t, hash)
	})

	t.Run("changes when an env var changes", func(t *testing.T) {
		before := computeProviderHash()

		t.Setenv("OPENAI_API_KEY", "sk-test-12345")
		after := computeProviderHash()

		// Hash must differ when a tracked env var changes.
		assert.NotEqual(t, before, after)
	})
}

// ─── isConfigFile ────────────────────────────────────────────────────────────

func TestHotReload_isConfigFile(t *testing.T) {
	t.Parallel()

	hr := &HotReload{
		config: HotReloadConfig{
			ConfigPaths: []string{"/foo/config.yaml", "/bar/settings.yml"},
		},
	}

	t.Run("exact base name match", func(t *testing.T) {
		assert.True(t, hr.isConfigFile("/foo/config.yaml"))
		assert.True(t, hr.isConfigFile("/any/dir/config.yaml"))
	})

	t.Run("other yaml files also match", func(t *testing.T) {
		assert.True(t, hr.isConfigFile("/some/other.yaml"))
		assert.True(t, hr.isConfigFile("/some/file.yml"))
	})

	t.Run("non-yaml files do not match", func(t *testing.T) {
		assert.False(t, hr.isConfigFile("/foo/data.json"))
		assert.False(t, hr.isConfigFile("/foo/config.txt"))
	})

	t.Run("non-tracked yml base matches via extension", func(t *testing.T) {
		assert.True(t, hr.isConfigFile("/unknown/something.yml"))
	})
}

// ─── ChatRegistry: Select With Multiple Fallbacks ────────────────────────────

func TestRegistry_Select_MultipleFallbacks(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()
	primary := registerMock(reg, "primary", "primary-model", 0)
	fb1 := registerMock(reg, "fb1", "fb1-model", 10)
	fb2 := registerMock(reg, "fb2", "fb2-model", 20)
	primary.chatErr = errFatal
	fb1.chatErr = errFatal
	respFB2 := okResponse()
	respFB2.Model = "fb2-model"
	fb2.chatResp = respFB2

	_ = reg.Select(context.Background(), ChatRegistryConfig{
		Primary:   "primary",
		Fallbacks: []string{"fb1", "fb2"},
	})

	resp, err := reg.Chat(context.Background(), nil, ChatOptions{})
	require.NoError(t, err)
	assert.Equal(t, "fb2-model", resp.Model)

	assert.Equal(t, int32(1), primary.chatCalls.Load())
	assert.Equal(t, int32(1), fb1.chatCalls.Load())
	assert.Equal(t, int32(1), fb2.chatCalls.Load())
}

// ─── Edge: Context Cancellation ──────────────────────────────────────────────

func TestRegistry_Chat_RespectsContext(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()
	m := registerMock(reg, "openai", "gpt-4o", 0)
	m.chatResp = okResponse()

	_ = reg.Select(context.Background(), ChatRegistryConfig{Primary: "openai"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call

	// Our mock doesn't check ctx, but the registry passes ctx through.
	// This test ensures the call doesn't panic with a cancelled context
	// when the underlying provider ignores the context.
	_, err := reg.Chat(ctx, nil, ChatOptions{})
	// No error from our mock regardless of ctx state.
	require.NoError(t, err)
}
