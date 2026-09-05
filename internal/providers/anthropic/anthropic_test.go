package anthropic

import (
	"context"
	"testing"

	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time assertion that Provider satisfies the embeddings.Provider interface.
var _ embeddings.Provider = (*Provider)(nil)

// wantModel is the model name hardcoded in Provider.Model().
const wantModel = "claude-3-haiku-20240307"

// validTestConfig returns a Config with an API key for successful construction.
func validTestConfig() Config {
	cfg := DefaultConfig()
	cfg.APIKey = "test-anthropic-key"
	return cfg
}

// ── Configuration ──────────────────────────────────────────────────────────────

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// The anthropic provider only carries an API key; everything else is static.
	// APIKey is intentionally left empty to avoid coupling to environment vars.
	assert.Empty(t, cfg.APIKey)
	assert.Equal(t, Config{}, cfg)
}

func TestConfigFromEnv(t *testing.T) {
	t.Run("with env var set", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "env-key-789")
		cfg := ConfigFromEnv()
		assert.Equal(t, "env-key-789", cfg.APIKey)
	})

	t.Run("without env var", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "")
		cfg := ConfigFromEnv()
		assert.Empty(t, cfg.APIKey)
	})
}

// ── Construction ───────────────────────────────────────────────────────────────

func TestNew(t *testing.T) {
	t.Run("missing API key returns error", func(t *testing.T) {
		p, err := New(DefaultConfig())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ANTHROPIC_API_KEY")
		assert.Contains(t, err.Error(), "not set")
		assert.Nil(t, p)
	})

	t.Run("valid config returns provider", func(t *testing.T) {
		p, err := New(validTestConfig())
		require.NoError(t, err)
		require.NotNil(t, p)
		assert.Equal(t, "test-anthropic-key", p.cfg.APIKey)
	})
}

// ── Accessors ─────────────────────────────────────────────────────────────────

func TestProviderAccessors(t *testing.T) {
	p, err := New(validTestConfig())
	require.NoError(t, err)

	assert.Equal(t, "anthropic", p.Name())
	assert.Equal(t, wantModel, p.Model())
	// Anthropic has no embedding API, so dimensions are always 0.
	assert.Equal(t, 0, p.Dimensions())
	assert.NoError(t, p.Close())
}

// ── GenerateEmbedding / GenerateEmbeddings (thin provider: always errors) ─────

func TestGenerateEmbedding_NoEmbeddingAPI(t *testing.T) {
	p, err := New(validTestConfig())
	require.NoError(t, err)

	result, err := p.GenerateEmbedding(context.Background(), "hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not provide an embedding API")
	assert.Nil(t, result)
}

func TestGenerateEmbeddings_NoEmbeddingAPI(t *testing.T) {
	p, err := New(validTestConfig())
	require.NoError(t, err)

	results, err := p.GenerateEmbeddings(context.Background(), []string{"hello", "world"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not provide an embedding API")
	assert.Nil(t, results)
}

// ── Factory ───────────────────────────────────────────────────────────────────

func TestFactory(t *testing.T) {
	t.Run("returns name and provider factory", func(t *testing.T) {
		name, factory := Factory()
		assert.Equal(t, "anthropic", name)
		require.NotNil(t, factory)
	})

	t.Run("env var set succeeds", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "factory-key")
		_, factory := Factory()
		p, err := factory(context.Background(), nil)
		require.NoError(t, err)
		assert.Equal(t, "anthropic", p.Name())
		assert.Equal(t, wantModel, p.Model())
	})

	t.Run("env var unset fails", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "")
		_, factory := Factory()
		_, err := factory(context.Background(), nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ANTHROPIC_API_KEY")
	})

	t.Run("provided config is ignored", func(t *testing.T) {
		// The anthropic factory builds from ConfigFromEnv() only and ignores the
		// passed embeddings.Config, so an API key in cfg cannot compensate for a
		// missing environment variable. This documents the current behavior.
		t.Setenv("ANTHROPIC_API_KEY", "")
		_, factory := Factory()
		_, err := factory(context.Background(), &embeddings.Config{APIKey: "ignored-key"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ANTHROPIC_API_KEY")
	})
}

// ── Register ──────────────────────────────────────────────────────────────────

func TestRegister(t *testing.T) {
	// NOTE: sequential (no t.Parallel) because it touches the global registry
	// and uses t.Setenv.

	embeddings.ResetRegistry()
	t.Setenv("ANTHROPIC_API_KEY", "register-key")

	Register()

	registry := embeddings.GetRegistry()
	names := registry.List()
	assert.Contains(t, names, "anthropic")

	provider, ok := registry.Get("anthropic")
	require.True(t, ok, "anthropic provider should be retrievable from registry")
	require.NotNil(t, provider)
	assert.Equal(t, "anthropic", provider.Name())
	assert.Equal(t, wantModel, provider.Model())
}
