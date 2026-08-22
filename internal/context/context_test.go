package context

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
)

func TestIntentTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		intent  IntentType
		wantStr string
	}{
		{"question", IntentQuestion, "question"},
		{"feature", IntentFeature, "feature"},
		{"bug", IntentBug, "bug"},
		{"refactor", IntentRefactor, "refactor"},
		{"review", IntentReview, "review"},
		{"deploy", IntentDeploy, "deploy"},
		{"docs", IntentDocs, "docs"},
		{"explore", IntentExplore, "explore"},
		{"search", IntentSearch, "search"},
		{"execute", IntentExecute, "execute"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if string(tc.intent) != tc.wantStr {
				t.Errorf("IntentType(%s) = %q, want %q", tc.name, tc.intent, tc.wantStr)
			}
		})
	}
}

func TestNewBuilder(t *testing.T) {
	t.Parallel()
	b := NewBuilder(zerolog.Nop())
	if b == nil {
		t.Fatal("NewBuilder returned nil")
	}
	if b.tokenCounter == nil {
		t.Error("tokenCounter should not be nil")
	}
}

func TestBuilderOptions(t *testing.T) {
	t.Parallel()

	called := false
	counter := func(_ string) int {
		called = true
		return 42
	}

	b := NewBuilder(zerolog.Nop(), WithTokenCounter(counter))
	if b.tokenCounter == nil {
		t.Fatal("tokenCounter should be set")
	}

	result := b.tokenCounter("test")
	if result != 42 {
		t.Errorf("tokenCounter result = %d, want 42", result)
	}
	if !called {
		t.Error("custom tokenCounter was not used")
	}
}

func TestDefaultTokenCounter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"hello", 1},
		{"hello world", 2},
		{"abcdefgh", 2},
	}

	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			got := defaultTokenCounter(tc.input)
			if got != tc.want {
				t.Errorf("defaultTokenCounter(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestTruncateQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		query  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"this is a long query", 10, "this is a " + "..."},
	}

	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			got := truncateQuery(tc.query, tc.maxLen)
			if got != tc.want {
				t.Errorf("truncateQuery(%q, %d) = %q, want %q", tc.query, tc.maxLen, got, tc.want)
			}
		})
	}
}

func TestExtractDocIDs(t *testing.T) {
	t.Parallel()

	docs := []Document{
		{ID: "doc1"},
		{ID: "doc2"},
		{ID: "doc3"},
	}

	ids := extractDocIDs(docs)
	if len(ids) != 3 {
		t.Fatalf("got %d ids, want 3", len(ids))
	}
	if ids[0] != "doc1" || ids[1] != "doc2" || ids[2] != "doc3" {
		t.Errorf("ids = %v, want [doc1 doc2 doc3]", ids)
	}
}

func TestExtractEntityIDs(t *testing.T) {
	t.Parallel()

	entities := []Entity{
		{Name: "entity1"},
		{Name: "entity2"},
	}

	ids := extractEntityIDs(entities)
	if len(ids) != 2 {
		t.Fatalf("got %d ids, want 2", len(ids))
	}
	if ids[0] != "entity1" || ids[1] != "entity2" {
		t.Errorf("ids = %v, want [entity1 entity2]", ids)
	}
}

func TestDeduplicateDocs(t *testing.T) {
	t.Parallel()

	docs := []Document{
		{ID: "a", Content: "first"},
		{ID: "b", Content: "second"},
		{ID: "a", Content: "duplicate"},
	}

	result := deduplicateDocs(docs)
	if len(result) != 2 {
		t.Fatalf("got %d docs, want 2", len(result))
	}
	if result[0].Content != "first" {
		t.Errorf("first doc content = %q, want %q", result[0].Content, "first")
	}
}

func TestDeduplicateChunks(t *testing.T) {
	t.Parallel()

	chunks := []Chunk{
		{ID: "1", Content: "chunk1"},
		{ID: "2", Content: "chunk2"},
		{ID: "1", Content: "dup"},
	}

	result := deduplicateChunks(chunks)
	if len(result) != 2 {
		t.Fatalf("got %d chunks, want 2", len(result))
	}
}

