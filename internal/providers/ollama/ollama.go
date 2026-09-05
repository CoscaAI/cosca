// Package ollama provides embedding generation via a local Ollama instance.
package ollama

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

// Default model names.
const (
	ModelNomicEmbed = "nomic-embed-text"
	ModelLlama3     = "llama3"
	ModelMxbaiEmbed = "mxbai-embed-large"
)

// Config defines configuration for the Ollama embedding provider.
type Config struct {
	// BaseURL is the Ollama server URL.
	BaseURL string

	// Model is the embedding model name.
	Model string

	// KeepAlive is the duration to keep the model loaded in memory.
	KeepAlive string

	// Timeout is the HTTP request timeout.
	Timeout time.Duration

	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int

	// BatchSize is the maximum number of texts to embed in a single
	// /api/embed request. Ollama natively supports input arrays.
	// Default: 50 (balance between throughput and memory).
	BatchSize int

	// Digest é o digest esperado do modelo (L376): quando definido, o
	// provider consulta /api/tags e REFUSA operar se o digest do modelo
	// carregado não bater — identidade é parte da integridade do índice.
	Digest string
}

// DefaultConfig returns a sensible default configuration.
// NOTE: The BaseURL is set to the default Ollama address without reading environment.
// Use ConfigFromEnv to load OLLAMA_HOST from environment.
func DefaultConfig() Config {
	return Config{
		BaseURL:    "http://localhost:11434",
		Model:      ModelNomicEmbed,
		KeepAlive:  "5m",
		Timeout:    120 * time.Second,
		MaxRetries: 2,
		BatchSize:  50,
	}
}

// ConfigFromEnv returns a Config populated from environment variables.
// It starts from DefaultConfig and overlays any non-empty env vars.
func ConfigFromEnv() Config {
	cfg := DefaultConfig()
	if v := os.Getenv("OLLAMA_HOST"); v != "" {
		// Ensure the URL has a scheme; "127.0.0.1:11435" is common but
		// net/http needs "http://127.0.0.1:11435".
		if !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") {
			v = "http://" + v
		}
		cfg.BaseURL = v
	}
	return cfg
}

// Provider implements the embeddings.Provider interface for Ollama.
type Provider struct {
	cfg       Config
	client    *http.Client
	modelDims int
	mu        sync.Mutex
}

// New creates a new Ollama embedding provider.
func New(cfg Config) (*Provider, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:11434"
	}
	if cfg.Model == "" {
		cfg.Model = ModelNomicEmbed
	}
	if cfg.KeepAlive == "" {
		cfg.KeepAlive = "5m"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 120 * time.Second
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 2
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 50
	}

	// Normalize base URL — strip trailing slash and any /v1 prefix
	// so that native Ollama endpoints (/api/embed, /api/tags) work
	// even when users configure an OpenAI-compatible-style base.
	cfg.BaseURL = strings.TrimSuffix(strings.TrimRight(cfg.BaseURL, "/"), "/v1")

	p := &Provider{
		cfg:       cfg,
		client:    providers.SharedHTTPClient(cfg.Timeout),
		modelDims: 768, // default for nomic-embed-text
	}

	// Pinagem do digest (L376): identidade é parte da integridade do índice.
	// Se um digest esperado foi configurado, o modelo carregado PRECISA ter
	// o mesmo digest — caso contrário, REFUSA operar (fail-closed contra
	// mudança silenciosa do modelo "latest").
	if cfg.Digest != "" {
		actual, err := p.fetchModelDigest(cfg.Model)
		if err != nil {
			return nil, fmt.Errorf("embedding identity check: %w", err)
		}
		if !strings.HasPrefix(actual, cfg.Digest) {
			return nil, fmt.Errorf(
				"%w: esperado digest %q, modelo carregado %q — REFUSING OPERATION (L376)",
				embeddings.ErrEmbeddingIdentityMismatch, cfg.Digest, actual)
		}
		log.Debug().Str("model", cfg.Model).Str("digest", actual).Msg("embedding identity verified")
	}

	log.Debug().
		Str("model", cfg.Model).
		Str("base_url", cfg.BaseURL).
		Int("batch_size", cfg.BatchSize).
		Msg("ollama embedding provider initialized")

	return p, nil
}

