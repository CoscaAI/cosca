package search

import (
	"context"
	"fmt"
	"testing"

	"github.com/CoscaAI/cosca/internal/graph"
	"github.com/CoscaAI/cosca/internal/vector"
)

func BenchmarkFTS5Search(b *testing.B) {
	engine := &MockSearchEngine{
		SearchFunc: func(_ context.Context, params SearchParams) (*SearchResults, error) {
			return &SearchResults{
				Results: []SearchResult{
					{ID: "1", Type: ResultDocument, Score: 0.95, Title: "Test Result", Content: "test content for search benchmarking", Source: "fts"},
				},
				TotalCount: 1,
				Query:      params.Query,
			}, nil
		},
	}

	ctx := context.Background()
	params := SearchParams{
		Query:        "test query",
		Limit:        20,
		EnableFTS:    true,
		EnableVector: false,
		EnableGraph:  false,
	}

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_, err := engine.Search(ctx, params)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSearchEngineCreation(b *testing.B) {
	for i := 0; b.Loop(); i++ {
		_ = DefaultSearchParams()
	}
}

func BenchmarkSearchParamsValidation(b *testing.B) {
	params := SearchParams{
		Query:        "test query for validation benchmark",
		Limit:        20,
		Offset:       0,
		EnableFTS:    true,
		EnableVector: true,
		EnableFacets: false,
	}
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		if params.Query == "" && len(params.Types) == 0 && params.Path == "" {
			b.Fatal("query or filter required")
		}
	}
}

func BenchmarkResultSorting(b *testing.B) {
	results := make([]SearchResult, 100)
	for i := 0; i < 100; i++ {
		results[i] = SearchResult{
			ID:    string(rune('A' + i%26)),
			Score: float64(100-i) / 100.0,
		}
	}
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		for j := 0; j < len(results); j++ {
			for k := j + 1; k < len(results); k++ {
				if results[k].Score > results[j].Score {
					results[j], results[k] = results[k], results[j]
				}
			}
		}
	}
}

// ── Enhanced Benchmarks ─────────────────────────────────────────────────────

// BenchmarkSearch_GraphOnly measures graph traversal search performance.
func BenchmarkSearch_GraphOnly(b *testing.B) {
	g := graph.New()
	for i := 0; i < 50; i++ {
		_ = g.AddNode(&graph.Node{
			ID:   fmt.Sprintf("node-%d", i),
			Type: "",
			Name: fmt.Sprintf("Performance Agent %d", i),
			Metadata: map[string]interface{}{
				"description": fmt.Sprintf("Agent %d handles performance optimization.", i),
			},
		})
	}

	engine := NewEngine(nil, nil, g, nil, nil)
	params := SearchParams{
		Query:       "performance",
		Limit:       20,
		EnableGraph: true,
	}

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		results, err := engine.Search(context.Background(), params)
		if err != nil {
			b.Fatal(err)
		}
		_ = results
	}
}

// BenchmarkSearch_VectorOnly measures the vector search path with mock store.
func BenchmarkSearch_VectorOnly(b *testing.B) {
	vs := &mockVectorStore{
		searchFunc: func(query []float64, limit int) ([]vector.SearchResult, error) {
			results := make([]vector.SearchResult, limit)
			for i := 0; i < limit; i++ {
				results[i] = vector.SearchResult{
					ID:         fmt.Sprintf("vec-%d", i),
					Score:      0.95 - float64(i)*0.01,
					Content:    fmt.Sprintf("Content for vector result %d.", i),
					DocumentID: fmt.Sprintf("doc-%d", i%10),
				}
			}
			return results, nil
		},
	}

	engine := NewEngine(nil, vs, nil, nil, stubEmbedFunc)
	params := SearchParams{
		Query:        "performance optimization",
		Limit:        20,
		EnableVector: true,
	}

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		results, err := engine.Search(context.Background(), params)
		if err != nil {
			b.Fatal(err)
		}
		_ = results
	}
}

// BenchmarkSearch_Hybrid simulates the full hybrid search pipeline (vector + graph).
func BenchmarkSearch_Hybrid(b *testing.B) {
	g := graph.New()
	for i := 0; i < 10; i++ {
		_ = g.AddNode(&graph.Node{
			ID:   fmt.Sprintf("gn-%d", i),
			Type: "",
			Name: fmt.Sprintf("Node %d", i),
			Metadata: map[string]interface{}{
				"description": fmt.Sprintf("Description for node %d.", i),
			},
		})
	}

	vs := &mockVectorStore{
		searchFunc: func(query []float64, limit int) ([]vector.SearchResult, error) {
			n := min(limit, 10)
			results := make([]vector.SearchResult, n)
			for i := 0; i < n; i++ {
				results[i] = vector.SearchResult{
					ID:         fmt.Sprintf("v-%d", i),
					Score:      0.9 - float64(i)*0.05,
					Content:    fmt.Sprintf("Hybrid content %d", i),
					DocumentID: fmt.Sprintf("doc-%d", i),
				}
			}
			return results, nil
		},
	}

	engine := NewEngine(nil, vs, g, nil, stubEmbedFunc)
	params := SearchParams{
		Query:        "hybrid search test",
		Limit:        20,
		EnableFTS:    false, // No FTS client
		EnableVector: true,
		EnableGraph:  true,
	}

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		results, err := engine.Search(context.Background(), params)
		if err != nil {
			b.Fatal(err)
		}
		_ = results
	}
}

