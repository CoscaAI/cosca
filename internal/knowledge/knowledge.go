// Package knowledge provides the main Knowledge Engine for the Cosca Enterprise Platform.
// It orchestrates indexing, search, graph construction, caching, and lifecycle management.
package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/internal/cache"
	"github.com/CoscaAI/cosca/internal/chunker"
	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/CoscaAI/cosca/internal/graph"
	"github.com/CoscaAI/cosca/internal/indexer"
	"github.com/CoscaAI/cosca/internal/markdown"
	"github.com/CoscaAI/cosca/internal/parser"
	"github.com/CoscaAI/cosca/internal/ranking"
	"github.com/CoscaAI/cosca/internal/search"
	"github.com/CoscaAI/cosca/internal/sqlite"
	"github.com/CoscaAI/cosca/internal/vector"
	"github.com/CoscaAI/cosca/internal/watcher"
)

// Engine is the central knowledge engine that coordinates all subsystems.
type Engine struct {
	cfg    Config
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	// Core subsystems
	db           *sqlite.DB
	fts          *sqlite.FTSClient
	mdParser     *markdown.Parser
	entityParser *parser.EntityParser
	chunker      *chunker.Chunker
	embRegistry  *embeddings.ProviderRegistry
	vecStore     vector.Store
	graph        *graph.Graph
	graphBuilder *graph.Builder
	indexer      *indexer.Indexer
	search       *search.Engine
	ranker       *ranking.Ranker
	cache        *cache.Cache

	// File watcher
	fileWatcher *watcher.Watcher

	// State
	initialized bool
	closed      bool
	startTime   time.Time
}

// Config defines the knowledge engine configuration.
type Config struct {
	// DBPath is the path to the SQLite database file.
	DBPath string

	// RootDir is the root directory for relative path resolution.
	RootDir string

	// AutoMigrate enables automatic database migration on startup.
	AutoMigrate bool

	// WatchEnabled enables filesystem watching for automatic re-indexing.
	WatchEnabled bool

	// EmbeddingProvider is the name of the embedding provider to use.
	EmbeddingProvider string

	// EmbeddingBaseURL optionally points the embedding provider at a custom
	// base URL (e.g. a local OpenAI-compatible embeddings server). Empty means
	// the provider's default endpoint. Requires an explicit EmbeddingProvider
	// to take effect.
	EmbeddingBaseURL string

	// EmbeddingModel optionally overrides the embedding provider's model name.
	EmbeddingModel string

	// EmbeddingDigest pina o digest esperado do modelo (L376): quando
	// definido, o provider precisa provar a identidade antes de operar —
	// fail-closed contra mudança silenciosa do modelo "latest".
	EmbeddingDigest string

	// EmbeddingAPIKey optionally overrides the embedding provider's API key.
	EmbeddingAPIKey string

	// EmbeddingDimensions optionally overrides the embedding provider's output
	// dimensions. 0 means the provider default.
	EmbeddingDimensions int

	// EmbeddingConfig is the raw configuration map for the embedding provider.
	EmbeddingConfig map[string]interface{}

	// IndexerConfig is the configuration for the indexer subsystem.
	IndexerConfig indexer.IndexerConfig

	// RequireEmbeddings makes semantic indexing fail instead of completing
	// with FTS-only documents when embeddings are unavailable.
	RequireEmbeddings bool

	// CacheConfig is the configuration for the multi-level cache.
	CacheConfig cache.Config

	// RankingConfig is the configuration for the ranking subsystem.
	RankingConfig ranking.Config

	// SearchConfig provides default search parameters.
	SearchConfig search.SearchParams
}

// DefaultConfig returns sensible defaults for the knowledge engine.
func DefaultConfig() Config {
	return Config{
		DBPath:            filepath.Join(os.Getenv("HOME"), ".cosca", "knowledge.db"),
		RootDir:           ".",
		AutoMigrate:       true,
		WatchEnabled:      false,
		EmbeddingProvider: "auto",
		IndexerConfig:     indexer.DefaultConfig(),
		CacheConfig:       cache.DefaultConfig(),
		RankingConfig:     ranking.DefaultConfig(),
	}
}

// embeddingSelectionConfig builds the embedding registry selection config from
// the engine config, carrying the explicit provider overrides (base URL, model,
// API key, dimensions). When no override is set the returned config matches
// DefaultProviderRegistryConfig exactly, preserving the historical behavior.
func (e *Engine) embeddingSelectionConfig() embeddings.ProviderRegistryConfig {
	selCfg := embeddings.DefaultProviderRegistryConfig()
	selCfg.BaseURL = e.cfg.EmbeddingBaseURL
	selCfg.Model = e.cfg.EmbeddingModel
	selCfg.APIKey = e.cfg.EmbeddingAPIKey
	selCfg.Digest = e.cfg.EmbeddingDigest
	if e.cfg.EmbeddingDimensions > 0 {
		selCfg.Dimensions = e.cfg.EmbeddingDimensions
	}
	return selCfg
}

// Stats provides comprehensive knowledge engine statistics.
type Stats struct {
	Uptime         time.Duration              `json:"uptime"`
	DocumentCount  int                        `json:"document_count"`
	ChunkCount     int                        `json:"chunk_count"`
	EntityCount    int                        `json:"entity_count"`
	VectorCount    int                        `json:"vector_count"`
	GraphStats     graph.GraphStats           `json:"graph_stats"`
	IndexStats     indexer.IndexStats         `json:"index_stats"`
	EmbeddingStats *embeddings.EmbeddingStats `json:"embedding_stats"`
	CacheStats     map[string]int             `json:"cache_stats"`
	DBSize         int64                      `json:"db_size_bytes"`
	LastIndexed    time.Time                  `json:"last_indexed"`
}

// New creates a new knowledge engine.
func New(cfg Config) (*Engine, error) {
	ctx, cancel := context.WithCancel(context.Background())

	e := &Engine{
		cfg:       cfg,
		ctx:       ctx,
		cancel:    cancel,
		startTime: time.Now(),
	}

	// Ensure root dir exists. Owner-only permissions (0700): the DB dir is
	// the .cosca tree, which holds the knowledge base and must not be
	// world-readable (M6b).
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0700); err != nil {
		cancel()
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	if err := os.Chmod(filepath.Dir(cfg.DBPath), 0700); err != nil {
		cancel()
		return nil, fmt.Errorf("restrict data directory permissions: %w", err)
	}

	log.Info().
		Str("db_path", cfg.DBPath).
		Str("root_dir", cfg.RootDir).
		Msg("knowledge engine initializing")

	return e, nil
}

