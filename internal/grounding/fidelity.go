package grounding

import (
	"context"
)

// DefaultFaithfulnessThreshold é o piso de fidelidade abaixo do qual o
// veredito é BLOCK (0.60, do ADR-011 §2/Bloco 1).
const DefaultFaithfulnessThreshold = 0.60

// Verdict é o veredito de fidelidade que o chamador aplica antes de entregar
// uma resposta generativa.
type Verdict string

const (
	// VerdictBlock: a resposta NÃO deve ser entregue generativamente.
	// O chamador entrega um placeholder seguro e preserva a resposta original
	// em BlockedResponse para auditoria.
	VerdictBlock Verdict = "block"
	// VerdictFlag: entrega com aviso/encaixe de baixa confiança por claim
	// (threshold <= faithfulness < 1.0).
	VerdictFlag Verdict = "flag"
	// VerdictDeliver: entrega. Também é o veredito quando o scorer está
	// indisponível (faithfulness == -1 → fail-open, nunca bloqueia).
	VerdictDeliver Verdict = "deliver"
)

// FidelityReport é o agregado da verificação: quantos claims foram
// sustentados pelos chunks e o veredito correspondente.
type FidelityReport struct {
	// Faithfulness = Supported/Total. -1 quando o scorer está indisponível
	// (sem chunks ou sem claims extraídos) — fail-open, NUNCA bloqueia.
	Faithfulness float64
	// Supported é o número de claims sustentados (Supported == true).
	Supported int
	// Total é o número de claims verificados.
	Total int
	// Claims é o veredito por claim (a atribuição claim → chunk + termos).
	Claims []ClaimVerdict
	// Threshold é o piso de fidelidade usado no Verdict (default
	// DefaultFaithfulnessThreshold). Zero cai no default.
	Threshold float64
}

// Verdict devolve o veredito de fidelidade na hora:
//
//	faithfulness == -1            → deliver (fail-open do scorer — nunca block)
//	0 <= faithfulness < threshold → block
//	threshold <= faithfulness < 1 → flag
//	faithfulness == 1.0           → deliver
//
// O piso default é DefaultFaithfulnessThreshold (0.60).
func (r FidelityReport) Verdict() Verdict {
	if r.Faithfulness < 0 {
		return VerdictDeliver
	}
	th := r.Threshold
	if th <= 0 {
		th = DefaultFaithfulnessThreshold
	}
	switch {
	case r.Faithfulness < th:
		return VerdictBlock
	case r.Faithfulness < 1.0:
		return VerdictFlag
	default:
		return VerdictDeliver
	}
}

// Verify orquestra a verificação de fidelidade de uma resposta: extrai os
// claims de `answer`, verifica cada um contra os `chunks` e computa a fração
// de claims sustentados.
//
// Regras:
//   - len(chunks) == 0 → Faithfulness = -1 (scorer indisponível; veredito
//     deliver — "sem evidência não gera" é responsabilidade do BuildAnswer).
//   - len(claims extraídos) == 0 → Faithfulness = -1 (nada a verificar;
//     fail-open, mesmo com chunks presentes).
//   - caso contrário → Faithfulness = Supported/Total.
func Verify(ctx context.Context, query, answer string, chunks []SourceChunk, opts VerifyOptions) FidelityReport {
	if len(chunks) == 0 {
		return FidelityReport{Faithfulness: -1, Total: 0}
	}
	_ = ctx // contexto reservado para uma futura verificação com cancelamento

	claims := ExtractClaims(answer, DefaultCitePatterns)
	if len(claims) == 0 {
		return FidelityReport{Faithfulness: -1, Total: 0, Threshold: opts.Threshold}
	}

	verdicts := make([]ClaimVerdict, 0, len(claims))
	supported := 0
	for _, c := range claims {
		v := VerifyClaim(query, c, chunks, opts)
		verdicts = append(verdicts, v)
		if v.Supported {
			supported++
		}
	}

	total := len(verdicts)
	faith := float64(supported) / float64(total)
	return FidelityReport{
		Faithfulness: round4(faith),
		Supported:    supported,
		Total:        total,
		Claims:       verdicts,
		Threshold:    opts.Threshold,
	}
}
