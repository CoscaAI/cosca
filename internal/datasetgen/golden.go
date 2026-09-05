package datasetgen

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"
)

// ─── GOLDEN SET CONGELADO (anti-autoengano) ─────────────────────────────────
//
// O golden set é o "crachá" do treinador de modelos. Regras (professor):
//
//  1. CONGELADO: versionado em `golden/golden.json`. Nunca muda no meio de uma
//     campanha de treino. Se mudar, você não sabe se o modelo melhorou por
//     causa do LoRA ou por causa do golden.
//
//  2. MÉTRICA OBJETIVA: a passagem de cada exemplo é decidida pelo RUNTIME
//     (posterior) — não pela opinião do modelo nem do professor. O que conta é
//     o estado final do workspace vs o estado esperado + se houve trabalho
//     agêntico real (evidência).
//
//  3. REPRESENTATIVO: cobre happy_path, recovery, request_info, search_first
//     e multi_file — não só casos felizes.
//
//  4. REGRA DE PROMOÇÃO: um LoRA só é promovido à produção se `pass%` no
//     golden set ANTES < DEPOIS (ganho real), sobre um CONJUNTO FIXO e em
//     condições idênticas (mesmas tools, mesmo prompt, mesmo runtime).

// GoldenCase é uma tarefa do golden set com resultado esperado conhecido.
//
// GOLDEN v2 (correção de contrato — descoberta epistemológica 2026-09-04):
// o antigo ExpectedState usava substring LITERAL (strings.Contains), o que
// produzia FALSOS NEGATIVOS: o modelo podia fazer uma edição semanticamente
// válida ("add proper state handling" → `state = "restarted"`) mas era
// reprovado por não conter a string exata ("return True").
//
// O NOVO contrato verifica a INTENÇÃO da tarefa, aceitando MÚLTIPLAS
// implementações válidas. Um caso passa se o estado final satisfaz uma das
// formas aceitas (StateOptions), avaliadas semanticamente.
type GoldenCase struct {
	// ID identifica a tarefa de forma estável (o mesmo ID NUNCA muda de
	// significado entre campanhas).
	ID string `json:"id"`
	// Task é a instrução ao agente.
	Task string `json:"task"`
	// InitialState é o estado inicial do workspace (path → conteúdo).
	InitialState map[string]string `json:"initial_state"`
	// ExpectedState é o estado esperado tradicional (path → substring).
	// MANTIDO para retrocompatibilidade e como referência do resultado ideal.
	// Um caso passa se ExpectedState OU qualquer StateOptions for satisfeito.
	// Vazio → só exige trabalho agêntico.
	ExpectedState map[string]string `json:"expected_state"`
	// StateOptions permite especificar MÚLTIPLAS formas semanticamente válidas
	// do estado final. Cada entrada: path → lista de fragmentos aceitáveis
	// (o caso passa se QUALQUER um deles estiver presente). Isso resolve o
	// falso negativo do contrato v1 (string literal).
	// Ex: { "main.py": ["state = 'restarted'", "return True", "# added state handling"] }
	StateOptions map[string][]string `json:"state_options,omitempty"`
	// Language é a linguagem/idioma do artefato.
	Language string `json:"language"`
	// Focus é a categoria de comportamento coberta.
	Focus Focus `json:"focus"`
}

// GoldenSet é a coleção CONGELADA de casos de avaliação.
type GoldenSet struct {
	// Meta descreve o golden set (versão, data). NUNCA alterar os casos, só a
	// meta de documentação.
	Meta GoldenMeta `json:"meta"`
	// Cases são as tarefas do conjunto.
	Cases []GoldenCase `json:"cases"`
}

// GoldenMeta é a metadados do golden set (não comportamento).
type GoldenMeta struct {
	Version     string `json:"version"`
	Description string `json:"description"`
	Date        string `json:"date"`
	// Frozen indica que o conjunto está congelado (não alterar os cases).
	Frozen bool `json:"frozen"`
}

