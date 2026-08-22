// Package openai provides embedding generation via OpenAI's Embeddings API.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/CoscaAI/cosca/internal/providers"
	"github.com/CoscaAI/cosca/internal/safe"
	"github.com/rs/zerolog/log"
)

// Default models and their dimensions.
const (
	ModelSmall = "text-embedding-3-small"
	ModelLarge = "text-embedding-3-large"
	ModelAda   = "text-embedding-ada-002"

	DimSmall = 512
	DimLarge = 256
	DimAda   = 1536
)

// Local default endpoint, model and dimensions (Don's order 2026-08-07):
// the provider defaults to the validated local OpenAI-compatible embeddings
// server (ollama on 127.0.0.1:11435) with the nomic-embed-text model instead
// of the OpenAI cloud.
const (
	DefaultLocalBaseURL    = "http://127.0.0.1:11435/v1"
	DefaultLocalModel      = "nomic-embed-text"
	DefaultLocalDimensions = 768
)

// Config defines configuration for the OpenAI embedding provider.
type Config struct {
	// APIKey is the OpenAI API key (from env OPENAI_API_KEY or config).
	APIKey string

	// Model is the embedding model name.
	Model string

	// Dimensions is the output dimensionality (varies by model).
	Dimensions int

	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int

	// Timeout is the HTTP request timeout.
	Timeout time.Duration

	// BaseURL is the OpenAI API base URL (for custom endpoints).
	BaseURL string

	// RateLimit is the maximum tokens per minute.
	RateLimit int

	// BatchSize is the maximum items per batch request.
	BatchSize int
}

// DefaultConfig returns a sensible default configuration.
// NOTE: The APIKey field is intentionally left empty to avoid coupling to environment variables.
// Use ConfigFromEnv to load API key from environment.
func DefaultConfig() Config {
	return Config{
		Model:      DefaultLocalModel,
		Dimensions: DefaultLocalDimensions,
		MaxRetries: 3,
		Timeout:    60 * time.Second,
		BaseURL:    DefaultLocalBaseURL,
		RateLimit:  1000000, // 1M TPM for most OpenAI tiers
		BatchSize:  20,
	}
}

// ConfigFromEnv returns a Config populated from environment variables.
// It starts from DefaultConfig and overlays any non-empty env vars.
func ConfigFromEnv() Config {
	cfg := DefaultConfig()
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	// Point the provider at a custom endpoint (e.g. a local OpenAI-compatible
	// embeddings server). Registry overrides applied by the factory take
	// precedence over this environment variable.
	if v := os.Getenv("COSCA_EMBEDDING_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	return cfg
}

// Provider implements the embeddings.Provider interface for OpenAI.
type Provider struct {
	cfg         Config
	client      *http.Client
	rateLimiter *providers.RateLimiter
	modelDims   int
}

// rateLimiter implements a simple token bucket rate limiter.

// New creates a new OpenAI embedding provider.
func New(cfg Config) (*Provider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY not set")
	}
	if cfg.Model == "" {
		cfg.Model = DefaultLocalModel
	}
	if cfg.Dimensions <= 0 {
		cfg.Dimensions = getDefaultDimensions(cfg.Model)
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultLocalBaseURL
	}
	if cfg.RateLimit <= 0 {
		cfg.RateLimit = 1000000
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 20
	}

	p := &Provider{
		cfg:         cfg,
		client:      providers.SharedHTTPClient(cfg.Timeout),
		rateLimiter: providers.NewRateLimiter(cfg.RateLimit),
		modelDims:   cfg.Dimensions,
	}

	log.Debug().
		Str("model", cfg.Model).
		Int("dims", p.modelDims).
		Str("base_url", cfg.BaseURL).
		Msg("openai embedding provider initialized")

	return p, nil
}

// getDefaultDimensions returns the default dimensions for a given model.
func getDefaultDimensions(model string) int {
	switch model {
	case ModelSmall:
		return DimSmall
	case ModelLarge:
		return DimLarge
	case ModelAda:
		return DimAda
	case DefaultLocalModel:
		return DefaultLocalDimensions
	default:
		return DimSmall
	}
}

// Name returns the provider name.
func (p *Provider) Name() string { return "openai" }

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

