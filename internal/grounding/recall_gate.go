package grounding

import (
	"context"
	"fmt"
	"math"
)

// Result é um resultado do retriever para o RecallGate: um chunk recuperado
// (ordenado por posição) e sua pontuação.
type Result struct {
	// ChunkID é o identificador do chunk (é o que os qrels comparam).
	ChunkID string
	// Score é a pontuação de relevância (BM25/cosíno), se disponível.
	Score float64
}

// Retriever é a interface de recuperação injetada no RecallGate. O gate
// permanece puro e determinístico — quem executa o retrieval (knowledge.Engine
// ou vector.Store) é o chamador. A assinatura segue o ADR §3.4: devolve os
// chunk_ids ordenados por relevância, top-k.
//
//	func(ctx, query, k) ([]Result, error)
type Retriever func(ctx context.Context, query string, k int) ([]Result, error)

// Floor é o piso de recall que o gate exige (defaults do ADR §3.4 e da fatia).
type Floor struct {
	// K é o número de resultados considerados para recall@K (default 5).
	K int
	// RecallAtK é o piso do recall@K (default 0.60).
	RecallAtK float64
	// FirstRelevant é o piso do first_relevant_hit (MRR, default 0.80).
	FirstRelevant float64
}

// DefaultFloor devolve o piso default: recall@5 >= 0.60, first_relevant_hit
// >= 0.80 (env COSCA_RECALL_FLOOR_* para override — não implementado aqui).
func DefaultFloor() Floor {
	return Floor{K: 5, RecallAtK: 0.60, FirstRelevant: 0.80}
}

// QueryRecall é o resultado por query do RecallGate.
type QueryRecall struct {
	// Query é a pergunta do gabarito.
	Query string
	// Retrieved são os chunk_ids devolvidos pelo retriever (top-k).
	Retrieved []string
	// Relevant são os chunk_ids relevantes do gabarito.
	Relevant []string
	// Recall é o recall@K desta query (0..1).
	Recall float64
	// FirstRelevant é o MRR desta query (1/rank_do_primeiro_relevante, 0..1).
	FirstRelevant float64
	// ExpectedTop1 é 1.0 quando o first_relevant_chunk_id está em retrieved[0].
	ExpectedTop1 float64
	// NDCG é o nDCG@k desta query (0..1).
	NDCG float64
	// Errored é true quando o retriever falhou para esta query.
	Errored bool
	// Err é a mensagem de erro quando Errored.
	Err string
}

// RecallReport é o relatório agregado do RecallGate.
type RecallReport struct {
	// Queries é o detalhe por query.
	Queries []QueryRecall
	// RecallAtK é a média do recall@K sobre todas as queries.
	RecallAtK float64
	// FirstRelevantHit é a média do first_relevant_hit (MRR) sobre todas.
	FirstRelevantHit float64
	// ExpectedTop1Hit é a fração de queries cuja expected top-1 foi o primeiro.
	ExpectedTop1Hit float64
	// NDCGAtK é a média do nDCG@k sobre todas as queries.
	NDCGAtK float64
	// QueriesErrored é o número de queries que o retriever não resolveu.
	QueriesErrored int
	// TotalQueries é o total de queries do gabarito.
	TotalQueries int
	// K é o K efetivamente usado.
	K int
	// Floor é o piso aplicado.
	Floor Floor
	// Passed é o veredito fail-closed do gate.
	Passed bool
	// FailReasons lista os motivos de falha (query erroda ou piso não atingido).
	FailReasons []string
}

// RecallGate é o gate determinístico de não-regressão de recall (C6).
type RecallGate struct {
	// Floor é o piso aplicado (zero cai no DefaultFloor).
	Floor Floor
}

// NewRecallGate cria um RecallGate com um piso explícito.
func NewRecallGate(floor Floor) *RecallGate {
	if floor.K <= 0 {
		floor.K = DefaultFloor().K
	}
	if floor.RecallAtK <= 0 {
		floor.RecallAtK = DefaultFloor().RecallAtK
	}
	if floor.FirstRelevant <= 0 {
		floor.FirstRelevant = DefaultFloor().FirstRelevant
	}
	return &RecallGate{Floor: floor}
}

