// Package mistral provides embedding generation via Mistral AI's Embeddings API.
// The implementation is shared via internal/providers/openaicompat.
package mistral

import (
	"context"

	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/CoscaAI/cosca/internal/providers/openaicompat"
)

// ─── Re-exported types (backward compatibility) ──────────────────────────────

// Config is the runtime configuration for the Mistral AI embedding provider.
type Config = openaicompat.EmbeddingConfig

// Provider implements embeddings.Provider for Mistral AI.
type Provider = openaicompat.EmbeddingProvider

// ─── Provider constants ──────────────────────────────────────────────────────

// DefaultModel is the default model name for the Mistral provider.
const (
	DefaultModel   = "mistral-embed"
	DefaultDim     = 1024
	DefaultBaseURL = "https://api.mistral.ai/v1/embeddings"
)

// ─── Provider config (used internally) ───────────────────────────────────────

var embProviderConfig = openaicompat.EmbeddingProviderConfig{
	ProviderName:     "mistral",
	DefaultModel:     DefaultModel,
	DefaultDims:      DefaultDim,
	DefaultBaseURL:   DefaultBaseURL,
	EnvVarName:       "MISTRAL_API_KEY",
	RegistryName:     "mistral",
	RegistryPriority: 40,
	Description:      "Mistral AI Embeddings API",
	RateLimitTPM:     500000,
}

// ─── Public API ──────────────────────────────────────────────────────────────

// DefaultConfig returns a sensible default configuration.
// NOTE: The APIKey field is intentionally left empty to avoid coupling to environment variables.
// Use ConfigFromEnv to load API key from environment.
func DefaultConfig() Config {
	return openaicompat.DefaultEmbeddingConfig(embProviderConfig)
}

// ConfigFromEnv returns a Config populated from environment variables.
// It starts from DefaultConfig and overlays any non-empty env vars.
func ConfigFromEnv() Config {
	return openaicompat.EmbeddingConfigFromEnv(embProviderConfig)
}

// New creates a new Mistral AI embedding provider.
func New(cfg Config) (*Provider, error) {
	return openaicompat.NewEmbeddingProvider(embProviderConfig, cfg)
}

// Factory returns an embeddings.ProviderFactory for the Mistral provider.
func Factory() (string, embeddings.ProviderFactory) {
	return "mistral", func(_ context.Context, cfg *embeddings.Config) (embeddings.Provider, error) {
		config := ConfigFromEnv()
		if cfg != nil {
			if cfg.APIKey != "" {
				config.APIKey = cfg.APIKey
			}
			if cfg.Model != "" {
				config.Model = cfg.Model
			}
			if cfg.Dimensions > 0 {
				config.Dimensions = cfg.Dimensions
			}
			if cfg.BaseURL != "" {
				config.BaseURL = cfg.BaseURL
			}
		}
		return New(config)
	}
}

// Register registers this provider with the global embedding provider registry.
func Register() {
	openaicompat.RegisterEmbedding(embProviderConfig)
}
