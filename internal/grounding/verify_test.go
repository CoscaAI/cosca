// Package grounding tests — VerifyClaim (token-overlap + bônus de n-grama e
// número). Determinístico e zero-LLM.
package grounding

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ch é um helper compacto para montar um SourceChunk com conteúdo (o conteúdo
// é necessário para o token-overlap — Content vazio não é verificável).
func ch(id, content string) SourceChunk {
	return SourceChunk{ChunkID: id, Content: content}
}

// TestVerifyClaim_FullMatchSupported verifica o caso feliz: claim idêntico ao
// chunk → cobertura total + bônus de bigrama e número → supported.
func TestVerifyClaim_FullMatchSupported(t *testing.T) {
	t.Parallel()

	claim := Claim{Text: "O homem tem 42 anos"}
	chunks := []SourceChunk{ch("c1", "O homem tem 42 anos")}

	verdict := VerifyClaim("", claim, chunks, VerifyOptions{})

	assert.True(t, verdict.Supported)
	assert.Equal(t, 0, verdict.BestChunkIdx)
	assert.InDelta(t, 1.25, verdict.Score, 1e-4) // 1.0 base + 0.15 bigram + 0.10 number
	assert.True(t, verdict.Claim.Substantiated)
	assert.NotEmpty(t, verdict.MatchingTerms)
}

// TestVerifyClaim_NoOverlapNotSupported: sem interseção de tokens → sem
// melhor chunk, score 0, não suportado.
func TestVerifyClaim_NoOverlapNotSupported(t *testing.T) {
	t.Parallel()

	claim := Claim{Text: "quantum entanglement in black holes"}
	chunks := []SourceChunk{ch("c1", "O café brasileiro é vendido em todos os mercados do interior")}

	verdict := VerifyClaim("", claim, chunks, VerifyOptions{})

	assert.False(t, verdict.Supported)
	assert.Equal(t, -1, verdict.BestChunkIdx)
	assert.Equal(t, 0.0, verdict.Score)
	assert.Empty(t, verdict.MatchingTerms)
	assert.False(t, verdict.Claim.Substantiated)
}

// TestVerifyClaim_NumberBonusRaiseScore: um número do claim presente apenas no
// chunk eleva o score acima da base de token-overlap pura.
func TestVerifyClaim_NumberBonusRaiseScore(t *testing.T) {
	t.Parallel()

	claim := Claim{Text: "o valor e 42"}
	chunks := []SourceChunk{ch("n1", "42")}

	verdict := VerifyClaim("", claim, chunks, VerifyOptions{})

	// base 1/4 = 0.25; number bonus = +0.10 → 0.35. Ainda abaixo do threshold,
	// mas acima da base pura (demonstra que o número contribui).
	assert.InDelta(t, 0.35, verdict.Score, 1e-4)
	assert.Greater(t, verdict.Score, 0.25)
	assert.False(t, verdict.Supported, "0.35 < 0.40 threshold")
	assert.Equal(t, 0, verdict.BestChunkIdx, "número casa com o chunk → best=0")

	// Número NOVO no claim deve cair no numericTokens do claim.
	assert.Contains(t, verdict.MatchingTerms, "42")
}

// TestVerifyClaim_NGramBonusAddsToScore provando o bônus de bigrama: dois
// chunks com a MESMA cobertura de tokens, mas um com o bigrama do claim →
// maior score e melhor chunk.
func TestVerifyClaim_NGramBonusAddsToScore(t *testing.T) {
	t.Parallel()

	claim := Claim{Text: "nome completo"}
	chunks := []SourceChunk{
		ch("a", "nome teste completo"), // ambas as palavras presentes, bigrama quebrado
		ch("b", "nome completo"),       // bigrama do claim presente
	}

	verdict := VerifyClaim("", claim, chunks, VerifyOptions{})

	require.GreaterOrEqual(t, len(chunks), 2)
	assert.Equal(t, 1, verdict.BestChunkIdx, "chunk B (bigrama correto) deve ganhar")
	assert.InDelta(t, 1.15, verdict.Score, 1e-4)
	assert.True(t, verdict.Supported)
}

// TestVerifyClaim_BestChunkIdxQuandoUmCasaMelhor: lista com um chunk parcial e
// um total; o total (index 1) deve ser o melhor.
func TestVerifyClaim_BestChunkIdxQuandoUmCasaMelhor(t *testing.T) {
	t.Parallel()

	claim := Claim{Text: "x y z"}
	chunks := []SourceChunk{
		ch("parcial", "x y"),
		ch("total", "x y z"),
	}

	verdict := VerifyClaim("", claim, chunks, VerifyOptions{})

	assert.Equal(t, 1, verdict.BestChunkIdx)
	assert.InDelta(t, 1.15, verdict.Score, 1e-4)
	assert.True(t, verdict.Supported)
}

// TestVerifyClaim_EmptyClaim: claim sem tokens (texto vazio/garbage) nunca é
// suportado e BestChunkIdx = -1.
func TestVerifyClaim_EmptyClaim(t *testing.T) {
	t.Parallel()

	for _, text := range []string{"", "!!!", "   "} {
		verdict := VerifyClaim("", Claim{Text: text}, []SourceChunk{ch("c", "qualquer conteúdo real aqui")}, VerifyOptions{})
		assert.False(t, verdict.Supported, "text=%q", text)
		assert.Equal(t, -1, verdict.BestChunkIdx, "text=%q", text)
		assert.Equal(t, 0.0, verdict.Score, "text=%q", text)
	}
}

// TestVerifyClaim_CustomThreshold: o threshold de suporte é configurável via
// opts; um score alto com threshold alto NÃO é suportado.
func TestVerifyClaim_CustomThreshold(t *testing.T) {
	t.Parallel()

	claim := Claim{Text: "a b c"}
	chunks := []SourceChunk{ch("c1", "a b c")} // score = 1.15 (sem número)

	strict := VerifyClaim("", claim, chunks, VerifyOptions{Threshold: 1.5})
	assert.False(t, strict.Supported, "score 1.15 < threshold 1.5 → not supported")

	loose := VerifyClaim("", claim, chunks, VerifyOptions{Threshold: 0.1})
	assert.True(t, loose.Supported, "score 1.15 >= 0.1 → supported")
}

// TestVerifyClaim_EmptyChunkContent: chunks com Content vazio são ignorados no
// ranking (não entram no best), mantendo BestChunkIdx = -1.
func TestVerifyClaim_EmptyChunkContent(t *testing.T) {
	t.Parallel()

	claim := Claim{Text: "um texto qualquer de teste"}
	chunks := []SourceChunk{
		{ChunkID: "vazio1"},   // sem Content → tokenize len 0 → skip
		{ChunkID: "vazio2"},   // sem Content → skip
	}

	verdict := VerifyClaim("", claim, chunks, VerifyOptions{})
	assert.Equal(t, -1, verdict.BestChunkIdx)
	assert.Equal(t, 0.0, verdict.Score)
	assert.False(t, verdict.Supported)
}

// TestVerifyClaim_Deterministic confirma que a mesma entrada produz o mesmo
// veredito (sem estado, sem LLM, sem tempo).
func TestVerifyClaim_Deterministic(t *testing.T) {
	t.Parallel()

	claim := Claim{Text: "O RAG usa chunks recuperados"}
	chunks := []SourceChunk{ch("c1", "RAG usa chunks recuperados como evidência")}

	a := VerifyClaim("", claim, chunks, VerifyOptions{})
	b := VerifyClaim("", claim, chunks, VerifyOptions{})
	assert.Equal(t, a, b)
}
