package ranking

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestExplain_NilItem(t *testing.T) {
	t.Parallel()

	r := New(DefaultConfig())
	if _, err := r.Explain(nil); err == nil {
		t.Error("Explain(nil) deveria retornar erro")
	}
}

func TestExplain_AllSignalsPresent(t *testing.T) {
	t.Parallel()

	r := New(DefaultConfig())
	item := &testItem{
		id:            "a",
		content:       "backup diário de dados com retenção",
		score:         0.9,
		timestamp:     time.Now().Unix(),
		refCount:      12,
		graphDistance: 1,
	}

	b, err := r.Explain(item)
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	if b.ItemID != "a" {
		t.Errorf("ItemID = %q, want a", b.ItemID)
	}
	if b.Total <= 0 {
		t.Errorf("Total deveria ser > 0, got %f", b.Total)
	}
	for _, f := range []string{"bm25", "vector", "graph", "freshness", "popularity"} {
		if _, ok := b.Signals[f]; !ok {
			t.Errorf("Signals deveria conter %q", f)
		}
	}
	if len(b.Signals) != 5 {
		t.Errorf("Signals deveria ter 5 entradas, got %d", len(b.Signals))
	}
}

func TestExplain_TotalIsWeightedSum(t *testing.T) {
	t.Parallel()

	r := New(DefaultConfig())
	item := &testItem{
		id:            "a",
		content:       "backup diário de dados",
		score:         0.9,
		timestamp:     time.Now().Unix(),
		refCount:      12,
		graphDistance: 1,
	}

	b, err := r.Explain(item)
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}

	want := r.cfg.BM25Weight*b.BM25 +
		r.cfg.VectorWeight*b.Vector +
		r.cfg.GraphWeight*b.Graph +
		r.cfg.FreshnessWeight*b.Freshness +
		r.cfg.PopularityWeight*b.Popularity
	if math.Abs(b.Total-want) > 1e-9 {
		t.Errorf("Total = %f, want %f (soma ponderada dos sinais)", b.Total, want)
	}

	var sum float64
	for _, v := range b.Signals {
		sum += v
	}
	if math.Abs(sum-b.Total) > 1e-9 {
		t.Errorf("soma dos Signals = %f, want %f (deve bater com Total)", sum, b.Total)
	}
}

func TestExplain_SignalsDifferAcrossItems(t *testing.T) {
	t.Parallel()

	now := time.Now()
	r := New(DefaultConfig())
	items := []Rankable{
		&testItem{
			id: "fresh", content: "backup novo diário",
			score: 0.5, timestamp: now.Unix(), refCount: 1, graphDistance: 2,
		},
		&testItem{
			id: "old", content: "backup antigo legado",
			score: 0.5, timestamp: now.Add(-60 * 24 * 3600 * time.Second).Unix(), refCount: 1, graphDistance: 2,
		},
		&testItem{
			id: "popular", content: "backup amplo corporativo",
			score: 0.5, timestamp: now.Unix(), refCount: 500, graphDistance: 2,
		},
	}

	byID := make(map[string]ScoreBreakdown)
	for _, b := range r.ExplainRanked(items, "backup") {
		byID[b.ItemID] = b
	}

	if !(byID["fresh"].Freshness > byID["old"].Freshness) {
		t.Errorf("item recente deveria ter frescor maior (fresh=%f, old=%f)",
			byID["fresh"].Freshness, byID["old"].Freshness)
	}
	if !(byID["popular"].Popularity > byID["fresh"].Popularity) {
		t.Errorf("item muito referenciado deveria ter popularidade maior (popular=%f, fresh=%f)",
			byID["popular"].Popularity, byID["fresh"].Popularity)
	}
	if !(byID["old"].Popularity == byID["fresh"].Popularity) {
		t.Errorf("mesmo refCount deveria dar mesma popularidade (old=%f, fresh=%f)",
			byID["old"].Popularity, byID["fresh"].Popularity)
	}
}

