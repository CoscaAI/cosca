package knowledge

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CoscaAI/cosca/internal/graph"
	"github.com/CoscaAI/cosca/internal/search"
	"github.com/CoscaAI/cosca/internal/vector"
)

// ── Fixture helpers ─────────────────────────────────────────────────────────

// buildEntityVectorGraph seeds the engine's in-memory graph with a realistic
// mix: semantic nodes (skill, adr, doc), a chunk WITH an edge and a chunk
// WITHOUT edges.
func buildEntityVectorGraph(t *testing.T, engine *Engine) {
	t.Helper()

	g := engine.graph
	require.NotNil(t, g)

	require.NoError(t, g.AddNode(&graph.Node{
		ID:   "skill-1",
		Type: "skill",
		Name: "cosca-database",
		Path: "/skills/database.md",
		Metadata: map[string]interface{}{
			"description": "consulta segura do knowledge base",
			"category":    "database",
			"status":      "active",
			"position":    16, // non-string metadata must not break the builder
		},
	}))
	require.NoError(t, g.AddNode(&graph.Node{
		ID:   "adr-1",
		Type: "adr",
		Name: "ADR-1234: usar SQLite",
		Path: "/adr/adr-1234.md",
		Metadata: map[string]interface{}{
			"decision":    "adotamos SQLite como storage",
			"status":      "accepted",
			"adr_number":  "1234",
			"description": "",
		},
	}))
	require.NoError(t, g.AddNode(&graph.Node{
		ID:   "doc-1",
		Type: "doc",
		Name: "Guia de arquitetura",
		Path: "/docs/guia.md",
	}))
	require.NoError(t, g.AddNode(&graph.Node{
		ID:   "chunk-edged",
		Type: "chunk",
		Name: "chunk 1",
		Path: "/docs/guia.md",
		Metadata: map[string]interface{}{
			"heading": "Introdução",
		},
	}))
	require.NoError(t, g.AddNode(&graph.Node{
		ID:   "chunk-lone",
		Type: "chunk",
		Name: "chunk 2",
		Path: "/docs/outro.md",
	}))

	// chunk-edged participates in an edge; chunk-lone does not.
	require.NoError(t, g.AddEdge(&graph.Edge{
		Source: "chunk-edged", Target: "doc-1", Type: graph.RelContains,
	}))
}

// countEntityVectors returns the number of vectors with entity_id populated.
func countEntityVectors(t *testing.T, engine *Engine) int {
	t.Helper()
	var n int
	require.NoError(t, engine.db.QueryRow(
		"SELECT COUNT(*) FROM vectors WHERE entity_id != ''").Scan(&n))
	return n
}

// ── IndexEntityVectors: gera vetores com entity_id ─────────────────────────

func TestIndexEntityVectors_GeneratesEntityVectors(t *testing.T) {
	engine := initTestEngine(t)
	buildEntityVectorGraph(t, engine)

	stats, err := engine.IndexEntityVectors(context.Background(), 0)
	require.NoError(t, err)

	// 5 nodes total; chunk-lone (no edges) is filtered out.
	assert.Equal(t, 5, stats.Total)
	assert.Equal(t, 1, stats.Skipped)
	assert.Equal(t, 4, stats.Indexed)
	assert.Equal(t, 0, stats.Missing)

	assert.Equal(t, 4, countEntityVectors(t, engine))

	// The vector rows carry entity_id + deterministic id "ent-<entity_id>"
	// and no document/chunk linkage.
	var id, entityID, docID, chunkID, content string
	err = engine.db.QueryRow(
		"SELECT id, entity_id, document_id, chunk_id, content FROM vectors WHERE entity_id = 'skill-1'",
	).Scan(&id, &entityID, &docID, &chunkID, &content)
	require.NoError(t, err)
	assert.Equal(t, "ent-skill-1", id)
	assert.Equal(t, "skill-1", entityID)
	assert.Equal(t, "", docID)
	assert.Equal(t, "", chunkID)
	assert.Contains(t, content, "skill: cosca-database")
	assert.Contains(t, content, "/skills/database.md")
	assert.Contains(t, content, "consulta segura do knowledge base")

	// Metadata carries entity_type + node metadata (string values only).
	var metadataJSON string
	require.NoError(t, engine.db.QueryRow(
		"SELECT metadata FROM vectors WHERE entity_id = 'adr-1'").Scan(&metadataJSON))
	assert.Contains(t, metadataJSON, `"entity_type":"adr"`)
	assert.Contains(t, metadataJSON, `"decision":"adotamos SQLite como storage"`)
}

