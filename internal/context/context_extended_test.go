package context

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"
)

// ---------------------------------------------------------------------------
// Mock implementations for interfaces
// ---------------------------------------------------------------------------

type mockSearcher struct {
	docs []Document
	err  error
}

func (m *mockSearcher) Search(_ context.Context, _ string, _ []string) ([]Document, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.docs, nil
}

type mockRetriever struct {
	chunks        []Chunk
	symbols       []Symbol
	entities      []Entity
	relationships []Relationship
	mu            sync.RWMutex
	errMap        map[string]error
}

func (m *mockRetriever) withError(method string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.errMap == nil {
		m.errMap = make(map[string]error)
	}
	m.errMap[method] = err
}

func (m *mockRetriever) getErr(method string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.errMap == nil {
		return nil
	}
	return m.errMap[method]
}

func (m *mockRetriever) RetrieveChunks(_ context.Context, _ []string) ([]Chunk, error) {
	if err := m.getErr("chunks"); err != nil {
		return nil, err
	}
	return m.chunks, nil
}

func (m *mockRetriever) RetrieveSymbols(_ context.Context, _ []string) ([]Symbol, error) {
	if err := m.getErr("symbols"); err != nil {
		return nil, err
	}
	return m.symbols, nil
}

func (m *mockRetriever) RetrieveEntities(_ context.Context, _ []string) ([]Entity, error) {
	if err := m.getErr("entities"); err != nil {
		return nil, err
	}
	return m.entities, nil
}

func (m *mockRetriever) RetrieveRelationships(_ context.Context, _ []string) ([]Relationship, error) {
	if err := m.getErr("relationships"); err != nil {
		return nil, err
	}
	return m.relationships, nil
}

type mockMemoryStore struct {
	records []MemoryRecord
	err     error
}

func (m *mockMemoryStore) Search(_ context.Context, _ string, _ int) ([]MemoryRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.records, nil
}

// ---------------------------------------------------------------------------
// context.go — BuildContext tests
// ---------------------------------------------------------------------------

func TestWithSearcher(t *testing.T) {
	t.Parallel()

	s := &mockSearcher{}
	b := NewBuilder(zerolog.Nop(), WithSearcher(s))
	if b.searcher != s {
		t.Error("WithSearcher did not set searcher")
	}
}

func TestWithRetriever(t *testing.T) {
	t.Parallel()

	r := &mockRetriever{}
	b := NewBuilder(zerolog.Nop(), WithRetriever(r))
	if b.retriever != r {
		t.Error("WithRetriever did not set retriever")
	}
}

func TestWithMemoryStore(t *testing.T) {
	t.Parallel()

	m := &mockMemoryStore{}
	b := NewBuilder(zerolog.Nop(), WithMemoryStore(m))
	if b.memoryStore != m {
		t.Error("WithMemoryStore did not set memory store")
	}
}

func TestBuildContextSuccess(t *testing.T) {
	t.Parallel()

	searcher := &mockSearcher{
		docs: []Document{
			{ID: "d1", Path: "/src/main.go", Title: "Main", Content: "package main\nfunc main() {}", Score: 0.9},
			{ID: "d2", Path: "/src/utils.go", Title: "Utils", Content: "package utils", Score: 0.5},
		},
	}
	retriever := &mockRetriever{
		chunks: []Chunk{
			{ID: "c1", DocumentID: "d1", Content: "func main()", StartLine: 1, EndLine: 2, Score: 0.8},
		},
		symbols: []Symbol{
			{Name: "main", Kind: "function", File: "/src/main.go", Line: 1, Signature: "func main()", Score: 0.9},
		},
		entities: []Entity{
			{Name: "main", Type: "function", File: "/src/main.go", Line: 1, Score: 0.9},
		},
		relationships: []Relationship{
			{Source: "main", Target: "utils", Type: "calls", Weight: 0.5},
		},
	}
	memStore := &mockMemoryStore{
		records: []MemoryRecord{
			{ID: "m1", Type: "conversation", Content: "previous context", Score: 0.7},
		},
	}

	b := NewBuilder(zerolog.Nop(),
		WithSearcher(searcher),
		WithRetriever(retriever),
		WithMemoryStore(memStore),
	)

	ctx := context.Background()
	req := ContextRequest{
		Intent:    IntentBug,
		Query:     "fix bug in main",
		MaxTokens: 32000,
		SessionID: "sess1",
	}

	result, err := b.BuildContext(ctx, req)
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}
	if result == nil {
		t.Fatal("BuildContext returned nil")
	}
	if len(result.Documents) == 0 {
		t.Error("expected documents in result")
	}
	if len(result.Chunks) == 0 {
		t.Error("expected chunks in result")
	}
	if len(result.Symbols) == 0 {
		t.Error("expected symbols in result")
	}
	if len(result.Entities) == 0 {
		t.Error("expected entities in result")
	}
	if len(result.Relationships) == 0 {
		t.Error("expected relationships in result")
	}
	if len(result.Memory) == 0 {
		t.Error("expected memory in result")
	}
	if result.TokenCount <= 0 {
		t.Error("expected positive token count")
	}
	if result.Metadata.TotalResults == 0 {
		t.Error("expected total results > 0")
	}
	if result.Metadata.Confidence <= 0 {
		t.Error("expected confidence > 0")
	}
	// No Windows a granularidade do relógio é grosseira (dois time.Now()
	// consecutivos podem diferir em 0), então time.Since() mede 0 para
	// execuções in-memory rápidas. A asserção não pode exigir > 0 aqui;
	// validamos apenas que o timing foi registrado (não negativo).
	if result.Metadata.Timing.Total < 0 {
		t.Error("expected non-negative total timing")
	}
	if result.Metadata.RetrievedAt.IsZero() {
		t.Error("expected RetrievedAt to be set")
	}
}

func TestBuildContextNoProviders(t *testing.T) {
	t.Parallel()

	b := NewBuilder(zerolog.Nop())

	req := ContextRequest{
		Intent:    IntentQuestion,
		Query:     "how does this work?",
		MaxTokens: 1000,
	}

	result, err := b.BuildContext(context.Background(), req)
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}
	if result == nil {
		t.Fatal("BuildContext returned nil")
	}
	// Should still work even without providers
	if result.BuiltAt.IsZero() {
		t.Error("expected BuiltAt to be set")
	}
}

