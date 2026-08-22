package chat

import (
	"context"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/circuitbreaker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── providerHealth: circuit breaker states ──────────────────────────────────

func TestProviderHealth_HealthStates(t *testing.T) {
	t.Parallel()

	t.Run("new tracker starts closed (health 1.0)", func(t *testing.T) {
		h := newProviderHealth()
		assert.Equal(t, 1.0, h.health())
	})

	t.Run("five consecutive failures open the circuit (health 0.0)", func(t *testing.T) {
		h := newProviderHealth()
		for i := 0; i < circuitbreaker.DefaultConfig().FailureThreshold; i++ {
			h.recordFailure()
		}
		assert.Equal(t, 0.0, h.health())
	})

	t.Run("half-open reports health 0.5", func(t *testing.T) {
		// Short-timeout breaker so the test does not wait the 30s default.
		h := &providerHealth{
			breaker: circuitbreaker.New(circuitbreaker.Config{
				FailureThreshold: 2,
				SuccessThreshold: 1,
				Timeout:          20 * time.Millisecond,
			}),
			latencyEMA: defaultLatencyEMA,
		}
		h.recordFailure()
		h.recordFailure()
		require.Equal(t, 0.0, h.health(), "circuit must be open after two failures")

		time.Sleep(30 * time.Millisecond)
		assert.Equal(t, 0.5, h.health(), "circuit must transition to half-open after timeout")
	})

	t.Run("successes in half-open recover to closed", func(t *testing.T) {
		h := &providerHealth{
			breaker: circuitbreaker.New(circuitbreaker.Config{
				FailureThreshold: 2,
				SuccessThreshold: 1,
				Timeout:          20 * time.Millisecond,
			}),
			latencyEMA: defaultLatencyEMA,
		}
		h.recordFailure()
		h.recordFailure()
		time.Sleep(30 * time.Millisecond)
		require.Equal(t, 0.5, h.health())

		h.recordSuccess(1 * time.Millisecond)
		assert.Equal(t, 1.0, h.health(), "one success in half-open must close the circuit")
	})

	t.Run("reset restores closed state and neutral latency", func(t *testing.T) {
		h := newProviderHealth()
		for i := 0; i < circuitbreaker.DefaultConfig().FailureThreshold; i++ {
			h.recordFailure()
		}
		require.Equal(t, 0.0, h.health())

		h.reset()
		assert.Equal(t, 1.0, h.health())
		assert.Equal(t, defaultLatencyEMA, h.latency())
		assert.Zero(t, h.samples)
	})
}

// ─── providerHealth: latency EMA ─────────────────────────────────────────────

func TestProviderHealth_LatencyEMA(t *testing.T) {
	t.Parallel()

	t.Run("low latency converges toward 1.0", func(t *testing.T) {
		h := newProviderHealth()
		for i := 0; i < 10; i++ {
			h.recordSuccess(1 * time.Millisecond)
		}
		assert.Greater(t, h.latency(), 0.9, "fast provider should score near 1.0")
	})

	t.Run("high latency converges toward 0", func(t *testing.T) {
		h := newProviderHealth()
		for i := 0; i < 10; i++ {
			h.recordSuccess(2 * time.Second)
		}
		assert.Less(t, h.latency(), 0.25, "slow provider should score near 0.2")
	})

	t.Run("fast provider outscores slow provider", func(t *testing.T) {
		fast := newProviderHealth()
		slow := newProviderHealth()
		for i := 0; i < 5; i++ {
			fast.recordSuccess(5 * time.Millisecond)
			slow.recordSuccess(3 * time.Second)
		}
		assert.Greater(t, fast.latency(), slow.latency())
	})

	t.Run("initial latency is neutral 0.5", func(t *testing.T) {
		h := newProviderHealth()
		assert.Equal(t, 0.5, h.latency())
	})
}

// ─── costScore ───────────────────────────────────────────────────────────────

func TestCostScore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		model string
		want  float64
	}{
		{name: "ollama", model: "llama3", want: 1.0},
		{name: "local", model: "local-model", want: 1.0},
		{name: "localhost", model: "x", want: 1.0},
		{name: "deepseek", model: "deepseek-chat", want: 0.8},
		{name: "groq", model: "llama-3", want: 0.8},
		{name: "openai", model: "gpt-4o", want: 0.4},
		{name: "gemini", model: "gemini-pro", want: 0.4},
		{name: "anthropic", model: "claude-sonnet", want: 0.4},
		{name: "azure", model: "gpt-4o", want: 0.5},
		{name: "bedrock", model: "claude", want: 0.5},
		{name: "mistral", model: "mistral-small", want: 0.5},
		{name: "unknown-provider", model: "some-model", want: 0.5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, costScore(tc.name, tc.model))
		})
	}
}