func TestDeduplicateSymbols(t *testing.T) {
	t.Parallel()

	symbols := []Symbol{
		{Name: "foo", File: "a.go"},
		{Name: "bar", File: "b.go"},
		{Name: "foo", File: "a.go"},
	}

	result := deduplicateSymbols(symbols)
	if len(result) != 2 {
		t.Fatalf("got %d symbols, want 2", len(result))
	}
}

func TestRankByRelevance(t *testing.T) {
	t.Parallel()

	docs := []Document{
		{ID: "low", Score: 1.0},
		{ID: "high", Score: 10.0},
		{ID: "mid", Score: 5.0},
	}

	rankByRelevance(docs, ContextRequest{})
	if docs[0].ID != "high" || docs[1].ID != "mid" || docs[2].ID != "low" {
		t.Errorf("docs order = %v, want [high mid low]", docIDs(docs))
	}
}

func TestRankByRelevanceChunks(t *testing.T) {
	t.Parallel()

	chunks := []Chunk{
		{ID: "a", Score: 2.0},
		{ID: "b", Score: 5.0},
		{ID: "c", Score: 1.0},
	}

	rankByRelevanceChunks(chunks, ContextRequest{})
	if chunks[0].ID != "b" {
		t.Errorf("top chunk = %s, want b", chunks[0].ID)
	}
}

func TestRankByRelevanceSymbols(t *testing.T) {
	t.Parallel()

	symbols := []Symbol{
		{Name: "x", Score: 10.0},
		{Name: "y", Score: 1.0},
	}

	rankByRelevanceSymbols(symbols, ContextRequest{})
	if symbols[0].Name != "x" {
		t.Errorf("top symbol = %s, want x", symbols[0].Name)
	}
}

func TestCalculateConfidence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ctx  *Context
		want float64
	}{
		{
			name: "empty context",
			ctx:  &Context{},
			want: 0.0,
		},
		{
			name: "with documents",
			ctx: &Context{
				Documents: []Document{
					{Score: 0.8},
					{Score: 0.6},
				},
			},
			want: 0.7,
		},
		{
			name: "with chunks only",
			ctx: &Context{
				Chunks: []Chunk{
					{Score: 1.0},
				},
			},
			want: 1.0,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := calculateConfidence(tc.ctx)
			if got != tc.want {
				t.Errorf("calculateConfidence = %f, want %f", got, tc.want)
			}
		})
	}
}

func TestAssembleWithinTokenBudget(t *testing.T) {
	t.Parallel()

	b := NewBuilder(zerolog.Nop())
	ctx := &Context{
		Documents: []Document{
			{ID: "d1", Content: "hello world"},
			{ID: "d2", Content: "a"},
		},
		Chunks: []Chunk{
			{ID: "c1", Content: "test content here"},
		},
		Symbols: []Symbol{
			{Name: "s1", Signature: "fn()", Documentation: "docs"},
		},
	}

	result := b.assembleWithinTokenBudget(ctx, 10)
	if result == nil {
		t.Fatal("assembleWithinTokenBudget returned nil")
	}
	if result.TokenCount == 0 {
		t.Error("TokenCount should not be zero")
	}
}

func TestAnalyzeIntent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	result, err := AnalyzeIntent(ctx, "fix a bug in the login", zerolog.Nop())
	if err != nil {
		t.Fatalf("AnalyzeIntent error: %v", err)
	}
	if result.Intent != IntentBug {
		t.Errorf("intent = %s, want %s", result.Intent, IntentBug)
	}
	if result.Confidence <= 0 {
		t.Error("confidence should be > 0")
	}
	if result.OriginalQuery != "fix a bug in the login" {
		t.Errorf("original query = %q", result.OriginalQuery)
	}
}

func TestAnalyzeIntentEmptyQuery(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	result, err := AnalyzeIntent(ctx, "", zerolog.Nop())
	if err != nil {
		t.Fatalf("AnalyzeIntent error: %v", err)
	}
	if result.Intent != IntentQuestion {
		t.Errorf("intent = %s, want %s", result.Intent, IntentQuestion)
	}
}