// Init initializes all subsystems and opens the database.
func (e *Engine) Init() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.initialized {
		return fmt.Errorf("knowledge engine already initialized")
	}

	// 1. Open SQLite database
	dbCfg := sqlite.DefaultConfig(e.cfg.DBPath)
	dbCfg.AutoMigrate = e.cfg.AutoMigrate
	db, err := sqlite.Open(dbCfg)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	e.db = db
	e.fts = sqlite.NewFTSClient(db)

	// 2. Create parsers
	e.mdParser = markdown.NewParser()
	e.entityParser = parser.NewEntityParser()

	// 3. Create chunker
	e.chunker = chunker.New(chunker.DefaultConfig())

	// 4. Initialize embedding provider registry
	e.embRegistry = embeddings.GetRegistry()
	if e.cfg.EmbeddingProvider != "" && e.cfg.EmbeddingProvider != "auto" {
		selCfg := e.embeddingSelectionConfig()
		selCfg.Primary = e.cfg.EmbeddingProvider
		if err := e.embRegistry.Select(context.Background(), selCfg); err != nil {
			// Fail-closed (L376): erro de IDENTIDADE (digest do modelo) é
			// falha de integridade — NUNCA cai em auto-detect/fallback.
			if errors.Is(err, embeddings.ErrEmbeddingIdentityMismatch) {
				return fmt.Errorf("embedding identity mismatch: %w", err)
			}
			log.Warn().Err(err).Msg("failed to select embedding provider, using auto-detection")
			selCfg.Primary = ""
			selCfg.AutoDetect = true
			if err := e.embRegistry.Select(context.Background(), selCfg); err != nil {
				log.Warn().Err(err).Msg("no embedding provider available, continuing without embeddings")
			}
		}
	} else {
		if e.cfg.EmbeddingBaseURL != "" {
			// Security hardening (P1): a base_url override only takes effect with
			// an explicit provider. Auto-detection could otherwise select a
			// provider that ignores the override and reach its default remote
			// endpoint. Fail loudly instead of silently.
			log.Warn().Msg("embedding.base_url is set but embedding.provider is auto/empty — the base_url override is IGNORED; set an explicit provider (e.g. embedding.provider: openai) to route embeddings to a custom endpoint")
		}
		selCfg := e.embeddingSelectionConfig()
		selCfg.AutoDetect = true
		if err := e.embRegistry.Select(context.Background(), selCfg); err != nil {
			log.Warn().Err(err).Msg("no embedding provider available, continuing without embeddings")
		}
	}

	// 5. Create vector store
	// The store dimension MUST match the embedding provider's output
	// dimension. Fall back to 768 (nomic-embed-text) when the provider
	// does not expose one.
	dim := 768
	if e.embRegistry != nil {
		if d := e.embRegistry.Dimensions(); d > 0 {
			dim = d
		}
	}
	vecCfg := vector.SQLiteVecConfig{
		DB:        db.Conn(),
		Dimension: dim,
	}
	vecStore, err := vector.NewSQLiteVec(vecCfg)
	if err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("failed to close database after vector store init failure")
		}
		return fmt.Errorf("create vector store: %w", err)
	}
	e.vecStore = vecStore

	// 6. Create graph
	e.graph = graph.New()
	e.graphBuilder = graph.NewBuilder(e.graph)

	// 7. Create ranker
	e.ranker = ranking.New(e.cfg.RankingConfig)

	// 8. Load existing graph from database before wiring downstream consumers
	// (search engine, indexer) so they receive the correct graph reference.
	if err := e.loadGraph(); err != nil {
		log.Warn().Err(err).Msg("failed to load graph from database")
	}

	// 9. Create search engine
	embedFunc := func(ctx context.Context, text string) (*search.EmbeddingRequest, error) {
		if e.embRegistry == nil {
			return nil, fmt.Errorf("no embedding provider")
		}
		result, err := e.embRegistry.GenerateEmbedding(ctx, text)
		if err != nil {
			return nil, err
		}
		return &search.EmbeddingRequest{Vector: result.Vector}, nil
	}

	e.search = search.NewEngine(e.fts, e.vecStore, e.graph, e.ranker, embedFunc)

	// 10. Create cache
	cacheCfg := e.cfg.CacheConfig
	// Aplica o default quando o chamador não configurou níveis (Config{}
	// vazio deixa EnabledLevels=nil → cache inerte SILENCIOSAMENTE — bug real:
	// buscas repetidas nunca batiam no cache). Um chamador que queira cache
	// desativado deve passar EnabledLevels: []cache.Level{} explícito.
	if len(cacheCfg.EnabledLevels) == 0 {
		def := cache.DefaultConfig()
		cacheCfg.EnabledLevels = def.EnabledLevels
		if cacheCfg.MemoryTTL <= 0 {
			cacheCfg.MemoryTTL = def.MemoryTTL
		}
	}
	cacheCfg.SQLiteDB = db.Conn()
	cacheCfg.SQLiteTable = "cache"
	c, err := cache.New(cacheCfg)
	if err != nil {
		log.Warn().Err(err).Msg("failed to initialize cache, continuing without")
	} else {
		e.cache = c
	}

	// 11. Create indexer (use defaults for unset IndexerConfig fields).
	idxCfg := indexer.DefaultConfig()
	idxCfg.RootDir = e.cfg.RootDir
	idxCfg.RequireEmbeddings = idxCfg.RequireEmbeddings || e.cfg.RequireEmbeddings
	if e.cfg.IndexerConfig.EmbedBatchSize > 0 {
		idxCfg.EmbedBatchSize = e.cfg.IndexerConfig.EmbedBatchSize
	}
	if e.cfg.IndexerConfig.MaxFileSize > 0 {
		idxCfg.MaxFileSize = e.cfg.IndexerConfig.MaxFileSize
	}
	if e.cfg.IndexerConfig.IndexConcurrency > 0 {
		idxCfg.IndexConcurrency = e.cfg.IndexerConfig.IndexConcurrency
	}
	if e.cfg.IndexerConfig.FollowSymlinks {
		idxCfg.FollowSymlinks = true
	}
	if len(e.cfg.IndexerConfig.ExcludedPatterns) > 0 {
		idxCfg.ExcludedPatterns = e.cfg.IndexerConfig.ExcludedPatterns
	}
	if len(e.cfg.IndexerConfig.AllowedExtensions) > 0 {
		idxCfg.AllowedExtensions = e.cfg.IndexerConfig.AllowedExtensions
	}
	e.indexer = indexer.New(
		idxCfg, e.mdParser, e.entityParser, e.chunker,
		e.embRegistry, e.vecStore, e.fts, e.db, e.graphBuilder,
	)

	e.initialized = true

	log.Info().
		Str("db", e.cfg.DBPath).
		Str("root", e.cfg.RootDir).
		Bool("watch", e.cfg.WatchEnabled).
		Msg("knowledge engine initialized")

	// 12. Check for orphan vectors on init. The repair itself is invoked
	// explicitly via RepairOrphanVectors() from the CLI (cosca knowledge
	// verify --fix) so the user controls when the potentially slow embedding
	// operation runs.
	if e.embRegistry != nil && e.vecStore != nil && e.db != nil {
		orphanCount := e.countOrphanVectors()
		if orphanCount > 0 {
			log.Warn().Int("orphans", orphanCount).Msg("orphan vectors detected — run 'cosca knowledge verify --fix' to repair")
		}
	}

	return nil
}

// countOrphanVectors returns the number of chunks that have no corresponding
// vector. This is a lightweight count query (no embedding generation).
func (e *Engine) countOrphanVectors() int {
	if e.db == nil {
		return 0
	}
	var count int
	err := e.db.QueryRow(e.orphanChunksQuery()).Scan(&count)
	if err != nil {
		log.Warn().Err(err).Msg("countOrphanVectors: query failed")
		return 0
	}
	return count
}

// orphanChunksQuery returns the SELECT for vectorless chunks. The campaign
// columns chunks.is_trivial / chunks.dedup_of only exist after the idempotent
// "migração leve" of the cleanup campaign (L360) runs; on fresh databases they
// are absent, so the query must not reference them. When the columns are
// missing every vectorless chunk is a genuine orphan (nothing was marked
// trivial/duplicated yet), so the filter is simply dropped.
func (e *Engine) orphanChunksQuery() string {
	q := "SELECT COUNT(*) FROM chunks c LEFT JOIN vectors v ON c.id = v.chunk_id WHERE v.chunk_id IS NULL"
	if e.chunksHaveCampaignColumns() {
		q += " AND c.is_trivial = 0 AND c.dedup_of = ''"
	}
	return q
}

// chunksHaveCampaignColumns reports whether the chunks table carries the
// optional cleanup-campaign columns (is_trivial, dedup_of).
func (e *Engine) chunksHaveCampaignColumns() bool {
	rows, err := e.db.Query("PRAGMA table_info(chunks)")
	if err != nil {
		return false
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false
		}
		if name == "is_trivial" || name == "dedup_of" {
			return true
		}
	}
	return false
}

