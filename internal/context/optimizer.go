package context

import (
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/rs/zerolog"
)

// Optimizer optimizes context for LLM consumption.
type Optimizer struct {
	logger          zerolog.Logger
	tokenCounter    func(text string) int
	maxContextSize  int
	reserveTokens   int
	compressionRate float64
}

// OptimizerOption configures the optimizer.
type OptimizerOption func(*Optimizer)

// WithOptimizerLogger sets the logger.
func WithOptimizerLogger(logger zerolog.Logger) OptimizerOption {
	return func(o *Optimizer) {
		o.logger = logger
	}
}

// WithMaxContextSize sets the maximum context size.
func WithMaxContextSize(size int) OptimizerOption {
	return func(o *Optimizer) {
		o.maxContextSize = size
	}
}

// WithReserveTokens sets the number of tokens to reserve for response.
func WithReserveTokens(tokens int) OptimizerOption {
	return func(o *Optimizer) {
		o.reserveTokens = tokens
	}
}

// NewOptimizer creates a new context optimizer.
func NewOptimizer(opts ...OptimizerOption) *Optimizer {
	o := &Optimizer{
		logger:          zerolog.Nop(),
		tokenCounter:    defaultTokenCounter,
		maxContextSize:  128000,
		reserveTokens:   4000,
		compressionRate: 0.7,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// Optimize optimizes the context to fit within limits.
func (o *Optimizer) Optimize(ctx *Context) (*Context, error) {
	o.logger.Debug().
		Int("current_tokens", ctx.TokenCount).
		Int("max_tokens", o.maxContextSize).
		Msg("optimizing context")

	availableTokens := o.maxContextSize - o.reserveTokens
	if availableTokens <= 0 {
		availableTokens = o.maxContextSize / 2
	}

	if ctx.TokenCount <= availableTokens {
		o.logger.Debug().
			Int("tokens", ctx.TokenCount).
			Msg("context already within budget, no optimization needed")
		return ctx, nil
	}

	// Apply optimization strategies in order
	ctx = o.compress(ctx)
	if ctx.TokenCount <= availableTokens {
		return ctx, nil
	}

	ctx = o.prioritize(ctx, availableTokens)
	if ctx.TokenCount <= availableTokens {
		return ctx, nil
	}

	ctx = o.summarize(ctx, availableTokens)

	o.logger.Debug().
		Int("final_tokens", ctx.TokenCount).
		Int("budget", availableTokens).
		Bool("truncated", ctx.Metadata.Truncated).
		Msg("context optimization complete")

	return ctx, nil
}

// compress removes redundancy from the context.
func (o *Optimizer) compress(ctx *Context) *Context {
	o.logger.Debug().Msg("compressing context")

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Compress documents
	wg.Add(1)
	go func() {
		defer wg.Done()
		compressed := o.compressDocuments(ctx.Documents)
		mu.Lock()
		ctx.Documents = compressed
		mu.Unlock()
	}()

	// Compress chunks
	wg.Add(1)
	go func() {
		defer wg.Done()
		compressed := o.compressChunks(ctx.Chunks)
		mu.Lock()
		ctx.Chunks = compressed
		mu.Unlock()
	}()

	// Remove duplicate entities
	wg.Add(1)
	go func() {
		defer wg.Done()
		ctx.Entities = o.deduplicateEntities(ctx.Entities)
	}()

	wg.Wait()

	// Recalculate tokens
	ctx.TokenCount = o.calculateTotalTokens(ctx)

	o.logger.Debug().
		Int("documents", len(ctx.Documents)).
		Int("chunks", len(ctx.Chunks)).
		Int("new_token_count", ctx.TokenCount).
		Msg("compression complete")

	return ctx
}

// compressDocuments removes redundant content from documents.
func (o *Optimizer) compressDocuments(docs []Document) []Document {
	if len(docs) == 0 {
		return nil
	}

	// Sort by score descending
	sorted := make([]Document, len(docs))
	copy(sorted, docs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Score > sorted[j].Score
	})

	// Remove documents with very low scores
	var result []Document
	seen := make(map[string]bool)

	for _, doc := range sorted {
		if doc.Score < 0.1 {
			continue
		}
		if seen[doc.ID] {
			continue
		}
		seen[doc.ID] = true

		// Truncate very long documents
		tokens := o.tokenCounter(doc.Content)
		maxTokens := 4000
		if tokens > maxTokens {
			// Keep only the most relevant portion
			lines := strings.Split(doc.Content, "\n")
			maxLines := maxTokens * 4 / 40 // rough estimate
			if len(lines) > maxLines {
				doc.Content = strings.Join(lines[:maxLines], "\n") + "\n... [truncated]"
			}
		}

		result = append(result, doc)
	}

	return result
}

// compressChunks removes redundant chunks.
func (o *Optimizer) compressChunks(chunks []Chunk) []Chunk {
	if len(chunks) == 0 {
		return nil
	}

	// Sort by score
	sorted := make([]Chunk, len(chunks))
	copy(sorted, chunks)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Score > sorted[j].Score
	})

	// Remove near-duplicate chunks (content overlap)
	var result []Chunk
	seenContent := make(map[string]bool)

	for _, chunk := range sorted {
		if chunk.Score < 0.1 {
			continue
		}

		// Simple dedup: check if similar content already exists
		contentHash := o.hashContent(chunk.Content)
		if seenContent[contentHash] {
			continue
		}
		seenContent[contentHash] = true

		result = append(result, chunk)
	}

	return result
}

