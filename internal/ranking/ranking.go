// Package ranking provides multi-factor re-ranking for search results.
// It combines BM25, cosine similarity, graph proximity, freshness, and popularity
// into a unified relevance score.
package ranking

import (
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Rankable defines an item that can be ranked.
type Rankable interface {
	// ID returns the unique identifier.
	ID() string

	// Content returns the text content for BM25 scoring.
	Content() string

	// Score returns the initial similarity score (e.g., from vector search).
	Score() float64

	// Timestamp returns the last modified time (Unix timestamp).
	Timestamp() int64

	// ReferenceCount returns how many other items reference this one.
	ReferenceCount() int

	// GraphDistance returns the distance from the query context in the knowledge graph.
	// Lower is closer (0 = same node).
	GraphDistance() int
}

// Config defines the weighting factors for the ranking formula.
type Config struct {
	// BM25Weight is the weight for BM25 text matching score.
	BM25Weight float64

	// VectorWeight is the weight for cosine similarity score.
	VectorWeight float64

	// GraphWeight is the weight for graph proximity score.
	GraphWeight float64

	// FreshnessWeight is the weight for recency (freshness) score.
	FreshnessWeight float64

	// PopularityWeight is the weight for reference count score.
	PopularityWeight float64

	// BM25K is the BM25 k1 parameter (default: 1.2).
	BM25K float64

	// BM25B is the BM25 b parameter (default: 0.75).
	BM25B float64

	// FreshnessHalfLife is the half-life in seconds for freshness decay (default: 7 days).
	FreshnessHalfLife float64

	// NormalizeScores enables min-max normalization before combining (default: true).
	NormalizeScores bool
}

// DefaultConfig returns sensible default weighting factors.
func DefaultConfig() Config {
	return Config{
		BM25Weight:        0.25,
		VectorWeight:      0.35,
		GraphWeight:       0.20,
		FreshnessWeight:   0.10,
		PopularityWeight:  0.10,
		BM25K:             1.2,
		BM25B:             0.75,
		FreshnessHalfLife: 7 * 24 * 3600, // 7 days in seconds
		NormalizeScores:   true,
	}
}

// Ranker performs multi-factor re-ranking of search results.
type Ranker struct {
	cfg       Config
	corpusAVG float64 // average document length in the corpus (for BM25)
}

// New creates a new Ranker with the given configuration.
//
// Um Config vazio (struct zero) não deve produzir um ranker que zera o score:
// sem os pesos default, o total ponderado seria 0.25*0+0.35*0+...=0 e TODO
// resultado sairia com score 0 (bug da Fase 2 no caminho real do knowledge
// search, onde o engine cria `ranking.New(e.cfg.RankingConfig)` com a struct
// zero). Aplicamos os defaults de DefaultConfig a qualquer campo zero — o
// chamador que quiser pesos personalizados passa todos os que importam.
func New(cfg Config) *Ranker {
	def := DefaultConfig()
	if cfg.BM25Weight <= 0 {
		cfg.BM25Weight = def.BM25Weight
	}
	if cfg.VectorWeight <= 0 {
		cfg.VectorWeight = def.VectorWeight
	}
	if cfg.GraphWeight <= 0 {
		cfg.GraphWeight = def.GraphWeight
	}
	if cfg.FreshnessWeight <= 0 {
		cfg.FreshnessWeight = def.FreshnessWeight
	}
	if cfg.PopularityWeight <= 0 {
		cfg.PopularityWeight = def.PopularityWeight
	}
	if cfg.BM25K <= 0 {
		cfg.BM25K = def.BM25K
	}
	if cfg.BM25B <= 0 {
		cfg.BM25B = def.BM25B
	}
	if cfg.FreshnessHalfLife <= 0 {
		cfg.FreshnessHalfLife = def.FreshnessHalfLife
	}
	// NormalizeScores é bool: na struct zero ele é false, mas o default é true.
	// Usamos um sentinela: se NENHUM campo foi setado (config realmente vazia),
	// aplicamos o default de normalização também. Quem passa uma config
	// personalizada com NormalizeScores=false explícito mantém o comportamento
	// (o caso de config totalmente vazia + NormalizeScores=false é intencional
	// apenas quando todos os pesos também são setados — raro).
	if cfg == (Config{}) {
		cfg.NormalizeScores = def.NormalizeScores
	}
	return &Ranker{cfg: cfg}
}

// SetCorpusAvg sets the average document length for BM25 scoring.
func (r *Ranker) SetCorpusAvg(avg float64) {
	r.corpusAVG = avg
}

// Rank re-ranks items based on the query and multi-factor scoring.
func (r *Ranker) Rank(items []Rankable, query string) []Rankable {
	if len(items) == 0 {
		return items
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return items
	}

	// Precompute individual scores
	type scoredItem struct {
		item    Rankable
		bm25    float64
		vector  float64
		graph   float64
		fresh   float64
		popular float64
		final   float64
	}

	scored := make([]scoredItem, len(items))
	for i, item := range items {
		scored[i] = scoredItem{
			item:    item,
			bm25:    r.computeBM25(item.Content(), query),
			vector:  item.Score(),
			graph:   r.computeGraphScore(item.GraphDistance()),
			fresh:   r.computeFreshness(item.Timestamp()),
			popular: r.computePopularity(item.ReferenceCount()),
		}
	}

	// Normalize scores if enabled
	if r.cfg.NormalizeScores {
		normalizeField := func(setter func(int, float64), getter func(int) float64) {
			minVal, maxVal := math.Inf(1), math.Inf(-1)
			for i := range scored {
				val := getter(i)
				if val < minVal {
					minVal = val
				}
				if val > maxVal {
					maxVal = val
				}
			}
			valueRange := maxVal - minVal
			if valueRange > 1e-10 {
				for i := range scored {
					setter(i, (getter(i)-minVal)/valueRange)
				}
			}
		}

		normalizeField(func(i int, v float64) { scored[i].bm25 = v }, func(i int) float64 { return scored[i].bm25 })
		normalizeField(func(i int, v float64) { scored[i].vector = v }, func(i int) float64 { return scored[i].vector })
		normalizeField(func(i int, v float64) { scored[i].graph = v }, func(i int) float64 { return scored[i].graph })
		normalizeField(func(i int, v float64) { scored[i].fresh = v }, func(i int) float64 { return scored[i].fresh })
		normalizeField(func(i int, v float64) { scored[i].popular = v }, func(i int) float64 { return scored[i].popular })
	}

	// Compute weighted final score
	for i := range scored {
		scored[i].final =
			r.cfg.BM25Weight*scored[i].bm25 +
				r.cfg.VectorWeight*scored[i].vector +
				r.cfg.GraphWeight*scored[i].graph +
				r.cfg.FreshnessWeight*scored[i].fresh +
				r.cfg.PopularityWeight*scored[i].popular
	}

	// Sort by final score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].final > scored[j].final
	})

	result := make([]Rankable, len(scored))
	for i, s := range scored {
		result[i] = s.item
	}

	return result
}

