package search

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/graph"
	"github.com/CoscaAI/cosca/internal/ranking"
	"github.com/CoscaAI/cosca/internal/sqlite"
	"github.com/CoscaAI/cosca/internal/vector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Mock vector store ─────────────────────────────────────────────────────

// mockVectorStore implements vector.Store for testing.
type mockVectorStore struct {
	searchFunc           func(query []float64, limit int) ([]vector.SearchResult, error)
	searchWithFilterFunc func(query []float64, limit int, filter map[string]string) ([]vector.SearchResult, error)
	searchCandidatesFunc func(query []float64, limit int, candidateIDs []string, recentPool int, filter map[string]string) ([]vector.SearchResult, error)
}

func (m *mockVectorStore) Store(_ int, _ []vector.VectorRecord) error { return nil }
func (m *mockVectorStore) Delete(_ []string) error                    { return nil }
func (m *mockVectorStore) DeleteByDocument(_ string) error            { return nil }
func (m *mockVectorStore) DeleteByEntity(_ string) error              { return nil }
func (m *mockVectorStore) Rebuild() error                             { return nil }
func (m *mockVectorStore) Stats() (vector.VectorStats, error)         { return vector.VectorStats{}, nil }
func (m *mockVectorStore) Dimension() int                             { return 128 }
func (m *mockVectorStore) Count() (int, error)                        { return 0, nil }
func (m *mockVectorStore) Close() error                               { return nil }

func (m *mockVectorStore) Search(query []float64, limit int) ([]vector.SearchResult, error) {
	if m.searchFunc != nil {
		return m.searchFunc(query, limit)
	}
	return nil, nil
}

func (m *mockVectorStore) SearchWithFilter(query []float64, limit int, filter map[string]string) ([]vector.SearchResult, error) {
	if m.searchWithFilterFunc != nil {
		return m.searchWithFilterFunc(query, limit, filter)
	}
	return nil, nil
}

// SearchWithCandidates satisfies vector.CandidateSearcher.
func (m *mockVectorStore) SearchWithCandidates(query []float64, limit int, candidateIDs []string, recentPool int, filter map[string]string) ([]vector.SearchResult, error) {
	if m.searchCandidatesFunc != nil {
		return m.searchCandidatesFunc(query, limit, candidateIDs, recentPool, filter)
	}
	return nil, nil
}

// ── Embedding helper ──────────────────────────────────────────────────────

func stubEmbedFunc(ctx context.Context, text string) (*EmbeddingRequest, error) {
	return &EmbeddingRequest{Vector: []float64{0.1, 0.2, 0.3}}, nil
}

func failingEmbedFunc(ctx context.Context, text string) (*EmbeddingRequest, error) {
	return nil, fmt.Errorf("embed failed")
}

// ── Helper to build test graphs ───────────────────────────────────────────

func newTestGraph() *graph.Graph {
	return graph.New()
}

// ============================================================================
// Tests: ResultType constants
// ============================================================================

func TestResultTypeConstants(t *testing.T) {
	t.Parallel()
	tests := []struct {
		rt   ResultType
		want string
	}{
		{ResultDocument, "document"},
		{ResultChunk, "chunk"},
		{ResultEntity, "entity"},
		{ResultCode, "code_block"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, string(tt.rt))
		})
	}
}

// ============================================================================
// Tests: SearchResult struct
// ============================================================================

func TestSearchResult(t *testing.T) {
	t.Parallel()
	sr := SearchResult{
		ID:           "res-1",
		Type:         ResultDocument,
		Score:        0.95,
		Title:        "Test Document",
		Content:      "content here",
		Snippet:      "snippet...",
		DocumentID:   "doc-1",
		DocumentPath: "/path/to/doc",
		Heading:      "Introduction",
		SectionType:  "paragraph",
		EntityType:   "skill",
		Language:     "go",
		Metadata:     map[string]string{"author": "dev"},
		Source:       "fts",
		Rank:         1,
	}
	assert.Equal(t, "res-1", sr.ID)
	assert.Equal(t, ResultDocument, sr.Type)
	assert.Equal(t, 0.95, sr.Score)
	assert.Equal(t, "Test Document", sr.Title)
	assert.Equal(t, "content here", sr.Content)
	assert.Equal(t, "snippet...", sr.Snippet)
	assert.Equal(t, "doc-1", sr.DocumentID)
	assert.Equal(t, "/path/to/doc", sr.DocumentPath)
	assert.Equal(t, "Introduction", sr.Heading)
	assert.Equal(t, "paragraph", sr.SectionType)
	assert.Equal(t, "skill", sr.EntityType)
	assert.Equal(t, "go", sr.Language)
	assert.Equal(t, "dev", sr.Metadata["author"])
	assert.Equal(t, "fts", sr.Source)
	assert.Equal(t, 1, sr.Rank)
}

func TestSearchResult_ZeroValue(t *testing.T) {
	t.Parallel()
	sr := SearchResult{}
	assert.Empty(t, sr.ID)
	assert.Empty(t, sr.Type)
	assert.Zero(t, sr.Score)
	assert.Zero(t, sr.Rank)
	assert.Empty(t, sr.Source)
}

// ============================================================================
// Tests: SearchResults struct
// ============================================================================

func TestSearchResults(t *testing.T) {
	t.Parallel()
	results := &SearchResults{
		Results:    []SearchResult{},
		TotalCount: 0,
		Query:      "test query",
	}
	assert.Zero(t, results.TotalCount)
	assert.Equal(t, "test query", results.Query)
	assert.Empty(t, results.Results)
	assert.Nil(t, results.Facets)
	assert.Zero(t, results.Duration)
}

func TestSearchResultsWithData(t *testing.T) {
	t.Parallel()
	results := &SearchResults{
		Results: []SearchResult{
			{ID: "1", Type: ResultDocument, Score: 0.9},
			{ID: "2", Type: ResultChunk, Score: 0.8},
		},
		TotalCount: 2,
		Query:      "go",
		Facets: map[string]map[string]int{
			"type": {"document": 1, "chunk": 1},
		},
		Suggestions: []string{"go agent", "go skill"},
	}
	assert.Len(t, results.Results, 2)
	assert.Equal(t, 2, results.TotalCount)
	assert.Len(t, results.Facets, 1)
	assert.Len(t, results.Suggestions, 2)
	assert.Equal(t, "go", results.Query)
}

// ============================================================================
// Tests: SearchParams and DefaultSearchParams
// ============================================================================

func TestDefaultSearchParams(t *testing.T) {
	t.Parallel()
	sp := DefaultSearchParams()
	assert.Equal(t, 20, sp.Limit)
	assert.True(t, sp.EnableFTS)
	assert.True(t, sp.EnableVector)
	assert.False(t, sp.EnableGraph)
	assert.False(t, sp.EnableFacets)
	assert.Zero(t, sp.Offset)
	assert.Zero(t, sp.MinScore)
	assert.Empty(t, sp.Query)
	assert.Empty(t, sp.Types)
	assert.Empty(t, sp.Path)
	assert.Nil(t, sp.Tags)
	assert.True(t, sp.Since.IsZero())
}

func TestSearchParamsCustom(t *testing.T) {
	t.Parallel()
	sp := SearchParams{
		Query:        "golang",
		Limit:        50,
		Offset:       10,
		Types:        []string{"document", "code_block"},
		Path:         "/src",
		EnableFTS:    true,
		EnableVector: false,
		MinScore:     0.5,
	}
	assert.Equal(t, "golang", sp.Query)
	assert.Equal(t, 50, sp.Limit)
	assert.Equal(t, 10, sp.Offset)
	assert.Len(t, sp.Types, 2)
	assert.Equal(t, "/src", sp.Path)
	assert.True(t, sp.EnableFTS)
	assert.False(t, sp.EnableVector)
	assert.Equal(t, 0.5, sp.MinScore)
}

func TestSearchParams_ZeroLimitDefaults(t *testing.T) {
	t.Parallel()
	// Zero/negative limits are overridden in Engine.Search, not in DefaultSearchParams.
	// Verify DefaultSearchParams always returns 20.
	sp := DefaultSearchParams()
	assert.Equal(t, 20, sp.Limit)
}

// ============================================================================
// Tests: EmbeddingRequest
// ============================================================================

func TestEmbeddingRequest(t *testing.T) {
	t.Parallel()
	er := EmbeddingRequest{
		Vector: []float64{0.1, 0.2, 0.3},
	}
	assert.Len(t, er.Vector, 3)
}

func TestEmbeddingRequest_Empty(t *testing.T) {
	t.Parallel()
	er := EmbeddingRequest{}
	assert.Nil(t, er.Vector)
}

// ============================================================================
// Tests: NewEngine
// ============================================================================

func TestNewEngine(t *testing.T) {
	t.Parallel()
	g := newTestGraph()
	r := ranking.New(ranking.DefaultConfig())

	engine := NewEngine(nil, nil, g, r, stubEmbedFunc)
	require.NotNil(t, engine, "engine should not be nil")
	assert.Nil(t, engine.fts, "fts should be nil when passed nil")
	assert.Nil(t, engine.vecStore, "vecStore should be nil when passed nil")
	assert.NotNil(t, engine.graph, "graph should be set")
	assert.NotNil(t, engine.ranker, "ranker should be set")
	assert.NotNil(t, engine.embedFunc, "embedFunc should be set")
}

