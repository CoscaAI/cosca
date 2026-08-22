// Package knowledge provides the main Knowledge Engine for the Cosca Enterprise Platform.
//
// entity_vectors.go — Vetorização dos NÓS do grafo (entities).
//
// O Don pediu: "index node vector busca semantica" — indexar os nós do grafo
// com vetores para que a busca semântica encontre skills/ADRs/patterns/bugs por
// similaridade, não só por substring de nome. Antes deste pipeline, a tabela
// vectors só tinha linhas ligadas via document_id/chunk_id (entity_id sempre
// ''): o grafo não tinha vetores próprios.
package knowledge

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/CoscaAI/cosca/internal/graph"
	"github.com/CoscaAI/cosca/internal/vector"
)

// EntityVectorStats reports the outcome of a graph-node vectorization pass.
type EntityVectorStats struct {
	// Total is the number of graph nodes examined.
	Total int `json:"total"`
	// Skipped is the number of nodes filtered out (chunk without edges,
	// chunk already vectorized by the document pipeline, or cut by limit).
	Skipped int `json:"skipped"`
	// Indexed is the number of vectors written with entity_id populated.
	Indexed int `json:"indexed"`
	// Missing is the number of nodes left without a vector because the
	// embedding provider was unavailable or failed (degradation).
	Missing int `json:"missing"`
	// Duration is the elapsed time of the pass.
	Duration time.Duration `json:"duration_ms"`
}

// entityVectorBatchSize bounds each embedding batch — same courtesy to the
// provider as RepairOrphanVectors.
const entityVectorBatchSize = 32

// IndexEntityVectors embeds the knowledge-graph nodes (entities) and stores the
// vectors in the vectors table with entity_id populated, so semantic search can
// find skills/ADRs/patterns/bugs by similarity — not only by name substring.
//
// The node set mirrors the persistence filter in persistGraphToSQL: chunk-type
// nodes without graph edges are skipped. Chunk-type nodes that already carry a
// vector (the document pipeline embeds every chunk's full content) are skipped
// too — an entity vector for them would only duplicate the coverage and pollute
// semantic results with short name/path texts.
//
// Degradation follows embedChunksWithMissing (indexer): without an embedding
// provider — or when a batch fails — the pass reports the missing nodes and
// returns nil, since entity vectors are an extra layer and FTS5 already covers
// the base search. With RequireEmbeddings the same conditions are terminal
// errors. Validation failures remain terminal in both modes.
//
// Idempotent: vectors are keyed by "ent-<entity_id>" (INSERT OR REPLACE) and
// stale entity vectors (entities removed from the graph since the last pass)
// are deleted up-front, so re-indexing never duplicates and never leaves
// orphans.
//
// limit bounds the number of nodes vectorized in this pass (0 or negative =
// all nodes). It is intended for slow embedding providers so the command can
// prove the pipeline on a subset.
func (e *Engine) IndexEntityVectors(ctx context.Context, limit int) (EntityVectorStats, error) {
	var stats EntityVectorStats

	e.mu.RLock()
	if !e.initialized {
		e.mu.RUnlock()
		return stats, fmt.Errorf("knowledge engine not initialized")
	}
	e.mu.RUnlock()

	if e.graph == nil || e.db == nil || e.vecStore == nil {
		return stats, fmt.Errorf("knowledge engine subsystems unavailable (graph, db or vector store)")
	}

	total, nodes, err := e.entityNodesForVectorization()
	if err != nil {
		return stats, err
	}
	stats.Total = total
	stats.Skipped = total - len(nodes)
	if stats.Total == 0 {
		log.Info().Msg("entity vectorization: no graph nodes to vectorize")
		return stats, nil
	}
	if len(nodes) == 0 {
		log.Info().Int("total", total).Msg("entity vectorization: all nodes filtered out")
		return stats, nil
	}

	// ── Degradation — same contract as embedChunksWithMissing ────────────
	if e.embRegistry == nil {
		if e.cfg.RequireEmbeddings {
			return stats, fmt.Errorf("embeddings required: no embedding provider available")
		}
		stats.Missing = stats.Total
		log.Warn().Int("nodes", stats.Total).
			Msg("entity vectorization skipped: no embedding provider; FTS remains available")
		return stats, nil
	}
	dim := e.embRegistry.Dimensions()
	if dim <= 0 {
		if e.cfg.RequireEmbeddings {
			return stats, fmt.Errorf("embeddings required: embedding dimension is required")
		}
		stats.Missing = stats.Total
		log.Warn().Msg("entity vectorization skipped: embedding provider has no dimension; FTS remains available")
		return stats, nil
	}

	start := time.Now()
	defer func() { stats.Duration = time.Since(start) }()

	// ── Apply the caller's limit to the vectorizable set ─────────────────
	if limit > 0 && len(nodes) > limit {
		stats.Skipped += len(nodes) - limit
		nodes = nodes[:limit]
	}
	if len(nodes) == 0 {
		return stats, nil
	}

	texts := make([]string, len(nodes))
	for i, node := range nodes {
		texts[i] = entityVectorContent(node)
	}

	// ── Writer lock: cleanup + stores must not race the document indexer ─
	unlockWriter := e.db.LockWriter()
	defer unlockWriter()

	// Full refresh: remove entity vectors left behind by entities that no
	// longer exist in the graph. INSERT OR REPLACE alone cannot clean them.
	if _, err := e.db.Exec("DELETE FROM vectors WHERE entity_id != ''"); err != nil {
		return stats, fmt.Errorf("clear stale entity vectors: %w", err)
	}

	for i := 0; i < len(nodes); i += entityVectorBatchSize {
		end := i + entityVectorBatchSize
		if end > len(nodes) {
			end = len(nodes)
		}
		batch := nodes[i:end]

		results, err := e.embRegistry.GenerateEmbeddings(ctx, texts[i:end])
		if err != nil {
			if e.cfg.RequireEmbeddings {
				return stats, fmt.Errorf("generate entity embeddings: %w", err)
			}
			stats.Missing += len(batch)
			log.Warn().Err(err).Int("batch", i/entityVectorBatchSize).
				Msg("entity vectorization: batch embedding failed, continuing with FTS")
			continue
		}
		if len(results) != len(batch) {
			return stats, fmt.Errorf(
				"embedding result count %d does not match node count %d", len(results), len(batch))
		}

		records := make([]vector.VectorRecord, 0, len(batch))
		for j, result := range results {
			if err := embeddings.ValidateEmbedding(result); err != nil {
				if e.cfg.RequireEmbeddings {
					return stats, fmt.Errorf("entity %s: %w", batch[j].ID, err)
				}
				stats.Missing++
				continue
			}
			if len(result.Vector) != dim {
				if e.cfg.RequireEmbeddings {
					return stats, fmt.Errorf(
						"entity %s embedding dimension %d does not match provider dimension %d",
						batch[j].ID, len(result.Vector), dim)
				}
				stats.Missing++
				continue
			}
			records = append(records, vector.VectorRecord{
				ID:       "ent-" + batch[j].ID,
				Vector:   result.Vector,
				EntityID: batch[j].ID,
				Content:  texts[i+j],
				Metadata: entityVectorMetadata(batch[j]),
			})
		}

		if len(records) > 0 {
			if err := e.vecStore.Store(dim, records); err != nil {
				return stats, fmt.Errorf("store entity vectors: %w", err)
			}
			stats.Indexed += len(records)
		}
	}

	log.Info().
		Int("total", stats.Total).
		Int("skipped", stats.Skipped).
		Int("indexed", stats.Indexed).
		Int("missing", stats.Missing).
		Dur("duration", stats.Duration).
		Msg("entity vectorization complete")

	return stats, nil
}