// ─── ChatRegistry.Select with Mode ───────────────────────────────────────────

// seedHealth records n successes with the given duration on a registry's
// tracker for name, so Select sees a realistic latency signal.
func seedHealth(r *ChatRegistry, name string, dur time.Duration, n int) {
	h := r.providerHealth(name)
	for i := 0; i < n; i++ {
		h.recordSuccess(dur)
	}
}

func TestRegistry_Select_ModeEmpty_PreservesOrder(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()
	openai := registerMock(reg, "openai", "gpt-4o", 0)
	anthropic := registerMock(reg, "anthropic", "claude-model", 10)
	deepseek := registerMock(reg, "deepseek", "deepseek-model", 20)

	// Metrics noise that WOULD reorder under routing, to prove Mode "" ignores it.
	seedHealth(reg, "deepseek", 1*time.Millisecond, 5)
	seedHealth(reg, "openai", 3*time.Second, 5)

	err := reg.Select(context.Background(), ChatRegistryConfig{
		Primary:   "openai",
		Fallbacks: []string{"anthropic", "deepseek"},
	})
	require.NoError(t, err)

	// Fixed order is preserved: primary first, then fallbacks in order.
	assert.Equal(t, "openai", reg.Name())

	openai.chatErr = errFatal
	anthropic.chatResp = okResponse()
	anthropic.chatResp.Model = "claude-model"
	deepseek.chatResp = okResponse()
	deepseek.chatResp.Model = "deepseek-model"

	resp, err := reg.Chat(context.Background(), nil, ChatOptions{})
	require.NoError(t, err)
	// anthropic is the first fallback, so it must be tried and succeed before
	// deepseek — proving the historical order is byte-for-byte preserved.
	assert.Equal(t, "claude-model", resp.Model)
	assert.Equal(t, int32(1), anthropic.chatCalls.Load())
	assert.Equal(t, int32(0), deepseek.chatCalls.Load())
}

