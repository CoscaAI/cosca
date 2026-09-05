package voice

// ── Phoneme: o átomo da voz Cosca ───────────────────────────────────────
// Em vez de []string, cada fonema carrega sua própria epistemologia:
// origem da evidência (FACT/LEXICON/INFERRED/FALLBACK), tonicidade,
// nasalidade e classe de duração — o que o sintetizador usa para
// prosódia e coarticulação. (Diretriz do Professor: a voz não é uma
// sequência de pedaços, é uma sequência de decisões com evidência.)

// Evidence: origem da pronúncia (régua P13 — nada sem verificação).
type Evidence string

const (
	EvidenceFact     Evidence = "FACT"     // regra ortográfica aplicada com certeza (dígrafos, c+e/i, etc.)
	EvidenceLexicon  Evidence = "LEXICON"  // pronúncia encontrada no vocabulário da casa
	EvidenceInferred Evidence = "INFERRED" // pronúncia inferida por regra contextual
	EvidenceFallback Evidence = "FALLBACK" // pronúncia pela regra geral (pode errar)
)

// DurClass: classe de duração relativa para o sintetizador.
type DurClass string

const (
	DurVowelLong   DurClass = "vowel_long"   // vogal tônica (a, e, o abertos)
	DurVowelShort  DurClass = "vowel_short"  // vogal átona
	DurNasal       DurClass = "nasal"        // vogal nasal
	DurStop        DurClass = "stop"         // oclusiva (p b t d k g)
	DurFricative   DurClass = "fricative"    // fricativa (f v s z S Z)
	DurNasalCons   DurClass = "nasal_cons"   // nasal consoante (m n J)
	DurLiquid      DurClass = "liquid"       // líquida (l L r R)
	DurGlide       DurClass = "glide"        // semivogal (j w)
	DurPause       DurClass = "pause"        // pausa entre palavras
)

// Phoneme: um fonema com sua evidência e atributos fonológicos.
type Phoneme struct {
	Symbol string    // SAMPA (a, E, e, S, J, ~, etc.)
	Stress bool      // é a vogal tônica da palavra?
	Nasal  bool      // é nasal?
	Dur    DurClass  // classe de duração
	Evid   Evidence  // origem da pronúncia
}

// IsVowel: é vogal (oral ou nasal)?
func (p Phoneme) IsVowel() bool {
	if len(p.Symbol) == 0 {
		return false
	}
	switch p.Symbol[0] {
	case 'a', 'e', 'i', 'o', 'u', 'E', 'O':
		return true
	}
	return false
}

// SAMPA: representação textual (interoperabilidade com o banco de dífonos).
func (p Phoneme) SAMPA() string {
	s := p.Symbol
	if p.Nasal && len(s) == 1 {
		s += "~"
	}
	if p.Stress && p.IsVowel() {
		s = "'" + s
	}
	return s
}

// EvidStr: rótulo curto para debugging.
func (p Phoneme) EvidStr() string { return string(p.Evid) }