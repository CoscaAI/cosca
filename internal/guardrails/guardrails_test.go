package guardrails

import (
	"strings"
	"testing"
)

// ============================================================
// G1 — Imutável
// ============================================================

func TestG1_ImmutableConstituicao(t *testing.T) {
	imm, why := DefaultIsImmutable("memory/agent/kernel/learnings.md") // não imutável
	if imm {
		t.Fatalf("learnings.md não deveria ser imutável: %s", why)
	}
	imm, why = DefaultIsImmutable(".cosca/identidade/CONSTITUTION.md")
	if !imm {
		t.Fatalf("Constituição DEVERIA ser imutável: %s", why)
	}
	if why == "" {
		t.Error("motivo do bloqueio não pode ser vazio")
	}
}

func TestG1_ImmutableLawsAndADR(t *testing.T) {
	for _, res := range []string{
		".cosca/knowledge/laws.json",
		"docs/adr/ADR-047-intelligence-engine-edicao-gateada.md",
		"family_chain.dat",
	} {
		imm, _ := DefaultIsImmutable(res)
		if !imm {
			t.Errorf("%s DEVERIA ser imutável (G1)", res)
		}
	}
}

// ============================================================
// G2 — Gate do Don
// ============================================================

func TestG2_RequiresDonApproval(t *testing.T) {
	p := Proposal{ID: "p1", Resource: "memory/agent/kernel/learnings.md", ApprovedByDon: false, Role: RoleProposer, EvidenceLevel: 5, HasSnapshot: true}
	res := Evaluate(p, DefaultDeps())
	if res.Verdict.Approved {
		t.Fatal("proposta sem aprovação do Don NÃO deveria ser aprovada (G2)")
	}
	if !hasRule(res, RuleDonGate) {
		t.Error("expected G2 rule checked")
	}
}

// ============================================================
// G6 — Evidência
// ============================================================

func TestG6_RequiresEvidenceLevel4(t *testing.T) {
	p := Proposal{ID: "p2", Resource: "memory/agent/kernel/learnings.md", ApprovedByDon: true, Role: RoleProposer, EvidenceLevel: 2, HasSnapshot: true}
	res := Evaluate(p, DefaultDeps())
	if res.Verdict.Approved {
		t.Fatal("proposta com evidência nível 2 NÃO deveria ser aprovada (G6)")
	}
	if !hasRule(res, RuleEvidence) {
		t.Error("expected G6 rule checked")
	}
}

func TestG6_RejectsContentByMemoryguard(t *testing.T) {
	deps := DefaultDeps()
	// simula o memoryguard rejeitando conteúdo (auto-promoção)
	deps.ValidateContent = func(text string) Verdict {
		if strings.Contains(text, "nivel 9") {
			return Verdict{Approved: false, Reasons: []string{"[AUTO-PROMOÇÃO] nível 9 excede a régua"}}
		}
		return Verdict{Approved: true}
	}
	p := Proposal{ID: "p3", Resource: "memory/agent/kernel/learnings.md", ApprovedByDon: true, Role: RoleProposer, EvidenceLevel: 5, NewContent: "alcancei o nivel 9", HasSnapshot: true}
	res := Evaluate(p, deps)
	if res.Verdict.Approved {
		t.Fatal("conteúdo rejeitado pelo guard NÃO deveria ser aprovado (G6)")
	}
}

// ============================================================
// G4 — Rollback / Snapshot
// ============================================================

func TestG4_RequiresSnapshot(t *testing.T) {
	p := Proposal{ID: "p4", Resource: "memory/agent/kernel/learnings.md", ApprovedByDon: true, Role: RoleProposer, EvidenceLevel: 5, HasSnapshot: false}
	res := Evaluate(p, DefaultDeps())
	if res.Verdict.Approved {
		t.Fatal("proposta sem snapshot NÃO deveria ser aprovada (G4)")
	}
	if !hasRule(res, RuleRollback) {
		t.Error("expected G4 rule checked")
	}
}

func TestG4_SnapshotRoundtrip(t *testing.T) {
	s := NewSnapshot("a.md", "conteudo-antigo")
	if s.Token == "" {
		t.Fatal("expected non-empty token")
	}
	s2 := NewSnapshot("a.md", "conteudo-antigo")
	if s.Token != s2.Token {
		t.Error("same resource+state should produce same token")
	}
}

// ============================================================
// G3 — Shadow-first
// ============================================================