// Run executa o gate contra um baseline de qrels. Cada query é recuperada via
// `retriever` (top-k), as métricas são computadas e o veredito fail-closed é
// aplicado:
//
//   - queries_errored > 0       → Passed = false (fail-closed: nunca descartar
//     uma query para inflar a nota — postura do ADR §3.4).
//   - recall@K < floor.RecallAtK → Passed = false.
//   - first_relevant_hit < floor.FirstRelevant → Passed = false.
//
// Quando o gabarito valida (ValidateQrels) e o retriever é nil, devolve erro:
// o gate não tem como medir sem um retriever.
func (g *RecallGate) Run(ctx context.Context, retriever Retriever, baseline []Qrels) (*RecallReport, error) {
	if retriever == nil {
		return nil, fmt.Errorf("grounding: recall gate needs a non-nil retriever")
	}
	if err := ValidateQrels(baseline); err != nil {
		return nil, err
	}
	floor := g.Floor
	if floor.K <= 0 {
		floor.K = DefaultFloor().K
	}
	if floor.RecallAtK <= 0 {
		floor.RecallAtK = DefaultFloor().RecallAtK
	}
	if floor.FirstRelevant <= 0 {
		floor.FirstRelevant = DefaultFloor().FirstRelevant
	}

	report := &RecallReport{
		Queries:      make([]QueryRecall, 0, len(baseline)),
		TotalQueries: len(baseline),
		K:            floor.K,
		Floor:        floor,
		Passed:       true,
	}

	var sumRecall, sumMRR, sumTop1, sumNDCG float64
	for _, q := range baseline {
		res := QueryRecall{
			Query:    q.Query,
			Relevant: q.RelevantChunkIDs,
		}

		got, err := retriever(ctx, q.Query, floor.K)
		if err != nil {
			res.Errored = true
			res.Err = err.Error()
			// Metrics ficam em zero (retriever não devolveu nada) e a query
			// conta como erro — fail-closed.
			report.QueriesErrored++
			report.Queries = append(report.Queries, res)
			if report.Passed {
				report.FailReasons = append(report.FailReasons,
					fmt.Sprintf("query %q errored: %v", q.Query, err))
				report.Passed = false
			}
			continue
		}

		for i := range got {
			if got[i].ChunkID != "" {
				res.Retrieved = append(res.Retrieved, got[i].ChunkID)
			}
		}
		if res.Retrieved == nil {
			res.Retrieved = []string{}
		}

		res.Recall = recallAtK(res.Relevant, res.Retrieved)
		res.FirstRelevant = firstRelevantMRR(res.Relevant, res.Retrieved)
		res.ExpectedTop1 = expectedTop1Hit(q.FirstRelevantChunkID, res.Retrieved)
		res.NDCG = ndcgAtK(res.Relevant, res.Retrieved)

		sumRecall += res.Recall
		sumMRR += res.FirstRelevant
		sumTop1 += res.ExpectedTop1
		sumNDCG += res.NDCG
		report.Queries = append(report.Queries, res)
	}

	n := float64(report.TotalQueries)
	report.RecallAtK = round4(sumRecall / n)
	report.FirstRelevantHit = round4(sumMRR / n)
	report.ExpectedTop1Hit = round4(sumTop1 / n)
	report.NDCGAtK = round4(sumNDCG / n)

	if report.QueriesErrored == 0 && report.Passed {
		if report.RecallAtK < floor.RecallAtK {
			report.Passed = false
			report.FailReasons = append(report.FailReasons,
				fmt.Sprintf("recall@%d %.2f < floor %.2f", floor.K, report.RecallAtK, floor.RecallAtK))
		}
		if report.FirstRelevantHit < floor.FirstRelevant {
			report.Passed = false
			report.FailReasons = append(report.FailReasons,
				fmt.Sprintf("first_relevant_hit %.2f < floor %.2f", report.FirstRelevantHit, floor.FirstRelevant))
		}
	}

	return report, nil
}

// recallAtK devolve |relevant ∩ retrieved| / |relevant|.
func recallAtK(relevant, retrieved []string) float64 {
	if len(relevant) == 0 {
		return 0
	}
	rel := toStringSet(relevant)
	hit := 0
	for _, id := range retrieved {
		if _, ok := rel[id]; ok {
			hit++
		}
	}
	return float64(hit) / float64(len(relevant))
}

// firstRelevantMRR devolve o recíproco da posição do primeiro chunk relevante
// (1/rank), 0 quando nenhum chunk relevante é recuperado.
func firstRelevantMRR(relevant, retrieved []string) float64 {
	rel := toStringSet(relevant)
	for i, id := range retrieved {
		if _, ok := rel[id]; ok {
			return 1.0 / float64(i+1)
		}
	}
	return 0
}

// expectedTop1Hit devolve 1.0 quando o first_relevant_chunk_id é o top-1
// recuperado. Um id vazio no gabarito não conta como hit.
func expectedTop1Hit(firstID string, retrieved []string) float64 {
	if firstID == "" || len(retrieved) == 0 {
		return 0
	}
	if retrieved[0] == firstID {
		return 1.0
	}
	// Top-1 pode ser qualquer relevante, não só o first_relevant_chunk_id.
	return 0
}

// ndcgAtK devolve o nDCG@k (bielar) para uma query. DCG usa 1/log2(i+2)
// (posição 1 → 1/log2(2) = 1.0). IDCG é o ranking ideal com todos os
// relevantes no topo.
func ndcgAtK(relevant, retrieved []string) float64 {
	rel := toStringSet(relevant)
	relCount := len(relevant)
	if relCount == 0 || len(retrieved) == 0 {
		return 0
	}

	var dcg float64
	for i, id := range retrieved {
		if _, ok := rel[id]; ok {
			dcg += 1.0 / math.Log2(float64(i+2))
		}
	}

	var idcg float64
	// Ideal: todos os relevantes nas primeiras posições (limitado por |retrieved|).
	limit := relCount
	if limit > len(retrieved) {
		limit = len(retrieved)
	}
	for i := 0; i < limit; i++ {
		idcg += 1.0 / math.Log2(float64(i+2))
	}
	if idcg == 0 {
		return 0
	}
	return dcg / idcg
}

// toStringSet converte uma lista em um set (para interseção O(1)).
func toStringSet(items []string) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, it := range items {
		set[it] = struct{}{}
	}
	return set
}
