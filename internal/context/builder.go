// Package context provides a context assembler for building structured
// prompt contexts from search results, knowledge graph entries, and
// retrieved memory records.
package context

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ContextAssembler assembles context from search results and retrieved data.
//
//nolint:revive // Stutter name preserved for API compatibility — used as context.ContextAssembler externally.
type ContextAssembler struct {
	logger         zerolog.Logger
	tokenCounter   func(text string) int
	maxChunkSize   int
	maxDocSize     int
	maxSymbols     int
	includeContent bool
}

// AssemblerOption configures the context assembler.
type AssemblerOption func(*ContextAssembler)

// WithAssemblerLogger sets the logger.
func WithAssemblerLogger(logger zerolog.Logger) AssemblerOption {
	return func(a *ContextAssembler) {
		a.logger = logger
	}
}

// WithMaxChunkSize sets the maximum chunk size in tokens.
func WithMaxChunkSize(size int) AssemblerOption {
	return func(a *ContextAssembler) {
		a.maxChunkSize = size
	}
}

// WithMaxDocSize sets the maximum document size in tokens.
func WithMaxDocSize(size int) AssemblerOption {
	return func(a *ContextAssembler) {
		a.maxDocSize = size
	}
}

// NewContextAssembler creates a new context assembler.
func NewContextAssembler(opts ...AssemblerOption) *ContextAssembler {
	a := &ContextAssembler{
		logger:         zerolog.Nop(),
		tokenCounter:   defaultTokenCounter,
		maxChunkSize:   2000,
		maxDocSize:     8000,
		maxSymbols:     50,
		includeContent: true,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Assemble assembles context from search results, symbols, and memory.
func (a *ContextAssembler) Assemble(_ context.Context, req ContextRequest, docs []Document, symbols []Symbol, relationships []Relationship, memories []MemoryRecord) (*Context, error) {
	start := time.Now()
	a.logger.Debug().
		Int("docs", len(docs)).
		Int("symbols", len(symbols)).
		Int("relationships", len(relationships)).
		Int("memories", len(memories)).
		Msg("assembling context")

	result := &Context{
		Request: req,
		BuiltAt: time.Now(),
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	errCh := make(chan error, 4)

	// Select relevant documents
	wg.Add(1)
	go func() {
		defer wg.Done()
		selected := a.selectRelevantDocs(docs, req)
		mu.Lock()
		result.Documents = selected
		mu.Unlock()
	}()

	// Extract relevant chunks
	wg.Add(1)
	go func() {
		defer wg.Done()
		chunks := a.extractChunks(docs, req)
		mu.Lock()
		result.Chunks = chunks
		mu.Unlock()
	}()

	// Include symbol definitions
	wg.Add(1)
	go func() {
		defer wg.Done()
		selected := a.selectRelevantSymbols(symbols, req)
		mu.Lock()
		result.Symbols = selected
		mu.Unlock()
	}()

	// Include entity relationships
	wg.Add(1)
	go func() {
		defer wg.Done()
		mu.Lock()
		result.Relationships = relationships
		mu.Unlock()
	}()

	wg.Wait()
	close(errCh)

	// Include memory
	result.Memory = a.selectRelevantMemories(memories, req)

	// Rank all items by relevance
	a.rankAll(result, req)

	// Deduplicate
	result.Documents = a.deduplicateDocuments(result.Documents)
	result.Chunks = a.deduplicateChunks(result.Chunks)

	// Limit to token budget
	result = a.limitToTokenBudget(result, req.MaxTokens)

	// Format for LLM consumption
	result = a.formatForLLM(result)

	result.Metadata.RetrievedAt = time.Now()
	result.Metadata.TotalResults = len(docs) + len(symbols)

	a.logger.Debug().
		Int("documents", len(result.Documents)).
		Int("chunks", len(result.Chunks)).
		Int("symbols", len(result.Symbols)).
		Int("tokens", result.TokenCount).
		Dur("duration", time.Since(start)).
		Msg("context assembly complete")

	return result, nil
}

// selectRelevantDocs selects documents relevant to the request.
func (a *ContextAssembler) selectRelevantDocs(docs []Document, req ContextRequest) []Document {
	if len(docs) == 0 {
		return nil
	}

	// Score each document
	type scoredDoc struct {
		doc   Document
		score float64
	}

	scored := make([]scoredDoc, 0, len(docs))
	queryLower := strings.ToLower(req.Query)

	for _, doc := range docs {
		score := doc.Score

		// Boost score if title/name matches query
		titleLower := strings.ToLower(doc.Title)
		if titleLower != "" && strings.Contains(queryLower, titleLower) {
			score += 0.3
		}

		// Boost for exact file path matches
		pathLower := strings.ToLower(doc.Path)
		if strings.Contains(queryLower, pathLower) {
			score += 0.2
		}

		// Apply filters if any
		if len(req.Filters) > 0 {
			if !a.matchesFilters(doc, req.Filters) {
				score -= 0.5
			}
		}

		scored = append(scored, scoredDoc{doc: doc, score: score})
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Take top results
	maxDocs := 20
	if len(scored) < maxDocs {
		maxDocs = len(scored)
	}

	result := make([]Document, maxDocs)
	for i := 0; i < maxDocs; i++ {
		result[i] = scored[i].doc
		result[i].Score = scored[i].score
	}

	return result
}

// extractChunks extracts relevant chunks from documents.
func (a *ContextAssembler) extractChunks(docs []Document, req ContextRequest) []Chunk {
	var chunks []Chunk
	queryLower := strings.ToLower(req.Query)

	for _, doc := range docs {
		// For small documents, use the whole document
		if a.tokenCounter(doc.Content) <= a.maxChunkSize {
			chunk := Chunk{
				ID:         doc.ID + ":full",
				DocumentID: doc.ID,
				Content:    doc.Content,
				Score:      doc.Score,
			}
			chunks = append(chunks, chunk)
			continue
		}

		// For large documents, extract relevant sections
		lines := strings.Split(doc.Content, "\n")
		currentChunk := ""
		chunkScore := 0.0
		startLine := 1

		for i, line := range lines {
			currentChunk += line + "\n"

			// Score this line's relevance
			lineLower := strings.ToLower(line)
			if strings.Contains(lineLower, queryLower) {
				chunkScore += 1.0
			}

			// Check for section boundaries
			if (i+1)%50 == 0 || i == len(lines)-1 {
				if a.tokenCounter(currentChunk) > 0 {
					divisor := float64(maxInt(1, (i+1-startLine)/10+1))
					chunks = append(chunks, Chunk{
						ID:         doc.ID + ":" + strconv.Itoa(startLine),
						DocumentID: doc.ID,
						Content:    strings.TrimSpace(currentChunk),
						StartLine:  startLine,
						EndLine:    i + 1,
						Score:      chunkScore / divisor,
					})
				}
				currentChunk = ""
				startLine = i + 2
				chunkScore = 0
			}
		}
	}

	return chunks
}

// selectRelevantSymbols selects symbols relevant to the request.
func (a *ContextAssembler) selectRelevantSymbols(symbols []Symbol, req ContextRequest) []Symbol {
	if len(symbols) == 0 {
		return nil
	}

	type scoredSym struct {
		sym   Symbol
		score float64
	}

	queryLower := strings.ToLower(req.Query)
	scored := make([]scoredSym, 0, len(symbols))

	for _, sym := range symbols {
		score := sym.Score
		symLower := strings.ToLower(sym.Name)
		if strings.Contains(queryLower, symLower) || strings.Contains(symLower, queryLower) {
			score += 0.5
		}
		scored = append(scored, scoredSym{sym: sym, score: score})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	maxCount := a.maxSymbols
	if len(scored) < maxCount {
		maxCount = len(scored)
	}

	result := make([]Symbol, maxCount)
	for i := 0; i < maxCount; i++ {
		result[i] = scored[i].sym
		result[i].Score = scored[i].score
	}

	return result
}

// selectRelevantMemories filters and scores memories.
func (a *ContextAssembler) selectRelevantMemories(memories []MemoryRecord, req ContextRequest) []MemoryRecord {
	if len(memories) == 0 {
		return nil
	}

	type scoredMem struct {
		mem   MemoryRecord
		score float64
	}

	queryLower := strings.ToLower(req.Query)
	scored := make([]scoredMem, 0, len(memories))

	for _, mem := range memories {
		score := mem.Score
		if strings.Contains(strings.ToLower(mem.Content), queryLower) {
			score += 0.3
		}
		scored = append(scored, scoredMem{mem: mem, score: score})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	maxMem := 10
	if len(scored) < maxMem {
		maxMem = len(scored)
	}

	result := make([]MemoryRecord, maxMem)
	for i := 0; i < maxMem; i++ {
		result[i] = scored[i].mem
		result[i].Score = scored[i].score
	}

	return result
}

// rankAll ranks all context items by relevance.
func (a *ContextAssembler) rankAll(ctx *Context, _ ContextRequest) {
	// Documents already sorted by selectRelevantDocs
	// Chunks already scored in extractChunks
	// Sort chunks by score
	sort.Slice(ctx.Chunks, func(i, j int) bool {
		return ctx.Chunks[i].Score > ctx.Chunks[j].Score
	})
}

// deduplicateDocuments removes duplicate documents.
func (a *ContextAssembler) deduplicateDocuments(docs []Document) []Document {
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

// deduplicateChunks removes duplicate chunks.
func (a *ContextAssembler) deduplicateChunks(chunks []Chunk) []Chunk {
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

// limitToTokenBudget ensures the context fits within the token budget.
func (a *ContextAssembler) limitToTokenBudget(ctx *Context, maxTokens int) *Context {
	if maxTokens <= 0 {
		maxTokens = 32000
	}

	totalTokens := 0

	// Add documents
	var selectedDocs []Document
	for _, doc := range ctx.Documents {
		tokens := a.tokenCounter(doc.Content)
		if tokens > a.maxDocSize {
			tokens = a.maxDocSize
		}
		if totalTokens+tokens <= maxTokens {
			doc.Tokens = tokens
			selectedDocs = append(selectedDocs, doc)
			totalTokens += tokens
		}
	}
	ctx.Documents = selectedDocs

	// Add chunks
	var selectedChunks []Chunk
	for _, chunk := range ctx.Chunks {
		tokens := a.tokenCounter(chunk.Content)
		if totalTokens+tokens <= maxTokens {
			chunk.Tokens = tokens
			selectedChunks = append(selectedChunks, chunk)
			totalTokens += tokens
		}
	}
	ctx.Chunks = selectedChunks

	// Add symbols (small items) — filter by budget
	var selectedSymbols []Symbol
	for _, sym := range ctx.Symbols {
		symTokens := a.tokenCounter(sym.Signature) + a.tokenCounter(sym.Documentation) + 10
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

// formatForLLM formats context for LLM consumption.
func (a *ContextAssembler) formatForLLM(ctx *Context) *Context {
	// Add source metadata
	sources := make(map[string]bool)
	for _, doc := range ctx.Documents {
		if doc.Path != "" {
			sources[doc.Path] = true
		}
	}
	for _, chunk := range ctx.Chunks {
		if chunk.DocumentID != "" {
			sources[chunk.DocumentID] = true
		}
	}

	ctx.Metadata.Sources = make([]string, 0, len(sources))
	for s := range sources {
		ctx.Metadata.Sources = append(ctx.Metadata.Sources, s)
	}

	return ctx
}

// maxInt returns the maximum of two integers.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// matchesFilters checks if a document matches the given filters.
func (a *ContextAssembler) matchesFilters(doc Document, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	for _, filter := range filters {
		if strings.Contains(strings.ToLower(doc.Path), strings.ToLower(filter)) ||
			strings.Contains(strings.ToLower(doc.Title), strings.ToLower(filter)) {
			return true
		}
	}
	return false
}