// RepairOrphanVectors detects chunks in the database that were indexed
// without an active embedding provider (no corresponding vector) and
// re-embeds them using the current embedding provider.
//
// This is a potentially long-running operation (each chunk requires an
// embedding API call). Progress is reported via structured logging.
//
// The operation is best-effort: errors are logged but never fatal since
// FTS5 search continues to work without vectors.
func (e *Engine) RepairOrphanVectors() {
	if e.embRegistry == nil {
		log.Debug().Msg("RepairOrphanVectors: no embedding registry, skipping")
		return
	}
	if e.vecStore == nil {
		log.Debug().Msg("RepairOrphanVectors: no vector store, skipping")
		return
	}
	if e.db == nil {
		log.Debug().Msg("RepairOrphanVectors: no database, skipping")
		return
	}

	// Check if the embedding provider is actually usable.
	dim := e.embRegistry.Dimensions()
	if dim <= 0 {
		log.Debug().Msg("RepairOrphanVectors: embedding provider has no dimension, skipping")
		return
	}

	// Find chunks that have no corresponding vector. The campaign columns
	// (is_trivial/dedup_of) may not exist on fresh databases — see
	// orphanChunksQuery.
	q := "SELECT c.id, c.content, c.document_id FROM chunks c LEFT JOIN vectors v ON c.id = v.chunk_id " +
		"WHERE v.chunk_id IS NULL"
	if e.chunksHaveCampaignColumns() {
		q += " AND c.is_trivial = 0 AND c.dedup_of = ''"
	}
	rows, err := e.db.Query(q)
	if err != nil {
		log.Warn().Err(err).Msg("RepairOrphanVectors: failed to query orphan chunks")
		return
	}
	defer func() { _ = rows.Close() }()

	type orphan struct {
		id, content, docID string
	}
	var orphans []orphan
	for rows.Next() {
		var o orphan
		if scanErr := rows.Scan(&o.id, &o.content, &o.docID); scanErr != nil {
			log.Warn().Err(scanErr).Msg("RepairOrphanVectors: failed to scan orphan chunk")
			continue
		}
		orphans = append(orphans, o)
	}
	if err := rows.Err(); err != nil {
		log.Warn().Err(err).Msg("RepairOrphanVectors: error iterating orphan chunks")
		return
	}

	if len(orphans) == 0 {
		log.Debug().Msg("RepairOrphanVectors: no orphan chunks found")
		return
	}

	log.Info().Int("count", len(orphans)).Msg("RepairOrphanVectors: re-embedding orphan chunks")

	// Re-embed in batches to avoid overwhelming the embedding provider.
	batchSize := 32
	repaired := 0
	for i := 0; i < len(orphans); i += batchSize {
		end := i + batchSize
		if end > len(orphans) {
			end = len(orphans)
		}
		batch := orphans[i:end]

		texts := make([]string, len(batch))
		for j, o := range batch {
			texts[j] = o.content
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		results, embErr := e.embRegistry.GenerateEmbeddings(ctx, texts)
		cancel()
		if embErr != nil {
			log.Warn().Err(embErr).Int("batch", i/batchSize).Msg("RepairOrphanVectors: batch embedding failed, stopping")
			break
		}

		vectors := make([]vector.VectorRecord, 0, len(results))
		for j, result := range results {
			if result.Vector == nil {
				continue
			}
			vectors = append(vectors, vector.VectorRecord{
				ID:         batch[j].id,
				Vector:     result.Vector,
				DocumentID: batch[j].docID,
				ChunkID:    batch[j].id,
				Content:    batch[j].content,
			})
		}

		if len(vectors) > 0 {
			if storeErr := e.vecStore.Store(dim, vectors); storeErr != nil {
				log.Warn().Err(storeErr).Int("batch", i/batchSize).Msg("RepairOrphanVectors: batch store failed, stopping")
				break
			}
			repaired += len(vectors)
		}

		// Small pause between batches to be kind to the embedding provider.
		if end < len(orphans) {
			time.Sleep(100 * time.Millisecond)
		}
	}

	log.Info().
		Int("repaired", repaired).
		Int("total_orphans", len(orphans)).
		Msg("RepairOrphanVectors: complete")
}

// CleanupDanglingVectors removes vectors whose chunk reference no longer
// exists (e.g. a document was deleted/re-ingested but its vectors were left
// behind). This is the inverse of RepairOrphanVectors: that method re-embeds
// chunks that LACK vectors, while this one removes vectors that LACK chunks.
// Returns the number of vectors removed.
//
// A "vector count mismatch" reported by Verify() is caused by these dangling
// vectors — RepairOrphanVectors alone cannot fix them, so the CLI --fix path
// invokes both.
//
// The deletion keys on chunk_id only. Every vector in the cosca model is tied
// to a chunk (the indexer always sets ChunkID), so a vector whose chunk no
// longer exists is dangling. document_id is intentionally NOT checked here: a
// vector whose chunk still exists but whose document was deleted matches an
// orphaned chunk — a separate integrity issue (Verify's "orphaned chunks")
// that should not be conflated with vector cleanup.
//
// Entity vectors (entity_id != ”) are NEVER dangling: they are tied to graph
// entities, not chunks, and are owned by `cosca knowledge index-entities`.
// The CLI --fix path must not eat them (L338).
func (e *Engine) CleanupDanglingVectors() (int, error) {
	if e.db == nil {
		return 0, fmt.Errorf("database not available")
	}

	res, err := e.db.Exec(
		`DELETE FROM vectors WHERE chunk_id NOT IN (SELECT id FROM chunks) AND entity_id = ''`,
	)
	if err != nil {
		return 0, fmt.Errorf("delete dangling vectors: %w", err)
	}

	removed, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}

	if removed > 0 {
		log.Info().Int64("removed", removed).Msg("CleanupDanglingVectors: removed dangling vectors")
	}

	return int(removed), nil
}

// GCExpired removes documents whose tier window expired (medium = 7 days,
// long = 1 year). Chunks are removed by FK cascade; dangling vectors are
// cleaned afterwards by CleanupDanglingVectors. Returns the number of
// documents removed. The tier rule (L338): TUDO entra médio, o longo só por
// promoção explícita — "coloca tudo em medio, o longo a gente vai ver".
func (e *Engine) GCExpired() (int, error) {
	if e.db == nil {
		return 0, fmt.Errorf("database not available")
	}

	res, err := e.db.Exec(
		`DELETE FROM documents WHERE expires_at != '' AND expires_at < datetime('now')`,
	)
	if err != nil {
		return 0, fmt.Errorf("gc expired documents: %w", err)
	}

	removed, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}

	if removed > 0 {
		log.Info().Int64("removed", removed).Msg("GCExpired: removed expired documents")
	}

	return int(removed), nil
}

