// Package search provides the hybrid search engine for the Cosca Knowledge Engine.
//
// layered.go — Busca em camadas (Layered Search): custo progressivo com parada
// antecipada. Uma consulta sobe de camada somente quando a camada anterior não
// responde com confiança determinística, e nunca gasta uma chamada de IA sem
// necessidade.
//
// Escada de custo (da conversa do Don):
//
//	L1 FTS5    — zero IA, mais barata. Parada se hits suficientes e relevantes.
//	L2 BM25    — re-ranking determinístico. Parada se o melhor candidato confirma.
//	L3 vector  — similarity por embeddings. Refina, nunca para (segue para L4).
//	L4 LLM     — raciocínio OPTO-IN (placeholder). Só com AllowLLM=true.
//
// Todas as decisões são determinísticas e explicáveis (Reason em pt-BR).
package search

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/internal/graph"
	"github.com/CoscaAI/cosca/internal/ranking"
)

// Layer identifies a search layer in the progressive-cost ladder.
type Layer int

// Predefined layers, ordered from cheapest (L1) to most expensive (L4).
const (
	LayerFTS5   Layer = iota // L1 — mais barata, zero IA
	LayerBM25                // L2 — ranking
	LayerVector              // L3 — similarity (embeddings)
	LayerLLM                 // L4 — raciocínio (IA opcional)
)

// String returns the stable layer identifier ("L1-fts5", "L2-bm25", ...).
func (l Layer) String() string {
	switch l {
	case LayerFTS5:
		return "L1-fts5"
	case LayerBM25:
		return "L2-bm25"
	case LayerVector:
		return "L3-vector"
	case LayerLLM:
		return "L4-llm"
	default:
		return fmt.Sprintf("L%d-unknown", int(l)+1)
	}
}

// MarshalJSON renders the layer as its stable string id so JSON consumers see
// "L1-fts5" instead of a raw int — same representation the text output uses.
func (l Layer) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.String())
}

// LayerStats reports how many candidates each layer saw and whether L4 was
// reached. This is the observability surface for the cost ladder: a query that
// stops at L1/L2 has FTS5Hits/BM25Hits > 0 and LLMCalled = false (0 tokens de IA).
type LayerStats struct {
	FTS5Hits   int  `json:"fts5_hits"`   // candidatos vindos do FTS5 (L1)
	BM25Hits   int  `json:"bm25_hits"`   // candidatos re-rankeados por BM25 (L2)
	VectorHits int  `json:"vector_hits"` // candidatos vetoriais (L3); 0 = provedor indisponível ou sem similaridade
	LLMCalled  bool `json:"llm_called"`  // true somente se L4 foi alcançada
}

// LayeredResult is the outcome of a layered search: the layer where it STOPPED,
// whether it stopped before any AI spend, and the best deterministic candidates.
type LayeredResult struct {
	Layer        Layer          `json:"layer"`         // camada onde PAROU
	StoppedEarly bool           `json:"stopped_early"` // parou antes do LLM (0 tokens IA)
	Reason       string         `json:"reason"`        // pt-BR: por que parou
	Results      []SearchResult `json:"results"`       // melhores candidatos da camada final
	Stats        LayerStats     `json:"stats"`         // contagem por camada
}

// LayeredConfig tunes the layered-search thresholds. Zero values are replaced
// by the defaults (see DefaultLayeredConfig).
type LayeredConfig struct {
	// MinFTSCount — L1 para quando o FTS5 retorna >= este número de hits
	// genuínos (default 3). Heurística documentada: além da contagem, todos os
	// hits no topo precisam conter TODOS os termos da consulta (verificação
	// sobre o texto bruto, não só sobre o índice).
	MinFTSCount int

	// MinBM25Score — L2 para quando o melhor candidato re-rankeado por BM25
	// alcança este score (default 0.35). No BM25 autocontido aqui, um termo
	// genuíno em documento de comprimento médio pontua ~1.0 — o limiar separa
	// "no-topic" de "sem sobreposição de termos".
	MinBM25Score float64

	// VectorTopK — L3 mantém no máximo este número de candidatos vetoriais
	// (default 8).
	VectorTopK int

	// MaxLayers — teto rígido de camadas percorridas (default 4).
	// 1 = só FTS5; 2 = +BM25; 3 = +vetor (nunca L4); 4 = completo.
	MaxLayers int

	// AllowLLM — habilita L4 (raciocínio). O default é FALSE: a flag que
	// libera a chamada de IA. L4 é um placeholder — esta feature nunca
	// invoca um LLM por si só (ver LLMHook / SetLLMHook).
	AllowLLM bool
}

