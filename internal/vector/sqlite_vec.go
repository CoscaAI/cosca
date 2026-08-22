// Package vector provides a SQLite-based vector store implementation.
package vector

import (
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CoscaAI/cosca/internal/safe"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// SQLiteVec implements the Store interface using SQLite as the backend.
// Vectors are stored as BLOBs in a dedicated table with indexing via
// brute-force cosine similarity (suitable for up to ~100K vectors).
//
// Since the L311 autópsia proved the full-scan is dominated by re-decoding the
// BLOBs through SQLite on every query (~94% of latency), SQLiteVec keeps a
// DERIVED in-memory snapshot of the vectors (see index.go): after the first
// search it scores the contiguous float32 slab instead of re-scanning SQLite.
// The snapshot is invalidated by a monotonic version counter bumped on every
// committed write, and any load failure falls back to the SQL scan (fail-safe).
type SQLiteVec struct {
	db         *sql.DB
	dimension  int
	mu         sync.RWMutex
	tableName  string
	normalized bool

	// version is bumped by every committed mutation (Store, Delete*,
	// Rebuild) and by StoreTxCommitted. It gates reloads of the in-memory
	// index: a snapshot is valid only while gen.version == version.
	version uint64
	// index is the derived in-memory snapshot (lazily built, atomically swapped).
	index *InMemoryIndex
	// indexEnabled lets the store skip the in-memory path (e.g. diagnostic
	// tooling that must measure the raw SQL scan). Defaults to true.
	indexEnabled atomic.Bool
}

// SQLiteVecConfig defines configuration for the SQLite vector store.
type SQLiteVecConfig struct {
	// DB is the SQLite database connection.
	DB *sql.DB

	// Dimension is the expected vector dimensionality.
	Dimension int

	// TableName is the name of the vector table (default: "vectors").
	TableName string

	// Normalized indicates whether vectors are stored normalized.
	Normalized bool

	// DisableInt8 turns off the quantized int8 AVX2 fast path of the in-memory
	// index, forcing exact float32 scoring. Default: false (fast path on).
	DisableInt8 bool

	// DisableInt16 turns off the int16 fast path (FASE A, L361). Default:
	// false — the int16 path is the production default (recall ~float32:
	// jaccard 0.989 / recall@10 0.995 no corpus limpo) with int8 as fallback
	// and float32 as exact oracle. Reversible at runtime via SetInt16Enabled.
	DisableInt16 bool
}

// NewSQLiteVec creates a new SQLite-backed vector store.
func NewSQLiteVec(cfg SQLiteVecConfig) (*SQLiteVec, error) {
	if cfg.DB == nil {
		return nil, fmt.Errorf("db connection is required")
	}
	if cfg.Dimension <= 0 {
		cfg.Dimension = 128
	}
	if cfg.TableName == "" {
		cfg.TableName = "vectors"
	}

	s := &SQLiteVec{
		db:         cfg.DB,
		dimension:  cfg.Dimension,
		tableName:  cfg.TableName,
		normalized: cfg.Normalized,
	}
	s.index = newInMemoryIndex(s)
	s.indexEnabled.Store(true)
	s.index.int8Enabled.Store(!cfg.DisableInt8)
	s.index.int16Enabled.Store(!cfg.DisableInt16)

	if err := s.createTable(); err != nil {
		return nil, fmt.Errorf("create vector table: %w", err)
	}

	log.Debug().
		Int("dimension", cfg.Dimension).
		Str("table", cfg.TableName).
		Msg("sqlite vector store initialized")

	return s, nil
}

// createTable ensures the vector storage table exists.
func (s *SQLiteVec) createTable() error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id          TEXT PRIMARY KEY,
			vector      BLOB NOT NULL,
			metadata    TEXT NOT NULL DEFAULT '{}',
			document_id TEXT NOT NULL DEFAULT '',
			chunk_id    TEXT NOT NULL DEFAULT '',
			entity_id   TEXT NOT NULL DEFAULT '',
			content     TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL DEFAULT (datetime('now'))
		)`, s.tableName)

	if _, err := s.db.Exec(query); err != nil {
		return err
	}

	// Indexes
	indexes := []string{
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_document ON %s(document_id)", s.tableName, s.tableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_entity ON %s(entity_id)", s.tableName, s.tableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_chunk ON %s(chunk_id)", s.tableName, s.tableName),
		// Recency pool of the hybrid-first candidate path scores the most
		// recently inserted vectors (created_at DESC) — without this index that
		// ORDER BY forces a full scan+sort of the store on every bounded search.
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_created ON %s(created_at)", s.tableName, s.tableName),
	}
	for _, idx := range indexes {
		if _, err := s.db.Exec(idx); err != nil {
			return err
		}
	}

	return nil
}

// Store stores vectors in the database.
func (s *SQLiteVec) Store(dimension int, vectors []VectorRecord) error {
	if len(vectors) == 0 {
		return nil
	}
	if dimension <= 0 {
		return fmt.Errorf("vector dimension is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if dimension != s.dimension {
		return fmt.Errorf("vector dimension mismatch: got %d, want %d", dimension, s.dimension)
	}
	for i, v := range vectors {
		if len(v.Vector) != dimension {
			return fmt.Errorf("vector %d dimension %d does not match %d", i, len(v.Vector), dimension)
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer safe.Rollback(tx)
	if err := s.storeTx(tx, dimension, vectors); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	s.bumpCommitted()
	return nil
}

// StoreTx stores vectors in the supplied transaction. It is used by the
// indexer to keep vector and document replacement in one atomic transaction.
//
// The transaction is committed by the CALLER, outside this lock. The version
// is bumped here (conservative invalidation) and again by StoreTxCommitted —
// which the indexer calls after its commit — so the in-memory index never
// bakes a pre-commit snapshot as final.
func (s *SQLiteVec) StoreTx(tx *sql.Tx, dimension int, vectors []VectorRecord) error {
	if tx == nil {
		return fmt.Errorf("transaction is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.version++ // staged write — invalidate now; StoreTxCommitted re-bumps post-commit
	return s.storeTx(tx, dimension, vectors)
}

// StoreTxCommitted implements vector.StoreTxCommitter. Callers that stage
// vectors via StoreTx MUST call this after their transaction commits. It
// invalidates the in-memory index against committed data: even if a search
// reloaded mid-transaction (pre-commit snapshot), this bump forces another
// reload once the commit lands. Fail-safe: no-ops harmless when no StoreTx is
// pending.
func (s *SQLiteVec) StoreTxCommitted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.version++
}

// bumpCommitted invalidates the in-memory index after a write committed inside
// this store's own lock. Callers must hold s.mu (write).
func (s *SQLiteVec) bumpCommitted() {
	s.version++
}

// storeTx is the implementation shared by Store and StoreTx. The caller owns
// transaction lifecycle; this function never commits or rolls it back.
func (s *SQLiteVec) storeTx(tx *sql.Tx, dimension int, vectors []VectorRecord) error {
	if len(vectors) == 0 {
		return nil
	}
	if dimension <= 0 {
		return fmt.Errorf("vector dimension is required")
	}
	if dimension != s.dimension {
		return fmt.Errorf("vector dimension mismatch: got %d, want %d", dimension, s.dimension)
	}
	for i, v := range vectors {
		if len(v.Vector) != dimension {
			return fmt.Errorf("vector %d dimension %d does not match %d", i, len(v.Vector), dimension)
		}
	}

	stmt, err := tx.Prepare(fmt.Sprintf(`
		INSERT OR REPLACE INTO %s (id, vector, metadata, document_id, chunk_id, entity_id, content, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'))`, s.tableName))
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, v := range vectors {
		id := v.ID
		if id == "" {
			id = uuid.New().String()
		}

		vec := v.Vector
		if s.normalized {
			vec = NormalizeVector(vec)
		}

		vectorBlob := float32SliceToBytes(vec)

		metadataJSON := "{}"
		if v.Metadata != nil {
			if data, err := json.Marshal(v.Metadata); err == nil {
				metadataJSON = string(data)
			}
		}

		if _, err := stmt.Exec(id, vectorBlob, metadataJSON, v.DocumentID, v.ChunkID, v.EntityID, v.Content); err != nil {
			return fmt.Errorf("insert vector %s: %w", id, err)
		}
	}

	return nil
}

