// Package cognitive — auto-inspeção do Kernel (as métricas da L13, em código).
//
// Inspirado (referência) nos specs antigos do backup (Entropia Cognitiva,
// B1-B5, "até onde prevê") — mas MUITO melhor: determinístico, mede dados
// reais, alinhado ao freio G1-G9. É a disciplina que já construímos, agora com
// NÚMERO. Cobre a Lição L13: "velocidade transforma desvio em 500 arquivos
// corrompidos" → medir reversões é a vacina contra o loop que matou o
// "outro Kernel".
package cognitive

// ============================================================
// QUALIDADE DE DECISÃO (B3/L13) — decisões → reversões → taxa
// ============================================================

// Categoria de causa raiz de uma reversão (as 5 da L13). Só medir a reversão
// não basta — medir a CLASSE evita recorrência (meta: zero na mesma classe).
const (
	CatBadEvidence        = "bad_evidence"
	CatMissingAlternative = "missing_alternative"
	CatAssumptionError    = "assumption_error"
	CatBypassGovernance   = "bypass_governance"
	CatOutdatedInfo       = "outdated_info"
)

// Categorias é a lista fixa das 5 categorias de causa raiz.
var Categorias = []string{
	CatBadEvidence, CatMissingAlternative, CatAssumptionError,
	CatBypassGovernance, CatOutdatedInfo,
}

// DecisionRecord é uma decisão tomada pelo Kernel com o seu resultado.
type DecisionRecord struct {
	ID       string `json:"id"`
	Decision string `json:"decision"`
	// Outcome: "success" | "reverted" (voltou atrás — um erro de decisão)
	Outcome string `json:"outcome"`
	// Category (só para reverted): a causa raiz — para medir recorrência
	Category string `json:"category,omitempty"`
}

// DecisionQuality é a métrica de qualidade de decisão do Kernel.
type DecisionQuality struct {
	Decisions   int     `json:"decisions"`
	Reverted    int     `json:"reverted"`
	SuccessRate float64 `json:"success_rate"` // 0-1
	RevertRate  float64 `json:"revert_rate"`  // 0-1
	// Categorias de reversão: quantas de cada CLASSE (para evitar recorrência)
	ByCategory map[string]int `json:"by_category"`
	// Cobertura de proteção: % de categorias que têm regra de proteção (L13)
	ProtectionCoverage float64 `json:"protection_coverage"`
}

// MeasureDecisionQuality calcula a qualidade das decisões do Kernel.
// A L13 é explícita: "não é zero reversões; é zero reversões da MESMA classe."
func MeasureDecisionQuality(records []DecisionRecord, protectedCategories map[string]bool) DecisionQuality {
	q := DecisionQuality{
		Decisions:  len(records),
		ByCategory: map[string]int{},
	}
	for _, r := range records {
		if r.Outcome == "reverted" {
			q.Reverted++
			if r.Category != "" {
				q.ByCategory[r.Category]++
			}
		}
	}
	if q.Decisions > 0 {
		q.SuccessRate = float64(q.Decisions-q.Reverted) / float64(q.Decisions)
		q.RevertRate = float64(q.Reverted) / float64(q.Decisions)
	}
	// cobertura de proteção: % das categorias com regra (o que a L13 pede)
	covered := 0
	for _, c := range Categorias {
		if protectedCategories[c] {
			covered++
		}
	}
	q.ProtectionCoverage = float64(covered) / float64(len(Categorias))
	return q
}

// VulnerableCategories devolve as categorias de reversão SEM regra de proteção
// — as que merecem atenção (recorrência possível).
func (q DecisionQuality) VulnerableCategories(protected map[string]bool) []string {
	var out []string
	for _, c := range Categorias {
		if q.ByCategory[c] > 0 && !protected[c] {
			out = append(out, c)
		}
	}
	return out
}

// ============================================================
// HORIZONTE (profundidade preditiva) — "até onde consegue prever"
// ============================================================

// PlanDepth avalia a profundidade (horizonte) de um plano: quantos passos de
// consequência a cadeia projeta. 0-2 = míope (resolve sintoma) · 15+ = maduro
// (antecipa cascata). Melhor que o spec antigo: é determinístico e conta
// passos reais da cadeia causal, não um palpite manual.
func PlanDepth(numSteps int) int {
	switch {
	case numSteps <= 2:
		return 1 // míope
	case numSteps <= 8:
		return 2 // intermediário
	case numSteps <= 15:
		return 3 // profundo
	default:
		return 4 // visionário
	}
}

// HorizonLevel descreve o nível de profundidade.
func HorizonLevel(depth int) string {
	switch depth {
	case 4:
		return "visionário (25+ passos) — antecipa cascatas"
	case 3:
		return "profundo (15+) — planejamento estratégico"
	case 2:
		return "intermediário (8) — considera consequências"
	default:
		return "míope (0-2) — resolve sintoma, cria problemas downstream"
	}
}
