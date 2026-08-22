package engine

import (
	"context"
	"fmt"

	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/search"
)

// Compile-time interface check.
var _ KnowledgeSearcher = (*knowledgeAdapter)(nil)

// knowledgeAdapter wraps knowledge.Engine to implement KnowledgeSearcher.
// It translates between engine-level KnowledgeSearchParams and the internal
// search.SearchParams used by the knowledge engine.
type knowledgeAdapter struct {
	engine *knowledge.Engine
}

// NewKnowledgeAdapter creates a new adapter backed by the given knowledge engine.
// The engine must be initialized (Init() called) before searches are performed.
func NewKnowledgeAdapter(engine *knowledge.Engine) *knowledgeAdapter {
	return &knowledgeAdapter{engine: engine}
}

// Search implements KnowledgeSearcher.Search.
func (a *knowledgeAdapter) Search(ctx context.Context, params KnowledgeSearchParams) (*KnowledgeSearchResults, error) {
	sp := toSearchParams(params)

	sr, err := a.engine.Search(ctx, sp)
	if err != nil {
		return nil, fmt.Errorf("knowledge adapter search: %w", err)
	}

	return toEngineKnowledgeResults(sr), nil
}

// ── Conversion: engine → search ──────────────────────────────────────────────

// toSearchParams converts engine KnowledgeSearchParams to internal search.SearchParams.
func toSearchParams(p KnowledgeSearchParams) search.SearchParams {
	sp := search.DefaultSearchParams()

	sp.Query = p.Query
	sp.Path = p.Path
	sp.MinScore = p.MinScore
	sp.EnableFTS = true
	sp.EnableVector = true
	sp.EnableGraph = false
	sp.EnableFacets = false

	if p.Limit > 0 {
		sp.Limit = p.Limit
	}

	if len(p.Types) > 0 {
		sp.Types = p.Types
	}

	return sp
}

// ── Conversion: search → engine ──────────────────────────────────────────────

// toEngineKnowledgeResults converts internal search.SearchResults to engine KnowledgeSearchResults.
func toEngineKnowledgeResults(sr *search.SearchResults) *KnowledgeSearchResults {
	if sr == nil {
		return &KnowledgeSearchResults{
			Results:    []KnowledgeSearchResult{},
			TotalCount: 0,
		}
	}

	results := make([]KnowledgeSearchResult, 0, len(sr.Results))
	for _, r := range sr.Results {
		title := r.Title
		if title == "" && r.Heading != "" {
			title = r.Heading
		}
		results = append(results, KnowledgeSearchResult{
			ID:           r.ID,
			Title:        title,
			Content:      r.Content,
			Snippet:      r.Snippet,
			Score:        r.Score,
			DocumentPath: r.DocumentPath,
		})
	}

	return &KnowledgeSearchResults{
		Results:    results,
		TotalCount: sr.TotalCount,
		Query:      sr.Query,
	}
}