// Search finds the closest vectors to the query vector using brute-force cosine similarity.
func (s *SQLiteVec) Search(query []float64, limit int) ([]SearchResult, error) {
	return s.SearchWithFilter(query, limit, nil)
}

// SearchWithFilter finds closest vectors with metadata filtering.
func (s *SQLiteVec) SearchWithFilter(query []float64, limit int, filter map[string]string) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	// In-memory fast path: full-scan WITHOUT metadata filter. The index keeps
	// only ids+vectors (SoA), so any filter (which needs the metadata column)
	// falls through to the SQL scan. Missing/stale/failed index degrades to
	// the same SQL scan (fail-safe — the index is never the source of truth).
	if len(filter) == 0 && s.indexEnabled.Load() {
		if scores := s.index.search(query, limit); scores != nil {
			return s.materializeScores(scores)
		}
	}

	where, args := s.filterClause(filter)
	rows, err := s.scanAll(where, args)
	if err != nil {
		return nil, err
	}
	return s.scoreAndMaterialize(query, limit, rows)
}

// SearchWithMetrics is the instrumented surface of the performance campaign
// (FASE 1): it runs the same candidate-restricted path as SearchWithCandidates
// and reports exactly what was scanned, so the real production route can be
// measured (candidate counts, full-scan vs bounded, int8 vs float32).
func (s *SQLiteVec) SearchWithMetrics(query []float64, limit int, candidateIDs []string, recentPool int, filter map[string]string) ([]SearchResult, SearchMetrics, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m := SearchMetrics{
		ReturnedK:   limit,
		Int8Enabled: s.index.int8Enabled.Load(),
		TotalVectors: func() int {
			if c := s.index.count(); c > 0 {
				return c
			}
			n, err := s.Count()
			if err != nil {
				return 0
			}
			return n
		}(),
	}

	if limit <= 0 {
		limit = 10
		m.ReturnedK = 10
	}

	// Caminho híbrido bounded (candidatos léxicos + pool de recentes).
	if len(candidateIDs) > 0 || recentPool > 0 {
		where, args := s.filterClause(filter)
		rows, err := s.scanCandidates(candidateIDs, recentPool, where, args)
		if err != nil {
			return nil, m, err
		}
		m.MetadataCandidates = len(rows)
		m.ScannedVectors = len(rows)
		m.UsedFloat32 = true
		res, err := s.materializeScores(scoreParallel(query, rows, limit))
		return res, m, err
	}

	// Sem candidatos: filtro → SQL scan; sem filtro → índice in-memory.
	if len(filter) > 0 || !s.indexEnabled.Load() {
		where, args := s.filterClause(filter)
		rows, err := s.scanAll(where, args)
		if err != nil {
			return nil, m, err
		}
		m.MetadataCandidates = len(rows)
		m.ScannedVectors = len(rows)
		m.UsedFloat32 = true
		res, err := s.materializeScores(scoreParallel(query, rows, limit))
		return res, m, err
	}

	// Fast path: índice in-memory (int8 AVX2 ou float32 exato).
	if scores := s.index.search(query, limit); scores != nil {
		m.MetadataCandidates = 0
		m.ScannedVectors = m.TotalVectors
		m.UsedFloat32 = !s.index.int8Enabled.Load() || s.index.gen.Load() == nil
		res, err := s.materializeScores(scores)
		return res, m, err
	}

	// Índice indisponível (load falhou) → SQL scan fail-safe.
	where, args := s.filterClause(filter)
	rows, err := s.scanAll(where, args)
	if err != nil {
		return nil, m, err
	}
	m.MetadataCandidates = len(rows)
	m.ScannedVectors = len(rows)
	m.UsedFloat32 = true
	res, err := s.materializeScores(scoreParallel(query, rows, limit))
	return res, m, err
}