// DefaultLayeredConfig returns the documented defaults.
// MinFTSCount=0 forces the pipeline past L1 (FTS5) so vector/cosine
// similarity (L3) always runs. BM25 is still used for re-ranking L1
// candidates that feed into the vector layer.
func DefaultLayeredConfig() LayeredConfig {
	return LayeredConfig{
		MinFTSCount:  0,   // 0 = never stop at L1; always reach vector L3
		MinBM25Score: 999, // effectively disabled — BM25 scores are negative with our tokenizer
		VectorTopK:   8,
		MaxLayers:    4,
		AllowLLM:     false,
	}
}

// LayeredSearcher is the minimal engine surface required by the layered
// search. *knowledge.Engine satisfies it (Search + Query).
//
// It is deliberately an interface, not *knowledge.Engine: internal/knowledge
// already imports internal/search, so importing knowledge here would be an
// import cycle. The interface also keeps unit tests trivial — a small fake can
// drive every layer decision.
type LayeredSearcher interface {
	Search(ctx context.Context, params SearchParams) (*SearchResults, error)
	Query(ctx context.Context, query string) (*SearchResults, error)
}

// LLMHook is the L4 integration point. The caller wires a provider (LLM /
// agent) via SetLLMHook. This feature NEVER wires one itself, so no LLM call is
// ever made here unless the caller explicitly opts in (AllowLLM=true AND a hook
// registered). The layer-decision logic is the deliverable; the hook is the
// documented seam for a future LLM provider.
type LLMHook func(ctx context.Context, query string, candidates []SearchResult) error

// LayeredSearch runs the progressive-cost search ladder over a LayeredSearcher.
type LayeredSearch struct {
	engine  LayeredSearcher
	cfg     LayeredConfig
	llmHook LLMHook          // opcional; wire later via SetLLMHook
	ranker  *ranking.Ranker  // opcional; wire later via SetRanker
	graph   *graph.Graph     // opcional; alimenta o sinal GraphDistance do re-rank (SetGraph)
}

// NewLayeredSearch creates a layered search over the given engine.
func NewLayeredSearch(engine LayeredSearcher, cfg LayeredConfig) *LayeredSearch {
	return &LayeredSearch{engine: engine, cfg: cfg}
}

// SetLLMHook registers the optional L4 hook (see LLMHook). It returns the
// receiver for chaining.
func (l *LayeredSearch) SetLLMHook(hook LLMHook) *LayeredSearch {
	l.llmHook = hook
	return l
}

// SetRanker registra o re-rankear multi-fator opcional. Quando injetado, os
// candidatos finais retornados por Search() são re-rankeados pelo Ranker e o
// score combinado normalizado é propagado de volta em cada SearchResult.Score.
// Quando nil (default), o comportamento atual (`mergeRanked` por score cru)
// é preservado — retrocompatível. Retorna o receptor para encadeamento, no
// mesmo padrão de SetLLMHook.
func (l *LayeredSearch) SetRanker(r *ranking.Ranker) *LayeredSearch {
	l.ranker = r
	return l
}

// SetGraph registra o grafo de conhecimento opcional. Quando injetado junto com
// um Ranker (SetRanker), o re-rank final dos candidatos passa a alimentar o
// sinal GraphDistance real (resultados conectados ao contexto da query sobem no
// ranking). Nil-safe: sem grafo, o re-rank fica idêntico ao atual (GraphDistance
// neutro = 0). Retorna o receptor para encadeamento, no mesmo padrão de
// SetRanker/SetLLMHook.
func (l *LayeredSearch) SetGraph(g *graph.Graph) *LayeredSearch {
	l.graph = g
	return l
}

