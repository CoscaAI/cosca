package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time assertion that Provider satisfies the embeddings.Provider interface.
var _ embeddings.Provider = (*Provider)(nil)

// validConfig returns a Config with an API key and sensible defaults for tests.
func validConfig() Config {
	cfg := DefaultConfig()
	cfg.APIKey = "test-openai-key"
	return cfg
}

// successResponse builds an OpenAI-style embeddings response for the given vector.
func successResponse(vector []float64, model string, totalTokens int) []byte {
	resp := map[string]interface{}{
		"object": "list",
		"data": []map[string]interface{}{
			{"object": "embedding", "embedding": vector, "index": 0},
		},
		"model": model,
		"usage": map[string]int{"prompt_tokens": 5, "total_tokens": totalTokens},
	}
	data, _ := json.Marshal(resp)
	return data
}

// newTestProvider creates a Provider configured to hit the given server URL.
func newTestProvider(t *testing.T, serverURL string) *Provider {
	t.Helper()
	cfg := validConfig()
	cfg.BaseURL = serverURL
	cfg.MaxRetries = 0 // fail fast unless a test overrides it
	p, err := New(cfg)
	require.NoError(t, err)
	return p
}

// ── Configuration ──────────────────────────────────────────────────────────────

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, DefaultLocalModel, cfg.Model)
	assert.Equal(t, DefaultLocalDimensions, cfg.Dimensions)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 60*time.Second, cfg.Timeout)
	assert.Equal(t, DefaultLocalBaseURL, cfg.BaseURL)
	assert.Equal(t, 1000000, cfg.RateLimit)
	assert.Equal(t, 20, cfg.BatchSize)
	// APIKey is intentionally left empty to avoid coupling to environment vars.
	assert.Empty(t, cfg.APIKey)
}

func TestConfigFromEnv(t *testing.T) {
	t.Run("with env var set", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "env-key-456")
		cfg := ConfigFromEnv()
		assert.Equal(t, "env-key-456", cfg.APIKey)
		assert.Equal(t, DefaultLocalModel, cfg.Model)
		assert.Equal(t, DefaultLocalDimensions, cfg.Dimensions)
	})

	t.Run("without env var", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "")
		cfg := ConfigFromEnv()
		assert.Empty(t, cfg.APIKey)
		assert.Equal(t, DefaultLocalModel, cfg.Model)
	})

	t.Run("COSCA_EMBEDDING_BASE_URL sets base URL", func(t *testing.T) {
		t.Setenv("COSCA_EMBEDDING_BASE_URL", "http://127.0.0.1:11435/v1")
		cfg := ConfigFromEnv()
		assert.Equal(t, "http://127.0.0.1:11435/v1", cfg.BaseURL)
	})

	t.Run("no COSCA_EMBEDDING_BASE_URL keeps default base URL", func(t *testing.T) {
		t.Setenv("COSCA_EMBEDDING_BASE_URL", "")
		cfg := ConfigFromEnv()
		assert.Equal(t, DefaultLocalBaseURL, cfg.BaseURL)
	})
}

// TestNew_LocalBaseURL verifies a provider built with a local base URL targets
// <baseURL>/embeddings (a local OpenAI-compatible server) rather than the
// default api.openai.com endpoint.
func TestNew_LocalBaseURL(t *testing.T) {
	cfg := Config{
		APIKey:  "dummy-local-key",
		BaseURL: "http://127.0.0.1:11435/v1",
	}
	p, err := New(cfg)
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, "http://127.0.0.1:11435/v1", p.cfg.BaseURL)
	assert.Equal(t, "http://127.0.0.1:11435/v1/embeddings", p.cfg.BaseURL+"/embeddings")
}

