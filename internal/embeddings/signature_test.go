// Package embeddings tests — C2/invariante de espaço único de embedding:
// assinatura canônica "model:dim" (Signature) e o filtro de quarentena
// (EmbeddingSpaceInvariant). Puramente aditivo e determinístico.
package embeddings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSignature_CanonicalFormat: Signature(model, dim) devolve "model:dim".
func TestSignature_CanonicalFormat(t *testing.T) {
	assert.Equal(t, "nomic-embed-text:768", Signature("nomic-embed-text", "768"))
	assert.Equal(t, "model-a:1024", Signature("model-a", "1024"))
}

// TestSignature_EmptyReturnsMode: model ou dim ausente → "" (modo atual, sem
// invariante). Espaços em branco também caem em vazio.
func TestSignature_EmptyReturnsMode(t *testing.T) {
	assert.Equal(t, "", Signature("", "768"))
	assert.Equal(t, "", Signature("nomic", ""))
	assert.Equal(t, "", Signature("", ""))
	assert.Equal(t, "", Signature("nomic", "  "))
	assert.Equal(t, "", Signature("   ", "768"))
}

// TestSignature_TrimsInput: a assinatura normaliza espaços internos/externos de
// cada componente.
func TestSignature_TrimsInput(t *testing.T) {
	assert.Equal(t, "nomic:768", Signature("  nomic  ", " 768 "))
	assert.Equal(t, "big:1536", Signature("big", "1536"))
}

// TestSignatureInt: variante inteira converte via Signature(model, strconv).
func TestSignatureInt(t *testing.T) {
	assert.Equal(t, "nomic:768", SignatureInt("nomic", 768))
	assert.Equal(t, "m:12", SignatureInt("m", 12))
	// dim <= 0 → "" (sem invariante).
	assert.Equal(t, "", SignatureInt("nomic", 0))
	assert.Equal(t, "", SignatureInt("nomic", -5))
}

// TestSignature_GetArtifactSignature: contrato do ADR §3.4 — variante int e
// string convergem ao mesmo formato canônico.
func TestSignature_GetArtifactSignature(t *testing.T) {
	assert.Equal(t, "nomic:768", GetArtifactSignature("nomic", 768))
	assert.Equal(t, Signature("nomic", "768"), GetArtifactSignature("nomic", 768))
	assert.Equal(t, "", GetArtifactSignature("nomic", 0))
}

// TestEmbeddingSpaceInvariant_SameAllowsReuse: assinatura persistida == ativa
// → reuso permitido.
func TestEmbeddingSpaceInvariant_SameAllowsReuse(t *testing.T) {
	assert.True(t, EmbeddingSpaceInvariant("nomic:768", "nomic:768"))
	assert.True(t, EmbeddingSpaceInvariant("a:b", "a:b"))
}

// TestEmbeddingSpaceInvariant_DivergenceQuarantines: assinaturas diferentes →
// mismatch → reuso vetado (quarentena / re-embed).
func TestEmbeddingSpaceInvariant_DivergenceQuarantines(t *testing.T) {
	assert.False(t, EmbeddingSpaceInvariant("nomic:768", "nomic:1024"))
	assert.False(t, EmbeddingSpaceInvariant("nomic:768", "other:768"))
	assert.False(t, EmbeddingSpaceInvariant("a:b", "a:c"))
}

// TestEmbeddingSpaceInvariant_EmptyMode: uma das duas vazia (ou ambas) → "sem
// invariante" → reuso permitido (comportamento atual).
func TestEmbeddingSpaceInvariant_EmptyMode(t *testing.T) {
	assert.True(t, EmbeddingSpaceInvariant("", ""))
	assert.True(t, EmbeddingSpaceInvariant("nomic:768", ""))
	assert.True(t, EmbeddingSpaceInvariant("", "nomic:768"))
	assert.True(t, EmbeddingSpaceInvariant("   ", "  "))
	assert.True(t, EmbeddingSpaceInvariant("nomic:768", "   "))
}

// TestEmbeddingSpaceInvariant_TrimsBeforeCompare: a comparação ignora espaços
// nas bordas (normalização antes do match).
func TestEmbeddingSpaceInvariant_TrimsBeforeCompare(t *testing.T) {
	assert.True(t, EmbeddingSpaceInvariant(" nomic:768 ", "nomic:768"))
	assert.False(t, EmbeddingSpaceInvariant(" nomic:768 ", "nomic:1024"))
}

// TestFromProvider: deriva a assinatura do provider ativo (Model + Dimensions).
func TestFromProvider(t *testing.T) {
	p := &mockProvider{name: "openai", model: "text-embedding-3", dimensions: 1536}
	assert.Equal(t, "text-embedding-3:1536", FromProvider(p))
	assert.Equal(t, "", FromProvider(nil))
}
