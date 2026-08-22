package openaicompat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Test Fixtures / Helpers ───────────────────────────────────────────────────

// testProviderConfig returns a standard EmbeddingProviderConfig for testing.
func testProviderConfig() EmbeddingProviderConfig {
	return EmbeddingProviderConfig{
		ProviderName:     "test-provider",
		DefaultModel:     "text-embedding-3-small",
		DefaultDims:      4,
		DefaultBaseURL:   "http://test.example.com/v1/embeddings",
		EnvVarName:       "TEST_API_KEY",
		RegistryName:     "test-embedding",
		RegistryPriority: 10,
		Description:      "Test embedding provider",
		RateLimitTPM:     1000000,
	}
}

// testEmbeddingConfig returns a standard EmbeddingConfig for testing.
func testEmbeddingConfig() EmbeddingConfig {
	return EmbeddingConfig{
		APIKey:     "test-key-12345",
		Model:      "text-embedding-3-small",
		Dimensions: 4,
		MaxRetries: 1,
		Timeout:    5 * time.Second,
		BaseURL:    "http://test.example.com/v1/embeddings",
		BatchSize:  10,
	}
}

// newTestProvider creates an EmbeddingProvider with a custom HTTP client
// that routes to the given test server. This avoids real HTTP calls.
func newTestProvider(t *testing.T, serverURL string) *EmbeddingProvider {
	t.Helper()

	cfg := testEmbeddingConfig()
	cfg.BaseURL = serverURL

	providerCfg := testProviderConfig()
	p := &EmbeddingProvider{
		cfg:         cfg,
		providerCfg: providerCfg,
		client:      &http.Client{Timeout: 5 * time.Second},
		rateLimiter: newRateLimiter(1000000),
		modelDims:   cfg.Dimensions,
	}
	return p
}

// readTestData reads a JSON fixture from the testdata directory.
func readTestData(t *testing.T, filename string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + filename)
	require.NoError(t, err, "failed to read testdata/"+filename)
	return data
}

// writeBatchEmbeddingResponse returns one complete, deliberately out-of-order
// fixture entry for every input in the request.
func writeBatchEmbeddingResponse(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	var request struct {
		Input []string `json:"input"`
	}
	require.NoError(t, json.NewDecoder(r.Body).Decode(&request))

	data := make([]map[string]interface{}, 0, len(request.Input))
	for i := len(request.Input) - 1; i >= 0; i-- {
		value := float64(i) + 0.1
		data = append(data, map[string]interface{}{
			"object":    "embedding",
			"embedding": []float64{value, value + 0.1, value + 0.2, value + 0.3},
			"index":     i,
		})
	}

	response := map[string]interface{}{
		"object": "list",
		"data":   data,
		"model":  "text-embedding-3-small",
		"usage":  map[string]int{"prompt_tokens": len(request.Input), "total_tokens": len(request.Input) * 4},
	}
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(response))
}

// ─── Lote 1: Pure Functions ────────────────────────────────────────────────────

// ── DefaultEmbeddingConfig ─────────────────────────────────────────────────────

func TestDefaultEmbeddingConfig(t *testing.T) {
	t.Parallel()

	pc := testProviderConfig()

	cfg := DefaultEmbeddingConfig(pc)

	t.Run("default values match provider config", func(t *testing.T) {
		assert.Equal(t, pc.DefaultModel, cfg.Model)
		assert.Equal(t, pc.DefaultDims, cfg.Dimensions)
		assert.Equal(t, pc.DefaultBaseURL, cfg.BaseURL)
	})

	t.Run("hardcoded defaults", func(t *testing.T) {
		assert.Equal(t, 3, cfg.MaxRetries)
		assert.Equal(t, 60*time.Second, cfg.Timeout)
		assert.Equal(t, 20, cfg.BatchSize)
	})

	t.Run("API key is empty", func(t *testing.T) {
		assert.Empty(t, cfg.APIKey, "APIKey should be empty by design")
	})
}

func TestDefaultEmbeddingConfig_ZeroProviderConfig(t *testing.T) {
	t.Parallel()

	pc := EmbeddingProviderConfig{}
	cfg := DefaultEmbeddingConfig(pc)

	assert.Empty(t, cfg.Model)
	assert.Equal(t, 0, cfg.Dimensions)
	assert.Empty(t, cfg.BaseURL)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 60*time.Second, cfg.Timeout)
	assert.Equal(t, 20, cfg.BatchSize)
	assert.Empty(t, cfg.APIKey)
}

// ── handleEmbeddingAPIError ────────────────────────────────────────────────────