// deduplicateEntities removes duplicate entities.
func (o *Optimizer) deduplicateEntities(entities []Entity) []Entity {
	seen := make(map[string]bool)
	var result []Entity
	for _, e := range entities {
		key := e.Name + ":" + e.Type
		if !seen[key] {
			seen[key] = true
			result = append(result, e)
		}
	}
	return result
}

// prioritize selects the most important information.
func (o *Optimizer) prioritize(ctx *Context, budget int) *Context {
	o.logger.Debug().Int("budget", budget).Msg("prioritizing context")

	// Calculate per-category budget (50% documents, 30% chunks, 10% symbols, 10% memory)
	docBudget := int(float64(budget) * 0.5)
	chunkBudget := int(float64(budget) * 0.3)
	symbolBudget := int(float64(budget) * 0.1)
	memBudget := int(float64(budget) * 0.1)

	// Prioritize documents
	ctx.Documents = o.prioritizeDocuments(ctx.Documents, docBudget)

	// Prioritize chunks
	ctx.Chunks = o.prioritizeChunks(ctx.Chunks, chunkBudget)

	// Prioritize symbols
	ctx.Symbols = o.prioritizeSymbols(ctx.Symbols, symbolBudget)

	// Prioritize memory
	ctx.Memory = o.prioritizeMemory(ctx.Memory, memBudget)

	// Recalculate
	ctx.TokenCount = o.calculateTotalTokens(ctx)
	ctx.Metadata.Truncated = ctx.TokenCount >= budget

	return ctx
}

// prioritizeDocuments selects documents up to the budget.
func (o *Optimizer) prioritizeDocuments(docs []Document, budget int) []Document {
	if len(docs) == 0 || budget <= 0 {
		return nil
	}

	// Already sorted by score
	var result []Document
	tokens := 0

	for _, doc := range docs {
		docTokens := o.tokenCounter(doc.Content)
		if docTokens > 4000 {
			docTokens = 4000
		}
		if tokens+docTokens <= budget {
			doc.Tokens = docTokens
			result = append(result, doc)
			tokens += docTokens
		}
	}

	return result
}

// prioritizeChunks selects chunks up to the budget.
func (o *Optimizer) prioritizeChunks(chunks []Chunk, budget int) []Chunk {
	if len(chunks) == 0 || budget <= 0 {
		return nil
	}

	var result []Chunk
	tokens := 0

	for _, chunk := range chunks {
		chunkTokens := o.tokenCounter(chunk.Content)
		if tokens+chunkTokens <= budget {
			chunk.Tokens = chunkTokens
			result = append(result, chunk)
			tokens += chunkTokens
		}
	}

	return result
}

// prioritizeSymbols selects symbols up to the budget.
func (o *Optimizer) prioritizeSymbols(symbols []Symbol, budget int) []Symbol {
	if len(symbols) == 0 || budget <= 0 {
		return nil
	}

	var result []Symbol
	tokens := 0

	for _, sym := range symbols {
		symTokens := o.tokenCounter(sym.Signature) + o.tokenCounter(sym.Documentation) + 10
		if tokens+symTokens <= budget {
			result = append(result, sym)
			tokens += symTokens
		}
	}

	return result
}

