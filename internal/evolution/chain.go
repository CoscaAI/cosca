// chain.go — Cadeia de Proveniência com propagação de CONTAMINAÇÃO (ADR-022).
//
// O desenho do professor (e a propriedade que a gema #4 deveria ter):
//
//	Source → Observation → Inference → Decision
//	⚠️ Source → ⚠️ Observation → ⚠️ Inference → 🚫 Decision blocked
//
// "Contaminação NÃO pode virar verdade simplesmente porque foi propagada pelo
// grafo." Se X depende de Y e Y depende de uma fonte comprometida, X DEVE
// carregar essa informação — automaticamente. Aqui, ao adicionar um link à
// cadeia, se um link anterior está contaminado, o novo herda o taint
// (propagação para baixo). A decisão é bloqueada, ou pelo menos recebe o nível
// de confiança/risco apropriado.
//
// COMPÕE com a epistemologia existente (NÃO cria uma paralela — ADR-022):
//   - Epistemic usa o vocabulário I4 (FACT/MEASURED/INFERRED) — produzido pelo
//     produtor, mapeado para provenance.ClaimKind na borda.
//   - Trust (gradiente de risco) mapeia para contenttrust.Trust (I8) na borda.
//   - ProvenanceRef é o I3 (de onde veio).
//   - Confidence é a conformal predição (#4) — quão certo.
//
// Determinístico (I1), fail-closed (I2): contaminação → decisão bloqueada.
package evolution

// Trust é o gradiente de confiança/risco de um link da cadeia.
type Trust int

const (
	TrustSound      Trust = iota // confiável
	TrustSuspicious              // suspeito (incerto, requer verificação)
	TrustPoisoned                // contaminada (fonte comprometida)
)

// Link é um nó da cadeia de proveniência.
type Link struct {
	Label         string  `json:"label"`          // "source"|"observation"|"inference"|"decision"
	Trust         Trust   `json:"trust"`
	Epistemic     string  `json:"epistemic"`      // "FACT"|"MEASURED"|"INFERRED" (I4)
	ProvenanceRef string  `json:"provenance_ref"` // I3
	Confidence    float64 `json:"confidence"`     // conformal calibrada (0..1)
}

// ProvenanceChain é a cadeia Source→Observation→Inference→Decision.
type ProvenanceChain struct {
	Links []Link `json:"links"`
}

// Add apende um link. PROPAGA contaminação: se a cadeia já está contaminada,
// o novo link é marcado como Poisoned também (⚠️ desce a cadeia inteira).
// Contaminação nunca "vira verdade" por ser propagada.
func (c *ProvenanceChain) Add(l Link) {
	if c.Contaminated() && l.Trust < TrustPoisoned {
		l.Trust = TrustPoisoned
	}
	c.Links = append(c.Links, l)
}

// Contaminated indica se algum link (direto ou propagado) é Poisoned.
func (c ProvenanceChain) Contaminated() bool {
	for _, l := range c.Links {
		if l.Trust == TrustPoisoned {
			return true
		}
	}
	return false
}

// MaxTrust devolve o pior nível de risco da cadeia.
func (c ProvenanceChain) MaxTrust() Trust {
	w := TrustSound
	for _, l := range c.Links {
		if l.Trust > w {
			w = l.Trust
		}
	}
	return w
}

// DecisionBlocked informa se a decisão (fim da cadeia) deve ser BLOQUEADA —
// fail-closed (I2): qualquer contaminação na cadeia bloqueia.
func (c ProvenanceChain) DecisionBlocked() bool {
	return c.Contaminated()
}

// ToContentTrustTrust mapeia o gradiente para o I8 (contenttrust.Trust) na
// borda: só TrustSound é "trusted"; o resto é "untrusted" (nunca autoridade).
func (t Trust) ToContentTrustTrust() string {
	if t == TrustSound {
		return "trusted"
	}
	return "untrusted"
}
