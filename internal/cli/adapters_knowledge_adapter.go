package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/search"
)

// Knowledge Adapter
// =============================================================================

// knowledgeEngineAdapter wraps knowledge.Engine to match CLI expected API.
type knowledgeEngineAdapter struct {
	inner  *knowledge.Engine
	dir    string
	closed bool // o ke foi fechado por uma chamada anterior (defer Close)
}

// newKnowledgeEngineAdapter creates a knowledge engine adapter.
// CLI calls knowledge.NewEngine(dir) but real API is knowledge.New(cfg).
func newKnowledgeEngineAdapter(dir string) *knowledgeEngineAdapter {
	return &knowledgeEngineAdapter{inner: nil, dir: dir}
}

// knowledgeDBPath returns the knowledge base path for the given root. The
// adapter is called with either the project root (os.Getwd()) or the .cosca
// dir itself (coscaDir), so the join is idempotent: if dir already is the
// .cosca directory, ".cosca" is not appended again. The final invariant is
// always <project>/.cosca/knowledge.db.
func knowledgeDBPath(dir string) string {
	if filepath.Base(dir) == ".cosca" {
		return filepath.Join(dir, "knowledge.db")
	}
	return filepath.Join(dir, ".cosca", "knowledge.db")
}

// getOrCreateEngine lazily initializes the engine.
func (a *knowledgeEngineAdapter) getOrCreateEngine() *knowledge.Engine {
	if a.inner != nil && !a.closed {
		return a.inner
	}
	if a.inner != nil && a.closed {
		// O ke anterior foi fechado (defer Close) — recria (L386: o Init de
		// um ke ja inicializado retorna "already initialized").
		a.inner = nil
		a.closed = false
	}

	// Honor the project config's embedding settings, mirroring serve.go and
	// runtime.go: embedding.provider selects the provider, embedding.base_url
	// points it at a custom endpoint (e.g. a local OpenAI-compatible embeddings
	// server), and api_key/model/dimensions are forwarded as provider
	// overrides. A config load failure is non-fatal and leaves the provider
	// empty so auto-detection applies.
	var (
		embeddingProvider   string
		embeddingBaseURL    string
		embeddingModel      string
		embeddingDigest     string
		embeddingAPIKey     string
		embeddingDimensions int
	)
	if c, err := config.Load(); err == nil {
		embeddingProvider = c.Embedding.Provider
		embeddingBaseURL = c.Embedding.BaseURL
		embeddingAPIKey = c.Embedding.APIKey
		// Model and dimensions are always forwarded from the project config —
		// the config is the source of truth for the embedding provider.
		// Providers keep their own local defaults (e.g. the openai provider
		// defaults to nomic-embed-text / 768 on the local endpoint) for
		// zero-config behavior, so an explicitly configured value must not be
		// suppressed even when it matches the Cosca defaults.
		embeddingModel = c.Embedding.Model
		embeddingDigest = c.Embedding.Digest
		embeddingDimensions = c.Embedding.Dimensions
	}

	ke, err := knowledge.New(knowledge.Config{
		DBPath:              knowledgeDBPath(a.dir),
		RootDir:             a.dir,
		AutoMigrate:         true,
		EmbeddingProvider:   embeddingProvider,
		EmbeddingBaseURL:    embeddingBaseURL,
		EmbeddingModel:      embeddingModel,
		EmbeddingDigest:     embeddingDigest,
		EmbeddingAPIKey:     embeddingAPIKey,
		EmbeddingDimensions: embeddingDimensions,
	})
	if err != nil {
		return nil
	}
	a.inner = ke
	return ke
}

// Search performs a search. CLI expects (results, error) with (query, limit, offset).
func (a *knowledgeEngineAdapter) Search(query string, limit, _ int) ([]KnowledgeSearchResult, error) {
	ke := a.getOrCreateEngine()
	if ke == nil {
		return nil, fmt.Errorf("knowledge engine not available")
	}
	// Initialize the engine
	if err := ke.Init(); err != nil {
		return nil, err
	}
	defer func() {
		if err := ke.Close(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to close knowledge engine: %v\n", err)
		}
		a.closed = true
	}()

	params := search.DefaultSearchParams()
	params.Query = query
	params.Limit = limit
	results, err := ke.Search(context.Background(), params)
	if err != nil {
		return nil, err
	}

	var out []KnowledgeSearchResult
	if results != nil {
		for _, r := range results.Results {
			out = append(out, KnowledgeSearchResult{
				Title:   r.Title,
				Type:    string(r.Type),
				Score:   r.Score,
				Snippet: r.Snippet,
			})
		}
	}
	return out, nil
}

// KnowledgeSearchResult holds a search result.
type KnowledgeSearchResult struct {
	Title   string  `json:"title"`
	Type    string  `json:"type"`
	Score   float64 `json:"score"`
	Snippet string  `json:"snippet"`
}