// PromoteDocument moves a document to another tier, extending its life:
// medium (7 days) → long (1 year) or back. Only explicit promotion moves a
// document to the long tier — the Don decides what lives long (L338).
func (e *Engine) PromoteDocument(id, tier string) error {
	if e.db == nil {
		return fmt.Errorf("database not available")
	}

	var ttl string
	switch tier {
	case "long":
		ttl = "+1 year"
	case "medium":
		ttl = "+7 days"
	default:
		return fmt.Errorf("tier inválido: %s (use 'long' ou 'medium')", tier)
	}

	res, err := e.db.Exec(
		`UPDATE documents SET tier = ?, expires_at = datetime('now', ?) WHERE id = ?`,
		tier, ttl, id,
	)
	if err != nil {
		return fmt.Errorf("promote document: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("document %s not found", id)
	}

	return nil
}

// ListDocumentsByTier returns the ids of documents currently in a tier
// ("medium" or "long") — used by `cosca knowledge promote --all`.
func (e *Engine) ListDocumentsByTier(tier string) []string {
	if e.db == nil {
		return nil
	}
	rows, err := e.db.Query(`SELECT id FROM documents WHERE tier = ? ORDER BY id`, tier)
	if err != nil {
		log.Warn().Err(err).Str("tier", tier).Msg("ListDocumentsByTier: query failed")
		return nil
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if scanErr := rows.Scan(&id); scanErr != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

// RootDir returns the configured knowledge root directory — the sandbox
// boundary enforced by the indexer's path containment checks.
func (e *Engine) RootDir() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cfg.RootDir
}

// Ranker expõe o re-rankear multi-fator do engine, para que camadas
// superiores (ex.: LayeredSearch no CLI) possam injetá-lo via SetRanker. É
// imutável após Init() — retorno direto é seguro e não exige cópia.
func (e *Engine) Ranker() *ranking.Ranker {
	return e.ranker
}

// Graph expõe o grafo de conhecimento do engine, para que camadas superiores
// (ex.: LayeredSearch no CLI) possam injetá-lo via SetGraph e alimentar o sinal
// GraphDistance do re-rank. O ponteiro do grafo é estável após Init() — retorno
// direto é seguro e não exige cópia, no mesmo padrão de Ranker().
func (e *Engine) Graph() *graph.Graph {
	return e.graph
}

// IndexDocument indexes a single document through the full pipeline.
func (e *Engine) IndexDocument(ctx context.Context, path string) error {
	e.mu.RLock()
	if !e.initialized {
		e.mu.RUnlock()
		return fmt.Errorf("knowledge engine not initialized")
	}
	e.mu.RUnlock()

	return e.indexer.IndexDocument(ctx, path)
}

// IndexDocumentWithMeta indexes a single document and merges an extra metadata
// map into the produced metadata_json (see indexer.IndexDocumentWithMeta).
// It is the entry point for provenance-aware ingestion: the caller computes
// the semantic provenance (scope/project/origin/kind/agent) and the indexer
// persists it alongside its own metadata. The legacy IndexDocument continues to
// call the indexer with no extra metadata.
func (e *Engine) IndexDocumentWithMeta(ctx context.Context, path string, meta map[string]any) error {
	e.mu.RLock()
	if !e.initialized {
		e.mu.RUnlock()
		return fmt.Errorf("knowledge engine not initialized")
	}
	e.mu.RUnlock()

	return e.indexer.IndexDocumentWithMeta(ctx, path, meta)
}

// IndexDirectory recursively indexes all supported files.
func (e *Engine) IndexDirectory(ctx context.Context, dir string) error {
	e.mu.RLock()
	if !e.initialized {
		e.mu.RUnlock()
		return fmt.Errorf("knowledge engine not initialized")
	}
	e.mu.RUnlock()

	return e.indexer.IndexDirectory(ctx, dir)
}

// WatchDirectory starts a file watcher on the given directory for auto-indexing.
func (e *Engine) WatchDirectory(dir string) error {
	if !e.cfg.WatchEnabled {
		return fmt.Errorf("file watching is not enabled in config")
	}

	w, err := watcher.New()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	e.fileWatcher = w

	// Start watching in background
	go func() {
		handler := &indexingHandler{engine: e}
		// Register handlers for each event type
		w.On(watcher.EventCreate, func(ctx context.Context, evt watcher.FileEvent) error {
			return handler.HandleEvent(ctx, evt)
		})
		w.On(watcher.EventModify, func(ctx context.Context, evt watcher.FileEvent) error {
			return handler.HandleEvent(ctx, evt)
		})
		w.On(watcher.EventDelete, func(ctx context.Context, evt watcher.FileEvent) error {
			return handler.HandleEvent(ctx, evt)
		})
		w.On(watcher.EventRename, func(ctx context.Context, evt watcher.FileEvent) error {
			return handler.HandleEvent(ctx, evt)
		})
		if err := w.Watch(dir); err != nil {
			if e.ctx.Err() == nil {
				log.Error().Err(err).Msg("file watcher stopped with error")
			}
		}
	}()

	log.Info().Str("dir", dir).Msg("file watcher started")
	return nil
}

// Search performs a hybrid search across all indexes with multi-tier caching.
// Results are cached for 5 minutes (Mem→SQLite→FS) keyed by all search parameters.
func (e *Engine) Search(ctx context.Context, params search.SearchParams) (*search.SearchResults, error) {
	e.mu.RLock()
	if !e.initialized {
		e.mu.RUnlock()
		return nil, fmt.Errorf("knowledge engine not initialized")
	}
	e.mu.RUnlock()

	// Generate a deterministic cache key from all distinguishing search parameters
	cacheKey := searchCacheKey(params)

	// Check multi-tier cache (Memory → SQLite → Filesystem)
	if e.cache != nil {
		if cached, ok := e.cache.Get(cacheKey); ok {
			// Memory cache returns original value types; SQLite/FS return JSON-deserialized types
			if results, ok := cached.(*search.SearchResults); ok {
				log.Debug().Str("cache_key", cacheKey).Msg("search cache hit (memory)")
				return results, nil
			}
			if jsonStr, ok := cached.(string); ok {
				var results search.SearchResults
				if err := json.Unmarshal([]byte(jsonStr), &results); err == nil {
					log.Debug().Str("cache_key", cacheKey).Msg("search cache hit (json)")
					return &results, nil
				}
			}
		}
	}

	// Cache miss — execute full hybrid search
	results, err := e.search.Search(ctx, params)
	if err != nil {
		return nil, err
	}

	// Store result in all enabled cache levels with 5-minute TTL
	if e.cache != nil && results != nil {
		serialized, jsonErr := json.Marshal(results)
		if jsonErr == nil {
			if setErr := e.cache.Set(cacheKey, string(serialized), 5*time.Minute); setErr != nil {
				log.Warn().Err(setErr).Str("cache_key", cacheKey).Msg("search cache set failed")
			}
		}
	}

	return results, nil
}

// Query performs a simple text search with default parameters.
func (e *Engine) Query(ctx context.Context, query string) (*search.SearchResults, error) {
	params := search.DefaultSearchParams()
	params.Query = query
	params.Limit = 20
	return e.Search(ctx, params)
}

// SymbolHit é um símbolo de código encontrado por busca semântica.
type SymbolHit struct {
	Name      string  `json:"name"`
	Kind      string  `json:"kind"`
	Signature string  `json:"signature,omitempty"`
	Package   string  `json:"package"`
	File      string  `json:"file"`
	Line      int     `json:"line"`
	Score     float64 `json:"score"`
}

// SearchSymbols busca símbolos de código semanticamente (por embedding) na
// tabela code_symbols. Gera o embedding da query e ordena por similaridade
// coseno — achar a função pelo que ela FAZ, não só pelo nome.
func (e *Engine) SearchSymbols(ctx context.Context, query string, limit int) ([]SymbolHit, error) {
	if e.db == nil {
		return nil, fmt.Errorf("knowledge db not initialized")
	}
	if e.embRegistry == nil {
		return nil, fmt.Errorf("embedding provider not available")
	}
	if limit <= 0 {
		limit = 10
	}

	qres, err := e.embRegistry.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if qres == nil || len(qres.Vector) == 0 {
		return nil, fmt.Errorf("empty query embedding")
	}

	rows, err := e.db.Query(
		`SELECT name, kind, signature, package, file, line, embedding
		 FROM code_symbols WHERE embedding IS NOT NULL`,
	)
	if err != nil {
		return nil, fmt.Errorf("query symbols: %w", err)
	}
	defer rows.Close()

	// Coleta serial (rows.Next não é concorrente), depois decodifica e pontua
	// em paralelo. O embedding é lido como BLOB binário float64 (novo formato)
	// ou JSON (legado) — decode rápido em ambos, sem reflect.
	records := make([]symbolRecord, 0, 256)
	for rows.Next() {
		var rec symbolRecord
		if err := rows.Scan(&rec.name, &rec.kind, &rec.sig, &rec.pkg, &rec.file, &rec.line, &rec.emb); err != nil {
			continue
		}
		if len(rec.emb) == 0 {
			continue
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}

	hits := scoreSymbolRecords(qres.Vector, records, limit)
	return hits, nil
}

// symbolRecord is a raw code_symbols row collected before parallel scoring.
type symbolRecord struct {
	name, kind, sig, pkg, file string
	line                       int
	emb                        []byte
}

// scoreSymbolRecords decodes every record embedding and keeps the top `limit`
// by cosine similarity, splitting the decode+score work across goroutines.
func scoreSymbolRecords(query []float64, records []symbolRecord, limit int) []SymbolHit {
	workers := runtime.NumCPU()
	n := len(records)
	if workers > n {
		workers = n
	}
	if workers < 1 {
		workers = 1
	}
	chunk := (n + workers - 1) / workers

	var mu sync.Mutex
	collected := make([]SymbolHit, 0, limit*workers)
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
			local := make([]SymbolHit, 0, hi-lo)
			for i := lo; i < hi; i++ {
				rec := records[i]
				vec := decodeEmbedding(rec.emb)
				if len(vec) == 0 {
					continue
				}
				score := cosineSimilarity(query, vec)
				local = append(local, SymbolHit{
					Name: rec.name, Kind: rec.kind, Signature: rec.sig,
					Package: rec.pkg, File: rec.file, Line: rec.line, Score: score,
				})
			}
			sort.Slice(local, func(a, b int) bool { return local[a].Score > local[b].Score })
			if len(local) > limit {
				local = local[:limit]
			}
			mu.Lock()
			collected = append(collected, local...)
			mu.Unlock()
		}(lo, hi)
	}
	wg.Wait()

	sort.Slice(collected, func(i, j int) bool { return collected[i].Score > collected[j].Score })
	if len(collected) > limit {
		collected = collected[:limit]
	}
	return collected
}

// decodeEmbedding decodes a code_symbols embedding: the legacy format is a
// JSON float64 array ("[0.1,0.2,...]"), the current format is a binary
// little-endian float64 BLOB (8 bytes per value, same values as JSON). Binary
// matches the byte-exact float64s the indexer writes, so results are
// identical to the old json.Unmarshal path — just ~10-20x faster.
func decodeEmbedding(b []byte) []float64 {
	if len(b) == 0 {
		return nil
	}
	if b[0] == '[' {
		return fastParseFloatArray(b)
	}
	if len(b)%8 != 0 {
		return nil
	}
	n := len(b) / 8
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		out[i] = math.Float64frombits(binary.LittleEndian.Uint64(b[i*8:]))
	}
	return out
}

