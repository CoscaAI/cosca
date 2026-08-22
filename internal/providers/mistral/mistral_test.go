package mistral

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
	if p.Name() != "mistral" {
		t.Fatalf("name = %q", p.Name())
	}
}

func TestFactory(t *testing.T) {
	name, factory := Factory()
	if name != "mistral" {
		t.Fatalf("name = %q", name)
	}
	if factory == nil {
		t.Fatal("nil factory")
	}

	t.Setenv("MISTRAL_API_KEY", "test-key")
	p, err := factory(context.Background(), &embeddings.Config{Model: "mistral-embed"})
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	if p.Model() != "mistral-embed" {
		t.Fatalf("model = %q", p.Model())
	}
}
