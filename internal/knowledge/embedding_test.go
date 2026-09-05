package knowledge

import (
	"context"
	"os"
	"testing"

	"github.com/CoscaAI/cosca/internal/embeddings"
)

// testEmbeddingProvider is deliberately local and deterministic.  Knowledge
// engine tests exercise indexing/search and must not depend on an installed
// provider, credentials, or network access.
type testEmbeddingProvider struct{}

func (testEmbeddingProvider) GenerateEmbedding(_ context.Context, _ string) (*embeddings.EmbeddingResult, error) {
	vector := make([]float64, 768)
	vector[0] = 1
	return &embeddings.EmbeddingResult{
		Vector:     vector,
		Model:      "test-deterministic",
		Dimensions: len(vector),
	}, nil
}

func (p testEmbeddingProvider) GenerateEmbeddings(ctx context.Context, texts []string) ([]*embeddings.EmbeddingResult, error) {
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

func (testEmbeddingProvider) Model() string   { return "test-deterministic" }
func (testEmbeddingProvider) Dimensions() int { return 768 }
func (testEmbeddingProvider) Name() string    { return "local" }
func (testEmbeddingProvider) Close() error    { return nil }

func TestMain(m *testing.M) {
	// Engine.Init selects "local" as the final fallback. Registering it here
	// keeps every fixture explicit while leaving production provider selection
	// and validation unchanged.
	embeddings.ResetRegistry()
	embeddings.GetRegistry().Register("local", func(context.Context, *embeddings.Config) (embeddings.Provider, error) {
		return testEmbeddingProvider{}, nil
	}, "deterministic test provider", 1)
	os.Exit(m.Run())
}