func TestHandleEmbeddingAPIError(t *testing.T) {
	t.Parallel()

	const provider = "test-provider"

	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    string
	}{
		{
			name:       "401 Unauthorized",
			statusCode: http.StatusUnauthorized,
			body:       `{"error":"invalid api key"}`,
			wantErr:    "test-provider: invalid API key (401)",
		},
		{
			name:       "429 Rate Limited",
			statusCode: http.StatusTooManyRequests,
			body:       `{"error":"too many requests"}`,
			wantErr:    "test-provider: rate limited (429):",
		},
		{
			name:       "503 Service Unavailable",
			statusCode: http.StatusServiceUnavailable,
			body:       "",
			wantErr:    "test-provider: service unavailable (503)",
		},
		{
			name:       "400 Bad Request",
			statusCode: http.StatusBadRequest,
			body:       `{"error":"bad input"}`,
			wantErr:    "test-provider: bad request (400):",
		},
		{
			name:       "500 Internal Server Error",
			statusCode: http.StatusInternalServerError,
			body:       "server error",
			wantErr:    "test-provider: server error (500):",
		},
		{
			name:       "502 Bad Gateway",
			statusCode: http.StatusBadGateway,
			body:       "bad gateway",
			wantErr:    "test-provider: server error (502):",
		},
		{
			name:       "418 I'm a Teapot (unexpected)",
			statusCode: http.StatusTeapot,
			body:       "teapot",
			wantErr:    "test-provider: unexpected status 418:",
		},
		{
			name:       "200 OK should not happen but test anyway",
			statusCode: http.StatusOK,
			body:       "ok",
			wantErr:    "test-provider: unexpected status 200:",
		},
		{
			name:       "100 Continue (unexpected low)",
			statusCode: http.StatusContinue,
			body:       "continue",
			wantErr:    "test-provider: unexpected status 100:",
		},
		{
			name:       "0 (unknown)",
			statusCode: 0,
			body:       "empty",
			wantErr:    "test-provider: unexpected status 0:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handleEmbeddingAPIError(provider, tt.statusCode, tt.body)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestHandleEmbeddingAPIError_BodyTruncated(t *testing.T) {
	t.Parallel()

	// The body should be truncated in the error message for non-401 codes.
	longBody := string(make([]byte, 500))
	err := handleEmbeddingAPIError("test", http.StatusInternalServerError, longBody)
	require.Error(t, err)
	// The error message should contain the truncated body (200 chars + "...")
	assert.Contains(t, err.Error(), "...")
	// The full 500-char body should NOT appear in the error
	assert.NotContains(t, err.Error(), string(make([]byte, 300)))
}

func TestHandleEmbeddingAPIError_EmptyBody(t *testing.T) {
	t.Parallel()

	err := handleEmbeddingAPIError("test", http.StatusBadGateway, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "test: server error (502):")
	// Empty body should still produce a valid error with an empty suffix
}

// ── isEmbeddingBadRequest ──────────────────────────────────────────────────────

func TestIsEmbeddingBadRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		err     error
		wantBad bool
	}{
		{
			name:    "400 Bad Request",
			err:     fmt.Errorf("test: bad request (400): invalid input"),
			wantBad: true,
		},
		{
			name:    "401 Unauthorized",
			err:     fmt.Errorf("test: invalid API key (401)"),
			wantBad: true,
		},
		{
			name:    "403 Forbidden",
			err:     fmt.Errorf("test: forbidden (403)"),
			wantBad: true,
		},
		{
			name:    "429 Too Many Requests",
			err:     fmt.Errorf("test: rate limited (429)"),
			wantBad: false,
		},
		{
			name:    "500 Internal Server Error",
			err:     fmt.Errorf("test: server error (500)"),
			wantBad: false,
		},
		{
			name:    "503 Service Unavailable",
			err:     fmt.Errorf("test: service unavailable (503)"),
			wantBad: false,
		},
		// NOTE: isEmbeddingBadRequest panics on nil error (calls err.Error()),
		// so we do not test that case here.
		{
			name:    "random error without status code",
			err:     errors.New("connection refused"),
			wantBad: false,
		},
		{
			name:    "error with 400 in middle of word",
			err:     fmt.Errorf("error (4000) something"),
			wantBad: false, // "(4000)" is not "(400)"
		},
		{
			name:    "error with 4000 (no parens pattern)",
			err:     fmt.Errorf("error 400"),
			wantBad: false, // no parens
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEmbeddingBadRequest(tt.err)
			assert.Equal(t, tt.wantBad, got)
		})
	}
}

// ── newRateLimiter ─────────────────────────────────────────────────────────────

func TestNewRateLimiter(t *testing.T) {
	t.Parallel()

	t.Run("positive TPM", func(t *testing.T) {
		rl := newRateLimiter(60000)
		require.NotNil(t, rl)
		assert.Equal(t, 60000, rl.capacity)
		assert.Equal(t, 60000, rl.tokens)
		assert.Equal(t, time.Minute, rl.interval)
		assert.False(t, rl.lastRefill.IsZero())
	})

	t.Run("zero TPM", func(t *testing.T) {
		rl := newRateLimiter(0)
		require.NotNil(t, rl)
		assert.Equal(t, 0, rl.capacity)
		assert.Equal(t, 0, rl.tokens)
	})

	t.Run("very large TPM", func(t *testing.T) {
		rl := newRateLimiter(10000000)
		require.NotNil(t, rl)
		assert.Equal(t, 10000000, rl.capacity)
		assert.Equal(t, 10000000, rl.tokens)
	})

	// The newRateLimiter does NOT clamp zero/negative unlike the providers.NewRateLimiter.
	// We test the actual behavior.
	t.Run("negative TPM", func(t *testing.T) {
		rl := newRateLimiter(-100)
		require.NotNil(t, rl)
		assert.Equal(t, -100, rl.capacity)
	})
}

