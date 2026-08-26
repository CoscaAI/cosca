// Package indexer provides the main indexing pipeline for the Cosca Knowledge Engine.
// It coordinates document parsing, chunking, embedding, vector storage, SQLite storage,
// and graph construction into a single pipeline with progress reporting and error tolerance.
package indexer

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CoscaAI/cosca/internal/chunker"
	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/CoscaAI/cosca/internal/graph"
	"github.com/CoscaAI/cosca/internal/markdown"
	"github.com/CoscaAI/cosca/internal/parser"
	"github.com/CoscaAI/cosca/internal/sqlite"
	"github.com/CoscaAI/cosca/internal/vector"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// ProgressCallback is called during indexing to report progress.
type ProgressCallback func(current, total int, phase string, err error)

// IndexStats provides statistics about the indexed content.
type IndexStats struct {
	TotalDocuments  int            `json:"total_documents"`
	TotalChunks     int            `json:"total_chunks"`
	TotalEntities   int            `json:"total_entities"`
	TotalVectors    int            `json:"total_vectors"`
	MissingVectors  int            `json:"missing_vectors"`
	TotalErrors     int            `json:"total_errors"`
	IndexedTypes    map[string]int `json:"indexed_types"`
	LastIndexed     time.Time      `json:"last_indexed"`
	Duration        time.Duration  `json:"duration"`
	CacheHitRate    float64        `json:"cache_hit_rate"`
	DocumentsByType map[string]int `json:"documents_by_type"`
}

// IndexerConfig defines the indexing pipeline configuration.
//
//nolint:revive // Stutter name preserved for API compatibility — used as indexer.IndexerConfig externally.
type IndexerConfig struct {
	// RootDir is the root directory for relative path resolution.
	RootDir string

	// EmbedBatchSize is the number of texts to embed in a single batch (default: 20).
	EmbedBatchSize int

	// IndexConcurrency is the number of worker goroutines used to index
	// documents in parallel (default: 0 = runtime.NumCPU(), capped at 8).
	// Parse/chunk/store run concurrently across workers while the embedding
	// provider serializes its own batch processing.
	IndexConcurrency int

	// IndexCodeBlocks enables indexing of code blocks separately (default: true).
	IndexCodeBlocks bool

	// IndexTables enables indexing of tables separately (default: true).
	IndexTables bool

	// ExtractEntities enables entity extraction (default: true).
	ExtractEntities bool

	// BuildGraph enables graph construction (default: true).
	BuildGraph bool

	// FollowSymlinks enables following symbolic links (default: false).
	FollowSymlinks bool

	// ExcludedPatterns is a list of glob patterns to exclude.
	ExcludedPatterns []string

	// AllowedExtensions restricts which file extensions to index.
	AllowedExtensions []string

	// MaxFileSize is the maximum file size in bytes to index (default: 10MB).
	MaxFileSize int64

	// RequireEmbeddings makes embedding generation a prerequisite for a
	// successful index operation. When false, documents are still indexed into
	// SQLite/FTS and missing vectors are reported in MissingVectors.
	RequireEmbeddings bool
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() IndexerConfig {
	return IndexerConfig{
		EmbedBatchSize:  20,
		IndexCodeBlocks: true,
		IndexTables:     true,
		ExtractEntities: true,
		BuildGraph:      true,
		FollowSymlinks:  false,
		MaxFileSize:     10 * 1024 * 1024, // 10MB
	}
}

// Indexer coordinates the full indexing pipeline.
type Indexer struct {
	cfg          IndexerConfig
	mdParser     *markdown.Parser
	entityParser *parser.EntityParser
	chunker      *chunker.Chunker
	embRegistry  *embeddings.ProviderRegistry
	vecStore     vector.Store
	fts          *sqlite.FTSClient
	db           *sqlite.DB
	graphBuilder *graph.Builder
	mu           sync.RWMutex
	stats        IndexStats
	errors       []IndexError
	progressCb   ProgressCallback
	docHashCache map[string]string // path -> hash
}

// IndexError records a per-file indexing error.
type IndexError struct {
	Path      string    `json:"path"`
	Error     string    `json:"error"`
	Phase     string    `json:"phase"`
	Timestamp time.Time `json:"timestamp"`
}

// New creates a new indexer with the given dependencies.
func New(
	cfg IndexerConfig,
	mdParser *markdown.Parser,
	entityParser *parser.EntityParser,
	chunker *chunker.Chunker,
	embRegistry *embeddings.ProviderRegistry,
	vecStore vector.Store,
	ftsClient *sqlite.FTSClient,
	db *sqlite.DB,
	graphBuilder *graph.Builder,
) *Indexer {
	return &Indexer{
		cfg:          cfg,
		mdParser:     mdParser,
		entityParser: entityParser,
		chunker:      chunker,
		embRegistry:  embRegistry,
		vecStore:     vecStore,
		fts:          ftsClient,
		db:           db,
		graphBuilder: graphBuilder,
		docHashCache: make(map[string]string),
		stats: IndexStats{
			IndexedTypes:    make(map[string]int),
			DocumentsByType: make(map[string]int),
		},
	}
}

// SetProgressCallback sets a callback for progress reporting.
func (idx *Indexer) SetProgressCallback(cb ProgressCallback) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.progressCb = cb
}

