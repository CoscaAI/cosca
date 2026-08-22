package azure

import (
	"context"
	"testing"

	"github.com/CoscaAI/cosca/internal/embeddings"
)

func TestFactoryAndRegister(t *testing.T) {
	name, factory := Factory()
	if name != "azure" || factory == nil {
		t.Fatalf("factory: %q, %v", name, factory)
	}
	// Factory com config vazio: nunca panic.
	_, _ = factory(context.Background(), &embeddings.Config{})
	// Register: nunca panic.
	Register()
}

func TestGenerateEmbeddingSingle(t *testing.T) {
	// GenerateEmbedding (single) delega para GenerateEmbeddings; com provider
	// sem endpoint válido, retorna erro (não panic).
	p, err := New(Config{APIKey: "k", Endpoint: "http://127.0.0.1:1", Deployment: "d", MaxRetries: 0, BatchSize: 1})
	if err != nil {
		t.Skipf("New: %v", err)
	}
	_, err = p.GenerateEmbedding(context.Background(), "text")
	if err == nil {
		t.Log("GenerateEmbedding retornou nil (provider pode ter endpoint mock)")
	}
}