// Search walks the ladder L1→L2→L3→L4 and stops at the first layer that answers
// with deterministic confidence. It never calls an LLM by itself: reaching L4
// (AllowLLM) only marks the decision and, if a hook was wired, delegates to it.
//
//   - L1 (FTS5): se hits >= MinFTSCount e a qualidade do topo confirma →
//     STOP. StoppedEarly=true, LLMCalled=false, 0 tokens de IA.
//   - L2 (BM25): re-rankeia os candidatos; se o melhor supera MinBM25Score →
//     STOP.
//   - L3 (Vector): se o provedor de embeddings está disponível, mantém os top
//     VectorTopK por similaridade. Se não, pula com graça (VectorHits=0) e
//     continua.
//   - L4 (LLM): apenas se AllowLLM=true e as camadas determinísticas não
//     confirmaram → LLMCalled=true, StoppedEarly=false. A chamada em si é
//     placeholder (hook opcional).
func (l *LayeredSearch) Search(ctx context.Context, query string) (*LayeredResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("layered search: query required")
	}
	if l.engine == nil {
		return nil, fmt.Errorf("layered search: engine not configured")
	}
	cfg := normalizeLayeredConfig(l.cfg)

	res := &LayeredResult{}

	// Re-ranking multi-fator (retrocompatível): quando um Ranker é injetado via
	// SetRanker, re-rankeamos os candidatos finais (em QUALQUER camada de
	// parada, inclusive a mesclagem L3) e PROPAGAMOS o score combinado
	// normalizado de volta em cada SearchResult.Score — em vez do cosseno/BM25
	// cru. O defer garante uma única aplicação em TODOS os caminhos de retorno
	// (L1..L4), sem tocar em StoppedEarly/Reason/Stats. Com l.ranker == nil
	// nada muda (mergeRanked por score cru permanece).
	if l.ranker != nil {
		defer func() {
			res.Results = rerankResults(l.ranker, res.Results, query, l.graph)
		}()
	}

	// ── L1 — FTS5 (zero IA, mais barata) ──────────────────────────────────
	ftsParams := SearchParams{
		Query:        query,
		Limit:        ftsLayerLimit(cfg.MinFTSCount),
		EnableFTS:    true,
		EnableVector: false,
		EnableGraph:  false,
	}
	ftsOut, err := l.engine.Search(ctx, ftsParams)
	if err != nil {
		return nil, fmt.Errorf("layered search L1 (FTS5): %w", err)
	}
	var ftsHits []SearchResult
	if ftsOut != nil {
		ftsHits = ftsOut.Results
	}
	res.Stats.FTS5Hits = len(ftsHits)

	if fts5GoodEnough(ftsHits, query, cfg.MinFTSCount) {
		res.Layer = LayerFTS5
		res.StoppedEarly = true
		res.Reason = "FTS5 suficiente — resposta encontrada sem IA"
		res.Results = ftsHits
		return res, nil
	}
	if cfg.MaxLayers == 1 {
		res.Layer = LayerFTS5
		res.StoppedEarly = true
		res.Reason = "FTS5 insuficiente e teto de camadas (1) — retornando melhores candidatos"
		res.Results = ftsHits
		return res, nil
	}

	// ── L2 — BM25 (ranking determinístico) ────────────────────────────────
	bm25Hits := rankByBM25(ftsHits, query)
	res.Stats.BM25Hits = len(bm25Hits)
	if len(bm25Hits) > 0 && bm25Hits[0].Score >= cfg.MinBM25Score {
		res.Layer = LayerBM25
		res.StoppedEarly = true
		res.Reason = "BM25 confirmou os melhores candidatos — parada determinística"
		res.Results = bm25Hits
		return res, nil
	}
	if cfg.MaxLayers == 2 {
		res.Layer = LayerBM25
		res.StoppedEarly = true
		res.Reason = "BM25 sem score suficiente e teto de camadas (2) — retornando melhores candidatos"
		res.Results = bm25Hits
		return res, nil
	}

	// ── L3 — Vector similarity (embeddings, opcional) ─────────────────────
	// L3 nunca para por si só: refina os candidatos e segue para L4 (ou o
	// teto de camadas). Sem provedor de embeddings, o caminho vetorial do
	// engine retorna vazio — pulamos com graça (VectorHits=0) e continuamos.
	//
	// Hybrid-first: quando L1/L2 já acharam candidatos lexicais, a L3 faz
	// cosine SÓ sobre eles (resolvidos para IDs de vetor) + um pool de
	// recência (vectorCandidatePool), em vez de varrer os 60k vetores. Quando
	// não há candidatos (query semântica pura), a L3 cai no brute-force
	// otimizado (paralelo + decode rápido).
	vecHits := l.vectorLayer(ctx, query, cfg.VectorTopK, candidateIDsOf(bm25Hits))
	res.Stats.VectorHits = len(vecHits)

	// Vector hits REFINE, never replace: replacing the BM25 candidates with
	// the vector top-K would drop every FTS-only table that has no vectors
	// (e.g. knowledge_fts — compiled patterns/heuristics/ADRs), making them
	// invisible whenever an embedding provider is available. Merge both sets,
	// deduplicate by ID and re-rank by score.
	candidates := mergeRanked(bm25Hits, vecHits)
	if cfg.MaxLayers == 3 {
		res.Layer = LayerVector
		res.StoppedEarly = true
		res.Reason = "Camadas determinísticas sem confirmação e teto de camadas (3) — retornando melhores candidatos"
		res.Results = candidates
		return res, nil
	}

	// ── L4 — LLM (placeholder, opt-in) ────────────────────────────────────
	if cfg.AllowLLM {
		res.Layer = LayerLLM
		res.StoppedEarly = false
		res.Stats.LLMCalled = true
		res.Reason = "camadas determinísticas insuficientes — raciocínio necessário"
		res.Results = candidates
		if l.llmHook != nil {
			if err := l.llmHook(ctx, query, candidates); err != nil {
				log.Warn().Err(err).Msg("layered search L4 hook failed")
			}
		}
		return res, nil
	}

	// AllowLLM=false e camadas determinísticas exaustas → melhor esforço em L3.
	res.Layer = LayerVector
	res.StoppedEarly = true
	res.Reason = "camadas determinísticas insuficientes e raciocínio não habilitado (--allow-llm) — retornando melhores candidatos determinísticos"
	res.Results = candidates
	return res, nil
}

