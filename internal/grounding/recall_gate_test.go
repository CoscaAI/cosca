// Package grounding tests — RecallGate.Run: recall@K, first_relevant_hit (MRR),
// expected_top1_hit, nDCG@k e a postura fail-closed (queries_errored / pisos).
// Determinístico e zero-LLM: o retriever é injetado como stub.
package grounding

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubRetriever devolve uma grounding.Retriever que resolve o resultado por
// query a partir de um mapa. Um erro marcado com "ERR" simula falha do
// retriever para aquela query.
func stubRetriever(byQuery map[string][]string) Retriever {
	return func(_ context.Context, query string, _ int) ([]Result, error) {
		if query == "ERR" || query == "erro" {
			return nil, errors.New("retriever falhou para essa query")
		}
		ids, ok := byQuery[query]
		if !ok {
			return nil, nil
		}
		out := make([]Result, 0, len(ids))
		for _, id := range ids {
			out = append(out, Result{ChunkID: id, Score: 1.0})
		}
		return out, nil
	}
}

func validBaseline() []Qrels {
	return []Qrels{
		{Query: "q1", RelevantChunkIDs: []string{"c1", "c2"}, FirstRelevantChunkID: "c1"},
		{Query: "q2", RelevantChunkIDs: []string{"c3"}, FirstRelevantChunkID: "c3"},
	}
}

// TestRecallGate_Pass solicita que o gate passe quando todas as queries
// atingem o piso.
func TestRecallGate_Pass(t *testing.T) {
	t.Parallel()

	gate := NewRecallGate(DefaultFloor())
	baseline := validBaseline()
	retriever := stubRetriever(map[string][]string{
		"q1": {"c1", "c2", "c4"},
		"q2": {"c3"},
	})

	rep, err := gate.Run(context.Background(), retriever, baseline)
	require.NoError(t, err)

	assert.True(t, rep.Passed)
	assert.Empty(t, rep.FailReasons)
	assert.Equal(t, 0, rep.QueriesErrored)
	assert.Equal(t, 2, rep.TotalQueries)
	assert.Equal(t, 5, rep.K, "default K = 5")
	assert.InDelta(t, 1.0, rep.RecallAtK, 1e-4)
	assert.InDelta(t, 1.0, rep.FirstRelevantHit, 1e-4)
	assert.InDelta(t, 1.0, rep.NDCGAtK, 1e-4)

	// Métricas por query.
	require.Len(t, rep.Queries, 2)
	assert.InDelta(t, 1.0, rep.Queries[0].Recall, 1e-4)
	assert.InDelta(t, 1.0, rep.Queries[0].FirstRelevant, 1e-4)
	assert.InDelta(t, 1.0, rep.Queries[0].ExpectedTop1, 1e-4)
	assert.False(t, rep.Queries[0].Errored)
}

// TestRecallGate_FailQueriesErrored: uma query do retriever que falha →
// queries_errored > 0 → fail-closed (Passed = false).
func TestRecallGate_FailQueriesErrored(t *testing.T) {
	t.Parallel()

	gate := NewRecallGate(DefaultFloor())
	baseline := []Qrels{
		{Query: "q1", RelevantChunkIDs: []string{"c1"}, FirstRelevantChunkID: "c1"},
		{Query: "ERR", RelevantChunkIDs: []string{"c2"}, FirstRelevantChunkID: "c2"},
	}
	retriever := stubRetriever(map[string][]string{"q1": {"c1"}})

	rep, err := gate.Run(context.Background(), retriever, baseline)
	require.NoError(t, err)

	assert.False(t, rep.Passed)
	assert.Equal(t, 1, rep.QueriesErrored)
	require.NotEmpty(t, rep.FailReasons)
	assert.Contains(t, rep.FailReasons[0], "errored")

	// A query com erro é marcada e as métricas ficam em zero (fail-closed).
	errored := rep.Queries[1]
	assert.True(t, errored.Errored)
	assert.Equal(t, 0.0, errored.Recall)
}

// TestRecallGate_FailRecallBelowFloor: recall@K abaixo do piso → fail.
func TestRecallGate_FailRecallBelowFloor(t *testing.T) {
	t.Parallel()

	gate := NewRecallGate(DefaultFloor())
	baseline := []Qrels{
		{Query: "q1", RelevantChunkIDs: []string{"c1", "c2"}, FirstRelevantChunkID: "c1"},
	}
	retriever := stubRetriever(map[string][]string{"q1": {"c3", "c4"}}) // nada relevante

	rep, err := gate.Run(context.Background(), retriever, baseline)
	require.NoError(t, err)

	assert.False(t, rep.Passed)
	assert.InDelta(t, 0.0, rep.RecallAtK, 1e-4)
	require.NotEmpty(t, rep.FailReasons)
	assert.Contains(t, rep.FailReasons[0], "recall@5")
	assert.Contains(t, rep.FailReasons[0], "floor")
}