// SearchWithCandidates finds the closest vectors restricted to candidateIDs
// plus the recentPool most recently inserted vectors. This is the hybrid-first
// path used by the layered search: when lexical candidates exist, the vector
// layer scores only those (bounded) instead of the full O(N) store.
func (s *SQLiteVec) SearchWithCandidates(query []float64, limit int, candidateIDs []string, recentPool int, filter map[string]string) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}
	if len(candidateIDs) == 0 && recentPool <= 0 {
		// No bounded set — degrade to the full scan path.
		return s.SearchWithFilter(query, limit, filter)
	}

	where, args := s.filterClause(filter)
	rows, err := s.scanCandidates(candidateIDs, recentPool, where, args)
	if err != nil {
		return nil, err
	}
	return s.scoreAndMaterialize(query, limit, rows)
}

// filterClause renders the metadata-filter WHERE fragment (" AND json_extract...").
func (s *SQLiteVec) filterClause(filter map[string]string) (string, []interface{}) {
	if len(filter) == 0 {
		return "", nil
	}
	var sb strings.Builder
	args := make([]interface{}, 0, len(filter))
	for k, v := range filter {
		sb.WriteString(" AND json_extract(metadata, '$.")
		sb.WriteString(escapeJSONPath(k))
		sb.WriteString("') = ?")
		args = append(args, v)
	}
	return sb.String(), args
}

