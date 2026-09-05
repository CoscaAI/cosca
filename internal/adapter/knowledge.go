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
	"github.com/CoscaAI/cosca/internal/modlink"
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
	// resolver/mode (FASE 1 routing/scope, ADR-013 §3.2): quando configurado
	// com um resolver e o modo `modular`, o adapter roteia a query e confina a
	// busca ao espaço roteado. Uma rota desconhecida (NoRoute) resulta em 0
	// resultados + o sinal NoRoute=true — NUNCA um full-scan silencioso. Em
	// modo `legacy` (default) nada muda.
	resolver *modlink.Resolver
	mode     string
}

// NewKnowledgeAdapter creates a new KnowledgeAdapter backed by the given
// knowledge engine. The engine must be initialized (Init() called) before
// searches are performed.
func NewKnowledgeAdapter(engine *knowledge.Engine) *KnowledgeAdapter {
	return &KnowledgeAdapter{engine: engine, mode: search.ModeLegacy}
}

// SetDegraded marca o adapter como degradado com o motivo informado. Operações
// bem-sucedidas passam a reportar Degraded=true no envelope (aditivo; as
// assinaturas legadas `(*T, error)` continuam inalteradas).
func (a *KnowledgeAdapter) SetDegraded(reason string) {
	a.degradedReason = reason
}

// WithScope configura o adapter para o roteamento determinístico: um resolver
// (modlink) e o modo (`legacy` ou `modular`). Em modo `modular` com resolver
// não-nil, a busca passa a ser confinada ao espaço roteado e uma NoRoute nunca
// faz full-scan. Devolve o próprio adapter para encadeamento.
func (a *KnowledgeAdapter) WithScope(resolver *modlink.Resolver, mode string) *KnowledgeAdapter {
	a.resolver = resolver
	if mode == "" {
		mode = search.ModeLegacy
	}
	a.mode = mode
	return a
}

// Search implements orchestration.KnowledgeSearcher. It converts orchestration
// search parameters to internal search parameters, delegates to the knowledge
// engine's Search method, and maps the results back to orchestration-level
// results.
func (a *KnowledgeAdapter) Search(ctx context.Context, params orchestration.KnowledgeSearchParams) (*orchestration.KnowledgeSearchResults, error) {
	sp := toSearchParams(params)

	if a.mode == search.ModeModular && a.resolver != nil {
		var scope *modlink.SearchScope
		sp, scope = search.ApplyScope(a.resolver, params.Query, sp)
		if scope.NoRoute {
			// NUNCA full-scan silencioso: sem espaço semântico confiável →
			// retrieval 0 + sinal NO_ROUTE explícito.
			return &orchestration.KnowledgeSearchResults{
				NoRoute: true,
				Query:   params.Query,
				Scope:   scope,
			}, nil
		}
		// FASE B (ADR-013 §3.2): confinar a fase vetorial aos candidatos
		// permitidos do escopo roteado (nunca full-scan do índice).
		if cands, cErr := a.engine.RouteCandidateIDs(scope); cErr == nil && len(cands) > 0 {
			sp.CandidateIDs = cands
		}
	}

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
	sp.Scope = p.Scope

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
		Epistemic:    r.Metadata["epistemic"], // FASE 4 — classe epistêmica do item
	}
}