// TestFactory_LocalBaseURLOverride verifies the registry override (a local
// OpenAI-compatible base URL) flows through the factory into the provider,
// even when an OPENAI_API_KEY is present in the environment.
func TestFactory_LocalBaseURLOverride(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "env-key")

	_, factory := Factory()
	p, err := factory(context.Background(), &embeddings.Config{
		APIKey:  "dummy-local-key",
		BaseURL: "http://127.0.0.1:11435/v1",
	})
	require.NoError(t, err)

	prov, ok := p.(*Provider)
	require.True(t, ok, "factory should return *Provider")
	assert.Equal(t, "http://127.0.0.1:11435/v1", prov.cfg.BaseURL)
	assert.Equal(t, "dummy-local-key", prov.cfg.APIKey)
}

// ── Construction ───────────────────────────────────────────────────────────────

func TestNew(t *testing.T) {
	t.Run("missing API key returns error", func(t *testing.T) {
		p, err := New(DefaultConfig())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "OPENAI_API_KEY")
		assert.Contains(t, err.Error(), "not set")
		assert.Nil(t, p)
	})

	t.Run("valid config returns provider", func(t *testing.T) {
		p, err := New(validConfig())
		require.NoError(t, err)
		require.NotNil(t, p)
		assert.NotNil(t, p.client)
		assert.NotNil(t, p.rateLimiter)
		assert.Equal(t, "test-openai-key", p.cfg.APIKey)
		assert.Equal(t, DefaultLocalDimensions, p.modelDims)
	})
}

// TestNew_AppliesDefaults verifies that zero/negative values fall back to defaults.
func TestNew_AppliesDefaults(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*Config)
		checkField string
		wantInt    int
		wantStr    string
		wantDur    time.Duration
	}{
		{
			name:       "empty model uses local model default",
			mutate:     func(c *Config) { c.Model = "" },
			checkField: "model",
			wantStr:    DefaultLocalModel,
		},
		{
			name:       "zero dimensions uses model default",
			mutate:     func(c *Config) { c.Dimensions = 0 },
			checkField: "dims",
			wantInt:    DefaultLocalDimensions,
		},
		{
			name:       "negative dimensions uses model default",
			mutate:     func(c *Config) { c.Dimensions = -5 },
			checkField: "dims",
			wantInt:    DefaultLocalDimensions,
		},
		{
			name:       "zero max retries uses default",
			mutate:     func(c *Config) { c.MaxRetries = 0 },
			checkField: "retries",
			wantInt:    3,
		},
		{
			name:       "negative max retries uses default",
			mutate:     func(c *Config) { c.MaxRetries = -1 },
			checkField: "retries",
			wantInt:    3,
		},
		{
			name:       "zero timeout uses default",
			mutate:     func(c *Config) { c.Timeout = 0 },
			checkField: "timeout",
			wantDur:    60 * time.Second,
		},
		{
			name:       "negative timeout uses default",
			mutate:     func(c *Config) { c.Timeout = -30 * time.Second },
			checkField: "timeout",
			wantDur:    60 * time.Second,
		},
		{
			name:       "empty base URL uses default",
			mutate:     func(c *Config) { c.BaseURL = "" },
			checkField: "baseurl",
			wantStr:    DefaultLocalBaseURL,
		},
		{
			name:       "zero rate limit uses default",
			mutate:     func(c *Config) { c.RateLimit = 0 },
			checkField: "ratelimit",
			wantInt:    1000000,
		},
		{
			name:       "zero batch size uses default",
			mutate:     func(c *Config) { c.BatchSize = 0 },
			checkField: "batch",
			wantInt:    20,
		},
		{
			name:       "negative batch size uses default",
			mutate:     func(c *Config) { c.BatchSize = -5 },
			checkField: "batch",
			wantInt:    20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(&cfg)
			p, err := New(cfg)
			require.NoError(t, err)
			require.NotNil(t, p)

			switch tt.checkField {
			case "model":
				assert.Equal(t, tt.wantStr, p.cfg.Model)
			case "dims":
				assert.Equal(t, tt.wantInt, p.cfg.Dimensions)
			case "retries":
				assert.Equal(t, tt.wantInt, p.cfg.MaxRetries)
			case "timeout":
				assert.Equal(t, tt.wantDur, p.cfg.Timeout)
			case "baseurl":
				assert.Equal(t, tt.wantStr, p.cfg.BaseURL)
			case "ratelimit":
				assert.Equal(t, tt.wantInt, p.cfg.RateLimit)
			case "batch":
				assert.Equal(t, tt.wantInt, p.cfg.BatchSize)
			}
		})
	}
}

