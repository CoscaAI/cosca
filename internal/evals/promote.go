package evals

import (
	"github.com/CoscaAI/cosca/internal/evalgo"
)

// Assinaturas de falha estáveis usadas pelo gate (fail-closed I2). O
// "status:error"/"status:timeout" denotam quebra de INFRA (pipeline rompeu) —
// nunca devem promover. Falhas de verificação usam assinatura granular.
const (
	sigError   = "status:error"
	sigTimeout = "status:timeout"
	sigFailed  = "status:failed"
)

// Promotion é o veredito de promoção de um run — a camada de gating do evalgo
// composta sobre os resultados do evals (ADR-023 item 6, Rota A). Reusa a
// suíte existente e DECIDE com fail-closed; não é um subsistema paralelo.
type Promotion struct {
	Decision evalgo.GateDecision `json:"decision"`
	Clusters []evalgo.Cluster    `json:"clusters"`
	PassRate float64             `json:"pass_rate"`
	Total    int                 `json:"total"`
	Passed   int                 `json:"passed"`
}

// failureSignature deriva uma assinatura estável de falha de um CaseResult,
// para cluster ("onde o agente erra consistentemente") + gate fail-closed.
func failureSignature(c CaseResult) string {
	switch c.Status {
	case StatusError:
		return sigError
	case StatusTimeout:
		return sigTimeout
	case StatusFailed:
		// Primeira verificação que falhou → assinatura GRANULAR (qual passo).
		for _, v := range c.VerifyResults {
			if !v.OK {
				return "verify:" + v.Command
			}
		}
		return sigFailed
	default:
		return ""
	}
}

// PromoteGate converte os casos do report em resultados do evalgo e aplica o
// gate para pronunciar a promoção. Fail-closed I2: o gate nunca "conserta"; só
// aprova se a taxa bater, houve casos suficientes e nenhuma assinatura
// bloqueante (erro/timeout de infra).
func PromoteGate(report *Report, gate *evalgo.Gate) Promotion {
	results := make([]evalgo.Result, 0, len(report.Cases))
	for _, c := range report.Cases {
		results = append(results, evalgo.Result{
			CaseID:    c.ID,
			Kind:      string(c.Status),
			Pass:      c.Status == StatusPassed,
			Signature: failureSignature(c),
		})
	}

	passed := 0
	for _, r := range results {
		if r.Pass {
			passed++
		}
	}
	total := len(results)
	rate := 0.0
	if total > 0 {
		rate = float64(passed) / float64(total)
	}

	return Promotion{
		Decision: gate.Decide(results),
		Clusters: evalgo.Clusters(results),
		PassRate: rate,
		Total:    total,
		Passed:   passed,
	}
}

// DefaultPromoteGate devolve o gate de promoção padrão: fail-closed em erro e
// timeout de infra (a pipeline quebrada nunca promove), pass rate 0.8, mínimo
// de 3 casos para o run ser significativo. Significa "evidência antes de
// promover" (anti ADR-016).
func DefaultPromoteGate() *evalgo.Gate {
	return evalgo.NewGate(
		evalgo.WithMinPassRate(0.8),
		evalgo.WithMinCases(3),
		evalgo.WithBlockingSignatures(sigError, sigTimeout),
	)
}