// IndexDocument indexes a single document through the full pipeline.
// Pipeline: parse -> chunk -> embed -> store -> graph
//
// Security: the path is validated against RootDir before anything is read
// (path traversal prevention, C1). Symlinks are resolved so a link inside
// the root cannot point at arbitrary files (e.g. /etc/passwd), and the file
// size is enforced before reading so oversized files are never loaded.
// IndexDocument indexes a single document through the full pipeline (legacy).
// It delegates to IndexDocumentWithMeta with no extra metadata so behavior is
// identical to before FASE 2.
func (idx *Indexer) IndexDocument(ctx context.Context, path string) error {
	return idx.indexDocument(ctx, path, nil)
}

// IndexDocumentWithMeta indexes a single document merging extraMeta into the
// produced metadata_json (preserving what the indexer already writes: title,
// path, type, headings, links). extraMeta carries the semantic provenance the
// caller computed (scope/project/origin/kind/agent) — the indexer writes it
// verbatim into metadata_json alongside its own data.
func (idx *Indexer) IndexDocumentWithMeta(ctx context.Context, path string, extraMeta map[string]any) error {
	return idx.indexDocument(ctx, path, extraMeta)
}

func (idx *Indexer) indexDocument(ctx context.Context, path string, extraMeta map[string]any) error {
	// Path containment: resolve symlinks and require the target to be equal
	// to or a descendant of RootDir.
	absPath, err := validatePathWithin(idx.cfg.RootDir, path)
	if err != nil {
		return err
	}

	// Enforce the size limit up-front so huge files are never read into
	// memory (and never indexed).
	maxSize := idx.cfg.MaxFileSize
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024 // 10MB default
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("read file: %q is a directory", path)
	}
	if info.Size() > maxSize {
		return fmt.Errorf("file too large: %d bytes (max %d)", info.Size(), maxSize)
	}

	// Read file
	content, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	// Check if file has changed
	hash := computeHash(string(content))
	idx.mu.RLock()
	oldHash, exists := idx.docHashCache[absPath]
	idx.mu.RUnlock()
	if exists && oldHash == hash {
		return nil // unchanged
	}

	docID := uuid.New().String()

	// Phase 1: Parse
	idx.reportProgress(0, 5, "parse", nil)
	doc, err := idx.mdParser.Parse(absPath, string(content))
	if err != nil {
		idx.recordError(absPath, err, "parse")
		return fmt.Errorf("parse: %w", err)
	}

	// Phase 2: Chunk
	idx.reportProgress(1, 5, "chunk", nil)
	chunks, err := idx.chunker.ChunkDocument(doc, docID)
	if err != nil {
		idx.recordError(absPath, err, "chunk")
		return fmt.Errorf("chunk: %w", err)
	}

	// Phase 3: Embed
	idx.reportProgress(2, 5, "embed", nil)
	vectors, missingVectors, err := idx.embedChunksWithMissing(ctx, chunks)
	if err != nil {
		idx.recordError(absPath, err, "embed")
		return fmt.Errorf("embed: %w", err)
	}
	if missingVectors > 0 {
		log.Warn().Str("path", absPath).Int("missing_vectors", missingVectors).
			Bool("require_embeddings", idx.cfg.RequireEmbeddings).
			Msg("document indexed without complete embeddings; FTS remains available")
	}

	// Phase 4: Store in SQLite
	idx.reportProgress(3, 5, "store", nil)
	if err := idx.storeDocument(docID, absPath, hash, doc, chunks, vectors, extraMeta); err != nil {
		idx.recordError(absPath, err, "store")
		return fmt.Errorf("store: %w", err)
	}

	// Phase 5: Build Graph
	if idx.cfg.BuildGraph {
		idx.reportProgress(4, 5, "graph", nil)
		if err := idx.graphBuilder.BuildFromDocument(doc, idx.entityParser); err != nil {
			idx.recordError(absPath, err, "graph")
			log.Warn().Err(err).Str("path", absPath).Msg("graph build failed, continuing")
		}

		if len(chunks) > 0 {
			if err := idx.graphBuilder.BuildFromChunks(chunks, docID, absPath); err != nil {
				log.Warn().Err(err).Str("path", absPath).Msg("chunk graph build failed")
			}
		}
	}

	// Update cache
	idx.mu.Lock()
	idx.docHashCache[absPath] = hash
	idx.stats.TotalDocuments++
	idx.stats.TotalChunks += len(chunks)
	idx.stats.TotalVectors += len(vectors)
	idx.stats.MissingVectors += missingVectors
	docType := "markdown"
	if t, ok := doc.Frontmatter.Data["type"]; ok {
		if ts, ok := t.(string); ok && ts != "" {
			docType = ts
		}
	}
	idx.stats.DocumentsByType[docType]++
	idx.stats.LastIndexed = time.Now()
	idx.mu.Unlock()

	idx.reportProgress(5, 5, "complete", nil)

	log.Debug().
		Str("path", absPath).
		Int("chunks", len(chunks)).
		Str("doc_id", docID[:8]).
		Msg("document indexed")

	return nil
}