func TestNew_ModelSpecificDimensions(t *testing.T) {
	tests := []struct {
		model string
		want  int
	}{
		{ModelSmall, DimSmall},
		{ModelLarge, DimLarge},
		{ModelAda, DimAda},
		{DefaultLocalModel, DefaultLocalDimensions},
		{"some-unknown-model", DimSmall}, // unknown models fall back to small dims
		{"", DefaultLocalDimensions},     // empty model falls back to the local model
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			cfg := validConfig()
			cfg.Model = tt.model
			cfg.Dimensions = 0
			p, err := New(cfg)
			require.NoError(t, err)
			assert.Equal(t, tt.want, p.Dimensions())
		})
	}
}

// ── Accessors ─────────────────────────────────────────────────────────────────

func TestProviderAccessors(t *testing.T) {
	p, err := New(validConfig())
	require.NoError(t, err)

	assert.Equal(t, "openai", p.Name())
	assert.Equal(t, DefaultLocalModel, p.Model())
	assert.Equal(t, DefaultLocalDimensions, p.Dimensions())
	assert.NoError(t, p.Close())
}

// ── Pure Functions ────────────────────────────────────────────────────────────

func TestHandleAPIError(t *testing.T) {
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
			wantErr:    "openai: invalid API key (401)",
		},
		{
			name:       "429 Rate Limited",
			statusCode: http.StatusTooManyRequests,
			body:       `{"error":"too many requests"}`,
			wantErr:    "openai: rate limited (429):",
		},
		{
			name:       "503 Service Unavailable",
			statusCode: http.StatusServiceUnavailable,
			body:       "",
			wantErr:    "openai: service unavailable (503)",
		},
		{
			name:       "400 Bad Request",
			statusCode: http.StatusBadRequest,
			body:       `{"error":"bad input"}`,
			wantErr:    "openai: bad request (400):",
		},
		{
			name:       "500 Internal Server Error",
			statusCode: http.StatusInternalServerError,
			body:       "server error",
			wantErr:    "openai: server error (500):",
		},
		{
			name:       "502 Bad Gateway",
			statusCode: http.StatusBadGateway,
			body:       "bad gateway",
			wantErr:    "openai: server error (502):",
		},
		{
			name:       "404 Not Found (unexpected)",
			statusCode: http.StatusNotFound,
			body:       "not found",
			wantErr:    "openai: unexpected status 404:",
		},
		{
			name:       "200 OK (should not happen)",
			statusCode: http.StatusOK,
			body:       "ok",
			wantErr:    "openai: unexpected status 200:",
		},
		{
			name:       "0 (unknown status)",
			statusCode: 0,
			body:       "empty",
			wantErr:    "openai: unexpected status 0:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handleAPIError(tt.statusCode, tt.body)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestHandleAPIError_TruncatesBody(t *testing.T) {
	// Long bodies should be truncated to 200 chars in the error message.
	longBody := string(make([]byte, 500))
	err := handleAPIError(http.StatusInternalServerError, longBody)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "...")
	assert.NotContains(t, err.Error(), string(make([]byte, 300)))
}