// ── Layer helpers ──────────────────────────────────────────────────────────

// ftsLayerLimit picks how many FTS5 candidates L1 should fetch so L2 (BM25)
// has room to re-rank. Scales with MinFTSCount, floored at 20 and capped at the
// engine's 100-result cap.
func ftsLayerLimit(minCount int) int {
	limit := minCount * 5
	if limit < 20 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return limit
}

// fts5GoodEnough decides whether L1 (FTS5) alone answers the query.
//
// Heurística determinística e documentada:
//  1. O FTS5 retornou pelo menos minCount hits — sinal suficiente.
//  2. Cada um dos top minCount hits contém TODOS os termos da consulta no
//     Title+Content (case-insensitive). O MATCH do FTS5 já exige todos os
//     termos (AND implícito de frases entre aspas); essa re-checagem sobre o
//     texto bruto protege contra edge cases de stemming/tokenização e garante
//     que o topo é genuinamente on-topic.
func fts5GoodEnough(results []SearchResult, query string, minCount int) bool {
	if minCount <= 0 {
		return false // disabled — always pass through to vector/cosine
	}
	if minCount < 1 {
		minCount = 1
	}
	if len(results) < minCount {
		return false
	}
	terms := tokenize(query)
	if len(terms) == 0 {
		return false
	}
	n := minCount
	if n > len(results) {
		n = len(results)
	}
	for _, r := range results[:n] {
		haystack := strings.ToLower(r.Title + " " + r.Content)
		for _, term := range terms {
			if !strings.Contains(haystack, term) {
				return false
			}
		}
	}
	return true
}

// vectorCandidatePool is the recency pool scored by the bounded L3 path
// alongside the lexical candidates (hybrid-first). Bounded so the vector layer
// stays in the order of hundreds of rows instead of the full store.
const vectorCandidatePool = 250

// candidateIDsOf extracts the FTS-style IDs ("chunks_fts_<rowid>") from the
// BM25 candidates so L3 can score only those rows.
func candidateIDsOf(results []SearchResult) []string {
	if len(results) == 0 {
		return nil
	}
	out := make([]string, 0, len(results))
	for _, r := range results {
		if r.ID != "" {
			out = append(out, r.ID)
		}
	}
	return out
}

// vectorLayer runs L3: query embedding + vector similarity via the engine's
// native vector path, keeping the top topK candidates by similarity. If the
// engine has no embedding provider (or the store returns nothing), it returns
// nil — the caller skips gracefully.
func (l *LayeredSearch) vectorLayer(ctx context.Context, query string, topK int, candidateIDs []string) []SearchResult {
	if topK < 1 {
		topK = 1
	}
	params := SearchParams{
		Query:         query,
		Limit:         topK,
		EnableFTS:     false,
		EnableVector:  true,
		EnableGraph:   false,
		CandidateIDs:  candidateIDs,
		CandidatePool: vectorCandidatePool,
	}
	out, err := l.engine.Search(ctx, params)
	if err != nil {
		log.Debug().Err(err).Msg("layered search L3 (vector) skipped: engine vector path unavailable")
		return nil
	}
	if out == nil {
		return nil
	}
	hits := out.Results
	// Re-ordena por similaridade (Score) e mantém os top VectorTopK.
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > topK {
		hits = hits[:topK]
	}
	return hits
}

