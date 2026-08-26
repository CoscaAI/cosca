package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/CoscaAI/cosca/internal/search"
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

func TestToEngineKnowledgeResults_Epistemic(t *testing.T) {
	t.Parallel()
	sr := &search.SearchResults{
		Results: []search.SearchResult{
			{ID: "a", Content: "medido", Metadata: map[string]string{"epistemic": "MEASURED"}},
			{ID: "b", Content: "sem classe", Metadata: map[string]string{}},
		},
	}
	out := toEngineKnowledgeResults(sr)
	assert.Len(t, out.Results, 2)
	assert.Equal(t, "MEASURED", out.Results[0].Epistemic)
	assert.Equal(t, "", out.Results[1].Epistemic)
}