// ── NewEmbeddingProvider ───────────────────────────────────────────────────────

func TestNewEmbeddingProvider(t *testing.T) {
	t.Parallel()

	pc := testProviderConfig()
	cfg := testEmbeddingConfig()

	t.Run("missing API key", func(t *testing.T) {
		badCfg := cfg
		badCfg.APIKey = ""
		p, err := NewEmbeddingProvider(pc, badCfg)
		require.Error(t, err)
		assert.Nil(t, p)
		assert.Contains(t, err.Error(), pc.EnvVarName)
		assert.Contains(t, err.Error(), "not set")
	})

	t.Run("empty model uses default", func(t *testing.T) {
		emptyCfg := cfg
		emptyCfg.Model = ""
		p, err := NewEmbeddingProvider(pc, emptyCfg)
		require.NoError(t, err)
		require.NotNil(t, p)
		assert.Equal(t, pc.DefaultModel, p.cfg.Model)
	})

	t.Run("zero dimensions uses default", func(t *testing.T) {
		zeroCfg := cfg
		zeroCfg.Dimensions = 0
		p, err := NewEmbeddingProvider(pc, zeroCfg)
		require.NoError(t, err)
		require.NotNil(t, p)
		assert.Equal(t, pc.DefaultDims, p.cfg.Dimensions)
	})

	t.Run("negative dimensions uses default", func(t *testing.T) {
		negCfg := cfg
		negCfg.Dimensions = -5
		p, err := NewEmbeddingProvider(pc, negCfg)
		require.NoError(t, err)
		require.NotNil(t, p)
		assert.Equal(t, pc.DefaultDims, p.cfg.Dimensions)
	})

	t.Run("zero max retries uses default", func(t *testing.T) {
		zeroCfg := cfg
		zeroCfg.MaxRetries = 0
		p, err := NewEmbeddingProvider(pc, zeroCfg)
		require.NoError(t, err)
		require.NotNil(t, p)
		assert.Equal(t, 3, p.cfg.MaxRetries)
	})

	t.Run("negative max retries uses default", func(t *testing.T) {
		negCfg := cfg
		negCfg.MaxRetries = -1
		p, err := NewEmbeddingProvider(pc, negCfg)
		require.NoError(t, err)
		require.NotNil(t, p)
		assert.Equal(t, 3, p.cfg.MaxRetries)
	})

	t.Run("zero timeout uses default", func(t *testing.T) {
		zeroCfg := cfg
		zeroCfg.Timeout = 0
		p, err := NewEmbeddingProvider(pc, zeroCfg)
		require.NoError(t, err)
		require.NotNil(t, p)
		assert.Equal(t, 60*time.Second, p.cfg.Timeout)
	})

	t.Run("negative timeout uses default", func(t *testing.T) {
		negCfg := cfg
		negCfg.Timeout = -30 * time.Second
		p, err := NewEmbeddingProvider(pc, negCfg)
		require.NoError(t, err)
		require.NotNil(t, p)
		assert.Equal(t, 60*time.Second, p.cfg.Timeout)
	})

	t.Run("empty base URL uses default", func(t *testing.T) {
		emptyCfg := cfg
		emptyCfg.BaseURL = ""
		p, err := NewEmbeddingProvider(pc, emptyCfg)
		require.NoError(t, err)
		require.NotNil(t, p)
		assert.Equal(t, pc.DefaultBaseURL, p.cfg.BaseURL)
	})

	t.Run("zero batch size uses default", func(t *testing.T) {
		zeroCfg := cfg
		zeroCfg.BatchSize = 0
		p, err := NewEmbeddingProvider(pc, zeroCfg)
		require.NoError(t, err)
		require.NotNil(t, p)
		assert.Equal(t, 20, p.cfg.BatchSize)
	})

	t.Run("negative batch size uses default", func(t *testing.T) {
		negCfg := cfg
		negCfg.BatchSize = -5
		p, err := NewEmbeddingProvider(pc, negCfg)
		require.NoError(t, err)
		require.NotNil(t, p)
		assert.Equal(t, 20, p.cfg.BatchSize)
	})

	t.Run("zero RateLimitTPM uses default 300000", func(t *testing.T) {
		zeroPC := pc
		zeroPC.RateLimitTPM = 0
		p, err := NewEmbeddingProvider(zeroPC, cfg)
		require.NoError(t, err)
		require.NotNil(t, p)
		assert.Equal(t, 300000, p.rateLimiter.capacity)
	})

	t.Run("negative RateLimitTPM uses default 300000", func(t *testing.T) {
		negPC := pc
		negPC.RateLimitTPM = -1
		p, err := NewEmbeddingProvider(negPC, cfg)
		require.NoError(t, err)
		require.NotNil(t, p)
		assert.Equal(t, 300000, p.rateLimiter.capacity)
	})

	t.Run("all valid returns provider", func(t *testing.T) {
		p, err := NewEmbeddingProvider(pc, cfg)
		require.NoError(t, err)
		require.NotNil(t, p)
		assert.Equal(t, cfg.APIKey, p.cfg.APIKey)
		assert.Equal(t, cfg.Model, p.Model())
		assert.Equal(t, cfg.Dimensions, p.Dimensions())
		assert.Equal(t, pc.ProviderName, p.Name())
		assert.NotNil(t, p.client)
		assert.NotNil(t, p.rateLimiter)
	})

	t.Run("client is non-nil", func(t *testing.T) {
		p, err := NewEmbeddingProvider(pc, cfg)
		require.NoError(t, err)
		require.NotNil(t, p.client)
	})
}

