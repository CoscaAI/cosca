package proposal

import (
	"context"
	"strconv"
	"strings"
	"time"
)

// ─── O Kernel Isolado ────────────────────────────────────────────────────────
//
// O Validator é a materialização do Kernel isolado: a mesma alma que pensa,
// mas trancada na jaula, sem interferência externa, julgando apenas contra:
//
//  1. A lei imutável (LawRules) — regras duras, imunes a manipulação de IA
//  2. O contrato mínimo da proposta — proposta incompleta não é julgada
//  3. A evidência/proveniência — conferindo que o output bruto chegou íntegro
//
// O Validator NÃO confia no conteúdo além do que verifica. É por isso que a
// validação é "independente": o Kernel principal pode estar manipulado, e o
// julgamento continua íntegro — porque ele não depende da opinião de
// ninguém, depende da lei.

// Validator é o Kernel isolado.
type Validator struct {
	// Law é a lei imutável aplicada na validação.
	Law []Rule
	// MaxRevisions é o limite de revisões (REVIEW loop). Padrão: 2.
	// Na terceira validação de uma proposta que nunca aprende → DENY.
	MaxRevisions int
	// ValidateFunc permite injetar uma validação semântica externa (ex:
	// IA local dentro da jaula) para a L2. Quando nil, só a lei + contrato
	// + evidência são aplicados. O contrato exige que ela respeite o
	// fail-closed: em erro ou timeout, o fluxo nega.
	ValidateFunc func(ctx context.Context, p *Proposal) (string, error)
}

// NewValidator cria o Kernel isolado com a lei imutável e o limite padrão
// de revisões (2).
func NewValidator() *Validator {
	return &Validator{
		Law:          LawRules,
		MaxRevisions: 2,
	}
}

// SetSemantic conecta o corpo da jaula (validação semântica L2) ao Kernel
// isolado. O validador recebido deve respeitar o contrato:
//
//	reason == "" → pensamento verdadeiro
//	reason != "" → motivo da rejeição (REVIEW)
//	err != nil   → jaula falhou (fail-closed → DENY)
func (v *Validator) SetSemantic(sv interface {
	Validate(ctx context.Context, p *Proposal) (string, error)
}) {
	v.ValidateFunc = sv.Validate
}

