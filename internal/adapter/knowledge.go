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
	"github.com/CoscaAI/cosca/internal/results"
	"github.com/CoscaAI/cosca/internal/search"
)

// KnowledgeAdapter adapts a knowledge.Engine to implement the
// orchestration.KnowledgeSearcher port interface. It translates between the
// simplified orchestration-level search parameters/results and the full
// search.SearchParams/search.SearchResults used internally by the knowledge
// engine.
type KnowledgeAdapter struct {
	engine *knowledge.Engine
	// degradedReason, quando não vazio, marca as operações bem-sucedidas como
	// Degraded (paridade D6) — ex.: backend opcional offline/fallback de
	// provider. Não é um error; a operação teve efeito.
	degradedReason string
}

// NewKnowledgeAdapter creates a new KnowledgeAdapter backed by the given
// knowledge engine. The engine must be initialized (Init() called) before
// searches are performed.
func NewKnowledgeAdapter(engine *knowledge.Engine) *KnowledgeAdapter {
	return &KnowledgeAdapter{engine: engine}
}

// SetDegraded marca o adapter como degradado com o motivo informado. Operações
// bem-sucedidas passam a reportar Degraded=true no envelope (aditivo; as
// assinaturas legadas `(*T, error)` continuam inalteradas).
func (a *KnowledgeAdapter) SetDegraded(reason string) {
	a.degradedReason = reason
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

// SearchResult é a forma com envelope (paridade D6): converte o resultado de
// Search em `*results.Result`, expondo sucesso/dados/degradado na fronteira
// sem ambiguidade de `nil/error`. Aditivo — Search continua disponível.
func (a *KnowledgeAdapter) SearchResult(ctx context.Context, params orchestration.KnowledgeSearchParams) *results.Result {
	data, err := a.Search(ctx, params)
	return adapterResult(data, err, a.degradedReason)
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