// candidateRow is a (id, blob) pair fetched from the store.
type candidateRow struct {
	id   string
	blob []byte
}

// scoredRow is a scored candidate before its full columns are materialized.
type scoredRow struct {
	id    string
	score float64
}

// scanAll loads id+vector for every row matching the WHERE fragment.
func (s *SQLiteVec) scanAll(where string, args []interface{}) ([]candidateRow, error) {
	q := fmt.Sprintf("SELECT id, vector FROM %s WHERE 1=1%s", s.tableName, where)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("query vectors: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]candidateRow, 0, 1024)
	for rows.Next() {
		var r candidateRow
		if err := rows.Scan(&r.id, &r.blob); err != nil {
			return nil, fmt.Errorf("scan vector: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// scanCandidates loads id+vector for the candidate IDs plus the recent pool.
func (s *SQLiteVec) scanCandidates(ids []string, recentPool int, where string, args []interface{}) ([]candidateRow, error) {
	seen := make(map[string]bool, len(ids)+recentPool)
	out := make([]candidateRow, 0, len(ids)+recentPool)

	if len(ids) > 0 {
		placeholders := make([]string, len(ids))
		for i := range ids {
			placeholders[i] = "?"
		}
		selArgs := make([]interface{}, 0, len(ids)+len(args))
		for _, id := range ids {
			selArgs = append(selArgs, id)
		}
		selArgs = append(selArgs, args...)
		q := fmt.Sprintf("SELECT id, vector FROM %s WHERE id IN (%s) AND 1=1%s",
			s.tableName, strings.Join(placeholders, ","), where)
		rows, err := s.db.Query(q, selArgs...)
		if err != nil {
			return nil, fmt.Errorf("query candidate vectors: %w", err)
		}
		for rows.Next() {
			var id string
			var blob []byte
			if err := rows.Scan(&id, &blob); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("scan candidate vector: %w", err)
			}
			if seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, candidateRow{id: id, blob: blob})
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
	}

	if recentPool > 0 {
		q := fmt.Sprintf("SELECT id, vector FROM %s WHERE 1=1%s ORDER BY created_at DESC LIMIT %d",
			s.tableName, where, recentPool)
		rows, err := s.db.Query(q, args...)
		if err != nil {
			return nil, fmt.Errorf("query recent vectors: %w", err)
		}
		for rows.Next() {
			var id string
			var blob []byte
			if err := rows.Scan(&id, &blob); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("scan recent vector: %w", err)
			}
			if seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, candidateRow{id: id, blob: blob})
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
	}

	return out, nil
}

// scoreAndMaterialize scores the fetched rows (parallel), keeps the top-K and
// fetches the full columns only for the survivors.
func (s *SQLiteVec) scoreAndMaterialize(query []float64, limit int, rows []candidateRow) ([]SearchResult, error) {
	return s.materializeScores(scoreParallel(query, rows, limit))
}

// materializeScores fetches the full columns for the scored rows and builds the
// results in score order. Rows that vanished between the two queries are
// skipped. Shared by the SQL path and the in-memory index path so both produce
// identical SearchResults.
func (s *SQLiteVec) materializeScores(scores []scoredRow) ([]SearchResult, error) {
	if len(scores) == 0 {
		return nil, nil
	}

	ids := make([]string, len(scores))
	for i, sc := range scores {
		ids[i] = sc.id
	}
	meta, err := s.fetchMeta(ids)
	if err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(scores))
	for _, sc := range scores {
		m, ok := meta[sc.id]
		if !ok {
			continue // row vanished between the two queries — skip it
		}
		results = append(results, SearchResult{
			ID:         sc.id,
			Score:      sc.score,
			Metadata:   m.metadata,
			DocumentID: m.documentID,
			ChunkID:    m.chunkID,
			EntityID:   m.entityID,
			Content:    m.content,
		})
	}
	return results, nil
}

type searchMeta struct {
	metadata   map[string]string
	documentID string
	chunkID    string
	entityID   string
	content    string
}

// fetchMeta loads the full columns for the given IDs (bounded by limit).
func (s *SQLiteVec) fetchMeta(ids []string) (map[string]searchMeta, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	q := fmt.Sprintf("SELECT id, metadata, document_id, chunk_id, entity_id, content FROM %s WHERE id IN (%s)",
		s.tableName, strings.Join(placeholders, ","))
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("query vector metadata: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string]searchMeta, len(ids))
	for rows.Next() {
		var id, metadataJSON, docID, chunkID, entityID, content string
		if err := rows.Scan(&id, &metadataJSON, &docID, &chunkID, &entityID, &content); err != nil {
			return nil, fmt.Errorf("scan vector metadata: %w", err)
		}
		var metadata map[string]string
		if metadataJSON != "" && metadataJSON != "{}" {
			_ = json.Unmarshal([]byte(metadataJSON), &metadata)
		}
		if metadata == nil {
			metadata = make(map[string]string)
		}
		out[id] = searchMeta{metadata: metadata, documentID: docID, chunkID: chunkID, entityID: entityID, content: content}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// scoreParallel computes exact cosine similarity of every row against query,
// splitting the work across goroutines. The stored vectors are float32 BLOBs,
// so each worker decodes into a reused scratch buffer (no per-row allocation).
func scoreParallel(query []float64, rows []candidateRow, limit int) []scoredRow {
	if len(rows) == 0 || len(query) == 0 {
		return nil
	}
	qlen := len(query)

	var normA float64
	for _, v := range query {
		normA += v * v
	}
	if normA == 0 {
		return nil
	}
	sqrtNormA := math.Sqrt(normA)

	n := len(rows)
	if n < 256 {
		return scoreSerial(query, rows, limit, sqrtNormA)
	}

	workers := runtime.NumCPU()
	if workers > n {
		workers = n
	}
	if workers < 1 {
		workers = 1
	}
	chunk := (n + workers - 1) / workers

	var mu sync.Mutex
	collected := make([]scoredRow, 0, limit*workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		lo := w * chunk
		if lo >= n {
			break
		}
		hi := lo + chunk
		if hi > n {
			hi = n
		}
		wg.Add(1)
		go func(lo, hi int) {
			defer wg.Done()
			scratch := make([]float32, qlen)
			local := make([]scoredRow, 0, hi-lo)
			for i := lo; i < hi; i++ {
				blob := rows[i].blob
				if len(blob) != qlen*4 {
					continue
				}
				score := dotFromBytes(query, blob, scratch, sqrtNormA)
				if math.IsNaN(score) {
					continue
				}
				local = append(local, scoredRow{id: rows[i].id, score: score})
			}
			sort.Slice(local, func(a, b int) bool { return local[a].score > local[b].score })
			if len(local) > limit {
				local = local[:limit]
			}
			mu.Lock()
			collected = append(collected, local...)
			mu.Unlock()
		}(lo, hi)
	}
	wg.Wait()

	if len(collected) == 0 {
		return nil
	}
	sort.Slice(collected, func(a, b int) bool { return collected[a].score > collected[b].score })
	if len(collected) > limit {
		collected = collected[:limit]
	}
	return collected
}

// scoreSerial is the single-goroutine variant used for small candidate sets
// (goroutine spawn overhead would dominate).
func scoreSerial(query []float64, rows []candidateRow, limit int, sqrtNormA float64) []scoredRow {
	scratch := make([]float32, len(query))
	collected := make([]scoredRow, 0, len(rows))
	for _, r := range rows {
		if len(r.blob) != len(query)*4 {
			continue
		}
		score := dotFromBytes(query, r.blob, scratch, sqrtNormA)
		if math.IsNaN(score) {
			continue
		}
		collected = append(collected, scoredRow{id: r.id, score: score})
	}
	if len(collected) == 0 {
		return nil
	}
	sort.Slice(collected, func(a, b int) bool { return collected[a].score > collected[b].score })
	if len(collected) > limit {
		collected = collected[:limit]
	}
	return collected
}

// dotFromBytes decodes a float32 BLOB into scratch and computes the exact
// cosine similarity dot/(‖a‖·‖b‖) against query. sqrtNormA is hoisted out of
// the row loop (‖a‖ is identical for every row), so the result is bit-identical
// to CosineSimilarity while avoiding per-row allocations.
func dotFromBytes(q []float64, blob []byte, scratch []float32, sqrtNormA float64) float64 {
	for i := range scratch {
		scratch[i] = math.Float32frombits(binary.LittleEndian.Uint32(blob[i*4:]))
	}
	var dot, nb float64
	for i, f := range scratch {
		v := float64(f)
		dot += q[i] * v
		nb += v * v
	}
	if nb == 0 || sqrtNormA == 0 {
		return 0
	}
	return dot / (sqrtNormA * math.Sqrt(nb))
}

// Delete removes vectors by their IDs.
func (s *SQLiteVec) Delete(ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE id IN (%s)", s.tableName, strings.Join(placeholders, ","))
	if _, err := s.db.Exec(query, args...); err != nil {
		return err
	}
	s.bumpCommitted()
	return nil
}

// DeleteByDocument removes all vectors associated with a document.
func (s *SQLiteVec) DeleteByDocument(documentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.db.Exec(fmt.Sprintf("DELETE FROM %s WHERE document_id = ?", s.tableName), documentID); err != nil {
		return err
	}
	s.bumpCommitted()
	return nil
}

// DeleteByEntity removes all vectors associated with an entity.
func (s *SQLiteVec) DeleteByEntity(entityID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.db.Exec(fmt.Sprintf("DELETE FROM %s WHERE entity_id = ?", s.tableName), entityID); err != nil {
		return err
	}
	s.bumpCommitted()
	return nil
}

// Rebuild rebuilds the vector store (recreates the table).
func (s *SQLiteVec) Rebuild() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := time.Now()

	if _, err := s.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", s.tableName)); err != nil {
		return fmt.Errorf("drop table: %w", err)
	}

	if err := s.createTable(); err != nil {
		return fmt.Errorf("recreate table: %w", err)
	}

	s.bumpCommitted()
	log.Info().Dur("duration", time.Since(start)).Msg("vector store rebuilt")
	return nil
}

// Stats returns current vector store statistics.
func (s *SQLiteVec) Stats() (VectorStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	err := s.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", s.tableName)).Scan(&count)
	if err != nil {
		return VectorStats{}, fmt.Errorf("count vectors: %w", err)
	}

	return VectorStats{
		TotalVectors: count,
		Dimensions:   s.dimension,
		IndexType:    "flat_bruteforce",
		MemoryUsage:  int64(count * s.dimension * 4), // rough estimate: float32 per dim
	}, nil
}

// Dimension returns the expected vector dimensionality.
func (s *SQLiteVec) Dimension() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dimension
}

// Count returns the total number of stored vectors.
func (s *SQLiteVec) Count() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	err := s.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", s.tableName)).Scan(&count)
	return count, err
}

// Close cleans up vector store resources.
func (s *SQLiteVec) Close() error {
	return nil
}

// SetInt8Enabled toggles the quantized int8 AVX2 fast path of the in-memory
// index at runtime. Off forces exact float32 scoring; the SQL scan remains the
// ultimate fail-safe either way.
func (s *SQLiteVec) SetInt8Enabled(on bool) {
	s.index.int8Enabled.Store(on)
}

// SetInt16Enabled toggles the int16 fast path (FASE A, L347) at runtime.
// When on, it takes precedence over the int8 path; the SQL scan remains the
// ultimate fail-safe.
func (s *SQLiteVec) SetInt16Enabled(on bool) {
	s.index.int16Enabled.Store(on)
}

// ── Binary encoding helpers ────────────────────────────────────────────────

// float32SliceToBytes encodes a []float64 as a byte slice using float32
// precision (4 bytes per value).
func float32SliceToBytes(v []float64) []byte {
	buf := make([]byte, len(v)*4)
	for i, val := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(float32(val)))
	}
	return buf
}

// bytesToFloat32Slice decodes a float32-encoded byte slice back to []float64.
func bytesToFloat32Slice(data []byte) []float64 {
	if len(data) == 0 {
		return nil
	}
	n := len(data) / 4
	v := make([]float64, n)
	for i := 0; i < n; i++ {
		v[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:])))
	}
	return v
}

// BytesToFloat64Slice decodes a float32-encoded byte slice (as stored by the
// vector store) back to []float64. Exported so the indexer can reuse existing
// vectors from the store without re-embedding identical content (dedup
// pré-embed). Nil/empty input yields nil.
func BytesToFloat64Slice(data []byte) []float64 {
	return bytesToFloat32Slice(data)
}

// escapeJSONPath escapes a string for use as a JSON path key.
func escapeJSONPath(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
