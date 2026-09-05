package evolution

import "testing"

func TestConformal_ThresholdAndPValue(t *testing.T) {
	cal := []float64{0.5, 0.6, 0.7, 0.8, 0.9, 0.95, 0.97, 0.99, 1.0, 0.85}
	// p-value de um score alto deve ser pequena (raro = anômalo).
	if p := ConformalPValue(cal, 1.0); p >= 0.5 {
		t.Fatalf("score máximo deve ter p-value baixa, got %f", p)
	}
	// p-value de um score mediano deve ser ~alta (comum = coberto).
	mid := ConformalPValue(cal, 0.6)
	if mid <= 0 {
		t.Fatalf("score mediano deve ter p-value não-nula, got %f", mid)
	}
	// Threshold conformal: deve estar dentro do range da calibração.
	th := ConformalThreshold(cal, 0.05)
	if th < cal[0] || th > cal[len(cal)-1] {
		t.Fatalf("threshold fora do range: %f", th)
	}
	// Covers: score alto cobre (p > alpha); score baixo pode ser anômalo.
	if !Covers(cal, 0.99, 0.05) {
		t.Fatal("score alto deve ser coberto")
	}
}

func TestBelief_AcceptProofGated(t *testing.T) {
	l := NewBeliefLedger()
	_ = l.Add(Belief{ID: "base", Statement: "teorema base"})
	_ = l.Add(Belief{ID: "deriv", Statement: "derivado", Dependencies: []string{"base"}})

	// Derivada não pode ser aceita se "base" não está aceita (fail-closed I2).
	if err := l.Accept("deriv"); err == nil {
		t.Fatal("aceitar dependente sem dependência aceita deve falhar (I2)")
	}
	// Aceita base → derivada pode ser aceita.
	if err := l.Accept("base"); err != nil {
		t.Fatalf("aceitar base: %v", err)
	}
	if err := l.Accept("deriv"); err != nil {
		t.Fatalf("aceitar derivada: %v", err)
	}
	if st, _ := l.State("deriv"); st != BeliefAccepted {
		t.Fatalf("derivada deveria estar accepted, got %s", st)
	}
}

func TestBelief_ContainmentTransitive(t *testing.T) {
	l := NewBeliefLedger()
	_ = l.Add(Belief{ID: "a", Statement: "a"})
	_ = l.Add(Belief{ID: "b", Statement: "b", Dependencies: []string{"a"}})
	_ = l.Add(Belief{ID: "c", Statement: "c", Dependencies: []string{"b"}})
	for _, id := range []string{"a", "b", "c"} {
		if err := l.Accept(id); err != nil {
			t.Fatalf("accept %s: %v", id, err)
		}
	}
	// Rejeita "a" → containment transitivo: "b" e "c" rebaixam a Pending.
	if err := l.Reject("a"); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if st, _ := l.State("a"); st != BeliefRejected {
		t.Fatalf("a deveria rejected, got %s", st)
	}
	if st, _ := l.State("b"); st != BeliefPending {
		t.Fatalf("b deveria rebaixar a pending (containment), got %s", st)
	}
	if st, _ := l.State("c"); st != BeliefPending {
		t.Fatalf("c deveria rebaixar a pending (containment transitivo), got %s", st)
	}
}

func TestBelief_FailClosed(t *testing.T) {
	l := NewBeliefLedger()
	_ = l.Add(Belief{ID: "x"})
	if err := l.Accept("nao-existe"); err == nil {
		t.Fatal("aceitar crença inexistente deve falhar")
	}
	if _, ok := l.State("nao-existe"); ok {
		t.Fatal("estado de crença inexistente não deve existir")
	}
}

func TestConformal_Deterministic(t *testing.T) {
	cal := []float64{0.4, 0.5, 0.6, 0.7}
	if ConformalThreshold(cal, 0.05) != ConformalThreshold(cal, 0.05) {
		t.Fatal("conformal deve ser determinístico (I1)")
	}
}