func TestNewEmbeddingProvider_WithEmptyEnvVarName(t *testing.T) {
	t.Parallel()

	// When EnvVarName is empty, the error message should still be sensible.
	pc := testProviderConfig()
	pc.EnvVarName = ""
	cfg := testEmbeddingConfig()
	cfg.APIKey = ""

	p, err := NewEmbeddingProvider(pc, cfg)
	require.Error(t, err)
	assert.Nil(t, p)
}

// ── Accessors ──────────────────────────────────────────────────────────────────

func TestEmbeddingProvider_Accessors(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(readTestData(t, "embedding_success.json"))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)

	// Test all accessor methods
	assert.Equal(t, "test-provider", p.Name())
	assert.Equal(t, "text-embedding-3-small", p.Model())
	assert.Equal(t, 4, p.Dimensions())

	// Close is a no-op
	err := p.Close()
	assert.NoError(t, err)
}

func TestEmbeddingProvider_Cfg(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(readTestData(t, "embedding_success.json"))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	cfg := p.Cfg()
	assert.Equal(t, "test-key-12345", cfg.APIKey)
	assert.Equal(t, 4, cfg.Dimensions)
	assert.Equal(t, 10, cfg.BatchSize)
}

// ── EmbeddingConfigFromEnv ─────────────────────────────────────────────────────

func TestEmbeddingConfigFromEnv(t *testing.T) {
	// NOTE: No t.Parallel() because subtests use t.Setenv().

	pc := testProviderConfig()
	envKey := pc.EnvVarName

	t.Run("with env var set", func(t *testing.T) {
		t.Setenv(envKey, "env-api-key-789")
		cfg := EmbeddingConfigFromEnv(pc)
		assert.Equal(t, "env-api-key-789", cfg.APIKey)
		assert.Equal(t, pc.DefaultModel, cfg.Model)
		assert.Equal(t, pc.DefaultDims, cfg.Dimensions)
	})

	t.Run("without env var", func(t *testing.T) {
		// Ensure env var is NOT set
		os.Unsetenv(envKey)
		cfg := EmbeddingConfigFromEnv(pc)
		assert.Empty(t, cfg.APIKey)
		assert.Equal(t, pc.DefaultModel, cfg.Model)
	})

	t.Run("with empty env var", func(t *testing.T) {
		t.Setenv(envKey, "")
		cfg := EmbeddingConfigFromEnv(pc)
		assert.Empty(t, cfg.APIKey, "empty env var should not override")
	})
}

// ── RegisterEmbedding ──────────────────────────────────────────────────────────

func TestRegisterEmbedding(t *testing.T) {
	// NOTE: No t.Parallel() because the "registered factory creates provider"
	// subtest uses t.Setenv().

	t.Run("registers provider in global registry", func(t *testing.T) {
		// Reset the global registry to ensure clean state
		embeddings.ResetRegistry()

		pc := testProviderConfig()
		RegisterEmbedding(pc)

		registry := embeddings.GetRegistry()
		require.NotNil(t, registry)

		names := registry.List()
		assert.Contains(t, names, pc.RegistryName)
	})

	t.Run("registered factory creates provider", func(t *testing.T) {
		embeddings.ResetRegistry()

		// Set the env var so the factory can create the provider
		t.Setenv("TEST_API_KEY", "factory-key")

		pc := testProviderConfig()
		RegisterEmbedding(pc)

		registry := embeddings.GetRegistry()
		provider, ok := registry.Get(pc.RegistryName)
		require.True(t, ok, "provider should be retrievable from registry")
		require.NotNil(t, provider)
		assert.Equal(t, pc.ProviderName, provider.Name())
		assert.Equal(t, pc.DefaultModel, provider.Model())
	})
}