// IndexDirectory recursively indexes all supported files in a directory.
func (idx *Indexer) IndexDirectory(ctx context.Context, dir string) error {
	start := time.Now()

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve directory: %w", err)
	}

	// Collect all files
	var files []string
	err = filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Warn().Err(err).Str("path", path).Msg("walk error, skipping")
			return nil // continue on error
		}

		if info.IsDir() {
			// Skip hidden directories
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file should be indexed
		if idx.shouldIndex(path, info) {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("walk directory: %w", err)
	}

	sort.Strings(files)
	total := len(files)

	log.Info().Int("files", total).Str("dir", absDir).Msg("indexing directory")

	// Reset stats
	idx.mu.Lock()
	idx.stats = IndexStats{
		IndexedTypes:    make(map[string]int),
		DocumentsByType: make(map[string]int),
		LastIndexed:     time.Now(),
	}
	idx.mu.Unlock()

	var errCount int

	// ── Parallel indexing ─────────────────────────────────────────────────
	// Documents are independent: each IndexDocument holds its own locks
	// (docHashCache/stats via idx.mu, SQLite writes via db.LockWriter), so a
	// worker pool is safe. Parse/chunk/store run concurrently across workers
	// while the embedding provider serializes its own batch work — this hides
	// parse latency behind embedding latency.
	conc := idx.cfg.IndexConcurrency
	if conc <= 0 {
		conc = runtime.NumCPU()
		if conc > 8 {
			conc = 8 // cap: embedding provider + SQLite writer are the bottlenecks
		}
	}
	if conc < 1 {
		conc = 1
	}

	var (
		wg        sync.WaitGroup
		errMu     sync.Mutex
		doneCount int32
	)

	jobs := make(chan string, conc*2) // buffered: avoids sender stall when workers are busy
	for w := 0; w < conc; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range jobs {
				done := int(atomic.AddInt32(&doneCount, 1))
				if err := idx.IndexDocument(ctx, file); err != nil {
					errMu.Lock()
					errCount++
					errMu.Unlock()
					log.Warn().Err(err).Str("path", file).Msg("failed to index document")
					idx.reportProgress(done, total, "error", err)
				} else {
					idx.reportProgress(done, total, "indexing", nil)
				}
			}
		}()
	}

feed:
	for _, file := range files {
		select {
		case jobs <- file:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()

	duration := time.Since(start)

	idx.mu.Lock()
	idx.stats.Duration = duration
	idx.stats.TotalErrors = errCount
	idx.mu.Unlock()

	// No batch FTS rebuild here. The FTS indexes are kept current
	// per-document by storeDocument (incremental IndexDocument/IndexChunk/
	// IndexCodeBlock/RemoveDocument, E-007) — a full RebuildIndex over the
	// whole corpus at the end of every directory index was the dominant cost
	// of re-indexing (E-006). The intentional full rebuild remains available
	// via RebuildAll.

	log.Info().
		Int("total", total).
		Int("errors", errCount).
		Dur("duration", duration).
		Msg("directory indexing completed")

	if idx.cfg.RequireEmbeddings && errCount > 0 {
		return fmt.Errorf("indexing incomplete: %d of %d files failed", errCount, total)
	}
	return nil
}

// IndexChanged re-indexes a single changed file (incremental update).
func (idx *Indexer) IndexChanged(ctx context.Context, path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	// Re-index. IndexDocument stages replacement atomically; removing the old
	// entry first would lose a valid index when parsing or embedding fails.
	return idx.IndexDocument(ctx, absPath)
}

// IndexRemoved removes a file from the index.
func (idx *Indexer) IndexRemoved(_ context.Context, path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	// Find document by path
	var docID string
	err = idx.db.QueryRow("SELECT id FROM documents WHERE path = ?", absPath).Scan(&docID)
	if err != nil {
		return err
	}

	// Remove from vector store
	if err := idx.vecStore.DeleteByDocument(docID); err != nil {
		log.Warn().Err(err).Str("doc_id", docID).Msg("failed to delete vectors")
	}

	// Remove from FTS
	if err := idx.fts.RemoveDocument(docID); err != nil {
		log.Warn().Err(err).Str("doc_id", docID).Msg("failed to remove from FTS")
	}

	// Remove from SQLite
	_, err = idx.db.Exec("DELETE FROM documents WHERE id = ?", docID)
	if err != nil {
		return fmt.Errorf("delete document: %w", err)
	}

	// Remove from graph
	if idx.cfg.BuildGraph {
		if err := idx.graphBuilder.RemoveDocument(absPath); err != nil {
			log.Warn().Err(err).Str("path", absPath).Msg("failed to remove from graph")
		}
	}

	// Remove from hash cache
	idx.mu.Lock()
	delete(idx.docHashCache, absPath)
	idx.mu.Unlock()

	log.Debug().Str("path", absPath).Str("doc_id", docID[:8]).Msg("document removed from index")
	return nil
}

// RebuildAll performs a full rebuild: clear everything and re-index.
func (idx *Indexer) RebuildAll(ctx context.Context, rootDir string) error {
	log.Info().Msg("rebuilding all indexes")

	// Clear graph
	if idx.cfg.BuildGraph {
		idx.graphBuilder.Clear()
	}

	// Clear vector store
	if err := idx.vecStore.Rebuild(); err != nil {
		return fmt.Errorf("rebuild vector store: %w", err)
	}

	// Rebuild FTS
	if err := idx.fts.RebuildIndex(); err != nil {
		return fmt.Errorf("rebuild FTS: %w", err)
	}

	// Clear hash cache
	idx.mu.Lock()
	idx.docHashCache = make(map[string]string)
	idx.mu.Unlock()

	// Re-index
	return idx.IndexDirectory(ctx, rootDir)
}

// GetIndexStats returns current indexing statistics.
func (idx *Indexer) GetIndexStats() IndexStats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.stats
}