// Consistency invariant: ExplainRanked devolve, para cada item, o MESMO score
// final que o Rank usa, e na MESMA ordem.
func TestExplainRanked_ConsistentWithRank(t *testing.T) {
	t.Parallel()

	r := New(DefaultConfig())
	now := time.Now()
	items := []Rankable{
		&testItem{
			id: "low", content: "banco de dados",
			score: 0.2, timestamp: now.Add(-90 * 24 * 3600 * time.Second).Unix(), refCount: 0, graphDistance: 5,
		},
		&testItem{
			id: "mid", content: "backup de dados diário",
			score: 0.5, timestamp: now.Add(-20 * 24 * 3600 * time.Second).Unix(), refCount: 4, graphDistance: 2,
		},
		&testItem{
			id: "high", content: "backup de dados crítico com retenção",
			score: 0.9, timestamp: now.Unix(), refCount: 30, graphDistance: 0,
		},
	}

	query := "backup"
	ranked := r.Rank(items, query)
	breakdowns := r.ExplainRanked(items, query)

	if len(ranked) != len(breakdowns) {
		t.Fatalf("len(Rank)=%d, len(ExplainRanked)=%d", len(ranked), len(breakdowns))
	}

	// 1. Mesma ordem de Rank.
	for i := range ranked {
		if ranked[i].ID() != breakdowns[i].ItemID {
			t.Fatalf("posição %d: Rank=%s, ExplainRanked=%s — ordem divergente",
				i, ranked[i].ID(), breakdowns[i].ItemID)
		}
	}

	// 2. Total interno consistente (Total == Σ peso × sinal) e
	//    Total == Σ dos Signals.
	for _, b := range breakdowns {
		want := r.cfg.BM25Weight*b.BM25 +
			r.cfg.VectorWeight*b.Vector +
			r.cfg.GraphWeight*b.Graph +
			r.cfg.FreshnessWeight*b.Freshness +
			r.cfg.PopularityWeight*b.Popularity
		if math.Abs(b.Total-want) > 1e-9 {
			t.Errorf("[%s] Total = %f, want %f", b.ItemID, b.Total, want)
		}
		var sum float64
		for _, v := range b.Signals {
			sum += v
		}
		if math.Abs(sum-b.Total) > 1e-9 {
			t.Errorf("[%s] soma(Signals) = %f, want %f", b.ItemID, sum, b.Total)
		}
	}
}

// Consistency invariant (recomputation independente): replicando o algoritmo
// do Rank com as mesmas primitivas + normalização min-max inline, o Total do
// breakdown deve bater com o score do Rank item a item.
func TestExplainRanked_TotalEqualsRecomputedRankScore(t *testing.T) {
	t.Parallel()

	r := New(DefaultConfig())
	now := time.Now()
	items := []Rankable{
		&testItem{
			id: "a", content: "backup de dados",
			score: 0.8, timestamp: now.Unix(), refCount: 10, graphDistance: 1,
		},
		&testItem{
			id: "b", content: "banco de dados",
			score: 0.4, timestamp: now.Add(-40 * 24 * 3600 * time.Second).Unix(), refCount: 2, graphDistance: 3,
		},
		&testItem{
			id: "c", content: "cópia de segurança",
			score: 0.6, timestamp: now.Add(-5 * 24 * 3600 * time.Second).Unix(), refCount: 6, graphDistance: 2,
		},
	}
	query := "backup"

	type sc struct{ bm25, vector, graph, fresh, pop float64 }
	raw := make([]sc, len(items))
	for i, it := range items {
		raw[i] = sc{
			r.computeBM25(it.Content(), query),
			it.Score(),
			r.computeGraphScore(it.GraphDistance()),
			r.computeFreshness(it.Timestamp()),
			r.computePopularity(it.ReferenceCount()),
		}
	}

	// Normalização min-max inline, idêntica ao Rank.
	norm := func(get func(int) float64, set func(int, float64)) {
		minV, maxV := math.Inf(1), math.Inf(-1)
		for i := range raw {
			v := get(i)
			if v < minV {
				minV = v
			}
			if v > maxV {
				maxV = v
			}
		}
		rng := maxV - minV
		if rng > 1e-10 {
			for i := range raw {
				set(i, (get(i)-minV)/rng)
			}
		}
	}
	norm(func(i int) float64 { return raw[i].bm25 }, func(i int, v float64) { raw[i].bm25 = v })
	norm(func(i int) float64 { return raw[i].vector }, func(i int, v float64) { raw[i].vector = v })
	norm(func(i int) float64 { return raw[i].graph }, func(i int, v float64) { raw[i].graph = v })
	norm(func(i int) float64 { return raw[i].fresh }, func(i int, v float64) { raw[i].fresh = v })
	norm(func(i int) float64 { return raw[i].pop }, func(i int, v float64) { raw[i].pop = v })

	expected := make(map[string]float64, len(items))
	for i, it := range items {
		expected[it.ID()] = r.cfg.BM25Weight*raw[i].bm25 +
			r.cfg.VectorWeight*raw[i].vector +
			r.cfg.GraphWeight*raw[i].graph +
			r.cfg.FreshnessWeight*raw[i].fresh +
			r.cfg.PopularityWeight*raw[i].pop
	}

	breakdowns := r.ExplainRanked(items, query)
	if len(breakdowns) != len(items) {
		t.Fatalf("ExplainRanked devolveu %d breakdowns, want %d", len(breakdowns), len(items))
	}
	for _, b := range breakdowns {
		if math.Abs(b.Total-expected[b.ItemID]) > 1e-9 {
			t.Errorf("[%s] Total = %f, want %f (score real do Rank)", b.ItemID, b.Total, expected[b.ItemID])
		}
	}
}

