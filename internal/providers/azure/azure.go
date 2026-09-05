// Package azure provides embedding generation via Azure OpenAI's Embeddings API.
package azure

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

const (
	// DefaultEmbedModel is the default embedding model for Azure OpenAI.
	DefaultEmbedModel = "text-embedding-3-small"
	// DefaultDim is the default output dimensionality.
	DefaultDim = 512
	// DefaultAPIVersion is the Azure OpenAI API version.
	DefaultAPIVersion = "2024-02-01"
)

// Config defines configuration for the Azure OpenAI embedding provider.
type Config struct {
	// APIKey is the Azure OpenAI API key (from env AZURE_OPENAI_API_KEY or config).
	APIKey string

	// Endpoint is the Azure OpenAI endpoint URL.
	Endpoint string

	// Deployment is the deployment name for the embedding model.
	Deployment string

	// APIVersion is the Azure OpenAI API version.
	APIVersion string

	// Model is the embedding model name.
	Model string

	// Dimensions is the output dimensionality.
	Dimensions int

	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int

	// Timeout is the HTTP request timeout.
	Timeout time.Duration

	// BatchSize is the maximum items per batch request.
	BatchSize int
}

// DefaultConfig returns a sensible default configuration.
// NOTE: The APIKey, Endpoint, and Deployment fields are intentionally left empty
// to avoid coupling to environment variables. Use ConfigFromEnv to load them.
func DefaultConfig() Config {
	return Config{
		APIVersion: DefaultAPIVersion,
		Model:      DefaultEmbedModel,
		Dimensions: DefaultDim,
		MaxRetries: 3,
		Timeout:    60 * time.Second,
		BatchSize:  20,
	}
}

// ConfigFromEnv returns a Config populated from environment variables.
// It starts from DefaultConfig and overlays any non-empty env vars.
func ConfigFromEnv() Config {
	cfg := DefaultConfig()
	if v := os.Getenv("AZURE_OPENAI_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("AZURE_OPENAI_ENDPOINT"); v != "" {
		cfg.Endpoint = v
	}
	if v := os.Getenv("AZURE_OPENAI_DEPLOYMENT"); v != "" {
		cfg.Deployment = v
	}
	return cfg
}

// Provider implements the embeddings.Provider interface for Azure OpenAI.
type Provider struct {
	cfg         Config
	client      *http.Client
	rateLimiter *providers.RateLimiter
	modelDims   int
}

// rateLimiter implements a simple token bucket rate limiter.

// New creates a new Azure OpenAI embedding provider.
func New(cfg Config) (*Provider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("AZURE_OPENAI_API_KEY not set")
	}
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("AZURE_OPENAI_ENDPOINT not set")
	}
	if cfg.Deployment == "" {
		return nil, fmt.Errorf("AZURE_OPENAI_DEPLOYMENT not set")
	}
	if cfg.APIVersion == "" {
		cfg.APIVersion = DefaultAPIVersion
	}
	if cfg.Model == "" {
		cfg.Model = DefaultEmbedModel
	}
	if cfg.Dimensions <= 0 {
		cfg.Dimensions = DefaultDim
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 20
	}

	// Normalize endpoint
	cfg.Endpoint = strings.TrimRight(cfg.Endpoint, "/")

	p := &Provider{
		cfg:         cfg,
		client:      providers.SharedHTTPClient(cfg.Timeout),
		rateLimiter: providers.NewRateLimiter(1000000), // 1M TPM
		modelDims:   cfg.Dimensions,
	}

	log.Debug().
		Str("model", cfg.Model).
		Str("deployment", cfg.Deployment).
		Str("endpoint", cfg.Endpoint).
		Int("dims", p.modelDims).
		Msg("azure embedding provider initialized")

	return p, nil
}

// Name returns the provider name.
func (p *Provider) Name() string { return "azure" }

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

// embedBatch sends a single batch request to the Azure OpenAI API.
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
	if p.cfg.Model == DefaultEmbedModel {
		requestBody["dimensions"] = p.modelDims
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Build Azure endpoint: {endpoint}/openai/deployments/{deployment}/embeddings?api-version={version}
	endpoint := fmt.Sprintf("%s/openai/deployments/%s/embeddings?api-version=%s",
		p.cfg.Endpoint, p.cfg.Deployment, p.cfg.APIVersion)

	// Send request with retry
	var lastErr error
	for attempt := 0; attempt <= p.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * time.Second
			log.Debug().Int("attempt", attempt+1).Dur("backoff", backoff).Msg("retrying azure embedding request")
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

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("api-key", p.cfg.APIKey)

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
			lastErr = handleAPIError(resp.StatusCode, string(respBody))
			if isBadRequest(lastErr) {
				return nil, lastErr
			}
			continue
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
			lastErr = fmt.Errorf("parse response: %w", err)
			continue
		}

		if len(apiResponse.Data) == 0 {
			return nil, fmt.Errorf("no embedding data in response")
		}

		// Truncate or pad to configured dimensions
		vector := apiResponse.Data[0].Embedding
		if len(vector) > p.modelDims {
			vector = vector[:p.modelDims]
		} else if len(vector) < p.modelDims {
			padded := make([]float64, p.modelDims)
			copy(padded, vector)
			vector = padded
		}

		result := &embeddings.EmbeddingResult{
			Vector:     vector,
			Model:      apiResponse.Model,
			Dimensions: len(vector),
			TokensUsed: apiResponse.Usage.TotalTokens,
		}

		return []*embeddings.EmbeddingResult{result}, nil
	}

	return nil, fmt.Errorf("azure embedding failed after %d retries: %w", p.cfg.MaxRetries, lastErr)
}

// handleAPIError translates HTTP status codes into meaningful errors.
func handleAPIError(statusCode int, body string) error {
	switch statusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("azure: invalid API key (401)")
	case http.StatusTooManyRequests:
		return fmt.Errorf("azure: rate limited (429): %s", providers.TruncateBody(body))
	case http.StatusServiceUnavailable:
		return fmt.Errorf("azure: service unavailable (503)")
	case http.StatusBadRequest:
		return fmt.Errorf("azure: bad request (400): %s", providers.TruncateBody(body))
	default:
		if statusCode >= 500 {
			return fmt.Errorf("azure: server error (%d): %s", statusCode, providers.TruncateBody(body))
		}
		return fmt.Errorf("azure: unexpected status %d: %s", statusCode, providers.TruncateBody(body))
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

// Factory returns an embeddings.ProviderFactory for the Azure provider.
func Factory() (string, embeddings.ProviderFactory) {
	return "azure", func(_ context.Context, cfg *embeddings.Config) (embeddings.Provider, error) {
		config := ConfigFromEnv()
		if cfg != nil {
			if cfg.APIKey != "" {
				config.APIKey = cfg.APIKey
			}
			if cfg.Endpoint != "" {
				config.Endpoint = cfg.Endpoint
			}
			if cfg.Deployment != "" {
				config.Deployment = cfg.Deployment
			}
			if cfg.Model != "" {
				config.Model = cfg.Model
			}
			if cfg.Dimensions > 0 {
				config.Dimensions = cfg.Dimensions
			}
		}
		return New(config)
	}
}

// Register registers this provider with the global embedding provider registry.
func Register() {
	registry := embeddings.GetRegistry()
	name, factory := Factory()
	registry.Register(name, factory, "Azure OpenAI Embeddings API", 30)
}