// ─── Lote 2: HTTP Tests with httptest ──────────────────────────────────────────

// ── embedBatch (core HTTP logic) ───────────────────────────────────────────────

func TestEmbedBatch_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request properties
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-key-12345", r.Header.Get("Authorization"))

		// Verify request body
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var reqMap map[string]interface{}
		err = json.Unmarshal(body, &reqMap)
		require.NoError(t, err)
		assert.Equal(t, "text-embedding-3-small", reqMap["model"])
		assert.Equal(t, "float", reqMap["encoding_format"])

		w.WriteHeader(http.StatusOK)
		w.Write(readTestData(t, "embedding_success.json"))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	results, err := p.embedBatch(context.Background(), []string{"hello world"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, []float64{0.001, 0.002, 0.003, 0.004}, results[0].Vector)
	assert.Equal(t, "text-embedding-3-small", results[0].Model)
	assert.Equal(t, 4, results[0].Dimensions)
	assert.Equal(t, 20, results[0].TokensUsed)
}

func TestEmbedBatch_Unauthorized(t *testing.T) {
	t.Parallel()

	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid api key"}`))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	_, err := p.embedBatch(context.Background(), []string{"test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid API key (401)")
	// Should NOT retry on 401 — only 1 call
	assert.Equal(t, 1, callCount)
}

func TestEmbedBatch_BadRequest(t *testing.T) {
	t.Parallel()

	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad input"}`))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	_, err := p.embedBatch(context.Background(), []string{"test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad request (400)")
	// Should NOT retry on 400
	assert.Equal(t, 1, callCount)
}

