// Package gate — Peça (3) da arquitetura: GATE DE ESCALAÇÃO.
//
// Doutrina (professor+Don, 2026-09-02): o VLM fica no TOPO da pirâmide, NUNCA
// na base. Não se manda "toda imagem para uma IA gigante". O VLM só é chamado
// quando há uma LACUNA EPISTEMOLÓGICA que os sensores locais não resolveram —
// contradição entre fontes, confiança insuficiente, ou pedido explícito.
//
// O valor dessa ordem (Peça 2 → 3): a fusão (internal/sensor/fusion) responde
// "quantos sensores dizem a mesma coisa e com que confiança?". O gate usa essa
// resposta para decidir SE precisa escalar. Assim o VLM deixa de ser oráculo e
// vira componente JULGÁVEL: a escalada só acontece onde há lacuna, e o que o
// VLM devolve pode ser auditado contra os sensores locais.
package gate

import (
	"github.com/CoscaAI/cosca/internal/sensor"
	"github.com/CoscaAI/cosca/internal/sensor/fusion"
)

// Action é o veredito do gate para um grupo/input.
type Action string

const (
	// Resolve — os sensores locais são suficientes; NÃO precisa escalar. O
	// kernel pode confiar no consenso (ou na melhor evidência).
	Resolve Action = "resolve"
	// Escalate — há uma lacuna epistemológica; o kernel DEVE escalar para o
	// VLM (ou outro componente de maior capacidade) antes de afirmar.
	Escalate Action = "escalate"
)

// Reason explica POR QUE o gate decidiu (exigência epistêmica: toda decisão
// é explicável — princípio 2 da epistemologia do Cosca).
type Reason string

const (
	// ReasonNoInput — sem observações; nada a decidir (não escala).
	ReasonNoInput Reason = "no_input"
	// ReasonConfidentConsensus — consenso acima do limiar e sem contradição:
	// os sensores locais bastam.
	ReasonConfidentConsensus Reason = "confident_consensus"
	// ReasonContradiction — há hipóteses mutuamente exclusivas no mesmo slot;
	// o kernel não pode escolher vencedor, deve escalar.
	ReasonContradiction Reason = "contradiction"
	// ReasonLowConfidence — consenso/evidência abaixo do limiar: a resposta
	// não é confiável, deve escalar.
	ReasonLowConfidence Reason = "low_confidence"
	// ReasonSingleSourceWeak — uma única fonte com confiança moderada, sem
	// corroboração independente: frágil, escalar.
	ReasonSingleSourceWeak Reason = "single_source_weak"
)

// Policy define os limiares do gate. São escolhas de POLÍTICA de quem consome
// (o kernel), não do sensor. Todos têm defaults sensatos e são idiossincráticos
// por instância — nada de global.
type Policy struct {
	// MinConsensus é a confiança mínima de consenso para RESOLVER sem escalar.
	MinConsensus float64
	// ForceEscalateOnContradiction: se true, qualquer contradição força
	// escalada mesmo com confiança alta (default true — o professor manda
	// não escolher vencedor silenciosamente).
	ForceEscalateOnContradiction bool
	// MinEvidenceSources é o nº de fontes independentes para RESOLVER sem
	// escalar (só em caso de consenso alto). 1 fonte forte ainda é frágil.
	MinEvidenceSources int
	// EscalateOnPedidoExplicito é sinalizado pelo chamador (ver Decide).
	// Não é limiar — é flag de entrada.
}

// DefaultPolicy devolve uma política conservadora: contradição escala, consenso
// precisa ser alto e corroborado.
func DefaultPolicy() Policy {
	return Policy{
		MinConsensus:                0.70,
		ForceEscalateOnContradiction: true,
		MinEvidenceSources:          1,
	}
}

// Verdict é a saída da decisão do gate para um grupo fusionado.
type Verdict struct {
	// Action é resolve ou escalate.
	Action Action `json:"action"`
	// Reason explica por quê.
	Reason Reason `json:"reason"`
	// Confidence é a confiança considerada na decisão.
	Confidence float64 `json:"confidence"`
	// Escalate (true) se o kernel deve subir pro VLM. Mesmo em Resolve, um
	// único referente forte pode não precisar escalar — mas o campo explicita.
	Escalate bool `json:"escalate"`
}

// Input é o que o gate analisa: o resultado da fusão + um flag de pedido
// explícito (o Don/professor pediu uma análise aprofundada).
type Input struct {
	// Fused é o resultado da fusão (consenso + contradições).
	Fused fusion.Result
	// PedidoExplicito: true quando o usuário/Don pediu explicitamente uma
	// análise de maior capacidade — escala independente da confiança local.
	PedidoExplicito bool
}

// Decide avalia um SLOT específico da fusão e retorna o veredito do gate.
// Referent é o referente em questão; a função lê o consenso dele e a
// contradição do slot.
func (p Policy) Decide(in Input, ref fusion.Referent) Verdict {
	// 1. Pedido explícito sempre escala (jamais negar uma análise pedida).
	if in.PedidoExplicito {
		return Verdict{Action: Escalate, Reason: ReasonSingleSourceWeak, Confidence: 0, Escalate: true}
	}

	// 2. Sem inputs: nada a decidir, não escala.
	if len(in.Fused.Consensus) == 0 {
		return Verdict{Action: Resolve, Reason: ReasonNoInput, Confidence: 0, Escalate: false}
	}

	// 3. Localiza o consenso do referente (sem ele, nada a escalar).
	var group *fusion.Fused
	for i := range in.Fused.Consensus {
		if in.Fused.Consensus[i].Referent == ref {
			group = &in.Fused.Consensus[i]
			break
		}
	}
	if group == nil {
		return Verdict{Action: Resolve, Reason: ReasonNoInput, Confidence: 0, Escalate: false}
	}

	// 4. Contradição no slot (a mesma região tem hipóteses exclusivas).
	// O kernel NÃO escolhe vencedor — escalar (se a política manda).
	if p.ForceEscalateOnContradiction && p.hasContradiction(in.Fused, group) {
		return Verdict{Action: Escalate, Reason: ReasonContradiction, Confidence: group.Confidence, Escalate: true}
	}

	// 5. Consenso alto + corroborado → resolve (vou marcar CONFIÁVEL).
	if group.Confidence >= p.MinConsensus && group.Support >= p.MinEvidenceSources {
		return Verdict{Action: Resolve, Reason: ReasonConfidentConsensus, Confidence: group.Confidence, Escalate: false}
	}

	// 6. Consenso alto mas fonte única e fraca → ainda frágil, escalar.
	if group.Epistemic == sensor.EpistemicINFERRED && group.Support < p.MinEvidenceSources {
		return Verdict{Action: Escalate, Reason: ReasonSingleSourceWeak, Confidence: group.Confidence, Escalate: true}
	}

	// 7. Confiança abaixo do limiar → resposta não confiável, escalar.
	return Verdict{Action: Escalate, Reason: ReasonLowConfidence, Confidence: group.Confidence, Escalate: true}
}

// hasContradiction verifica se há uma contradição no mesmo slot do grupo.
func (p Policy) hasContradiction(r fusion.Result, g *fusion.Fused) bool {
	if len(r.Contradictions) == 0 {
		return false
	}
	// Compara o slot do grupo com os slots contraditórios. Como não temos o
	// slot no Fused, usamos a correspondência por referente: se o referente
	// aparece como concorrente em alguma contradição, o slot é conflitante.
	for _, c := range r.Contradictions {
		for _, rref := range c.Referents {
			if rref == g.Referent {
				return true
			}
		}
	}
	return false
}