func TestIsBadRequest(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantBad bool
	}{
		{name: "400 bad request", err: fmt.Errorf("openai: bad request (400): invalid"), wantBad: true},
		{name: "401 unauthorized", err: fmt.Errorf("openai: invalid API key (401)"), wantBad: true},
		{name: "403 forbidden", err: fmt.Errorf("openai: forbidden (403)"), wantBad: true},
		{name: "429 rate limited", err: fmt.Errorf("openai: rate limited (429)"), wantBad: false},
		{name: "500 server error", err: fmt.Errorf("openai: server error (500)"), wantBad: false},
		{name: "503 unavailable", err: fmt.Errorf("openai: service unavailable (503)"), wantBad: false},
		{name: "random error", err: errors.New("connection refused"), wantBad: false},
		{name: "400 without parens", err: errors.New("error 400"), wantBad: false},
		{name: "4000 in parens is not 400", err: errors.New("error (4000)"), wantBad: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantBad, isBadRequest(tt.err))
		})
	}
}

// ── HTTP: sendRequest / embedBatch success paths ──────────────────────────────

func TestGenerateEmbedding_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/embeddings", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-openai-key", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		w.Write(successResponse([]float64{0.1, 0.2, 0.3, 0.4}, ModelSmall, 12))
	}))
	defer server.Close()

	cfg := validConfig()
	cfg.BaseURL = server.URL
	cfg.Dimensions = 4
	cfg.MaxRetries = 0
	p, err := New(cfg)
	require.NoError(t, err)

	result, err := p.GenerateEmbedding(context.Background(), "hello world")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, []float64{0.1, 0.2, 0.3, 0.4}, result.Vector)
	assert.Equal(t, ModelSmall, result.Model)
	assert.Equal(t, 4, result.Dimensions)
	assert.Equal(t, 12, result.TokensUsed)
}

func TestGenerateEmbedding_NoEmbeddingData(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"object": "list",
			"data":   []interface{}{},
			"model":  ModelSmall,
			"usage":  map[string]int{"prompt_tokens": 0, "total_tokens": 0},
		}
		data, _ := json.Marshal(resp)
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	// New() normalizes MaxRetries<=0 to 3, so override post-construction to fail fast:
	// the "no embedding data" error is retryable and would otherwise wait 1s+4s+9s.
	p.cfg.MaxRetries = 0
	_, err := p.GenerateEmbedding(context.Background(), "hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no embedding data in response")
}

// ── HTTP: request body content ────────────────────────────────────────────────

func TestEmbedBatch_RequestBodyContent(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write(successResponse([]float64{0.1, 0.2, 0.3, 0.4}, ModelSmall, 8))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	_, err := p.GenerateEmbeddings(context.Background(), []string{"text1", "text2"})
	require.NoError(t, err)

	var reqBody map[string]interface{}
	require.NoError(t, json.Unmarshal(capturedBody, &reqBody))
	assert.Equal(t, DefaultLocalModel, reqBody["model"])
	assert.Equal(t, "float", reqBody["encoding_format"])
	assert.Equal(t, float64(DefaultLocalDimensions), reqBody["dimensions"])

	inputs, ok := reqBody["input"].([]interface{})
	require.True(t, ok)
	assert.Equal(t, "text1", inputs[0].(string))
	assert.Equal(t, "text2", inputs[1].(string))
}

func TestEmbedBatch_NoDimensionsForAdaModel(t *testing.T) {
	t.Parallel()

	var capturedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write(successResponse(make([]float64, DimAda), ModelAda, 8))
	}))
	defer server.Close()

	cfg := validConfig()
	cfg.BaseURL = server.URL
	cfg.Model = ModelAda
	cfg.Dimensions = DimAda
	cfg.MaxRetries = 0
	p, err := New(cfg)
	require.NoError(t, err)

	_, err = p.GenerateEmbeddings(context.Background(), []string{"text"})
	require.NoError(t, err)

	var reqBody map[string]interface{}
	require.NoError(t, json.Unmarshal(capturedBody, &reqBody))
	assert.Equal(t, ModelAda, reqBody["model"])
	// The "dimensions" key must NOT be sent for ada-002.
	_, hasDimensions := reqBody["dimensions"]
	assert.False(t, hasDimensions)
}

// ── HTTP: error handling & retries ────────────────────────────────────────────

