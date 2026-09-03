package datasetgen

import (
	"context"
	"os"
	"testing"
)

// TestLoadGoldenSetFrozen valida que o golden set é carregado, está congelado
// e cobre os focos representativos (não só happy path).
func TestLoadGoldenSetFrozen(t *testing.T) {
	gs, err := LoadGoldenSet(DefaultGoldenSetPath())
	if err != nil {
		t.Fatalf("LoadGoldenSet: %v", err)
	}
	if !gs.Meta.Frozen {
		t.Errorf("golden set NÃO está marcado como congelado (anti-autoengano)")
	}
	if len(gs.Cases) < 6 {
		t.Errorf("golden set pequeno demais (%d cases), esperado >= 6 para ser representativo", len(gs.Cases))
	}
	// Cobertura dos focos.
	byFocus := map[Focus]bool{}
	ids := map[string]bool{}
	for _, c := range gs.Cases {
		byFocus[c.Focus] = true
		if ids[c.ID] {
			t.Errorf("ID duplicado: %s", c.ID)
		}
		ids[c.ID] = true
	}
	for _, f := range []Focus{FocusHappyPath, FocusRecovery, FocusSearchFirst} {
		if !byFocus[f] {
			t.Errorf("golden set não cobre o foco essencial %s", f)
		}
	}
	t.Logf("golden set: %d cases, focos=%v", len(gs.Cases), byFocus)
}

// TestGoldenEvaluatePromotionMetric checka a MÉTRICA de promoção com um
// cenário sintético: avaliar e confirmar que o relatório agrega pass%.
func TestGoldenEvaluatePromotionMetric(t *testing.T) {
	// Golden set pequeno com um caso já resolvido (workspace com expected
	// atingido) e outro cuja trajetória é FAILURE — para ver a métrica.
	// Usamos um Runner fake-free mas com o caso que NÃO depende de modelo
	// real? EvaluateGolden chama GenerateExample -> provider. Isso exige
	// Ollama. Então este teste foca na AGRAGAÇÃO, com casos sintéticos.
	//
	// Como EvaluateGolden depende do provider (E2E), este teste apenas
	// valida a agregação do relatório quando recebemos resultados já
	// computados (não re-roda o modelo). Isso separa a lógica de agregação
	// (determinística) da execução (E2E, opcional).
	report := buildGoldenReportFromResults([]GoldenResult{
		{CaseID: "go-happy-simple", Focus: FocusHappyPath, Label: LabelSuccess, Passed: true, PassRate: 1.0},
		{CaseID: "go-recovery-stale", Focus: FocusRecovery, Label: LabelRecoverySuccess, Passed: true, PassRate: 1.0},
		{CaseID: "py-happy-simple", Focus: FocusHappyPath, Label: LabelFailure, Passed: false, PassRate: 0.0},
	})
	if report.N != 3 {
		t.Fatalf("N = %d, esperado 3", report.N)
	}
	if report.Passed != 2 {
		t.Fatalf("passed = %d, esperado 2", report.Passed)
	}
	want := 2.0 / 3.0
	if report.AggregatePassRate < want-0.001 || report.AggregatePassRate > want+0.001 {
		t.Errorf("pass rate = %v, esperado %v", report.AggregatePassRate, want)
	}
	// A métrica de promoção: se before (modelo base) < after (LoRA), promove.
	t.Logf("métrica de promoção = AggregatePassRate (before vs after do LoRA); pass%% verificado: %.2f", report.AggregatePassRate)
}

// buildGoldenReportFromResults agrega um relatório a partir de resultados já
// computados (separado da execução do provider para testabilidade).
func buildGoldenReportFromResults(results []GoldenResult) *GoldenReport {
	report := &GoldenReport{
		Metric:     "pass%",
		N:          len(results),
		ByLabel:    map[Label]int{},
		ByFocus:    map[Focus]int{},
		Results:    results,
	}
	for _, r := range results {
		if r.Passed {
			report.Passed++
		}
		report.ByLabel[r.Label]++
		report.ByFocus[r.Focus]++
		if r.Label == LabelRecoverySuccess {
			report.AggregateRecoveryRate++
		}
	}
	if len(results) > 0 {
		report.AggregatePassRate = float64(report.Passed) / float64(len(results))
		report.AggregateRecoveryRate = report.AggregateRecoveryRate / float64(len(results))
	}
	return report
}