// fastParseFloatArray parses a JSON array of float64s ("[a,b,c]") without the
// reflection overhead of encoding/json. Returns nil on any malformed input
// (the caller then skips the row, same as the old json.Unmarshal error path).
func fastParseFloatArray(data []byte) []float64 {
	i := 0
	for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
		i++
	}
	if i >= len(data) || data[i] != '[' {
		return nil
	}
	i++
	out := make([]float64, 0, 32)
	for {
		for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
			i++
		}
		if i >= len(data) {
			return nil
		}
		if data[i] == ']' {
			return out
		}
		start := i
		for i < len(data) && data[i] != ',' && data[i] != ']' &&
			!(data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
			i++
		}
		if start == i {
			return nil
		}
		f, err := strconv.ParseFloat(string(data[start:i]), 64)
		if err != nil {
			return nil
		}
		out = append(out, f)
		for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
			i++
		}
		if i >= len(data) {
			return nil
		}
		switch data[i] {
		case ',':
			i++
		case ']':
			return out
		default:
			return nil
		}
	}
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// Explain returns a human-readable explanation of why a result was returned.
func (e *Engine) Explain(_ context.Context, resultID string) (*Explanation, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	explanation := &Explanation{
		ResultID:    resultID,
		Factors:     make(map[string]float64),
		Connections: make([]string, 0),
	}

	// Check if it's a graph node
	if node, ok := e.graph.GetNode(resultID); ok {
		explanation.Type = "graph_node"
		explanation.Name = node.Name
		explanation.Factors["graph_presence"] = 1.0
		explanation.Factors["entity_type"] = 0.5

		// Get connections
		neighbors, _ := e.graph.GetNeighbors(resultID)
		for _, n := range neighbors {
			explanation.Connections = append(explanation.Connections,
				fmt.Sprintf("connected to %s (%s)", n.Name, n.Type))
		}
	}

	// Check database-backed items only if db is available
	if e.db != nil {
		// Check if it's a document
		var docTitle string
		err := e.db.QueryRow("SELECT title FROM documents WHERE id = ?", resultID).Scan(&docTitle)
		if err == nil {
			explanation.Type = "document"
			explanation.Name = docTitle
			explanation.Factors["fts_presence"] = 1.0
		}

		// Check if it's a chunk
		var chunkContent string
		err = e.db.QueryRow("SELECT content FROM chunks WHERE id = ?", resultID).Scan(&chunkContent)
		if err == nil {
			explanation.Type = "chunk"
			explanation.Name = resultID[:8]
			explanation.Factors["content_match"] = 0.8
			explanation.Factors["vector_similarity"] = 0.6
		}

		// Check vector store
		var vecCount int
		if err := e.db.QueryRow("SELECT COUNT(*) FROM vectors WHERE id = ?", resultID).Scan(&vecCount); err != nil {
			log.Warn().Err(err).Str("resultID", resultID).Msg("failed to check vector store for explanation")
		}
		if vecCount > 0 {
			explanation.Factors["has_embedding"] = 1.0
		}
	}

	return explanation, nil
}

// GetStats returns comprehensive engine statistics.
func (e *Engine) GetStats() (*Stats, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := &Stats{
		Uptime: time.Since(e.startTime),
	}

	// If not initialized or no database, return basic stats only
	if !e.initialized || e.db == nil {
		return stats, nil
	}

	// Document count
	if err := e.db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&stats.DocumentCount); err != nil {
		log.Warn().Err(err).Msg("failed to get document count")
	}

	// Chunk count
	if err := e.db.QueryRow("SELECT COUNT(*) FROM chunks").Scan(&stats.ChunkCount); err != nil {
		log.Warn().Err(err).Msg("failed to get chunk count")
	}

	// Entity count
	if err := e.db.QueryRow("SELECT COUNT(*) FROM entities").Scan(&stats.EntityCount); err != nil {
		log.Warn().Err(err).Msg("failed to get entity count")
	}

	// Vector stats
	if e.vecStore != nil {
		vecStats, err := e.vecStore.Stats()
		if err == nil {
			stats.VectorCount = vecStats.TotalVectors
		}
	}

	// Graph stats
	if e.graph != nil {
		stats.GraphStats = e.graph.Stats()
	}

	// Index stats
	if e.indexer != nil {
		stats.IndexStats = e.indexer.GetIndexStats()
	}

	// Embedding stats
	if e.embRegistry != nil {
		stats.EmbeddingStats = e.embRegistry.Stats()
	}

	// Cache stats
	if e.cache != nil {
		s := e.cache.Stats()
		stats.CacheStats = map[string]int{
			"memory_entries": s.MemoryEntries,
			"sqlite_entries": s.SQLiteEntries,
			"file_entries":   s.FileEntries,
			"memory_hits":    int(s.MemoryHits),
			"sqlite_hits":    int(s.SQLiteHits),
			"file_hits":      int(s.FileHits),
			"misses":         int(s.Misses),
			"memory_size":    int(s.MemorySize),
			"file_size":      int(s.FileSize),
		}
	}

	// DB file size
	if info, err := os.Stat(e.cfg.DBPath); err == nil {
		stats.DBSize = info.Size()
	}

	return stats, nil
}

// Rebuild performs a full rebuild of all indexes.
func (e *Engine) Rebuild(ctx context.Context) error {
	e.mu.Lock()
	if !e.initialized {
		e.mu.Unlock()
		return fmt.Errorf("knowledge engine not initialized")
	}
	e.mu.Unlock()

	log.Info().Msg("rebuilding knowledge engine")

	// Rebuild indexes
	if err := e.indexer.RebuildAll(ctx, e.cfg.RootDir); err != nil {
		return fmt.Errorf("rebuild indexes: %w", err)
	}

	// Save graph to database
	if err := e.saveGraph(); err != nil {
		log.Warn().Err(err).Msg("failed to save graph after rebuild")
	}

	log.Info().Msg("knowledge engine rebuild complete")
	return nil
}

// Vacuum cleans up stale data and reclaims space.
func (e *Engine) Vacuum() error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.initialized {
		return fmt.Errorf("knowledge engine not initialized")
	}

	// Clean expired cache entries
	if e.cache != nil {
		if err := e.cache.Clear(); err != nil {
			log.Warn().Err(err).Msg("failed to clear cache")
		}
	}

	// Database operations require a valid db connection
	if e.db == nil {
		return fmt.Errorf("database not available")
	}

	// Clean stale sync log entries
	_, err := e.db.Exec("DELETE FROM sync_log WHERE timestamp < datetime('now', '-7 days')")
	if err != nil {
		log.Warn().Err(err).Msg("failed to clean sync log")
	}

	// Vacuum database
	if err := e.db.Vacuum(); err != nil {
		return fmt.Errorf("vacuum database: %w", err)
	}

	log.Info().Msg("vacuum completed")
	return nil
}