// ─── Avaliação (Golden Gate multicritério) ─────────────────────────────────
//
// O "crachá" do treinador NÃO é só pass% — é um GOLDEN GATE em camadas
// (professor, decisão arquitetural). Nenhuma média pode COMPENSAR uma
// violação crítica: uma geração que sobe o score mas edita arquivo sem ler
// (unsafe mutation) é REPROVADA, mesmo com 95% success.
//
// Camadas:
//  0. Task Success      — atingiu o estado esperado
//  1. Tool-call válido  — chamou as tools certas, sem tool inexistente
//  2. Read → Edit correto— NUNCA editou sem haver lido antes (invariante)
//  3. Recovery após erro — recuperou de um erro (reflexo)
//  4. Test/Build + evidência — produziu evidência de verificação
//  5. False Completion == 0 — não declarou sucesso sem evidência
//  6. Unsafe Mutation == 0 — não editou sem ler / old_string inventado
//
// REGRA CRÍTICA: camadas 5 e 6 são IRRECOMPENSÁVEIS. Se qualquer caso violar,
// a geração NÃO é promovida, independente do score agregado.
type GoldenResult struct {
	CaseID   string `json:"case_id"`
	Focus    Focus  `json:"focus"`
	Label    Label  `json:"label"`
	Passed   bool   `json:"passed"`    // camada 0: atingiu expected (via runtime)
	PassRate float64 `json:"pass_rate"` // 1.0/0.0 por caso
	ToolCallsValid   bool `json:"tool_calls_valid"`   // camada 1
	ReadEditCorrect  bool `json:"read_edit_correct"`  // camada 2 (invariante)
	Recovered        bool `json:"recovered"`          // camada 3
	TestEvidence     bool `json:"test_evidence"`      // camada 4
	FalseCompletion  bool `json:"false_completion"`   // camada 5 (violação crítica)
	UnsafeMutation   bool `json:"unsafe_mutation"`    // camada 6 (violação crítica)
}

// GoldenReport é o relatório agregado da campanha (before vs after).
type GoldenReport struct {
	Metric       string          `json:"metric"`        // ex.: "golden_gate"
	GeneratedAt  string          `json:"generated_at"`
	N            int             `json:"n"`
	Passed       int             `json:"passed"`
	AggregatePassRate float64     `json:"aggregate_pass_rate"`
	AggregateRecoveryRate float64 `json:"aggregate_recovery_rate"`
	AggregateToolValidity float64 `json:"aggregate_tool_validity"`
	AggregateReadEdit float64     `json:"aggregate_read_edit"`
	AggregateTestEvidence float64 `json:"aggregate_test_evidence"`
	// Violações críticas (camadas 5/6) — IRRECOMPENSÁVEIS.
	FalseCompletionCount int `json:"false_completion_count"`
	UnsafeMutationCount  int `json:"unsafe_mutation_count"`
	CriticalViolations int `json:"critical_violations"`
	ByLabel      map[Label]int   `json:"by_label"`
	ByFocus      map[Focus]int   `json:"by_focus"`
	Results      []GoldenResult  `json:"results"`
	Tags         []string        `json:"tags,omitempty"`
}

// ─── Carregamento do golden set ─────────────────────────────────────────────

// DefaultGoldenSetPath retorna o caminho padrão do golden set versionado.
// Resolve a partir do diretório DESTE arquivo (datasetgen), não do CWD —
// assim funciona tanto em `go test` (que roda no dir do pacote) quanto em
// execução a partir da raiz do módulo.
func DefaultGoldenSetPath() string {
	_, thisFile, _, _ := runtime.Caller(0)
	pkgDir := filepath.Dir(thisFile)
	return filepath.Join(pkgDir, "golden", "golden.json")
}

// DefaultGoldenSetPathV2 retorna o caminho do GOLDEN v2 (contrato calibrado)
// — a régua CORRETA para campanhas. O v1 (golden.json) permanece como
// histórico (descoberta de falha de contrato 2026-09-04).
func DefaultGoldenSetPathV2() string {
	_, thisFile, _, _ := runtime.Caller(0)
	pkgDir := filepath.Dir(thisFile)
	return filepath.Join(pkgDir, "golden", "golden_v2.json")
}

// LoadGoldenSet carrega o golden set CONGELADO do disco.
func LoadGoldenSet(path string) (*GoldenSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("golden set: %w", err)
	}
	var gs GoldenSet
	if err := json.Unmarshal(data, &gs); err != nil {
		return nil, fmt.Errorf("golden set: parse %w", err)
	}
	if len(gs.Cases) == 0 {
		return nil, fmt.Errorf("golden set: conjunto vazio (sem casos de avaliação)")
	}
	return &gs, nil
}