func TestEmbedBatch_RateLimitedThenSucceeds(t *testing.T) {
	t.Parallel()

	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limited"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(readTestData(t, "embedding_success.json"))
	}))
	defer server.Close()

	// Use more retries to allow recovery
	p := newTestProvider(t, server.URL)
	p.cfg.MaxRetries = 2
	results, err := p.embedBatch(context.Background(), []string{"test"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	// Should have retried after 429
	assert.Equal(t, 2, callCount)
}

func TestEmbedBatch_ServerErrorRetriesThenFails(t *testing.T) {
	t.Parallel()

	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	p.cfg.MaxRetries = 1
	_, err := p.embedBatch(context.Background(), []string{"test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embedding failed after 1 retries")
	// Should have attempted MaxRetries+1 times
	assert.Equal(t, 2, callCount)
}

func TestEmbedBatch_EmptyResponseData(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(readTestData(t, "embedding_empty_data.json"))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	_, err := p.embedBatch(context.Background(), []string{"test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no embedding data in response")
}

func TestEmbedBatch_MalformedJSONResponse(t *testing.T) {
	t.Parallel()

	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		w.Write(readTestData(t, "embedding_malformed.json"))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	p.cfg.MaxRetries = 1
	_, err := p.embedBatch(context.Background(), []string{"test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse response")
	// Should retry on malformed JSON
	assert.Equal(t, 2, callCount)
}

func TestEmbedBatch_HTTPRequestError(t *testing.T) {
	t.Parallel()

	// Use a server that closes immediately to simulate a connection error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Hijack and close to simulate network error
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	server.Close() // Close before using to force connection error

	p := newTestProvider(t, server.URL)
	p.cfg.MaxRetries = 1
	_, err := p.embedBatch(context.Background(), []string{"test"})
	require.Error(t, err)
	// Should contain either "http request" or connection-related error
	assert.Contains(t, err.Error(), "embedding failed after")
}

func TestEmbedBatch_ContextCancelled(t *testing.T) {
	t.Parallel()

	// Server that blocks to trigger context cancellation
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't write anything — just block
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	p.cfg.Timeout = 1 * time.Second
	p.client.Timeout = 1 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Let context expire
	time.Sleep(20 * time.Millisecond)

	_, err := p.embedBatch(ctx, []string{"test"})
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestEmbedBatch_RejectsShortVector(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a vector with 2 elements (less than modelDims=4)
		resp := map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{
					"object":    "embedding",
					"embedding": []float64{0.1, 0.2},
					"index":     0,
				},
			},
			"model": "text-embedding-3-small",
			"usage": map[string]int{
				"prompt_tokens": 5,
				"total_tokens":  10,
			},
		}
		data, _ := json.Marshal(resp)
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	_, err := p.embedBatch(context.Background(), []string{"hello"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embedding dimension mismatch")
}

func TestEmbedBatch_RejectsLongVector(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a vector with 10 elements (more than modelDims=4)
		vec := make([]float64, 10)
		for i := 0; i < 10; i++ {
			vec[i] = float64(i+1) * 0.1
		}
		resp := map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{
					"object":    "embedding",
					"embedding": vec,
					"index":     0,
				},
			},
			"model": "text-embedding-3-small",
			"usage": map[string]int{
				"prompt_tokens": 5,
				"total_tokens":  10,
			},
		}
		data, _ := json.Marshal(resp)
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	_, err := p.embedBatch(context.Background(), []string{"hello"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embedding dimension mismatch")
}

func TestEmbedBatch_ExactDimensions(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a vector with exactly 4 elements (same as modelDims)
		resp := map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{
					"object":    "embedding",
					"embedding": []float64{0.1, 0.2, 0.3, 0.4},
					"index":     0,
				},
			},
			"model": "text-embedding-3-small",
			"usage": map[string]int{
				"prompt_tokens": 5,
				"total_tokens":  10,
			},
		}
		data, _ := json.Marshal(resp)
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	results, err := p.embedBatch(context.Background(), []string{"hello"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 4, len(results[0].Vector))
	assert.Equal(t, []float64{0.1, 0.2, 0.3, 0.4}, results[0].Vector)
}

func TestEmbedBatch_HTTPErrorThenSuccess(t *testing.T) {
	t.Parallel()

	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 1 {
			// Fail with 503 (retryable) once
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"overloaded"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(readTestData(t, "embedding_success.json"))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	p.cfg.MaxRetries = 1
	results, err := p.embedBatch(context.Background(), []string{"test"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 2, callCount)
}

func TestEmbedBatch_RequestBodyContent(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(readTestData(t, "embedding_multi.json"))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	_, err := p.embedBatch(context.Background(), []string{"text1", "text2"})
	require.NoError(t, err)

	var reqBody map[string]interface{}
	err = json.Unmarshal(capturedBody, &reqBody)
	require.NoError(t, err)

	assert.Equal(t, "text-embedding-3-small", reqBody["model"])
	assert.Equal(t, "float", reqBody["encoding_format"])

	inputs, ok := reqBody["input"].([]interface{})
	require.True(t, ok)
	assert.Equal(t, "text1", inputs[0].(string))
	assert.Equal(t, "text2", inputs[1].(string))
}

// ── GenerateEmbedding (single → delegates to GenerateEmbeddings) ──────────────

func TestGenerateEmbedding_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(readTestData(t, "embedding_success.json"))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	result, err := p.GenerateEmbedding(context.Background(), "hello world")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, []float64{0.001, 0.002, 0.003, 0.004}, result.Vector)
	assert.Equal(t, "text-embedding-3-small", result.Model)
}

func TestGenerateEmbedding_EmptyResults(t *testing.T) {
	t.Parallel()

	// When embedBatch returns an empty-slice response, the error propagates
	// from embedBatch through GenerateEmbeddings to GenerateEmbedding.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Simulate a response with no data
		resp := map[string]interface{}{
			"object": "list",
			"data":   []interface{}{},
			"model":  "test",
			"usage":  map[string]int{"prompt_tokens": 0, "total_tokens": 0},
		}
		data, _ := json.Marshal(resp)
		w.Write(data)
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	_, err := p.GenerateEmbedding(context.Background(), "hello")
	require.Error(t, err)
	// The error comes from embedBatch: "batch 0: no embedding data in response"
	assert.Contains(t, err.Error(), "no embedding data in response")
}

func TestGenerateEmbedding_PropagatesBatchError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"bad key"}`))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	_, err := p.GenerateEmbedding(context.Background(), "hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid API key")
}

// ── GenerateEmbeddings (batching logic) ────────────────────────────────────────

func TestGenerateEmbeddings_SingleBatch(t *testing.T) {
	t.Parallel()

	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		w.Write(readTestData(t, "embedding_success.json"))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	results, err := p.GenerateEmbeddings(context.Background(), []string{"hello world"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 1, requestCount)
}

func TestGenerateEmbeddings_MultipleBatches(t *testing.T) {
	t.Parallel()

	var requestCount int
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		writeBatchEmbeddingResponse(t, w, r)
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	p.cfg.BatchSize = 2 // Small batch to force multiple batches

	texts := []string{"text1", "text2", "text3", "text4", "text5"}
	results, err := p.GenerateEmbeddings(context.Background(), texts)
	require.NoError(t, err)
	// Each batch returns one result per input, preserving the batch's index order.
	require.Len(t, results, len(texts))
	assert.InDeltaSlice(t, []float64{0.1, 0.2, 0.3, 0.4}, results[0].Vector, 1e-9)
	assert.InDeltaSlice(t, []float64{1.1, 1.2, 1.3, 1.4}, results[1].Vector, 1e-9)
	assert.InDeltaSlice(t, []float64{0.1, 0.2, 0.3, 0.4}, results[2].Vector, 1e-9)
	assert.InDeltaSlice(t, []float64{1.1, 1.2, 1.3, 1.4}, results[3].Vector, 1e-9)
	assert.InDeltaSlice(t, []float64{0.1, 0.2, 0.3, 0.4}, results[4].Vector, 1e-9)
	assert.Equal(t, 3, requestCount)
}

func TestGenerateEmbeddings_EmptyTexts(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(readTestData(t, "embedding_success.json"))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	_, err := p.GenerateEmbeddings(context.Background(), []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no texts provided")
}

func TestGenerateEmbeddings_NilTexts(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(readTestData(t, "embedding_success.json"))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	_, err := p.GenerateEmbeddings(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no texts provided")
}

func TestGenerateEmbeddings_BatchErrorPropagation(t *testing.T) {
	t.Parallel()

	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	p.cfg.MaxRetries = 0 // No retries to fail fast
	p.cfg.BatchSize = 2

	texts := []string{"text1", "text2", "text3"}
	_, err := p.GenerateEmbeddings(context.Background(), texts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "batch 0")
}

func TestGenerateEmbeddings_ExactBatchSize(t *testing.T) {
	t.Parallel()

	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		writeBatchEmbeddingResponse(t, w, r)
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	p.cfg.BatchSize = 3

	texts := []string{"a", "b", "c"}
	results, err := p.GenerateEmbeddings(context.Background(), texts)
	require.NoError(t, err)
	require.Len(t, results, len(texts))
	assert.InDeltaSlice(t, []float64{0.1, 0.2, 0.3, 0.4}, results[0].Vector, 1e-9)
	assert.InDeltaSlice(t, []float64{1.1, 1.2, 1.3, 1.4}, results[1].Vector, 1e-9)
	assert.InDeltaSlice(t, []float64{2.1, 2.2, 2.3, 2.4}, results[2].Vector, 1e-9)
	assert.Equal(t, 1, requestCount)
}

func TestGenerateEmbeddings_LargeNumberOfTexts(t *testing.T) {
	t.Parallel()

	var requestCount int
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		writeBatchEmbeddingResponse(t, w, r)
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	p.cfg.BatchSize = 5

	// 27 texts -> 6 batches (5+5+5+5+5+2)
	texts := make([]string, 27)
	for i := 0; i < 27; i++ {
		texts[i] = fmt.Sprintf("text-%d", i)
	}

	results, err := p.GenerateEmbeddings(context.Background(), texts)
	require.NoError(t, err)
	require.Len(t, results, len(texts))
	for i, result := range results {
		batchIndex := i % p.cfg.BatchSize
		assert.Equal(t, float64(batchIndex)+0.1, result.Vector[0])
	}
	assert.Equal(t, 6, requestCount)
}

// ── RateLimiter Wait in embedBatch ────────────────────────────────────────────

func TestEmbedBatch_RateLimiterBlocks(t *testing.T) {
	t.Parallel()

	// This test verifies that the rate limiter is called before the HTTP request.
	// We set a very low rate limit to force blocking.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(readTestData(t, "embedding_success.json"))
	}))
	defer server.Close()

	// Create a provider with a very slow rate limiter (1 TPM)
	cfg := testEmbeddingConfig()
	cfg.BaseURL = server.URL
	providerCfg := testProviderConfig()
	p := &EmbeddingProvider{
		cfg:         cfg,
		providerCfg: providerCfg,
		client:      &http.Client{Timeout: 5 * time.Second},
		rateLimiter: newRateLimiter(1), // Only 1 token per minute
		modelDims:   cfg.Dimensions,
	}

	// First call should succeed (we have 1 token)
	ctx := context.Background()
	results, err := p.embedBatch(ctx, []string{"hello"})
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Second call with the same text would need more tokens but fails
	// because the text is short (1 token estimate), and we have a capacity of 1
	// Actually with a ~0 token estimate from EstimateTokens("hello") = 5/4 = 1
	// So we consumed 1 token, leaving 0. Second call needs 1 token, which means waiting ~60s.
	ctx2, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err = p.embedBatch(ctx2, []string{"hello"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit wait")
}

// ── Concurrency ───────────────────────────────────────────────────────────────

func TestEmbeddingProvider_ConcurrentCalls(t *testing.T) {
	t.Parallel()

	var requestCount int
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		w.Write(readTestData(t, "embedding_success.json"))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	p.cfg.BatchSize = 20

	// Run 10 concurrent GenerateEmbeddings calls
	var wg sync.WaitGroup
	errs := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			texts := []string{fmt.Sprintf("concurrent-text-%d", id)}
			_, err := p.GenerateEmbeddings(context.Background(), texts)
			errs <- err
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		assert.NoError(t, err, "concurrent call should not error")
	}

	assert.Equal(t, 10, requestCount)
}

// ── HTTP Timeout Test ─────────────────────────────────────────────────────────

func TestEmbedBatch_HTTPClientTimeout(t *testing.T) {
	t.Parallel()

	// Server that takes longer than client timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write(readTestData(t, "embedding_success.json"))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	p.client.Timeout = 50 * time.Millisecond

	_, err := p.embedBatch(context.Background(), []string{"test"})
	require.Error(t, err)
	// The error should be a timeout/context-related error wrapped in "http request"
	assert.Contains(t, err.Error(), "http request")
}

// ── Integration: Full Round Trip ──────────────────────────────────────────────

func TestGenerateEmbeddings_Integration_RoundTrip(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read and verify the request
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var req map[string]interface{}
		err = json.Unmarshal(body, &req)
		require.NoError(t, err)

		assert.Equal(t, "text-embedding-3-small", req["model"])
		assert.Equal(t, "float", req["encoding_format"])

		inputs := req["input"].([]interface{})
		assert.Equal(t, "integration-test-text", inputs[0].(string))

		// Return a proper response
		resp := map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{
					"object":    "embedding",
					"embedding": []float64{0.5, 0.6, 0.7, 0.8},
					"index":     0,
				},
			},
			"model": "text-embedding-3-small",
			"usage": map[string]int{
				"prompt_tokens": 4,
				"total_tokens":  8,
			},
		}
		data, _ := json.Marshal(resp)
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	result, err := p.GenerateEmbedding(context.Background(), "integration-test-text")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, []float64{0.5, 0.6, 0.7, 0.8}, result.Vector)
	assert.Equal(t, 4, result.Dimensions)
	assert.Equal(t, 8, result.TokensUsed)
}

// ── NewEmbeddingProvider with client & rateLimiter ────────────────────────────

func TestNewEmbeddingProvider_RateLimiterConfiguration(t *testing.T) {
	t.Parallel()

	t.Run("custom TPM from provider config", func(t *testing.T) {
		pc := testProviderConfig()
		pc.RateLimitTPM = 50000
		cfg := testEmbeddingConfig()

		p, err := NewEmbeddingProvider(pc, cfg)
		require.NoError(t, err)
		require.NotNil(t, p)
		assert.Equal(t, 50000, p.rateLimiter.capacity)
	})

	t.Run("default TPM when RateLimitTPM is zero", func(t *testing.T) {
		pc := testProviderConfig()
		pc.RateLimitTPM = 0
		cfg := testEmbeddingConfig()

		p, err := NewEmbeddingProvider(pc, cfg)
		require.NoError(t, err)
		require.NotNil(t, p)
		assert.Equal(t, 300000, p.rateLimiter.capacity)
	})
}

// ── Error message format tests ────────────────────────────────────────────────

func TestHandleEmbeddingAPIError_ErrorFormatting(t *testing.T) {
	t.Parallel()

	err := handleEmbeddingAPIError("my-provider", http.StatusForbidden, "access denied")
	require.Error(t, err)
	// The handleEmbeddingAPIError function doesn't have a specific case for 403,
	// so it falls through to the default case (status >= 500 is false, so "unexpected status")
	// Actually wait: 403 is not >= 500, so it goes to "unexpected status"
	assert.Contains(t, err.Error(), "unexpected status 403")

	err = handleEmbeddingAPIError("my-provider", http.StatusNotFound, "not found")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 404")
}

// ── Validate request headers in embedBatch ─────────────────────────────────────

func TestEmbedBatch_AuthorizationHeader(t *testing.T) {
	t.Parallel()

	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write(readTestData(t, "embedding_success.json"))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	_, err := p.embedBatch(context.Background(), []string{"test"})
	require.NoError(t, err)
	assert.Equal(t, "Bearer test-key-12345", authHeader)
}

func TestEmbedBatch_ContentTypeHeader(t *testing.T) {
	t.Parallel()

	var contentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		w.Write(readTestData(t, "embedding_success.json"))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	_, err := p.embedBatch(context.Background(), []string{"test"})
	require.NoError(t, err)
	assert.Equal(t, "application/json", contentType)
}

func TestEmbedBatch_RejectsIncompleteAndDuplicateIndexes(t *testing.T) {
	for _, tc := range []struct {
		name string
		data string
	}{
		{"missing", `{"data":[{"embedding":[0.1,0.2,0.3,0.4],"index":0}],"usage":{"total_tokens":4}}`},
		{"duplicate", `{"data":[{"embedding":[0.1,0.2,0.3,0.4],"index":0},{"embedding":[0.1,0.2,0.3,0.4],"index":0}],"usage":{"total_tokens":4}}`},
		{"out-of-range", `{"data":[{"embedding":[0.1,0.2,0.3,0.4],"index":2},{"embedding":[0.1,0.2,0.3,0.4],"index":0}],"usage":{"total_tokens":4}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.data))
			}))
			defer server.Close()
			p := newTestProvider(t, server.URL)
			_, err := p.embedBatch(context.Background(), []string{"one", "two"})
			require.Error(t, err)
		})
	}
}

func TestEmbedBatch_DistributesBatchUsageOnce(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"embedding":[0.1,0.2,0.3,0.4],"index":0},{"embedding":[0.1,0.2,0.3,0.4],"index":1}],"usage":{"total_tokens":5}}`))
	}))
	defer server.Close()
	p := newTestProvider(t, server.URL)
	results, err := p.embedBatch(context.Background(), []string{"one", "two"})
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, 5, results[0].TokensUsed+results[1].TokensUsed)
}
