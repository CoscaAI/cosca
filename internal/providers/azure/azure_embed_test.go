package azure

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAzureGenerateEmbeddings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"embedding": []float64{0.1, 0.2}, "index": 0},
				{"embedding": []float64{0.3, 0.4}, "index": 1},
			},
			"model": "text-embedding-3-small",
		})
	}))
	defer srv.Close()

	p, err := New(Config{
		APIKey:     "key",
		Endpoint:   srv.URL,
		Deployment: "embed",
		BatchSize:  1,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	results, err := p.GenerateEmbeddings(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("GenerateEmbeddings: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d", len(results))
	}
	// The provider pads/normalizes vectors to its configured dimension (512),
	// preserving the leading values from the API response.
	if len(results[0].Vector) < 2 || results[0].Vector[0] != 0.1 || results[0].Vector[1] != 0.2 {
		t.Fatalf("vector head: %+v", results[0].Vector[:2])
	}
	if p.Dimensions() <= 0 {
		t.Fatalf("dims = %d", p.Dimensions())
	}

	// Empty input → error.
	if _, err := p.GenerateEmbeddings(context.Background(), nil); err == nil {
		t.Fatal("empty texts must error")
	}
}

func TestAzureGenerateEmbeddingsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	p, _ := New(Config{APIKey: "k", Endpoint: srv.URL, Deployment: "d", MaxRetries: 0})
	_, err := p.GenerateEmbeddings(context.Background(), []string{"x"})
	if err == nil {
		t.Fatal("server error must propagate")
	}
}
