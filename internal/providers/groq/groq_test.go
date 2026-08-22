package groq

import (
	"context"
	"testing"

	"github.com/CoscaAI/cosca/internal/embeddings"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Model == "" {
		t.Fatal("default model empty")
	}
}

func TestNew(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIKey = "test-key"
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Name() != "groq" {
		t.Fatalf("name = %q", p.Name())
	}
}

func TestFactory(t *testing.T) {
	name, factory := Factory()
	if name != "groq" {
		t.Fatalf("name = %q", name)
	}
	if factory == nil {
		t.Fatal("nil factory")
	}

	t.Setenv("GROQ_API_KEY", "test-key")
	p, err := factory(context.Background(), &embeddings.Config{Model: "embed-2", Dimensions: 128})
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	if p.Model() != "embed-2" {
		t.Fatalf("model = %q", p.Model())
	}
	if p.Dimensions() <= 0 {
		t.Fatalf("dims = %d", p.Dimensions())
	}
}