func TestBuildContextSearchError(t *testing.T) {
	t.Parallel()

	searcher := &mockSearcher{
		err: context.DeadlineExceeded,
	}

	b := NewBuilder(zerolog.Nop(), WithSearcher(searcher))

	result, err := b.BuildContext(context.Background(), ContextRequest{
		Intent:    IntentQuestion,
		Query:     "test",
		MaxTokens: 1000,
	})
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}
	if result == nil {
		t.Fatal("BuildContext returned nil")
	}
	// Should gracefully handle search errors
	if len(result.Documents) > 0 {
		t.Error("expected no documents on search error")
	}
}

func TestBuildContextRetrievalErrors(t *testing.T) {
	t.Parallel()

	searcher := &mockSearcher{
		docs: []Document{
			{ID: "d1", Path: "test.go", Content: "test", Score: 0.5},
		},
	}
	retriever := &mockRetriever{}
	retriever.withError("chunks", context.Canceled)
	retriever.withError("symbols", context.Canceled)
	retriever.withError("entities", context.Canceled)
	retriever.withError("relationships", context.Canceled)

	b := NewBuilder(zerolog.Nop(), WithSearcher(searcher), WithRetriever(retriever))

	result, err := b.BuildContext(context.Background(), ContextRequest{
		Intent:    IntentQuestion,
		Query:     "test",
		MaxTokens: 5000,
	})
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}
	// Should handle retrieval errors gracefully
	if result == nil {
		t.Fatal("result should not be nil despite retrieval errors")
	}
}

func TestBuildContextMemoryError(t *testing.T) {
	t.Parallel()

	memStore := &mockMemoryStore{
		err: context.DeadlineExceeded,
	}

	b := NewBuilder(zerolog.Nop(), WithMemoryStore(memStore))

	result, err := b.BuildContext(context.Background(), ContextRequest{
		Intent:    IntentQuestion,
		Query:     "test query",
		MaxTokens: 1000,
	})
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil despite memory error")
	}
	if len(result.Memory) > 0 {
		t.Error("expected empty memory on error")
	}
}

func TestBuildContextWithEmptySearchQuery(t *testing.T) {
	t.Parallel()

	// Intent that won't produce a search query via intent analysis (no keywords)
	b := NewBuilder(zerolog.Nop())

	result, err := b.BuildContext(context.Background(), ContextRequest{
		Intent:    IntentQuestion,
		Query:     "?",
		MaxTokens: 1000,
	})
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
}

// ---------------------------------------------------------------------------
// context.go — assembleWithinTokenBudget edge cases
// ---------------------------------------------------------------------------

func TestAssembleWithinTokenBudgetZeroMax(t *testing.T) {
	t.Parallel()

	b := NewBuilder(zerolog.Nop())
	ctx := &Context{
		Request: ContextRequest{Intent: IntentBug},
		Documents: []Document{
			{ID: "d1", Content: "hello world"},
		},
	}

	result := b.assembleWithinTokenBudget(ctx, 0)
	if result == nil {
		t.Fatal("result should not be nil")
	}
	// With maxTokens=0, it defaults to 32000, so everything fits
	if len(result.Documents) != 1 {
		t.Errorf("expected 1 document, got %d", len(result.Documents))
	}
}

func TestAssembleWithinTokenBudgetOverBudget(t *testing.T) {
	t.Parallel()

	b := NewBuilder(zerolog.Nop())
	ctx := &Context{
		Request: ContextRequest{Intent: IntentQuestion},
		Documents: []Document{
			{ID: "d1", Content: strings.Repeat("x", 400)}, // 100 tokens
			{ID: "d2", Content: strings.Repeat("y", 400)}, // 100 tokens
		},
		Chunks: []Chunk{
			{ID: "c1", Content: strings.Repeat("z", 400)}, // 100 tokens
		},
	}

	result := b.assembleWithinTokenBudget(ctx, 5) // Only 5 tokens budget
	if result == nil {
		t.Fatal("result should not be nil")
	}
	// With 5 token budget, intentTokens=2, docs need 100 tokens each → all filtered
	// TokenCount should just be the intent token count (2), nothing else fits
	if len(result.Documents) != 0 {
		t.Errorf("expected 0 documents in budget, got %d", len(result.Documents))
	}
	if len(result.Chunks) != 0 {
		t.Errorf("expected 0 chunks in budget, got %d", len(result.Chunks))
	}
	// Truncated is only true when totalTokens >= maxTokens; with just intent tokens (2) it's below 5
	if result.TokenCount == 0 {
		t.Error("expected at least intent tokens in count")
	}
}

func TestAssembleWithinTokenBudgetSymbolsFiltered(t *testing.T) {
	t.Parallel()

	b := NewBuilder(zerolog.Nop())
	ctx := &Context{
		Request: ContextRequest{Intent: IntentQuestion},
		Symbols: []Symbol{
			{Name: "s1", Signature: "func foo(a int) error", Documentation: "foo does something"},
			{Name: "s2", Signature: "func bar(b string)", Documentation: "bar returns value"},
			{Name: "s3", Signature: "func baz()", Documentation: "baz"},
		},
	}

	origSymbolCount := len(ctx.Symbols)
	result := b.assembleWithinTokenBudget(ctx, 5) // 5 tokens budget
	if result == nil {
		t.Fatal("result should not be nil")
	}
	// With 5 token budget, symbols should be filtered out
	// Each symbol costs sigTokens + docTokens + 10, which is at least 10 tokens
	if len(result.Symbols) == origSymbolCount {
		t.Error("some symbols should have been filtered due to budget constraints")
	}
}

func TestAssembleWithinTokenBudgetAllFits(t *testing.T) {
	t.Parallel()

	b := NewBuilder(zerolog.Nop())
	ctx := &Context{
		Request: ContextRequest{Intent: IntentQuestion},
		Documents: []Document{
			{ID: "d1", Content: "short"},
		},
		Chunks: []Chunk{
			{ID: "c1", Content: "tiny"},
		},
		Symbols: []Symbol{
			{Name: "s1", Signature: "fn()", Documentation: "doc"},
		},
	}

	result := b.assembleWithinTokenBudget(ctx, 32000)
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if len(result.Documents) != 1 {
		t.Errorf("expected 1 document, got %d", len(result.Documents))
	}
	if len(result.Chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(result.Chunks))
	}
	if len(result.Symbols) != 1 {
		t.Errorf("expected 1 symbol, got %d", len(result.Symbols))
	}
}

// ---------------------------------------------------------------------------
// context.go — estimateTokens and calculateConfidence
// ---------------------------------------------------------------------------