func TestEmbedBatch_UnauthorizedNoRetry(t *testing.T) {
	t.Parallel()

	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid api key"}`))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	p.cfg.MaxRetries = 2
	_, err := p.GenerateEmbedding(context.Background(), "hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid API key (401)")
	// Non-retryable: exactly one request.
	assert.Equal(t, 1, callCount)
}

func TestEmbedBatch_BadRequestNoRetry(t *testing.T) {
	t.Parallel()

	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad input"}`))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	p.cfg.MaxRetries = 2
	_, err := p.GenerateEmbedding(context.Background(), "hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad request (400)")
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
		w.Write(successResponse([]float64{0.1, 0.2, 0.3, 0.4}, ModelSmall, 8))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	p.cfg.MaxRetries = 1
	results, err := p.GenerateEmbeddings(context.Background(), []string{"hello"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	// 429 is retryable: first attempt fails, second succeeds.
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
	_, err := p.GenerateEmbeddings(context.Background(), []string{"hello"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "openai embedding failed after 1 retries")
	// MaxRetries + 1 attempts.
	assert.Equal(t, 2, callCount)
}

func TestEmbedBatch_MalformedJSONRetries(t *testing.T) {
	t.Parallel()

	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{not valid json`))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	p.cfg.MaxRetries = 0
	_, err := p.GenerateEmbeddings(context.Background(), []string{"hello"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse response")
	assert.Equal(t, 1, callCount)
}

func TestEmbedBatch_ConnectionError(t *testing.T) {
	t.Parallel()

	// Closing the server forces a connection error.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	serverURL := server.URL
	server.Close()

	p := newTestProvider(t, serverURL)
	p.cfg.MaxRetries = 0
	_, err := p.GenerateEmbeddings(context.Background(), []string{"hello"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http request")
}

func TestEmbedBatch_ContextCancelledDuringBackoff(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	p.cfg.MaxRetries = 1 // forces a backoff wait after the first failure

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := p.GenerateEmbeddings(ctx, []string{"hello"})
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestEmbedBatch_HTTPClientTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	p.client.Timeout = 50 * time.Millisecond
	p.cfg.MaxRetries = 0

	_, err := p.GenerateEmbeddings(context.Background(), []string{"hello"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http request")
}

// ── HTTP: vector dimension normalization ──────────────────────────────────────

func TestEmbedBatch_VectorPadding(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a 2-dim vector; the provider should pad to modelDims (768).
		w.WriteHeader(http.StatusOK)
		w.Write(successResponse([]float64{0.1, 0.2}, DefaultLocalModel, 8))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	results, err := p.GenerateEmbeddings(context.Background(), []string{"hello"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Len(t, results[0].Vector, DefaultLocalDimensions)
	assert.Equal(t, []float64{0.1, 0.2}, results[0].Vector[:2])
	assert.Equal(t, DefaultLocalDimensions, results[0].Dimensions)
}

func TestEmbedBatch_VectorTruncation(t *testing.T) {
	t.Parallel()

	vec := make([]float64, 900)
	for i := range vec {
		vec[i] = float64(i)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a 900-dim vector; the provider should truncate to modelDims (768).
		w.WriteHeader(http.StatusOK)
		w.Write(successResponse(vec, DefaultLocalModel, 8))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	results, err := p.GenerateEmbeddings(context.Background(), []string{"hello"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Len(t, results[0].Vector, DefaultLocalDimensions)
	assert.Equal(t, DefaultLocalDimensions, results[0].Dimensions)
}

func TestEmbedBatch_ExactDimensions(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(successResponse([]float64{0.1, 0.2, 0.3, 0.4}, ModelSmall, 8))
	}))
	defer server.Close()

	cfg := validConfig()
	cfg.BaseURL = server.URL
	cfg.Dimensions = 4
	cfg.MaxRetries = 0
	p, err := New(cfg)
	require.NoError(t, err)

	results, err := p.GenerateEmbeddings(context.Background(), []string{"hello"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, []float64{0.1, 0.2, 0.3, 0.4}, results[0].Vector)
	assert.Equal(t, 4, results[0].Dimensions)
}

// ── Batching logic ────────────────────────────────────────────────────────────

func TestGenerateEmbeddings_EmptyTexts(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(successResponse([]float64{0.1}, ModelSmall, 8))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	_, err := p.GenerateEmbeddings(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no texts provided")
}

func TestGenerateEmbeddings_MultipleBatches(t *testing.T) {
	t.Parallel()

	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		w.Write(successResponse([]float64{0.1, 0.2, 0.3, 0.4}, ModelSmall, 8))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	p.cfg.BatchSize = 2

	// 5 texts → batches [0:2], [2:4], [4:5] = 3 requests = 3 results
	texts := []string{"a", "b", "c", "d", "e"}
	results, err := p.GenerateEmbeddings(context.Background(), texts)
	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Equal(t, 3, requestCount)
}

func TestGenerateEmbeddings_BatchErrorPropagation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"boom"}`))
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	p.cfg.MaxRetries = 0 // fail fast: 500 is retryable and New() normalized MaxRetries<=0 to 3
	p.cfg.BatchSize = 2

	_, err := p.GenerateEmbeddings(context.Background(), []string{"a", "b", "c"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "batch 0")
}

// ── Rate limiting ─────────────────────────────────────────────────────────────

func TestEmbedBatch_RateLimiterBlocks(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(successResponse([]float64{0.1}, ModelSmall, 8))
	}))
	defer server.Close()

	cfg := validConfig()
	cfg.BaseURL = server.URL
	cfg.RateLimit = 1 // one token per minute
	cfg.MaxRetries = 0
	p, err := New(cfg)
	require.NoError(t, err)

	// First call consumes the only token ("hello" estimates to 1 token).
	_, err = p.GenerateEmbeddings(context.Background(), []string{"hello"})
	require.NoError(t, err)

	// Second call must wait for a refill; a short-lived context cancels it.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err = p.GenerateEmbeddings(ctx, []string{"hello"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit wait")
}

