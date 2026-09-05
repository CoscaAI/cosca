package ranking

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Signal labels used in ScoreBreakdown.Signals. Each label maps to the
// weighted contribution of that signal to the final score.
const (
	signalBM25       = "bm25"
	signalVector     = "vector"
	signalGraph      = "graph"
	signalFreshness  = "freshness"
	signalPopularity = "popularity"
)

// signalLabels maps signal keys to pt-BR human-readable labels used by
// Justification.
var signalLabels = map[string]string{
	signalBM25:       "BM25 (casamento textual)",
	signalVector:     "Vetor (similaridade semântica)",
	signalGraph:      "Grafo (proximidade)",
	signalFreshness:  "Frescor (recência)",
	signalPopularity: "Popularidade (referências)",
}

// ScoreBreakdown exposes the per-signal contribution of a ranked item,
// making the multi-factor ranking fully auditable: how much each of the five
// signals (BM25, vector, graph, freshness, popularity) contributed to the
// final Total.
//
// The individual fields (BM25, Vector, Graph, Freshness, Popularity) hold the
// signal values AFTER the same min-max normalization applied by Rank (raw
// values when the normalization is the identity, e.g. single-item sets).
// Signals holds label → weighted contribution (field × its weight), and Total
// is the sum of the weighted contributions — the exact score Rank uses to sort.
type ScoreBreakdown struct {
	ItemID     string             `json:"item_id"`
	Total      float64            `json:"total"`
	BM25       float64            `json:"bm25"`
	Vector     float64            `json:"vector"`
	Graph      float64            `json:"graph"`
	Freshness  float64            `json:"freshness"`
	Popularity float64            `json:"popularity"`
	Signals    map[string]float64 `json:"signals"` // label → weighted contribution
}

// Explain recomputes the individual ranking signals for a single item.
//
// It uses the exact same scoring primitives and weights as Rank. For the
// degenerate single-item candidate set the min-max normalization is the
// identity (min == max), so Explain returns the raw signals and Total equals
// the score Rank assigns when the candidate set is {item}. For a
// multi-item ranking context (where normalization is meaningful), use
// ExplainRanked — the breakdowns it returns are guaranteed to match Rank.
func (r *Ranker) Explain(item Rankable) (ScoreBreakdown, error) {
	if item == nil {
		return ScoreBreakdown{}, fmt.Errorf("cannot explain a nil item")
	}
	return r.ExplainRanked([]Rankable{item}, "")[0], nil
}

// ExplainRanked returns the score breakdown for every item, in the same order
// Rank returns them.
//
// CONSISTENCY INVARIANT: ExplainRanked mirrors the Rank scoring path exactly —
// the same compute* primitives, the same inline min-max normalization (skipped
// when the range is degenerate or the query is empty) and the same weights.
// Therefore, for any item, breakdowns[i].Total is bit-for-bit the final score
// Rank assigns to that item, and the breakdowns are ordered identically to
// Rank's output. If Rank changes, this method must change with it.
func (r *Ranker) ExplainRanked(items []Rankable, query string) []ScoreBreakdown {
	if len(items) == 0 {
		return nil
	}
	query = strings.TrimSpace(query)

	scored := r.scoreItems(items, query)

	order := make([]int, len(scored))
	for i := range scored {
		order[i] = i
	}
	if query != "" {
		sort.Slice(order, func(a, b int) bool {
			return scored[order[a]].total > scored[order[b]].total
		})
	}

	out := make([]ScoreBreakdown, len(scored))
	for pos, idx := range order {
		s := scored[idx]
		contribs := map[string]float64{
			signalBM25:       r.cfg.BM25Weight * s.bm25,
			signalVector:     r.cfg.VectorWeight * s.vector,
			signalGraph:      r.cfg.GraphWeight * s.graph,
			signalFreshness:  r.cfg.FreshnessWeight * s.fresh,
			signalPopularity: r.cfg.PopularityWeight * s.pop,
		}
		out[pos] = ScoreBreakdown{
			ItemID:     s.item.ID(),
			Total:      s.total,
			BM25:       s.bm25,
			Vector:     s.vector,
			Graph:      s.graph,
			Freshness:  s.fresh,
			Popularity: s.pop,
			Signals:    contribs,
		}
	}
	return out
}

