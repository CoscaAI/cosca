package memory

import (
	"context"
	"testing"
)

func BenchmarkMemoryStore(b *testing.B) {
	e, err := NewEngine(WithConfig(EngineConfig{DataDir: b.TempDir()}))
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = e.Close() }()

	ctx := context.Background()
	record := MemoryRecord{
		Type:    MemoryTypeDecision,
		Layer:   LayerProject,
		Content: "test content for store benchmarking",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := e.Store(ctx, record)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemorySearch(b *testing.B) {
	e, err := NewEngine(WithConfig(EngineConfig{DataDir: b.TempDir()}))
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = e.Close() }()

	ctx := context.Background()
	// Add test data
	for i := 0; i < 100; i++ {
		_, err := e.Store(ctx, MemoryRecord{
			Type:    MemoryTypeDecision,
			Layer:   LayerProject,
			Content: "test content for search benchmarking iteration",
		})
		if err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := e.Search(ctx, "test", SearchOptions{Limit: 20})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemoryEngineCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		e, err := NewEngine(WithConfig(EngineConfig{DataDir: b.TempDir()}))
		if err != nil {
			b.Fatal(err)
		}
		_ = e.Close()
	}
}

func BenchmarkMemoryRetrieve(b *testing.B) {
	e, err := NewEngine(WithConfig(EngineConfig{DataDir: b.TempDir()}))
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = e.Close() }()

	ctx := context.Background()
	saved, err := e.Store(ctx, MemoryRecord{
		Type:    MemoryTypeDecision,
		Layer:   LayerProject,
		Content: "test content for retrieve benchmarking",
	})
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := e.Retrieve(ctx, saved.ID, LayerProject)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemoryDelete(b *testing.B) {
	e, err := NewEngine(WithConfig(EngineConfig{DataDir: b.TempDir()}))
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = e.Close() }()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		saved, err := e.Store(ctx, MemoryRecord{
			Type:    MemoryTypeDecision,
			Layer:   LayerProject,
			Content: "test content for delete benchmarking",
		})
		if err != nil {
			b.Fatal(err)
		}
		_, _ = e.Promote(ctx, saved.ID, LayerProject, LayerGlobal)
	}
}
