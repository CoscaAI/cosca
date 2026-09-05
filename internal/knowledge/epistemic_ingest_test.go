package knowledge

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CoscaAI/cosca/internal/search"
)

// FASE 4.1 — a classe epistêmica armazenada em metadata_json.epistemic chega ao
// resultado via enrichEpistemic, para o agente ver a natureza do item e o
// filtro `epistemic=` funcionar de ponta a ponta.

// TestEnrichEpistemic_ByPath isola a lógica de enrichment: indexa um documento
// com `metadata_json.epistemic` e confirma que um resultado que carrega o
// DocumentPath desse documento é enriquecido.
func TestEnrichEpistemic_ByPath(t *testing.T) {
	engine := memoryOnlyEngine(t)

	mdPath := filepath.Join(engine.cfg.RootDir, "alpha.md")
	require.NoError(t, os.WriteFile(mdPath, []byte("# Alpha\n\nconteúdo x"), 0o644))
	require.NoError(t, engine.IndexDocumentWithMeta(context.Background(), mdPath, map[string]any{
		"epistemic": "FACT",
		"kind":      "decision",
	}))

	results := &search.SearchResults{Results: []search.SearchResult{
		{ID: "r1", DocumentPath: mdPath}, // resultado que carrega o path do doc
	}}
	engine.enrichEpistemic(results)

	assert.Equal(t, "FACT", results.Results[0].Metadata["epistemic"],
		"enrichEpistemic deve preencher Metadata['epistemic'] pelo path do documento")
}

// TestEnrichEpistemic_ByID confirma a chave por DocumentID.
func TestEnrichEpistemic_ByID(t *testing.T) {
	engine := memoryOnlyEngine(t)

	mdPath := filepath.Join(engine.cfg.RootDir, "beta.md")
	require.NoError(t, os.WriteFile(mdPath, []byte("# Beta\n\nconteúdo y"), 0o644))
	require.NoError(t, engine.IndexDocumentWithMeta(context.Background(), mdPath, map[string]any{
		"epistemic": "MEASURED",
	}))

	// Descobre o id do documento recém-indexado.
	var docID string
	require.NoError(t, engine.db.QueryRow(`SELECT id FROM documents WHERE path = ?`, mdPath).Scan(&docID))

	results := &search.SearchResults{Results: []search.SearchResult{
		{ID: "r1", DocumentID: docID},
	}}
	engine.enrichEpistemic(results)
	assert.Equal(t, "MEASURED", results.Results[0].Metadata["epistemic"])
}