// GetErrors returns all recorded indexing errors.
func (idx *Indexer) GetErrors() []IndexError {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	errors := make([]IndexError, len(idx.errors))
	copy(errors, idx.errors)
	return errors
}

// ── Internal pipeline steps ────────────────────────────────────────────

func (idx *Indexer) embedChunks(ctx context.Context, chunks []chunker.Chunk) ([]vector.VectorRecord, error) {
	vectors, _, err := idx.embedChunksWithMissing(ctx, chunks)
	return vectors, err
}

// embedChunksWithMissing returns vectors and the number of chunks that could
// not be embedded. Provider/generation failures are degradable by default so
// FTS indexing can proceed; RequireEmbeddings turns the same condition into a
// terminal error. Validation failures remain terminal in both modes.
// embedChunksWithMissing returns vectors and the number of chunks that could
// not be embedded. Provider/generation failures are degradable by default so
// FTS indexing can proceed; RequireEmbeddings turns the same condition into a
// terminal error. Validation failures remain terminal in both modes.
//
// DEDUP PRÉ-EMBED (campanha 2026-08-21): antes de chamar o provider, cada
// chunk é consultado no estoque por conteúdo (chunks.content). Se um chunk
// com o MESMO conteúdo já possui vetor armazenado, o vetor existente é
// REUTILIZADO (lido do banco) e o provider NÃO é chamado para esse chunk —
// a chamada ao provider é economizada e o resultado final é idêntico (o
// embedding de um texto é determinístico para o mesmo modelo). Apenas chunks
// SEM vetor existente são enviados ao provider. A deduplicação é feita por
// content exato (nunca por similaridade) para não alterar a semântica.
func (idx *Indexer) embedChunksWithMissing(ctx context.Context, chunks []chunker.Chunk) ([]vector.VectorRecord, int, error) {
	if idx.embRegistry == nil {
		return idx.embeddingFailure("no embedding provider available", len(chunks))
	}

	if len(chunks) == 0 {
		return nil, 0, nil
	}

	// Fase A — dedup pré-embed: identifica chunks cujo conteúdo já tem vetor
	// no estoque e reutiliza esses vetores, economizando chamadas ao provider.
	vectors := make([]vector.VectorRecord, 0, len(chunks))
	reused := 0
	toEmbed := make([]chunker.Chunk, 0, len(chunks))
	toEmbedIdx := make([]int, 0, len(chunks))
	reusedVectors := make(map[int]vector.VectorRecord, len(chunks))

	for i, chunk := range chunks {
		existing, ok, err := idx.existingVectorForContent(ctx, chunk.Content)
		if err != nil {
			// Falha na consulta de dedup não pode derrubar a indexação: degrada
			// para o caminho antigo (embed direto) preservando o comportamento.
			idx.mu.Lock()
			idx.stats.TotalErrors++
			idx.mu.Unlock()
			log.Warn().Err(err).Str("path", chunk.DocumentID).Msg("dedup pre-embed lookup failed, embedding directly")
			toEmbed = append(toEmbed, chunk)
			toEmbedIdx = append(toEmbedIdx, i)
			continue
		}
		if ok {
			// Reutiliza APENAS o vetor (float64[]) do canônico. O VectorRecord
			// é reconstruído com os metadados/IDs do chunk ATUAL — nunca com os
			// do chunk canônico — para que o storeDocument grave este vetor
			// sob o chunk novo (o embedding é determinístico: mesmo content,
			// mesmo modelo → mesmo vetor; o do canônico é idêntico ao que o
			// provider geraria aqui).
			reused++
			metadata := map[string]string{
				"section_type": chunk.SectionType,
				"heading":      chunk.Heading,
				"position":     fmt.Sprintf("%d", chunk.Position),
			}
			for k, v := range chunk.Metadata {
				metadata[k] = v
			}
			reusedVectors[i] = vector.VectorRecord{
				ID:         chunk.ID,
				Vector:     existing.Vector,
				Metadata:   metadata,
				DocumentID: chunk.DocumentID,
				ChunkID:    chunk.ID,
				Content:    chunk.Content,
			}
			continue
		}
		toEmbed = append(toEmbed, chunk)
		toEmbedIdx = append(toEmbedIdx, i)
	}

	// Fase B — gera embeddings APENAS para chunks sem vetor existente.
	if len(toEmbed) > 0 {
		texts := make([]string, len(toEmbed))
		for i, chunk := range toEmbed {
			texts[i] = chunk.Content
		}

		results, err := idx.embRegistry.GenerateEmbeddings(ctx, texts)
		if err != nil {
			return idx.embeddingFailure(fmt.Sprintf("generate embeddings: %v", err), len(chunks))
		}
		if len(results) != len(toEmbed) {
			return nil, 0, fmt.Errorf("embedding result count %d does not match chunk count %d", len(results), len(toEmbed))
		}
		dimension := idx.embRegistry.Dimensions()
		if dimension <= 0 {
			return nil, 0, fmt.Errorf("embedding dimension is required")
		}

		for j, result := range results {
			if err := embeddings.ValidateEmbedding(result); err != nil {
				return nil, 0, fmt.Errorf("chunk %d: %w", j, err)
			}
			if result.Dimensions != dimension {
				return nil, 0, fmt.Errorf("chunk %d embedding dimension %d does not match provider dimension %d", j, result.Dimensions, dimension)
			}

			chunk := toEmbed[j]
			metadata := map[string]string{
				"section_type": chunk.SectionType,
				"heading":      chunk.Heading,
				"position":     fmt.Sprintf("%d", chunk.Position),
			}
			for k, v := range chunk.Metadata {
				metadata[k] = v
			}

			reusedVectors[toEmbedIdx[j]] = vector.VectorRecord{
				ID:         chunk.ID,
				Vector:     result.Vector,
				Metadata:   metadata,
				DocumentID: chunk.DocumentID,
				ChunkID:    chunk.ID,
				Content:    chunk.Content,
			}
		}
	}

	// Fase C — recompoe os vetores na ordem original dos chunks.
	for i := range chunks {
		if v, ok := reusedVectors[i]; ok {
			vectors = append(vectors, v)
		}
	}

	if reused > 0 {
		log.Info().
			Int("total", len(chunks)).
			Int("reused", reused).
			Int("generated", len(chunks)-reused).
			Msg("dedup pré-embed: vetores reutilizados do estoque")
	}

	return vectors, 0, nil
}