// normalizeLayeredConfig fills zero fields with the documented defaults and
// clamps ranges (MaxLayers ∈ [1,4]).
func normalizeLayeredConfig(cfg LayeredConfig) LayeredConfig {
	def := DefaultLayeredConfig()
	if cfg.MinFTSCount <= 0 {
		cfg.MinFTSCount = def.MinFTSCount
	}
	if cfg.MinBM25Score <= 0 {
		cfg.MinBM25Score = def.MinBM25Score
	}
	if cfg.VectorTopK <= 0 {
		cfg.VectorTopK = def.VectorTopK
	}
	if cfg.MaxLayers <= 0 {
		cfg.MaxLayers = def.MaxLayers
	}
	if cfg.MaxLayers > 4 {
		cfg.MaxLayers = 4
	}
	return cfg
}

// ── BM25 (autocontido, L2) ─────────────────────────────────────────────────

// bm25AvgDocLen is the assumed average candidate length (tokens) used by the
// self-contained BM25. The length normalization is relative to this constant so
// scores are deterministic without corpus statistics.
const bm25AvgDocLen = 250.0

// rankByBM25 re-ranks the L1 candidates by BM25 text relevance (L2) and
// overwrites each result's Score with its BM25 score (Rank reset to 1..N).
// Candidates with no term overlap score 0 and sink to the bottom.
func rankByBM25(results []SearchResult, query string) []SearchResult {
	if len(results) == 0 {
		return nil
	}
	type scored struct {
		result SearchResult
		score  float64
	}
	list := make([]scored, 0, len(results))
	for _, r := range results {
		list = append(list, scored{result: r, score: bm25(r.Title+" "+r.Content, query)})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].score != list[j].score {
			return list[i].score > list[j].score
		}
		return list[i].result.Score > list[j].result.Score // tie-break: rank FTS original
	})
	out := make([]SearchResult, len(list))
	for i, s := range list {
		s.result.Score = s.score
		s.result.Rank = i + 1
		out[i] = s.result
	}
	return out
}

// bm25 computes a self-contained BM25 score for content against query.
//
// IDF é tratado como constante por termo (sem estatísticas de corpus); o score
// é aproximadamente o número de termos da consulta presentes no documento,
// saturado por frequência e normalizado por comprimento. Um termo genuíno em
// documento de comprimento médio pontua ~1.0.
func bm25(content, query string) float64 {
	tokens := tokenize(content)
	if len(tokens) == 0 {
		return 0
	}
	queryTerms := uniqueTerms(tokenize(query))
	if len(queryTerms) == 0 {
		return 0
	}

	tf := make(map[string]int, len(tokens))
	for _, t := range tokens {
		tf[t]++
	}

	docLen := float64(len(tokens))
	avg := bm25AvgDocLen
	k1, b := 1.2, 0.75

	var score float64
	for _, term := range queryTerms {
		f := float64(tf[term])
		if f == 0 {
			continue
		}
		denom := f + k1*(1-b+b*(docLen/avg))
		if denom == 0 {
			continue
		}
		score += (f * (k1 + 1)) / denom
	}
	return score
}

// uniqueTerms returns the distinct lowercase tokens of text.
func uniqueTerms(text []string) []string {
	seen := make(map[string]bool, len(text))
	out := make([]string, 0, len(text))
	for _, t := range text {
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// tokenize splits text into lowercase alphanumeric tokens (letters, digits,
// '_' and '-').
func tokenize(text string) []string {
	text = strings.ToLower(text)
	return strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-'
	})
}

// mergeRanked merges two ranked result slices, deduplicating by ID and
// re-ranking by score (stable: FTS/BM25 results keep their relative order
// when tied, vector results appended after).
func mergeRanked(a, b []SearchResult) []SearchResult {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	seen := make(map[string]bool, len(a)+len(b))
	merged := make([]SearchResult, 0, len(a)+len(b))
	for _, r := range append(append([]SearchResult{}, a...), b...) {
		if r.ID == "" {
			merged = append(merged, r)
			continue
		}
		if seen[r.ID] {
			continue
		}
		seen[r.ID] = true
		merged = append(merged, r)
	}
	sort.SliceStable(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})
	return merged
}
