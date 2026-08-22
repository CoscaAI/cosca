package handler

import (
	"context"
	"os"
	"testing"

	"github.com/CoscaAI/cosca/internal/embeddings"
)

// handlerTestEmbeddingProvider is deterministic and local so handler tests
// that initialize the knowledge engine never need credentials or network I/O.
type handlerTestEmbeddingProvider struct{}

func (handlerTestEmbeddingProvider) GenerateEmbedding(context.Context, string) (*embeddings.EmbeddingResult, error) {
	vector := make([]float64, 768)
	vector[0] = 1
	return &embeddings.EmbeddingResult{
		Vector:     vector,
		Model:      "handler-test-deterministic",
		Dimensions: len(vector),
	}, nil
}

func (p handlerTestEmbeddingProvider) GenerateEmbeddings(ctx context.Context, texts []string) ([]*embeddings.EmbeddingResult, error) {
	results := make([]*embeddings.EmbeddingResult, len(texts))
	for i, text := range texts {
		result, err := p.GenerateEmbedding(ctx, text)
		if err != nil {
			return nil, err
		}
		results[i] = result
	}
	return results, nil
}

func (handlerTestEmbeddingProvider) Model() string   { return "handler-test-deterministic" }
func (handlerTestEmbeddingProvider) Dimensions() int { return 768 }
func (handlerTestEmbeddingProvider) Name() string    { return "local" }
func (handlerTestEmbeddingProvider) Close() error    { return nil }

var _ embeddings.Provider = handlerTestEmbeddingProvider{}

func TestMain(m *testing.M) {
	// The knowledge engine's "auto" selection always considers local as its
	// final fallback. Register it explicitly for this package's test binary.
	embeddings.ResetRegistry()
	embeddings.GetRegistry().Register("local", func(context.Context, *embeddings.Config) (embeddings.Provider, error) {
		return handlerTestEmbeddingProvider{}, nil
	}, "deterministic handler test provider", 1)
	os.Exit(m.Run())
}