// computeBM25 computes the BM25 score for a document against the query.
func (r *Ranker) computeBM25(content, query string) float64 {
	if content == "" || query == "" {
		return 0
	}

	docLen := float64(countWords(content))
	if docLen == 0 {
		return 0
	}

	avgDocLen := r.corpusAVG
	if avgDocLen <= 0 {
		avgDocLen = docLen
	}

	k1 := r.cfg.BM25K
	b := r.cfg.BM25B

	queryTerms := tokenize(query)
	if len(queryTerms) == 0 {
		return 0
	}

	docTerms := tokenize(content)
	termFreq := make(map[string]int)
	for _, term := range docTerms {
		termFreq[term]++
	}

	var score float64
	for _, term := range queryTerms {
		tf := float64(termFreq[term])
		if tf == 0 {
			continue
		}

		// BM25 with simplified IDF (no corpus statistics needed)
		idf := math.Log(1 + (1 / (tf + 1)))

		numer := tf * (k1 + 1)
		denom := tf + k1*(1-b+b*(docLen/avgDocLen))
		score += idf * numer / denom
	}

	return score
}

// computeGraphScore converts graph distance to a proximity score.
func (r *Ranker) computeGraphScore(distance int) float64 {
	if distance < 0 {
		return 0
	}
	if distance == 0 {
		return 1.0
	}
	// Exponential decay with distance
	return math.Exp(-float64(distance) / 3.0)
}

// computeFreshness computes a recency score based on timestamp.
func (r *Ranker) computeFreshness(timestamp int64) float64 {
	if timestamp <= 0 {
		return 0.5 // neutral score for unknown timestamps
	}

	age := float64(time.Now().Unix() - timestamp)
	if age <= 0 {
		return 1.0
	}

	halfLife := r.cfg.FreshnessHalfLife
	return math.Exp2(-age / halfLife)
}

// computePopularity computes a score based on reference count.
func (r *Ranker) computePopularity(refCount int) float64 {
	if refCount <= 0 {
		return 0
	}
	// Logarithmic scaling to prevent extremely popular items from dominating
	return math.Log2(float64(refCount+1)) / 10.0
}

// ReciprocalRankFusion combines multiple ranked result lists using RRF.
// Each list must be sorted by relevance descending.
func ReciprocalRankFusion(lists [][]Rankable, limit int) []Rankable {
	if len(lists) == 0 {
		return nil
	}

	type rrfEntry struct {
		item  Rankable
		score float64
	}

	itemMap := make(map[string]*rrfEntry)
	k := 60.0 // RRF constant

	for _, list := range lists {
		for rank, item := range list {
			id := item.ID()
			if _, exists := itemMap[id]; !exists {
				itemMap[id] = &rrfEntry{item: item}
			}
			// RRF formula: score += 1 / (k + rank)
			itemMap[id].score += 1.0 / (k + float64(rank+1))
		}
	}

	// Convert to slice and sort
	entries := make([]*rrfEntry, 0, len(itemMap))
	for _, entry := range itemMap {
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].score > entries[j].score
	})

	if limit <= 0 || limit > len(entries) {
		limit = len(entries)
	}

	result := make([]Rankable, limit)
	for i, entry := range entries[:limit] {
		result[i] = entry.item
	}

	return result
}

// NormalizeScores min-max normalizes a slice of scores in-place.
func NormalizeScores(scores []float64) {
	if len(scores) == 0 {
		return
	}

	minVal, maxVal := scores[0], scores[0]
	for _, s := range scores {
		if s < minVal {
			minVal = s
		}
		if s > maxVal {
			maxVal = s
		}
	}

	valueRange := maxVal - minVal
	if valueRange < 1e-10 {
		for i := range scores {
			scores[i] = 0.5
		}
		return
	}

	for i := range scores {
		scores[i] = (scores[i] - minVal) / valueRange
	}
}

// ── Helpers ────────────────────────────────────────────────────────────────

// countWords approximates the number of words in a string.
func countWords(s string) int {
	if strings.TrimSpace(s) == "" {
		return 0
	}
	return len(strings.Fields(s))
}

// tokenize splits text into lowercase tokens for BM25.
func tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}