// ── Factory & Register ────────────────────────────────────────────────────────

func TestFactory(t *testing.T) {
	t.Run("returns name and provider", func(t *testing.T) {
		name, factory := Factory()
		assert.Equal(t, "openai", name)
		require.NotNil(t, factory)
	})

	t.Run("nil config requires env var", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "")
		_, factory := Factory()
		_, err := factory(context.Background(), nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "OPENAI_API_KEY")
	})

	t.Run("config overrides env values", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "env-key")
		_, factory := Factory()

		cfg := &embeddings.Config{
			APIKey:     "override-key",
			Model:      ModelAda,
			Dimensions: 512,
			BaseURL:    "http://custom.example.com/v1",
		}
		p, err := factory(context.Background(), cfg)
		require.NoError(t, err)
		assert.Equal(t, "openai", p.Name())
		assert.Equal(t, ModelAda, p.Model())
		assert.Equal(t, 512, p.Dimensions())
	})

	t.Run("partial config keeps defaults", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "env-key")
		_, factory := Factory()

		p, err := factory(context.Background(), &embeddings.Config{APIKey: "key-only"})
		require.NoError(t, err)
		assert.Equal(t, DefaultLocalModel, p.Model())
		assert.Equal(t, DefaultLocalDimensions, p.Dimensions())
	})
}

func TestRegister(t *testing.T) {
	// NOTE: sequential (no t.Parallel) because it touches the global registry
	// and uses t.Setenv.

	embeddings.ResetRegistry()
	t.Setenv("OPENAI_API_KEY", "register-key")

	Register()

	registry := embeddings.GetRegistry()
	names := registry.List()
	assert.Contains(t, names, "openai")

	provider, ok := registry.Get("openai")
	require.True(t, ok, "openai provider should be retrievable from registry")
	require.NotNil(t, provider)
	assert.Equal(t, "openai", provider.Name())
	assert.Equal(t, DefaultLocalModel, provider.Model())
}
