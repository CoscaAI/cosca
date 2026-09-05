package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CoscaAI/cosca/internal/embeddings"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.BaseURL != "http://localhost:11434" {
		t.Fatalf("base url = %q", cfg.BaseURL)
	}
	if cfg.Model != ModelNomicEmbed {
		t.Fatalf("model = %q", cfg.Model)
	}
	if cfg.Timeout <= 0 || cfg.MaxRetries <= 0 || cfg.BatchSize <= 0 {
		t.Fatalf("invalid defaults: %+v", cfg)
	}
}

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "")
	if cfg := ConfigFromEnv(); cfg.BaseURL != "http://localhost:11434" {
		t.Fatalf("no env: %q", cfg.BaseURL)
	}

	t.Setenv("OLLAMA_HOST", "127.0.0.1:11435")
	if cfg := ConfigFromEnv(); cfg.BaseURL != "http://127.0.0.1:11435" {
		t.Fatalf("scheme added: %q", cfg.BaseURL)
	}

	t.Setenv("OLLAMA_HOST", "http://ollama:11434")
	if cfg := ConfigFromEnv(); cfg.BaseURL != "http://ollama:11434" {
		t.Fatalf("scheme preserved: %q", cfg.BaseURL)
	}
}

func TestNewDefaults(t *testing.T) {
	p, err := New(Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Name() != "ollama" || p.Model() != ModelNomicEmbed {
		t.Fatalf("provider: %+v", p)
	}
	if p.Dimensions() != 768 {
		t.Fatalf("dims = %d", p.Dimensions())
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestNewNormalizesBaseURL(t *testing.T) {
	p, _ := New(Config{BaseURL: "http://host:11434/v1/"})
	if p.cfg.BaseURL != "http://host:11434" {
		t.Fatalf("normalized = %q", p.cfg.BaseURL)
	}
}

func TestGenerateEmbeddings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			t.Errorf("path = %q, want /api/embed", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"embeddings": [][]float64{{0.1, 0.2, 0.3}, {0.4, 0.5, 0.6}},
		})
	}))
	defer srv.Close()

	p, err := New(Config{BaseURL: srv.URL, MaxRetries: 1, BatchSize: 1})
	if err != nil {
		t.Fatal(err)
	}

	results, err := p.GenerateEmbeddings(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("GenerateEmbeddings: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d", len(results))
	}
	for _, r := range results {
		if len(r.Vector) != 3 || r.Dimensions != 3 {
			t.Fatalf("result: %+v", r)
		}
	}
	// Dimension auto-detected from response.
	if p.Dimensions() != 3 {
		t.Fatalf("detected dims = %d", p.Dimensions())
	}

	// Empty input → error.
	if _, err := p.GenerateEmbeddings(context.Background(), nil); err == nil {
		t.Fatal("empty texts must error")
	}
}

func TestGenerateEmbeddingsServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	p, _ := New(Config{BaseURL: srv.URL, MaxRetries: 0, BatchSize: 1})
	_, err := p.GenerateEmbeddings(context.Background(), []string{"x"})
	if err == nil {
		t.Fatal("server error must propagate")
	}
}

func TestGenerateEmbeddingsNetworkError(t *testing.T) {
	// Port that refuses connections: 127.0.0.1:1.
	p, _ := New(Config{BaseURL: "http://127.0.0.1:1", MaxRetries: 0, BatchSize: 1})
	_, err := p.GenerateEmbeddings(context.Background(), []string{"x"})
	if err == nil {
		t.Fatal("network error must propagate")
	}
}

var _ embeddings.Provider = (*Provider)(nil)
