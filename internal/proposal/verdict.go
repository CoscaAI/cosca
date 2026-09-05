package proposal

import (
	"fmt"
	"strings"
	"time"
)

// VerdictResult é o resultado da validação independente do Kernel isolado.
// É a resposta ao Kernel principal: APPROVE / DENY / REVIEW, sempre com
// motivo e registrando quais regras da lei foram tocadas.
type VerdictResult struct {
	// ProposalID é o ID da proposta julgada.
	ProposalID string `json:"proposal_id"`
	// Verdict é o veredicto: APPROVE, DENY ou REVIEW.
	Verdict Verdict `json:"verdict"`
	// Reason é o motivo do veredicto — obrigatório para DENY e REVIEW.
	Reason string `json:"reason"`
	// RulesHit lista os IDs das regras da lei imutável acionadas na
	// validação (vazia quando nenhuma regra foi tocada).
	RulesHit []string `json:"rules_hit,omitempty"`
	// NeedsDon informa se a execução ainda exige a aprovação do Don
	// (classe destrutiva/estratégica). Quando true, APPROVE do Kernel
	// isolado é recomendação — não liberação.
	NeedsDon bool `json:"needs_don"`
	// ValidatedAt é o momento do veredicto.
	ValidatedAt time.Time `json:"validated_at"`
	// IsFailClosed marca veredictos de emergência (timeout / kernel isolado
	// indisponível) — o padrão fail-closed nega sem abrir exceção.
	IsFailClosed bool `json:"is_fail_closed,omitempty"`
}

// Approved informa se o veredicto libera execução.
func (v *VerdictResult) Approved() bool { return v.Verdict == VerdictApprove }

// String devolve a representação compacta do veredicto.
func (v *VerdictResult) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s", v.Verdict)
	if v.NeedsDon {
		b.WriteString(" ⚠ exige Don")
	}
	if v.IsFailClosed {
		b.WriteString(" (fail-closed)")
	}
	if v.Reason != "" {
		fmt.Fprintf(&b, " — %s", v.Reason)
	}
	if len(v.RulesHit) > 0 {
		fmt.Fprintf(&b, " [%s]", strings.Join(v.RulesHit, ", "))
	}
	return b.String()
}

// ─── Resultado pós-execução (retroalimentação) ───────────────────────────────

// Outcome é o resultado real de uma execução — o feedback que volta para a
// memória validada e alimenta o julgamento futuro do Kernel isolado.
type Outcome string

const (
	// OutcomeSuccess: a execução cumpriu o objetivo.
	OutcomeSuccess Outcome = "success"
	// OutcomePartial: a execução cumpriu parcialmente.
	OutcomePartial Outcome = "partial"
	// OutcomeFailure: a execução falhou — gera revisão obrigatória.
	OutcomeFailure Outcome = "failure"
)

// ParseOutcome converte uma string em Outcome válida.
func ParseOutcome(s string) (Outcome, error) {
	switch Outcome(strings.ToLower(strings.TrimSpace(s))) {
	case OutcomeSuccess:
		return OutcomeSuccess, nil
	case OutcomePartial:
		return OutcomePartial, nil
	case OutcomeFailure:
		return OutcomeFailure, nil
	}
	return "", fmt.Errorf("resultado inválido %q (use: success, partial, failure)", s)
}

// Execution registra a execução de uma proposta aprovada.
type Execution struct {
	// ProposalID é a proposta executada.
	ProposalID string `json:"proposal_id"`
	// Actor é quem executou (papel: don, admin, specialist...).
	Actor string `json:"actor"`
	// ExecutedAt é o momento da execução.
	ExecutedAt time.Time `json:"executed_at"`
	// Outcome é o resultado real da execução.
	Outcome Outcome `json:"outcome"`
	// Note é a observação livre do executor.
	Note string `json:"note"`
}

// Failed informa se a execução falhou (gera revisão obrigatória da proposta).
func (e *Execution) Failed() bool { return e.Outcome == OutcomeFailure }
