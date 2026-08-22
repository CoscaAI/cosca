// Package google provides embedding generation via Google AI (Gemini) API.
package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/CoscaAI/cosca/internal/providers"
	"github.com/CoscaAI/cosca/internal/safe"
	"github.com/rs/zerolog/log"
)

// Default model names.
const (
	ModelTextEmbedding004 = "text-embedding-004"
	ModelGecko            = "textembedding-gecko@003"
)

// Config defines configuration for the Google AI embedding provider.
type Config struct {
	// APIKey is the Google AI API key (from env GOOGLE_API_KEY or config).
	APIKey string

	// Model is the embedding model name.
	Model string

	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int

	// Timeout is the HTTP request timeout.
	Timeout time.Duration

	// BatchSize is the maximum number of texts per request.
	BatchSize int
}

// DefaultConfig returns a sensible default configuration.
// NOTE: The APIKey field is intentionally left empty to avoid coupling to environment variables.
// Use ConfigFromEnv to load API key from environment.
func DefaultConfig() Config {
	return Config{
		Model:      ModelTextEmbedding004,
		MaxRetries: 3,
		Timeout:    60 * time.Second,
		BatchSize:  10,
	}
}

// ConfigFromEnv returns a Config populated from environment variables.
// It starts from DefaultConfig and overlays any non-empty env vars.
func ConfigFromEnv() Config {
	cfg := DefaultConfig()
	if v := os.Getenv("GOOGLE_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	return cfg
}

// Provider implements the embeddings.Provider interface for Google AI.
type Provider struct {
	cfg         Config
	client      *http.Client
	modelDims   int
	mu          sync.Mutex
	rateLimiter *providers.RateLimiter
}

// New creates a new Google AI embedding provider.
func New(cfg Config) (*Provider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("GOOGLE_API_KEY not set")
	}
	if cfg.Model == "" {
		cfg.Model = ModelTextEmbedding004
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 10
	}

	p := &Provider{
		cfg:         cfg,
		client:      providers.SharedHTTPClient(cfg.Timeout),
		modelDims:   768, // default for text-embedding-004
		rateLimiter: providers.NewRateLimiter(1500),
	}

	log.Debug().
		Str("model", cfg.Model).
		Msg("google ai embedding provider initialized")

	return p, nil
}

// Name returns the provider name.
func (p *Provider) Name() string { return "google" }

// Model returns the model name.
func (p *Provider) Model() string { return p.cfg.Model }

// Dimensions returns the embedding dimensionality.
func (p *Provider) Dimensions() int { return p.modelDims }

// Close cleans up provider resources.
func (p *Provider) Close() error { return nil }

// GenerateEmbedding generates an embedding for a single text string.
func (p *Provider) GenerateEmbedding(ctx context.Context, text string) (*embeddings.EmbeddingResult, error) {
	results, err := p.GenerateEmbeddings(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no embedding result returned")
	}
	return results[0], nil
}

// GenerateEmbeddings generates embeddings for a batch of texts.
func (p *Provider) GenerateEmbeddings(ctx context.Context, texts []string) ([]*embeddings.EmbeddingResult, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}

	// Process in batches
	var allResults []*embeddings.EmbeddingResult
	for i := 0; i < len(texts); i += p.cfg.BatchSize {
		end := i + p.cfg.BatchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		batchResults, err := p.embedBatch(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("batch %d: %w", i/p.cfg.BatchSize, err)
		}
		allResults = append(allResults, batchResults...)
	}

	return allResults, nil
}

// embedBatch sends a batch of texts to the Google AI embedding API.
func (p *Provider) embedBatch(ctx context.Context, texts []string) ([]*embeddings.EmbeddingResult, error) {
	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:batchEmbedContents?key=%s",
		p.cfg.Model, p.cfg.APIKey)

	type embedContentRequest struct {
		Model   string `json:"model"`
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	}

	requests := make([]embedContentRequest, len(texts))
	for i, text := range texts {
		requests[i].Model = fmt.Sprintf("models/%s", p.cfg.Model)
		requests[i].Content.Parts = []struct {
			Text string `json:"text"`
		}{{Text: text}}
	}

	requestBody := struct {
		Requests []embedContentRequest `json:"requests"`
	}{
		Requests: requests,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= p.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * time.Second
			timer := time.NewTimer(backoff)
			select {
			case <-timer.C:
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return nil, ctx.Err()
			}
		}

		if err := p.rateLimiter.Wait(ctx, 1); err != nil {
			return nil, fmt.Errorf("rate limit wait: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Goog-Api-Key", p.cfg.APIKey)

		resp, err := p.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http request: %w", err)
			continue
		}

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
		safe.Close(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("read response: %w", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("google: status %d: %s", resp.StatusCode, providers.TruncateBody(string(respBody)))
			if isQuotaError(resp.StatusCode) {
				// Quota errors are transient
				continue
			}
			return nil, lastErr
		}

		var apiResponse struct {
			Embeddings []struct {
				Values []float64 `json:"values"`
			} `json:"embeddings"`
		}

		if err := json.Unmarshal(respBody, &apiResponse); err != nil {
			lastErr = fmt.Errorf("parse response: %w", err)
			continue
		}

		results := make([]*embeddings.EmbeddingResult, len(apiResponse.Embeddings))
		for i, emb := range apiResponse.Embeddings {
			p.mu.Lock()
			p.modelDims = len(emb.Values)
			p.mu.Unlock()

			results[i] = &embeddings.EmbeddingResult{
				Vector:     emb.Values,
				Model:      p.cfg.Model,
				Dimensions: len(emb.Values),
				TokensUsed: estimateTokens(texts[i]),
			}
		}

		return results, nil
	}

	return nil, fmt.Errorf("google embedding failed after %d retries: %w", p.cfg.MaxRetries, lastErr)
}

// isQuotaError checks if the status code indicates a quota/rate limit error.
func isQuotaError(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests ||
		statusCode == http.StatusServiceUnavailable ||
		statusCode == 429
}

// estimateTokens provides a rough token count estimate.
func estimateTokens(text string) int {
	return len(text) / 4
}

// truncateBody truncates long error bodies.

// Factory returns an embeddings.ProviderFactory for the Google provider.
func Factory() (string, embeddings.ProviderFactory) {
	return "google", func(_ context.Context, cfg *embeddings.Config) (embeddings.Provider, error) {
		config := ConfigFromEnv()
		if cfg != nil {
			if cfg.APIKey != "" {
				config.APIKey = cfg.APIKey
			}
			if cfg.Model != "" {
				config.Model = cfg.Model
			}
		}
		return New(config)
	}
}

// Register registers this provider with the global embedding provider registry.
func Register() {
	registry := embeddings.GetRegistry()
	name, factory := Factory()
	registry.Register(name, factory, "Google AI Embeddings API", 30)
}

// rateLimiter implements a simple token-bucket rate limiter.
