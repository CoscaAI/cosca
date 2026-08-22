// Package adapter provides concrete adapters that wire internal subsystem
// implementations into the orchestration engine's port interfaces. Each adapter
// translates between domain-level types and orchestration-level projections.
//
// The adapters never import orchestration into the source packages — they always
// map in the orchestration → internal direction, keeping the source packages
// independent of the orchestration layer.
package adapter

import (
	"context"
	"fmt"

	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/orchestration"
	"github.com/CoscaAI/cosca/internal/search"
)

// KnowledgeAdapter adapts a knowledge.Engine to implement the
// orchestration.KnowledgeSearcher port interface. It translates between the
// simplified orchestration-level search parameters/results and the full
// search.SearchParams/search.SearchResults used internally by the knowledge
// engine.
type KnowledgeAdapter struct {
	engine *knowledge.Engine
}

// NewKnowledgeAdapter creates a new KnowledgeAdapter backed by the given
// knowledge engine. The engine must be initialized (Init() called) before
// searches are performed.
func NewKnowledgeAdapter(engine *knowledge.Engine) *KnowledgeAdapter {
	return &KnowledgeAdapter{engine: engine}
}

// Search implements orchestration.KnowledgeSearcher. It converts orchestration
// search parameters to internal search parameters, delegates to the knowledge
// engine's Search method, and maps the results back to orchestration-level
// results.
func (a *KnowledgeAdapter) Search(ctx context.Context, params orchestration.KnowledgeSearchParams) (*orchestration.KnowledgeSearchResults, error) {
	sp := toSearchParams(params)

	sr, err := a.engine.Search(ctx, sp)
	if err != nil {
		return nil, fmt.Errorf("knowledge adapter search: %w", err)
	}

	return toOrchSearchResults(sr), nil
}

// ── Mapping: orchestration → search ────────────────────────────────────────

// toSearchParams converts orchestration-level KnowledgeSearchParams to the
// internal search.SearchParams used by the search engine.
func toSearchParams(p orchestration.KnowledgeSearchParams) search.SearchParams {
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

// ── Mapping: search → orchestration ────────────────────────────────────────

// toOrchSearchResults converts internal search.SearchResults to orchestration-level
// KnowledgeSearchResults.
func toOrchSearchResults(sr *search.SearchResults) *orchestration.KnowledgeSearchResults {
	if sr == nil {
		return &orchestration.KnowledgeSearchResults{
			Results:    []orchestration.KnowledgeSearchResult{},
			TotalCount: 0,
		}
	}

	results := make([]orchestration.KnowledgeSearchResult, 0, len(sr.Results))
	for _, r := range sr.Results {
		results = append(results, toOrchSearchResult(r))
	}

	return &orchestration.KnowledgeSearchResults{
		Results:    results,
		TotalCount: sr.TotalCount,
		Query:      sr.Query,
	}
}

// toOrchSearchResult converts a single internal search.SearchResult to an
// orchestration-level KnowledgeSearchResult.
func toOrchSearchResult(r search.SearchResult) orchestration.KnowledgeSearchResult {
	title := r.Title
	if title == "" && r.Heading != "" {
		title = r.Heading
	}

	return orchestration.KnowledgeSearchResult{
		ID:           r.ID,
		Title:        title,
		Content:      r.Content,
		Snippet:      r.Snippet,
		Score:        r.Score,
		DocumentPath: r.DocumentPath,
	}
}
