package context

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// IntentType represents the type of user intent.
type IntentType string

// Predefined intent types.
const (
	IntentQuestion IntentType = "question"
	IntentFeature  IntentType = "feature"
	IntentBug      IntentType = "bug"
	IntentRefactor IntentType = "refactor"
	IntentReview   IntentType = "review"
	IntentDeploy   IntentType = "deploy"
	IntentDocs     IntentType = "docs"
	IntentExplore  IntentType = "explore"
	IntentSearch   IntentType = "search"
	IntentExecute  IntentType = "execute"
)

// ContextRequest defines what context is being requested.
//
//nolint:revive // Stutter name preserved for API compatibility — used as context.ContextRequest externally.
type ContextRequest struct {
	Intent    IntentType `json:"intent"`
	Query     string     `json:"query"`
	Files     []string   `json:"files,omitempty"`
	Entities  []string   `json:"entities,omitempty"`
	Filters   []string   `json:"filters,omitempty"`
	MaxTokens int        `json:"max_tokens"`
	SessionID string     `json:"session_id,omitempty"`
	ProjectID string     `json:"project_id,omitempty"`
}

// Context is the assembled context for LLM consumption.
type Context struct {
	Request       ContextRequest  `json:"request"`
	Documents     []Document      `json:"documents"`
	Chunks        []Chunk         `json:"chunks"`
	Symbols       []Symbol        `json:"symbols"`
	Entities      []Entity        `json:"entities"`
	Relationships []Relationship  `json:"relationships"`
	Memory        []MemoryRecord  `json:"memory"`
	Metadata      ContextMetadata `json:"metadata"`
	TokenCount    int             `json:"token_count"`
	BuiltAt       time.Time       `json:"built_at"`
}

// Document represents a source document.
type Document struct {
	ID       string  `json:"id"`
	Path     string  `json:"path"`
	Title    string  `json:"title,omitempty"`
	Content  string  `json:"content"`
	Language string  `json:"language,omitempty"`
	Score    float64 `json:"score"`
	Tokens   int     `json:"tokens"`
}

// Chunk represents a segment of a document.
type Chunk struct {
	ID         string  `json:"id"`
	DocumentID string  `json:"document_id"`
	Content    string  `json:"content"`
	StartLine  int     `json:"start_line"`
	EndLine    int     `json:"end_line"`
	Score      float64 `json:"score"`
	Tokens     int     `json:"tokens"`
}

// Symbol represents a code symbol definition.
type Symbol struct {
	Name          string  `json:"name"`
	Kind          string  `json:"kind"`
	File          string  `json:"file"`
	Line          int     `json:"line"`
	Signature     string  `json:"signature,omitempty"`
	Documentation string  `json:"documentation,omitempty"`
	Score         float64 `json:"score"`
}

// Entity represents a named entity in the codebase.
type Entity struct {
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	File       string            `json:"file"`
	Line       int               `json:"line"`
	Properties map[string]string `json:"properties,omitempty"`
	Score      float64           `json:"score"`
}

// Relationship represents a relationship between entities.
type Relationship struct {
	Source string  `json:"source"`
	Target string  `json:"target"`
	Type   string  `json:"type"`
	Weight float64 `json:"weight"`
}

// MemoryRecord represents a memory entry.
type MemoryRecord struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Score     float64   `json:"score"`
}

// ContextMetadata contains metadata about the context assembly.
//
//nolint:revive // Stutter name preserved for API compatibility — used as context.ContextMetadata externally.
type ContextMetadata struct {
	Sources      []string       `json:"sources"`
	Confidence   float64        `json:"confidence"`
	Timing       AssemblyTiming `json:"timing"`
	Truncated    bool           `json:"truncated"`
	TotalResults int            `json:"total_results"`
	RetrievedAt  time.Time      `json:"retrieved_at"`
}

// AssemblyTiming tracks the time spent in each phase.
type AssemblyTiming struct {
	IntentAnalysis time.Duration `json:"intent_analysis"`
	Search         time.Duration `json:"search"`
	Retrieve       time.Duration `json:"retrieve"`
	Deduplicate    time.Duration `json:"deduplicate"`
	Rank           time.Duration `json:"rank"`
	Build          time.Duration `json:"build"`
	Total          time.Duration `json:"total"`
}

