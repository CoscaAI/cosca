package knowledge

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// FASE 4 — Epistemologia: a classe epistêmica de um item de conhecimento.
// A regra de ouro: INFERRED/EVIDENCE NUNCA são confiáveis como fato.
func TestKnowledgeEpistemic_Valid(t *testing.T) {
	t.Parallel()
	for _, e := range []KnowledgeEpistemic{
		EpistemicFACT, EpistemicMEASURED, EpistemicEVIDENCE,
		EpistemicINFERRED, EpistemicRULE, EpistemicDECISION, EpistemicPROFILE,
	} {
		assert.True(t, e.Valid(), "%s deve ser válido", e)
	}
	assert.False(t, KnowledgeEpistemic("BOGUS").Valid())
	assert.False(t, KnowledgeEpistemic("").Valid())
}

func TestKnowledgeEpistemic_Trustworthy_Gate(t *testing.T) {
	t.Parallel()
	// Regra de ouro: só FACT/MEASURED são apresentados como fato.
	assert.True(t, EpistemicFACT.Trustworthy())
	assert.True(t, EpistemicMEASURED.Trustworthy())
	// INFERRED NUNCA é fato (a regra central da Fase 4).
	assert.False(t, EpistemicINFERRED.Trustworthy())
	assert.False(t, EpistemicEVIDENCE.Trustworthy())
	// Autoritativos, mas não são medição-fato.
	assert.False(t, EpistemicRULE.Trustworthy())
	assert.False(t, EpistemicDECISION.Trustworthy())
	assert.False(t, EpistemicPROFILE.Trustworthy())
}

func TestKnowledgeEpistemic_Label(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "[FACT]", EpistemicFACT.Label())
	assert.Equal(t, "[INFERRED]", EpistemicINFERRED.Label())
	assert.Equal(t, "[MEASURED]", EpistemicMEASURED.Label())
	assert.Equal(t, "[UNKNOWN]", KnowledgeEpistemic("").Label())
}

func TestEpistemicFor_Mapping(t *testing.T) {
	t.Parallel()
	assert.Equal(t, EpistemicDECISION, EpistemicFor("decision"))
	assert.Equal(t, EpistemicDECISION, EpistemicFor("adr"))
	assert.Equal(t, EpistemicDECISION, EpistemicFor("architecture"))
	assert.Equal(t, EpistemicRULE, EpistemicFor("rule"))
	assert.Equal(t, EpistemicPROFILE, EpistemicFor("profile"))
	assert.Equal(t, EpistemicPROFILE, EpistemicFor("capability"))
	// Aprendizado/padrão/técnica = derivação → INFERRED (nunca fato).
	assert.Equal(t, EpistemicINFERRED, EpistemicFor("learning"))
	assert.Equal(t, EpistemicINFERRED, EpistemicFor("pattern"))
	assert.Equal(t, EpistemicINFERRED, EpistemicFor("technique"))
	// Desconhecido → EVIDENCE (observação registrada, a classe mais honesta).
	assert.Equal(t, EpistemicEVIDENCE, EpistemicFor("algo-novo"))
}