// Justification returns a pt-BR sentence explaining why the item ranked as it
// did, naming the dominant signal (and the runner-up) by weighted
// contribution.
func (b ScoreBreakdown) Justification() string {
	order := []string{signalBM25, signalVector, signalGraph, signalFreshness, signalPopularity}
	best, second := "", ""
	bestVal, secondVal := math.Inf(-1), math.Inf(-1)
	for _, label := range order {
		v := b.Signals[label]
		if v > bestVal {
			second, secondVal = best, bestVal
			best, bestVal = label, v
		} else if v > secondVal {
			second, secondVal = label, v
		}
	}
	if best == "" || bestVal <= 0 {
		return "O resultado não apresentou sinais positivos suficientes para justificar o ranqueamento."
	}
	if second == "" || secondVal <= 0 {
		return fmt.Sprintf("O resultado foi ranqueado por %s (dominante).", signalLabel(best))
	}
	return fmt.Sprintf("O resultado foi ranqueado por %s (dominante) e %s.", signalLabel(best), signalLabel(second))
}

// signalLabel resolves a signal key to its pt-BR human-readable label.
func signalLabel(label string) string {
	if l, ok := signalLabels[label]; ok {
		return l
	}
	return label
}

// ── Shared scoring path (mirrors Rank) ────────────────────────────────────

// scoredSignals holds the raw (or normalized) per-signal values plus the final
// weighted total for one item.
type scoredSignals struct {
	item   Rankable
	bm25   float64
	vector float64
	graph  float64
	fresh  float64
	pop    float64
	total  float64
}

// scoreItems computes per-signal values for every item and combines them into
// the final weighted total. This is the SAME algorithm used by Rank — it must
// stay in lock-step with it (see the consistency invariant on ExplainRanked).
func (r *Ranker) scoreItems(items []Rankable, query string) []scoredSignals {
	scored := make([]scoredSignals, len(items))
	for i, item := range items {
		scored[i] = scoredSignals{
			item:   item,
			bm25:   r.computeBM25(item.Content(), query),
			vector: item.Score(),
			graph:  r.computeGraphScore(item.GraphDistance()),
			fresh:  r.computeFreshness(item.Timestamp()),
			pop:    r.computePopularity(item.ReferenceCount()),
		}
	}

	// Min-max normalization across the candidate set — inline, identical to Rank.
	if r.cfg.NormalizeScores && query != "" {
		normalizeField := func(getter func(int) float64, setter func(int, float64)) {
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

		normalizeField(func(i int) float64 { return scored[i].bm25 }, func(i int, v float64) { scored[i].bm25 = v })
		normalizeField(func(i int) float64 { return scored[i].vector }, func(i int, v float64) { scored[i].vector = v })
		normalizeField(func(i int) float64 { return scored[i].graph }, func(i int, v float64) { scored[i].graph = v })
		normalizeField(func(i int) float64 { return scored[i].fresh }, func(i int, v float64) { scored[i].fresh = v })
		normalizeField(func(i int) float64 { return scored[i].pop }, func(i int, v float64) { scored[i].pop = v })
	}

	// Weighted final score — same weights as Rank.
	for i := range scored {
		scored[i].total =
			r.cfg.BM25Weight*scored[i].bm25 +
				r.cfg.VectorWeight*scored[i].vector +
				r.cfg.GraphWeight*scored[i].graph +
				r.cfg.FreshnessWeight*scored[i].fresh +
				r.cfg.PopularityWeight*scored[i].pop
	}

	return scored
}