// BenchmarkSearch_Deduplication measures deduplication overhead with many results.
func BenchmarkSearch_ResultDedup(b *testing.B) {
	vs := &mockVectorStore{
		searchFunc: func(query []float64, limit int) ([]vector.SearchResult, error) {
			results := make([]vector.SearchResult, 100)
			for i := 0; i < 100; i++ {
				results[i] = vector.SearchResult{
					ID:         fmt.Sprintf("v-%d", i),
					Score:      0.5 + float64(i)*0.004,
					Content:    fmt.Sprintf("Large result set content %d", i),
					DocumentID: fmt.Sprintf("doc-%d", i%20),
				}
			}
			return results, nil
		},
	}

	engine := NewEngine(nil, vs, nil, nil, stubEmbedFunc)
	params := SearchParams{
		Query:        "large result test",
		Limit:        50,
		EnableVector: true,
	}

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		results, err := engine.Search(context.Background(), params)
		if err != nil {
			b.Fatal(err)
		}
		_ = results
	}
}

// BenchmarkSearch_FacetsEnabled measures facet computation overhead.
func BenchmarkSearch_FacetsEnabled(b *testing.B) {
	vs := &mockVectorStore{
		searchFunc: func(query []float64, limit int) ([]vector.SearchResult, error) {
			results := make([]vector.SearchResult, 20)
			for i := 0; i < 20; i++ {
				results[i] = vector.SearchResult{
					ID:         fmt.Sprintf("f-%d", i),
					Score:      0.8,
					Content:    "facet test",
					DocumentID: fmt.Sprintf("doc-%d", i),
					Metadata:   map[string]string{"language": "go", "entity_type": "skill"},
				}
			}
			return results, nil
		},
	}

	engine := NewEngine(nil, vs, nil, nil, stubEmbedFunc)
	params := SearchParams{
		Query:        "facet search",
		Limit:        20,
		EnableVector: true,
		EnableFacets: true,
	}

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		results, err := engine.Search(context.Background(), params)
		if err != nil {
			b.Fatal(err)
		}
		_ = results
	}
}

// BenchmarkSearch_Suggestions measures query suggestion generation overhead.
func BenchmarkSearch_Suggestions(b *testing.B) {
	engine := NewEngine(nil, nil, nil, nil, nil)
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_ = engine.generateSuggestions("performance optimization")
	}
}

// BenchmarkResolveFTSTables tests table resolution logic.
func BenchmarkResolveFTSTables(b *testing.B) {
	types := []string{"document", "chunk", "entity", "code_block"}
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_ = resolveFTSTables(types)
	}
}

// BenchmarkTruncateContent tests content truncation.
func BenchmarkTruncateContent(b *testing.B) {
	content := "This is a long content string that needs to be truncated for display purposes in search results."
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_ = truncateContent(content, 50)
	}
}

// BenchmarkGenerateSnippet tests snippet generation with match.
func BenchmarkGenerateSnippet(b *testing.B) {
	content := "The Cosca knowledge engine provides hybrid search combining FTS5 full-text search with vector similarity and graph traversal."
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_ = generateSnippet(content, "hybrid search", 100)
	}
}

// BenchmarkSearchResultCreation measures struct allocation overhead.
func BenchmarkSearchResultCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_ = SearchResult{
			ID:           "result-1",
			Type:         ResultDocument,
			Score:        0.95,
			Title:        "Performance Analysis Document",
			Content:      "Detailed content about performance optimization strategies.",
			Snippet:      "optimization strategies...",
			Highlighted:  "<mark>Performance</mark> Analysis",
			DocumentID:   "doc-1",
			DocumentPath: "/docs/performance.md",
			Heading:      "Optimization",
			SectionType:  "text",
			Language:     "go",
			Metadata:     map[string]string{"author": "cosca-performance"},
			Rank:         1,
			Source:       "fts",
		}
	}
}

// BenchmarkSearch_FTS5TableToResultType tests lookup mapping.
func BenchmarkSearch_FTS5TableToResultType(b *testing.B) {
	tables := []string{"documents_fts", "chunks_fts", "entities_fts", "code_blocks_fts"}
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		for _, t := range tables {
			_ = ftsTableToResultType(t)
		}
	}
}