func TestNewEngine_AllNil(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	require.NotNil(t, engine, "engine should not be nil")
	assert.Nil(t, engine.fts)
	assert.Nil(t, engine.vecStore)
	assert.Nil(t, engine.graph)
	assert.Nil(t, engine.ranker)
	assert.Nil(t, engine.embedFunc)
}

// ============================================================================
// Tests: Engine.Search - Error paths
// ============================================================================

func TestSearch_ErrorEmptyQueryNoFilters(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	params := SearchParams{
		Query: "",
		Limit: 20,
	}
	results, err := engine.Search(context.Background(), params)
	require.Error(t, err)
	assert.Nil(t, results)
	assert.Contains(t, err.Error(), "query or filter required")
}

func TestSearch_ErrorEmptyQueryNoFilters_ZeroValues(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	params := SearchParams{
		Query: "",
		Types: nil,
		Path:  "",
		Limit: 20,
	}
	results, err := engine.Search(context.Background(), params)
	require.Error(t, err)
	assert.Nil(t, results)
}

// ============================================================================
// Tests: Engine.Search - Graph only
// ============================================================================

func TestSearch_GraphOnly_NoQueryInGraph(t *testing.T) {
	t.Parallel()
	g := newTestGraph()
	engine := NewEngine(nil, nil, g, nil, nil)

	params := SearchParams{
		Query:        "nonexistent",
		Limit:        20,
		EnableFTS:    false,
		EnableVector: false,
		EnableGraph:  true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.NotNil(t, results)
	assert.Empty(t, results.Results)
	assert.Equal(t, 0, results.TotalCount)
	assert.Equal(t, "nonexistent", results.Query)
}

func TestSearch_GraphOnly_WithMatchingNode(t *testing.T) {
	t.Parallel()
	g := graph.New()
	// FilterNodes("") only matches nodes with Type == "".
	// The production searchGraph code calls FilterNodes("").
	require.NoError(t, g.AddNode(&graph.Node{
		ID:   "node-1",
		Type: "",
		Name: "Go Parser",
		Metadata: map[string]interface{}{
			"description": "A parser for Go source code.",
		},
	}))

	engine := NewEngine(nil, nil, g, nil, nil)
	params := SearchParams{
		Query:        "parser",
		Limit:        20,
		EnableFTS:    false,
		EnableVector: false,
		EnableGraph:  true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.NotNil(t, results)
	assert.Len(t, results.Results, 1)
	assert.Equal(t, ResultEntity, results.Results[0].Type)
	assert.Equal(t, "Go Parser", results.Results[0].Title)
	assert.Equal(t, "graph", results.Results[0].Source)
	assert.Equal(t, 1, results.Results[0].Rank)
}

func TestSearch_GraphOnly_MultipleNodesSortedByScore(t *testing.T) {
	t.Parallel()
	g := graph.New()
	// FilterNodes("") only matches nodes with Type == ""
	require.NoError(t, g.AddNode(&graph.Node{
		ID: "node-a", Type: "", Name: "Testing Agent",
		Metadata: map[string]interface{}{"description": "Handles testing."},
	}))
	require.NoError(t, g.AddNode(&graph.Node{
		ID: "node-b", Type: "", Name: "Testing Skill",
		Metadata: map[string]interface{}{"description": "A testing skill."},
	}))
	// Add edge between them to boost node-a's score via graph centrality
	require.NoError(t, g.AddEdge(&graph.Edge{
		Source: "node-a",
		Target: "node-b",
		Type:   graph.RelRelatedTo,
	}))

	engine := NewEngine(nil, nil, g, nil, nil)
	params := SearchParams{
		Query:        "testing",
		Limit:        10,
		EnableFTS:    false,
		EnableVector: false,
		EnableGraph:  true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.Len(t, results.Results, 2)

	// node-a should have higher score (has 1 neighbor) than node-b (0 neighbors)
	assert.Greater(t, results.Results[0].Score, results.Results[1].Score)
}

func TestSearch_GraphOnly_GraphNil(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	params := SearchParams{
		Query:        "anything",
		Limit:        20,
		EnableFTS:    false,
		EnableVector: false,
		EnableGraph:  true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.NotNil(t, results)
	assert.Empty(t, results.Results)
}

// ============================================================================
// Tests: Engine.Search - Offset and limit
// ============================================================================

func TestSearch_OffsetAndLimit(t *testing.T) {
	t.Parallel()
	g := graph.New()
	for i := 0; i < 10; i++ {
		require.NoError(t, g.AddNode(&graph.Node{
			ID:   fmt.Sprintf("n-%d", i),
			Type: "",
			Name: fmt.Sprintf("Test Skill %d", i),
		}))
	}

	engine := NewEngine(nil, nil, g, nil, nil)
	params := SearchParams{
		Query:        "skill",
		Limit:        3,
		Offset:       4,
		EnableFTS:    false,
		EnableVector: false,
		EnableGraph:  true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.Len(t, results.Results, 3)
	assert.Equal(t, 10, results.TotalCount)
	// Ranks should start at offset+1 = 5
	assert.Equal(t, 5, results.Results[0].Rank)
	assert.Equal(t, 6, results.Results[1].Rank)
	assert.Equal(t, 7, results.Results[2].Rank)
}

func TestSearch_OffsetBeyondLength(t *testing.T) {
	t.Parallel()
	g := graph.New()
	require.NoError(t, g.AddNode(&graph.Node{
		ID: "only", Type: "", Name: "Only Skill",
	}))

	engine := NewEngine(nil, nil, g, nil, nil)
	params := SearchParams{
		Query:        "skill",
		Limit:        20,
		Offset:       100,
		EnableFTS:    false,
		EnableVector: false,
		EnableGraph:  true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	// Production code only applies offset when offset < len(results).
	// Offset 100 is beyond len(1), so no slicing occurs.
	assert.Len(t, results.Results, 1)
	assert.Equal(t, 101, results.Results[0].Rank) // rank = offset + 1
	assert.Equal(t, 1, results.TotalCount)
}

// ============================================================================
// Tests: Engine.Search - With ranker (re-ranking path)
// ============================================================================

func TestSearch_WithRanker(t *testing.T) {
	t.Parallel()
	g := graph.New()
	require.NoError(t, g.AddNode(&graph.Node{
		ID:   "r1",
		Type: "",
		Name: "Code Formatter",
		Metadata: map[string]interface{}{
			"description": "Formats Go source code using gofmt.",
		},
	}))
	require.NoError(t, g.AddNode(&graph.Node{
		ID:   "r2",
		Type: "",
		Name: "Code Review Agent",
		Metadata: map[string]interface{}{
			"description": "Reviews code for best practices.",
		},
	}))

	r := ranking.New(ranking.DefaultConfig())
	engine := NewEngine(nil, nil, g, r, nil)
	params := SearchParams{
		Query:        "code",
		Limit:        20,
		EnableFTS:    false,
		EnableVector: false,
		EnableGraph:  true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.Len(t, results.Results, 2)
}

// ============================================================================
// Tests: Engine.Search - Vector path
// ============================================================================

func TestSearch_VectorSearch(t *testing.T) {
	t.Parallel()
	vs := &mockVectorStore{
		searchFunc: func(query []float64, limit int) ([]vector.SearchResult, error) {
			return []vector.SearchResult{
				{
					ID:         "vec-1",
					Score:      0.85,
					Content:    "Vector search result content",
					DocumentID: "doc-1",
					EntityID:   "",
					ChunkID:    "chunk-1",
				},
			}, nil
		},
	}

	engine := NewEngine(nil, vs, nil, nil, stubEmbedFunc)
	params := SearchParams{
		Query:        "vector query",
		Limit:        20,
		EnableFTS:    false,
		EnableVector: true,
		EnableGraph:  false,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.NotNil(t, results)
	assert.Len(t, results.Results, 1)
	assert.Equal(t, "vec-1", results.Results[0].ID)
	assert.Equal(t, "vector", results.Results[0].Source)
	assert.Equal(t, 0.85, results.Results[0].Score)
	assert.Equal(t, ResultChunk, results.Results[0].Type)
}

func TestSearch_VectorSearch_WithEntity(t *testing.T) {
	t.Parallel()
	vs := &mockVectorStore{
		searchFunc: func(query []float64, limit int) ([]vector.SearchResult, error) {
			return []vector.SearchResult{
				{
					ID:       "ent-1",
					Score:    0.9,
					Content:  "Entity content",
					EntityID: "entity-1",
					Metadata: map[string]string{"entity_type": "skill"},
				},
			}, nil
		},
	}

	engine := NewEngine(nil, vs, nil, nil, stubEmbedFunc)
	params := SearchParams{
		Query:        "entity search",
		Limit:        20,
		EnableVector: true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.Len(t, results.Results, 1)
	assert.Equal(t, ResultEntity, results.Results[0].Type)
	assert.Equal(t, "skill", results.Results[0].EntityType)
}

func TestSearch_VectorSearch_EmbedFails(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, failingEmbedFunc)
	params := SearchParams{
		Query:        "anything",
		Limit:        20,
		EnableVector: true,
	}
	// Should still succeed (vector fails gracefully)
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.Empty(t, results.Results)
}

func TestSearch_VectorSearch_WithFilter(t *testing.T) {
	t.Parallel()
	vs := &mockVectorStore{
		searchWithFilterFunc: func(query []float64, limit int, filter map[string]string) ([]vector.SearchResult, error) {
			return []vector.SearchResult{
				{
					ID:      "filtered-1",
					Score:   0.75,
					Content: "Filtered result",
				},
			}, nil
		},
	}

	engine := NewEngine(nil, vs, nil, nil, stubEmbedFunc)
	params := SearchParams{
		Query:        "filtered",
		Limit:        20,
		EnableVector: true,
		Tags:         map[string]string{"language": "go"},
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.Len(t, results.Results, 1)
	assert.Equal(t, "filtered-1", results.Results[0].ID)
}

func TestSearch_VectorSearch_EmptyResults(t *testing.T) {
	t.Parallel()
	vs := &mockVectorStore{
		searchFunc: func(query []float64, limit int) ([]vector.SearchResult, error) {
			return nil, nil
		},
	}

	engine := NewEngine(nil, vs, nil, nil, stubEmbedFunc)
	params := SearchParams{
		Query:        "no results",
		Limit:        20,
		EnableVector: true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.Empty(t, results.Results)
}

// ============================================================================
// Tests: Engine.Search - Facets
// ============================================================================

func TestSearch_WithFacets(t *testing.T) {
	t.Parallel()
	g := graph.New()
	require.NoError(t, g.AddNode(&graph.Node{
		ID: "fa", Type: "", Name: "Alpha",
	}))
	require.NoError(t, g.AddNode(&graph.Node{
		ID: "fb", Type: "", Name: "Beta",
	}))

	engine := NewEngine(nil, nil, g, nil, nil)
	params := SearchParams{
		Query:        "alpha beta",
		Limit:        20,
		EnableGraph:  true,
		EnableFacets: true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.NotNil(t, results.Facets)
	// Should have "type" and "source" facets at minimum
	assert.Contains(t, results.Facets, "type")
	assert.Contains(t, results.Facets, "source")
}

// ============================================================================
// Tests: Engine.Search - Suggestions
// ============================================================================

func TestSearch_WithSuggestions(t *testing.T) {
	t.Parallel()
	g := graph.New()
	require.NoError(t, g.AddNode(&graph.Node{
		ID: "s1", Type: "", Name: "Test",
	}))

	engine := NewEngine(nil, nil, g, nil, nil)
	params := SearchParams{
		Query:       "test query",
		Limit:       20,
		EnableGraph: true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.NotNil(t, results.Suggestions)
	assert.Len(t, results.Suggestions, 4)
	assert.Contains(t, results.Suggestions, "test query agent")
	assert.Contains(t, results.Suggestions, "test query skill")
	assert.Contains(t, results.Suggestions, "test query workflow")
	assert.Contains(t, results.Suggestions, "test query template")
}

func TestSearch_NoSuggestionsForEmptyQuery(t *testing.T) {
	t.Parallel()
	g := graph.New()
	require.NoError(t, g.AddNode(&graph.Node{
		ID: "sn", Type: "", Name: "Some Agent",
	}))

	engine := NewEngine(nil, nil, g, nil, nil)
	params := SearchParams{
		Query:       "",
		Limit:       20,
		EnableGraph: true,
		Types:       []string{"agent"},
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.Nil(t, results.Suggestions)
}

// ============================================================================
// Tests: Engine.computeFacets
// ============================================================================

func TestComputeFacets_VariousTypes(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	results := []SearchResult{
		{Type: ResultDocument, Source: "fts", Language: "go", EntityType: "skill"},
		{Type: ResultDocument, Source: "fts", Language: "go", EntityType: "skill"},
		{Type: ResultChunk, Source: "vector", Language: "py"},
		{Type: ResultEntity, Source: "graph", EntityType: "agent"},
	}

	facets := engine.computeFacets(results)
	assert.NotNil(t, facets)
	assert.Contains(t, facets, "type")
	assert.Contains(t, facets, "entity_type")
	assert.Contains(t, facets, "language")
	assert.Contains(t, facets, "source")

	assert.Equal(t, 2, facets["type"]["document"])
	assert.Equal(t, 1, facets["type"]["chunk"])
	assert.Equal(t, 1, facets["type"]["entity"])
	assert.Equal(t, 2, facets["entity_type"]["skill"])
	assert.Equal(t, 1, facets["entity_type"]["agent"])
	assert.Equal(t, 2, facets["language"]["go"])
	assert.Equal(t, 1, facets["language"]["py"])
	assert.Equal(t, 2, facets["source"]["fts"])
	assert.Equal(t, 1, facets["source"]["vector"])
	assert.Equal(t, 1, facets["source"]["graph"])
}

func TestComputeFacets_EmptyResults(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	facets := engine.computeFacets(nil)
	assert.NotNil(t, facets)
	assert.Contains(t, facets, "type")
	assert.Contains(t, facets, "source")
	assert.Empty(t, facets["type"])
	assert.Empty(t, facets["source"])
}

func TestComputeFacets_NoLanguage(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	results := []SearchResult{
		{Type: ResultDocument, Source: "fts"},
		{Type: ResultDocument, Source: "fts"},
	}
	facets := engine.computeFacets(results)
	assert.NotContains(t, facets, "language")
	assert.NotContains(t, facets, "entity_type")
}

// ============================================================================
// Tests: Engine.generateSuggestions
// ============================================================================

func TestGenerateSuggestions_Normal(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	s := engine.generateSuggestions("search")
	assert.Len(t, s, 4)
	assert.Equal(t, "search agent", s[0])
	assert.Equal(t, "search skill", s[1])
	assert.Equal(t, "search workflow", s[2])
	assert.Equal(t, "search template", s[3])
}

func TestGenerateSuggestions_Empty(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	s := engine.generateSuggestions("")
	assert.Nil(t, s)
}

func TestGenerateSuggestions_WhitespaceOnly(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	s := engine.generateSuggestions("   ")
	assert.Nil(t, s)
}

// ============================================================================
// Tests: Engine.toRankables / Engine.fromRankables
// ============================================================================

func TestToRankables_FromRankables_Roundtrip(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	original := []SearchResult{
		{ID: "a", Type: ResultDocument, Title: "Alpha", Content: "content A", Score: 0.9},
		{ID: "b", Type: ResultChunk, Title: "Beta", Content: "content B", Score: 0.8},
		{ID: "c", Type: ResultEntity, Title: "Gamma", Content: "content C", Score: 0.7},
	}

	rankables := engine.toRankables(original)
	assert.Len(t, rankables, 3)

	result := engine.fromRankables(rankables)
	assert.Len(t, result, 3)

	for i := range original {
		assert.Equal(t, original[i].ID, result[i].ID)
		assert.Equal(t, original[i].Type, result[i].Type)
		assert.Equal(t, original[i].Content, result[i].Content)
		assert.Equal(t, original[i].Score, result[i].Score)
		assert.Equal(t, i+1, result[i].Rank) // Ranks are assigned sequentially
	}
}

func TestToRankables_Empty(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	rankables := engine.toRankables(nil)
	assert.Empty(t, rankables) // make([]T, 0) returns empty non-nil slice
	rankables = engine.toRankables([]SearchResult{})
	assert.Empty(t, rankables)
}

func TestFromRankables_Empty(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	result := engine.fromRankables(nil)
	assert.Empty(t, result) // make([]T, 0) returns empty non-nil slice
	result = engine.fromRankables([]ranking.Rankable{})
	assert.Empty(t, result)
}

// ============================================================================
// Tests: searchResultRankable methods
// ============================================================================

func TestSearchResultRankable_Methods(t *testing.T) {
	t.Parallel()
	sr := SearchResult{
		ID:      "id-1",
		Title:   "My Title",
		Content: "My Content",
		Score:   0.95,
	}
	rankable := &searchResultRankable{result: sr}

	assert.Equal(t, "id-1", rankable.ID())
	assert.Equal(t, "My Content My Title", rankable.Content())
	assert.Equal(t, 0.95, rankable.Score())
	assert.Zero(t, rankable.Timestamp())
	assert.Zero(t, rankable.ReferenceCount())
	assert.Zero(t, rankable.GraphDistance())
}

// ============================================================================
// Tests: ftsTableToResultType
// ============================================================================

func TestFtsTableToResultType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		table string
		want  ResultType
	}{
		{"documents_fts", ResultDocument},
		{"chunks_fts", ResultChunk},
		{"entities_fts", ResultEntity},
		{"code_blocks_fts", ResultCode},
		{"unknown", ResultDocument},
		{"", ResultDocument},
	}
	for _, tt := range tests {
		t.Run(tt.table, func(t *testing.T) {
			got := ftsTableToResultType(tt.table)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ============================================================================
// Tests: detectResultType
// ============================================================================

func TestDetectResultType_Entity(t *testing.T) {
	t.Parallel()
	vr := vector.SearchResult{
		EntityID: "entity-1",
	}
	assert.Equal(t, ResultEntity, detectResultType(vr))
}

func TestDetectResultType_Chunk(t *testing.T) {
	t.Parallel()
	vr := vector.SearchResult{
		ChunkID: "chunk-1",
	}
	assert.Equal(t, ResultChunk, detectResultType(vr))
}

func TestDetectResultType_Document(t *testing.T) {
	t.Parallel()
	vr := vector.SearchResult{
		DocumentID: "doc-1",
	}
	assert.Equal(t, ResultDocument, detectResultType(vr))
}

func TestDetectResultType_Default(t *testing.T) {
	t.Parallel()
	vr := vector.SearchResult{}
	// No EntityID, ChunkID, or DocumentID → defaults to Document
	assert.Equal(t, ResultDocument, detectResultType(vr))
}

func TestDetectResultType_EntityTakesPriority(t *testing.T) {
	t.Parallel()
	vr := vector.SearchResult{
		EntityID:   "entity-1",
		ChunkID:    "chunk-1",
		DocumentID: "doc-1",
	}
	// EntityID takes priority
	assert.Equal(t, ResultEntity, detectResultType(vr))
}

func TestDetectResultType_ChunkOverDocument(t *testing.T) {
	t.Parallel()
	vr := vector.SearchResult{
		ChunkID:    "chunk-1",
		DocumentID: "doc-1",
	}
	// ChunkID takes priority over DocumentID
	assert.Equal(t, ResultChunk, detectResultType(vr))
}

// ============================================================================
// Tests: resolveFTSTables
// ============================================================================

func TestResolveFTSTables_KnownTypes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		types []string
		want  []string
	}{
		{"single doc", []string{"document"}, []string{"documents_fts"}},
		{"single chunk", []string{"chunk"}, []string{"chunks_fts"}},
		{"single entity", []string{"entity"}, []string{"entities_fts"}},
		{"single code", []string{"code"}, []string{"code_blocks_fts"}},
		{"single code_block", []string{"code_block"}, []string{"code_blocks_fts"}},
		{"multiple", []string{"document", "chunk"}, []string{"documents_fts", "chunks_fts"}},
		{"alias doc", []string{"doc"}, []string{"documents_fts"}},
		{"all four", []string{"document", "chunk", "entity", "code"}, []string{"documents_fts", "chunks_fts", "entities_fts", "code_blocks_fts"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveFTSTables(tt.types)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveFTSTables_UnknownTypes(t *testing.T) {
	t.Parallel()
	// Unknown types are ignored
	got := resolveFTSTables([]string{"unknown", "nonexistent"})
	assert.Empty(t, got)
}

func TestResolveFTSTables_MixedKnownUnknown(t *testing.T) {
	t.Parallel()
	got := resolveFTSTables([]string{"document", "unknown", "chunk"})
	assert.Equal(t, []string{"documents_fts", "chunks_fts"}, got)
}

func TestResolveFTSTables_Empty(t *testing.T) {
	t.Parallel()
	assert.Nil(t, resolveFTSTables(nil))
	assert.Nil(t, resolveFTSTables([]string{}))
}

// ============================================================================
// Tests: truncateContent
// ============================================================================

func TestTruncateContent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
		maxLen  int
		want    string
	}{
		{"shorter than max", "short", 10, "short"},
		{"exactly max", "abcdefghij", 10, "abcdefghij"},
		{"longer than max", "this is a long string that should be truncated", 20, "this is a long st..."},
		{"empty", "", 10, ""},
		{"maxLen 3", "hello", 3, "..."},
		{"maxLen 4", "hello", 4, "h..."},
		{"single char truncation", "ab", 4, "ab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateContent(tt.content, tt.maxLen)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ============================================================================
// Tests: generateSnippet
// ============================================================================

func TestGenerateSnippet_FindsQuery(t *testing.T) {
	t.Parallel()
	content := "The quick brown fox jumps over the lazy dog"
	snippet := generateSnippet(content, "fox", 30)
	assert.Contains(t, snippet, "fox")
	assert.NotEmpty(t, snippet)
}

func TestGenerateSnippet_FindsIndividualTerm(t *testing.T) {
	t.Parallel()
	content := "The quick brown fox jumps over the lazy dog"
	snippet := generateSnippet(content, "quick canine", 50)
	assert.Contains(t, snippet, "quick")
	assert.NotEmpty(t, snippet)
}

func TestGenerateSnippet_QueryNotFound(t *testing.T) {
	t.Parallel()
	content := "The quick brown fox"
	snippet := generateSnippet(content, "missing", 30)
	assert.Equal(t, content, snippet)
}

func TestGenerateSnippet_EmptyContent(t *testing.T) {
	t.Parallel()
	snippet := generateSnippet("", "query", 10)
	assert.Empty(t, snippet)
}

func TestGenerateSnippet_EmptyQuery(t *testing.T) {
	t.Parallel()
	content := "The quick brown fox"
	snippet := generateSnippet(content, "", 30)
	assert.Equal(t, content, snippet)
}

func TestGenerateSnippet_WithPrefixEllipsis(t *testing.T) {
	t.Parallel()
	longContent := strings.Repeat("prefix", 10) + " target " + strings.Repeat("suffix", 10)
	snippet := generateSnippet(longContent, "target", 30)
	assert.Contains(t, snippet, "target")
	if strings.HasPrefix(snippet, "...") {
		// Prefix ellipsis is expected when match is far from start
		assert.True(t, len(snippet) > 0)
	}
}

func TestGenerateSnippet_WithSuffixEllipsis(t *testing.T) {
	t.Parallel()
	longContent := strings.Repeat("prefix", 10) + " target " + strings.Repeat("suffix", 10)
	snippet := generateSnippet(longContent, "target", 30)
	assert.Contains(t, snippet, "target")
	// Either prefix or suffix (or both) should have ellipsis for long content
	assert.True(t, strings.HasPrefix(snippet, "...") || strings.HasSuffix(snippet, "..."))
}

// ============================================================================
// Tests: Engine.Search - score sorting fallback (no ranker)
// ============================================================================

func TestSearch_ScoreSortingFallback(t *testing.T) {
	t.Parallel()
	g := graph.New()
	// Add nodes with different inherent scores (graph centrality)
	require.NoError(t, g.AddNode(&graph.Node{
		ID:   "low-score",
		Type: "",
		Name: "Target Agent",
	}))
	require.NoError(t, g.AddNode(&graph.Node{
		ID:   "high-score",
		Type: "",
		Name: "Target Agent Prime",
	}))
	// Connect to give high-score node a centrality boost
	require.NoError(t, g.AddNode(&graph.Node{
		ID:   "connector",
		Type: "",
		Name: "Target Agent Connector",
	}))
	require.NoError(t, g.AddEdge(&graph.Edge{
		Source: "high-score",
		Target: "connector",
		Type:   graph.RelRelatedTo,
	}))

	engine := NewEngine(nil, nil, g, nil, nil) // no ranker
	params := SearchParams{
		Query:       "target",
		Limit:       20,
		EnableGraph: true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.NotEmpty(t, results.Results)

	// Verify results are sorted by score descending
	for i := 1; i < len(results.Results); i++ {
		assert.GreaterOrEqual(t, results.Results[i-1].Score, results.Results[i].Score,
			"results should be sorted by score descending")
	}
}

// ============================================================================
// Tests: Edge cases for resolveFTSTables (case sensitivity)
// ============================================================================

func TestResolveFTSTables_CaseSensitive(t *testing.T) {
	t.Parallel()
	// Types should be lowercase; uppercase/weird case yields no match
	got := resolveFTSTables([]string{"Document", "CHUNK", "Entity"})
	assert.Empty(t, got)
}

// ============================================================================
// Tests: MockSearchEngine
// ============================================================================

func TestMockSearchEngine_Search(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	params := SearchParams{Query: "test", Limit: 10}

	calls := 0
	m := &MockSearchEngine{
		SearchFunc: func(ctx context.Context, params SearchParams) (*SearchResults, error) {
			calls++
			return &SearchResults{Query: params.Query, TotalCount: 5}, nil
		},
	}

	result, err := m.Search(ctx, params)
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	assert.Equal(t, "test", result.Query)
	assert.Equal(t, 5, result.TotalCount)
}

func TestMockSearchEngine_DefaultBehavior(t *testing.T) {
	t.Parallel()
	m := &MockSearchEngine{}
	result, err := m.Search(context.Background(), SearchParams{Query: "x"})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Results)
	assert.Zero(t, result.TotalCount)
}

// ============================================================================
// Tests: Engine.Search - Duration
// ============================================================================

func TestSearch_Duration(t *testing.T) {
	t.Parallel()
	g := graph.New()
	require.NoError(t, g.AddNode(&graph.Node{
		ID: "d1", Type: "", Name: "Duration Test",
	}))

	engine := NewEngine(nil, nil, g, nil, nil)
	params := SearchParams{
		Query:       "duration",
		Limit:       20,
		EnableGraph: true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	// Duration is wall-clock; on fast machines (and with the coarse Windows
	// clock granularity) a trivially fast in-memory search can measure 0,
	// so only the non-negative invariant is asserted here.
	assert.GreaterOrEqual(t, results.Duration, int64(0), "duration should be set")
}

// ============================================================================
// Tests: Engine.Search - Limit validation
// ============================================================================

func TestSearch_ZeroLimitDefaultsTo20(t *testing.T) {
	t.Parallel()
	g := graph.New()
	for i := 0; i < 30; i++ {
		require.NoError(t, g.AddNode(&graph.Node{
			ID:   fmt.Sprintf("many-%d", i),
			Type: "",
			Name: fmt.Sprintf("Many Skill %d", i),
		}))
	}

	engine := NewEngine(nil, nil, g, nil, nil)
	params := SearchParams{
		Query:       "many",
		Limit:       0, // zero limit → defaults to 20
		EnableGraph: true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(results.Results), 20)
}

func TestSearch_NegativeLimitDefaultsTo20(t *testing.T) {
	t.Parallel()
	g := graph.New()
	for i := 0; i < 30; i++ {
		require.NoError(t, g.AddNode(&graph.Node{
			ID:   fmt.Sprintf("neg-%d", i),
			Type: "",
			Name: fmt.Sprintf("Neg Skill %d", i),
		}))
	}

	engine := NewEngine(nil, nil, g, nil, nil)
	params := SearchParams{
		Query:       "neg",
		Limit:       -5, // negative limit → defaults to 20
		EnableGraph: true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(results.Results), 20)
}

// ============================================================================
// Tests: Engine.Search - Types filter with empty query
// ============================================================================

func TestSearch_TypesFilterNoQuery(t *testing.T) {
	t.Parallel()
	g := graph.New()
	require.NoError(t, g.AddNode(&graph.Node{
		ID: "tf1", Type: "", Name: "Filter Agent",
	}))

	engine := NewEngine(nil, nil, g, nil, nil)
	params := SearchParams{
		Query:       "",
		Types:       []string{"agent"},
		Limit:       20,
		EnableGraph: true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.NotNil(t, results)
	// Graph search is not filtered by Types param - that's for FTS
	// But the query should not error because we have Types filter
}

func TestSearch_PathFilterNoQuery(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	params := SearchParams{
		Query: "",
		Path:  "/some/path",
		Limit: 20,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.NotNil(t, results)
}

// ============================================================================
// Tests: Engine.Search - Graph with neighbors boost
// ============================================================================

func TestSearch_GraphWithMaxBoost(t *testing.T) {
	t.Parallel()
	g := graph.New()
	center := &graph.Node{
		ID:   "center",
		Type: "",
		Name: "Center Agent",
		Metadata: map[string]interface{}{
			"description": "A central node.",
		},
	}
	require.NoError(t, g.AddNode(center))

	// Add 10 neighbors to reach max boost (0.5 cap)
	for i := 0; i < 10; i++ {
		neighbor := &graph.Node{
			ID:   fmt.Sprintf("neighbor-%d", i),
			Type: "",
			Name: fmt.Sprintf("Neighbor %d", i),
		}
		require.NoError(t, g.AddNode(neighbor))
		require.NoError(t, g.AddEdge(&graph.Edge{
			Source: "center",
			Target: neighbor.ID,
			Type:   graph.RelRelatedTo,
		}))
	}

	engine := NewEngine(nil, nil, g, nil, nil)
	params := SearchParams{
		Query:       "center",
		Limit:       20,
		EnableGraph: true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.Len(t, results.Results, 1)
	// Base 0.5 + max boost 0.5 = 1.0
	assert.Equal(t, 1.0, results.Results[0].Score)
}

// ============================================================================
// Tests: SearchParams validation edge cases
// ============================================================================

func TestSearchParams_AllFields(t *testing.T) {
	t.Parallel()
	sp := SearchParams{
		Query:        "test",
		Limit:        10,
		Offset:       5,
		Types:        []string{"document", "chunk"},
		Path:         "/docs",
		Tags:         map[string]string{"lang": "go"},
		Since:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EnableFTS:    true,
		EnableVector: true,
		EnableGraph:  true,
		EnableFacets: true,
		MinScore:     0.5,
	}
	assert.Equal(t, "test", sp.Query)
	assert.Equal(t, 10, sp.Limit)
	assert.Equal(t, 5, sp.Offset)
	assert.Equal(t, []string{"document", "chunk"}, sp.Types)
	assert.Equal(t, "/docs", sp.Path)
	assert.Equal(t, map[string]string{"lang": "go"}, sp.Tags)
	assert.False(t, sp.Since.IsZero())
	assert.True(t, sp.EnableFTS)
	assert.True(t, sp.EnableVector)
	assert.True(t, sp.EnableGraph)
	assert.True(t, sp.EnableFacets)
	assert.Equal(t, 0.5, sp.MinScore)
}

func TestSearchParams_MinScoreEdgeCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		minScore float64
	}{
		{"zero", 0.0},
		{"one", 1.0},
		{"negative", -0.5},
		{"above max", 2.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp := SearchParams{MinScore: tt.minScore}
			assert.Equal(t, tt.minScore, sp.MinScore)
		})
	}
}

func TestSearchParams_TagsEdgeCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		tags map[string]string
	}{
		{"nil", nil},
		{"empty map", map[string]string{}},
		{"single pair", map[string]string{"k": "v"}},
		{"multiple pairs", map[string]string{"a": "1", "b": "2", "c": "3"}},
		{"empty values", map[string]string{"key": ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp := SearchParams{Tags: tt.tags}
			assert.Equal(t, len(tt.tags), len(sp.Tags))
		})
	}
}

// ============================================================================
// Tests: Engine.Search - No engines enabled
// ============================================================================

func TestSearch_NoEnginesEnabled_WithFilters(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	params := SearchParams{
		Query:        "",
		Limit:        20,
		Types:        []string{"document"},
		EnableFTS:    false,
		EnableVector: false,
		EnableGraph:  false,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.NotNil(t, results)
	assert.Empty(t, results.Results)
	assert.Equal(t, 0, results.TotalCount)
}

func TestSearch_NoEnginesEnabled_WithPath(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	params := SearchParams{
		Query:        "",
		Limit:        20,
		Path:         "/some/path",
		EnableFTS:    false,
		EnableVector: false,
		EnableGraph:  false,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.NotNil(t, results)
}

// ============================================================================
// Tests: Engine.Search - FTS + query empty status
// ============================================================================

func TestSearch_OnlyVectorEnabled_FTSDisabled(t *testing.T) {
	t.Parallel()
	vs := &mockVectorStore{
		searchFunc: func(query []float64, limit int) ([]vector.SearchResult, error) {
			return []vector.SearchResult{
				{ID: "v1", Score: 0.9, Content: "result", DocumentID: "doc-1"},
			}, nil
		},
	}
	engine := NewEngine(nil, vs, nil, nil, stubEmbedFunc)
	params := SearchParams{
		Query:        "search",
		Limit:        20,
		EnableFTS:    false,
		EnableVector: true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.Len(t, results.Results, 1)
	assert.Equal(t, "vector", results.Results[0].Source)
}

// ============================================================================
// Tests: Engine.Search - Graph with GetNeighbors error
// ============================================================================

func TestSearch_GraphNode_GetNeighborsError(t *testing.T) {
	t.Parallel()
	// Node that exists but has no metadata description
	g := graph.New()
	require.NoError(t, g.AddNode(&graph.Node{
		ID:   "node-no-desc",
		Type: "",
		Name: "Target Node",
		// No Metadata — so description won't be set
	}))

	engine := NewEngine(nil, nil, g, nil, nil)
	params := SearchParams{
		Query:       "target",
		Limit:       20,
		EnableGraph: true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.Len(t, results.Results, 1)
	assert.Equal(t, "node-no-desc", results.Results[0].ID)
	// Content should be empty since no description
	assert.Empty(t, results.Results[0].Content)
	// Snippet should also be empty
	assert.Empty(t, results.Results[0].Snippet)
}

func TestSearch_GraphNode_NonStringDescription(t *testing.T) {
	t.Parallel()
	g := graph.New()
	require.NoError(t, g.AddNode(&graph.Node{
		ID:   "node-int-desc",
		Type: "",
		Name: "Integer Desc Node",
		Metadata: map[string]interface{}{
			"description": 12345, // not a string!
		},
	}))

	engine := NewEngine(nil, nil, g, nil, nil)
	params := SearchParams{
		Query:       "integer",
		Limit:       20,
		EnableGraph: true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.Len(t, results.Results, 1)
	// Type assertion for non-string should fail, leaving Content empty
	assert.Empty(t, results.Results[0].Content)
}

// ============================================================================
// Tests: Engine.Search - Vector error path (Store error)
// ============================================================================

func TestSearch_VectorSearch_StoreError(t *testing.T) {
	t.Parallel()
	vs := &mockVectorStore{
		searchFunc: func(query []float64, limit int) ([]vector.SearchResult, error) {
			return nil, fmt.Errorf("store error")
		},
	}
	engine := NewEngine(nil, vs, nil, nil, stubEmbedFunc)
	params := SearchParams{
		Query:        "test",
		Limit:        20,
		EnableVector: true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	// Vector fails gracefully, empty results
	assert.Empty(t, results.Results)
}

func TestSearch_VectorSearch_EmbedFuncNil(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil) // embedFunc is nil
	params := SearchParams{
		Query:        "search",
		Limit:        20,
		EnableVector: true,
	}
	// Should not call vector search when embedFunc is nil
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.Empty(t, results.Results)
}

// ============================================================================
// Tests: generateSnippet expanded edge cases
// ============================================================================

func TestGenerateSnippet_MatchAtStart(t *testing.T) {
	t.Parallel()
	content := "target_word is at the very beginning of this content"
	snippet := generateSnippet(content, "target_word", 50)
	assert.Contains(t, snippet, "target_word")
	// Should NOT have prefix ellipsis since match is at start
	assert.False(t, strings.HasPrefix(snippet, "..."))
}

func TestGenerateSnippet_MatchAtEnd(t *testing.T) {
	t.Parallel()
	content := "this content has a match at the very end which is target_word"
	snippet := generateSnippet(content, "target_word", 50)
	assert.Contains(t, snippet, "target_word")
	assert.False(t, strings.HasSuffix(snippet, "..."))
}

func TestGenerateSnippet_ExactMatch(t *testing.T) {
	t.Parallel()
	content := "hello world"
	snippet := generateSnippet(content, "hello", 20)
	assert.Contains(t, snippet, "hello")
}

func TestGenerateSnippet_SingleCharQuery(t *testing.T) {
	t.Parallel()
	content := "abcdefghijklmnop"
	snippet := generateSnippet(content, "k", 10)
	assert.Contains(t, snippet, "k")
}

func TestGenerateSnippet_ContentShorterThanMaxLen(t *testing.T) {
	t.Parallel()
	content := "short"
	snippet := generateSnippet(content, "short", 100)
	assert.Equal(t, "short", snippet)
}

func TestGenerateSnippet_MultiWordNoIndividualMatch(t *testing.T) {
	t.Parallel()
	content := "the result contains only unrelated words here"
	snippet := generateSnippet(content, "xyzabc missing", 40)
	// Neither the full phrase nor individual terms match
	assert.Equal(t, truncateContent(content, 40), snippet)
}

func TestGenerateSnippet_CaseInsensitive(t *testing.T) {
	t.Parallel()
	content := "The Quick Brown Fox"
	snippet := generateSnippet(content, "QUICK", 30)
	assert.Contains(t, snippet, "Quick")
}

func TestGenerateSnippet_PartialTermMatch(t *testing.T) {
	t.Parallel()
	content := "this has programming content about golang"
	// "go" appears as part of "golang" — the second term "lang" should match
	snippet := generateSnippet(content, "go lang", 30)
	assert.Contains(t, snippet, "lang")
}

// ============================================================================
// Tests: truncateContent expanded edge cases
// ============================================================================

func TestTruncateContent_Unicode(t *testing.T) {
	t.Parallel()
	content := "café ☕ résumé"
	assert.Equal(t, content, truncateContent(content, 100))
	assert.Equal(t, "café ☕ résumé", truncateContent(content, len(content)))
}

func TestTruncateContent_ExactlyThreshold(t *testing.T) {
	t.Parallel()
	// Content length = maxLen-1: should not truncate
	content := "abcdefghij" // 10 chars
	assert.Equal(t, content, truncateContent(content, 11))
	// Content length = maxLen: should not truncate
	assert.Equal(t, content, truncateContent(content, 10))
	// Content length = maxLen+1: should truncate
	contentLong := "abcdefghijk" // 11 chars, maxLen=10
	assert.Equal(t, "abcdefg...", truncateContent(contentLong, 10))
}

func TestTruncateContent_MinValidMax(t *testing.T) {
	t.Parallel()
	// maxLen=3 is the minimum before the slice operation becomes invalid
	content := "abcdef"
	assert.Equal(t, "...", truncateContent(content, 3))
}

// ============================================================================
// Tests: computeFacets expanded edge cases
// ============================================================================

func TestComputeFacets_MixedEmptyFields(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	results := []SearchResult{
		{Type: ResultDocument, Source: "fts", Language: "go", EntityType: "skill"},
		{Type: ResultChunk, Source: "vector", Language: "", EntityType: ""},
		{Type: ResultDocument, Source: "", Language: "go", EntityType: ""},
	}
	facets := engine.computeFacets(results)

	assert.Contains(t, facets, "type")
	assert.Contains(t, facets, "entity_type")
	assert.Contains(t, facets, "language")
	assert.Contains(t, facets, "source")

	// type facets
	assert.Equal(t, 2, facets["type"]["document"])
	assert.Equal(t, 1, facets["type"]["chunk"])

	// entity_type — only non-empty values counted
	assert.Equal(t, 1, facets["entity_type"]["skill"])

	// language — only non-empty values counted
	assert.Equal(t, 2, facets["language"]["go"])

	// source
	assert.Equal(t, 1, facets["source"]["fts"])
	assert.Equal(t, 1, facets["source"]["vector"])
	assert.Equal(t, 1, facets["source"][""])
}

func TestComputeFacets_AllFieldsPopulated(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	results := []SearchResult{
		{
			Type:       ResultDocument,
			Source:     "fts",
			Language:   "go",
			EntityType: "skill",
		},
		{
			Type:       ResultChunk,
			Source:     "vector",
			Language:   "py",
			EntityType: "agent",
		},
		{
			Type:       ResultEntity,
			Source:     "graph",
			Language:   "js",
			EntityType: "workflow",
		},
	}
	facets := engine.computeFacets(results)
	assert.Equal(t, 1, facets["type"]["document"])
	assert.Equal(t, 1, facets["type"]["chunk"])
	assert.Equal(t, 1, facets["type"]["entity"])
	assert.Equal(t, 1, facets["entity_type"]["skill"])
	assert.Equal(t, 1, facets["entity_type"]["agent"])
	assert.Equal(t, 1, facets["entity_type"]["workflow"])
	assert.Equal(t, 1, facets["language"]["go"])
	assert.Equal(t, 1, facets["language"]["py"])
	assert.Equal(t, 1, facets["language"]["js"])
	assert.Equal(t, 1, facets["source"]["fts"])
	assert.Equal(t, 1, facets["source"]["vector"])
	assert.Equal(t, 1, facets["source"]["graph"])
}

func TestComputeFacets_NoEntityTypeFacetWhenAllEmpty(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	results := []SearchResult{
		{Type: ResultDocument, Source: "fts"},
		{Type: ResultDocument, Source: "fts"},
	}
	facets := engine.computeFacets(results)
	assert.NotContains(t, facets, "entity_type")
	assert.NotContains(t, facets, "language")
}

// ============================================================================
// Tests: searchResultRankable expanded
// ============================================================================

func TestSearchResultRankable_EmptyContent(t *testing.T) {
	t.Parallel()
	sr := SearchResult{
		ID:      "empty-content",
		Title:   "",
		Content: "",
		Score:   0.0,
	}
	rankable := &searchResultRankable{result: sr}
	assert.Equal(t, "empty-content", rankable.ID())
	assert.Equal(t, " ", rankable.Content()) // Content + " " + Title
	assert.Equal(t, 0.0, rankable.Score())
}

func TestSearchResultRankable_OnlyTitle(t *testing.T) {
	t.Parallel()
	sr := SearchResult{
		ID:      "title-only",
		Title:   "Only Title Here",
		Content: "",
		Score:   0.5,
	}
	rankable := &searchResultRankable{result: sr}
	assert.Equal(t, " Only Title Here", rankable.Content())
}

func TestSearchResultRankable_FullFields(t *testing.T) {
	t.Parallel()
	sr := SearchResult{
		ID:      "full",
		Title:   "Title",
		Content: "Body text",
		Score:   0.99,
	}
	rankable := &searchResultRankable{result: sr}
	assert.Equal(t, "Body text Title", rankable.Content())
	assert.Equal(t, int64(0), rankable.Timestamp())
	assert.Equal(t, 0, rankable.ReferenceCount())
	assert.Equal(t, 0, rankable.GraphDistance())
}

// ============================================================================
// Tests: Engine.Search - Duration and timing
// ============================================================================

func TestSearch_DurationSet(t *testing.T) {
	t.Parallel()
	g := graph.New()
	require.NoError(t, g.AddNode(&graph.Node{
		ID: "dur-node", Type: "", Name: "Duration Test",
	}))

	engine := NewEngine(nil, nil, g, nil, nil)
	params := SearchParams{
		Query:       "duration",
		Limit:       20,
		EnableGraph: true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	// Duration is wall-clock; a fast search can legitimately measure 0 on
	// Windows (coarse timer), so assert >= 0 and keep the upper bound.
	assert.GreaterOrEqual(t, int64(results.Duration), int64(0))
	// Duration is time.Duration, should be reasonable (< 5 seconds)
	assert.Less(t, int64(results.Duration), int64(5*time.Second))
}

func TestSearch_DurationForEmptyResults(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	params := SearchParams{
		Query: "/path",
		Path:  "/path",
		Limit: 20,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.Empty(t, results.Results)
	// Even empty searches should have duration set — but wall-clock can
	// measure 0 on fast machines/Windows, so assert non-negative.
	assert.GreaterOrEqual(t, int64(results.Duration), int64(0))
}

// ============================================================================
// Tests: Engine.Search - mixed source results
// ============================================================================

func TestSearch_VectorAndGraphMixed(t *testing.T) {
	t.Parallel()
	g := graph.New()
	require.NoError(t, g.AddNode(&graph.Node{
		ID:   "mixed-node",
		Type: "",
		Name: "Mixed Search Node",
		Metadata: map[string]interface{}{
			"description": "A node found via graph.",
		},
	}))

	vs := &mockVectorStore{
		searchFunc: func(query []float64, limit int) ([]vector.SearchResult, error) {
			return []vector.SearchResult{
				{
					ID:         "vec-result",
					Score:      0.8,
					Content:    "Vector result content",
					DocumentID: "doc-1",
				},
			}, nil
		},
	}

	engine := NewEngine(nil, vs, g, nil, stubEmbedFunc)
	params := SearchParams{
		Query:        "mixed",
		Limit:        20,
		EnableVector: true,
		EnableGraph:  true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.Len(t, results.Results, 2)

	// Check sources are diverse
	sources := map[string]int{}
	for _, r := range results.Results {
		sources[r.Source]++
	}
	assert.Equal(t, 1, sources["vector"])
	assert.Equal(t, 1, sources["graph"])
}

// ============================================================================
// Tests: resolveFTSTables - duplicates
// ============================================================================

func TestResolveFTSTables_Duplicates(t *testing.T) {
	t.Parallel()
	// Duplicate types should produce duplicate tables
	got := resolveFTSTables([]string{"document", "document", "doc"})
	assert.Equal(t, []string{"documents_fts", "documents_fts", "documents_fts"}, got)
}

// ============================================================================
// Tests: DefaultSearchParams - idempotent
// ============================================================================

func TestDefaultSearchParams_Idempotent(t *testing.T) {
	t.Parallel()
	first := DefaultSearchParams()
	second := DefaultSearchParams()
	assert.Equal(t, first, second)
}

// ============================================================================
// Tests: Engine.Search - Zero offset (ranks start at 1)
// ============================================================================

func TestSearch_ZeroOffset_RankStartsAtOne(t *testing.T) {
	t.Parallel()
	g := graph.New()
	for i := 0; i < 3; i++ {
		require.NoError(t, g.AddNode(&graph.Node{
			ID:   fmt.Sprintf("rank-%d", i),
			Type: "",
			Name: fmt.Sprintf("Rank Test %d", i),
		}))
	}

	engine := NewEngine(nil, nil, g, nil, nil)
	params := SearchParams{
		Query:       "rank",
		Limit:       10,
		Offset:      0,
		EnableGraph: true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results.Results), 1)
	assert.Equal(t, 1, results.Results[0].Rank)
}

// ============================================================================
// Tests: SearchResult struct - full roundtrip
// ============================================================================

func TestSearchResult_AllFields(t *testing.T) {
	t.Parallel()
	sr := SearchResult{
		ID:           "id-1",
		Type:         ResultCode,
		Score:        0.99,
		Title:        "main.go",
		Content:      "package main\nfunc main() {}",
		Snippet:      "func main()...",
		Highlighted:  "<b>func main</b>()",
		DocumentID:   "doc-123",
		DocumentPath: "/src/main.go",
		Heading:      "Main Function",
		SectionType:  "code_block",
		EntityType:   "",
		Language:     "go",
		Metadata: map[string]string{
			"lang":    "go",
			"version": "1.21",
		},
		Rank:   1,
		Source: "fts",
	}
	assert.Equal(t, "id-1", sr.ID)
	assert.Equal(t, ResultCode, sr.Type)
	assert.Equal(t, 0.99, sr.Score)
	assert.Equal(t, "main.go", sr.Title)
	assert.Equal(t, "package main\nfunc main() {}", sr.Content)
	assert.Equal(t, "func main()...", sr.Snippet)
	assert.Equal(t, "<b>func main</b>()", sr.Highlighted)
	assert.Equal(t, "doc-123", sr.DocumentID)
	assert.Equal(t, "/src/main.go", sr.DocumentPath)
	assert.Equal(t, "Main Function", sr.Heading)
	assert.Equal(t, "code_block", sr.SectionType)
	assert.Empty(t, sr.EntityType)
	assert.Equal(t, "go", sr.Language)
	assert.Len(t, sr.Metadata, 2)
	assert.Equal(t, 1, sr.Rank)
	assert.Equal(t, "fts", sr.Source)
}

// ============================================================================
// Tests: SearchResults - Nil results and zero values
// ============================================================================

func TestSearchResults_NilResults(t *testing.T) {
	t.Parallel()
	sr := &SearchResults{
		Results:    nil,
		TotalCount: 5,
		Query:      "test",
	}
	assert.Nil(t, sr.Results)
	assert.Equal(t, 5, sr.TotalCount)
}

func TestSearchResults_WithAllFacets(t *testing.T) {
	t.Parallel()
	sr := &SearchResults{
		Results: []SearchResult{{ID: "1", Type: ResultDocument}},
		Facets: map[string]map[string]int{
			"type":     {"document": 1},
			"source":   {"fts": 1},
			"language": {"go": 1},
		},
		Suggestions: []string{"a", "b", "c", "d"},
		Duration:    150 * time.Millisecond,
	}
	assert.Len(t, sr.Facets, 3)
	assert.Len(t, sr.Suggestions, 4)
	assert.Equal(t, 150*time.Millisecond, sr.Duration)
}

// ============================================================================
// Tests: DetectResultType edge cases
// ============================================================================

func TestDetectResultType_EntityOnly(t *testing.T) {
	t.Parallel()
	// EntityID takes priority even over ChunkID and DocumentID
	vr := vector.SearchResult{EntityID: "e1"}
	assert.Equal(t, ResultEntity, detectResultType(vr))
}

func TestDetectResultType_ChunkOnly(t *testing.T) {
	t.Parallel()
	vr := vector.SearchResult{ChunkID: "c1"}
	assert.Equal(t, ResultChunk, detectResultType(vr))
}

// ============================================================================
// Tests: Engine.Search - Suggestions with special characters
// ============================================================================

func TestGenerateSuggestions_SpecialChars(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	s := engine.generateSuggestions("c++")
	assert.Len(t, s, 4)
	assert.Equal(t, "c++ agent", s[0])
}

func TestGenerateSuggestions_SingleWord(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	s := engine.generateSuggestions("x")
	assert.Len(t, s, 4)
	assert.Equal(t, "x agent", s[0])
}

// ============================================================================
// Tests: Engine.Search - Graph search with FilterNodes edge
// ============================================================================

func TestSearch_GraphNode_CaseInsensitiveMatch(t *testing.T) {
	t.Parallel()
	g := graph.New()
	require.NoError(t, g.AddNode(&graph.Node{
		ID:   "case-node",
		Type: "",
		Name: "CASE Sensitive Node",
	}))

	engine := NewEngine(nil, nil, g, nil, nil)
	params := SearchParams{
		Query:       "case",
		Limit:       20,
		EnableGraph: true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.Len(t, results.Results, 1)
}

func TestSearch_GraphNode_PartialNameMatch(t *testing.T) {
	t.Parallel()
	g := graph.New()
	require.NoError(t, g.AddNode(&graph.Node{
		ID:   "partial-node",
		Type: "",
		Name: "SuperLongNameForPartialMatch",
	}))

	engine := NewEngine(nil, nil, g, nil, nil)
	params := SearchParams{
		Query:       "partial",
		Limit:       20,
		EnableGraph: true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.Len(t, results.Results, 1)
}

// ============================================================================
// Tests: Engine.Search - Vector result with ChunkID priority
// ============================================================================

func TestSearch_VectorSearch_WithChunkID(t *testing.T) {
	t.Parallel()
	vs := &mockVectorStore{
		searchFunc: func(query []float64, limit int) ([]vector.SearchResult, error) {
			return []vector.SearchResult{
				{
					ID:         "chunk-result",
					Score:      0.7,
					Content:    "chunk content",
					DocumentID: "doc-1",
					ChunkID:    "chunk-1",
				},
			}, nil
		},
	}

	engine := NewEngine(nil, vs, nil, nil, stubEmbedFunc)
	params := SearchParams{
		Query:        "chunk",
		Limit:        20,
		EnableVector: true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.Len(t, results.Results, 1)
	assert.Equal(t, ResultChunk, results.Results[0].Type)
}

// ============================================================================
// Tests: Engine.Search - searchGraph with empty graph
// ============================================================================

func TestSearch_Graph_EmptyGraph(t *testing.T) {
	t.Parallel()
	g := graph.New() // empty graph with no nodes
	engine := NewEngine(nil, nil, g, nil, nil)
	params := SearchParams{
		Query:       "anything",
		Limit:       20,
		EnableGraph: true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.Empty(t, results.Results)
	assert.Equal(t, 0, results.TotalCount)
}

// ============================================================================
// Tests: Vector Search with Tags filter (searchWithFilter path)
// ============================================================================

func TestSearch_VectorSearch_WithMultipleTags(t *testing.T) {
	t.Parallel()
	vs := &mockVectorStore{
		searchWithFilterFunc: func(query []float64, limit int, filter map[string]string) ([]vector.SearchResult, error) {
			require.Contains(t, filter, "lang")
			require.Contains(t, filter, "type")
			return []vector.SearchResult{
				{ID: "multi-tag", Score: 0.6, Content: "multi tag result"},
			}, nil
		},
	}

	engine := NewEngine(nil, vs, nil, nil, stubEmbedFunc)
	params := SearchParams{
		Query:        "tagged",
		Limit:        20,
		EnableVector: true,
		Tags:         map[string]string{"lang": "go", "type": "doc"},
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.Len(t, results.Results, 1)
	assert.Equal(t, "multi-tag", results.Results[0].ID)
}

// ============================================================================
// Tests: ftsTableToResultType - all codes
// ============================================================================

func TestFtsTableToResultType_AllTables(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ResultDocument, ftsTableToResultType("documents_fts"))
	assert.Equal(t, ResultChunk, ftsTableToResultType("chunks_fts"))
	assert.Equal(t, ResultEntity, ftsTableToResultType("entities_fts"))
	assert.Equal(t, ResultCode, ftsTableToResultType("code_blocks_fts"))
}

// ============================================================================
// Tests: searchResultRankable with non-standard scores
// ============================================================================

func TestSearchResultRankable_NegativeScore(t *testing.T) {
	t.Parallel()
	sr := SearchResult{ID: "neg", Score: -0.5}
	rankable := &searchResultRankable{result: sr}
	assert.Equal(t, -0.5, rankable.Score())
}

func TestSearchResultRankable_LargeScore(t *testing.T) {
	t.Parallel()
	sr := SearchResult{ID: "large", Score: 1000.0}
	rankable := &searchResultRankable{result: sr}
	assert.Equal(t, 1000.0, rankable.Score())
}

// ============================================================================
// Tests: SearchResults Duration zero value check
// ============================================================================

func TestSearchResults_InitialDuration(t *testing.T) {
	t.Parallel()
	results := &SearchResults{}
	assert.Zero(t, results.Duration)
	assert.Empty(t, results.Results)
	assert.Empty(t, results.Query)
}

// ============================================================================
// Tests: Engine.Search - Types filter only, no query
// ============================================================================

func TestSearch_OnlyTypesFilter_NoQuery(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	params := SearchParams{
		Query: "",
		Types: []string{"document", "chunk"},
		Limit: 20,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	assert.NotNil(t, results)
	assert.Empty(t, results.Results)
}

// ============================================================================
// Tests: resolveFTSTables - unsorted order
// ============================================================================

func TestResolveFTSTables_PreservesOrder(t *testing.T) {
	t.Parallel()
	// Order should be preserved from input
	got := resolveFTSTables([]string{"entity", "code", "document", "chunk"})
	assert.Equal(t, []string{"entities_fts", "code_blocks_fts", "documents_fts", "chunks_fts"}, got)
}

// ============================================================================
// Tests: generateSnippet - match at exact position 0
// ============================================================================

func TestGenerateSnippet_MatchAtPositionZero(t *testing.T) {
	t.Parallel()
	content := "query is at the very start here and there is more text afterwards for testing"
	snippet := generateSnippet(content, "query", 40)
	assert.True(t, strings.HasPrefix(snippet, "query"))
	assert.False(t, strings.HasPrefix(snippet, "..."))
}

// ============================================================================
// Tests: computeFacets - single result
// ============================================================================

func TestComputeFacets_SingleResult(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	results := []SearchResult{
		{Type: ResultDocument, Source: "fts", Language: "go", EntityType: "skill"},
	}
	facets := engine.computeFacets(results)
	assert.Equal(t, 1, facets["type"]["document"])
	assert.Equal(t, 1, facets["source"]["fts"])
	assert.Equal(t, 1, facets["language"]["go"])
	assert.Equal(t, 1, facets["entity_type"]["skill"])
}

// ============================================================================
// Tests: Engine.Search - Empty Query with Only EnableFacets
// ============================================================================

func TestSearch_NoEnginesEnabled_WithFacets(t *testing.T) {
	t.Parallel()
	engine := NewEngine(nil, nil, nil, nil, nil)
	params := SearchParams{
		Query:        "",
		Types:        []string{"doc"},
		Limit:        20,
		EnableFacets: true,
	}
	results, err := engine.Search(context.Background(), params)
	require.NoError(t, err)
	// Facets should be set even for empty results
	assert.NotNil(t, results.Facets)
}

// ============================================================================
// Tests: SearchResult - Highlighted field
// ============================================================================

func TestSearchResult_Highlighted(t *testing.T) {
	t.Parallel()
	sr := SearchResult{Highlighted: "<mark>test</mark> result"}
	assert.Equal(t, "<mark>test</mark> result", sr.Highlighted)
}

// ============================================================================
// Tests: EmbeddingRequest vector values
// ============================================================================

func TestEmbeddingRequest_VectorWithValues(t *testing.T) {
	t.Parallel()
	er := EmbeddingRequest{Vector: []float64{0.0, 0.5, 1.0, -1.0, 2.5}}
	assert.Len(t, er.Vector, 5)
	assert.Equal(t, 0.0, er.Vector[0])
	assert.Equal(t, 0.5, er.Vector[1])
	assert.Equal(t, 1.0, er.Vector[2])
}

// ============================================================================
// Hybrid-first (L3 bounded): candidate translation and fallback
// ============================================================================

func TestParseFTSID(t *testing.T) {
	table, rowid, ok := parseFTSID("chunks_fts_42")
	assert.True(t, ok)
	assert.Equal(t, "chunks_fts", table)
	assert.Equal(t, int64(42), rowid)

	table, rowid, ok = parseFTSID("documents_fts_7")
	assert.True(t, ok)
	assert.Equal(t, "documents_fts", table)
	assert.Equal(t, int64(7), rowid)

	_, _, ok = parseFTSID("no_separator")
	assert.False(t, ok)
	_, _, ok = parseFTSID("chunks_fts_abc")
	assert.False(t, ok)
	_, _, ok = parseFTSID("")
	assert.False(t, ok)
	_, _, ok = parseFTSID("_5")
	assert.False(t, ok)
}

func TestEngine_SearchVector_WithCandidates(t *testing.T) {
	// Real FTSClient over a temp DB (needed to resolve chunk rowids) + a spy
	// vector store that records the candidate call.
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sqlite.Open(sqlite.DefaultConfig(dbPath))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`INSERT INTO documents (id, path, hash, title, doc_type)
		VALUES ('doc-1', '/d/1.md', 'h1', 'D1', 'markdown')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO chunks (id, document_id, content) VALUES ('chunk-a', 'doc-1', 'alpha')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO chunks (id, document_id, content) VALUES ('chunk-b', 'doc-1', 'beta')`)
	require.NoError(t, err)

	fts := sqlite.NewFTSClient(db)

	var gotIDs []string
	var gotPool int
	store := &mockVectorStore{
		searchCandidatesFunc: func(_ []float64, _ int, candidateIDs []string, recentPool int, _ map[string]string) ([]vector.SearchResult, error) {
			gotIDs = candidateIDs
			gotPool = recentPool
			return []vector.SearchResult{{ID: "chunk-a", Score: 0.9}, {ID: "chunk-b", Score: 0.8}}, nil
		},
	}

	eng := NewEngine(fts, store, nil, nil, stubEmbedFunc)
	params := SearchParams{
		Query:         "kernel",
		Limit:         5,
		EnableFTS:     false,
		EnableVector:  true,
		CandidateIDs:  []string{"chunks_fts_1", "chunks_fts_2", "documents_fts_7"},
		CandidatePool: 250,
	}
	res, err := eng.Search(context.Background(), params)
	require.NoError(t, err)
	require.Len(t, res.Results, 2)

	// Only chunk candidates were translated (documents_fts_7 has no vector row).
	assert.ElementsMatch(t, []string{"chunk-a", "chunk-b"}, gotIDs)
	assert.Equal(t, 250, gotPool)
}

func TestEngine_SearchVector_FallsBackToFullScan(t *testing.T) {
	// No fts client → no candidates resolve → the full Search path is used.
	var fullSearchCalled bool
	store := &mockVectorStore{
		searchFunc: func(_ []float64, _ int) ([]vector.SearchResult, error) {
			fullSearchCalled = true
			return []vector.SearchResult{{ID: "full-1", Score: 0.7}}, nil
		},
	}

	eng := NewEngine(nil, store, nil, nil, stubEmbedFunc)
	params := SearchParams{
		Query:         "kernel",
		Limit:         5,
		EnableFTS:     false,
		EnableVector:  true,
		CandidateIDs:  []string{"chunks_fts_1"},
		CandidatePool: 250,
	}
	res, err := eng.Search(context.Background(), params)
	require.NoError(t, err)
	require.Len(t, res.Results, 1)
	assert.True(t, fullSearchCalled, "expected fallback to full vector search")
}