// prioritizeMemory selects memories up to the budget.
func (o *Optimizer) prioritizeMemory(memories []MemoryRecord, budget int) []MemoryRecord {
	if len(memories) == 0 || budget <= 0 {
		return nil
	}

	var result []MemoryRecord
	tokens := 0

	for _, mem := range memories {
		memTokens := o.tokenCounter(mem.Content)
		if tokens+memTokens <= budget {
			result = append(result, mem)
			tokens += memTokens
		}
	}

	return result
}

// summarize creates summaries when content needs to be further reduced.
func (o *Optimizer) summarize(ctx *Context, budget int) *Context {
	o.logger.Debug().Int("budget", budget).Msg("summarizing context")

	// If we're still over budget, aggressively prune
	if ctx.TokenCount > budget {
		// Keep only top 3 documents, top 5 chunks, top 10 symbols
		if len(ctx.Documents) > 3 {
			ctx.Documents = ctx.Documents[:3]
		}
		if len(ctx.Chunks) > 5 {
			ctx.Chunks = ctx.Chunks[:5]
		}
		if len(ctx.Symbols) > 10 {
			ctx.Symbols = ctx.Symbols[:10]
		}
		if len(ctx.Memory) > 5 {
			ctx.Memory = ctx.Memory[:5]
		}

		// For remaining documents, truncate to first 1000 chars
		for i := range ctx.Documents {
			if len(ctx.Documents[i].Content) > 1000 {
				ctx.Documents[i].Content = ctx.Documents[i].Content[:1000] + "\n... [summarized]"
			}
		}
	}

	ctx.TokenCount = o.calculateTotalTokens(ctx)
	ctx.Metadata.Truncated = true

	return ctx
}

// TokenCount counts tokens in the given text.
func (o *Optimizer) TokenCount(text string) int {
	return o.tokenCounter(text)
}

// ContextWindowRemaining calculates remaining tokens in the context window.
func (o *Optimizer) ContextWindowRemaining(ctx *Context) int {
	used := ctx.TokenCount
	maxContext := o.maxContextSize - o.reserveTokens
	remaining := maxContext - used
	if remaining < 0 {
		return 0
	}
	return remaining
}

// calculateTotalTokens calculates total tokens across all context items.
func (o *Optimizer) calculateTotalTokens(ctx *Context) int {
	total := 0
	for _, doc := range ctx.Documents {
		total += o.tokenCounter(doc.Content)
	}
	for _, chunk := range ctx.Chunks {
		total += o.tokenCounter(chunk.Content)
	}
	for _, sym := range ctx.Symbols {
		total += o.tokenCounter(sym.Signature) + o.tokenCounter(sym.Documentation) + 20
	}
	for _, mem := range ctx.Memory {
		total += o.tokenCounter(mem.Content)
	}
	return total
}

// hashContent creates a simple hash for content deduplication.
func (o *Optimizer) hashContent(content string) string {
	// Simple approach: use first and last 50 chars as a fingerprint
	if len(content) <= 100 {
		return content
	}
	return content[:50] + content[len(content)-50:]
}

// EstimateTokens provides a quick token estimate for a text.
func EstimateTokens(text string) int {
	return int(math.Ceil(float64(len(text)) / 4.0))
}

// TruncateToTokens truncates text to fit within a token budget.
func TruncateToTokens(text string, maxTokens int) string {
	tokens := EstimateTokens(text)
	if tokens <= maxTokens {
		return text
	}

	// Truncate to approximate length
	maxChars := maxTokens * 4
	if maxChars >= len(text) {
		return text
	}

	// Try to break at a sentence or paragraph boundary
	truncated := text[:maxChars]

	// Find the last sentence boundary
	lastPeriod := strings.LastIndex(truncated, ".")
	if lastPeriod > maxChars/2 {
		return truncated[:lastPeriod+1] + "\n... [truncated]"
	}

	// Find the last newline
	lastNewline := strings.LastIndex(truncated, "\n")
	if lastNewline > maxChars/2 {
		return truncated[:lastNewline] + "\n... [truncated]"
	}

	return truncated + "\n... [truncated]"
}
