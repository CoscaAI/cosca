package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/CoscaAI/cosca/internal/providers"
	"github.com/CoscaAI/cosca/internal/safe"
	"github.com/rs/zerolog/log"
)

// ─── Embedding Config Types ───────────────────────────────────────────────────

// EmbeddingConfig defines runtime configuration for an OpenAI-compatible embedding provider.
type EmbeddingConfig struct {
	APIKey     string
	Model      string
	Dimensions int
	MaxRetries int
	Timeout    time.Duration
	BaseURL    string
	BatchSize  int
}

// EmbeddingProviderConfig defines the static identity of a specific embedding provider.
type EmbeddingProviderConfig struct {
	ProviderName     string
	DefaultModel     string
	DefaultDims      int
	DefaultBaseURL   string
	EnvVarName       string
	RegistryName     string
	RegistryPriority int
	Description      string
	// RateLimitTPM is the tokens-per-minute rate limit (used for rate limiter capacity).
	RateLimitTPM int
}

// DefaultEmbeddingConfig returns a sensible default embedding config from the provider config.
// NOTE: The APIKey field is intentionally left empty to avoid coupling to environment variables.
// Use EmbeddingConfigFromEnv to load API key from environment.
func DefaultEmbeddingConfig(pc EmbeddingProviderConfig) EmbeddingConfig {
	return EmbeddingConfig{
		Model:      pc.DefaultModel,
		Dimensions: pc.DefaultDims,
		MaxRetries: 3,
		Timeout:    60 * time.Second,
		BaseURL:    pc.DefaultBaseURL,
		BatchSize:  20,
	}
}

// EmbeddingConfigFromEnv returns an EmbeddingConfig populated from environment variables.
// It starts from DefaultEmbeddingConfig and overlays any non-empty env vars.
func EmbeddingConfigFromEnv(pc EmbeddingProviderConfig) EmbeddingConfig {
	cfg := DefaultEmbeddingConfig(pc)
	if v := os.Getenv(pc.EnvVarName); v != "" {
		cfg.APIKey = v
	}
	return cfg
}

// ─── EmbeddingProvider ────────────────────────────────────────────────────────

// EmbeddingProvider implements embeddings.Provider for OpenAI-compatible APIs.
type EmbeddingProvider struct {
	cfg         EmbeddingConfig
	providerCfg EmbeddingProviderConfig
	client      *http.Client
	rateLimiter *rateLimiter
	modelDims   int
}

// Compile-time interface check.
var _ embeddings.Provider = (*EmbeddingProvider)(nil)

// NewEmbeddingProvider creates a new OpenAI-compatible embedding provider.
func NewEmbeddingProvider(pc EmbeddingProviderConfig, cfg EmbeddingConfig) (*EmbeddingProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("%s not set", pc.EnvVarName)
	}
	if cfg.Model == "" {
		cfg.Model = pc.DefaultModel
	}
	if cfg.Dimensions <= 0 {
		cfg.Dimensions = pc.DefaultDims
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = pc.DefaultBaseURL
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 20
	}

	tpm := pc.RateLimitTPM
	if tpm <= 0 {
		tpm = 300000
	}

	p := &EmbeddingProvider{
		cfg:         cfg,
		providerCfg: pc,
		client:      providers.SharedHTTPClient(cfg.Timeout),
		rateLimiter: newRateLimiter(tpm),
		modelDims:   cfg.Dimensions,
	}

	log.Debug().
		Str("model", cfg.Model).
		Int("dims", p.modelDims).
		Msg(pc.ProviderName + " embedding provider initialized")

	return p, nil
}

// Name returns the provider name.
func (p *EmbeddingProvider) Name() string { return p.providerCfg.ProviderName }

// Model returns the model name.
func (p *EmbeddingProvider) Model() string { return p.cfg.Model }

// Dimensions returns the embedding dimensionality.
func (p *EmbeddingProvider) Dimensions() int { return p.modelDims }

// Close cleans up provider resources.
func (p *EmbeddingProvider) Close() error { return nil }