// TestGoldenE2E roda o golden set completo contra o Ollama (opcional). Requer
// COSCA_DATASETGEN_RUN_EVAL=1. Isto é a CAMPANHA — chame ANTES (baseline) e
// DEPOIS (LoRA) e compare AggregatePassRate.
func TestGoldenE2E(t *testing.T) {
	if os.Getenv("COSCA_DATASETGEN_RUN_EVAL") != "1" {
		t.Skip("pule (defina COSCA_DATASETGEN_RUN_EVAL=1 para rodar a campanha)"  )
	}
	cfg := DefaultGeneratorConfig()
	r := NewRunner(cfg)
	gs, err := LoadGoldenSet(DefaultGoldenSetPath())
	if err != nil {
		t.Fatalf("LoadGoldenSet: %v", err)
	}
	ctx := context.Background()
	report, err := EvaluateGolden(ctx, r, gs)
	if err != nil {
		t.Fatalf("EvaluateGolden: %v", err)
	}
	t.Logf("golden pass%% = %.2f (%d/%d) | recovery%% = %.2f",
		report.AggregatePassRate, report.Passed, report.N, report.AggregateRecoveryRate)
	for _, res := range report.Results {
		t.Logf("  %s focus=%s label=%s passed=%v", res.CaseID, res.Focus, res.Label, res.Passed)
	}
}

// TestPromotionGateCriticalNotCompensable valida a regra do professor: uma
// geração que SOBE o score mas tem violação crítica (unsafe/false_completion)
// NÃO é promovida. Nenhuma média compensa.
func TestPromotionGateCriticalNotCompensable(t *testing.T) {
	crit := DefaultPromotionCriteria()

	before := &GoldenReport{N: 12, Passed: 11, AggregatePassRate: 0.92}
	// After: subiu para 95% MAS tem 1 unsafe_mutation.
	after := &GoldenReport{
		N: 12, Passed: 11, AggregatePassRate: 0.95,
		UnsafeMutationCount: 1, CriticalViolations: 1,
		AggregateToolValidity: 1.0, AggregateRecoveryRate: 0.3,
	}

	d := CheckPromotion(before, after, crit)
	if d.Promote {
		t.Fatalf("candidato com unsafe_mutation=1 NÃO deveria ser promovido (violação crítica)")
	}
	t.Logf("reprovado corretamente: %v", d.Reasons)
}

// TestPromotionGateSuccessRegression valida que NÃO é permitido regredir.
func TestPromotionGateSuccessRegression(t *testing.T) {
	crit := DefaultPromotionCriteria()
	before := &GoldenReport{N: 12, Passed: 11, AggregatePassRate: 0.92}
	after := &GoldenReport{
		N: 12, Passed: 10, AggregatePassRate: 0.83,
		AggregateToolValidity: 1.0, AggregateRecoveryRate: 0.3,
	}
	d := CheckPromotion(before, after, crit)
	if d.Promote {
		t.Fatalf("candidato regrediu (0.92 -> 0.83) NÃO deveria ser promovido")
	}
}

// TestPromotionGateHealthyPromotes valida o caminho feliz: melhora + zero
// críticas + acima dos mínimos → promove.
func TestPromotionGateHealthyPromotes(t *testing.T) {
	crit := DefaultPromotionCriteria()
	before := &GoldenReport{N: 12, Passed: 9, AggregatePassRate: 0.75}
	after := &GoldenReport{
		N: 12, Passed: 11, AggregatePassRate: 0.92,
		AggregateToolValidity: 1.0, AggregateRecoveryRate: 0.33,
	}
	d := CheckPromotion(before, after, crit)
	if !d.Promote {
		t.Fatalf("candidato saudável deveria ser promovido: %v", d.Reasons)
	}
}