func TestClassifyIntent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		query    string
		wantType IntentType
	}{
		{"fix the crash bug", IntentBug},
		{"add new feature for login", IntentFeature},
		{"refactor the code base", IntentRefactor},
		{"review the pull request", IntentReview},
		{"deploy to production", IntentDeploy},
		{"write documentation", IntentDocs},
		{"explore the architecture", IntentExplore},
		{"search for the function", IntentSearch},
		{"run the test suite", IntentExecute},
		{"how does this work?", IntentQuestion},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.query[:10], func(t *testing.T) {
			t.Parallel()
			intent, _ := classifyIntent(tc.query)
			if intent != tc.wantType {
				t.Errorf("classifyIntent(%q) = %s, want %s", tc.query, intent, tc.wantType)
			}
		})
	}
}

func TestExtractEntities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		query string
		want  int
	}{
		{"no entities here", 0},
		{"camelCase is an identifier", 1},
		{"snake_case too", 1},
		{"the big TestFunction", 1},
	}

	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			got := extractEntities(tc.query)
			if len(got) != tc.want {
				t.Errorf("extractEntities(%q) = %v, want %d entities", tc.query, got, tc.want)
			}
		})
	}
}

func TestIsStopWord(t *testing.T) {
	t.Parallel()

	if !isStopWord("the") {
		t.Error("'the' should be a stopword")
	}
	if isStopWord("function") {
		t.Error("'function' should not be a stopword")
	}
}

func TestLookLikeIdentifier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		word string
		want bool
	}{
		{"camelCase", true},
		{"snake_case", true},
		{"ABC", true},
		{"hello", false},
		{"justwords", false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.word, func(t *testing.T) {
			t.Parallel()
			got := looksLikeIdentifier(tc.word)
			if got != tc.want {
				t.Errorf("looksLikeIdentifier(%q) = %v, want %v", tc.word, got, tc.want)
			}
		})
	}
}

func TestDetermineRequiredContext(t *testing.T) {
	t.Parallel()

	ctx := determineRequiredContext(IntentBug, []string{"login"})
	if len(ctx) < 2 {
		t.Errorf("expected at least 2 context types for bug, got %d", len(ctx))
	}

	ctx2 := determineRequiredContext(IntentQuestion, nil)
	if len(ctx2) < 1 {
		t.Error("expected at least 1 context type for question")
	}
}

func TestBuildSearchQuery(t *testing.T) {
	t.Parallel()

	query := buildSearchQuery("can you please help me find the login function", IntentQuestion, []string{"login"})
	if query == "" {
		t.Error("search query should not be empty")
	}
}

func TestNewContextAssembler(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler()
	if a == nil {
		t.Fatal("NewContextAssembler returned nil")
	}
	if a.maxChunkSize != 2000 {
		t.Errorf("maxChunkSize = %d, want 2000", a.maxChunkSize)
	}
	if a.maxDocSize != 8000 {
		t.Errorf("maxDocSize = %d, want 8000", a.maxDocSize)
	}
}

func TestContextAssemblerOptions(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler(
		WithMaxChunkSize(100),
		WithMaxDocSize(500),
	)
	if a.maxChunkSize != 100 {
		t.Errorf("maxChunkSize = %d, want 100", a.maxChunkSize)
	}
	if a.maxDocSize != 500 {
		t.Errorf("maxDocSize = %d, want 500", a.maxDocSize)
	}
}

func TestAssemblerSelectRelevantDocs(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler()
	docs := []Document{
		{ID: "1", Title: "login", Path: "/src/login.go", Score: 0.5},
		{ID: "2", Title: "logout", Path: "/src/logout.go", Score: 0.3},
	}

	selected := a.selectRelevantDocs(docs, ContextRequest{Query: "login", Filters: []string{"login"}})
	if len(selected) == 0 {
		t.Fatal("should select at least one doc")
	}
	if len(selected) > 0 && selected[0].ID != "1" {
		t.Errorf("top doc = %s, want 1", selected[0].ID)
	}
}

func TestAssemblerDeduplicateDocuments(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler()
	docs := []Document{
		{ID: "a"}, {ID: "b"}, {ID: "a"},
	}
	result := a.deduplicateDocuments(docs)
	if len(result) != 2 {
		t.Errorf("got %d, want 2", len(result))
	}
}