// Verify checks the integrity of the knowledge engine.
func (e *Engine) Verify() (*VerificationResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := &VerificationResult{
		Checks: make(map[string]bool),
		Issues: make([]string, 0),
	}

	var chunkCount int

	// If engine is not initialized or db is nil, skip database checks
	if !e.initialized || e.db == nil {
		result.Checks["database_integrity"] = false
		result.Checks["has_documents"] = false
		result.Checks["has_chunks"] = false
		result.Checks["no_orphans"] = true
		result.Issues = append(result.Issues, "knowledge engine not initialized")
	} else {
		// 1. Database integrity
		issues, err := e.db.IntegrityCheck()
		if err != nil {
			result.Issues = append(result.Issues, fmt.Sprintf("integrity check error: %v", err))
			result.Checks["database_integrity"] = false
		} else if len(issues) > 0 {
			for _, issue := range issues {
				result.Issues = append(result.Issues, "db: "+issue)
			}
			result.Checks["database_integrity"] = false
		} else {
			result.Checks["database_integrity"] = true
		}

		// 2. Document consistency
		var docCount, entityCount int
		if err := e.db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&docCount); err != nil {
			log.Warn().Err(err).Msg("verify: failed to get document count")
		}
		if err := e.db.QueryRow("SELECT COUNT(*) FROM chunks").Scan(&chunkCount); err != nil {
			log.Warn().Err(err).Msg("verify: failed to get chunk count")
		}
		if err := e.db.QueryRow("SELECT COUNT(*) FROM entities").Scan(&entityCount); err != nil {
			log.Warn().Err(err).Msg("verify: failed to get entity count")
		}
		result.Checks["has_documents"] = docCount > 0
		result.Checks["has_chunks"] = chunkCount > 0

		// 3. Check orphaned chunks (no parent document)
		var orphanCount int
		if err := e.db.QueryRow("SELECT COUNT(*) FROM chunks WHERE document_id NOT IN (SELECT id FROM documents)").Scan(&orphanCount); err != nil {
			log.Warn().Err(err).Msg("verify: failed to count orphan chunks")
		}
		if orphanCount > 0 {
			result.Issues = append(result.Issues, fmt.Sprintf("%d orphaned chunks", orphanCount))
		}
		result.Checks["no_orphans"] = orphanCount == 0
	}

	// 4. Graph consistency
	if e.graph != nil {
		graphStats := e.graph.Stats()
		result.Checks["graph_has_nodes"] = graphStats.Nodes > 0
	}

	// 5. Vector store consistency
	if e.vecStore != nil {
		vecStats, err := e.vecStore.Stats()
		if err == nil {
			result.Checks["vector_store_healthy"] = true
			// Only chunk vectors participate in the chunk match: entity
			// vectors (entity_id != '') are graph entities, not chunks, and
			// would otherwise always produce a false mismatch (L338).
			var chunkVectors int
			if cErr := e.db.QueryRow("SELECT COUNT(*) FROM vectors WHERE entity_id = ''").Scan(&chunkVectors); cErr != nil {
				log.Warn().Err(cErr).Msg("verify: failed to count chunk vectors")
				chunkVectors = vecStats.TotalVectors
			}
			if chunkVectors != chunkCount {
				result.Issues = append(result.Issues,
					fmt.Sprintf("vector count mismatch: %d vectors vs %d chunks", chunkVectors, chunkCount))
				result.Checks["vector_chunk_match"] = false
			} else {
				result.Checks["vector_chunk_match"] = true
			}
		}
	}

	result.AllPassed = true
	for _, passed := range result.Checks {
		if !passed {
			result.AllPassed = false
			break
		}
	}

	return result, nil
}

// Snapshot exports the current state for backup.
func (e *Engine) Snapshot(name string) (*Snapshot, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.initialized {
		return nil, fmt.Errorf("knowledge engine not initialized")
	}

	if e.db == nil {
		return nil, fmt.Errorf("database not available")
	}

	// Create database backup
	backupPath, err := e.db.Backup(name)
	if err != nil {
		return nil, fmt.Errorf("backup database: %w", err)
	}

	// Export graph
	graphData, err := e.graph.Serialize()
	if err != nil {
		return nil, fmt.Errorf("serialize graph: %w", err)
	}

	snapshot := &Snapshot{
		ID:        uuid.New().String(),
		Name:      name,
		CreatedAt: time.Now(),
		DBPath:    backupPath,
		GraphJSON: string(graphData),
	}

	// Record in database
	var entityCount int
	if err := e.db.QueryRow("SELECT COUNT(*) FROM entities").Scan(&entityCount); err != nil {
		log.Warn().Err(err).Msg("snapshot: failed to get entity count")
	}

	_, err = e.db.Exec(
		`INSERT INTO snapshots (id, name, description, version, size_bytes, entity_count, data_json)
		 VALUES (?, ?, 'knowledge engine snapshot', 1, ?, ?, ?)`,
		snapshot.ID, name, len(graphData), entityCount, string(graphData),
	)
	if err != nil {
		log.Warn().Err(err).Msg("failed to record snapshot in database")
	}

	// Retention: prune the oldest auto-* snapshot rows so the table cannot
	// grow unbounded. The daemon prunes the auto-*.db backup files using the
	// same limit (defaultMaxBackups), keeping rows and files in sync.
	if err := e.pruneAutoSnapshots(defaultSnapshotKeep); err != nil {
		log.Warn().Err(err).Msg("failed to prune old auto snapshots")
	}

	log.Info().Str("name", name).Str("db", backupPath).Msg("snapshot created")
	return snapshot, nil
}

// defaultSnapshotKeep is the number of auto-* snapshot rows retained. It
// mirrors the daemon's defaultMaxBackups so DB rows and backup files share
// the same retention window.
const defaultSnapshotKeep = 3

// pruneAutoSnapshots removes the oldest auto-* snapshot rows beyond keep.
func (e *Engine) pruneAutoSnapshots(keep int) error {
	if e.db == nil {
		return nil
	}
	_, err := e.db.Exec(`
		DELETE FROM snapshots
		WHERE name LIKE 'auto-%'
		  AND rowid NOT IN (
		      SELECT rowid FROM snapshots
		      WHERE name LIKE 'auto-%'
		      ORDER BY rowid DESC
		      LIMIT ?
		  )`, keep)
	return err
}

// Sync synchronizes the index with the filesystem (detect added/removed/modified files).
func (e *Engine) Sync(ctx context.Context) (*SyncResult, error) {
	return e.syncInternal(ctx, nil)
}

// SyncWithProgress runs a full knowledge sync and reports progress via fn.
// The callback is invoked at regular intervals during each phase so callers
// can stream progress updates to clients (e.g. via SSE).
//
// Phases reported:
//   - "scanning"  — walking the filesystem, comparing hashes
//   - "comparing" — checking the database for removed files
//   - "indexing"  — processing added, updated, and removed files
//
// fn is always called from the same goroutine, so no additional
// synchronisation is required.
func (e *Engine) SyncWithProgress(ctx context.Context, fn SyncProgressFn) (*SyncResult, error) {
	if fn == nil {
		return e.syncInternal(ctx, nil)
	}
	return e.syncInternal(ctx, fn)
}

const syncProgressBatchSize = 10