// embedBatch sends a single batch request to the OpenAI API.
func (p *Provider) embedBatch(ctx context.Context, texts []string) ([]*embeddings.EmbeddingResult, error) {
	// Apply rate limiting
	tokenEstimate := providers.EstimateTokens(texts)
	if err := p.rateLimiter.Wait(ctx, tokenEstimate); err != nil {
		return nil, fmt.Errorf("rate limit wait: %w", err)
	}

	// Build request body
	requestBody := map[string]interface{}{
		"input":           texts,
		"model":           p.cfg.Model,
		"encoding_format": "float",
	}
	if supportsDimensions(p.cfg.Model) {
		requestBody["dimensions"] = p.modelDims
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Send request with retry
	var lastErr error
	for attempt := 0; attempt <= p.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * time.Second
			log.Debug().Int("attempt", attempt+1).Dur("backoff", backoff).Msg("retrying openai embedding request")
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

		var results []*embeddings.EmbeddingResult
		results, lastErr = p.sendRequest(ctx, body)
		if lastErr == nil {
			return results, nil
		}

		// Don't retry on bad request
		if isBadRequest(lastErr) {
			return nil, lastErr
		}

		log.Warn().Err(lastErr).Int("attempt", attempt+1).Msg("openai embedding request failed")
	}

	return nil, fmt.Errorf("openai embedding failed after %d retries: %w", p.cfg.MaxRetries, lastErr)
}

// supportsDimensions reports whether the request body should include an
// explicit dimensions parameter for the given model. OpenAI's
// text-embedding-3-small/large and the local default model (nomic-embed-text)
// accept it; ada-002 and unknown models do not.
func supportsDimensions(model string) bool {
	switch model {
	case ModelSmall, ModelLarge, DefaultLocalModel:
		return true
	default:
		return false
	}
}

// sendRequest sends the HTTP request to the OpenAI API.
func (p *Provider) sendRequest(ctx context.Context, body []byte) ([]*embeddings.EmbeddingResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.BaseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer safe.Close(resp.Body)

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, handleAPIError(resp.StatusCode, string(respBody))
	}

	var apiResponse struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
		Usage struct {
			PromptTokens int `json:"prompt_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
		Model string `json:"model"`
	}

	if err := json.Unmarshal(respBody, &apiResponse); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if len(apiResponse.Data) == 0 {
		return nil, fmt.Errorf("no embedding data in response")
	}

	// Build one result per embedding in the response. OpenAI-compatible
	// servers (including local servers such as ollama/llama-server) return
	// one entry in `data` per input text; every entry must be preserved so
	// batch counts match the input count.
	results := make([]*embeddings.EmbeddingResult, 0, len(apiResponse.Data))
	for _, item := range apiResponse.Data {
		// Truncate or pad to configured dimensions
		vector := item.Embedding
		if len(vector) > p.modelDims {
			vector = vector[:p.modelDims]
		} else if len(vector) < p.modelDims {
			padded := make([]float64, p.modelDims)
			copy(padded, vector)
			vector = padded
		}

		results = append(results, &embeddings.EmbeddingResult{
			Vector:     vector,
			Model:      apiResponse.Model,
			Dimensions: len(vector),
			TokensUsed: apiResponse.Usage.TotalTokens,
		})
	}

	return results, nil
}

// handleAPIError translates HTTP status codes into meaningful errors.
func handleAPIError(statusCode int, body string) error {
	switch statusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("openai: invalid API key (401)")
	case http.StatusTooManyRequests:
		return fmt.Errorf("openai: rate limited (429): %s", providers.TruncateBody(body))
	case http.StatusServiceUnavailable:
		return fmt.Errorf("openai: service unavailable (503)")
	case http.StatusBadRequest:
		return fmt.Errorf("openai: bad request (400): %s", providers.TruncateBody(body))
	default:
		if statusCode >= 500 {
			return fmt.Errorf("openai: server error (%d): %s", statusCode, providers.TruncateBody(body))
		}
		return fmt.Errorf("openai: unexpected status %d: %s", statusCode, providers.TruncateBody(body))
	}
}

// isBadRequest returns true for non-retryable errors.
func isBadRequest(err error) bool {
	return strings.Contains(err.Error(), "(400)") ||
		strings.Contains(err.Error(), "(401)") ||
		strings.Contains(err.Error(), "(403)")
}

// estimateTokens estimates the token count for a set of texts.

// truncateBody truncates error response bodies for logging.

// Factory returns an embeddings.ProviderFactory for the OpenAI provider.
func Factory() (string, embeddings.ProviderFactory) {
	return "openai", func(_ context.Context, cfg *embeddings.Config) (embeddings.Provider, error) {
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
	registry := embeddings.GetRegistry()
	name, factory := Factory()
	registry.Register(name, factory, "OpenAI Embeddings API", 10)
}
