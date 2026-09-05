package search

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// FASE 5.2 — meaning-first: no espaço roteado (modular + vetor), a ordenação é
// guiada pelo SIGNIFICADO (score do vetor/cosine), com FTS como recall suplementar.

func TestMeaningFirstRank_VectorLeads(t *testing.T) {
	results := []SearchResult{
		{ID: "fts-2", Source: "fts", Score: 0.9},     // BM25 alto, mas LEXICAL
		{ID: "vec-1", Source: "vector", Score: 0.75}, // cosine (significado)
		{ID: "vec-2", Source: "vector", Score: 0.85}, // cosine (significado) melhor
		{ID: "fts-1", Source: "fts", Score: 0.5},
	}

	out := meaningFirstRank(results)

	// Vetores (significado) à frente, por cosine desc — mesmo que FTS tenha BM25 maior.
	assert.Equal(t, "vec-2", out[0].ID, "vetor com maior cosine lidera")
	assert.Equal(t, "vec-1", out[1].ID, "vetor com menor cosine em seguida")
	// FTS entra depois (recall), por score desc.
	assert.Equal(t, "fts-2", out[2].ID)
	assert.Equal(t, "fts-1", out[3].ID)
}

func TestMeaningFirstRank_SingleSourceUnchanged(t *testing.T) {
	// Uma única fonte (só vetor) mantém a ordem por cosine — sem surpresas.
	results := []SearchResult{
		{ID: "a", Source: "vector", Score: 0.4},
		{ID: "b", Source: "vector", Score: 0.9},
		{ID: "c", Source: "vector", Score: 0.6},
	}
	out := meaningFirstRank(results)
	assert.Equal(t, "b", out[0].ID)
	assert.Equal(t, "c", out[1].ID)
	assert.Equal(t, "a", out[2].ID)
}

func TestMeaningFirstRank_StableAmongFTS(t *testing.T) {
	// Ordem relativa dos FTS preservada quando scores iguais (estável).
	results := []SearchResult{
		{ID: "vec", Source: "vector", Score: 0.5},
		{ID: "x", Source: "fts", Score: 0.7},
		{ID: "y", Source: "fts", Score: 0.7},
	}
	out := meaningFirstRank(results)
	assert.Equal(t, "vec", out[0].ID)
	assert.Equal(t, "x", out[1].ID)
	assert.Equal(t, "y", out[2].ID)
}
