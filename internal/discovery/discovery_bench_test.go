package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func BenchmarkDiscoveryEngineCreation(b *testing.B) {
	for i := 0; b.Loop(); i++ {
		_ = NewEngine()
	}
}

func BenchmarkDiscoveryEngineCreationWithOptions(b *testing.B) {
	for i := 0; b.Loop(); i++ {
		_ = NewEngine(
			WithWorkDir("/tmp"),
			WithLogger(zerolog.Nop()),
			WithCacheTTL(60*time.Second),
		)
	}
}

func BenchmarkDiscoverProject(b *testing.B) {
	e := NewEngine(WithWorkDir("."))
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_, err := e.DiscoverProject(ctx)
		if err != nil {
			// Project discovery may fail in some environments; not fatal for benchmark
			_ = err
		}
	}
}

func BenchmarkDiscoverWorkspace(b *testing.B) {
	e := NewEngine(WithWorkDir("."))
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_, err := e.DiscoverWorkspace(ctx)
		if err != nil {
			_ = err
		}
	}
}

func BenchmarkDiscoverRuntime(b *testing.B) {
	e := NewEngine()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_, err := e.DiscoverRuntime(ctx)
		if err != nil {
			_ = err
		}
	}
}

func BenchmarkDiscoverEnvironment(b *testing.B) {
	e := NewEngine()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_, err := e.DiscoverEnvironment(ctx)
		if err != nil {
			_ = err
		}
	}
}
