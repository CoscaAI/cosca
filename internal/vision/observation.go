// Primitive epistemico: transforma percepcao em evidencia com rigor.
//
// COSCA nao diz "eu vi" sem dizer em que nivel de certeza. Toda observacao
// carrega um estado epistemico e uma correcao via corroboracao. E isso que
// separa um sistema que mede de um que "descreve telas".
//
//   OBSERVED  -> o sensor reportou (nao e verdade ontologica, mas e reportado)
//   TRACKED   -> identidade mantida entre frames
//   PREDICTED -> extrapolado sem deteccao (NAO vira certeza)
//   UNKNOWN   -> nao da para saber
//
// Corroboracao: outras fontes (OCR, estado estrutural, outro modelo, frames
// vizinhos) elevam uma observacao de candidata a evidencia.
package vision

// EpistemicState rotula a origem/certeza de uma observacao.
type EpistemicState string

const (
	EpistemicObserved  EpistemicState = "OBSERVED"
	EpistemicTracked   EpistemicState = "TRACKED"
	EpistemicPredicted EpistemicState = "PREDICTED"
	EpistemicUnknown   EpistemicState = "UNKNOWN"
)

// Observation e uma percepcao rotulada epistemicamente.
type Observation struct {
	// Epistemic e o estado de certeza da observacao.
	Epistemic EpistemicState `json:"epistemic"`
	// Text e a descricao (VLM ou OCR) do que foi observado.
	Text string `json:"text,omitempty"`
	// Object e a entidade percebida (ex.: "person"/"avatar"/"currency_counter").
	Object string `json:"object,omitempty"`
	// Value e um numero medido, se houver (ex.: contagem de moedas).
	Value float64 `json:"value,omitempty"`
	// Confidence e a confianca reportada pelo sensor (0..1).
	Confidence float64 `json:"confidence,omitempty"`
	// Corroborated indica se ja passou por corroboracao (vira evidencia).
	Corroborated bool `json:"corroborated,omitempty"`
	// CorroboratedBy lista as fontes que corroboraram.
	CorroboratedBy []string `json:"corroborated_by,omitempty"`
}

// EvidenceLevel e o grau de evidencia apos corroboracao.
type EvidenceLevel string

const (
	EvidenceNone    EvidenceLevel = "NONE"    // apenas observacao candidata
	EvidenceLow     EvidenceLevel = "LOW"     // uma fonte, nao contradita
	EvidenceMedium  EvidenceLevel = "MEDIUM"  // 2+ fontes concordam
	EvidenceHigh    EvidenceLevel = "HIGH"    // fontes independentes concordam
)

// IsReported responde se a observacao veio de um sensor (OBSERVED/TRACKED).
func (o Observation) IsReported() bool {
	return o.Epistemic == EpistemicObserved || o.Epistemic == EpistemicTracked
}

// IsCertain responde se a observacao atingiu nivel de evidencia.
func (o Observation) IsCertain() bool {
	return o.Corroborated && o.Epistemic == EpistemicObserved
}

// Corroborate marca a observacao como corroborada por uma fonte e eleva a
// evidencia. Nunca transforma UNKNOWN/PREDICTED em OBSERVED — apenas registra
// que a evidencia ganhou suporte.
func (o *Observation) Corroborate(source string) {
	if source == "" {
		return
	}
	o.Corroborated = true
	o.CorroboratedBy = append(o.CorroboratedBy, source)
}

// level computa o nivel de evidencia a partir da corroboracao.
func (o *Observation) Level() EvidenceLevel {
	n := len(o.CorroboratedBy)
	switch {
	case o.Epistemic == EpistemicUnknown || !o.IsReported():
		if !o.Corroborated {
			return EvidenceNone
		}
		return EvidenceLow
	case n >= 2:
		return EvidenceHigh
	case n == 1:
		return EvidenceMedium
	default:
		return EvidenceLow
	}
}