// GenerateEmbedding generates an embedding for a single text string.
func (p *EmbeddingProvider) GenerateEmbedding(ctx context.Context, text string) (*embeddings.EmbeddingResult, error) {
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
func (p *EmbeddingProvider) GenerateEmbeddings(ctx context.Context, texts []string) ([]*embeddings.EmbeddingResult, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}

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

// Cfg returns the runtime configuration (for testing).
func (p *EmbeddingProvider) Cfg() EmbeddingConfig { return p.cfg }

// embedBatch sends a single batch request to the API.
func (p *EmbeddingProvider) embedBatch(ctx context.Context, texts []string) ([]*embeddings.EmbeddingResult, error) {
	tokenEstimate := providers.EstimateTokens(texts)
	if err := p.rateLimiter.Wait(ctx, tokenEstimate); err != nil {
		return nil, fmt.Errorf("rate limit wait: %w", err)
	}

	requestBody := map[string]interface{}{
		"input":           texts,
		"model":           p.cfg.Model,
		"encoding_format": "float",
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= p.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * time.Second
			log.Debug().Int("attempt", attempt+1).Dur("backoff", backoff).Msg("retrying " + p.providerCfg.ProviderName + " embedding request")
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

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.BaseURL, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)

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
			lastErr = handleEmbeddingAPIError(p.providerCfg.ProviderName, resp.StatusCode, string(respBody))
			if isEmbeddingBadRequest(lastErr) {
				return nil, lastErr
			}
			continue
		}

		var apiResponse struct {
			Data []struct {
				Embedding []float64 `json:"embedding"`
				Index     int       `json:"index"`
				Object    string    `json:"object"`
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
		if len(apiResponse.Data) != len(texts) {
			return nil, fmt.Errorf("embedding result count %d does not match batch size %d", len(apiResponse.Data), len(texts))
		}

		model := apiResponse.Model
		if model == "" {
			model = p.cfg.Model
		}
		results := make([]*embeddings.EmbeddingResult, len(texts))
		seen := make([]bool, len(texts))
		// OpenAI-compatible APIs report usage once per request.  Distribute it
		// deterministically across results so consumers summing TokensUsed do
		// not count one batch's usage once per embedding.
		usageEach := apiResponse.Usage.TotalTokens / len(texts)
		usageRemainder := apiResponse.Usage.TotalTokens % len(texts)
		for _, item := range apiResponse.Data {
			if item.Index < 0 || item.Index >= len(texts) || seen[item.Index] {
				return nil, fmt.Errorf("invalid embedding index: %d", item.Index)
			}
			seen[item.Index] = true
			vector := item.Embedding
			if len(vector) != p.modelDims {
				return nil, fmt.Errorf("embedding dimension mismatch at index %d: got %d, want %d", item.Index, len(vector), p.modelDims)
			}
			result := &embeddings.EmbeddingResult{
				Vector: vector, Model: model,
				Dimensions: len(vector), TokensUsed: usageEach,
			}
			if item.Index < usageRemainder {
				result.TokensUsed++
			}
			if err := embeddings.ValidateEmbedding(result); err != nil {
				return nil, fmt.Errorf("invalid embedding at index %d: %w", item.Index, err)
			}
			if result.Dimensions != p.modelDims {
				return nil, fmt.Errorf("embedding dimension mismatch: got %d, want %d", result.Dimensions, p.modelDims)
			}
			results[item.Index] = result
		}
		for i, result := range results {
			if !seen[i] || result == nil {
				return nil, fmt.Errorf("missing embedding index: %d", i)
			}
		}
		return results, nil
	}

	return nil, fmt.Errorf("%s embedding failed after %d retries: %w", p.providerCfg.ProviderName, p.cfg.MaxRetries, lastErr)
}

// ─── Embedding Registry ───────────────────────────────────────────────────────

// RegisterEmbedding registers an embedding provider with the global registry.
func RegisterEmbedding(pc EmbeddingProviderConfig) {
	registry := embeddings.GetRegistry()
	name := pc.RegistryName
	factory := func(_ context.Context, embCfg *embeddings.Config) (embeddings.Provider, error) {
		config := EmbeddingConfigFromEnv(pc)
		if embCfg != nil {
			if embCfg.APIKey != "" {
				config.APIKey = embCfg.APIKey
			}
			if embCfg.Model != "" {
				config.Model = embCfg.Model
			}
			if embCfg.Dimensions > 0 {
				config.Dimensions = embCfg.Dimensions
			}
			if embCfg.BaseURL != "" {
				config.BaseURL = embCfg.BaseURL
			}
		}
		return NewEmbeddingProvider(pc, config)
	}
	registry.Register(name, factory, pc.Description, pc.RegistryPriority)
}

// ─── Embedding Utilities ──────────────────────────────────────────────────────

// handleEmbeddingAPIError translates HTTP status codes into meaningful errors.
func handleEmbeddingAPIError(providerName string, statusCode int, body string) error {
	switch statusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("%s: invalid API key (401)", providerName)
	case http.StatusTooManyRequests:
		return fmt.Errorf("%s: rate limited (429): %s", providerName, providers.TruncateBody(body))
	case http.StatusServiceUnavailable:
		return fmt.Errorf("%s: service unavailable (503)", providerName)
	case http.StatusBadRequest:
		return fmt.Errorf("%s: bad request (400): %s", providerName, providers.TruncateBody(body))
	default:
		if statusCode >= 500 {
			return fmt.Errorf("%s: server error (%d): %s", providerName, statusCode, providers.TruncateBody(body))
		}
		return fmt.Errorf("%s: unexpected status %d: %s", providerName, statusCode, providers.TruncateBody(body))
	}
}

// isEmbeddingBadRequest returns true for non-retryable errors.
func isEmbeddingBadRequest(err error) bool {
	return strings.Contains(err.Error(), "(400)") ||
		strings.Contains(err.Error(), "(401)") ||
		strings.Contains(err.Error(), "(403)")
}

// ─── Rate Limiter ─────────────────────────────────────────────────────────────

// rateLimiter implements a simple token bucket rate limiter.
type rateLimiter struct {
	mu         sync.Mutex
	tokens     int
	capacity   int
	interval   time.Duration
	lastRefill time.Time
}

func newRateLimiter(tokensPerMinute int) *rateLimiter {
	return &rateLimiter{
		tokens:     tokensPerMinute,
		capacity:   tokensPerMinute,
		interval:   time.Minute,
		lastRefill: time.Now(),
	}
}

func (r *rateLimiter) Wait(ctx context.Context, tokens int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Refill
	elapsed := time.Since(r.lastRefill)
	r.lastRefill = time.Now()
	r.tokens += int(float64(elapsed) / float64(r.interval) * float64(r.capacity))
	if r.tokens > r.capacity {
		r.tokens = r.capacity
	}

	if r.tokens >= tokens {
		r.tokens -= tokens
		return nil
	}

	// Need to wait
	needed := tokens - r.tokens
	waitDuration := time.Duration(float64(needed)/float64(r.capacity)) * r.interval

	timer := time.NewTimer(waitDuration)
	select {
	case <-timer.C:
		r.tokens = 0
		return nil
	case <-ctx.Done():
		if !timer.Stop() {
			<-timer.C
		}
		return ctx.Err()
	}
}