// ─── Avaliação de um caso ───────────────────────────────────────────────────

// evaluateCase roda UM caso do golden set e devolve o resultado com as
// camadas do gate preenchidas. A passagem (camada 0) e os violações críticas
// são decididos pelo RUNTIME (trajetória + estado final), nunca pela opinião
// do modelo/professor.
func evaluateCase(ctx context.Context, r *Runner, c GoldenCase) (*GoldenResult, error) {
	spec := c.toTaskSpec()
	ex, err := r.GenerateExample(ctx, spec)
	if err != nil {
		return &GoldenResult{CaseID: c.ID, Focus: c.Focus, Label: LabelFailure}, err
	}

	res := &GoldenResult{
		CaseID:   c.ID,
		Focus:    c.Focus,
		Label:    ex.Label,
		Passed:   ex.Label.IsPositive(),
		ToolCallsValid: len(ex.ToolNames) > 0,
		ReadEditCorrect: ex.readsBeforeEdit(),
		Recovered:       ex.Label == LabelRecoverySuccess,
		TestEvidence:    ex.hasTestEvidence(),
		FalseCompletion: ex.Label == LabelFalseCompletion,
		UnsafeMutation:  ex.hasUnsafeMutation(),
	}
	if res.Passed {
		res.PassRate = 1.0
	}
	return res, nil
}

// toTaskSpec converte um GoldenCase em TaskSpec (para o runner).
func (c GoldenCase) toTaskSpec() TaskSpec {
	return TaskSpec{
		Task:          c.Task,
		InitialState:  c.InitialState,
		ExpectedState: c.ExpectedState,
		StateOptions:  c.StateOptions,
		Language:      c.Language,
		Focus:         c.Focus,
	}
}

// ─── Campanha (before vs after) ─────────────────────────────────────────────

// EvaluateGolden roda o golden set inteiro e agrega o relatório do gate.
// É a régua de promoção: chame ANTES do LoRA (baseline) e DEPOIS do LoRA
// (candidato) e compare via PromotionGate.
func EvaluateGolden(ctx context.Context, r *Runner, gs *GoldenSet) (*GoldenReport, error) {
	report := &GoldenReport{
		Metric:      "golden_gate",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		N:           len(gs.Cases),
		ByLabel:     map[Label]int{},
		ByFocus:     map[Focus]int{},
	}

	var results []GoldenResult
	for _, c := range gs.Cases {
		res, err := evaluateCase(ctx, r, c)
		if err != nil {
			// Falha de infra: conta como fail do caso, não derruba o golden.
			logWarnf("golden: caso %s erro de execução: %v", c.ID, err)
			res = &GoldenResult{CaseID: c.ID, Focus: c.Focus, Label: LabelFailure}
		}
		results = append(results, *res)

		if res.Passed {
			report.Passed++
		}
		if res.ToolCallsValid {
			report.AggregateToolValidity++
		}
		if res.ReadEditCorrect {
			report.AggregateReadEdit++
		}
		if res.Recovered {
			report.AggregateRecoveryRate++
		}
		if res.TestEvidence {
			report.AggregateTestEvidence++
		}
		if res.FalseCompletion {
			report.FalseCompletionCount++
		}
		if res.UnsafeMutation {
			report.UnsafeMutationCount++
		}
		if res.FalseCompletion || res.UnsafeMutation {
			report.CriticalViolations++
		}
		report.ByLabel[res.Label]++
		report.ByFocus[res.Focus]++
	}

	report.Results = results
	if report.N > 0 {
		f := float64(report.N)
		report.AggregatePassRate = float64(report.Passed) / f
		report.AggregateRecoveryRate = report.AggregateRecoveryRate / f
		report.AggregateToolValidity = report.AggregateToolValidity / f
		report.AggregateReadEdit = report.AggregateReadEdit / f
		report.AggregateTestEvidence = report.AggregateTestEvidence / f
	}
	// Ordena por case_id para relatório determinístico.
	sort.Slice(report.Results, func(i, j int) bool { return report.Results[i].CaseID < report.Results[j].CaseID })
	return report, nil
}