// fetchModelDigest consulta /api/tags e devolve o digest do modelo.
func (p *Provider) fetchModelDigest(model string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, p.cfg.BaseURL+"/api/tags", nil)
	if err != nil {
		return "", err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama /api/tags status %d", resp.StatusCode)
	}
	var out struct {
		Models []struct {
			Name   string `json:"name"`
			Digest string `json:"digest"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	for _, m := range out.Models {
		if m.Name == model || strings.HasPrefix(m.Name, model+":") {
			return m.Digest, nil
		}
	}
	return "", fmt.Errorf("modelo %q não encontrado no ollama", model)
}

// Name returns the provider name.
func (p *Provider) Name() string { return "ollama" }

// Model returns the model name.
func (p *Provider) Model() string { return p.cfg.Model }

// Dimensions returns the embedding dimensionality.
func (p *Provider) Dimensions() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.modelDims
}

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
// Uses Ollama's native /api/embed endpoint with input array for true batching.
func (p *Provider) GenerateEmbeddings(ctx context.Context, texts []string) ([]*embeddings.EmbeddingResult, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}

	results := make([]*embeddings.EmbeddingResult, 0, len(texts))
	batchSize := p.cfg.BatchSize

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		batchResults, err := p.embedBatch(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("batch %d: %w", i/batchSize, err)
		}
		results = append(results, batchResults...)
	}

	return results, nil
}

// embedBatch sends multiple texts in a single /api/embed request.
// Ollama natively supports input as a string array.
func (p *Provider) embedBatch(ctx context.Context, texts []string) ([]*embeddings.EmbeddingResult, error) {
	requestBody := map[string]interface{}{
		"model": p.cfg.Model,
		"input": texts,
		"options": map[string]interface{}{
			"num_ctx": 8192,
		},
		"keep_alive": p.cfg.KeepAlive,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	endpoint := strings.TrimSuffix(p.cfg.BaseURL, "/v1") + "/api/embed"

	var lastErr error
	for attempt := 0; attempt <= p.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * 2 * time.Second
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

		resp, err := p.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http request: %w", err)
			log.Warn().Err(lastErr).Int("attempt", attempt+1).Msg("ollama batch request failed")
			continue
		}

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024)) // 50MB for large batches
		safe.Close(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("read response: %w", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("ollama: status %d: %s", resp.StatusCode, providers.TruncateBody(string(respBody)))
			continue
		}

		// Ollama /api/embed returns {"embeddings": [[...], [...], ...]}
		var apiResponse struct {
			Embeddings [][]float64 `json:"embeddings"`
		}

		if err := json.Unmarshal(respBody, &apiResponse); err != nil {
			// Fallback: try single embedding format {"embedding": [...]}
			var singleResponse struct {
				Embedding []float64 `json:"embedding"`
			}
			if err2 := json.Unmarshal(respBody, &singleResponse); err2 != nil {
				lastErr = fmt.Errorf("parse response: %w (tried batch and single)", err)
				continue
			}
			if len(singleResponse.Embedding) == 0 {
				lastErr = fmt.Errorf("empty embedding vector")
				continue
			}
			apiResponse.Embeddings = [][]float64{singleResponse.Embedding}
		}

		if len(apiResponse.Embeddings) == 0 {
			lastErr = fmt.Errorf("empty embedding response")
			continue
		}

		// Update detected dimensions from the first embedding
		if len(apiResponse.Embeddings) > 0 && len(apiResponse.Embeddings[0]) > 0 {
			p.mu.Lock()
			p.modelDims = len(apiResponse.Embeddings[0])
			p.mu.Unlock()
		}

		// Map embeddings back to results, preserving index order
		// Ollama returns embeddings in the same order as the input array
		batchResults := make([]*embeddings.EmbeddingResult, len(texts))
		for j := 0; j < len(texts); j++ {
			if j < len(apiResponse.Embeddings) {
				batchResults[j] = &embeddings.EmbeddingResult{
					Vector:     apiResponse.Embeddings[j],
					Model:      p.cfg.Model,
					Dimensions: len(apiResponse.Embeddings[j]),
					TokensUsed: estimateTokens(texts[j]),
				}
			}
		}

		return batchResults, nil
	}

	return nil, fmt.Errorf("ollama embedding failed after %d retries: %w", p.cfg.MaxRetries, lastErr)
}

// AvailableModels returns a list of models available on the Ollama server.
func AvailableModels(ctx context.Context, baseURL string) ([]string, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	baseURL = strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/v1")

	client := providers.SharedHTTPClient(10 * time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("create list request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list models request: %w", err)
	}
	defer safe.Close(resp.Body)

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var listResponse struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.Unmarshal(body, &listResponse); err != nil {
		return nil, fmt.Errorf("parse model list: %w", err)
	}

	models := make([]string, len(listResponse.Models))
	for i, m := range listResponse.Models {
		models[i] = m.Name
	}

	return models, nil
}

// estimateTokens provides a rough token count estimate.
func estimateTokens(text string) int {
	return len(text) / 4
}

// truncateBody truncates long error bodies.

// getEnvOrDefault returns the environment variable or a default value.
func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// Factory returns an embeddings.ProviderFactory for the Ollama provider.
func Factory() (string, embeddings.ProviderFactory) {
	return "ollama", func(_ context.Context, cfg *embeddings.Config) (embeddings.Provider, error) {
		config := ConfigFromEnv()
		if cfg != nil {
			if cfg.BaseURL != "" {
				config.BaseURL = cfg.BaseURL
			}
			if cfg.Model != "" {
				config.Model = cfg.Model
			}
			if cfg.KeepAlive != "" {
				config.KeepAlive = cfg.KeepAlive
			}
			config.Digest = cfg.Digest
		}
		return New(config)
	}
}

// Register registers this provider with the global embedding provider registry.
func Register() {
	registry := embeddings.GetRegistry()
	name, factory := Factory()
	registry.Register(name, factory, "Local Ollama embeddings", 20)
}
