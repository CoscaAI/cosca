package evals

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/evalgo"
)

func TestFailureSignature_Granular(t *testing.T) {
	// Falha de verificação → assinatura granular (qual passo falhou).
	c := CaseResult{ID: "c1", Status: StatusFailed, VerifyResults: []VerifyResult{
		{Command: "test -f out.txt", OK: false},
	}}
	if s := failureSignature(c); s != "verify:test -f out.txt" {
		t.Fatalf("assinatura inesperada: %q", s)
	}
}

func TestFailureSignature_Infra(t *testing.T) {
	if s := failureSignature(CaseResult{ID: "e", Status: StatusError}); s != sigError {
		t.Fatalf("erro deveria ser %q, got %q", sigError, s)
	}
	if s := failureSignature(CaseResult{ID: "t", Status: StatusTimeout}); s != sigTimeout {
		t.Fatalf("timeout deveria ser %q, got %q", sigTimeout, s)
	}
}

func TestFailureSignature_FailedNoVerify(t *testing.T) {
	if s := failureSignature(CaseResult{ID: "f", Status: StatusFailed}); s != sigFailed {
		t.Fatalf("falha sem verify deveria ser %q, got %q", sigFailed, s)
	}
}

func TestPromoteGate_BlocksInfra(t *testing.T) {
	report := &Report{
		Suite: "s",
		Cases: []CaseResult{
			{ID: "a", Status: StatusPassed},
			{ID: "b", Status: StatusPassed},
			{ID: "c", Status: StatusPassed},
			{ID: "d", Status: StatusError}, // infra quebrou → bloqueia
		},
	}
	p := PromoteGate(report, DefaultPromoteGate())
	if p.Decision.Promoted {
		t.Fatal("erro de infra (status:error) nao deveria promover (fail-closed I2)")
	}
	if len(p.Clusters) != 1 || p.Clusters[0].Signature != sigError {
		t.Fatalf("cluster de infra inesperado: %+v", p.Clusters)
	}
}

func TestPromoteGate_Promotes(t *testing.T) {
	report := &Report{
		Suite: "s",
		Cases: []CaseResult{
			{ID: "a", Status: StatusPassed},
			{ID: "b", Status: StatusPassed},
			{ID: "c", Status: StatusPassed},
			{ID: "d", Status: StatusPassed},
		},
	}
	p := PromoteGate(report, DefaultPromoteGate())
	if !p.Decision.Promoted {
		t.Fatalf("4/4 deveria promover: %s", p.Decision.Summary)
	}
	if p.PassRate != 1.0 {
		t.Fatalf("pass rate inesperado: %f", p.PassRate)
	}
}

func TestPromoteGate_BlocksInsufficientCases(t *testing.T) {
	report := &Report{
		Suite: "s",
		Cases: []CaseResult{{ID: "a", Status: StatusPassed}},
	}
	if PromoteGate(report, DefaultPromoteGate()).Decision.Promoted {
		t.Fatal("casos insuficientes (<3) nao deveria promover")
	}
}

func TestPromoteGate_BlocksVerifySignature(t *testing.T) {
	// Mesmo com taxa alta, uma assinatura BLOQUEANTE extra (ex.: vazamento)
	// impede a promoção — fail-closed I8.
	gate := evalgo.NewGate(evalgo.WithBlockingSignatures("leaked:"))
	report := &Report{
		Suite: "s",
		Cases: []CaseResult{
			{ID: "a", Status: StatusPassed},
			{ID: "b", Status: StatusPassed},
			{ID: "c", Status: StatusPassed},
			{ID: "d", Status: StatusFailed, VerifyResults: []VerifyResult{{Command: "leaked:KEY", OK: false}}},
		},
	}
	p := PromoteGate(report, gate)
	if p.Decision.Promoted {
		t.Fatal("assinatura bloqueante nao deveria permitir promoção")
	}
}
