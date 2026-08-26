package engine

import (
	"context"
	"fmt"

	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/modlink"
	"github.com/CoscaAI/cosca/internal/search"
)

// Compile-time interface check.
var _ KnowledgeSearcher = (*knowledgeAdapter)(nil)

// knowledgeAdapter wraps knowledge.Engine to implement KnowledgeSearcher.
// It translates between engine-level KnowledgeSearchParams and the internal
// search.SearchParams used by the knowledge engine.
//
// FASE 1 (routing/scope): quando configurado com um resolver e o modo
// `modular`, o adapter roteia a query e confina a busca ao espaço roteado. Uma
// rota desconhecida (NoRoute) resulta em 0 resultados + o sinal NoRoute=true —
// NUNCA um full-scan silencioso. Em modo `legacy` (default) nada muda.
type knowledgeAdapter struct {
	engine   *knowledge.Engine
	resolver *modlink.Resolver // nil = sem roteamento (legacy)
	mode     string            // search.ModeLegacy (default) | search.ModeModular
}

// NewKnowledgeAdapter creates a new adapter backed by the given knowledge engine.
// The engine must be initialized (Init() called) before searches are performed.
func NewKnowledgeAdapter(engine *knowledge.Engine) *knowledgeAdapter {
	return &knowledgeAdapter{engine: engine, mode: search.ModeLegacy}
}

// WithScope configura o adapter para o roteamento determinístico: um resolver
// (modlink) e o modo (`legacy` ou `modular`). Em modo `modular` com resolver
// não-nil, a busca passa a ser confinada ao espaço roteado e uma NoRoute nunca
// faz full-scan. Devolve o próprio adapter para encadeamento.
func (a *knowledgeAdapter) WithScope(resolver *modlink.Resolver, mode string) *knowledgeAdapter {
	a.resolver = resolver
	if mode == "" {
		mode = search.ModeLegacy
	}
	a.mode = mode
	return a
}

// Search implements KnowledgeSearcher.Search.
func (a *knowledgeAdapter) Search(ctx context.Context, params KnowledgeSearchParams) (*KnowledgeSearchResults, error) {
	sp := toSearchParams(params)

	if a.mode == search.ModeModular && a.resolver != nil {
		var scope *modlink.SearchScope
		sp, scope = search.ApplyScope(a.resolver, params.Query, sp)
		if scope.NoRoute {
			// NUNCA full-scan silencioso: sem espaço semântico confiável →
			// retrieval 0 + sinal NO_ROUTE explícito.
			return &KnowledgeSearchResults{NoRoute: true, Query: params.Query, Scope: scope}, nil
		}
	}

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
	sp.Scope = p.Scope

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