func TestIndexEntityVectors_MetadataStringValuesOnly(t *testing.T) {
	engine := initTestEngine(t)
	buildEntityVectorGraph(t, engine)

	_, err := engine.IndexEntityVectors(context.Background(), 0)
	require.NoError(t, err)

	// position (int) must not appear as a metadata key; name/path must.
	var metadataJSON string
	require.NoError(t, engine.db.QueryRow(
		"SELECT metadata FROM vectors WHERE entity_id = 'skill-1'").Scan(&metadataJSON))
	assert.Contains(t, metadataJSON, `"name":"cosca-database"`)
	assert.NotContains(t, metadataJSON, "position")
}

// ── IndexEntityVectors: filtra chunks sem arestas ──────────────────────────

func TestIndexEntityVectors_SkipsChunkWithoutEdges(t *testing.T) {
	engine := initTestEngine(t)
	buildEntityVectorGraph(t, engine)

	stats, err := engine.IndexEntityVectors(context.Background(), 0)
	require.NoError(t, err)

	assert.Equal(t, 4, stats.Indexed)
	assert.Equal(t, 0, countEntityVectorsFor(t, engine, "chunk-lone"))
	assert.Equal(t, 1, countEntityVectorsFor(t, engine, "chunk-edged"))
}

// ── IndexEntityVectors: pula chunk que já tem vetor ────────────────────────

func TestIndexEntityVectors_SkipsChunkWithExistingVector(t *testing.T) {
	engine := initTestEngine(t)
	buildEntityVectorGraph(t, engine)

	// Simulate the document pipeline: chunk-edged already has a chunk vector.
	require.NoError(t, engine.vecStore.Store(engine.vecStore.Dimension(), []vector.VectorRecord{{
		ID: "vec-chunk-edged", Vector: make([]float64, engine.vecStore.Dimension()),
		ChunkID: "chunk-edged", Content: "conteúdo completo do chunk",
	}}))

	stats, err := engine.IndexEntityVectors(context.Background(), 0)
	require.NoError(t, err)

	// chunk-edged now skipped too (already vectorized as a chunk).
	assert.Equal(t, 2, stats.Skipped)
	assert.Equal(t, 3, stats.Indexed)
	assert.Equal(t, 0, countEntityVectorsFor(t, engine, "chunk-edged"))
	assert.Equal(t, 1, countEntityVectorsFor(t, engine, "skill-1"))
}

// ── IndexEntityVectors: degradação sem registry ────────────────────────────

func TestIndexEntityVectors_NilRegistryDegrades(t *testing.T) {
	engine := initTestEngine(t)
	buildEntityVectorGraph(t, engine)
	engine.embRegistry = nil

	stats, err := engine.IndexEntityVectors(context.Background(), 0)
	require.NoError(t, err)

	assert.Equal(t, 5, stats.Total)
	assert.Equal(t, 5, stats.Missing)
	assert.Equal(t, 0, stats.Indexed)
	assert.Equal(t, 0, countEntityVectors(t, engine))
}