// existingVectorForContent consulta o estoque por conteúdo e devolve o vetor
// já armazenado para um chunk com o MESMO content (se existir). A leitura é
// apenas de metadados/vetor — nada é alterado. Ok=false quando nenhum chunk
// com esse conteúdo possui vetor.
func (idx *Indexer) existingVectorForContent(ctx context.Context, content string) (vector.VectorRecord, bool, error) {
	if idx.db == nil {
		return vector.VectorRecord{}, false, nil
	}

	var (
		blob       []byte
		chunkID    string
		documentID string
	)
	// JOIN: um chunk com o mesmo conteúdo que JÁ tem vetor armazenado.
	err := idx.db.QueryRow(`
		SELECT v.vector, v.chunk_id, v.document_id
		FROM vectors v
		JOIN chunks c ON c.id = v.chunk_id
		WHERE c.content = ?
		LIMIT 1`, content).Scan(&blob, &chunkID, &documentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return vector.VectorRecord{}, false, nil
		}
		return vector.VectorRecord{}, false, err
	}

	vec := vector.BytesToFloat64Slice(blob)
	if len(vec) == 0 {
		return vector.VectorRecord{}, false, nil
	}

	return vector.VectorRecord{
		Vector:     vec,
		ChunkID:    chunkID,
		DocumentID: documentID,
	}, true, nil
}

// chunksHaveDedupCols verifica (PRAGMA) se a tabela chunks tem a coluna
// dedup_of (migração da limpeza L360 / schema novo). Fail-safe: sem a
// coluna, o comportamento histórico é preservado (sem dedup preventivo).
func (idx *Indexer) chunksHaveDedupCols(q interface {
	QueryRow(query string, args ...interface{}) *sql.Row
}) bool {
	var name string
	err := q.QueryRow("SELECT name FROM pragma_table_info('chunks') WHERE name = 'dedup_of'").Scan(&name)
	return err == nil && name == "dedup_of"
}

func (idx *Indexer) embeddingFailure(reason string, missing int) ([]vector.VectorRecord, int, error) {
	if idx.cfg.RequireEmbeddings {
		return nil, 0, fmt.Errorf("embeddings required: %s", reason)
	}
	return nil, missing, nil
}

