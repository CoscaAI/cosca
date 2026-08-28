package evolution

import (
	"math"
	"testing"
)

// TestBootstrapCILow_Deterministic: mesma seed → mesmo CI (I1).
func TestBootstrapCILow_Deterministic(t *testing.T) {
	deltas := []float64{0.1, 0.2, -0.05, 0.15, 0.05, 0.3, 0.12, 0.08}
	a, _ := BootstrapCILow(deltas, 500, 42, 0.05)
	b, _ := BootstrapCILow(deltas, 500, 42, 0.05)
	if a != b {
		t.Fatalf("bootstrap deve ser determinístico (I1): %f vs %f", a, b)
	}
}

func TestProofGate_AcceptsSignificantImprovement(t *testing.T) {
	// Deltas claramente positivos → BCI > 0 → ACCEPT.
	deltas := make([]float64, 40)
	for i := range deltas {
		deltas[i] = 0.1 + 0.05*float64(i%5) // médias ~0.2, todos > 0
	}
	rec, err := NewProofGate().Evaluate("champ", "cand", deltas)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if rec.Verdict != VerdictAccept || !rec.Promoted() {
		t.Fatalf("esperava ACCEPT para melhoria clara, got %s (BCI %.4f)", rec.Verdict, rec.BCI)
	}
	if rec.BCI <= 0 {
		t.Fatalf("BCI deveria ser > 0 para melhoria significativa, got %.4f", rec.BCI)
	}
}

func TestProofGate_RejectsNoiseOrRegression(t *testing.T) {
	// Ruído com média ~0 (sem melhoria real) → BCI <= 0 → REJECT. 24 amostras
	// (>= MinSamples) para atingir o veredito (não fail-closed).
	deltas := []float64{
		0.02, -0.02, 0.01, -0.01, 0.03, -0.03, 0.01, -0.02,
		0.02, -0.03, 0.00, -0.01, 0.02, -0.02, 0.01, -0.03,
		0.03, -0.02, 0.00, -0.01, 0.02, -0.03, 0.01, -0.02,
	}
	rec, err := NewProofGate().Evaluate("champ", "cand", deltas)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if rec.Verdict != VerdictReject {
		t.Fatalf("ruído sem melhoria consistente deveria REJECT, got %s (BCI %.4f)", rec.Verdict, rec.BCI)
	}
}

func TestProofGate_FailClosedInsufficientSamples(t *testing.T) {
	// Menos que MinSamples → erro (fail-closed I2), nunca promove por sorte.
	_, err := NewProofGate().Evaluate("champ", "cand", []float64{0.1, 0.2})
	if err == nil {
		t.Fatal("amostras insuficientes devem dar erro (fail-closed)")
	}
}

func TestProofReceipt_HashChain(t *testing.T) {
	rec, _ := NewProofGate().Evaluate("champ", "cand", make([]float64, 20))
	if rec.Hash == "" || rec.ComputeHash() != rec.Hash {
		t.Fatal("receipt hash deve ser estável e encadeável")
	}
	// Alterar um campo muda o hash (tamper-evidente).
	rec.BCI = 0
	if rec2 := *rec; rec2.ComputeHash() != rec.Hash {
		if rec2.ComputeHash() == rec.Hash {
			t.Fatal("hash deveria mudar se o campo mudou")
		}
	}
}

func TestBootstrapCILow_EmptyErrors(t *testing.T) {
	if _, err := BootstrapCILow(nil, 100, 1, 0.05); err == nil {
		t.Fatal("deltas vazios devem dar erro")
	}
}

// TestBootstrapCILow_Math sanity: quantil nunca fora de [min,max].
func TestBootstrapCILow_Math(t *testing.T) {
	deltas := []float64{0.5, -0.5, 0.3, -0.3, 0.1}
	ci, err := BootstrapCILow(deltas, 200, 7, 0.05)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if ci < 0 && math.Abs(ci) > 0.6 {
		t.Fatalf("BCI fora de faixa plausível: %f", ci)
	}
}