// ─── Promotion Gate (regra versionada, irrecompensável) ─────────────────────

// PromotionCriteria é a regra de promoção VERSIONADA. Congele-a por campanha:
// alterar os critérios no meio muda a régua — viola o anti-autoengano.
type PromotionCriteria struct {
	// SuccessGate: success_after >= success_before (não dá para regredir).
	RequireSuccessImprovement bool `json:"require_success_improvement"`
	// CriticalZero: violations críticas DEVEM ser 0.
	RequireZeroCriticalViolations bool `json:"require_zero_critical"`
	// ToolValidityMin: taxa mínima de tool-call válido (camada 1).
	ToolValidityMin float64 `json:"tool_validity_min"`
	// RecoveryMin: taxa mínima de recovery (camada 3).
	RecoveryMin float64 `json:"recovery_min"`
}

// DefaultPromotionCriteria retorna a régua de promoção padrão.
func DefaultPromotionCriteria() PromotionCriteria {
	return PromotionCriteria{
		RequireSuccessImprovement: true,
		RequireZeroCriticalViolations: true,
		ToolValidityMin: 0.8,
		RecoveryMin:     0.2,
	}
}

// PromotionDecision é o veredito do gate.
type PromotionDecision struct {
	Promote     bool     `json:"promote"`
	Reasons     []string `json:"reasons"`   // motivos de REPROVAÇÃO (vazio = promove)
	Observations []string `json:"observations,omitempty"` // notas informativas
}

// CheckPromotion aplica PromotionCriteria sobre before (baseline) e after
// (candidato). NENHUMA média compensa uma violação crítica:
//   - se after tem qualquer UnsafeMutation/FalseCompletion e
//     RequireZeroCriticalViolations, REPROVA imediatamente.
//   - success_after < success_before → REPROVA (regrediu).
//   - tool_validity/recovery abaixo do mínimo → REPROVA.
func CheckPromotion(before, after *GoldenReport, crit PromotionCriteria) *PromotionDecision {
	d := &PromotionDecision{Promote: true}

	// 1. Violação crítica: IRRECOMPENSÁVEL.
	if crit.RequireZeroCriticalViolations {
		if after.UnsafeMutationCount > 0 {
			d.Reasons = append(d.Reasons, fmt.Sprintf("unsafe_mutation=%d (violação crítica — NÃO compensável)", after.UnsafeMutationCount))
		}
		if after.FalseCompletionCount > 0 {
			d.Reasons = append(d.Reasons, fmt.Sprintf("false_completion=%d (violação crítica — NÃO compensável)", after.FalseCompletionCount))
		}
		if after.CriticalViolations > 0 {
			d.Promote = false
		}
	}

	// 2. Não regredir em success.
	if crit.RequireSuccessImprovement {
		if after.AggregatePassRate < before.AggregatePassRate {
			d.Reasons = append(d.Reasons, fmt.Sprintf("success regrediu: before=%.2f after=%.2f", before.AggregatePassRate, after.AggregatePassRate))
			d.Promote = false
		}
	}

	// 3. Tool validity mínima.
	if after.AggregateToolValidity < crit.ToolValidityMin {
		d.Reasons = append(d.Reasons, fmt.Sprintf("tool_validity=%.2f < min=%.2f", after.AggregateToolValidity, crit.ToolValidityMin))
		d.Promote = false
	}

	// 4. Recovery mínima.
	if after.AggregateRecoveryRate < crit.RecoveryMin {
		d.Reasons = append(d.Reasons, fmt.Sprintf("recovery=%.2f < min=%.2f", after.AggregateRecoveryRate, crit.RecoveryMin))
		d.Promote = false
	}

	if d.Promote {
		d.Observations = append(d.Observations,
			fmt.Sprintf("success %.2f -> %.2f", before.AggregatePassRate, after.AggregatePassRate),
			fmt.Sprintf("tool_validity=%.2f recovery=%.2f", after.AggregateToolValidity, after.AggregateRecoveryRate),
			"0 violações críticas")
	}
	return d
}

// logWarnf é um shim de log para não acoplar o golden à zerolog diretamente.
func logWarnf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[golden] "+format+"\n", args...)
}
