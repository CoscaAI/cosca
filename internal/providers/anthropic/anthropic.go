package anthropic

import (
	"context"
	"fmt"
	"os"

	"github.com/CoscaAI/cosca/internal/embeddings"
)

// Config holds the configuration for the Anthropic provider.
type Config struct {
	APIKey string
}

// DefaultConfig returns a Config with sensible defaults.
// NOTE: The APIKey field is intentionally left empty to avoid coupling to environment variables.
// Use ConfigFromEnv to load API key from environment.
func DefaultConfig() Config {
	return Config{}
}

// ConfigFromEnv returns a Config populated from environment variables.
// It starts from DefaultConfig and overlays any non-empty env vars.
func ConfigFromEnv() Config {
	cfg := DefaultConfig()
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	return cfg
}

// Provider implements the embeddings.Provider interface for Anthropic.
type Provider struct {
	cfg Config
}

// New creates a new Anthropic provider.
func New(cfg Config) (*Provider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}
	return &Provider{cfg: cfg}, nil
}

// Name returns the provider name.
func (p *Provider) Name() string { return "anthropic" }

// Model returns the model name.
func (p *Provider) Model() string { return "claude-3-haiku-20240307" }

// Dimensions returns the embedding dimension count.
func (p *Provider) Dimensions() int { return 0 }

// Close releases resources held by the provider.
func (p *Provider) Close() error { return nil }

// GenerateEmbedding generates an embedding for a single text.
func (p *Provider) GenerateEmbedding(_ context.Context, _ string) (*embeddings.EmbeddingResult, error) {
	return nil, fmt.Errorf("anthropic does not provide an embedding API")
}

// GenerateEmbeddings generates embeddings for multiple texts.
func (p *Provider) GenerateEmbeddings(_ context.Context, _ []string) ([]*embeddings.EmbeddingResult, error) {
	return nil, fmt.Errorf("anthropic does not provide an embedding API")
}

// Factory returns the provider name and a factory function for registration.
func Factory() (string, embeddings.ProviderFactory) {
	return "anthropic", func(_ context.Context, _ *embeddings.Config) (embeddings.Provider, error) {
		return New(ConfigFromEnv())
	}
}

// Register registers this provider with the global embedding provider registry.
func Register() {
	embeddings.GetRegistry().Register("anthropic", func(_ context.Context, _ *embeddings.Config) (embeddings.Provider, error) {
		return New(ConfigFromEnv())
	}, "Anthropic Claude (no embedding API - LLM only)", 200)
}