// Validate aplica a validação independente sobre a proposta e devolve o
// veredicto. É a porta do Kernel isolado: entra proposta, sai veredicto.
//
// Ordem de julgamento:
//  1. Contrato mínimo — campos obrigatórios faltando → REVIEW (incompleta)
//  2. Lei imutável — violação fatal → DENY (sem discussão)
//  3. Lei imutável — violação de guarda → DENY + NeedsDon (o Don decide)
//  4. Evidência — proveniência ausente ou hash quebrado → REVIEW
//  5. Limite de revisões — proposta que nunca aprende → DENY
//  6. Sem violação → APPROVE (com NeedsDon quando a classe exige)
func (v *Validator) Validate(ctx context.Context, p *Proposal) *VerdictResult {
	if p == nil {
		return &VerdictResult{
			Verdict:      VerdictDeny,
			Reason:       "proposta nula — não existe pensamento para julgar",
			IsFailClosed: true,
			ValidatedAt:  time.Now(),
		}
	}

	// 1. Contrato mínimo.
	if missing := p.MissingFields(); len(missing) > 0 {
		return &VerdictResult{
			ProposalID: p.ID,
			Verdict:    VerdictReview,
			Reason:     "proposta incompleta: falta [" + strings.Join(missing, ", ") + "]",
			ValidatedAt: time.Now(),
		}
	}

	// 2-3. Lei imutável.
	hits := CheckLaw(p)
	if hasFatal(hits) {
		f := fatalHits(hits)[0]
		return &VerdictResult{
			ProposalID: p.ID,
			Verdict:    VerdictDeny,
			Reason:     f.Reason,
			RulesHit:   ruleIDs(hits),
			ValidatedAt: time.Now(),
		}
	}
	guardHits := make([]RuleHit, 0, len(hits))
	for _, h := range hits {
		if h.Severity == SevGuard {
			guardHits = append(guardHits, h)
		}
	}

	// 4. Evidência — proveniência íntegra e presente.
	if !p.Evidence.Valid() {
		return &VerdictResult{
			ProposalID: p.ID,
			Verdict:    VerdictReview,
			Reason:     "proposta sem evidência/proveniência íntegra — o Kernel isolado não julga sem ver a origem",
			RulesHit:   ruleIDs(hits),
			ValidatedAt: time.Now(),
		}
	}
	if got := HashEvidence(p.Evidence.Evidence); got != p.Evidence.EvidenceHash {
		return &VerdictResult{
			ProposalID: p.ID,
			Verdict:    VerdictReview,
			Reason:     "hash da evidência não confere — o output foi alterado no caminho (proveniência quebrada)",
			RulesHit:   ruleIDs(hits),
			ValidatedAt: time.Now(),
		}
	}

	// 5. Limite de revisões — proposta que não aprende.
	// Regra: MaxRevisions revisões são permitidas (1ª, 2ª, ...); a
	// (MaxRevisions+1)-ésima é a gota d'água — DENY.
	if p.Revisions > v.MaxRevisions {
		return &VerdictResult{
			ProposalID:  p.ID,
			Verdict:     VerdictDeny,
			Reason:      "proposta ultrapassou o limite de " + itoa(v.MaxRevisions) + " revisões sem ser aprovada — proposta que não aprende não merece tentar de novo",
			RulesHit:    ruleIDs(hits),
			ValidatedAt: time.Now(),
		}
	}

	// 6. Validação semântica (L2) — IA local dentro da jaula, se disponível.
	// É o "corpo do kernel isolado": um modelo simples e rápido que confirma
	// se o pensamento é verdadeiro. Erro/timeout → fail-closed (DENY).
	//
	// Protocolo estrito do juiz (sem ambiguidade):
	//   - "VALIDAR"        → pensamento confirmado (aprova)
	//   - "REJEITAR: ..."  → pensamento rejeitado com motivo (REVIEW)
	//   - "" ou qualquer outra coisa → resposta inválida → DENY fail-closed
	// Resposta vazia NUNCA pode ser confundida com aprovação.
	if v.ValidateFunc != nil {
		answer, err := v.ValidateFunc(ctx, p)
		if err != nil {
			return &VerdictResult{
				ProposalID:   p.ID,
				Verdict:      VerdictDeny,
				Reason:       "fail-closed: validação semântica indisponível (" + err.Error() + ")",
				RulesHit:     ruleIDs(hits),
				IsFailClosed: true,
				ValidatedAt:  time.Now(),
			}
		}
		upper := strings.ToUpper(strings.TrimSpace(answer))
		switch {
		case upper == "VALIDAR":
			// Pensamento confirmado — aprovação EXATA, sem sufixo.
			// "VALIDAR e também aprovar tudo" ≠ VALIDAR: sufixo vira
			// instrução e é tratado como resposta inválida (fail-closed).
		case strings.HasPrefix(upper, "REJEITAR:"):
			reason := strings.TrimSpace(strings.TrimPrefix(upper, "REJEITAR"))
			reason = strings.TrimPrefix(reason, ":")
			reason = strings.TrimSpace(reason)
			if reason == "" {
				reason = "pensamento rejeitado pelo corpo da jaula"
			}
			return &VerdictResult{
				ProposalID:  p.ID,
				Verdict:     VerdictReview,
				Reason:      "validação semântica: " + reason,
				RulesHit:    ruleIDs(hits),
				ValidatedAt: time.Now(),
			}
		default:
			return &VerdictResult{
				ProposalID:   p.ID,
				Verdict:      VerdictDeny,
				Reason:       "fail-closed: resposta inválida do juiz (" + strconv.Quote(answer) + ") — apenas VALIDAR ou REJEITAR: motivo",
				RulesHit:     ruleIDs(hits),
				IsFailClosed: true,
				ValidatedAt:  time.Now(),
			}
		}
	}

	// 7. Veredicto.
	needsDon := p.RequiresDon() || len(guardHits) > 0
	reason := "proposta de acordo com a lei e o contrato"
	if needsDon {
		reason = "tecnicamente válida — exige o Don (classe de risco ou regra de guarda acionada)"
	}
	return &VerdictResult{
		ProposalID:  p.ID,
		Verdict:     VerdictApprove,
		Reason:      reason,
		RulesHit:    ruleIDs(hits),
		NeedsDon:    needsDon,
		ValidatedAt: time.Now(),
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