func TestIndexEntityVectors_NilRegistryRequireEmbeddingsFails(t *testing.T) {
	engine := initTestEngine(t)
	buildEntityVectorGraph(t, engine)
	engine.embRegistry = nil
	engine.cfg.RequireEmbeddings = true

	_, err := engine.IndexEntityVectors(context.Background(), 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embeddings required")
	assert.Equal(t, 0, countEntityVectors(t, engine))
}

// ── IndexEntityVectors: idempotência ───────────────────────────────────────

func TestIndexEntityVectors_Idempotent(t *testing.T) {
	engine := initTestEngine(t)
	buildEntityVectorGraph(t, engine)

	stats1, err := engine.IndexEntityVectors(context.Background(), 0)
	require.NoError(t, err)

	stats2, err := engine.IndexEntityVectors(context.Background(), 0)
	require.NoError(t, err)

	assert.Equal(t, stats1.Indexed, stats2.Indexed)
	assert.Equal(t, 4, countEntityVectors(t, engine), "re-index must not duplicate")
}

func TestIndexEntityVectors_Limit(t *testing.T) {
	engine := initTestEngine(t)
	buildEntityVectorGraph(t, engine)

	stats, err := engine.IndexEntityVectors(context.Background(), 2)
	require.NoError(t, err)

	assert.Equal(t, 2, stats.Indexed)
	assert.Equal(t, 3, stats.Skipped) // 1 sem arestas + 1 cortado pelo limite
	assert.Equal(t, 2, countEntityVectors(t, engine))
}

// ── IndexEntityVectors: grafo vazio ────────────────────────────────────────

func TestIndexEntityVectors_EmptyGraph(t *testing.T) {
	engine := initTestEngine(t)

	stats, err := engine.IndexEntityVectors(context.Background(), 0)
	require.NoError(t, err)

	assert.Zero(t, stats.Total)
	assert.Zero(t, stats.Indexed)
	assert.Equal(t, 0, countEntityVectors(t, engine))
}

func TestIndexEntityVectors_NotInitialized(t *testing.T) {
	engine := newTestEngine(t, filepath.Join(t.TempDir(), "ni.db"))

	_, err := engine.IndexEntityVectors(context.Background(), 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

// ── Busca semântica integrada ──────────────────────────────────────────────

// TestIndexEntityVectors_SearchReturnsEntities prova o circuito completo: após
// a vetorização, a busca vetorial do engine retorna as entidades com
// EntityType resolvido a partir do metadata (search.vectorResults lê
// entity_id + entity_type).
func TestIndexEntityVectors_SearchReturnsEntities(t *testing.T) {
	engine := initTestEngine(t)
	buildEntityVectorGraph(t, engine)

	stats, err := engine.IndexEntityVectors(context.Background(), 0)
	require.NoError(t, err)
	require.Equal(t, 4, stats.Indexed)

	results, err := engine.search.Search(context.Background(), search.SearchParams{
		Query: "consulta segura", Limit: 10, EnableFTS: false, EnableVector: true, EnableGraph: false,
	})
	require.NoError(t, err)
	require.NotNil(t, results)

	var foundSkill bool
	for _, r := range results.Results {
		if r.EntityType == "skill" {
			foundSkill = true
			assert.Equal(t, "cosca-database", r.Title, "title comes from metadata['name']")
			assert.NotEmpty(t, r.Content)
		}
	}
	assert.True(t, foundSkill, "semantic search must return the skill entity")
}

// ── entityVectorContent / entityVectorMetadata (unidades) ──────────────────

func TestEntityVectorContent(t *testing.T) {
	node := &graph.Node{
		ID: "n1", Type: "pattern", Name: "circuit-breaker", Path: "/patterns/cb.md",
		Metadata: map[string]interface{}{
			"description": "evita cascata de falhas",
			"empty":       "",
			"n":           7,
		},
	}
	content := entityVectorContent(node)
	assert.Contains(t, content, "pattern: circuit-breaker")
	assert.Contains(t, content, "/patterns/cb.md")
	assert.Contains(t, content, "evita cascata de falhas")
	assert.NotContains(t, content, "empty")
}

func TestEntityVectorMetadata(t *testing.T) {
	node := &graph.Node{
		ID: "n1", Type: "bug", Name: "nil pointer", Path: "/bugs/b1.md",
		Metadata: map[string]interface{}{
			"status":  "open",
			"severity": "high",
			"count":   3, // non-string: dropped
		},
	}
	meta := entityVectorMetadata(node)
	assert.Equal(t, "bug", meta["entity_type"])
	assert.Equal(t, "nil pointer", meta["name"])
	assert.Equal(t, "/bugs/b1.md", meta["path"])
	assert.Equal(t, "open", meta["status"])
	assert.Equal(t, "high", meta["severity"])
	_, ok := meta["count"]
	assert.False(t, ok)
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func countEntityVectorsFor(t *testing.T, engine *Engine, entityID string) int {
	t.Helper()
	var n int
	require.NoError(t, engine.db.QueryRow(
		"SELECT COUNT(*) FROM vectors WHERE entity_id = ?", entityID).Scan(&n))
	return n
}