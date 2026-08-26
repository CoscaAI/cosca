package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── IndexDocumentWithMeta: grava metadata_json com proveniência ──────────────

func TestIndexDocumentWithMeta_WritesProvenanceMetadata(t *testing.T) {
	t.Parallel()

	engine := initTestEngine(t)
	rootDir := engine.cfg.RootDir

	p := writeTempMarkdown(t, rootDir, "provenance.md", "# Provenance\n\nConteúdo de teste com proveniência.")
	meta := map[string]any{
		"scope":   "project",
		"origin":  "opencode",
		"kind":    "learning",
		"agent":   "cosca-uiux",
		"project": "cosca",
	}
	require.NoError(t, engine.IndexDocumentWithMeta(context.Background(), p, meta))

	var metaJSON string
	require.NoError(t, engine.db.QueryRow("SELECT metadata_json FROM documents WHERE path = ?", p).Scan(&metaJSON))

	var stored map[string]any
	require.NoError(t, json.Unmarshal([]byte(metaJSON), &stored))
	assert.Equal(t, "project", stored["scope"])
	assert.Equal(t, "opencode", stored["origin"])
	assert.Equal(t, "learning", stored["kind"])
	assert.Equal(t, "cosca-uiux", stored["agent"])
	assert.Equal(t, "cosca", stored["project"])
	// O indexer preserva o que ele já produzia.
	assert.Equal(t, "Provenance", stored["title"])
	assert.Equal(t, "markdown", stored["type"])
}

// ── IndexDocument legacy: sem metadata extra (não muda comportamento) ────────

func TestIndexDocument_Legacy_NoProvenanceMetadata(t *testing.T) {
	t.Parallel()

	engine := initTestEngine(t)
	rootDir := engine.cfg.RootDir

	p := writeTempMarkdown(t, rootDir, "legacy.md", "# Legacy\n\nSem proveniência semântica.")
	require.NoError(t, engine.IndexDocument(context.Background(), p))

	var metaJSON string
	require.NoError(t, engine.db.QueryRow("SELECT metadata_json FROM documents WHERE path = ?", p).Scan(&metaJSON))

	assert.Contains(t, metaJSON, `"title"`)
	assert.Contains(t, metaJSON, `"path"`)
	assert.Contains(t, metaJSON, `"type":"markdown"`)
	assert.NotContains(t, metaJSON, `"scope"`)
	assert.NotContains(t, metaJSON, `"kind"`)
	assert.NotContains(t, metaJSON, `"origin"`)
	assert.NotContains(t, metaJSON, `"agent"`)
}

// ── Idempotência: mesmo SHA256 ingerido duas vezes não duplica ───────────────

func TestIngestLearningBlock_IdempotentBySHA256(t *testing.T) {
	t.Parallel()

	engine := initTestEngine(t)
	rootDir := engine.cfg.RootDir

	content := "PREV: 000\nID: L1\nLEVEL: 4\n---\n## L1 | 2026-08-26 | achado persistente | Level 4\n\n" +
		"| Field | Value |\n|-------|-------|\n| **Learned** | conhecimento com valor futuro |\n| **Outcome** | success |\n"
	sum := sha256.Sum256([]byte(content))
	hash := hex.EncodeToString(sum[:])

	blockPath := filepath.Join(rootDir, hash+".md")
	require.NoError(t, os.WriteFile(blockPath, []byte(content), 0o644))

	// 1ª ingestão → ingested.
	res1, err := engine.IngestLearningBlock(context.Background(), blockPath, "cosca-kernel")
	require.NoError(t, err)
	require.NotNil(t, res1)
	assert.Equal(t, IngestStatusIngested, res1.Status)
	assert.Equal(t, hash, res1.Hash)
	assert.Equal(t, ScopeProject, res1.Scope, "origem de projeto nunca vira scope=global")
	assert.Equal(t, KindLearning, res1.Kind)

	// 2ª ingestão → already_ingested (idempotência por conteúdo).
	res2, err := engine.IngestLearningBlock(context.Background(), blockPath, "cosca-kernel")
	require.NoError(t, err)
	require.NotNil(t, res2)
	assert.Equal(t, IngestStatusAlreadyIngested, res2.Status)

	// Apenas UM documento com esse hash existe.
	var count int
	require.NoError(t, engine.db.QueryRow("SELECT COUNT(*) FROM documents WHERE hash = ?", hash).Scan(&count))
	assert.Equal(t, 1, count, "o mesmo bloco (mesmo SHA256) NÃO deve duplicar")
}

// ── Ingestão: bloco não-persistente (fail-closed) não indexa ─────────────────

func TestIngestLearningBlock_NotPersistent_Skipped(t *testing.T) {
	t.Parallel()

	engine := initTestEngine(t)
	rootDir := engine.cfg.RootDir

	// Bloco ambíguo, incapaz de ser classificado com confiança.
	content := "texto aleatório sem marcador de conhecimento consolidado"
	sum := sha256.Sum256([]byte(content))
	hash := hex.EncodeToString(sum[:])
	blockPath := filepath.Join(rootDir, hash+".md")
	require.NoError(t, os.WriteFile(blockPath, []byte(content), 0o644))

	res, err := engine.IngestLearningBlock(context.Background(), blockPath, "")
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, IngestStatusSkippedNotPersistent, res.Status)

	// Nada foi indexado.
	var count int
	require.NoError(t, engine.db.QueryRow("SELECT COUNT(*) FROM documents WHERE hash = ?", hash).Scan(&count))
	assert.Equal(t, 0, count, "bloco não-classificado NÃO deve ser indexado (fail-closed)")
}

// ── Ingestão: blockPath com nome que não é o SHA256 → erro ───────────────────

func TestIngestLearningBlock_WrongFilenameHash_Error(t *testing.T) {
	t.Parallel()

	engine := initTestEngine(t)
	rootDir := engine.cfg.RootDir

	content := "| **Learned** | algo |"
	blockPath := filepath.Join(rootDir, "not-the-hash.md")
	require.NoError(t, os.WriteFile(blockPath, []byte(content), 0o644))

	_, err := engine.IngestLearningBlock(context.Background(), blockPath, "")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "filename"), "deve reclamar do nome do arquivo != sha256")
}
