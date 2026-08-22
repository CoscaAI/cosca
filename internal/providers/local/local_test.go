package local

import (
	"context"
	"testing"
)

func TestNew(t *testing.T) {
	p, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if p.Name() != "local" {
		t.Errorf("Name = %q", p.Name())
	}
	if p.Dimensions() != 128 {
		t.Errorf("Dimensions = %d", p.Dimensions())
	}
}

func TestGenerateEmbedding(t *testing.T) {
	p, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	result, err := p.GenerateEmbedding(context.Background(), "test authentication flow")
	if err != nil {
		t.Fatalf("GenerateEmbedding error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if len(result.Vector) != 128 {
		t.Errorf("Vector length = %d", len(result.Vector))
	}
	if result.Dimensions != 128 {
		t.Errorf("Dimensions = %d", result.Dimensions)
	}
	if result.Model == "" {
		t.Error("Model is empty")
	}
}

func TestGenerateEmbeddings(t *testing.T) {
	p, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	results, err := p.GenerateEmbeddings(context.Background(), []string{"hello", "world", ""})
	if err != nil {
		t.Fatalf("GenerateEmbeddings error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	if results[2] == nil {
		t.Fatal("empty text result is nil")
	}
}

func TestGenerateEmbeddingDeterministic(t *testing.T) {
	p, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	first, err := p.GenerateEmbedding(context.Background(), "deterministic text")
	if err != nil {
		t.Fatalf("first embedding error: %v", err)
	}
	// Unrelated calls must not change the hash assignment.
	if _, err := p.GenerateEmbedding(context.Background(), "another text"); err != nil {
		t.Fatalf("unrelated embedding error: %v", err)
	}
	second, err := p.GenerateEmbedding(context.Background(), "deterministic text")
	if err != nil {
		t.Fatalf("second embedding error: %v", err)
	}
	for i := range first.Vector {
		if first.Vector[i] != second.Vector[i] {
			t.Fatalf("vector changed at index %d: %v != %v", i, first.Vector[i], second.Vector[i])
		}
	}
}

func TestFitAndReset(t *testing.T) {
	p, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	err = p.Fit([]string{"document one about testing", "document two about authentication"})
	if err != nil {
		t.Fatalf("Fit error: %v", err)
	}

	p.Reset()
	// After reset, embeddings should still work
	_, err = p.GenerateEmbedding(context.Background(), "test")
	if err != nil {
		t.Fatalf("GenerateEmbedding after reset error: %v", err)
	}
}

func TestEmptyText(t *testing.T) {
	p, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	result, err := p.GenerateEmbedding(context.Background(), "")
	if err != nil {
		t.Fatalf("GenerateEmbedding empty text error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	// Empty text should return zero vector
	zero := true
	for _, v := range result.Vector {
		if v != 0 {
			zero = false
			break
		}
	}
	if !zero {
		t.Error("expected zero vector for empty text")
	}
}

func TestClose(t *testing.T) {
	p, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Errorf("Close error: %v", err)
	}
}

func TestFactory(t *testing.T) {
	name, factory := Factory()
	if name != "local" {
		t.Errorf("name = %q", name)
	}
	provider, err := factory(context.Background(), nil)
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	if provider.Name() != "local" {
		t.Errorf("provider.Name() = %q", provider.Name())
	}
}
