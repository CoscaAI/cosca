// Package search provides the hybrid search engine for the Cosca Knowledge Engine.
// It combines FTS5 full-text search, vector similarity search, and graph traversal
// into a unified search pipeline with re-ranking.
package search

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/graph"
	"github.com/CoscaAI/cosca/internal/modlink"
	"github.com/CoscaAI/cosca/internal/ranking"
	"github.com/CoscaAI/cosca/internal/sqlite"
	"github.com/CoscaAI/cosca/internal/vector"
	"github.com/rs/zerolog/log"
)

// ResultType categorizes search results.
type ResultType string

// Predefined search result types.
const (
	ResultDocument  ResultType = "document"
	ResultChunk     ResultType = "chunk"
	ResultEntity    ResultType = "entity"
	ResultCode      ResultType = "code_block"
	ResultKnowledge ResultType = "knowledge"
)

// SearchResult represents a single search result from the hybrid engine.
//
//nolint:revive // Stutter name preserved for API compatibility — used as search.SearchResult externally.
type SearchResult struct {
	ID           string            `json:"id"`
	Type         ResultType        `json:"type"`
	Score        float64           `json:"score"`
	Title        string            `json:"title"`
	Content      string            `json:"content"`
	Snippet      string            `json:"snippet"`
	Highlighted  string            `json:"highlighted,omitempty"`
	DocumentID   string            `json:"document_id,omitempty"`
	DocumentPath string            `json:"document_path,omitempty"`
	Heading      string            `json:"heading,omitempty"`
	SectionType  string            `json:"section_type,omitempty"`
	EntityType   string            `json:"entity_type,omitempty"`
	Language     string            `json:"language,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Rank         int               `json:"rank"`
	Source       string            `json:"source"` // "fts", "vector", "graph"
}

// SearchResults holds the complete set of search results.
//
//nolint:revive // Stutter name preserved for API compatibility — used as search.SearchResults externally.
type SearchResults struct {
	Results     []SearchResult            `json:"results"`
	TotalCount  int                       `json:"total_count"`
	Query       string                    `json:"query"`
	Facets      map[string]map[string]int `json:"facets,omitempty"`
	Duration    time.Duration             `json:"duration_ms"`
	Suggestions []string                  `json:"suggestions,omitempty"`
}

// SearchParams defines parameters for a hybrid search query.
//
//nolint:revive // Stutter name preserved for API compatibility — used as search.SearchParams externally.
type SearchParams struct {
	// Query is the search query string.
	Query string

	// Limit is the maximum number of results (default: 20).
	Limit int

	// Offset is the number of results to skip (default: 0).
	Offset int

	// Types restricts results to specific entity/document types.
	Types []string

	// Path restricts results to a specific path prefix.
	Path string

	// Tags restricts results to those with specific metadata tags.
	Tags map[string]string

	// Since restricts results to items updated after this time.
	Since time.Time

	// EnableFTS enables full-text search (default: true).
	EnableFTS bool

	// EnableVector enables vector search (default: true).
	EnableVector bool

	// EnableGraph enables graph traversal (default: false).
	EnableGraph bool

	// EnableFacets enables faceted result counting (default: false).
	EnableFacets bool

	// MinScore filters results below this score threshold.
	MinScore float64

	// CandidateIDs (layered search, L3) restricts the vector layer to these
	// candidates instead of a full brute-force scan. Two vocabularies are
	// accepted:
	//
	//   - FTS-style ("chunks_fts_<rowid>") — the legacy lexical candidate set
	//     (hybrid-first/L3 bounded): chunk hits are resolved to vector-store
	//     IDs and the rest contribute nothing. Valid with or without a routed
	//     scope.
	//   - Direct vector IDs (the vectoragg.SearchRequest.CandidateIDs
	//     vocabulary) — the permitted candidates verbatim (the vector store
	//     `id` column). Honoured ONLY when a routed scope is present (ADR-013
	//     §3.2, Fase B): the deterministic router chose the space, so the
	//     vector phase stays confined to these candidates instead of the full
	//     brute-force scan. Without a routed scope the full-scan fallback is
	//     the legitimate baseline and these IDs are dropped.
	//
	// Empty = full vector scan.
	CandidateIDs []string

	// CandidatePool is the number of recent vectors scored alongside
	// CandidateIDs (hybrid-first recency pool). Ignored when CandidateIDs is
	// empty. When <= 0, no recency pool is added.
	CandidatePool int

	// Scope, quando não-nil e com Modules não-vazio, confina a busca híbrida ao
	// espaço de busca roteado pelo modlink (ADR-013 §3.2): apenas resultados que
	// mapeiam a um dos módulos do escopo (via `documents.path` ou `entity_type`)
	// são retornados; os demais são descartados. A busca REFINA o espaço já
	// escolhido pelo roteador determinístico — ela nunca escolhe o espaço.
	//
	// Retrocompatível (invariante do professor): quando Scope é nil OU tem
	// Modules vazio (ex.: um escopo NoRoute), a busca é ilimitada — exatamente o
	// comportamento atual. Não cria coluna `domain`/`module` no banco (isso é
	// Fatia 3, condicionada a ter conteúdo de mundo indexado); usa o sinal
	// honesto já existente: o path do documento (e o entity_type).
	Scope *modlink.SearchScope
}

// DefaultSearchParams returns sensible defaults.
func DefaultSearchParams() SearchParams {
	return SearchParams{
		Limit:        20,
		EnableFTS:    true,
		EnableVector: true,
		EnableGraph:  false,
		EnableFacets: false,
	}
}

// Engine is the hybrid search engine combining FTS5, vector, and graph search.
type Engine struct {
	fts       *sqlite.FTSClient
	vecStore  vector.Store
	graph     *graph.Graph
	ranker    *ranking.Ranker
	embedFunc func(ctx context.Context, text string) (*EmbeddingRequest, error)

	// MetricsSink, quando definido, recebe as métricas do caminho real de cada
	// busca vetorial (campanha de performance — FASE 1: descobrir quantos
	// vetores chegam de fato ao kernel). Nil-safe: sem sink, custo zero.
	MetricsSink func(vector.SearchMetrics)
}

// EmbeddingRequest mirrors a simplified embedding result for the search engine.
type EmbeddingRequest struct {
	Vector []float64
}

// NewEngine creates a new hybrid search engine.
func NewEngine(
	ftsClient *sqlite.FTSClient,
	vecStore vector.Store,
	g *graph.Graph,
	ranker *ranking.Ranker,
	embedFunc func(ctx context.Context, text string) (*EmbeddingRequest, error),
) *Engine {
	return &Engine{
		fts:       ftsClient,
		vecStore:  vecStore,
		graph:     g,
		ranker:    ranker,
		embedFunc: embedFunc,
	}
}

// Search performs a hybrid search across all available indexes.
func (e *Engine) Search(ctx context.Context, params SearchParams) (*SearchResults, error) {
	start := time.Now()

	if params.Query == "" && len(params.Types) == 0 && params.Path == "" {
		return nil, fmt.Errorf("query or filter required")
	}
	if params.Limit <= 0 {
		params.Limit = 20
	}
	// Pagination hard caps (M9b — DoS hardening, last line of defense).
	// REST and gRPC layers also clamp, but no caller may force unbounded
	// result materialization at the engine boundary.
	const (
		maxEngineLimit  = 100
		maxEngineOffset = 1000
	)
	if params.Limit > maxEngineLimit {
		params.Limit = maxEngineLimit
	}
	if params.Offset < 0 {
		params.Offset = 0
	}
	if params.Offset > maxEngineOffset {
		params.Offset = maxEngineOffset
	}

	results := &SearchResults{
		Query:    params.Query,
		Duration: 0,
	}

	var allResults []SearchResult
	seen := make(map[string]bool)

	// Phase 1: FTS5 Search
	if params.EnableFTS && params.Query != "" {
		ftsResults, err := e.searchFTS(params)
		if err != nil {
			log.Warn().Err(err).Msg("fts search failed")
		} else {
			for _, r := range ftsResults {
				if !seen[r.ID] {
					r.Source = "fts"
					allResults = append(allResults, r)
					seen[r.ID] = true
				}
			}
		}
	}

	// Phase 2: Vector Search
	if params.EnableVector && params.Query != "" && e.embedFunc != nil {
		vecResults, err := e.searchVector(ctx, params)
		if err != nil {
			log.Warn().Err(err).Msg("vector search failed")
		} else {
			for _, r := range vecResults {
				if !seen[r.ID] {
					r.Source = "vector"
					allResults = append(allResults, r)
					seen[r.ID] = true
				}
			}
		}
	}

	// Phase 3: Graph Traversal
	if params.EnableGraph && e.graph != nil && params.Query != "" {
		graphResults, err := e.searchGraph(params)
		if err != nil {
			log.Warn().Err(err).Msg("graph search failed")
		} else {
			for _, r := range graphResults {
				if !seen[r.ID] {
					r.Source = "graph"
					allResults = append(allResults, r)
					seen[r.ID] = true
				}
			}
		}
	}

	// Phase 3.5 — confinamento por módulo (ADR-013 §3.2, Fatia 2).
	// Quando um `Scope` roteado (modlink) veio com módulos, a busca NUNCA
	// "pesquisa tudo": apenas resultados que mapeiam a um dos módulos do escopo
	// sobrevivem, e os demais são descartados do espaço roteado. Scope nil ou
	// Modules vazio → busca ilimitada (comportamento atual, retrocompatível).
	// Isso acontece ANTES do re-rank: o ranking opera só sobre o espaço já
	// confinado, e `totalCount` conta apenas os hits in-scope.
	if params.Scope != nil && len(params.Scope.Modules) > 0 {
		allResults = confineToScope(allResults, params.Scope.Modules)
	}

	// Phase 4: Re-rank — only when results come from multiple sources.
	// Single-source results (FTS-only or vector-only) keep their raw scores.
	hasFTS := params.EnableFTS && params.Query != ""
	hasVector := params.EnableVector && params.Query != "" && e.embedFunc != nil
	hasGraph := params.EnableGraph && e.graph != nil && params.Query != ""
	sourceCount := 0
	if hasFTS {
		sourceCount++
	}
	if hasVector {
		sourceCount++
	}
	if hasGraph {
		sourceCount++
	}
	if sourceCount > 1 && params.Query != "" && e.ranker != nil {
		// Re-ranking multi-fator: reordena pelos scores combinados E PROPAGA o
		// score final normalizado de volta (bug #2 — o Score exposto antes
		// permanecia o cosseno/BM25 cru). O módulo de ranking é reutilizado por
		// completo (pesos/fórmulas intactos).
		allResults = rerankResults(e.ranker, allResults, params.Query, e.graph)
	} else {
		// Sort by score descending (preserves raw vector/BM25 scores)
		sort.Slice(allResults, func(i, j int) bool {
			return allResults[i].Score > allResults[j].Score
		})
	}

	// Apply offset and limit
	totalCount := len(allResults)
	if params.Offset > 0 && params.Offset < len(allResults) {
		allResults = allResults[params.Offset:]
	}
	if len(allResults) > params.Limit {
		allResults = allResults[:params.Limit]
	}

	// Assign ranks
	for i := range allResults {
		allResults[i].Rank = i + 1 + params.Offset
	}

	results.Results = allResults
	results.TotalCount = totalCount
	results.Duration = time.Since(start)

	// Phase 5: Facets
	if params.EnableFacets {
		results.Facets = e.computeFacets(allResults)
	}

	// Phase 6: Suggestions
	if params.Query != "" {
		results.Suggestions = e.generateSuggestions(params.Query)
	}

	log.Debug().
		Str("query", params.Query).
		Int("results", len(allResults)).
		Int("total", totalCount).
		Dur("duration", results.Duration).
		Msg("search completed")

	return results, nil
}

// searchFTS performs full-text search using FTS5.
func (e *Engine) searchFTS(params SearchParams) ([]SearchResult, error) {
	tableNames := resolveFTSTables(params.Types)

	ftsParams := sqlite.FTSSearchParams{
		Query:      params.Query,
		TableNames: tableNames,
		Limit:      params.Limit * 2, // fetch more for re-ranking
		MaxSnippet: 250,
	}

	ftsResults, _, err := e.fts.Search(ftsParams)
	if err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(ftsResults))
	for _, fts := range ftsResults {
		result := SearchResult{
			ID:          fmt.Sprintf("%s_%d", fts.TableName, fts.RowID),
			Type:        ftsTableToResultType(fts.TableName),
			Score:       fts.Rank,
			Title:       fts.Title,
			Content:     truncateContent(fts.Content, 500),
			Snippet:     fts.Snippet,
			Highlighted: fts.Highlighted,
			DocumentID:  fts.DocumentID,
			Heading:     fts.Heading,
			SectionType: fts.SectionType,
			EntityType:  fts.EntityType,
			Metadata:    make(map[string]string),
		}

		if fts.Language != "" {
			result.Language = fts.Language
			if result.Metadata == nil {
				result.Metadata = make(map[string]string)
			}
			result.Metadata["language"] = fts.Language
		}
		if fts.DocType != "" {
			if result.Metadata == nil {
				result.Metadata = make(map[string]string)
			}
			result.Metadata["doc_type"] = fts.DocType
		}

		results = append(results, result)
	}

	return results, nil
}

// searchVector performs vector similarity search.
func (e *Engine) searchVector(ctx context.Context, params SearchParams) ([]SearchResult, error) {
	// Get embedding for query
	embReq, err := e.embedFunc(ctx, params.Query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	searchLimit := params.Limit * 3

	var vecResults []vector.SearchResult

	// Hybrid-first (L3 bounded) + Fase B (ADR-013 §3.2): when candidates exist
	// (lexical FTS candidates, and/or the permitted vector candidates of a
	// routed scope), restrict the vector scan to them + a recency pool instead
	// of the full O(N) brute force. Falls back to the full scan when the store
	// does not support candidate search or no candidate resolved.
	candidateIDs := e.resolveRouteCandidates(params)
	var m vector.SearchMetrics
	if len(candidateIDs) > 0 {
		if ms, ok := e.vecStore.(vector.MetricsSearcher); ok {
			var err error
			start := time.Now()
			vecResults, m, err = ms.SearchWithMetrics(embReq.Vector, searchLimit, candidateIDs, params.CandidatePool, params.Tags)
			m.Latency = time.Since(start)
			e.emitMetrics(m)
			if err != nil {
				return nil, fmt.Errorf("candidate vector search: %w", err)
			}
			return e.vectorResults(vecResults, params)
		}
		if cs, ok := e.vecStore.(vector.CandidateSearcher); ok {
			var err error
			start := time.Now()
			vecResults, err = cs.SearchWithCandidates(embReq.Vector, searchLimit, candidateIDs, params.CandidatePool, params.Tags)
			m.Latency = time.Since(start)
			e.emitMetrics(m)
			if err != nil {
				return nil, fmt.Errorf("candidate vector search: %w", err)
			}
			return e.vectorResults(vecResults, params)
		}
	}

	start := time.Now()
	// Sem candidatos: preenche as métricas reais quando o store expõe a
	// superfície instrumentada (full-scan do índice ou SQL com filtro).
	if ms, ok := e.vecStore.(vector.MetricsSearcher); ok {
		var err error
		vecResults, m, err = ms.SearchWithMetrics(embReq.Vector, searchLimit, nil, 0, params.Tags)
		m.Latency = time.Since(start)
		e.emitMetrics(m)
		if err != nil {
			return nil, fmt.Errorf("vector search: %w", err)
		}
		return e.vectorResults(vecResults, params)
	}
	if len(params.Tags) > 0 {
		vecResults, err = e.vecStore.SearchWithFilter(embReq.Vector, searchLimit, params.Tags)
	} else {
		vecResults, err = e.vecStore.Search(embReq.Vector, searchLimit)
	}
	m.Latency = time.Since(start)
	e.emitMetrics(m)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	return e.vectorResults(vecResults, params)
}

// emitMetrics forwards the raw vector-store metrics to the sink, if any.
func (e *Engine) emitMetrics(m vector.SearchMetrics) {
	if e.MetricsSink != nil {
		e.MetricsSink(m)
	}
}

// resolveChunkCandidates translates FTS-style candidate IDs
// ("chunks_fts_<rowid>") into vector-store chunk IDs. Non-chunk candidates
// (documents, entities, code blocks, knowledge) have no vector rows and are
// dropped — they still reach the final ranking through mergeRanked.
func (e *Engine) resolveChunkCandidates(candidates []string) []string {
	if len(candidates) == 0 || e.fts == nil {
		return nil
	}
	var rowids []int64
	for _, id := range candidates {
		table, rowid, ok := parseFTSID(id)
		if ok && table == "chunks_fts" {
			rowids = append(rowids, rowid)
		}
	}
	if len(rowids) == 0 {
		return nil
	}
	resolved, err := e.fts.ResolveChunkIDs(rowids)
	if err != nil {
		log.Warn().Err(err).Msg("resolve chunk candidates failed")
		return nil
	}
	out := make([]string, 0, len(resolved))
	seen := make(map[string]bool, len(resolved))
	for _, id := range resolved {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// resolveRouteCandidates translates the CandidateIDs in params into the actual
// vector-store candidate IDs that confine the phase (Fase B — ADR-013 §3.2).
// Two vocabularies are supported:
//
//   - FTS-style IDs resolve through the FTS index into chunk vector IDs (the
//     legacy lexical-candidate / L3-bounded path, valid with or without a
//     routed scope).
//   - Direct vector IDs (the vectoragg.SearchRequest.CandidateIDs vocabulary)
//     are used verbatim — they are the permitted candidates of the routed
//     space. They are honoured ONLY when the space is routed (see scopeRouted):
//     without a routed scope the full-scan fallback is the legitimate baseline
//     and these IDs are dropped (never a "candidate confinement" outside a
//     routed space).
//
// The returned IDs are exactly what the vector store's candidate search
// (`SearchWithMetrics`/`SearchWithCandidates`) expects: the `id` column of the
// vector table — the same vocabulary `vectoragg.RetrieveCandidates` consumes.
func (e *Engine) resolveRouteCandidates(params SearchParams) []string {
	var legacy []string
	var direct []string
	for _, id := range params.CandidateIDs {
		if isFTSResultID(id) {
			legacy = append(legacy, id)
		} else {
			direct = append(direct, id)
		}
	}
	out := e.resolveChunkCandidates(legacy)
	if scopeRouted(params.Scope) {
		out = mergeCandidateIDs(out, direct)
	}
	return out
}

// isFTSResultID reports whether an ID is an FTS result id of the shape
// "<fts_table>_<rowid>" for one of the known FTS tables. IDs that are NOT FTS
// result ids are direct vector IDs (the vectoragg vocabulary). This guards
// against a plain vector ID like "vec-0001" being misread as an FTS id of an
// unknown table (which parseFTSID would accept) and silently dropped.
func isFTSResultID(id string) bool {
	table, _, ok := parseFTSID(id)
	if !ok {
		return false
	}
	switch table {
	case "documents_fts", "chunks_fts", "entities_fts", "code_blocks_fts", "knowledge_fts":
		return true
	default:
		return false
	}
}

// mergeCandidateIDs appends direct vector IDs to an existing candidate list,
// deduplicating and preserving the first-occurrence order.
func mergeCandidateIDs(existing, direct []string) []string {
	if len(direct) == 0 {
		return existing
	}
	seen := make(map[string]bool, len(existing)+len(direct))
	for _, id := range existing {
		seen[id] = true
	}
	for _, id := range direct {
		if id != "" && !seen[id] {
			seen[id] = true
			existing = append(existing, id)
		}
	}
	return existing
}

// parseFTSID splits an FTS result id of the form "<table>_<rowid>" (the table
// name itself may contain underscores, so the last separator wins).
func parseFTSID(id string) (string, int64, bool) {
	i := strings.LastIndex(id, "_")
	if i <= 0 || i == len(id)-1 {
		return "", 0, false
	}
	rowid, err := strconv.ParseInt(id[i+1:], 10, 64)
	if err != nil {
		return "", 0, false
	}
	return id[:i], rowid, true
}

// vectorResults converts raw vector store results into search results.
func (e *Engine) vectorResults(vecResults []vector.SearchResult, params SearchParams) ([]SearchResult, error) {
	results := make([]SearchResult, 0, len(vecResults))

	// Propaga o path canônico do documento (documents.path) nos resultados
	// vetoriais, pois o confinamento por escopo roteado (confineToScope /
	// moduleMatches) chaveia por DocumentPath — não por DocumentID. Sem isso,
	// todo resultado vetorial é descartado pelo escopo (DocumentPath vazio ⇒
	// moduleMatches==false ⇒ confineToScope remove), produzindo recall=0 no
	// caminho roteado (BUG confirmado na auditoria).
	//
	// Materialização em lote (uma única consulta WHERE id IN (...)) para evitar
	// N+1. DocumentID vazio ⇒ sem path (resultado de cliente desconhecido).
	var docPaths map[string]string
	if e.fts != nil && len(vecResults) > 0 {
		ids := make([]string, 0, len(vecResults))
		seen := make(map[string]bool, len(vecResults))
		for _, vr := range vecResults {
			if vr.DocumentID != "" && !seen[vr.DocumentID] {
				seen[vr.DocumentID] = true
				ids = append(ids, vr.DocumentID)
			}
		}
		if len(ids) > 0 {
			if p, err := e.fts.DocumentPaths(ids); err == nil {
				docPaths = p
			}
		}
	}

	for _, vr := range vecResults {
		result := SearchResult{
			ID:         vr.ID,
			Type:       detectResultType(vr),
			Score:      vr.Score,
			Content:    truncateContent(vr.Content, 500),
			DocumentID: vr.DocumentID,
			Metadata:   vr.Metadata,
			Snippet:    generateSnippet(vr.Content, params.Query, 200),
		}
		if docPaths != nil && vr.DocumentID != "" {
			result.DocumentPath = docPaths[vr.DocumentID]
		}
		if vr.EntityID != "" {
			result.EntityType = vr.Metadata["entity_type"]
			if name := vr.Metadata["name"]; name != "" {
				result.Title = name
			}
		}
		results = append(results, result)
	}
	return results, nil
}

// searchGraph performs graph-based search (finds nodes connected to query context).
func (e *Engine) searchGraph(params SearchParams) ([]SearchResult, error) {
	// Find nodes matching the query by name
	var matchedNodes []*graph.Node

	if e.graph != nil {
		for _, node := range e.graph.FilterNodes("") {
			if strings.Contains(strings.ToLower(node.Name), strings.ToLower(params.Query)) {
				matchedNodes = append(matchedNodes, node)
			}
		}
	}

	results := make([]SearchResult, 0, len(matchedNodes))
	for _, node := range matchedNodes {
		result := SearchResult{
			ID:           node.ID,
			Type:         ResultEntity,
			Score:        0.5, // base score for graph matches
			Title:        node.Name,
			EntityType:   node.Type,
			DocumentPath: node.Path, // sinal de domínio (path) p/ o confinamento por escopo
			Metadata:     make(map[string]string),
		}

		if desc, ok := node.Metadata["description"].(string); ok {
			result.Content = truncateContent(desc, 500)
			result.Snippet = generateSnippet(desc, params.Query, 200)
		}

		// Boost score by graph centrality (number of connections)
		if neighbors, err := e.graph.GetNeighbors(node.ID); err == nil {
			boost := 0.1 * float64(len(neighbors))
			if boost > 0.5 {
				boost = 0.5
			}
			result.Score += boost
		}

		results = append(results, result)
	}

	return results, nil
}

// computeFacets computes faceted counts for results.
func (e *Engine) computeFacets(results []SearchResult) map[string]map[string]int {
	facets := make(map[string]map[string]int)

	// Type facet
	typeCounts := make(map[string]int)
	for _, r := range results {
		typeCounts[string(r.Type)]++
	}
	facets["type"] = typeCounts

	// Entity type facet
	entityCounts := make(map[string]int)
	for _, r := range results {
		if r.EntityType != "" {
			entityCounts[r.EntityType]++
		}
	}
	if len(entityCounts) > 0 {
		facets["entity_type"] = entityCounts
	}

	// Language facet
	langCounts := make(map[string]int)
	for _, r := range results {
		if r.Language != "" {
			langCounts[r.Language]++
		}
	}
	if len(langCounts) > 0 {
		facets["language"] = langCounts
	}

	// Source facet
	sourceCounts := make(map[string]int)
	for _, r := range results {
		sourceCounts[r.Source]++
	}
	facets["source"] = sourceCounts

	return facets
}

// generateSuggestions generates query suggestions.
func (e *Engine) generateSuggestions(query string) []string {
	// Simple suggestion: add common suffixes
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}

	suggestions := []string{
		query + " agent",
		query + " skill",
		query + " workflow",
		query + " template",
	}

	return suggestions
}

// ── Rankable adapter ───────────────────────────────────────────────────────

// graphDistanceMax é o teto de "distância semântica" no grafo. Um resultado que
// não está no grafo, ou que não alcança nenhum nó-semente da consulta dentro
// desse raio, recebe essa distância — o que dá graph score ≈ exp(-6/3) ≈ 0.135
// (quase sem peso de grafo). Resultados conectados ao contexto da query ficam
// com distância baixa e sobem no ranking.
const graphDistanceMax = 6

type searchResultRankable struct {
	result SearchResult

	// graph dá acesso ao grafo de conhecimento para computar GraphDistance.
	// Nil-safe: sem grafo, GraphDistance() retorna 0 (comportamento atual,
	// retrocompatível).
	graph *graph.Graph

	// query é o texto da consulta que define o "contexto" (nós-semente).
	query string

	// querySeeds é o conjunto de nós do grafo que casam com o contexto da
	// consulta (pré-computado uma vez por re-rank para não varrer o grafo a
	// cada item — O(x) por busca, não por candidato).
	querySeeds map[string]bool
}

func (r *searchResultRankable) ID() string          { return r.result.ID }
func (r *searchResultRankable) Content() string     { return r.result.Content + " " + r.result.Title }
func (r *searchResultRankable) Score() float64      { return r.result.Score }
func (r *searchResultRankable) Timestamp() int64    { return 0 }
func (r *searchResultRankable) ReferenceCount() int { return 0 }

// GraphDistance() devolve a distância real no grafo de conhecimento entre o
// nó que representa o resultado e o nó-semente (contexto) mais próximo da
// consulta.
//
// ESTRATÉGIA (honesta e deliberadamente simples — não é uma ontologia):
//   - A consulta é uma string; não existe um "nó da query" direto no grafo.
//     Então definimos o CONTEXTO da query como o conjunto de nós-semente cuja
//     name/label/path/description contém algum termo da consulta.
//   - Para o resultado, localizamos o nó do grafo correspondente por: ID exato,
//     DocumentID, caminho do documento ou proximidade de título/conteúdo.
//   - A distância é o menor caminho BFS do nó do resultado até a semente mais
//     próxima (0 = o próprio nó é contexto; 1 = vizinho direto; etc.).
//
// CASO BASE (retrocompatível): grafo vazio (GetNodeCount()==0), grafo nulo,
// consulta vazia ou resultado sem ID retornam 0 — exatamente o comportamento
// anterior (GraphDistance hardcoded 0). Como computeGraphScore(0)=1.0 é
// constante, nenhum campo do ranking é diferenciado → o ranking fica idêntico
// ao atual. Só quando o grafo tem nós e a consulta casa com algum deles é que a
// distância passa a diferenciar (resultado conectado ao contexto sobe).
func (r *searchResultRankable) GraphDistance() int {
	if r.graph == nil || r.query == "" || r.result.ID == "" || r.graph.GetNodeCount() == 0 {
		return 0 // caso base: sem grafo/contexto → sem sinal (retrocompatível)
	}

	nodeID := resultNodeID(r.graph, r.result)
	if nodeID == "" {
		// Resultado não está no grafo → sem conexão semântica de contexto.
		return graphDistanceMax
	}

	// O próprio nó é contexto da query → distância 0 (boost máximo).
	if r.querySeeds[nodeID] {
		return 0
	}

	if d, ok := nearestSeedDistance(r.graph, nodeID, r.querySeeds); ok {
		return d
	}
	// Nenhuma semente alcançável dentro do raio → tratado como desconectado.
	return graphDistanceMax
}

func (e *Engine) toRankables(results []SearchResult) []ranking.Rankable {
	rankables := make([]ranking.Rankable, len(results))
	for i, r := range results {
		rankables[i] = &searchResultRankable{result: r, graph: e.graph}
	}
	return rankables
}

func (e *Engine) fromRankables(rankables []ranking.Rankable) []SearchResult {
	results := make([]SearchResult, len(rankables))
	for i, r := range rankables {
		results[i] = r.(*searchResultRankable).result
		results[i].Rank = i + 1
	}
	return results
}

// rerankResults re-rankeia results com o Ranker multi-fator, reordenando pelos
// scores combinados e PROPAGANDO o score final normalizado de volta em cada
// SearchResult.Score (e Rank = posição). Reutiliza ranking.Rank (ordem final) e
// ranking.ExplainRanked (score combinado por item — mesmo algoritmo e mesmas
// normalizações min-max), então nenhuma lógica de score é duplicada.
//
// Retrocompatível: quando ranker == nil, a query é vazia ou não há resultados,
// devolve os resultados inalterados — o comportamento de mergeRanked por score
// cru permanece. O parâmetro `g` (grafo de conhecimento) alimenta o sinal
// GraphDistance dos rankables: quando é nil, o grafo está vazio, ou a query não
// casa com nenhum nó, o sinal é neutro (0) e o ranking fica idêntico ao atual.
func rerankResults(ranker *ranking.Ranker, results []SearchResult, query string, g *graph.Graph) []SearchResult {
	if ranker == nil || len(results) == 0 || strings.TrimSpace(query) == "" {
		return results
	}
	// Nós-semente (contexto da consulta) pré-computados uma única vez por
	// re-rank, para não varrer o grafo a cada candidato.
	seeds := graphQuerySeeds(g, query)
	rankables := make([]ranking.Rankable, len(results))
	for i := range results {
		rankables[i] = &searchResultRankable{result: results[i], graph: g, query: query, querySeeds: seeds}
	}

	// Score combinado por item. O ExplainRanked retorna um breakdown por item
	// NA MESMA ORDEM e com o MESMO algoritmo que o Rank, então casamos por
	// POSIÇÃO, não por ID: os IDs dos resultados vindos do vectorLayer (chunk_id)
	// e do FTS5 (chunks_fts_<rowid>) têm formatos diferentes e nunca bateriam
	// num mapa por chave — era a causa do score zerado na saída (Fase 2 bug).
	breaks := ranker.ExplainRanked(rankables, query)
	totalByPos := make([]float64, len(breaks))
	for i, b := range breaks {
		totalByPos[i] = b.Total
	}

	ranked := ranker.Rank(rankables, query)
	out := make([]SearchResult, len(ranked))
	for i, r := range ranked {
		sr := r.(*searchResultRankable).result
		sr.Rank = i + 1
		// O Rank reordena exatamente conforme a ordem dos breaks, então a
		// posição i no ranked corresponde à posição i no breakdown.
		if i < len(totalByPos) {
			sr.Score = totalByPos[i]
		}
		out[i] = sr
	}
	return out
}

// ── Graph context helpers (sinal semântico do grafo) ───────────────────────

// graphQuerySeeds devolve os IDs dos nós do grafo que casam com o contexto da
// consulta (por substring nos campos name/label/path/description). Pré-computado
// uma vez por re-rank. Grafo nulo ou consulta vazia → conjunto vazio (sem
// sinal — retrocompatível).
func graphQuerySeeds(g *graph.Graph, query string) map[string]bool {
	seeds := make(map[string]bool)
	if g == nil || strings.TrimSpace(query) == "" {
		return seeds
	}
	terms := tokenize(query) // reusa o tokenizer do pacote (layered.go)
	if len(terms) == 0 {
		return seeds
	}
	for _, n := range g.GetAllNodes() {
		haystack := strings.ToLower(n.Name + " " + n.Label + " " + n.Path)
		if d, ok := n.Metadata["description"].(string); ok {
			haystack += " " + strings.ToLower(d)
		}
		for _, t := range terms {
			if t != "" && strings.Contains(haystack, t) {
				seeds[n.ID] = true
				break
			}
		}
	}
	return seeds
}

// resultNodeID localiza o nó do grafo que representa um SearchResult, na ordem:
// ID exato → DocumentID → caminho do documento → proximidade de título/conteúdo.
// Retorna "" quando o resultado não tem correspondência no grafo (→ sem boost).
func resultNodeID(g *graph.Graph, sr SearchResult) string {
	if sr.ID != "" {
		if _, ok := g.GetNode(sr.ID); ok {
			return sr.ID
		}
	}
	if sr.DocumentID != "" {
		if _, ok := g.GetNode(sr.DocumentID); ok {
			return sr.DocumentID
		}
	}
	if sr.DocumentPath != "" {
		for _, n := range g.GetAllNodes() {
			if n.Path != "" && n.Path == sr.DocumentPath {
				return n.ID
			}
		}
	}
	if sr.Title != "" || sr.Content != "" {
		for _, n := range g.GetAllNodes() {
			if n.Name == "" {
				continue
			}
			if strings.EqualFold(n.Name, sr.Title) {
				return n.ID
			}
			if sr.Content != "" && strings.Contains(sr.Content, n.Name) {
				return n.ID
			}
		}
	}
	return ""
}

// nearestSeedDistance faz BFS a partir de `start` e devolve a distância (hops)
// até a semente de contexto mais próxima. Limita a expansão ao raio
// graphDistanceMax — além desse raio tratamos como "sem conexão" (retorna false),
// o que equivale a devolver graphDistanceMax. BFS garante a MENOR distância.
func nearestSeedDistance(g *graph.Graph, start string, seeds map[string]bool) (int, bool) {
	if len(seeds) == 0 {
		return 0, false
	}
	type queueItem struct {
		id    string
		depth int
	}
	visited := map[string]bool{start: true}
	queue := []queueItem{{id: start, depth: 0}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.depth >= graphDistanceMax {
			continue // não expande além do raio de contexto
		}
		neighbors, err := g.GetNeighbors(cur.id)
		if err != nil {
			continue
		}
		for _, nb := range neighbors {
			d := cur.depth + 1
			if seeds[nb.ID] {
				return d, true
			}
			if !visited[nb.ID] {
				visited[nb.ID] = true
				queue = append(queue, queueItem{id: nb.ID, depth: d})
			}
		}
	}
	return 0, false
}

// ── Helpers ────────────────────────────────────────────────────────────────

func ftsTableToResultType(table string) ResultType {
	switch table {
	case "documents_fts":
		return ResultDocument
	case "chunks_fts":
		return ResultChunk
	case "entities_fts":
		return ResultEntity
	case "code_blocks_fts":
		return ResultCode
	case "knowledge_fts":
		return ResultKnowledge
	default:
		return ResultDocument
	}
}

func detectResultType(vr vector.SearchResult) ResultType {
	if vr.EntityID != "" {
		return ResultEntity
	}
	if vr.ChunkID != "" {
		return ResultChunk
	}
	if vr.DocumentID != "" {
		return ResultDocument
	}
	return ResultDocument
}

func resolveFTSTables(types []string) []string {
	if len(types) == 0 {
		return nil // search all
	}

	tableMap := map[string]string{
		"document":   "documents_fts",
		"doc":        "documents_fts",
		"chunk":      "chunks_fts",
		"entity":     "entities_fts",
		"code":       "code_blocks_fts",
		"code_block": "code_blocks_fts",
		"knowledge":  "knowledge_fts",
	}

	var tables []string
	for _, t := range types {
		if table, ok := tableMap[t]; ok {
			tables = append(tables, table)
		}
	}

	return tables
}

func truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen-3] + "..."
}

func generateSnippet(content, query string, maxLen int) string {
	if content == "" || query == "" {
		return truncateContent(content, maxLen)
	}

	lowerContent := strings.ToLower(content)
	lowerQuery := strings.ToLower(query)

	// Find the first occurrence of any query term
	pos := strings.Index(lowerContent, lowerQuery)
	if pos < 0 {
		// Try individual terms
		for _, term := range strings.Fields(lowerQuery) {
			pos = strings.Index(lowerContent, term)
			if pos >= 0 {
				break
			}
		}
	}

	if pos < 0 {
		return truncateContent(content, maxLen)
	}

	// Extract context around the match
	start := pos - maxLen/3
	if start < 0 {
		start = 0
	}
	end := pos + len(query) + maxLen/3
	if end > len(content) {
		end = len(content)
	}

	snippet := content[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet = snippet + "..."
	}

	return snippet
}
