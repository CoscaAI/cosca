package deepseek

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/CoscaAI/cosca/internal/providers"
	"github.com/CoscaAI/cosca/internal/safe"
)

const (
	defaultModel = "deepseek-embedding"
	defaultDims  = 1024
	defaultURL   = "https://api.deepseek.com/v1/embeddings"
)

// Config holds the configuration for the DeepSeek provider.
type Config struct {
	APIKey     string
	Model      string
	Dimensions int
	MaxRetries int
	Timeout    time.Duration
	BaseURL    string
	BatchSize  int
}

// DefaultConfig returns a Config with sensible defaults.
// NOTE: The APIKey field is intentionally left empty to avoid coupling to environment variables.
// Use ConfigFromEnv to load API key from environment.
func DefaultConfig() Config {
	return Config{
		Model:      defaultModel,
		Dimensions: defaultDims,
		MaxRetries: 3,
		Timeout:    60 * time.Second,
		BaseURL:    defaultURL,
		BatchSize:  20,
	}
}

// ConfigFromEnv returns a Config populated from environment variables.
// It starts from DefaultConfig and overlays any non-empty env vars.
func ConfigFromEnv() Config {
	cfg := DefaultConfig()
	if v := os.Getenv("DEEPSEEK_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	return cfg
}

// Provider implements the embeddings.Provider interface for DeepSeek.
type Provider struct {
	cfg         Config
	client      *http.Client
	rateLimiter *providers.RateLimiter
}

// New creates a new DeepSeek provider.
func New(cfg Config) (*Provider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("DEEPSEEK_API_KEY not set")
	}
	return &Provider{
		cfg:         cfg,
		client:      providers.SharedHTTPClient(cfg.Timeout),
		rateLimiter: providers.NewRateLimiter(500),
	}, nil
}

// Name returns the provider name.
func (p *Provider) Name() string { return "deepseek" }

// Model returns the model name.
func (p *Provider) Model() string { return p.cfg.Model }

// Dimensions returns the embedding dimension count.
func (p *Provider) Dimensions() int { return p.cfg.Dimensions }

// Close releases resources held by the provider.
func (p *Provider) Close() error { return nil }

// GenerateEmbedding generates an embedding for a single text.
func (p *Provider) GenerateEmbedding(ctx context.Context, text string) (*embeddings.EmbeddingResult, error) {
	results, err := p.GenerateEmbeddings(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no embedding result")
	}
	return results[0], nil
}

// GenerateEmbeddings generates embeddings for multiple texts.
func (p *Provider) GenerateEmbeddings(ctx context.Context, texts []string) ([]*embeddings.EmbeddingResult, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}

	if err := p.rateLimiter.Wait(ctx, 1); err != nil {
		return nil, fmt.Errorf("rate limit wait: %w", err)
	}

	reqBody := map[string]interface{}{
		"model": p.cfg.Model,
		"input": texts,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.cfg.BaseURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	results := make([]*embeddings.EmbeddingResult, len(apiResp.Data))
	for i, d := range apiResp.Data {
		results[i] = &embeddings.EmbeddingResult{
			Vector:     d.Embedding,
			Model:      p.cfg.Model,
			Dimensions: len(d.Embedding),
			TokensUsed: apiResp.Usage.TotalTokens / len(apiResp.Data),
		}
	}

	return results, nil
}

// Register registers this provider with the global embedding provider registry.
func Register() {
	embeddings.GetRegistry().Register("deepseek", func(_ context.Context, _ *embeddings.Config) (embeddings.Provider, error) {
		return New(ConfigFromEnv())
	}, "DeepSeek Embeddings API", 70)
}