func (idx *Indexer) storeDocument(docID, path, hash string, doc *markdown.Document, chunks []chunker.Chunk, vectors []vector.VectorRecord, extraMeta map[string]any) error {
	// SQLite permits one writer. Serialize at the DB wrapper (rather than on
	// the Indexer) so multiple indexers sharing one database coordinate too.
	unlockWriter := idx.db.LockWriter()
	defer unlockWriter()

	if len(vectors) > 0 {
		if err := idx.vecStoreDimensionCheck(vectors); err != nil {
			return err
		}
	}

	// Replacements are deliberately staged in one SQLite transaction.  The
	// old row is removed only as part of that transaction, and the transaction
	// is not committed until the new vectors have been accepted.  This is
	// important: deleting the old document first used to turn an embedding or
	// insert failure into data loss.
	var oldID string
	err := idx.db.QueryRow("SELECT id FROM documents WHERE path = ?", path).Scan(&oldID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("find existing document: %w", err)
	}

	// Incremental FTS (E-007): a re-indexed document must never leave stale
	// entries behind. The old document's FTS entries are removed BEFORE the
	// replacement transaction (the new rows are only inserted below), so an
	// UPDATE is remove-then-insert instead of an append.
	if oldID != "" && idx.fts != nil {
		if err := idx.fts.RemoveDocument(oldID); err != nil {
			return fmt.Errorf("remove old FTS entries: %w", err)
		}
	}

	tx, err := idx.db.Begin()
	if err != nil {
		return fmt.Errorf("begin document replacement: %w", err)
	}
	rollback := func(cause error) error {
		_ = tx.Rollback()
		if len(vectors) > 0 && idx.vecStore != nil {
			// Best effort: the database rollback is authoritative, while this
			// prevents staged vectors from becoming orphaned.
			_ = idx.vecStore.DeleteByDocument(docID)
		}
		return cause
	}

	if oldID != "" {
		for _, table := range []string{"chunks", "headings", "code_blocks", "tables"} {
			if _, err := tx.Exec("DELETE FROM "+table+" WHERE document_id = ?", oldID); err != nil {
				return rollback(fmt.Errorf("remove old %s: %w", table, err))
			}
		}
		if _, err := tx.Exec("DELETE FROM documents WHERE id = ?", oldID); err != nil {
			return rollback(fmt.Errorf("remove old document: %w", err))
		}
	}
	// Insert document
	frontmatterJSON := "{}"
	if doc.Frontmatter.Data != nil {
		if data, err := toJSON(doc.Frontmatter.Data); err == nil {
			frontmatterJSON = data
		}
	}

	metadataJSON := "{}"
	// Build metadata from frontmatter. extraMeta (proveniência semântica
	// calculada pelo chamador — scope/project/origin/kind/agent) é MESCLADO,
	// preservando o que o indexer já produz (title/path/type/headings/links).
	meta := map[string]interface{}{
		"title":    doc.Title,
		"path":     path,
		"type":     "markdown",
		"headings": len(doc.Headings),
		"links":    len(doc.Links),
	}
	for k, v := range extraMeta {
		meta[k] = v
	}
	if data, err := toJSON(meta); err == nil {
		metadataJSON = data
	}

	_, err = tx.Exec(
		`INSERT INTO documents (id, path, hash, title, doc_type, metadata_json, frontmatter_json, size, token_count, created_at, updated_at, tier, expires_at)
		 VALUES (?, ?, ?, ?, 'markdown', ?, ?, ?, ?, datetime('now'), datetime('now'), 'medium', datetime('now', '+7 days'))`,
		docID, path, hash, doc.Title, metadataJSON, frontmatterJSON, len(doc.RawContent), doc.TokenCount,
	)
	if err != nil {
		return rollback(fmt.Errorf("insert document: %w", err))
	}

	// Insert chunks — com dedup preventivo (L371): conteúdo já existente no
	// estoque (com vetor) é marcado como dedup_of=<canonico> e NÃO ganha vetor
	// (o estoque limpo não re-suja; proveniência preservada). Fail-safe: se a
	// coluna dedup_of não existe (db antigo sem migração), o comportamento
	// histórico é preservado (INSERT sem a coluna, sem dedup).
	hasDedupCols := idx.chunksHaveDedupCols(tx)
	dedupCache := map[string]string{} // chunk.Hash -> canonical chunk id
	dedupChunkIDs := map[string]bool{}
	for _, chunk := range chunks {
		canon := ""
		if hasDedupCols {
			if c, ok := dedupCache[chunk.Hash]; ok {
				canon = c
			} else {
				err := tx.QueryRow(
					`SELECT id FROM chunks WHERE content = ? AND EXISTS
					 (SELECT 1 FROM vectors v WHERE v.chunk_id = chunks.id) LIMIT 1`,
					chunk.Content,
				).Scan(&canon)
				if err == sql.ErrNoRows {
					canon = ""
				} else if err != nil {
					return rollback(fmt.Errorf("dedup check chunk %s: %w", chunk.ID, err))
				}
				dedupCache[chunk.Hash] = canon
			}
			if canon != "" {
				dedupChunkIDs[chunk.ID] = true
			}
		}
		var err error
		if hasDedupCols {
			_, err = tx.Exec(
				`INSERT OR REPLACE INTO chunks (id, document_id, content, heading, section_type, position, hash, token_count, metadata_json, dedup_of)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				chunk.ID, chunk.DocumentID, chunk.Content, chunk.Heading, chunk.SectionType,
				chunk.Position, chunk.Hash, chunk.TokenCount, "{}", canon,
			)
		} else {
			_, err = tx.Exec(
				`INSERT OR REPLACE INTO chunks (id, document_id, content, heading, section_type, position, hash, token_count, metadata_json)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				chunk.ID, chunk.DocumentID, chunk.Content, chunk.Heading, chunk.SectionType,
				chunk.Position, chunk.Hash, chunk.TokenCount, "{}",
			)
		}
		if err != nil {
			return rollback(fmt.Errorf("insert chunk %s: %w", chunk.ID, err))
		}
	}

	// Remove os vetores dos chunks deduplicados (marcados, sem vetor).
	if len(dedupChunkIDs) > 0 {
		kept := vectors[:0]
		for _, v := range vectors {
			if !dedupChunkIDs[v.ChunkID] {
				kept = append(kept, v)
			}
		}
		vectors = kept
	}

	// Store vectors
	if len(vectors) > 0 {
		transactionalStore, ok := idx.vecStore.(vector.TransactionalStore)
		if !ok {
			return rollback(fmt.Errorf("vector store does not support transactional storage"))
		}
		if err := transactionalStore.StoreTx(tx, len(vectors[0].Vector), vectors); err != nil {
			return rollback(fmt.Errorf("store vectors: %w", err))
		}
	}

	// Index headings
	for _, h := range doc.Headings {
		hid := uuid.New().String()
		_, err := tx.Exec(
			`INSERT OR REPLACE INTO headings (id, document_id, level, text, position)
			 VALUES (?, ?, ?, ?, ?)`,
			hid, docID, h.Level, h.Text, h.Position,
		)
		if err != nil {
			return rollback(fmt.Errorf("insert heading: %w", err))
		}
	}

	// Index code blocks
	var codeBlockIDs []string
	if idx.cfg.IndexCodeBlocks {
		for _, cb := range doc.CodeBlocks {
			cbid := uuid.New().String()
			codeBlockIDs = append(codeBlockIDs, cbid)
			_, err := tx.Exec(
				`INSERT OR REPLACE INTO code_blocks (id, document_id, language, content, position, token_count)
				 VALUES (?, ?, ?, ?, ?, ?)`,
				cbid, docID, cb.Language, cb.Content, cb.Position, cb.TokenCount,
			)
			if err != nil {
				return rollback(fmt.Errorf("insert code block: %w", err))
			}
		}
	}

	// Index tables
	if idx.cfg.IndexTables {
		for _, t := range doc.Tables {
			tid := uuid.New().String()
			headersJSON, _ := toJSON(t.Headers)
			rowsJSON, _ := toJSON(t.Rows)
			_, err := tx.Exec(
				`INSERT OR REPLACE INTO tables (id, document_id, caption, headers_json, rows_json, position)
				 VALUES (?, ?, ?, ?, ?, ?)`,
				tid, docID, t.Caption, headersJSON, rowsJSON, t.Position,
			)
			if err != nil {
				return rollback(fmt.Errorf("insert table: %w", err))
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return rollback(fmt.Errorf("commit document replacement: %w", err))
	}

	// Incremental FTS (E-007): index the freshly stored document immediately.
	// The per-document incremental functions replace the batch RebuildIndex
	// that IndexDirectory used to run at the end — the FTS must be current
	// without any full rebuild. Errors propagate (fail-closed): a document is
	// never silently indexed without its FTS entries.
	if idx.fts != nil {
		if err := idx.indexDocumentFTS(docID, chunks, codeBlockIDs); err != nil {
			return err
		}
	}

	// The staged StoreTx vectors committed NOW. Stores that keep derived
	// in-memory state (the vector full-scan index) must refresh against this
	// committed state — notify them so a search can never bake a pre-commit
	// snapshot as final (see vector.StoreTxCommitter).
	if committer, ok := idx.vecStore.(vector.StoreTxCommitter); ok {
		committer.StoreTxCommitted()
	}

	// External indexes are cleaned only after the new database state is safe.
	// Rebuilding FTS from the committed source tables also avoids relying on
	// child rows that the transaction has already removed.
	// Delete vectors belonging to the replaced document independently of the
	// new vector set. In FTS-only/degraded replacements vectors is empty, but
	// stale vectors for oldID must still be removed after the transaction is
	// committed. Keeping this outside the transaction preserves rollback of
	// the authoritative SQLite replacement.
	if oldID != "" {
		if err := idx.deleteReplacedVectors(oldID); err != nil {
			log.Warn().Err(err).Str("doc_id", oldID).Msg("failed to remove replaced vectors")
		}
	}
	// FTS rebuild is deferred to batch at end of IndexDirectory.
	// Per-document rebuild is O(n²) — see AUTO-EVOLUTION L170.
	if oldID != "" && idx.cfg.BuildGraph && idx.graphBuilder != nil {
		if err := idx.graphBuilder.RemoveDocument(path); err != nil {
			log.Warn().Err(err).Str("path", path).Msg("failed to remove replaced graph document")
		}
	}

	return nil
}

func (idx *Indexer) deleteReplacedVectors(oldID string) error {
	if idx.vecStore == nil {
		return nil
	}
	return idx.vecStore.DeleteByDocument(oldID)
}

// indexDocumentFTS writes the freshly stored document's entries into the FTS
// indexes using the incremental functions (E-007). The FTS indexes are keyed
// by the integer rowid of the content tables (documents/chunks/code_blocks),
// never by the string uuid, so each rowid is looked up before inserting.
func (idx *Indexer) indexDocumentFTS(docID string, chunks []chunker.Chunk, codeBlockIDs []string) error {
	// Document entry.
	var docRowID int64
	var docTitle, docType string
	if err := idx.db.QueryRow(
		"SELECT rowid, title, doc_type FROM documents WHERE id = ?", docID,
	).Scan(&docRowID, &docTitle, &docType); err != nil {
		return fmt.Errorf("lookup document rowid: %w", err)
	}
	if err := idx.fts.IndexDocument(strconv.FormatInt(docRowID, 10), docTitle, "", docType); err != nil {
		return fmt.Errorf("index document FTS: %w", err)
	}

	// Chunk entries.
	for _, chunk := range chunks {
		var rowID int64
		if err := idx.db.QueryRow("SELECT rowid FROM chunks WHERE id = ?", chunk.ID).Scan(&rowID); err != nil {
			return fmt.Errorf("lookup chunk rowid %s: %w", chunk.ID, err)
		}
		if err := idx.fts.IndexChunk(strconv.FormatInt(rowID, 10), chunk.Content, chunk.Heading, chunk.SectionType); err != nil {
			return fmt.Errorf("index chunk FTS %s: %w", chunk.ID, err)
		}
	}

	// Code block entries (mirrors idx.cfg.IndexCodeBlocks in storeDocument).
	if idx.cfg.IndexCodeBlocks {
		for _, cbid := range codeBlockIDs {
			var rowID int64
			var content, language string
			if err := idx.db.QueryRow(
				"SELECT rowid, content, language FROM code_blocks WHERE id = ?", cbid,
			).Scan(&rowID, &content, &language); err != nil {
				return fmt.Errorf("lookup code block rowid %s: %w", cbid, err)
			}
			if err := idx.fts.IndexCodeBlock(strconv.FormatInt(rowID, 10), content, language); err != nil {
				return fmt.Errorf("index code block FTS %s: %w", cbid, err)
			}
		}
	}

	return nil
}

func (idx *Indexer) vecStoreDimensionCheck(vectors []vector.VectorRecord) error {
	dimension := len(vectors[0].Vector)
	if dimension == 0 {
		return fmt.Errorf("vector dimension is required")
	}
	for i, v := range vectors {
		if len(v.Vector) != dimension {
			return fmt.Errorf("vector %d dimension %d does not match %d", i, len(v.Vector), dimension)
		}
	}
	return nil
}

func (idx *Indexer) removeExistingDocument(path string) error {
	var oldID string
	err := idx.db.QueryRow("SELECT id FROM documents WHERE path = ?", path).Scan(&oldID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if idx.vecStore != nil {
		if err := idx.vecStore.DeleteByDocument(oldID); err != nil {
			return err
		}
	}
	if idx.fts != nil {
		if err := idx.fts.RemoveDocument(oldID); err != nil {
			return err
		}
	}
	if idx.cfg.BuildGraph && idx.graphBuilder != nil {
		if err := idx.graphBuilder.RemoveDocument(path); err != nil {
			return err
		}
	}
	for _, table := range []string{"chunks", "headings", "code_blocks", "tables"} {
		if _, err := idx.db.Exec("DELETE FROM "+table+" WHERE document_id = ?", oldID); err != nil {
			return err
		}
	}
	_, err = idx.db.Exec("DELETE FROM documents WHERE id = ?", oldID)
	return err
}

// ── Helpers ────────────────────────────────────────────────────────────────

func (idx *Indexer) shouldIndex(path string, info os.FileInfo) bool {
	// Check size
	if info.Size() > idx.cfg.MaxFileSize {
		return false
	}

	// Check extension
	if len(idx.cfg.AllowedExtensions) > 0 {
		ext := strings.ToLower(filepath.Ext(path))
		allowed := false
		for _, allowedExt := range idx.cfg.AllowedExtensions {
			if ext == allowedExt || ext == "."+allowedExt {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}

	// Check excluded patterns
	for _, pattern := range idx.cfg.ExcludedPatterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return false
		}
	}

	return true
}

func (idx *Indexer) reportProgress(current, total int, phase string, err error) {
	idx.mu.RLock()
	cb := idx.progressCb
	idx.mu.RUnlock()

	if cb != nil {
		cb(current, total, phase, err)
	}
}

func (idx *Indexer) recordError(path string, err error, phase string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.errors = append(idx.errors, IndexError{
		Path:      path,
		Error:     err.Error(),
		Phase:     phase,
		Timestamp: time.Now(),
	})
}

func computeHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

func toJSON(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}", err
	}
	return string(data), nil
}

// ── Path containment (C1: path traversal prevention) ────────────────────

// validatePathWithin resolves target to an absolute, symlink-resolved path
// and verifies that it is equal to or a descendant of rootDir. It returns the
// resolved absolute path on success.
//
// The pattern mirrors the sandbox containment implemented in
// internal/orchestration/tool_exec.go (resolveAndValidate + isPathWithin):
// EvalSymlinks + filepath.Rel. This is the authoritative containment check
// for the knowledge indexer — callers (REST handlers) must never be able to
// index files outside RootDir and later retrieve them via knowledge search.
func validatePathWithin(rootDir, target string) (string, error) {
	root := rootDir
	if root == "" {
		root = "."
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve root directory: %w", err)
	}
	rootAbs = filepath.Clean(rootAbs)

	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	targetAbs = filepath.Clean(targetAbs)

	// Resolve symlinks so symlink tricks cannot escape the root. If the
	// target does not exist, fall back to the cleaned absolute path and
	// validate it lexically.
	resolved := targetAbs
	if r, err := filepath.EvalSymlinks(targetAbs); err == nil {
		resolved = r
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	if !isPathWithin(rootAbs, resolved) {
		return "", fmt.Errorf("path outside root directory: %s", target)
	}
	return resolved, nil
}

// isPathWithin reports whether child is equal to or a descendant of parent.
func isPathWithin(parent, child string) bool {
	rel, err := filepath.Rel(filepath.Clean(parent), filepath.Clean(child))
	if err != nil {
		return false
	}
	// Only ".." (exact) or "../…" escape the root; a prefix like "..foo"
	// is a legitimate sibling name.
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
