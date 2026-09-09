package cognitive

import "testing"

// ============================================================
// QUALIDADE DE DECISÃO (B3/L13)
// ============================================================

func TestMeasureDecisionQuality_HighRevert(t *testing.T) {
	// Cenário tipo L13: decisão com bypass_governance revertida
	records := []DecisionRecord{
		{ID: "d1", Decision: "executar destrutivo sem DRY_RUN", Outcome: "reverted", Category: CatBypassGovernance},
		{ID: "d2", Decision: "refatorar modulo", Outcome: "success"},
		{ID: "d3", Decision: "ajustar query", Outcome: "success"},
	}
	// só 1 categoria protegida (bypass_governance) de 5
	prot := map[string]bool{CatBypassGovernance: true}
	q := MeasureDecisionQuality(records, prot)

	if q.Decisions != 3 {
		t.Fatalf("expected 3 decisions, got %d", q.Decisions)
	}
	if q.Reverted != 1 {
		t.Errorf("expected 1 reverted, got %d", q.Reverted)
	}
	if q.SuccessRate != 2.0/3.0 {
		t.Errorf("expected success rate 0.667, got %.2f", q.SuccessRate)
	}
	if q.ByCategory[CatBypassGovernance] != 1 {
		t.Errorf("expected 1 bypass_governance, got %d", q.ByCategory[CatBypassGovernance])
	}
	// cobertura: 1/5 = 20% (a métrica exata do L13)
	if q.ProtectionCoverage != 0.2 {
		t.Errorf("expected coverage 0.2, got %.2f", q.ProtectionCoverage)
	}
	// bypass_governance tem regra -> nao vulneravel
	if vul := q.VulnerableCategories(prot); len(vul) != 0 {
		t.Errorf("expected no vulnerable (bypass protegido), got %v", vul)
	}
}

func TestMeasureDecisionQuality_NoReverts(t *testing.T) {
	records := []DecisionRecord{}
	prot := map[string]bool{}
	q := MeasureDecisionQuality(records, prot)
	if q.Decisions != 0 {
		t.Fatalf("expected 0, got %d", q.Decisions)
	}
	if q.SuccessRate != 0 {
		t.Errorf("expected 0 success rate (sem dados), got %.2f", q.SuccessRate)
	}
}

// ============================================================
// HORIZONTE (profundidade preditiva)
// ============================================================

func TestPlanDepth(t *testing.T) {
	if PlanDepth(1) != 1 {
		t.Error("1 passo = miope")
	}
	if PlanDepth(3) != 2 {
		t.Error("3 passos = intermediario")
	}
	if PlanDepth(10) != 3 {
		t.Error("10 passos = profundo")
	}
	if PlanDepth(20) != 4 {
		t.Error("20 passos = visionario")
	}
}

func TestHorizonLevel(t *testing.T) {
	if HorizonLevel(1) != "míope (0-2) — resolve sintoma, cria problemas downstream" {
		t.Errorf("unexpected horizon level for miope")
	}
	if HorizonLevel(4) == "" {
		t.Error("expected non-empty for visionario")
	}
}