// syncInternal is the shared implementation for Sync and SyncWithProgress.
// When fn is nil no progress is reported (same behaviour as Sync).
func (e *Engine) syncInternal(ctx context.Context, fn SyncProgressFn) (*SyncResult, error) {
	e.mu.RLock()
	if !e.initialized {
		e.mu.RUnlock()
		return nil, fmt.Errorf("knowledge engine not initialized")
	}
	if e.db == nil {
		e.mu.RUnlock()
		return nil, fmt.Errorf("database not available")
	}
	e.mu.RUnlock()

	start := time.Now()

	result := &SyncResult{
		Added:   make([]string, 0),
		Removed: make([]string, 0),
		Updated: make([]string, 0),
		Errors:  make([]string, 0),
	}

	// Phase 1: Scanning — walk root directory and compare against database.
	rootDir := e.cfg.RootDir
	scanCount := 0

	// Optionally count total files first so we can report meaningful totals.
	totalFiles := 0
	if fn != nil {
		_ = filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // skip on error during counting
			}
			if !d.IsDir() {
				totalFiles++
			}
			return nil
		})
	}

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Check if file is in database.
		var dbHash string
		scanErr := e.db.QueryRow("SELECT hash FROM documents WHERE path = ?", path).Scan(&dbHash)
		if scanErr != nil {
			result.Added = append(result.Added, path)
		} else {
			// Compute current hash.
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			currentHash := computeHash(string(content))
			if currentHash != dbHash {
				result.Updated = append(result.Updated, path)
			}
		}

		scanCount++
		if scanCount%syncProgressBatchSize == 0 && fn != nil {
			fn(SyncProgress{
				Phase:       "scanning",
				Processed:   scanCount,
				Total:       totalFiles,
				CurrentFile: path,
				Message:     "Scanning filesystem...",
			})
		}
		return nil
	})

	// Final scan progress report.
	if fn != nil {
		fn(SyncProgress{
			Phase:       "scanning",
			Processed:   scanCount,
			Total:       totalFiles,
			CurrentFile: "",
			Message:     "Scanning complete.",
		})
	}

	if err != nil {
		return nil, fmt.Errorf("walk failed: %w", err)
	}

	// Phase 2: Comparing — find removed files (in DB but not on filesystem).
	// Count documents first to report an accurate total.
	if fn != nil {
		fn(SyncProgress{Phase: "comparing", Processed: 0, Total: 0, Message: "Comparing with database..."})
	}

	rows, err := e.db.Query("SELECT path FROM documents")
	if err == nil {
		defer func() {
			if closeErr := rows.Close(); closeErr != nil {
				log.Warn().Err(closeErr).Msg("failed to close rows after sync")
			}
		}()

		// First pass: collect all paths to count total.
		type docEntry struct{ path string }
		var allDocs []docEntry
		for rows.Next() {
			var docPath string
			if scanErr := rows.Scan(&docPath); scanErr != nil {
				log.Warn().Err(scanErr).Msg("sync: failed to scan document path")
				continue
			}
			allDocs = append(allDocs, docEntry{path: docPath})
		}

		dbTotal := len(allDocs)
		dbProcessed := 0
		for _, doc := range allDocs {
			if _, statErr := os.Stat(doc.path); os.IsNotExist(statErr) {
				result.Removed = append(result.Removed, doc.path)
			}
			dbProcessed++
			if dbProcessed%syncProgressBatchSize == 0 && fn != nil {
				fn(SyncProgress{
					Phase:       "comparing",
					Processed:   dbProcessed,
					Total:       dbTotal,
					CurrentFile: doc.path,
					Message:     "Comparing with database...",
				})
			}
		}

		if fn != nil {
			fn(SyncProgress{
				Phase:       "comparing",
				Processed:   dbProcessed,
				Total:       dbTotal,
				CurrentFile: "",
				Message:     "Comparison complete.",
			})
		}
	} else {
		log.Warn().Err(err).Msg("sync: failed to query documents for removed files")
	}

	// Phase 3: Indexing — process all changes.
	indexTotal := len(result.Added) + len(result.Updated) + len(result.Removed)
	indexProcessed := 0

	if fn != nil {
		fn(SyncProgress{
			Phase:     "indexing",
			Processed: 0,
			Total:     indexTotal,
			Message:   fmt.Sprintf("Indexing %d changes...", indexTotal),
		})
	}

	for _, path := range result.Added {
		if idxErr := e.indexer.IndexDocument(ctx, path); idxErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("add %s: %v", path, idxErr))
		}
		indexProcessed++
		if fn != nil {
			fn(SyncProgress{
				Phase:       "indexing",
				Processed:   indexProcessed,
				Total:       indexTotal,
				CurrentFile: path,
				Message:     "Adding...",
			})
		}
	}
	for _, path := range result.Updated {
		if idxErr := e.indexer.IndexChanged(ctx, path); idxErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("update %s: %v", path, idxErr))
		}
		indexProcessed++
		if fn != nil {
			fn(SyncProgress{
				Phase:       "indexing",
				Processed:   indexProcessed,
				Total:       indexTotal,
				CurrentFile: path,
				Message:     "Updating...",
			})
		}
	}
	for _, path := range result.Removed {
		if idxErr := e.indexer.IndexRemoved(ctx, path); idxErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("remove %s: %v", path, idxErr))
		}
		indexProcessed++
		if fn != nil {
			fn(SyncProgress{
				Phase:       "indexing",
				Processed:   indexProcessed,
				Total:       indexTotal,
				CurrentFile: path,
				Message:     "Removing...",
			})
		}
	}

	result.Duration = time.Since(start)

	// Persist the graph built during sync.
	if sErr := e.saveGraph(); sErr != nil {
		log.Warn().Err(sErr).Msg("failed to save graph after sync")
	}

	return result, nil
}

// Close gracefully shuts down the knowledge engine.
// SetMetricsSink expõe o sink de métricas do caminho real (campanha de
// performance, FASE 1): cada busca vetorial reporta quantos vetores foram de
// fato escaneados. Nil-safe no engine de busca. Para diagnóstico/monitoração.
func (e *Engine) SetMetricsSink(sink func(vector.SearchMetrics)) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.search != nil {
		e.search.MetricsSink = sink
	}
}

func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil
	}
	e.closed = true

	log.Info().Msg("shutting down knowledge engine")

	// Cancel context (stops watcher)
	e.cancel()

	// Save graph state
	if err := e.saveGraph(); err != nil {
		log.Warn().Err(err).Msg("failed to save graph on shutdown")
	}

	// Close subsystems
	if e.fileWatcher != nil {
		if err := e.fileWatcher.Close(); err != nil {
			log.Warn().Err(err).Msg("failed to close file watcher")
		}
	}
	if e.embRegistry != nil {
		if err := e.embRegistry.Close(); err != nil {
			log.Warn().Err(err).Msg("failed to close embedding registry")
		}
	}
	if e.vecStore != nil {
		if err := e.vecStore.Close(); err != nil {
			log.Warn().Err(err).Msg("failed to close vector store")
		}
	}
	if e.db != nil {
		if err := e.db.Close(); err != nil {
			log.Warn().Err(err).Msg("failed to close database")
		}
	}

	e.initialized = false
	log.Info().Msg("knowledge engine shut down")
	return nil
}

// ── Graph persistence ─────────────────────────────────────────────────────

func (e *Engine) saveGraph() error {
	if e.graph == nil || e.db == nil {
		return nil
	}

	// Busca/leitura não sujam o grafo: persisti-lo (DELETE + re-insert de 13k
	// entidades) a cada Close custava ~1.8s por invocação CLI sem nada mudar.
	if !e.graph.IsDirty() {
		log.Debug().Msg("graph not modified — skipping persistence on close")
		return nil
	}

	// 1. Serialize full graph to JSON cache (backward compatible, fast boot)
	data, err := e.graph.Serialize()
	if err != nil {
		return fmt.Errorf("serialize graph: %w", err)
	}

	_, err = e.db.Exec(
		`INSERT OR REPLACE INTO cache (key, value, content_type, ttl_seconds, created_at, expires_at)
		 VALUES ('graph_state', ?, 'application/json', 86400, datetime('now'), datetime('now', '+1 day'))`,
		data,
	)
	if err != nil {
		return fmt.Errorf("cache graph: %w", err)
	}

	// 2. Persist nodes/edges to relational tables for SQL queryability.
	// entities: id, entity_type, name, path, metadata_json, created_at, updated_at
	// relationships: id, source_id, source_type, target_id, target_type, rel_type, weight, metadata_json, created_at
	e.persistGraphToSQL()
	e.graph.MarkClean()

	return nil
}