// Rebuild rebuilds the knowledge base. CLI expects no args.
func (a *knowledgeEngineAdapter) Rebuild() error {
	ke := a.getOrCreateEngine()
	if ke == nil {
		return fmt.Errorf("knowledge engine not available")
	}
	if err := ke.Init(); err != nil {
		return err
	}
	defer func() {
		if err := ke.Close(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to close knowledge engine: %v\n", err)
		}
		a.closed = true
	}()
	return ke.Rebuild(context.Background())
}

// IndexDirectory indexes a specific directory into the knowledge base (FTS + vectors + graph).
func (a *knowledgeEngineAdapter) IndexDirectory(dir string) error {
	ke := a.getOrCreateEngine()
	if ke == nil {
		return fmt.Errorf("knowledge engine not available")
	}
	if err := ke.Init(); err != nil {
		return err
	}
	defer func() {
		if err := ke.Close(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to close knowledge engine: %v\n", err)
		}
		a.closed = true
	}()
	return ke.IndexDirectory(context.Background(), dir)
}

// Verify verifies the knowledge base.
func (a *knowledgeEngineAdapter) Verify() (KnowledgeVerifyResult, error) {
	ke := a.getOrCreateEngine()
	if ke == nil {
		return KnowledgeVerifyResult{}, fmt.Errorf("knowledge engine not available")
	}
	if err := ke.Init(); err != nil {
		return KnowledgeVerifyResult{}, err
	}
	defer func() {
		if err := ke.Close(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to close knowledge engine: %v\n", err)
		}
		a.closed = true
	}()
	result, err := ke.Verify()
	if err != nil {
		return KnowledgeVerifyResult{}, err
	}
	return KnowledgeVerifyResult{
		Status:         map[bool]string{true: "passed", false: "failed"}[result.AllPassed],
		Issues:         len(result.Issues),
		EntriesChecked: len(result.Checks),
		IssueList: func() []KnowledgeIssue {
			issues := make([]KnowledgeIssue, 0, len(result.Issues))
			for _, issue := range result.Issues {
				issues = append(issues, KnowledgeIssue{
					Severity: "error",
					Message:  issue,
				})
			}
			return issues
		}(),
	}, nil
}

// RepairOrphanVectors re-embeds chunks that have no corresponding vector.
// This is a potentially long-running operation.
func (a *knowledgeEngineAdapter) RepairOrphanVectors() {
	ke := a.getOrCreateEngine()
	if ke == nil {
		return
	}
	if err := ke.Init(); err != nil {
		return
	}
	defer func() {
		if err := ke.Close(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to close knowledge engine: %v\n", err)
		}
		a.closed = true
	}()
	ke.RepairOrphanVectors()
}

// CleanupDanglingVectors removes vectors whose chunk/document references no
// longer exist (the inverse of RepairOrphanVectors). Returns the number of
// vectors removed, or -1 if the engine is unavailable.
func (a *knowledgeEngineAdapter) CleanupDanglingVectors() int {
	ke := a.getOrCreateEngine()
	if ke == nil {
		return -1
	}
	if err := ke.Init(); err != nil {
		return -1
	}
	defer func() {
		if err := ke.Close(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to close knowledge engine: %v\n", err)
		}
		a.closed = true
	}()
	removed, err := ke.CleanupDanglingVectors()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to clean dangling vectors: %v\n", err)
		return -1
	}
	return removed
}

// GCExpired removes documents whose tier window expired (medium 7d / long 1y)
// and cleans dangling vectors left behind. Returns the number of documents
// removed, or -1 if the engine is unavailable.
func (a *knowledgeEngineAdapter) GCExpired() int {
	ke := a.getOrCreateEngine()
	if ke == nil {
		return -1
	}
	if err := ke.Init(); err != nil {
		return -1
	}
	defer func() {
		if err := ke.Close(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to close knowledge engine: %v\n", err)
		}
		a.closed = true
	}()
	removed, err := ke.GCExpired()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to gc expired documents: %v\n", err)
		return -1
	}
	if removed > 0 {
		ke.CleanupDanglingVectors()
	}
	return removed
}

// PromoteDocument moves a document to another tier (long = 1 year, medium =
// 7 days) — the Don decides what lives long (L338).
func (a *knowledgeEngineAdapter) PromoteDocument(id, tier string) error {
	ke := a.getOrCreateEngine()
	if ke == nil {
		return fmt.Errorf("knowledge engine not available")
	}
	if err := ke.Init(); err != nil {
		return err
	}
	defer func() {
		if err := ke.Close(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to close knowledge engine: %v\n", err)
		}
		a.closed = true
	}()
	return ke.PromoteDocument(id, tier)
}

// ListDocumentsByTier returns document ids in the given tier (medium/long).
func (a *knowledgeEngineAdapter) ListDocumentsByTier(tier string) []string {
	ke := a.getOrCreateEngine()
	if ke == nil {
		return nil
	}
	if err := ke.Init(); err != nil {
		return nil
	}
	defer func() {
		if err := ke.Close(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to close knowledge engine: %v\n", err)
		}
		a.closed = true
	}()
	return ke.ListDocumentsByTier(tier)
}

