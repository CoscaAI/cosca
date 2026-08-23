// Package grounding tests — fatia de fidelidade anti-alucinação do RAG.
//
// Estes testes são determinísticos e zero-LLM: nenhum modelo é chamado, e
// toda a verificação se apoia em ExtractClaims (textual) + VerifyClaim
// (token-overlap determinístico).
package grounding

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtractClaims_Citations marcada claims com citação → HasCitation true.
func TestExtractClaims_Citations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		text    string
		pattern []string
		wantCite bool
		wantMarker string
	}{
		{
			name:    "citação RAG",
			text:    "A resposta está em [RAG:doc-001]. Ela é correta.",
			pattern: nil,
			wantCite: true,
			wantMarker: "[RAG:doc-001]",
		},
		{
			name:    "citação posicional Chunk",
			text:    "A evidência [Chunk 3] sustenta a afirmação.",
			pattern: nil,
			wantCite: true,
			wantMarker: "[Chunk 3]",
		},
		{
			name:    "citação KnowledgeItem K",
			text:    "Segundo [K-1234], o RAG reduz alucinação.",
			pattern: nil,
			wantCite: true,
			wantMarker: "[K-1234]",
		},
		{
			name:    "sem citação",
			text:    "Esta sentença não tem nenhum marcador de citação.",
			pattern: nil,
			wantCite: false,
			wantMarker: "",
		},
		{
			name:    "multiple markers usa o primeiro",
			text:    "Veja [Chunk 2] e também [Chunk 9] para detalhes.",
			pattern: nil,
			wantCite: true,
			wantMarker: "[Chunk 2]",
		},
		{
			name:    "pattern customizado",
			text:    "O valor nominal é (abc) qualquer.",
			pattern: []string{`\([A-Za-z]+\)`},
			wantCite: true,
			wantMarker: "(abc)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := ExtractClaims(tt.text, tt.pattern)
			require.NotEmpty(t, claims, "extract should yield at least one claim")

			// O claim com citação é o primeiro que contém o marcador; vamos
			// localizar um claim que satisfaça o caso e assertar sobre ele.
			var cited *Claim
			for i := range claims {
				if claims[i].HasCitation == tt.wantCite {
					cited = &claims[i]
					break
				}
			}
			require.NotNil(t, cited, "no claim with HasCitation=%v", tt.wantCite)
			assert.Equal(t, tt.wantCite, cited.HasCitation)
			assert.Equal(t, tt.wantMarker, cited.CitationMarker)
			assert.NotEmpty(t, cited.Text)
		})
	}
}

// TestExtractClaims_SentenceSplitting confirma que a resposta é quebrada em
// sentenças e cada uma vira um Claim independente.
func TestExtractClaims_SentenceSplitting(t *testing.T) {
	t.Parallel()

	text := "O sol é uma estrela de plasma. A lua é um satélite natural. [RAG:x] certo."
	claims := ExtractClaims(text, DefaultCitePatterns)

	require.Len(t, claims, 3)

	// Só a terceira sentença tem citação.
	assert.False(t, claims[0].HasCitation)
	assert.False(t, claims[1].HasCitation)
	assert.True(t, claims[2].HasCitation)
	assert.Equal(t, "[RAG:x]", claims[2].CitationMarker)

	// O texto deve preservar o conteúdo (trimmed).
	assert.Equal(t, "O sol é uma estrela de plasma.", claims[0].Text)
	assert.Equal(t, "A lua é um satélite natural.", claims[1].Text)
	assert.Equal(t, "[RAG:x] certo.", claims[2].Text)
}

// TestExtractClaims_NilFallsBackToDefault confirma que patterns nil/vazio
// caem nos DefaultCitePatterns.
func TestExtractClaims_NilFallsBackToDefault(t *testing.T) {
	t.Parallel()

	for _, pattern := range [][]string{nil, {}, {""}} {
		claims := ExtractClaims("Fato verificado [Chunk 1] aqui.", pattern)
		require.NotEmpty(t, claims)
		assert.True(t, claims[0].HasCitation, "expected default patterns to match [Chunk N]")
		assert.Equal(t, "[Chunk 1]", claims[0].CitationMarker)
	}
}

// TestExtractClaims_EmptyText confirma que texto vazio produz zero claims.
func TestExtractClaims_EmptyText(t *testing.T) {
	t.Parallel()

	assert.Empty(t, ExtractClaims("", DefaultCitePatterns))
	assert.Empty(t, ExtractClaims("   ", DefaultCitePatterns))
}

// TestClaim_DefaultValues confirma o estado inicial de um Claim extraído
// (Substantiated pendente).
func TestClaim_DefaultValues(t *testing.T) {
	t.Parallel()

	claims := ExtractClaims("A resposta [RAG:a].", DefaultCitePatterns)
	require.Len(t, claims, 1)
	assert.False(t, claims[0].Substantiated, "fresh claim must not be substantiated")
	assert.True(t, claims[0].HasCitation)
}