// entityNodesForVectorization returns the graph nodes to vectorize, mirroring
// the persistence filter in persistGraphToSQL (knowledge.go): chunk-type nodes
// without edges are skipped. Chunk nodes that already carry a vector (the
// document pipeline embeds every chunk's full content) are skipped too.
// It returns the total node count (for stats) and the surviving candidates.
func (e *Engine) entityNodesForVectorization() (int, []*graph.Node, error) {
	nodes := e.graph.GetAllNodes()
	edges := e.graph.GetAllEdges()

	referenced := make(map[string]bool, len(edges))
	for _, edge := range edges {
		referenced[edge.Source] = true
		referenced[edge.Target] = true
	}

	vectorized := make(map[string]bool)
	rows, err := e.db.Query("SELECT DISTINCT chunk_id FROM vectors WHERE chunk_id != ''")
	if err != nil {
		return 0, nil, fmt.Errorf("query vectorized chunks: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, nil, fmt.Errorf("scan vectorized chunk: %w", err)
		}
		vectorized[id] = true
	}
	if err := rows.Err(); err != nil {
		return 0, nil, err
	}

	out := make([]*graph.Node, 0, len(nodes))
	for _, node := range nodes {
		if node.Type == "chunk" && (!referenced[node.ID] || vectorized[node.ID]) {
			continue
		}
		out = append(out, node)
	}
	return len(nodes), out, nil
}

// entityVectorContent builds the representative text embedded for a graph
// node: type, name and path plus the semantically richest metadata fields
// (title, description, summary, decision, context, consequences, category,
// scope, status, owner, heading). The full file is deliberately NOT read —
// the node already carries the summary the vectorization needs.
func entityVectorContent(node *graph.Node) string {
	var sb strings.Builder
	if node.Type != "" {
		sb.WriteString(node.Type)
		sb.WriteString(": ")
	}
	sb.WriteString(node.Name)
	if node.Path != "" {
		sb.WriteString(" — ")
		sb.WriteString(node.Path)
	}
	for _, key := range []string{
		"title", "description", "summary", "decision", "context",
		"consequences", "category", "scope", "status", "owner", "heading",
	} {
		if v, ok := node.Metadata[key].(string); ok && strings.TrimSpace(v) != "" {
			sb.WriteString(". ")
			sb.WriteString(v)
		}
	}
	return sb.String()
}

// entityVectorMetadata builds the vector-record metadata for a node:
// entity_type plus name/path and the node's string-valued metadata. Search
// reads entity_type (entity results) and facets/filters read these keys via
// json_extract.
func entityVectorMetadata(node *graph.Node) map[string]string {
	meta := make(map[string]string, len(node.Metadata)+3)
	meta["entity_type"] = node.Type
	if node.Name != "" {
		meta["name"] = node.Name
	}
	if node.Path != "" {
		meta["path"] = node.Path
	}
	keys := make([]string, 0, len(node.Metadata))
	for k := range node.Metadata {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if v, ok := node.Metadata[k].(string); ok && v != "" {
			meta[k] = v
		}
	}
	return meta
}