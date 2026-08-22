package knowledge

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/CoscaAI/cosca/internal/search"
)

// TestEmbeddingSelectionConfig_NoOverrides verifies that with no override
// fields set the selection config matches the registry defaults exactly —
// preserving zero-config behavior.
func TestEmbeddingSelectionConfig_NoOverrides(t *testing.T) {
	e := &Engine{cfg: Config{}}
	selCfg := e.embeddingSelectionConfig()
	assert.Equal(t, embeddings.DefaultProviderRegistryConfig(), selCfg)
}

// TestEmbeddingSelectionConfig_CarriesOverrides verifies that the explicit
// embedding overrides (base URL, model, API key, dimensions) are copied into
// the registry selection config.
func TestEmbeddingSelectionConfig_CarriesOverrides(t *testing.T) {
	e := &Engine{cfg: Config{
		EmbeddingBaseURL:    "http://127.0.0.1:11435/v1",
		EmbeddingModel:      "text-embedding-3-small",
		EmbeddingAPIKey:     "local-key",
		EmbeddingDimensions: 768,
	}}
	selCfg := e.embeddingSelectionConfig()
	assert.Equal(t, "http://127.0.0.1:11435/v1", selCfg.BaseURL)
	assert.Equal(t, "text-embedding-3-small", selCfg.Model)
	assert.Equal(t, "local-key", selCfg.APIKey)
	assert.Equal(t, 768, selCfg.Dimensions)
}

// TestInit_EmbeddingOverridesFlowToFactory verifies the full plumbing: a
// knowledge engine configured with a local embedding base URL passes the
// overrides through registry selection into the provider factory.
func TestInit_EmbeddingOverridesFlowToFactory(t *testing.T) {
	embeddings.ResetRegistry()
	registry := embeddings.GetRegistry()

	var gotCfg *embeddings.Config
	registry.Register("openai", func(_ context.Context, cfg *embeddings.Config) (embeddings.Provider, error) {
		gotCfg = cfg
		return testEmbeddingProvider{}, nil
	}, "recording", 10)
	registry.Register("local", func(_ context.Context, _ *embeddings.Config) (embeddings.Provider, error) {
		return testEmbeddingProvider{}, nil
	}, "deterministic test provider", 100)

	cfg := Config{
		DBPath:              filepath.Join(t.TempDir(), "override.db"),
		RootDir:             t.TempDir(),
		AutoMigrate:         true,
		EmbeddingProvider:   "openai",
		EmbeddingBaseURL:    "http://127.0.0.1:11435/v1",
		EmbeddingModel:      "text-embedding-3-small",
		EmbeddingAPIKey:     "local-key",
		EmbeddingDimensions: 768,
		IndexerConfig:       DefaultConfig().IndexerConfig,
		CacheConfig:         DefaultConfig().CacheConfig,
		RankingConfig:       DefaultConfig().RankingConfig,
		SearchConfig:        search.DefaultSearchParams(),
	}

	ke, err := New(cfg)
	require.NoError(t, err)
	defer func() { _ = ke.Close() }()

	require.NoError(t, ke.Init(), "Init should succeed with the recording provider")

	require.NotNil(t, gotCfg, "openai factory should have received an override Config")
	assert.Equal(t, "http://127.0.0.1:11435/v1", gotCfg.BaseURL)
	assert.Equal(t, "text-embedding-3-small", gotCfg.Model)
	assert.Equal(t, "local-key", gotCfg.APIKey)
	assert.Equal(t, 768, gotCfg.Dimensions)
}

// TestInit_NoOverridesPassesNilToFactory verifies that without overrides the
// factory receives nil, preserving the historical nil-config contract.
func TestInit_NoOverridesPassesNilToFactory(t *testing.T) {
	embeddings.ResetRegistry()
	registry := embeddings.GetRegistry()

	var gotCfg *embeddings.Config
	registry.Register("openai", func(_ context.Context, cfg *embeddings.Config) (embeddings.Provider, error) {
		gotCfg = cfg
		return testEmbeddingProvider{}, nil
	}, "recording", 10)
	registry.Register("local", func(_ context.Context, _ *embeddings.Config) (embeddings.Provider, error) {
		return testEmbeddingProvider{}, nil
	}, "deterministic test provider", 100)

	cfg := Config{
		DBPath:            filepath.Join(t.TempDir(), "nil.db"),
		RootDir:           t.TempDir(),
		AutoMigrate:       true,
		EmbeddingProvider: "openai",
		IndexerConfig:     DefaultConfig().IndexerConfig,
		CacheConfig:       DefaultConfig().CacheConfig,
		RankingConfig:     DefaultConfig().RankingConfig,
		SearchConfig:      search.DefaultSearchParams(),
	}

	ke, err := New(cfg)
	require.NoError(t, err)
	defer func() { _ = ke.Close() }()

	require.NoError(t, ke.Init(), "Init should succeed with the recording provider")

	assert.Nil(t, gotCfg, "openai factory should have received nil Config when no overrides are set")
}
