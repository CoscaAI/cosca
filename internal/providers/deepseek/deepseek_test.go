package deepseek

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/CoscaAI/cosca/internal/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time assertion that Provider satisfies the embeddings.Provider interface.
var _ embeddings.Provider = (*Provider)(nil)

// validTestConfig returns a Config with an API key and default values,
// ready for HTTP-level tests (BaseURL is overridden per test).
func validTestConfig() Config {
	cfg := DefaultConfig()
	cfg.APIKey = "test-deepseek-key"
	return cfg
}

// deepseekSuccessJSON returns a minimal DeepSeek embeddings API response
// containing two 3-dimensional embeddings.
func deepseekSuccessJSON() []byte {
	resp := map[string]interface{}{
		"data": []map[string]interface{}{
			{"embedding": []float64{0.1, 0.2, 0.3}, "index": 0},
			{"embedding": []float64{0.4, 0.5, 0.6}, "index": 1},
		},
		"usage": map[string]int{"prompt_tokens": 10, "total_tokens": 20},
	}
	data, _ := json.Marshal(resp)
	return data
}

// ── Configuration ──────────────────────────────────────────────────────────────

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, defaultModel, cfg.Model)
	assert.Equal(t, defaultDims, cfg.Dimensions)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 60*time.Second, cfg.Timeout)
	assert.Equal(t, defaultURL, cfg.BaseURL)
	assert.Equal(t, 20, cfg.BatchSize)
	// APIKey is intentionally left empty to avoid coupling to environment vars.
	assert.Empty(t, cfg.APIKey)
}

func TestConfigFromEnv(t *testing.T) {
	t.Run("with env var set", func(t *testing.T) {
		t.Setenv("DEEPSEEK_API_KEY", "env-key-123")
		cfg := ConfigFromEnv()
		assert.Equal(t, "env-key-123", cfg.APIKey)
		assert.Equal(t, defaultModel, cfg.Model)
		assert.Equal(t, defaultDims, cfg.Dimensions)
	})

	t.Run("without env var", func(t *testing.T) {
		t.Setenv("DEEPSEEK_API_KEY", "")
		cfg := ConfigFromEnv()
		assert.Empty(t, cfg.APIKey)
		assert.Equal(t, defaultModel, cfg.Model)
	})
}

// ── Construction ───────────────────────────────────────────────────────────────

func TestNew(t *testing.T) {
	t.Run("missing API key returns error", func(t *testing.T) {
		p, err := New(DefaultConfig())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "DEEPSEEK_API_KEY")
		assert.Contains(t, err.Error(), "not set")
		assert.Nil(t, p)
	})

	t.Run("valid config returns provider", func(t *testing.T) {
		p, err := New(validTestConfig())
		require.NoError(t, err)
		require.NotNil(t, p)
		assert.NotNil(t, p.client)
		assert.NotNil(t, p.rateLimiter)
		assert.Equal(t, "test-deepseek-key", p.cfg.APIKey)
	})
}

// ── Accessors ─────────────────────────────────────────────────────────────────

func TestProviderAccessors(t *testing.T) {
	p, err := New(validTestConfig())
	require.NoError(t, err)

	assert.Equal(t, "deepseek", p.Name())
	assert.Equal(t, defaultModel, p.Model())
	assert.Equal(t, defaultDims, p.Dimensions())
	assert.NoError(t, p.Close())
}

// ── GenerateEmbeddings (HTTP) ─────────────────────────────────────────────────

func TestGenerateEmbeddings_Success(t *testing.T) {
	t.Parallel()

	var method, contentType, authHeader string
	var capturedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		contentType = r.Header.Get("Content-Type")
		authHeader = r.Header.Get("Authorization")
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write(deepseekSuccessJSON())
	}))
	defer server.Close()

	cfg := validTestConfig()
	cfg.BaseURL = server.URL
	p, err := New(cfg)
	require.NoError(t, err)

	results, err := p.GenerateEmbeddings(context.Background(), []string{"hello", "world"})
	require.NoError(t, err)
	require.Len(t, results, 2)

	// Request verification.
	assert.Equal(t, http.MethodPost, method)
	assert.Equal(t, "application/json", contentType)
	assert.Equal(t, "Bearer test-deepseek-key", authHeader)

	var reqBody map[string]interface{}
	require.NoError(t, json.Unmarshal(capturedBody, &reqBody))
	assert.Equal(t, defaultModel, reqBody["model"])
	inputs, ok := reqBody["input"].([]interface{})
	require.True(t, ok)
	assert.Equal(t, "hello", inputs[0].(string))
	assert.Equal(t, "world", inputs[1].(string))

	// Response verification.
	assert.Equal(t, []float64{0.1, 0.2, 0.3}, results[0].Vector)
	assert.Equal(t, []float64{0.4, 0.5, 0.6}, results[1].Vector)
	assert.Equal(t, defaultModel, results[0].Model)
	assert.Equal(t, 3, results[0].Dimensions)
	// Tokens are split evenly across results: 20 total / 2 results.
	assert.Equal(t, 10, results[0].TokensUsed)
}