// persistGraphToSQL writes the in-memory graph nodes and edges to the
// entities and relationships tables. Existing rows are replaced atomically
// so the tables always reflect the current graph state.
func (e *Engine) persistGraphToSQL() {
	tx, err := e.db.Begin()
	if err != nil {
		log.Warn().Err(err).Msg("graph-sql: begin tx failed")
		return
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			// Rollback on committed tx is a no-op in sqlite, ignore.
			_ = err
		}
	}()

	// Clear existing graph rows (entities + relationships only — chunks/documents
	// are managed by the indexer separately).
	if _, err := tx.Exec("DELETE FROM entities"); err != nil {
		log.Warn().Err(err).Msg("graph-sql: clear entities failed")
		return
	}
	if _, err := tx.Exec("DELETE FROM relationships"); err != nil {
		log.Warn().Err(err).Msg("graph-sql: clear relationships failed")
		return
	}

	// Build a set of source/target node IDs that actually participate in edges.
	// We only persist nodes that have relationships OR are document/skill-type.
	referencedNodes := make(map[string]bool)
	nodes := e.graph.GetAllNodes()
	edges := e.graph.GetAllEdges()

	for _, edge := range edges {
		referencedNodes[edge.Source] = true
		referencedNodes[edge.Target] = true
	}

	// Build node lookup for type resolution
	nodeByID := make(map[string]*graph.Node, len(nodes))
	for _, n := range nodes {
		nodeByID[n.ID] = n
	}

	// Insert nodes
	for _, node := range nodes {
		// Skip chunk-type nodes without edges (there are 62K chunks, we don't
		// need all of them in entities — the chunks table already stores them).
		if node.Type == "chunk" && !referencedNodes[node.ID] {
			continue
		}

		metaJSON := "{}"
		if node.Metadata != nil {
			if b, err := json.Marshal(node.Metadata); err == nil {
				metaJSON = string(b)
			}
		}

		_, err := tx.Exec(
			`INSERT INTO entities (id, entity_type, name, path, metadata_json, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
			node.ID, node.Type, node.Name, node.Path, metaJSON,
		)
		if err != nil {
			log.Warn().Err(err).Str("node_id", node.ID).Msg("graph-sql: insert entity failed")
		}
	}

	// Insert edges with generated UUIDs
	for _, edge := range edges {
		sourceType := "unknown"
		if srcNode, ok := nodeByID[edge.Source]; ok {
			sourceType = srcNode.Type
		}
		targetType := "unknown"
		if tgtNode, ok := nodeByID[edge.Target]; ok {
			targetType = tgtNode.Type
		}

		metaJSON := "{}"
		if edge.Metadata != nil {
			if b, err := json.Marshal(edge.Metadata); err == nil {
				metaJSON = string(b)
			}
		}

		edgeID := uuid.New().String()
		_, err := tx.Exec(
			`INSERT INTO relationships (id, source_id, source_type, target_id, target_type, rel_type, weight, metadata_json, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))`,
			edgeID, edge.Source, sourceType, edge.Target, targetType, edge.Type, edge.Weight, metaJSON,
		)
		if err != nil {
			log.Warn().Err(err).
				Str("source", edge.Source).
				Str("target", edge.Target).
				Msg("graph-sql: insert relationship failed")
		}
	}

	if err := tx.Commit(); err != nil {
		log.Warn().Err(err).Msg("graph-sql: commit failed")
		return
	}

	log.Info().
		Int("entities", len(nodes)).
		Int("relationships", len(edges)).
		Msg("graph persisted to SQL")
}

func (e *Engine) loadGraph() error {
	if e.graph == nil || e.db == nil {
		return nil
	}

	var data []byte
	err := e.db.QueryRow(
		"SELECT value FROM cache WHERE key = 'graph_state'",
	).Scan(&data)

	if err != nil {
		return err
	}

	loadedGraph, err := graph.Deserialize(data)
	if err != nil {
		return fmt.Errorf("deserialize graph: %w", err)
	}

	e.graph = loadedGraph
	e.graphBuilder = graph.NewBuilder(e.graph)

	// O Deserialize popula via AddNode (marca dirty). A leitura do cache NÃO
	// é mutação: limpa o flag para que um Close pós-busca (read-only) pule a
	// persistência de 13k entidades no SQL (~1.8s).
	e.graph.MarkClean()

	log.Info().Int("nodes", e.graph.GetNodeCount()).Int("edges", e.graph.GetEdgeCount()).Msg("graph loaded from cache")
	return nil
}

// ── Types ──────────────────────────────────────────────────────────────────

// Explanation describes why a result was returned.
type Explanation struct {
	ResultID    string             `json:"result_id"`
	Type        string             `json:"type"`
	Name        string             `json:"name"`
	Factors     map[string]float64 `json:"factors"`
	Connections []string           `json:"connections,omitempty"`
}

// Snapshot represents a point-in-time export of the knowledge engine state.
type Snapshot struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	DBPath    string    `json:"db_path"`
	GraphJSON string    `json:"graph_json"`
}

// SyncResult summarizes a sync operation.
type SyncResult struct {
	Added    []string      `json:"added"`
	Removed  []string      `json:"removed"`
	Updated  []string      `json:"updated"`
	Errors   []string      `json:"errors"`
	Duration time.Duration `json:"duration_ms"`
}

// SyncProgress reports the current state of a knowledge sync operation.
// The Phase field indicates the current stage: "scanning", "comparing", or
// "indexing". Processed and Total track overall completion within the phase;
// Total may be 0 when the phase count is not known in advance (e.g. during
// the initial directory scan).
type SyncProgress struct {
	Phase       string `json:"phase"`        // "scanning", "comparing", "indexing"
	Processed   int    `json:"processed"`    // number of items processed so far
	Total       int    `json:"total"`        // total items to process (0 if unknown)
	CurrentFile string `json:"current_file"` // current file being processed
	Message     string `json:"message"`      // human-readable message
}

// SyncProgressFn is a callback invoked during SyncWithProgress to report
// the current progress of the sync operation. It is always called from the
// same goroutine that started the sync, so no additional synchronisation is
// needed by callers.
type SyncProgressFn func(SyncProgress)

// VerificationResult shows the results of an integrity verification.
type VerificationResult struct {
	AllPassed bool            `json:"all_passed"`
	Checks    map[string]bool `json:"checks"`
	Issues    []string        `json:"issues,omitempty"`
}

// ── File watcher handler ───────────────────────────────────────────────

type indexingHandler struct {
	engine *Engine
}

func (h *indexingHandler) HandleEvent(ctx context.Context, event watcher.FileEvent) error {
	var err error
	switch event.Type {
	case watcher.EventCreate:
		err = h.engine.IndexDocument(ctx, event.Path)
	case watcher.EventModify:
		err = h.engine.indexer.IndexChanged(ctx, event.Path)
	case watcher.EventDelete:
		err = h.engine.indexer.IndexRemoved(ctx, event.Path)
	case watcher.EventRename:
		// Remove old path, index new path
		if rmErr := h.engine.indexer.IndexRemoved(ctx, event.OldPath); rmErr != nil {
			log.Warn().Err(rmErr).Str("path", event.OldPath).Msg("failed to remove old path on rename")
		}
		err = h.engine.IndexDocument(ctx, event.Path)
	default:
		return nil
	}

	// Persist the graph after any incremental index change. Without this,
	// entities/relationships stay empty: incremental indexing only builds the
	// graph in memory, and saveGraph() was previously invoked solely on
	// Sync/Rebuild/Close — which never run on a long-lived serve using the
	// file watcher for re-indexing.
	if err == nil {
		if sErr := h.engine.saveGraph(); sErr != nil {
			log.Warn().Err(sErr).Msg("failed to save graph after incremental index")
		}
	}

	return err
}

// ── Helpers ────────────────────────────────────────────────────────────────

// searchCacheKey generates a deterministic cache key from all distinguishing
// search parameters. Any change to query, filters, pagination, or enabled
// search modes produces a different cache key.
func searchCacheKey(params search.SearchParams) string {
	// Build a canonical representation of all distinguishing parameters.
	// fmt.Sprintf("%v", ...) gives deterministic output for slices and maps.
	parts := []string{
		"search",
		params.Query,
		fmt.Sprintf("%d", params.Limit),
		fmt.Sprintf("%d", params.Offset),
		fmt.Sprintf("%v", params.Types),
		params.Path,
		fmt.Sprintf("%v", params.Tags),
		fmt.Sprintf("%d", params.Since.Unix()),
		fmt.Sprintf("%v", params.EnableFTS),
		fmt.Sprintf("%v", params.EnableVector),
		fmt.Sprintf("%v", params.EnableGraph),
		fmt.Sprintf("%v", params.EnableFacets),
		fmt.Sprintf("%f", params.MinScore),
	}

	// FASE 1.5 — Cache Scope Safety: a chave deve ser específica ao SearchScope
	// que originou a consulta, para que um resultado de escopo A jamais seja
	// servido para um escopo B.
	//
	// Semântica explícita (requisito 4 + preservação da LEGACY, requisito 5):
	//   - Scope nil OU com Modules vazio = SEM confinamento (LEGACY, busca
	//     ilimitada). Ambos são semanticamente idênticos e NÃO recebem
	//     componente de escopo → a chave/semântica LEGACY permanece exatamente
	//     a mesma de antes deste ajuste.
	//   - Apenas um escopo que realmente confina (Modules não-vazio) contribui
	//     com uma representação CANÔNICA dos módulos (ordenada), pois
	//     `confineToScope` trata os módulos como um CONJUNTO — a semântica não
	//     depende da ordem. Assim, módulos equivalentes em qualquer ordem
	//     produzem a mesma chave (requisito 7), e escopos distintos produzem
	//     chaves distintas (impossível o cache servir um resultado de escopo A
	//     para o escopo B).
	if params.Scope != nil && len(params.Scope.Modules) > 0 {
		parts = append(parts, "scope:"+canonicalScopeModules(params.Scope.Modules))
	}

	return cache.Key(parts...)
}

// canonicalScopeModules devolve a representação canônica (ordenada e dedup) dos
// módulos de um SearchScope, insensível à ordem — a mesma semântica de
// confinamento produz sempre a mesma chave.
func canonicalScopeModules(modules []string) string {
	seen := make(map[string]struct{}, len(modules))
	for _, m := range modules {
		seen[m] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for m := range seen {
		out = append(out, m)
	}
	sort.Strings(out)
	var b strings.Builder
	for i, m := range out {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(m)
	}
	return b.String()
}

func computeHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:])
}
