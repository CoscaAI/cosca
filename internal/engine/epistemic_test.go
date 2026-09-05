package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// FASE 4.1 — o resultado de conhecimento carrega a classe epistêmica e o
// contexto a exibe. Um item INFERRED NUNCA é lido como fato.

func TestConcatEpistemic_Prefix(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "[INFERRED] derivado", concatEpistemic(KnowledgeSearchResult{Epistemic: "INFERRED", Content: "derivado"}))
	assert.Equal(t, "[FACT] é assim", concatEpistemic(KnowledgeSearchResult{Epistemic: "FACT", Content: "é assim"}))
	// Sem classe → conteúdo puro (compatível).
	assert.Equal(t, "conteúdo", concatEpistemic(KnowledgeSearchResult{Content: "conteúdo"}))
}