// IntentResult contains the result of intent analysis.
type IntentResult struct {
	Intent          IntentType `json:"intent"`
	Confidence      float64    `json:"confidence"`
	Entities        []string   `json:"entities"`
	RequiredContext []string   `json:"required_context"`
	SearchQuery     string     `json:"search_query"`
	OriginalQuery   string     `json:"original_query"`
}

// Builder assembles context from various sources.
type Builder struct {
	logger       zerolog.Logger
	tokenCounter func(text string) int
	searcher     Searcher
	retriever    Retriever
	memoryStore  MemoryStore
}

// Searcher searches for relevant documents.
type Searcher interface {
	Search(ctx context.Context, query string, filters []string) ([]Document, error)
}

// Retriever retrieves chunks and symbols for documents.
type Retriever interface {
	RetrieveChunks(ctx context.Context, docIDs []string) ([]Chunk, error)
	RetrieveSymbols(ctx context.Context, docIDs []string) ([]Symbol, error)
	RetrieveEntities(ctx context.Context, docIDs []string) ([]Entity, error)
	RetrieveRelationships(ctx context.Context, entityIDs []string) ([]Relationship, error)
}

// MemoryStore provides access to memory records.
type MemoryStore interface {
	Search(ctx context.Context, query string, limit int) ([]MemoryRecord, error)
}