// TestRecallGate_FailFirstRelevantBelowFloor: recall alto mas first_relevant
// abaixo do piso (o PRIMEIRO relevante longe do topo) → fail.
func TestRecallGate_FailFirstRelevantBelowFloor(t *testing.T) {
	t.Parallel()

	gate := NewRecallGate(DefaultFloor())
	baseline := []Qrels{
		{Query: "q1", RelevantChunkIDs: []string{"c1"}, FirstRelevantChunkID: "c1"},
	}
	// O primeiro recuperado (z1) é irrelevante; o relevante c1 aparece na 2ª
	// posição → recall 1.0 (bom) mas MRR = 1/2 = 0.5 < 0.8.
	retriever := stubRetriever(map[string][]string{"q1": {"z1", "c1"}})

	rep, err := gate.Run(context.Background(), retriever, baseline)
	require.NoError(t, err)

	assert.InDelta(t, 1.0, rep.RecallAtK, 1e-4, "recall ainda é bom")
	assert.InDelta(t, 0.5, rep.FirstRelevantHit, 1e-4)
	assert.False(t, rep.Passed)
	require.NotEmpty(t, rep.FailReasons)
	assert.Contains(t, rep.FailReasons[0], "first_relevant_hit")
}

// TestRecallGate_NilRetriever: sem retriever o gate não tem como medir →
// erro explícito (não um fail-silencioso).
func TestRecallGate_NilRetriever(t *testing.T) {
	t.Parallel()

	gate := NewRecallGate(DefaultFloor())
	_, err := gate.Run(context.Background(), nil, validBaseline())
	require.Error(t, err)
	assert.ErrorContains(t, err, "non-nil retriever")
}

// TestRecallGate_InvalidBaseline: baseline vazio/inválido → erro de validação
// (fail-closed: gabarito corrompido não passa o gate).
func TestRecallGate_InvalidBaseline(t *testing.T) {
	t.Parallel()

	gate := NewRecallGate(DefaultFloor())
	_, err := gate.Run(context.Background(), stubRetriever(nil), nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "vazio")

	_, err = gate.Run(context.Background(), stubRetriever(nil), []Qrels{{Query: "q", RelevantChunkIDs: nil}})
	require.Error(t, err)
	assert.ErrorContains(t, err, "sem relevant_chunk_ids")
}

// TestRecallGate_ZeroFloorAppliesDefaults: NewRecallGate(Floor{}) cai nos
// defaults (K=5, recall 0.60, first 0.80).
func TestRecallGate_ZeroFloorAppliesDefaults(t *testing.T) {
	t.Parallel()

	gate := NewRecallGate(Floor{})
	require.Equal(t, 5, gate.Floor.K)
	assert.InDelta(t, 0.60, gate.Floor.RecallAtK, 1e-9)
	assert.InDelta(t, 0.80, gate.Floor.FirstRelevant, 1e-9)

	rep, err := gate.Run(context.Background(), stubRetriever(map[string][]string{"q1": {"c1"}}), []Qrels{
		{Query: "q1", RelevantChunkIDs: []string{"c1"}, FirstRelevantChunkID: "c1"},
	})
	require.NoError(t, err)
	assert.Equal(t, 5, rep.K)
	assert.True(t, rep.Passed)
}

// TestRecallGate_NilRelevantSkipsMetrics: query sem relevantes (não deveria
// passar no ValidateQrels, mas se chegar) → recall 0 sem div por zero.
func TestRecallGate_ExpectedTop1MissingFirst(t *testing.T) {
	t.Parallel()

	// first_relevant_chunk_id vazio no gabarito → expected_top1 = 0 (não conta hit).
	gate := NewRecallGate(DefaultFloor())
	baseline := []Qrels{
		{Query: "q1", RelevantChunkIDs: []string{"c1"}}, // sem first_relevant
	}
	retriever := stubRetriever(map[string][]string{"q1": {"c1"}})

	rep, err := gate.Run(context.Background(), retriever, baseline)
	require.NoError(t, err)
	assert.InDelta(t, 0.0, rep.Queries[0].ExpectedTop1, 1e-9)
	assert.InDelta(t, 0.0, rep.ExpectedTop1Hit, 1e-9)
}
