package knowledge

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FASE 5 — Backfill epistêmico: povoar a classe epistêmica de documentos
// LEGADOS (indexados antes da Fase 4, logo sem `metadata_json.epistemic`).

func writeDoc(t *testing.T, engine *Engine, rel string, content string) string {
	t.Helper()
	d := filepath.Join(engine.cfg.RootDir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(d), 0o755))
	require.NoError(t, os.WriteFile(d, []byte(content), 0o644))
	return d
}

func TestBackfillEpistemic_PopulatesDECISION(t *testing.T) {
	engine := memoryOnlyEngine(t)

	// ADR → kind=adr → Epistemic=DECISION. Indexado SEM epistemic (legado).
	d := writeDoc(t, engine, filepath.Join("docs", "adr", "ADR-test.md"),
		"# ADR-001\n\nDecisão de teste\n**Status**: accepted\n")
	require.NoError(t, engine.IndexDocumentWithMeta(context.Background(), d, map[string]any{"scope": "project"}))

	rep, err := engine.BackfillEpistemic(context.Background(), false)
	require.NoError(t, err)
	require.Equal(t, 1, rep.Scanned)
	require.Equal(t, 1, rep.Updated)
	assert.Equal(t, 1, rep.ByEpistemic["DECISION"], "ADR deve virar DECISION")

	var meta string
	require.NoError(t, engine.db.QueryRow(`SELECT metadata_json FROM documents WHERE path=?`, d).Scan(&meta))
	assert.Contains(t, meta, `"epistemic":"DECISION"`)
	assert.Contains(t, meta, `"scope":"project"`)
	assert.Contains(t, meta, `"kind":"adr"`)
}

func TestBackfillEpistemic_LearningBecomesINFERRED(t *testing.T) {
	engine := memoryOnlyEngine(t)
	d := writeDoc(t, engine, filepath.Join("memory", "pattern", "p.md"),
		"# Pattern\n\nPadrão reutilizável\n**Applicability**: geral\n")
	require.NoError(t, engine.IndexDocumentWithMeta(context.Background(), d, map[string]any{"scope": "project"}))

	rep, err := engine.BackfillEpistemic(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 1, rep.ByEpistemic["INFERRED"], "pattern deve virar INFERRED")
}

func TestBackfillEpistemic_Idempotent(t *testing.T) {
	engine := memoryOnlyEngine(t)
	d := writeDoc(t, engine, filepath.Join("docs", "adr", "ADR-x.md"),
		"# ADR-002\n\nConteúdo\n")
	require.NoError(t, engine.IndexDocumentWithMeta(context.Background(), d, map[string]any{"scope": "project"}))

	rep1, err := engine.BackfillEpistemic(context.Background(), false)
	require.NoError(t, err)
	require.Equal(t, 1, rep1.Updated)

	// segunda execução: nada muda (already_labeled), não duplica epi.
	rep2, err := engine.BackfillEpistemic(context.Background(), false)
	require.NoError(t, err)
	require.Equal(t, 0, rep2.Updated)
	require.Equal(t, 1, rep2.AlreadyLabeled)
	assert.Equal(t, 0, len(rep2.ByEpistemic), "não deve re-classificar o já rotulado")
}

func TestBackfillEpistemic_DryRunDoesNotWrite(t *testing.T) {
	engine := memoryOnlyEngine(t)
	d := writeDoc(t, engine, filepath.Join("docs", "adr", "ADR-dry.md"),
		"# ADR-003\n\nConteúdo\n")
	require.NoError(t, engine.IndexDocumentWithMeta(context.Background(), d, map[string]any{"scope": "project"}))

	rep, err := engine.BackfillEpistemic(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, 1, rep.Updated, "dry-run calcula o prognóstico")

	var meta string
	require.NoError(t, engine.db.QueryRow(`SELECT metadata_json FROM documents WHERE path=?`, d).Scan(&meta))
	assert.NotContains(t, meta, `"epistemic"`, "dry-run NÃO escreve")
}

func TestBackfillEpistemic_FailClosed(t *testing.T) {
	engine := memoryOnlyEngine(t)
	// Artifato transitório (path com "session") → classificador NÃO persiste.
	d := writeDoc(t, engine, filepath.Join("session", "foobar.md"),
		"# Sessão\n\nlog de execução\n")
	require.NoError(t, engine.IndexDocumentWithMeta(context.Background(), d, map[string]any{"scope": "project"}))

	rep, err := engine.BackfillEpistemic(context.Background(), false)
	require.NoError(t, err)
	require.Equal(t, 1, rep.NotPersistent, "transiente não deve ser classificado")
	assert.Equal(t, 0, rep.Updated)

	var meta string
	require.NoError(t, engine.db.QueryRow(`SELECT metadata_json FROM documents WHERE path=?`, d).Scan(&meta))
	assert.NotContains(t, meta, `"epistemic"`, "fail-closed: não rotula transitório")
}
