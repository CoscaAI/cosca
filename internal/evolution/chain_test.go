package evolution

import "testing"

// buildSoundChain monta uma cadeia SOUND (source→obs→inf→decision, tudo ok).
func buildSoundChain() ProvenanceChain {
	var c ProvenanceChain
	c.Add(Link{Label: "source", Trust: TrustSound, Epistemic: "MEASURED", ProvenanceRef: "sensor:rf-1", Confidence: 0.9})
	c.Add(Link{Label: "observation", Trust: TrustSound, Epistemic: "MEASURED", ProvenanceRef: "spot:1"})
	c.Add(Link{Label: "inference", Trust: TrustSuspicious, Epistemic: "INFERRED", Confidence: 0.7})
	c.Add(Link{Label: "decision", Trust: TrustSuspicious})
	return c
}

func TestChain_Propagation_NeverTruthByPropagation(t *testing.T) {
	// ⚠️ Source → ⚠️ Observation → ⚠️ Inference → 🚫 Decision blocked.
	var c ProvenanceChain
	c.Add(Link{Label: "source", Trust: TrustPoisoned, Epistemic: "MEASURED", ProvenanceRef: "compromise:d"})
	c.Add(Link{Label: "observation", Trust: TrustSound}) // parece limpo, MAS...
	c.Add(Link{Label: "inference", Trust: TrustSound})
	c.Add(Link{Label: "decision", Trust: TrustSound})

	// ...não vira verdade: a propagação marcou tudo como Poisoned, e a decisão
	// é bloqueada. Este é o ponto do professor.
	if !c.Contaminated() {
		t.Fatal("cadeia com fonte contaminada deve estar contaminada (propagação)")
	}
	if !c.DecisionBlocked() {
		t.Fatal("decisão deve ser bloqueada (fail-closed I2)")
	}
	if c.Links[1].Trust != TrustPoisoned || c.Links[3].Trust != TrustPoisoned {
		t.Fatalf("propagação deveria marcar observation/inference/decision: %+v", c.Links)
	}
}

func TestChain_Sound_NotBlocked(t *testing.T) {
	c := buildSoundChain()
	if c.Contaminated() || c.DecisionBlocked() {
		t.Fatalf("cadeia sound não deve bloquear: %+v", c.Links)
	}
	if c.MaxTrust() != TrustSuspicious {
		t.Fatalf("MaxTrust deveria ser Suspicious (inf/decision inferidos): %v", c.MaxTrust())
	}
}

func TestChain_ToContentTrustTrust(t *testing.T) {
	if TrustSound.ToContentTrustTrust() != "trusted" {
		t.Fatalf("sound deveria mapear para trusted (I8)")
	}
	if TrustPoisoned.ToContentTrustTrust() != "untrusted" {
		t.Fatalf("poisoned deveria mapear para untrusted (I8)")
	}
}

func TestBelief_AcceptSound_BlockedByContaminatedChain(t *testing.T) {
	l := NewBeliefLedger()
	_ = l.Add(Belief{ID: "x", Statement: "derivado de fonte comprometida"})

	// Cadeia contaminada → fail-closed: NUNCA aceita (professor: contaminação
	// não vira verdade).
	bad := ProvenanceChain{}
	bad.Add(Link{Label: "source", Trust: TrustPoisoned})
	bad.Add(Link{Label: "observation", Trust: TrustSound})
	bad.Add(Link{Label: "inference", Trust: TrustSound})
	if err := l.AcceptSound("x", bad); err == nil {
		t.Fatal("aceitar crença de cadeia contaminada deve falhar (fail-closed)")
	}
	if st, _ := l.State("x"); st != BeliefPending {
		t.Fatalf("crença contaminada deve permanecer pending, got %s", st)
	}

	// Cadeia sound → aceita.
	if err := l.AcceptSound("x", buildSoundChain()); err != nil {
		t.Fatalf("cadeia sound deve aceitar: %v", err)
	}
}

func TestChain_AddPropagation_NewLinkInherits(t *testing.T) {
	var c ProvenanceChain
	c.Add(Link{Label: "source", Trust: TrustPoisoned})
	c.Add(Link{Label: "inference", Trust: TrustSound})
	if c.Links[1].Trust != TrustPoisoned {
		t.Fatal("novo link deve herdar a contaminação (propagação desce)")
	}
}
