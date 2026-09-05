// Package embeddings — Fase 1 (L376): fail-closed no provider + pinagem do
// digest. Identidade de embeddings é parte da integridade do índice.
package embeddings

import (
	"context"
	"errors"
	"testing"
)

func TestFailClosedNoLocalFallback(t *testing.T) {
	reg := GetRegistry()
	reg.Register("fake-explicit", func(_ context.Context, _ *Config) (Provider, error) {
		return nil, errors.New("provider explícito indisponível")
	}, "fake", 50)

	// Primary explícito + provider falha → Select FALHA (não cai no local).
	err := reg.Select(context.Background(), ProviderRegistryConfig{
		Primary:    "fake-explicit",
		AutoDetect: false,
	})
	if err == nil {
		t.Fatal("fail-closed: esperava ERRO, mas o Select passou (caiu no local?)")
	}
}

func TestDigestPassedToProvider(t *testing.T) {
	reg := GetRegistry()
	var got *Config
	reg.Register("fake-digest", func(_ context.Context, cfg *Config) (Provider, error) {
		got = cfg
		return nil, errors.New("stop")
	}, "fake", 50)
	_ = reg.Select(context.Background(), ProviderRegistryConfig{
		Primary: "fake-digest",
		Digest:  "abc123",
	})
	if got == nil || got.Digest != "abc123" {
		t.Fatalf("digest não propagado: %+v", got)
	}
}

type okProvider struct{}

func (okProvider) GenerateEmbedding(context.Context, string) (*EmbeddingResult, error) {
	return &EmbeddingResult{Vector: []float64{1, 0, 0}, Dimensions: 3}, nil
}
func (okProvider) GenerateEmbeddings(context.Context, []string) ([]*EmbeddingResult, error) {
	return nil, nil
}
func (okProvider) Name() string    { return "ok-fake" }
func (okProvider) Model() string   { return "ok-model" }
func (okProvider) Dimensions() int { return 3 }
func (okProvider) Close() error    { return nil }

type failUseProvider struct{}

func (failUseProvider) GenerateEmbedding(context.Context, string) (*EmbeddingResult, error) {
	return nil, errors.New("provider explícito fora no uso")
}
func (failUseProvider) GenerateEmbeddings(context.Context, []string) ([]*EmbeddingResult, error) {
	return nil, errors.New("provider explícito fora no uso")
}
func (failUseProvider) Name() string    { return "fail-use" }
func (failUseProvider) Model() string   { return "m" }
func (failUseProvider) Dimensions() int { return 3 }
func (failUseProvider) Close() error    { return nil }

func TestFailClosedInUsage(t *testing.T) {
	// Primary EXPLÍCITO + falha NO USO → o erro é retornado (fallbacks vazios,
	// sem cair no local silenciosamente — L376).
	reg := GetRegistry()
	reg.Register("fail-use", func(_ context.Context, _ *Config) (Provider, error) {
		return failUseProvider{}, nil
	}, "fake", 60)
	if err := reg.Select(context.Background(), ProviderRegistryConfig{
		Primary:    "fail-use",
		AutoDetect: false,
	}); err != nil {
		t.Fatalf("select: %v", err)
	}
	_, err := reg.GenerateEmbedding(context.Background(), "teste")
	if err == nil {
		t.Fatal("fail-closed no uso: esperava ERRO, mas gerou (caiu no fallback?)")
	}
	t.Logf("fail-closed no uso OK: %v", err)
}

func TestAutoDetectWithoutPrimaryStillWorks(t *testing.T) {
	// Sem Primary explícito, o auto-detect permanece funcional (o fail-closed
	// só vale para provider EXPLÍCITO — L376).
	reg := GetRegistry()
	reg.Register("ok-fake", func(_ context.Context, _ *Config) (Provider, error) {
		return okProvider{}, nil
	}, "fake ok", 90)
	err := reg.Select(context.Background(), ProviderRegistryConfig{
		AutoDetect: true,
	})
	if err != nil {
		t.Fatalf("auto-detect deveria funcionar: %v", err)
	}
}