// NewBuilder creates a new context builder.
func NewBuilder(logger zerolog.Logger, opts ...BuilderOption) *Builder {
	b := &Builder{
		logger:       logger,
		tokenCounter: defaultTokenCounter,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// BuilderOption configures the context builder.
type BuilderOption func(*Builder)

// WithTokenCounter sets a custom token counting function.
func WithTokenCounter(counter func(text string) int) BuilderOption {
	return func(b *Builder) {
		b.tokenCounter = counter
	}
}

// WithSearcher sets the search provider.
func WithSearcher(s Searcher) BuilderOption {
	return func(b *Builder) {
		b.searcher = s
	}
}

// WithRetriever sets the retrieval provider.
func WithRetriever(r Retriever) BuilderOption {
	return func(b *Builder) {
		b.retriever = r
	}
}

// WithMemoryStore sets the memory store provider.
func WithMemoryStore(m MemoryStore) BuilderOption {
	return func(b *Builder) {
		b.memoryStore = m
	}
}

// BuildContext assembles context based on the request.
func (b *Builder) BuildContext(ctx context.Context, req ContextRequest) (*Context, error) {
	start := time.Now()
	b.logger.Info().
		Str("intent", string(req.Intent)).
		Str("query", truncateQuery(req.Query, 100)).
		Int("max_tokens", req.MaxTokens).
		Msg("building context")

	var timing AssemblyTiming
	result := &Context{
		Request:    req,
		BuiltAt:    time.Now(),
		TokenCount: 0,
	}

	// Phase 1: Intent Analysis
	t1 := time.Now()
	intentResult, err := AnalyzeIntent(ctx, req.Query, b.logger)
	timing.IntentAnalysis = time.Since(t1)
	if err != nil {
		b.logger.Warn().Err(err).Msg("intent analysis failed, using provided intent")
		intentResult = &IntentResult{
			Intent:        req.Intent,
			Confidence:    1.0,
			OriginalQuery: req.Query,
		}
	}
	if intentResult.Intent != "" {
		req.Intent = intentResult.Intent
		result.Request.Intent = intentResult.Intent
	}

	// Phase 2: Search
	t2 := time.Now()
	if b.searcher != nil {
		query := intentResult.SearchQuery
		if query == "" {
			query = req.Query
		}
		docs, err := b.searcher.Search(ctx, query, req.Filters)
		timing.Search = time.Since(t2)
		if err != nil {
			b.logger.Warn().Err(err).Msg("search failed")
		} else {
			result.Documents = docs
			result.Metadata.TotalResults = len(docs)
		}
	}

	// Phase 3: Retrieve
	t3 := time.Now()
	if b.retriever != nil && len(result.Documents) > 0 {
		docIDs := extractDocIDs(result.Documents)

		var wg sync.WaitGroup
		errCh := make(chan error, 4)

		wg.Add(4)
		go func() {
			defer wg.Done()
			chunks, err := b.retriever.RetrieveChunks(ctx, docIDs)
			if err != nil {
				errCh <- err
				return
			}
			result.Chunks = chunks
		}()
		go func() {
			defer wg.Done()
			symbols, err := b.retriever.RetrieveSymbols(ctx, docIDs)
			if err != nil {
				errCh <- err
				return
			}
			result.Symbols = symbols
		}()
		entityCh := make(chan []Entity, 1)

		go func() {
			defer wg.Done()
			entities, err := b.retriever.RetrieveEntities(ctx, docIDs)
			if err != nil {
				errCh <- err
				entityCh <- nil
				return
			}
			result.Entities = entities
			entityCh <- entities
		}()
		go func() {
			defer wg.Done()
			entities := <-entityCh // Wait for entities to avoid race on result.Entities
			if entities == nil {
				return // Error already sent to errCh
			}
			entityIDs := extractEntityIDs(entities)
			rels, err := b.retriever.RetrieveRelationships(ctx, entityIDs)
			if err != nil {
				errCh <- err
				return
			}
			result.Relationships = rels
		}()

		wg.Wait()
		close(errCh)
		for err := range errCh {
			b.logger.Warn().Err(err).Msg("retrieval error")
		}
	}
	timing.Retrieve = time.Since(t3)

	// Phase 4: Deduplicate
	t4 := time.Now()
	result.Documents = deduplicateDocs(result.Documents)
	result.Chunks = deduplicateChunks(result.Chunks)
	result.Symbols = deduplicateSymbols(result.Symbols)
	timing.Deduplicate = time.Since(t4)

	// Phase 5: Rank
	t5 := time.Now()
	rankByRelevance(result.Documents, req)
	rankByRelevanceChunks(result.Chunks, req)
	rankByRelevanceSymbols(result.Symbols, req)
	timing.Rank = time.Since(t5)

	// Phase 6: Retrieve memory
	if b.memoryStore != nil {
		memories, err := b.memoryStore.Search(ctx, req.Query, 10)
		if err == nil {
			result.Memory = memories
		}
	}

	// Phase 7: Build (token-aware assembly)
	t6 := time.Now()
	result = b.assembleWithinTokenBudget(result, req.MaxTokens)
	timing.Build = time.Since(t6)

	// Fill metadata
	timing.Total = time.Since(start)
	result.Metadata.Timing = timing
	result.Metadata.RetrievedAt = time.Now()
	result.Metadata.Confidence = calculateConfidence(result)

	b.logger.Info().
		Int("documents", len(result.Documents)).
		Int("chunks", len(result.Chunks)).
		Int("symbols", len(result.Symbols)).
		Int("tokens", result.TokenCount).
		Dur("total", timing.Total).
		Bool("truncated", result.Metadata.Truncated).
		Msg("context built successfully")

	return result, nil
}

// assembleWithinTokenBudget ensures the context fits within the token budget.
func (b *Builder) assembleWithinTokenBudget(ctx *Context, maxTokens int) *Context {
	if maxTokens <= 0 {
		maxTokens = 32000 // Default max tokens
	}

	totalTokens := 0

	// Always include intent information
	intentTokens := estimateTokens(string(ctx.Request.Intent))
	totalTokens += intentTokens

	// Add documents with highest scores first
	var selectedDocs []Document
	for _, doc := range ctx.Documents {
		tokens := b.tokenCounter(doc.Content)
		if totalTokens+tokens <= maxTokens {
			doc.Tokens = tokens
			selectedDocs = append(selectedDocs, doc)
			totalTokens += tokens
		}
	}
	ctx.Documents = selectedDocs

	// Add chunks with highest scores
	var selectedChunks []Chunk
	for _, chunk := range ctx.Chunks {
		tokens := b.tokenCounter(chunk.Content)
		if totalTokens+tokens <= maxTokens {
			chunk.Tokens = tokens
			selectedChunks = append(selectedChunks, chunk)
			totalTokens += tokens
		}
	}
	ctx.Chunks = selectedChunks

	// Add symbols (each symbol is small)
	var selectedSymbols []Symbol
	for _, sym := range ctx.Symbols {
		sigTokens := b.tokenCounter(sym.Signature)
		docTokens := b.tokenCounter(sym.Documentation)
		symTokens := sigTokens + docTokens + 10
		if totalTokens+symTokens <= maxTokens {
			selectedSymbols = append(selectedSymbols, sym)
			totalTokens += symTokens
		}
	}
	ctx.Symbols = selectedSymbols

	ctx.TokenCount = totalTokens
	ctx.Metadata.Truncated = totalTokens >= maxTokens

	return ctx
}

// defaultTokenCounter estimates tokens using a simple heuristic (~4 chars per token).
func defaultTokenCounter(text string) int {
	if text == "" {
		return 0
	}
	return len(text) / 4
}

// estimateTokens provides a quick token estimate.
func estimateTokens(text string) int {
	return len(text) / 4
}

// truncateQuery truncates a query for logging.
func truncateQuery(q string, maxLen int) string {
	if len(q) <= maxLen {
		return q
	}
	return q[:maxLen] + "..."
}

// extractDocIDs extracts document IDs from documents.
func extractDocIDs(docs []Document) []string {
	ids := make([]string, len(docs))
	for i, d := range docs {
		ids[i] = d.ID
	}
	return ids
}

// extractEntityIDs extracts entity IDs from entities.
func extractEntityIDs(entities []Entity) []string {
	ids := make([]string, len(entities))
	for i, e := range entities {
		ids[i] = e.Name
	}
	return ids
}

// deduplicateDocs removes duplicate documents by ID.
func deduplicateDocs(docs []Document) []Document {
	seen := make(map[string]bool)
	var result []Document
	for _, doc := range docs {
		if !seen[doc.ID] {
			seen[doc.ID] = true
			result = append(result, doc)
		}
	}
	return result
}

// deduplicateChunks removes duplicate chunks by ID.
func deduplicateChunks(chunks []Chunk) []Chunk {
	seen := make(map[string]bool)
	var result []Chunk
	for _, chunk := range chunks {
		if !seen[chunk.ID] {
			seen[chunk.ID] = true
			result = append(result, chunk)
		}
	}
	return result
}

// deduplicateSymbols removes duplicate symbols by name and file.
func deduplicateSymbols(symbols []Symbol) []Symbol {
	seen := make(map[string]bool)
	var result []Symbol
	for _, sym := range symbols {
		key := sym.Name + ":" + sym.File
		if !seen[key] {
			seen[key] = true
			result = append(result, sym)
		}
	}
	return result
}

// rankByRelevance sorts documents by their score.
func rankByRelevance(docs []Document, _ ContextRequest) {
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Score > docs[j].Score
	})
}

// rankByRelevanceChunks sorts chunks by their score.
func rankByRelevanceChunks(chunks []Chunk, _ ContextRequest) {
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Score > chunks[j].Score
	})
}

// rankByRelevanceSymbols sorts symbols by their score.
func rankByRelevanceSymbols(symbols []Symbol, _ ContextRequest) {
	sort.Slice(symbols, func(i, j int) bool {
		return symbols[i].Score > symbols[j].Score
	})
}

// calculateConfidence calculates an overall confidence score.
func calculateConfidence(ctx *Context) float64 {
	if len(ctx.Documents) == 0 && len(ctx.Chunks) == 0 && len(ctx.Memory) == 0 {
		return 0.0
	}
	score := 0.0
	count := 0
	for _, d := range ctx.Documents {
		score += d.Score
		count++
	}
	for _, c := range ctx.Chunks {
		score += c.Score
		count++
	}
	if count == 0 {
		return 0.0
	}
	return score / float64(count)
}