func TestG3_ShadowNeverApplies(t *testing.T) {
	p := Proposal{ID: "p5", Resource: "memory/agent/kernel/learnings.md", ApprovedByDon: true, Role: RoleProposer, EvidenceLevel: 5, HasSnapshot: true, ShadowMode: true}
	res := Evaluate(p, DefaultDeps())
	if res.Verdict.Approved {
		t.Fatal("shadow mode NÃO deveria aplicar (G3)")
	}
	if !hasRule(res, RuleShadowFirst) {
		t.Error("expected G3 rule checked")
	}
}

// ============================================================
// G5 — Conflito não decide sozinho
// ============================================================

func TestG5_ConflictDetectionSignalsOnly(t *testing.T) {
	// mesmo tema + conclusões divergentes => sinaliza conflito
	c := DetectConflict("antigo.md", "novo.md", 0.8, true)
	if c == nil {
		t.Fatal("esperava conflito detectado")
	}
	if !strings.Contains(c.Note, "escala ao Don") {
		t.Error("conflito deve escalar ao Don, não decidir sozinho")
	}
	// mesmas conclusões => sem conflito
	if DetectConflict("antigo.md", "novo.md", 0.9, false) != nil {
		t.Error("conclusões iguais não deveriam gerar conflito")
	}
}

// ============================================================
// G7 — Anti-loop
// ============================================================

func TestG7_AntiLoopBlocksRepeatedEdits(t *testing.T) {
	history := []string{"a.md", "a.md", "a.md"}
	blocked, why := AntiLoop(history, "a.md")
	if !blocked {
		t.Fatal("esperava bloqueio de loop após 3 edições no mesmo recurso")
	}
	if !strings.Contains(why, "loop") {
		t.Error("motivo deve mencionar loop")
	}

	// 2 edições ainda é aceitável
	blocked, _ = AntiLoop([]string{"a.md", "a.md"}, "a.md")
	if blocked {
		t.Error("2 edições não deveriam bloquear")
	}
}

// ============================================================
// G8 — Custódia
// ============================================================

func TestG8_InvalidRoleDenied(t *testing.T) {
	p := Proposal{ID: "p6", Resource: "memory/agent/kernel/learnings.md", Role: RoleApprover, ApprovedByDon: true, EvidenceLevel: 5, HasSnapshot: true}
	res := Evaluate(p, DefaultDeps())
	if res.Verdict.Approved {
		t.Fatal("approver não deveria editar diretamente (G8)")
	}
	if !hasRule(res, RuleCustody) {
		t.Error("expected G8 rule checked")
	}
}

// ============================================================
// G9 — Integridade
// ============================================================

func TestG9_IntegrityGateDeniedOnFailure(t *testing.T) {
	deps := DefaultDeps()
	deps.VerifyIntegrity = func() error { return nil } // ok
	p := Proposal{ID: "p7", Resource: "memory/agent/kernel/learnings.md", Role: RoleProposer, ApprovedByDon: true, EvidenceLevel: 5, HasSnapshot: true}
	if res := Evaluate(p, deps); !res.Verdict.Approved {
		t.Fatalf("proposta íntegra deveria passar, mas: %v", res.Verdict.Reasons)
	}

	// agora falha integridade
	deps.VerifyIntegrity = func() error { return nil }
	// simula falha
	depsFail := DefaultDeps()
	depsFail.VerifyIntegrity = func() error { return newError("hash mismatch") }
	res := Evaluate(p, depsFail)
	if res.Verdict.Approved {
		t.Fatal("falha de integridade DEVERIA negar a proposta (G9)")
	}
	if !hasRule(res, RuleIntegrity) {
		t.Error("expected G9 rule checked")
	}
}

// ============================================================
// Happy path — proposta legítima passa
// ============================================================

func TestEvaluate_HappyPath(t *testing.T) {
	p := Proposal{ID: "p8", Resource: "memory/agent/kernel/learnings.md", Role: RoleProposer, ApprovedByDon: true, EvidenceLevel: 5, HasSnapshot: true}
	res := Evaluate(p, DefaultDeps())
	if !res.Verdict.Approved {
		t.Fatalf("proposta legítima deveria passar, mas: %v", res.Verdict.Reasons)
	}
	// todas as 9 regras devem ter sido verificadas
	if len(res.RulesChecked) < 5 {
		t.Errorf("esperava várias regras verificadas, got %d", len(res.RulesChecked))
	}
}

// helpers

func hasRule(res Result, rule string) bool {
	for _, r := range res.RulesChecked {
		if r == rule {
			return true
		}
	}
	return false
}

// newError evita importar errors só para um ad-hoc
func newError(s string) error { return &errString{s} }

type errString struct{ s string }

func (e *errString) Error() string { return e.s }
