package engine

import "context"

// ═══════════════════════════════════════════════════════════════════════════════
// Memory & Knowledge Port Interfaces
//
// These interfaces mirror the orchestration-level ports defined in
// internal/orchestration/ports.go so that the AgentEngine can wire
// memory and knowledge dependencies without circular imports.
//
// Production implementations live in internal/memory/ and internal/knowledge/.
// Tests may provide lightweight stubs.
// ═══════════════════════════════════════════════════════════════════════════════

// MemoryRetriever searches and retrieves records from the memory engine.
// Matches the orchestration.MemoryRetriever interface.
type MemoryRetriever interface {
	// Search searches across memory layers for records matching the query.
	Search(ctx context.Context, query string, opts MemorySearchOptions) ([]MemoryRecord, error)

	// Retrieve fetches a specific memory record by ID and layer.
	Retrieve(ctx context.Context, id, layer string) (*MemoryRecord, error)
}

// MemoryStorer persists results into the memory engine.
// Matches the orchestration.MemoryStorer interface.
type MemoryStorer interface {
	// Store saves a memory record and returns the persisted record.
	Store(ctx context.Context, record MemoryRecord) (*MemoryRecord, error)
}

// KnowledgeSearcher searches the knowledge engine for information relevant
// to the current request.
// Matches the orchestration.KnowledgeSearcher interface.
type KnowledgeSearcher interface {
	// Search executes a knowledge-base search and returns matching documents.
	Search(ctx context.Context, params KnowledgeSearchParams) (*KnowledgeSearchResults, error)
}

// ─── Supporting Types ─────────────────────────────────────────────────────────

// MemoryRecord is a projection of a memory record carrying only the fields
// needed by the engine. Matches orchestration.MemoryRecord.
type MemoryRecord struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Layer    string            `json:"layer"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Priority int               `json:"priority"`
}

// MemorySearchOptions filters memory searches within the engine layer.
// Matches orchestration.MemorySearchOptions.
type MemorySearchOptions struct {
	Types    []string `json:"types,omitempty"`
	Layers   []string `json:"layers,omitempty"`
	Limit    int      `json:"limit"`
	MinScore float64  `json:"min_score,omitempty"`
}

// KnowledgeSearchParams carries search parameters for the knowledge engine.
// Matches orchestration.KnowledgeSearchParams.
type KnowledgeSearchParams struct {
	Query    string   `json:"query"`
	Limit    int      `json:"limit,omitempty"`
	Types    []string `json:"types,omitempty"`
	Path     string   `json:"path,omitempty"`
	MinScore float64  `json:"min_score,omitempty"`
}

// KnowledgeSearchResult is a single knowledge-base search hit.
// Matches orchestration.KnowledgeSearchResult.
type KnowledgeSearchResult struct {
	ID           string  `json:"id"`
	Title        string  `json:"title,omitempty"`
	Content      string  `json:"content,omitempty"`
	Snippet      string  `json:"snippet,omitempty"`
	Score        float64 `json:"score"`
	DocumentPath string  `json:"document_path,omitempty"`
}

// KnowledgeSearchResults bundles search hits with query metadata.
// Matches orchestration.KnowledgeSearchResults.
type KnowledgeSearchResults struct {
	Results    []KnowledgeSearchResult `json:"results"`
	TotalCount int                     `json:"total_count"`
	Query      string                  `json:"query"`
}