func TestRegistry_Select_ModeFast_ReroutesChain(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()
	deepseek := registerMock(reg, "deepseek", "deepseek-chat", 0)
	ollama := registerMock(reg, "ollama", "llama3", 10)

	// ollama is fast/cheap; deepseek is slow — the fixed order would pick
	// deepseek, but Mode "fast" must promote ollama to primary.
	seedHealth(reg, "ollama", 1*time.Millisecond, 5)
	seedHealth(reg, "deepseek", 3*time.Second, 5)

	err := reg.Select(context.Background(), ChatRegistryConfig{
		Primary:   "deepseek",
		Fallbacks: []string{"ollama"},
		Mode:      "fast",
	})
	require.NoError(t, err)

	assert.Equal(t, "ollama", reg.Name(), "fast mode must reorder the chain by score")

	// Reordered fallback chain still works end-to-end.
	ollama.chatErr = errFatal
	deepseek.chatResp = okResponse()
	resp, err := reg.Chat(context.Background(), nil, ChatOptions{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestRegistry_Select_ModeOffline_NoOfflineProvider(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()
	_ = registerMock(reg, "openai", "gpt-4o", 0)
	_ = registerMock(reg, "anthropic", "claude-model", 10)

	// No local provider registered: ranking fails (ErrNoOfflineProvider), so
	// the current order must be kept — fail-open, no crash.
	err := reg.Select(context.Background(), ChatRegistryConfig{
		Primary:   "openai",
		Fallbacks: []string{"anthropic"},
		Mode:      "offline",
	})
	require.NoError(t, err)
	assert.Equal(t, "openai", reg.Name())
}

func TestRegistry_Select_ModeOffline_PrefersOffline(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()
	_ = registerMock(reg, "openai", "gpt-4o", 0)
	_ = registerMock(reg, "ollama", "llama3", 10)

	// Even with openai much faster, Mode "offline" must restrict to ollama.
	seedHealth(reg, "openai", 1*time.Millisecond, 5)
	seedHealth(reg, "ollama", 3*time.Second, 5)

	err := reg.Select(context.Background(), ChatRegistryConfig{
		Primary:   "openai",
		Fallbacks: []string{"ollama"},
		Mode:      "offline",
	})
	require.NoError(t, err)
	assert.Equal(t, "ollama", reg.Name(), "offline mode must select the local provider")
}

func TestRegistry_Select_ModeInvalid_FailOpen(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()
	_ = registerMock(reg, "openai", "gpt-4o", 0)

	// An unknown mode must not break selection: warn + keep current order.
	err := reg.Select(context.Background(), ChatRegistryConfig{
		Primary: "openai",
		Mode:    "modo-invalido",
	})
	require.NoError(t, err)
	assert.Equal(t, "openai", reg.Name())
}

// ─── Chat/ChatStream update provider health (passive tracking) ───────────────

func TestRegistry_Chat_UpdatesProviderHealth(t *testing.T) {
	t.Parallel()

	t.Run("success records success + latency", func(t *testing.T) {
		reg := newTestRegistry()
		m := registerMock(reg, "openai", "gpt-4o", 0)
		m.chatResp = okResponse()

		_ = reg.Select(context.Background(), ChatRegistryConfig{Primary: "openai"})

		_, err := reg.Chat(context.Background(), nil, ChatOptions{})
		require.NoError(t, err)

		h := reg.providerHealth("openai")
		assert.Equal(t, 1.0, h.health())
		assert.Greater(t, h.latency(), 0.5, "a fast mock call should beat the neutral score")
		assert.Equal(t, 1, h.samples)
	})

	t.Run("repeated failures open the circuit", func(t *testing.T) {
		reg := newTestRegistry()
		m := registerMock(reg, "openai", "gpt-4o", 0)
		m.chatErr = errFatal

		_ = reg.Select(context.Background(), ChatRegistryConfig{Primary: "openai"})

		for i := 0; i < circuitbreaker.DefaultConfig().FailureThreshold; i++ {
			_, err := reg.Chat(context.Background(), nil, ChatOptions{})
			require.Error(t, err)
		}

		assert.Equal(t, 0.0, reg.providerHealth("openai").health())
	})

	t.Run("fallback failure is tracked too", func(t *testing.T) {
		reg := newTestRegistry()
		primary := registerMock(reg, "openai", "gpt-4o", 0)
		fallback := registerMock(reg, "anthropic", "claude-model", 10)
		primary.chatErr = errFatal
		fallback.chatResp = okResponse()

		_ = reg.Select(context.Background(), ChatRegistryConfig{
			Primary:   "openai",
			Fallbacks: []string{"anthropic"},
		})

		_, err := reg.Chat(context.Background(), nil, ChatOptions{})
		require.NoError(t, err)

		// A single failure does not trip the circuit (default threshold is 5),
		// but it must be recorded in the breaker stats.
		openaiStats := reg.providerHealth("openai").breaker.Stats()
		assert.Equal(t, int64(1), openaiStats.Failures, "primary failure must be tracked")
		assert.Equal(t, 1.0, reg.providerHealth("openai").health(), "circuit still closed after one failure")
		assert.Equal(t, 1.0, reg.providerHealth("anthropic").health())
	})
}

func TestRegistry_ChatStream_UpdatesProviderHealth(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()
	m := registerMock(reg, "openai", "gpt-4o", 0)
	m.streamResp = okStream()

	_ = reg.Select(context.Background(), ChatRegistryConfig{Primary: "openai"})

	stream, err := reg.ChatStream(context.Background(), nil, ChatOptions{})
	require.NoError(t, err)
	require.NotNil(t, stream)

	assert.Equal(t, 1.0, reg.providerHealth("openai").health())
	assert.Greater(t, reg.providerHealth("openai").latency(), 0.5)
	assert.Equal(t, 1, reg.providerHealth("openai").samples)
}
