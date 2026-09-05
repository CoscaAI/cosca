// Package grounding tests — FidelityReport.Verdict + verify orquestração.
// Determinístico e zero-LLM.
package grounding

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVerdict_NoScorerFailOpen: faithfulness == -1 (scorer indisponível) NUNCA
// bloqueia — fail-open (C3/ADR-011 §2).
func TestVerdict_FailOpenOnMinusOne(t *testing.T) {
	t.Parallel()

	r := FidelityReport{Faithfulness: -1, Total: 0}
	assert.Equal(t, VerdictDeliver, r.Verdict(), "-1 deve entregar, nunca bloqueia")
}

// TestVerdict_ThresholdBoundaries cobre a régua do Verdict: block / flag /
// deliver com threshold default 0.60.
func TestVerdict_ThresholdBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		faithfulness  float64
		want          Verdict
	}{
		{"zero → block", 0.0, VerdictBlock},
		{"0.5 → block", 0.5, VerdictBlock},
		{"0.599 → block", 0.599, VerdictBlock},
		{"0.60 → flag", 0.60, VerdictFlag},
		{"0.8 → flag", 0.8, VerdictFlag},
		{"0.999 → flag", 0.999, VerdictFlag},
		{"1.0 → deliver", 1.0, VerdictDeliver},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := FidelityReport{Faithfulness: tt.faithfulness, Threshold: DefaultFaithfulnessThreshold}
			assert.Equal(t, tt.want, r.Verdict())
		})
	}
}

// TestVerdict_CustomThreshold: o chamador pode mudar o piso; o report usa o
// threshold informado.
func TestVerdict_CustomThreshold(t *testing.T) {
	t.Parallel()

	assert.Equal(t, VerdictBlock, FidelityReport{Faithfulness: 0.70, Threshold: 0.90}.Verdict())
	assert.Equal(t, VerdictFlag, FidelityReport{Faithfulness: 0.70, Threshold: 0.50}.Verdict())
	assert.Equal(t, VerdictFlag, FidelityReport{Faithfulness: 0.99, Threshold: 0.99}.Verdict(),
		"0.99 < 1.0 → flag; deliver só para faithfulness == 1.0")
	assert.Equal(t, VerdictDeliver, FidelityReport{Faithfulness: 1.0, Threshold: 0.99}.Verdict())
}

// TestVerify_NoChunks: sem chunks → Faithfulness = -1 (fail-open) e Total 0.
func TestVerify_NoChunks(t *testing.T) {
	t.Parallel()

	rep := Verify(context.Background(), "query", "resposta qualquer", nil, VerifyOptions{})
	assert.Equal(t, -1.0, rep.Faithfulness)
	assert.Equal(t, 0, rep.Total)
	assert.Equal(t, 0, rep.Supported)
	assert.Nil(t, rep.Claims)
}

// TestVerify_NoClaims: chunks presentes mas resposta sem claims (texto vazio)
// → Faithfulness = -1 (nada a verificar).
func TestVerify_NoClaims(t *testing.T) {
	t.Parallel()

	chunks := []SourceChunk{ch("c1", "EVIDÊNCIA: O RAG reduz alucinação")}
	rep := Verify(context.Background(), "q", "", chunks, VerifyOptions{})
	assert.Equal(t, -1.0, rep.Faithfulness)
	assert.Equal(t, 0, rep.Total)
	assert.Equal(t, 0, rep.Supported)
}

// TestVerify_FullSupport: um único claim sustentado → Faithfulness 1.0 →
// deliver.
func TestVerify_FullSupport(t *testing.T) {
	t.Parallel()

	answer := "O sol e uma estrela de plasma."
	chunks := []SourceChunk{ch("c1", "O sol e uma estrela de plasma")}

	rep := Verify(context.Background(), "o que é o sol?", answer, chunks, VerifyOptions{})

	require.Equal(t, 1, rep.Total)
	assert.Equal(t, 1, rep.Supported)
	assert.InDelta(t, 1.0, rep.Faithfulness, 1e-4)
	assert.Equal(t, VerdictDeliver, rep.Verdict())
	require.Len(t, rep.Claims, 1)
	assert.True(t, rep.Claims[0].Supported)
}

// TestVerify_PartialSupport: três claims, dois sustentados → 2/3 ≈ 0.667 →
// flag (0.60 <= faithfulness < 1.0).
func TestVerify_PartialSupport(t *testing.T) {
	t.Parallel()

	answer := "O sol e uma estrela de plasma. A terra gira ao redor do sol. Marte tem dois satelites pequenos."
	chunks := []SourceChunk{ch("c1", "O sol e uma estrela de plasma. A terra gira ao redor do sol.")}

	rep := Verify(context.Background(), "q", answer, chunks, VerifyOptions{})

	require.Equal(t, 3, rep.Total)
	assert.Equal(t, 2, rep.Supported)
	assert.InDelta(t, 0.6667, rep.Faithfulness, 1e-4)
	assert.Equal(t, VerdictFlag, rep.Verdict())
	assert.Len(t, rep.Claims, 3)
	assert.True(t, rep.Claims[0].Supported)
	assert.True(t, rep.Claims[1].Supported)
	assert.False(t, rep.Claims[2].Supported)
}

// TestVerify_AllUnsupported: nenhum claim sustentado → 0.0 → block.
func TestVerify_AllUnsupported(t *testing.T) {
	t.Parallel()

	answer := "O sol e feito de queijo derretido. A lua e um queijo suico."
	chunks := []SourceChunk{ch("c1", "RAG e custo e latencia")} // irrelevante

	rep := Verify(context.Background(), "q", answer, chunks, VerifyOptions{})

	assert.Equal(t, 2, rep.Total)
	assert.Equal(t, 0, rep.Supported)
	assert.InDelta(t, 0.0, rep.Faithfulness, 1e-4)
	assert.Equal(t, VerdictBlock, rep.Verdict())
}