func TestAssemblerDeduplicateChunks(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler()
	chunks := []Chunk{
		{ID: "x"}, {ID: "y"}, {ID: "x"},
	}
	result := a.deduplicateChunks(chunks)
	if len(result) != 2 {
		t.Errorf("got %d, want 2", len(result))
	}
}

func TestAssemblerLimitToTokenBudget(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler()
	ctx := &Context{
		Documents: []Document{
			{ID: "d1", Content: "short"},
			{ID: "d2", Content: "this is a much longer document content here"},
		},
	}
	result := a.limitToTokenBudget(ctx, 5)
	if result == nil {
		t.Fatal("result should not be nil")
	}
}

func TestAssemblerMatchesFilters(t *testing.T) {
	t.Parallel()

	a := NewContextAssembler()
	doc := Document{Path: "/src/main.go", Title: "Main File"}

	if !a.matchesFilters(doc, []string{"main"}) {
		t.Error("should match filter 'main'")
	}
	if a.matchesFilters(doc, []string{"other"}) {
		t.Error("should not match filter 'other'")
	}
}

func TestNewOptimizer(t *testing.T) {
	t.Parallel()

	o := NewOptimizer()
	if o == nil {
		t.Fatal("NewOptimizer returned nil")
	}
	if o.maxContextSize != 128000 {
		t.Errorf("maxContextSize = %d, want 128000", o.maxContextSize)
	}
	if o.reserveTokens != 4000 {
		t.Errorf("reserveTokens = %d, want 4000", o.reserveTokens)
	}
}

func TestOptimizerOptions(t *testing.T) {
	t.Parallel()

	o := NewOptimizer(
		WithMaxContextSize(64000),
		WithReserveTokens(2000),
	)
	if o.maxContextSize != 64000 {
		t.Errorf("maxContextSize = %d, want 64000", o.maxContextSize)
	}
	if o.reserveTokens != 2000 {
		t.Errorf("reserveTokens = %d, want 2000", o.reserveTokens)
	}
}

func TestOptimizerOptimizeWithinBudget(t *testing.T) {
	t.Parallel()

	o := NewOptimizer(WithMaxContextSize(1000), WithReserveTokens(100))
	ctx := &Context{TokenCount: 50}
	result, err := o.Optimize(ctx)
	if err != nil {
		t.Fatalf("Optimize error: %v", err)
	}
	if result.TokenCount != 50 {
		t.Errorf("TokenCount = %d, want 50", result.TokenCount)
	}
}

func TestOptimizerTokenCount(t *testing.T) {
	t.Parallel()

	o := NewOptimizer()
	count := o.TokenCount("hello world")
	if count <= 0 {
		t.Errorf("TokenCount = %d, want > 0", count)
	}
}

func TestOptimizerContextWindowRemaining(t *testing.T) {
	t.Parallel()

	o := NewOptimizer(WithMaxContextSize(1000), WithReserveTokens(200))
	ctx := &Context{TokenCount: 300}
	remaining := o.ContextWindowRemaining(ctx)
	if remaining != 500 {
		t.Errorf("remaining = %d, want 500", remaining)
	}
}

func TestEstimateTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		text string
		want int
	}{
		{"", 0},
		{"hello", 2},
		{"hello world test", 4},
	}

	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			got := EstimateTokens(tc.text)
			if got != tc.want {
				t.Errorf("EstimateTokens(%q) = %d, want %d", tc.text, got, tc.want)
			}
		})
	}
}

func TestTruncateToTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		text      string
		maxTokens int
	}{
		{"short text", 100},
		{"this is a long text that might need truncation", 2},
	}

	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			result := TruncateToTokens(tc.text, tc.maxTokens)
			if result == "" {
				t.Error("result should not be empty")
			}
		})
	}
}

func TestHashContent(t *testing.T) {
	t.Parallel()

	o := NewOptimizer()
	h1 := o.hashContent("short")
	h2 := o.hashContent("short")
	if h1 != h2 {
		t.Error("same content should produce same hash")
	}
}

// docIDs is a helper to extract document IDs.
func docIDs(docs []Document) []string {
	ids := make([]string, len(docs))
	for i, d := range docs {
		ids[i] = d.ID
	}
	return ids
}