func TestGenerateEmbeddings_EmptyTexts(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(deepseekSuccessJSON())
	}))
	defer server.Close()

	cfg := validTestConfig()
	cfg.BaseURL = server.URL
	p, err := New(cfg)
	require.NoError(t, err)

	_, err = p.GenerateEmbeddings(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no texts provided")
}

func TestGenerateEmbeddings_HTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server exploded"}`))
	}))
	defer server.Close()

	cfg := validTestConfig()
	cfg.BaseURL = server.URL
	p, err := New(cfg)
	require.NoError(t, err)

	_, err = p.GenerateEmbeddings(context.Background(), []string{"hello"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API error 500")
	assert.Contains(t, err.Error(), "server exploded")
}

func TestGenerateEmbeddings_Unauthorized(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid key"}`))
	}))
	defer server.Close()

	cfg := validTestConfig()
	cfg.BaseURL = server.URL
	p, err := New(cfg)
	require.NoError(t, err)

	_, err = p.GenerateEmbeddings(context.Background(), []string{"hello"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API error 401")
}

func TestGenerateEmbeddings_MalformedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{not valid json`))
	}))
	defer server.Close()

	cfg := validTestConfig()
	cfg.BaseURL = server.URL
	p, err := New(cfg)
	require.NoError(t, err)

	_, err = p.GenerateEmbeddings(context.Background(), []string{"hello"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode response")
}

func TestGenerateEmbeddings_EmptyData(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"data":  []interface{}{},
			"usage": map[string]int{"prompt_tokens": 0, "total_tokens": 0},
		}
		data, _ := json.Marshal(resp)
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	cfg := validTestConfig()
	cfg.BaseURL = server.URL
	p, err := New(cfg)
	require.NoError(t, err)

	// Empty data produces an empty result slice without error.
	results, err := p.GenerateEmbeddings(context.Background(), []string{"hello"})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestGenerateEmbeddings_RequestCreationError(t *testing.T) {
	t.Parallel()

	// An invalid URL causes http.NewRequestWithContext to fail.
	cfg := validTestConfig()
	cfg.BaseURL = "http://%zz"
	p, err := New(cfg)
	require.NoError(t, err)

	_, err = p.GenerateEmbeddings(context.Background(), []string{"hello"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create request")
}

func TestGenerateEmbeddings_ConnectionError(t *testing.T) {
	t.Parallel()

	// Closing the server forces a connection error on the client.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	serverURL := server.URL
	server.Close()

	cfg := validTestConfig()
	cfg.BaseURL = serverURL
	p, err := New(cfg)
	require.NoError(t, err)

	_, err = p.GenerateEmbeddings(context.Background(), []string{"hello"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http request")
}

func TestGenerateEmbeddings_RateLimitWait(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(deepseekSuccessJSON())
	}))
	defer server.Close()

	cfg := validTestConfig()
	cfg.BaseURL = server.URL
	p, err := New(cfg)
	require.NoError(t, err)
	// Override with a tiny bucket (1 token/min) to force a wait on the second call.
	p.rateLimiter = providers.NewRateLimiter(1)

	// First call consumes the only available token.
	_, err = p.GenerateEmbeddings(context.Background(), []string{"hello"})
	require.NoError(t, err)

	// Second call must wait for a refill; the short-lived context cancels it.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err = p.GenerateEmbeddings(ctx, []string{"hello"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit wait")
}

// ── GenerateEmbedding (single → delegates to GenerateEmbeddings) ──────────────

func TestGenerateEmbedding_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(deepseekSuccessJSON())
	}))
	defer server.Close()

	cfg := validTestConfig()
	cfg.BaseURL = server.URL
	p, err := New(cfg)
	require.NoError(t, err)

	result, err := p.GenerateEmbedding(context.Background(), "hello")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, []float64{0.1, 0.2, 0.3}, result.Vector)
	assert.Equal(t, defaultModel, result.Model)
}

func TestGenerateEmbedding_NoResults(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"data":  []interface{}{},
			"usage": map[string]int{"prompt_tokens": 0, "total_tokens": 0},
		}
		data, _ := json.Marshal(resp)
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	cfg := validTestConfig()
	cfg.BaseURL = server.URL
	p, err := New(cfg)
	require.NoError(t, err)

	_, err = p.GenerateEmbedding(context.Background(), "hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no embedding result")
}

func TestGenerateEmbedding_PropagatesBatchError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad input"}`))
	}))
	defer server.Close()

	cfg := validTestConfig()
	cfg.BaseURL = server.URL
	p, err := New(cfg)
	require.NoError(t, err)

	_, err = p.GenerateEmbedding(context.Background(), "hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API error 400")
}

// ── Register ──────────────────────────────────────────────────────────────────

func TestRegister(t *testing.T) {
	// NOTE: sequential (no t.Parallel) because it touches the global registry
	// and uses t.Setenv.

	embeddings.ResetRegistry()
	t.Setenv("DEEPSEEK_API_KEY", "register-key")

	Register()

	registry := embeddings.GetRegistry()
	names := registry.List()
	assert.Contains(t, names, "deepseek")

	provider, ok := registry.Get("deepseek")
	require.True(t, ok, "deepseek provider should be retrievable from registry")
	require.NotNil(t, provider)
	assert.Equal(t, "deepseek", provider.Name())
	assert.Equal(t, defaultModel, provider.Model())
}