// Explain(item) deve ser consistente com o Rank para o conjunto degenerado
// de um único item (normalização = identidade).
func TestExplain_MatchesSingletonRank(t *testing.T) {
	t.Parallel()

	r := New(DefaultConfig())
	item := &testItem{
		id: "a", content: "backup diário",
		score: 0.7, timestamp: time.Now().Unix(), refCount: 5, graphDistance: 1,
	}

	b, err := r.Explain(item)
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	single := r.ExplainRanked([]Rankable{item}, "")[0]
	if math.Abs(b.Total-single.Total) > 1e-12 {
		t.Errorf("Explain(item).Total = %f, ExplainRanked singleton = %f", b.Total, single.Total)
	}
	if b.ItemID != single.ItemID {
		t.Errorf("ItemID divergente: %s vs %s", b.ItemID, single.ItemID)
	}
}

func TestExplain_JustificationMentionsDominantSignal(t *testing.T) {
	t.Parallel()

	// Vetor com o maior peso (0.35) domina por construção.
	b := ScoreBreakdown{
		Total: 0.87,
		Signals: map[string]float64{
			signalBM25:       0.20,
			signalVector:     0.35,
			signalGraph:      0.15,
			signalFreshness:  0.10,
			signalPopularity: 0.07,
		},
	}
	just := b.Justification()
	if !strings.Contains(just, "Vetor") {
		t.Errorf("justificação deveria citar Vetor: %q", just)
	}
	if !strings.Contains(just, "(dominante)") {
		t.Errorf("justificação deveria marcar (dominante): %q", just)
	}
	if !strings.Contains(just, "BM25") {
		t.Errorf("justificação deveria citar o segundo sinal (BM25): %q", just)
	}
}

func TestExplain_JustificationAllZero(t *testing.T) {
	t.Parallel()

	b := ScoreBreakdown{
		Total: 0,
		Signals: map[string]float64{
			signalBM25: 0, signalVector: 0, signalGraph: 0,
			signalFreshness: 0, signalPopularity: 0,
		},
	}
	just := b.Justification()
	if !strings.Contains(just, "não apresentou sinais positivos") {
		t.Errorf("justificação para sinais zerados deveria ser a mensagem de fallback: %q", just)
	}
}

func TestExplainRanked_EmptyAndNoQuery(t *testing.T) {
	t.Parallel()

	r := New(DefaultConfig())
	if r.ExplainRanked(nil, "q") != nil {
		t.Error("ExplainRanked(nil) deveria retornar nil")
	}

	items := []Rankable{
		&testItem{id: "x", content: "backup", score: 0.9, timestamp: time.Now().Unix(), refCount: 3, graphDistance: 0},
	}
	bd := r.ExplainRanked(items, "   ")
	if len(bd) != 1 {
		t.Fatalf("ExplainRanked com query vazia deveria devolver 1 breakdown, got %d", len(bd))
	}
	if bd[0].ItemID != "x" {
		t.Errorf("ItemID = %s, want x", bd[0].ItemID)
	}
	// Query vazia → BM25 = 0 (espelha o Rank, que retorna os itens sem reordenação).
	if bd[0].BM25 != 0 {
		t.Errorf("BM25 com query vazia deveria ser 0, got %f", bd[0].BM25)
	}
}