// KnowledgeVerifyResult holds verification results.
type KnowledgeVerifyResult struct {
	Status         string           `json:"status"`
	Issues         int              `json:"issues"`
	EntriesChecked int              `json:"entries_checked"`
	IssueList      []KnowledgeIssue `json:"issue_list,omitempty"`
}

// KnowledgeIssue holds a single issue.
type KnowledgeIssue struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// Stats returns knowledge statistics.
func (a *knowledgeEngineAdapter) Stats() (KnowledgeStatsEx, error) {
	ke := a.getOrCreateEngine()
	if ke == nil {
		return KnowledgeStatsEx{}, fmt.Errorf("knowledge engine not available")
	}
	if err := ke.Init(); err != nil {
		return KnowledgeStatsEx{}, err
	}
	defer func() {
		if err := ke.Close(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to close knowledge engine: %v\n", err)
		}
		a.closed = true
	}()
	stats, err := ke.GetStats()
	if err != nil {
		return KnowledgeStatsEx{}, err
	}
	return KnowledgeStatsEx{
		TotalEntries:      stats.DocumentCount + stats.ChunkCount,
		ByType:            nil,
		DatabaseSize:      fmt.Sprintf("%d bytes", stats.DBSize),
		LastUpdated:       stats.LastIndexed,
		AvgEmbeddingScore: 0,
	}, nil
}

// KnowledgeStatsEx holds knowledge statistics.
type KnowledgeStatsEx struct {
	TotalEntries      int                  `json:"total_entries"`
	ByType            []KnowledgeTypeCount `json:"by_type,omitempty"`
	DatabaseSize      string               `json:"database_size"`
	LastUpdated       time.Time            `json:"last_updated"`
	AvgEmbeddingScore float64              `json:"avg_embedding_score"`
}

// KnowledgeTypeCount holds count by type.
type KnowledgeTypeCount struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

// Benchmark runs benchmarks. Placeholder.
func (a *knowledgeEngineAdapter) Benchmark() (KnowledgeBenchmarkResult, error) {
	return KnowledgeBenchmarkResult{
		AvgSearchTime: "0s",
		P50:           "0s",
		P95:           "0s",
		P99:           "0s",
		QueriesRun:    0,
		IndexSize:     "0 bytes",
	}, nil
}

// KnowledgeBenchmarkResult holds benchmark results.
type KnowledgeBenchmarkResult struct {
	AvgSearchTime string `json:"avg_search_time"`
	P50           string `json:"p50"`
	P95           string `json:"p95"`
	P99           string `json:"p99"`
	QueriesRun    int    `json:"queries_run"`
	IndexSize     string `json:"index_size"`
}

// Vacuum cleans up stale knowledge data.
func (a *knowledgeEngineAdapter) Vacuum() (VacuumResult, error) {
	ke := a.getOrCreateEngine()
	if ke == nil {
		return VacuumResult{}, fmt.Errorf("knowledge engine not available")
	}
	if err := ke.Init(); err != nil {
		return VacuumResult{}, err
	}
	defer func() {
		if err := ke.Close(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to close knowledge engine: %v\n", err)
		}
		a.closed = true
	}()
	if err := ke.Vacuum(); err != nil {
		return VacuumResult{}, err
	}
	return VacuumResult{
		Removed:        0,
		SpaceReclaimed: "0 bytes",
		Duration:       "0s",
	}, nil
}

// VacuumResult holds vacuum results.
type VacuumResult struct {
	Removed        int    `json:"removed"`
	SpaceReclaimed string `json:"space_reclaimed"`
	Duration       string `json:"duration"`
}

// Explain explains a search result.
func (a *knowledgeEngineAdapter) Explain(resultID string) (ExplanationEx, error) {
	ke := a.getOrCreateEngine()
	if ke == nil {
		return ExplanationEx{}, fmt.Errorf("knowledge engine not available")
	}
	if err := ke.Init(); err != nil {
		return ExplanationEx{}, err
	}
	defer func() {
		if err := ke.Close(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to close knowledge engine: %v\n", err)
		}
		a.closed = true
	}()
	exp, err := ke.Explain(context.Background(), resultID)
	if err != nil {
		return ExplanationEx{}, err
	}
	return ExplanationEx{
		Score:         0,
		KeywordScore:  0,
		SemanticScore: 0,
		GraphScore:    0,
		RecencyBoost:  0,
		MatchedTerms:  exp.Connections,
	}, nil
}

// ExplanationEx holds explanation data.
type ExplanationEx struct {
	Score         float64  `json:"score"`
	KeywordScore  float64  `json:"keyword_score"`
	SemanticScore float64  `json:"semantic_score"`
	GraphScore    float64  `json:"graph_score"`
	RecencyBoost  float64  `json:"recency_boost"`
	MatchedTerms  []string `json:"matched_terms,omitempty"`
}

// =============================================================================