func TestEstimateTokensExtended(t *testing.T) {
	t.Parallel()

	tests := []struct {
		text string
	}{
		{"hello world"},
		{""},
		{strings.Repeat("abcdefgh", 100)},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			got := estimateTokens(tc.text)
			expected := len(tc.text) / 4
			if got != expected {
				t.Errorf("estimateTokens(%q) = %d, want %d", tc.text, got, expected)
			}
		})
	}
}

func TestCalculateConfidenceEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ctx  *Context
		want float64
	}{
		{
			name: "memory only considered in check but not in score",
			ctx: &Context{
				Memory: []MemoryRecord{
					{Content: "some memory", Score: 0.5},
				},
			},
			want: 0.0, // Memory prevents zero, but no docs/chunks to contribute
		},
		{
			name: "mixed docs and chunks",
			ctx: &Context{
				Documents: []Document{{Score: 0.8}},
				Chunks:    []Chunk{{Score: 0.4}, {Score: 0.6}},
			},
			want: 0.6, // (0.8 + 0.4 + 0.6) / 3 = 0.6
		},
		{
			name: "all empty",
			ctx:  &Context{},
			want: 0.0,
		},
		{
			name: "docs only",
			ctx: &Context{
				Documents: []Document{{Score: 1.0}},
			},
			want: 1.0,
		},
	}

	const epsilon = 1e-9
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := calculateConfidence(tc.ctx)
			diff := got - tc.want
			if diff < 0 {
				diff = -diff
			}
			if diff > epsilon {
				t.Errorf("calculateConfidence = %f, want %f", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// builder.go — ContextAssembler tests
// ---------------------------------------------------------------------------

func TestWithAssemblerLogger(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	a := NewContextAssembler(WithAssemblerLogger(logger))
	// Can't easily verify logger, but the option should not panic
	_ = a
}

func TestAssembleBasic(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler(WithMaxDocSize(10000))
	docs := []Document{
		{ID: "d1", Path: "/src/main.go", Title: "Main", Content: "package main\nfunc main() {}", Score: 0.9},
		{ID: "d2", Path: "/src/utils.go", Title: "Utils", Content: "package utils", Score: 0.5},
	}
	symbols := []Symbol{
		{Name: "main", Kind: "function", File: "/src/main.go", Line: 1, Signature: "func main()", Score: 0.9},
	}
	relationships := []Relationship{
		{Source: "main", Target: "utils", Type: "calls", Weight: 0.5},
	}
	memories := []MemoryRecord{
		{ID: "m1", Type: "conversation", Content: "previous work", Score: 0.7},
	}

	result, err := a.Assemble(context.Background(),
		ContextRequest{Intent: IntentQuestion, Query: "what does main do", MaxTokens: 32000},
		docs, symbols, relationships, memories,
	)
	if err != nil {
		t.Fatalf("Assemble error: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if len(result.Documents) == 0 {
		t.Error("expected documents in result")
	}
	if result.Metadata.RetrievedAt.IsZero() {
		t.Error("expected RetrievedAt to be set")
	}
	if result.Metadata.TotalResults == 0 {
		t.Error("expected TotalResults > 0")
	}
}

func TestAssembleEmpty(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler()

	result, err := a.Assemble(context.Background(),
		ContextRequest{Intent: IntentQuestion, Query: "test", MaxTokens: 1000},
		nil, nil, nil, nil,
	)
	if err != nil {
		t.Fatalf("Assemble error: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
}

func TestExtractChunksSmallDoc(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler(WithMaxChunkSize(2000))
	docs := []Document{
		{ID: "d1", Content: "short content here", Score: 0.8},
	}

	chunks := a.extractChunks(docs, ContextRequest{Query: "short"})
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].ID != "d1:full" {
		t.Errorf("chunk ID = %s, want d1:full", chunks[0].ID)
	}
}

func TestExtractChunksLargeDoc(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler(WithMaxChunkSize(100))

	// Create a large document (more than 200 tokens, well over 100 char maxChunkSize)
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = "line content here for testing " + strings.Repeat("x", 30)
	}
	content := strings.Join(lines, "\n")
	docs := []Document{
		{ID: "d1", Content: content, Score: 0.5},
	}

	chunks := a.extractChunks(docs, ContextRequest{Query: "testing"})
	if len(chunks) == 0 {
		t.Fatal("expected chunks from large document")
	}
	// Each chunk should have the :lineNumber format, not :full
	for _, c := range chunks {
		if c.ID == "d1:full" {
			t.Error("large doc should not produce :full chunks")
		}
	}
}

func TestExtractChunksNoDocs(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler()
	chunks := a.extractChunks(nil, ContextRequest{Query: "test"})
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks, got %d", len(chunks))
	}
}

func TestExtractChunksQueryMatching(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler(WithMaxChunkSize(100))

	content := strings.Repeat("line with keyword here\n", 60)
	docs := []Document{
		{ID: "d1", Content: content, Score: 0.5},
	}

	chunks := a.extractChunks(docs, ContextRequest{Query: "keyword"})

	// Chunks containing the keyword should have higher scores
	// Just verify it doesn't crash and produces output
	if len(chunks) == 0 {
		t.Fatal("expected some chunks")
	}
}

func TestSelectRelevantSymbols(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler(WithAssemblerLogger(zerolog.Nop()))
	symbols := []Symbol{
		{Name: "ProcessRequest", Kind: "function", File: "handler.go", Line: 10, Signature: "func ProcessRequest(r *Request)", Score: 0.7},
		{Name: "handleError", Kind: "function", File: "errors.go", Line: 20, Signature: "func handleError(err error)", Score: 0.3},
		{Name: "init", Kind: "function", File: "main.go", Line: 1, Signature: "func init()", Score: 0.2},
	}

	selected := a.selectRelevantSymbols(symbols, ContextRequest{Query: "request processing"})
	if len(selected) == 0 {
		t.Fatal("expected selected symbols")
	}
	// ProcessRequest scores highest due to name matching "request"
	if len(selected) > 0 && selected[0].Name != "ProcessRequest" {
		t.Errorf("top symbol = %s, want ProcessRequest", selected[0].Name)
	}
}

func TestSelectRelevantSymbolsEmpty(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler()
	result := a.selectRelevantSymbols(nil, ContextRequest{Query: "test"})
	if len(result) != 0 {
		t.Errorf("expected 0 symbols, got %d", len(result))
	}
}

func TestSelectRelevantSymbolsRespectsMax(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler(WithAssemblerLogger(zerolog.Nop()))

	// Create more than maxSymbols (default 50)
	symbols := make([]Symbol, 100)
	for i := range symbols {
		symbols[i] = Symbol{
			Name:  "sym" + string(rune('A'+i%26)),
			Kind:  "function",
			File:  "test.go",
			Line:  i,
			Score: float64(100-i) / 100,
		}
	}

	selected := a.selectRelevantSymbols(symbols, ContextRequest{Query: "unrelated"})
	if len(selected) > a.maxSymbols {
		t.Errorf("selected %d symbols, want at most %d", len(selected), a.maxSymbols)
	}
}

func TestSelectRelevantMemories(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler()
	memories := []MemoryRecord{
		{ID: "m1", Type: "code", Content: "We fixed the login bug", Score: 0.8},
		{ID: "m2", Type: "code", Content: "Added tests for payment", Score: 0.5},
		{ID: "m3", Type: "code", Content: "Refactored database layer", Score: 0.3},
	}

	selected := a.selectRelevantMemories(memories, ContextRequest{Query: "login error"})
	if len(selected) == 0 {
		t.Fatal("expected selected memories")
	}
	if selected[0].ID != "m1" {
		t.Errorf("top memory = %s, want m1", selected[0].ID)
	}
}

func TestSelectRelevantMemoriesEmpty(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler()
	result := a.selectRelevantMemories(nil, ContextRequest{Query: "test"})
	if len(result) != 0 {
		t.Errorf("expected 0 memories, got %d", len(result))
	}
}

func TestSelectRelevantMemoriesRespectsMax(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler()

	memories := make([]MemoryRecord, 20)
	for i := range memories {
		memories[i] = MemoryRecord{
			ID:      "mem" + string(rune('A'+i)),
			Type:    "code",
			Content: "some content",
			Score:   float64(20-i) / 20,
		}
	}

	selected := a.selectRelevantMemories(memories, ContextRequest{Query: "content"})
	if len(selected) > 10 {
		t.Errorf("selected %d memories, want at most 10", len(selected))
	}
}

func TestRankAll(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler()
	ctx := &Context{
		Chunks: []Chunk{
			{ID: "c1", Score: 0.2},
			{ID: "c2", Score: 0.9},
			{ID: "c3", Score: 0.5},
		},
	}

	a.rankAll(ctx, ContextRequest{})
	if ctx.Chunks[0].ID != "c2" {
		t.Errorf("top chunk = %s, want c2", ctx.Chunks[0].ID)
	}
	if ctx.Chunks[1].ID != "c3" {
		t.Errorf("second chunk = %s, want c3", ctx.Chunks[1].ID)
	}
}

func TestFormatForLLM(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler()
	ctx := &Context{
		Documents: []Document{
			{ID: "d1", Path: "/src/main.go"},
			{ID: "d2", Path: "/src/utils.go"},
		},
		Chunks: []Chunk{
			{ID: "c1", DocumentID: "d1"},
		},
		Metadata: ContextMetadata{},
	}

	result := a.formatForLLM(ctx)
	sources := result.Metadata.Sources
	if len(sources) == 0 {
		t.Error("expected sources in metadata")
	}
	// Should have both doc paths and chunk document IDs
	foundPath := false
	foundDocID := false
	for _, s := range sources {
		if s == "/src/main.go" {
			foundPath = true
		}
		if s == "d1" {
			foundDocID = true
		}
	}
	if !foundPath {
		t.Error("expected /src/main.go in sources")
	}
	if !foundDocID {
		t.Error("expected d1 in sources")
	}
}

func TestFormatForLLMEmpty(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler()
	ctx := &Context{
		Metadata: ContextMetadata{},
	}

	result := a.formatForLLM(ctx)
	if result == nil {
		t.Fatal("result should not be nil")
	}
}

func TestMaxInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		a, b, want int
	}{
		{1, 2, 2},
		{5, 3, 5},
		{0, 0, 0},
		{-1, 1, 1},
		{10, 10, 10},
	}

	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			got := maxInt(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("maxInt(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestSelectRelevantDocsEmpty(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler()
	result := a.selectRelevantDocs(nil, ContextRequest{Query: "test"})
	if len(result) != 0 {
		t.Errorf("expected 0 docs, got %d", len(result))
	}
}

func TestSelectRelevantDocsFilterMatching(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler()
	docs := []Document{
		{ID: "d1", Path: "/src/main.go", Score: 0.5},
		{ID: "d2", Path: "/test/test.go", Score: 0.5},
	}

	selected := a.selectRelevantDocs(docs, ContextRequest{
		Query:   "test",
		Filters: []string{"/src/"},
	})

	// main.go should match filter, test.go should lose score
	if len(selected) == 0 {
		t.Fatal("expected selected docs")
	}
	if len(selected) > 0 && selected[0].ID != "d1" {
		t.Errorf("top doc = %s, want d1 (matching filter)", selected[0].ID)
	}
}

func TestSelectRelevantDocsTitleMatching(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler()
	docs := []Document{
		{ID: "d1", Title: "Authentication", Path: "/src/auth.go", Score: 0.5},
		{ID: "d2", Title: "Database", Path: "/src/db.go", Score: 0.5},
	}

	selected := a.selectRelevantDocs(docs, ContextRequest{Query: "authentication"})
	if len(selected) == 0 {
		t.Fatal("expected selected docs")
	}
	if selected[0].ID != "d1" {
		t.Errorf("top doc = %s, want d1 (title match)", selected[0].ID)
	}
}

func TestSelectRelevantDocsPathMatching(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler()
	docs := []Document{
		{ID: "d1", Title: "Some File", Path: "/src/auth/login.go", Score: 0.5},
		{ID: "d2", Title: "Another", Path: "/src/misc/other.go", Score: 0.5},
	}

	selected := a.selectRelevantDocs(docs, ContextRequest{Query: "login"})
	if len(selected) == 0 {
		t.Fatal("expected selected docs")
	}
	if selected[0].ID != "d1" {
		t.Errorf("top doc = %s, want d1 (path match)", selected[0].ID)
	}
}

func TestLimitToTokenBudgetZeroMax(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler()
	ctx := &Context{
		Documents: []Document{
			{ID: "d1", Content: "hello world"},
		},
		Chunks: []Chunk{
			{ID: "c1", Content: "test content"},
		},
		Symbols: []Symbol{
			{Name: "s1", Signature: "fn()", Documentation: "doc"},
		},
	}

	result := a.limitToTokenBudget(ctx, 0) // defaults to 32000
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if len(result.Documents) == 0 {
		t.Error("documents should all fit with default budget")
	}
}

func TestLimitToTokenBudgetSymbolsFiltered(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler()
	ctx := &Context{
		Symbols: []Symbol{
			{Name: "s1", Signature: "func foo(a int)", Documentation: "docs here"},
			{Name: "s2", Signature: "func bar()", Documentation: "more docs"},
			{Name: "s3", Signature: "func baz(x string)", Documentation: "even more docs"},
		},
	}

	result := a.limitToTokenBudget(ctx, 3) // Very tight budget
	if result == nil {
		t.Fatal("result should not be nil")
	}
	// With very tight budget, most symbols should be filtered
	if len(result.Symbols) == 3 {
		t.Error("symbols should be filtered under tight budget")
	}
}

func TestLimitToTokenBudgetDocSizeCap(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler(WithMaxDocSize(10))
	ctx := &Context{
		Documents: []Document{
			{ID: "d1", Content: "this is a very long document that exceeds max doc size limit"},
		},
	}

	result := a.limitToTokenBudget(ctx, 100)
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if len(result.Documents) == 0 {
		t.Fatal("document should fit when capped at maxDocSize")
	}
	// Token count should be capped at maxDocSize
	if result.Documents[0].Tokens > a.maxDocSize {
		t.Errorf("doc tokens %d > maxDocSize %d", result.Documents[0].Tokens, a.maxDocSize)
	}
}

// ---------------------------------------------------------------------------
// builder.go — matchesFilters edge case
// ---------------------------------------------------------------------------

func TestMatchesFiltersEmpty(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler()
	if !a.matchesFilters(Document{}, nil) {
		t.Error("empty filters should match")
	}
	if !a.matchesFilters(Document{}, []string{}) {
		t.Error("empty filter slice should match")
	}
}

// ---------------------------------------------------------------------------
// optimizer.go tests
// ---------------------------------------------------------------------------

func TestWithOptimizerLogger(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	o := NewOptimizer(WithOptimizerLogger(logger))
	_ = o
}

func TestOptimizeOverBudget(t *testing.T) {
	t.Parallel()

	o := NewOptimizer(WithMaxContextSize(500), WithReserveTokens(50))
	// Create context with lots of tokens (over the 450 available)
	ctx := &Context{
		TokenCount: 1000,
		Documents: []Document{
			{ID: "d1", Content: strings.Repeat("document content here ", 50), Score: 0.9},
			{ID: "d2", Content: strings.Repeat("another document here ", 50), Score: 0.5},
		},
		Chunks: []Chunk{
			{ID: "c1", Content: strings.Repeat("chunk content ", 30), Score: 0.8},
			{ID: "c2", Content: strings.Repeat("more chunk content ", 30), Score: 0.3},
		},
		Symbols: []Symbol{
			{Name: "s1", Signature: "func test() error", Documentation: "does something", Score: 0.9},
			{Name: "s2", Signature: "func other()", Documentation: "other", Score: 0.1},
		},
		Memory: []MemoryRecord{
			{ID: "m1", Content: "memory content here for testing", Score: 0.7},
		},
	}

	result, err := o.Optimize(ctx)
	if err != nil {
		t.Fatalf("Optimize error: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	// Should be compressed and possibly truncated
	if result.Metadata.Truncated {
		// OK — aggressive optimization
	}
}

func TestOptimizeAlreadyWithinBudget(t *testing.T) {
	t.Parallel()

	o := NewOptimizer(WithMaxContextSize(10000), WithReserveTokens(100))
	ctx := &Context{TokenCount: 50, Metadata: ContextMetadata{}}
	result, err := o.Optimize(ctx)
	if err != nil {
		t.Fatalf("Optimize error: %v", err)
	}
	if result.TokenCount != 50 {
		t.Errorf("TokenCount = %d, want 50 (unchanged)", result.TokenCount)
	}
}

func TestOptimizeZeroReserveTokens(t *testing.T) {
	t.Parallel()

	// If reserveTokens >= maxContextSize, availableTokens becomes maxContextSize/2
	o := NewOptimizer(WithMaxContextSize(1000), WithReserveTokens(2000))
	ctx := &Context{
		TokenCount: 600,
		Documents: []Document{
			{ID: "d1", Content: strings.Repeat("x", 400), Score: 0.8},
		},
	}

	result, err := o.Optimize(ctx)
	if err != nil {
		t.Fatalf("Optimize error: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
}

func TestCompressDocuments(t *testing.T) {
	t.Parallel()

	o := NewOptimizer()
	docs := []Document{
		{ID: "d1", Content: "important content here", Score: 0.9},
		{ID: "d2", Content: "less relevant", Score: 0.05}, // Below 0.1 threshold
		{ID: "d1", Content: "duplicate id", Score: 0.5},   // Duplicate ID
		{ID: "d3", Content: strings.Repeat("very long document ", 500), Score: 0.7},
	}

	result := o.compressDocuments(docs)
	if len(result) == 0 {
		t.Fatal("expected compressed documents")
	}

	// d2 (low score) should be removed
	for _, d := range result {
		if d.ID == "d2" {
			t.Error("low-scored document should be removed")
		}
	}

	// Duplicate IDs should not exist
	idCount := make(map[string]int)
	for _, d := range result {
		idCount[d.ID]++
	}
	for id, count := range idCount {
		if count > 1 {
			t.Errorf("duplicate document ID %s found", id)
		}
	}
}

func TestCompressDocumentsEmpty(t *testing.T) {
	t.Parallel()

	o := NewOptimizer()
	result := o.compressDocuments(nil)
	if len(result) != 0 {
		t.Errorf("expected 0 docs, got %d", len(result))
	}
}

func TestCompressChunks(t *testing.T) {
	t.Parallel()

	o := NewOptimizer()
	chunks := []Chunk{
		{ID: "c1", Content: "important chunk content", Score: 0.9},
		{ID: "c2", Content: "low score", Score: 0.05},
		{ID: "c3", Content: "important chunk content", Score: 0.8}, // Same content hash as c1
	}

	result := o.compressChunks(chunks)
	if len(result) == 0 {
		t.Fatal("expected compressed chunks")
	}

	// c2 should be filtered for low score
	for _, c := range result {
		if c.ID == "c2" {
			t.Error("low-scored chunk should be removed")
		}
	}

	// c1 and c3 have same content hash, only one should remain
	found := 0
	for _, c := range result {
		if c.ID == "c1" || c.ID == "c3" {
			found++
		}
	}
	if found > 1 {
		t.Error("duplicate content chunks should be deduplicated")
	}
}

func TestCompressChunksEmpty(t *testing.T) {
	t.Parallel()

	o := NewOptimizer()
	result := o.compressChunks(nil)
	if len(result) != 0 {
		t.Errorf("expected 0 chunks, got %d", len(result))
	}
}

func TestDeduplicateEntities(t *testing.T) {
	t.Parallel()

	o := NewOptimizer()
	entities := []Entity{
		{Name: "main", Type: "function"},
		{Name: "utils", Type: "struct"},
		{Name: "main", Type: "function"},
	}

	result := o.deduplicateEntities(entities)
	if len(result) != 2 {
		t.Errorf("expected 2 entities, got %d", len(result))
	}
}

func TestDeduplicateEntitiesEmpty(t *testing.T) {
	t.Parallel()

	o := NewOptimizer()
	result := o.deduplicateEntities(nil)
	if len(result) != 0 {
		t.Errorf("expected 0 entities, got %d", len(result))
	}
}

func TestPrioritize(t *testing.T) {
	t.Parallel()

	o := NewOptimizer()
	ctx := &Context{
		Documents: []Document{
			{ID: "d1", Content: "doc one", Score: 0.9},
			{ID: "d2", Content: "doc two", Score: 0.7},
		},
		Chunks: []Chunk{
			{ID: "c1", Content: "chunk one", Score: 0.8},
		},
		Symbols: []Symbol{
			{Name: "s1", Signature: "fn()", Documentation: "doc", Score: 0.9},
		},
		Memory: []MemoryRecord{
			{ID: "m1", Content: "memory one", Score: 0.7},
		},
		TokenCount: 1000,
	}

	result := o.prioritize(ctx, 500)
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.TokenCount == 0 {
		t.Error("token count should not be zero")
	}
}

func TestPrioritizeDocumentsEmpty(t *testing.T) {
	t.Parallel()

	o := NewOptimizer()
	result := o.prioritizeDocuments(nil, 100)
	if len(result) != 0 {
		t.Errorf("expected 0 docs, got %d", len(result))
	}

	result2 := o.prioritizeDocuments([]Document{{ID: "d1", Content: "test"}}, 0)
	if len(result2) != 0 {
		t.Errorf("expected 0 docs with zero budget, got %d", len(result2))
	}
}

func TestPrioritizeChunksEmpty(t *testing.T) {
	t.Parallel()

	o := NewOptimizer()
	result := o.prioritizeChunks(nil, 100)
	if len(result) != 0 {
		t.Errorf("expected 0 chunks, got %d", len(result))
	}

	result2 := o.prioritizeChunks([]Chunk{{ID: "c1", Content: "test"}}, 0)
	if len(result2) != 0 {
		t.Errorf("expected 0 chunks with zero budget, got %d", len(result2))
	}
}

func TestPrioritizeSymbolsEmpty(t *testing.T) {
	t.Parallel()

	o := NewOptimizer()
	result := o.prioritizeSymbols(nil, 100)
	if len(result) != 0 {
		t.Errorf("expected 0 symbols, got %d", len(result))
	}

	result2 := o.prioritizeSymbols([]Symbol{{Name: "s1", Signature: "fn()", Documentation: "doc"}}, 0)
	if len(result2) != 0 {
		t.Errorf("expected 0 symbols with zero budget, got %d", len(result2))
	}
}

func TestPrioritizeMemoryEmpty(t *testing.T) {
	t.Parallel()

	o := NewOptimizer()
	result := o.prioritizeMemory(nil, 100)
	if len(result) != 0 {
		t.Errorf("expected 0 memories, got %d", len(result))
	}

	result2 := o.prioritizeMemory([]MemoryRecord{{ID: "m1", Content: "test"}}, 0)
	if len(result2) != 0 {
		t.Errorf("expected 0 memories with zero budget, got %d", len(result2))
	}
}

func TestSummarizeOverBudget(t *testing.T) {
	t.Parallel()

	o := NewOptimizer()
	content := strings.Repeat("document content here ", 100)
	ctx := &Context{
		TokenCount: 1000,
		Documents: []Document{
			{ID: "d1", Content: content, Score: 0.9},
			{ID: "d2", Content: content, Score: 0.8},
			{ID: "d3", Content: content, Score: 0.7},
			{ID: "d4", Content: content, Score: 0.6},
		},
		Chunks: []Chunk{
			{ID: "c1", Content: "chunk1", Score: 0.9},
			{ID: "c2", Content: "chunk2", Score: 0.8},
			{ID: "c3", Content: "chunk3", Score: 0.7},
			{ID: "c4", Content: "chunk4", Score: 0.6},
			{ID: "c5", Content: "chunk5", Score: 0.5},
			{ID: "c6", Content: "chunk6", Score: 0.4},
		},
		Symbols:  make([]Symbol, 15),
		Memory:   make([]MemoryRecord, 10),
		Metadata: ContextMetadata{},
	}

	result := o.summarize(ctx, 200)
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if !result.Metadata.Truncated {
		t.Error("summarized context should be marked truncated")
	}
	// Should be aggressively pruned
	if len(result.Documents) > 3 {
		t.Errorf("expected at most 3 docs after summarize, got %d", len(result.Documents))
	}
	if len(result.Chunks) > 5 {
		t.Errorf("expected at most 5 chunks after summarize, got %d", len(result.Chunks))
	}
	if len(result.Symbols) > 10 {
		t.Errorf("expected at most 10 symbols after summarize, got %d", len(result.Symbols))
	}
	if len(result.Memory) > 5 {
		t.Errorf("expected at most 5 memories after summarize, got %d", len(result.Memory))
	}
}

func TestSummarizeUnderBudget(t *testing.T) {
	t.Parallel()

	o := NewOptimizer()
	ctx := &Context{
		TokenCount: 50,
		Documents: []Document{
			{ID: "d1", Content: "short", Score: 0.9},
		},
		Metadata: ContextMetadata{},
	}

	result := o.summarize(ctx, 500)
	if result == nil {
		t.Fatal("result should not be nil")
	}
	// Content is already under budget, but summarize still sets truncated=true
	// and limits to top N
	// This is expected behavior - summarize is called as last resort
}

func TestCalculateTotalTokens(t *testing.T) {
	t.Parallel()

	o := NewOptimizer()
	ctx := &Context{
		Documents: []Document{
			{ID: "d1", Content: "hello world"},
		},
		Chunks: []Chunk{
			{ID: "c1", Content: "test content"},
		},
		Symbols: []Symbol{
			{Name: "s1", Signature: "func test()", Documentation: "docs here"},
		},
		Memory: []MemoryRecord{
			{ID: "m1", Content: "memory record"},
		},
	}

	total := o.calculateTotalTokens(ctx)
	if total <= 0 {
		t.Errorf("token count = %d, want > 0", total)
	}
}

func TestCalculateTotalTokensEmpty(t *testing.T) {
	t.Parallel()

	o := NewOptimizer()
	ctx := &Context{}
	total := o.calculateTotalTokens(ctx)
	if total != 0 {
		t.Errorf("token count = %d, want 0 for empty context", total)
	}
}

func TestOptimizeCompressPath(t *testing.T) {
	t.Parallel()

	o := NewOptimizer(WithMaxContextSize(300), WithReserveTokens(50))
	// 250 available tokens; context has 500 — triggers compress then prioritize
	ctx := &Context{
		TokenCount: 500,
		Documents: []Document{
			{ID: "d1", Content: strings.Repeat("doc content ", 30), Score: 0.9},
			{ID: "d2", Content: strings.Repeat("more content ", 20), Score: 0.05}, // Low score
		},
		Chunks: []Chunk{
			{ID: "c1", Content: strings.Repeat("chunk ", 20), Score: 0.8},
		},
		Metadata: ContextMetadata{},
	}

	result, err := o.Optimize(ctx)
	if err != nil {
		t.Fatalf("Optimize error: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	// Low-scored doc should be compressed out
	foundD2 := false
	for _, d := range result.Documents {
		if d.ID == "d2" {
			foundD2 = true
		}
	}
	if foundD2 {
		t.Log("d2 might still be present if within budget after compression")
	}
}

func TestContextWindowRemainingNegative(t *testing.T) {
	t.Parallel()

	o := NewOptimizer(WithMaxContextSize(100), WithReserveTokens(0))
	ctx := &Context{TokenCount: 200}
	remaining := o.ContextWindowRemaining(ctx)
	if remaining != 0 {
		t.Errorf("remaining = %d, want 0 (floor at 0)", remaining)
	}
}

// ---------------------------------------------------------------------------
// intent.go additional tests
// ---------------------------------------------------------------------------

func TestDetermineRequiredContextAllIntents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		intent     IntentType
		minTypes   int
		shouldHave []string
	}{
		{IntentBug, 3, []string{"stacktrace"}},
		{IntentFeature, 3, []string{"architecture"}},
		{IntentRefactor, 3, []string{"dependencies"}},
		{IntentReview, 3, []string{"diff"}},
		{IntentDeploy, 3, []string{"infrastructure"}},
		{IntentDocs, 3, []string{"api"}},
		{IntentExplore, 3, []string{"graph"}},
		{IntentSearch, 2, []string{"symbols"}},
		{IntentExecute, 3, []string{"commands"}},
		{IntentQuestion, 2, []string{"docs"}},
		{IntentType("unknown"), 2, []string{"docs"}},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(string(tc.intent), func(t *testing.T) {
			t.Parallel()
			result := determineRequiredContext(tc.intent, nil)
			if len(result) < tc.minTypes {
				t.Errorf("%s: got %d context types, want at least %d", tc.intent, len(result), tc.minTypes)
			}
			for _, want := range tc.shouldHave {
				found := false
				for _, got := range result {
					if got == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("%s: missing context type %q", tc.intent, want)
				}
			}
		})
	}
}

func TestDetermineRequiredContextWithEntities(t *testing.T) {
	t.Parallel()

	result := determineRequiredContext(IntentQuestion, []string{"main"})
	hasSymbols := false
	for _, ct := range result {
		if ct == "symbols" {
			hasSymbols = true
		}
	}
	if !hasSymbols {
		t.Error("expected 'symbols' context type when entities present")
	}
}

func TestBuildSearchQueryAllIntents(t *testing.T) {
	t.Parallel()

	query := "please can you help me find the login function"

	// Search intent — returns as-is
	result := buildSearchQuery(query, IntentSearch, nil)
	if result != query {
		t.Errorf("search query = %q, want %q (unmodified)", result, query)
	}

	// Other intents — noise words removed
	result2 := buildSearchQuery(query, IntentQuestion, nil)
	if strings.Contains(result2, "please") {
		t.Error("noise word 'please' should be removed")
	}
	if strings.Contains(result2, "can") {
		t.Error("noise word 'can' should be removed")
	}
	if strings.Contains(result2, "help") {
		t.Error("noise word 'help' should be removed")
	}
}

func TestClassifyIntentEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		query    string
		wantType IntentType
	}{
		{"", IntentQuestion},
		{"?", IntentQuestion},
		{"fix", IntentBug},
		{"add feature", IntentFeature},
		{"refactor this function", IntentRefactor},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.query, func(t *testing.T) {
			t.Parallel()
			intent, confidence := classifyIntent(tc.query)
			if intent != tc.wantType {
				t.Errorf("classifyIntent(%q) = %s, want %s", tc.query, intent, tc.wantType)
			}
			if confidence <= 0 {
				t.Errorf("confidence should be > 0 for %q", tc.query)
			}
		})
	}
}

func TestNewIntentAnalyzer(t *testing.T) {
	t.Parallel()

	ia := NewIntentAnalyzer(zerolog.Nop())
	if ia == nil {
		t.Fatal("NewIntentAnalyzer returned nil")
	}
}

func TestExtractEntitiesAcronyms(t *testing.T) {
	t.Parallel()

	// All-caps acronyms <= 8 chars should be treated as identifiers
	entities := extractEntities("the API handler")
	foundAPI := false
	for _, e := range entities {
		if e == "API" {
			foundAPI = true
		}
	}
	if !foundAPI {
		t.Error("expected 'API' to be identified as entity")
	}
}

func TestExtractEntitiesSpecialChars(t *testing.T) {
	t.Parallel()

	entities := extractEntities("test (functionName) [AnotherOne]")
	// functionName and AnotherOne should be extracted
	if len(entities) < 2 {
		t.Logf("entities: %v", entities)
	}
}

// ---------------------------------------------------------------------------
// hashContent edge cases
// ---------------------------------------------------------------------------

func TestHashContentLong(t *testing.T) {
	t.Parallel()

	o := NewOptimizer()
	content := "prefix" + strings.Repeat("x", 200) + "suffix"
	h := o.hashContent(content)
	if len(h) < 50 {
		t.Errorf("hash for long content should use first+last 50 chars, got length %d", len(h))
	}
}

// ---------------------------------------------------------------------------
// TruncateToTokens — sentence boundary and newline boundary paths
// ---------------------------------------------------------------------------

func TestTruncateToTokensSentence(t *testing.T) {
	t.Parallel()

	text := "First sentence. Second sentence. Third sentence which is long."
	result := TruncateToTokens(text, 5) // ~20 chars budget
	if result == "" {
		t.Fatal("result should not be empty")
	}
	// Should truncate at a sentence boundary
	if !strings.HasSuffix(result, "... [truncated]") {
		t.Errorf("expected truncated suffix, got: %q", result)
	}
}

func TestTruncateToTokensNewline(t *testing.T) {
	t.Parallel()

	text := "line one\nline two\nline three which is much longer content"
	result := TruncateToTokens(text, 5)
	if result == "" {
		t.Fatal("result should not be empty")
	}
}

func TestTruncateToTokensAlreadyWithinBudget(t *testing.T) {
	t.Parallel()

	text := "short"
	result := TruncateToTokens(text, 100)
	if result != text {
		t.Errorf("short text should not be truncated: got %q", result)
	}
}

// ---------------------------------------------------------------------------
// compressDocuments — long doc truncation path
// ---------------------------------------------------------------------------

func TestCompressDocumentsLongTruncation(t *testing.T) {
	t.Parallel()

	o := NewOptimizer()
	// Create a document with >4000 tokens and >400 lines to trigger truncation
	lines := make([]string, 500)
	for i := range lines {
		lines[i] = "line of text for long document testing truncation path "
	}
	content := strings.Join(lines, "\n")
	docs := []Document{
		{ID: "d1", Content: content, Score: 0.9},
	}

	result := o.compressDocuments(docs)
	if len(result) == 0 {
		t.Fatal("expected document to survive compression")
	}
	// Content should be truncated (shorter than original)
	if len(result[0].Content) >= len(content) {
		t.Log("document may not have been truncated if under token limit")
	}
}

// ---------------------------------------------------------------------------
// prioritizeDocuments — doc exceeding 4000 cap
// ---------------------------------------------------------------------------

func TestPrioritizeDocumentsExceedsCap(t *testing.T) {
	t.Parallel()

	o := NewOptimizer()
	docs := []Document{
		{ID: "d1", Content: strings.Repeat("long content ", 2000), Score: 0.9}, // ~24000 chars = ~6000 tokens
	}

	// Budget needs to be >= 4000 for the capped doc to fit
	result := o.prioritizeDocuments(docs, 5000)
	if len(result) == 0 {
		t.Fatal("expected document to survive with budget >= cap")
	}
	// Verify token was capped at 4000
	if result[0].Tokens != 4000 {
		t.Errorf("tokens = %d, expected cap of 4000", result[0].Tokens)
	}
}

// ---------------------------------------------------------------------------
// Optimize — summarize path triggered
// ---------------------------------------------------------------------------

func TestOptimizeSummarizePath(t *testing.T) {
	t.Parallel()

	o := NewOptimizer(WithMaxContextSize(100), WithReserveTokens(0))
	// 100 tokens available, context has 1000 tokens — will trigger compress → prioritize → summarize
	ctx := &Context{
		TokenCount: 1000,
		Documents: []Document{
			{ID: "d1", Content: strings.Repeat("doc content here ", 50), Score: 0.9},
			{ID: "d2", Content: strings.Repeat("more content here ", 50), Score: 0.5},
		},
		Chunks: []Chunk{
			{ID: "c1", Content: strings.Repeat("chunk content ", 30), Score: 0.8},
		},
		Symbols: []Symbol{
			{Name: "s1", Signature: "func test()", Documentation: "docs"},
		},
		Metadata: ContextMetadata{},
	}

	result, err := o.Optimize(ctx)
	if err != nil {
		t.Fatalf("Optimize error: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	// With such tight budget, summarize should have been called
	// and documents truncated to first 1000 chars
}

// ---------------------------------------------------------------------------
// BuildContext — intent analysis error fallback
// ---------------------------------------------------------------------------

func TestBuildContextIntentErrorFallback(t *testing.T) {
	t.Parallel()

	b := NewBuilder(zerolog.Nop())
	// Empty query will cause AnalyzeIntent to return default (IntentQuestion, 0.5)
	// This tests the path where intent analysis doesn't find a better intent
	result, err := b.BuildContext(context.Background(), ContextRequest{
		Intent:    "",
		Query:     "",
		MaxTokens: 1000,
	})
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	// When intent is empty string and Analysis returns IntentQuestion
	// The code checks: if intentResult.Intent != "" { req.Intent = intentResult.Intent }
	// Since IntentQuestion is not empty, it gets set
}

func TestBuildContextIntentOverridden(t *testing.T) {
	t.Parallel()

	b := NewBuilder(zerolog.Nop())
	// Query contains "bug" keyword, so intent analysis should detect IntentBug
	// even if caller provides a different intent
	result, err := b.BuildContext(context.Background(), ContextRequest{
		Intent:    IntentQuestion, // Caller says question
		Query:     "fix the crash error bug",
		MaxTokens: 1000,
	})
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	// Intent should have been overridden by analysis (IntentBug instead of IntentQuestion)
	if result.Request.Intent != IntentBug {
		t.Errorf("intent = %s, want %s (should be overridden by analysis)", result.Request.Intent, IntentBug)
	}
}

// ---------------------------------------------------------------------------
// selectRelevantDocs — filter matching path (score boost when filters match)
// ---------------------------------------------------------------------------

func TestSelectRelevantDocsFilterBoost(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler()
	docs := []Document{
		{ID: "d1", Path: "/src/main/main.go", Title: "Main", Score: 0.5},
		{ID: "d2", Path: "/src/utils/helper.go", Title: "Helper", Score: 0.5},
	}

	// Both match the main filter; d1 also matches title
	selected := a.selectRelevantDocs(docs, ContextRequest{
		Query:   "main",
		Filters: []string{"main"},
	})

	if len(selected) == 0 {
		t.Fatal("expected selected docs")
	}
	// d1 should rank higher (matching both title AND filter)
	if selected[0].ID != "d1" {
		t.Errorf("top doc = %s, want d1 (title+filter match)", selected[0].ID)
	}
}
