package voice

// ── Regras ortográficas do PB (grafema → fonema) ─────────────────────────
// Cada regra retorna o(s) fonema(s) e a evidência correspondente.
// Regras CERTAS (dígrafos, c+e/i, g+e/i, s intervocálico) = FACT.
// Regras CONTEXTUAIS (x, r, nasalização) = INFERRED.
// Regra geral de última instância = FALLBACK.

import (
	"strings"
)

// conjuntos úteis
var vogaisLetras = "aeiouáéíóúâêôãõà"
var consoantesLetras = "bcdfghjklmnpqrstvxz"

func isVogalLetra(r rune) bool { return strings.ContainsRune(vogaisLetras, r) }

func isConsoanteLetra(r rune) bool { return strings.ContainsRune(consoantesLetras, r) }

// regraC: c + e/i → s ; c + a/o/u → k ; ch → S. FACT.
func regraC(c, nx, nxx rune) ([]Phoneme, int) {
	if c == 'c' && nx == 'h' {
		return []Phoneme{{Symbol: "S", Evid: EvidenceFact}}, 2
	}
	if c == 'c' {
		if nx == 'e' || nx == 'i' {
			return []Phoneme{{Symbol: "s", Evid: EvidenceFact}}, 1
		}
		return []Phoneme{{Symbol: "k", Evid: EvidenceFact}}, 1
	}
	return nil, 0
}

// regraG: g + e/i → Z ; g + a/o/u → g ; gu+e/i → g. FACT.
func regraG(c, nx, nxx rune) ([]Phoneme, int) {
	if c == 'g' && nx == 'u' && (nxx == 'e' || nxx == 'i') {
		return []Phoneme{{Symbol: "g", Evid: EvidenceFact}}, 2
	}
	if c == 'g' && nx == 'u' && nxx == 'a' {
		return []Phoneme{{Symbol: "g", Evid: EvidenceFact}, {Symbol: "w", Evid: EvidenceFact}}, 2
	}
	if c == 'g' {
		if nx == 'e' || nx == 'i' {
			return []Phoneme{{Symbol: "Z", Evid: EvidenceFact}}, 1
		}
		return []Phoneme{{Symbol: "g", Evid: EvidenceFact}}, 1
	}
	return nil, 0
}

// regraQ: qu+e/i → k ; qua → kw. FACT.
func regraQ(c, nx, nxx rune) ([]Phoneme, int) {
	if c == 'q' && nx == 'u' && (nxx == 'e' || nxx == 'i') {
		return []Phoneme{{Symbol: "k", Evid: EvidenceFact}}, 2
	}
	if c == 'q' && nx == 'u' && nxx == 'a' {
		return []Phoneme{{Symbol: "k", Evid: EvidenceFact}, {Symbol: "w", Evid: EvidenceFact}}, 2
	}
	if c == 'q' {
		return []Phoneme{{Symbol: "k", Evid: EvidenceFact}}, 1
	}
	return nil, 0
}

// regraS: s entre vogais → z ; s + consoante/fim → s ; ss → s. FACT.
func regraS(c, prev, nx rune, isVogalPrev, isVogalNx bool) ([]Phoneme, int) {
	if c != 's' {
		return nil, 0
	}
	if isVogalPrev && isVogalNx {
		return []Phoneme{{Symbol: "z", Evid: EvidenceFact}}, 1
	}
	return []Phoneme{{Symbol: "s", Evid: EvidenceFact}}, 1
}

// regraX: a grande vilã. /S/ caixa, /s/ máximo, /z/ exame, /ks/ táxi.
// Estratégia: prefixos conhecidos + contexto, resto FALLBACK /S/.
//   exa-, exe-, exi- + vogal → z (exame, exército, exato)
//   -x- entre vogais em palavras como "táxi", "sintaxe" → ks (palavra rara: FALLBACK)
//   -x- após consoante (máximo, texto) → s
//   início + vogal (xícara, xadrez) → S
//   "x" final (fax, xerox) → S
func regraX(w []rune, i int) ([]Phoneme, int) {
	if w[i] != 'x' {
		return nil, 0
	}
	n := len(w)
	prev := rune(0)
	if i > 0 {
		prev = w[i-1]
	}
	nx := rune(0)
	if i+1 < n {
		nx = w[i+1]
	}
	// exa-/exe-/exi- + vogal → z (exame, exército, exato): INFERRED.
	// O x do prefixo "ex" está no índice 1 (após o 'e').
	if i == 1 && w[0] == 'e' && i+1 < n && (w[i+1] == 'a' || w[i+1] == 'e' || w[i+1] == 'i') {
		return []Phoneme{{Symbol: "z", Evid: EvidenceInferred}}, 1
	}
	// após consoante → s (máximo, texto, sintaxe): INFERRED
	if prev != 0 && isConsoanteLetra(prev) {
		return []Phoneme{{Symbol: "s", Evid: EvidenceInferred}}, 1
	}
	// entre vogais: /S/ (caixa, bruxa, enxame — mais comum)
	if prev != 0 && nx != 0 && isVogalLetra(prev) && isVogalLetra(nx) {
		return []Phoneme{{Symbol: "S", Evid: EvidenceInferred}}, 1
	}
	// início de palavra + vogal → S (xícara, xadrez): INFERRED
	if i == 0 && nx != 0 && isVogalLetra(nx) {
		return []Phoneme{{Symbol: "S", Evid: EvidenceInferred}}, 1
	}
	// resto: FALLBACK /S/
	return []Phoneme{{Symbol: "S", Evid: EvidenceFallback}}, 1
}

// regraR: início de palavra ou após l/n/s → R (forte) ; senão r (simples).
// A realização acústica (tepe vs vibrante) fica para o sintetizador.
func regraR(w []rune, i int) ([]Phoneme, int) {
	if w[i] != 'r' {
		return nil, 0
	}
	if i == 0 {
		return []Phoneme{{Symbol: "R", Evid: EvidenceInferred}}, 1
	}
	prev := w[i-1]
	if prev == 'l' || prev == 'n' || prev == 's' {
		return []Phoneme{{Symbol: "R", Evid: EvidenceInferred}}, 1
	}
	return []Phoneme{{Symbol: "r", Evid: EvidenceInferred}}, 1
}

// regraNasal: vogal + m/n diante de consoante ou fim → vogal nasal,
// CONSUMANDO o m/n (correção do Professor: a regra antiga gerava
// fonemas extras). Ex.: campo → k a~ p u (m consumido).
// Evidência: INFERRED (padrão confiável do PB).
func regraNasal(c, nx, nxx rune, isVogalNx bool) ([]Phoneme, int) {
	if !(c == 'a' || c == 'e' || c == 'i' || c == 'o' || c == 'u') {
		return nil, 0
	}
	if nx != 'm' && nx != 'n' {
		return nil, 0
	}
	// m/n seguido de consoante ou fim de palavra → nasal + consume
	if nxx == 0 || isConsoanteLetra(nxx) || nxx == 's' || nxx == 'z' {
		base := map[rune]string{'a': "a~", 'e': "e~", 'i': "i~", 'o': "o~", 'u': "u~"}[c]
		return []Phoneme{{Symbol: base, Nasal: true, Evid: EvidenceInferred}}, 2 // consome m/n
	}
	// m/n seguido de vogal (banana, animal): vogal oral, m/n = consoante
	return nil, 0
}

// regraL: l final de sílaba → w (vocalização do PB). INFERRED.
func regraL(c, nx rune, isVogalNx bool) ([]Phoneme, int) {
	if c != 'l' {
		return nil, 0
	}
	if nx == 0 || !isVogalNx {
		return []Phoneme{{Symbol: "w", Evid: EvidenceInferred}}, 1
	}
	return []Phoneme{{Symbol: "l", Evid: EvidenceFact}}, 1
}

// regraH: h é mudo (exceto ch/lh/nh tratados antes). FACT.
func regraH(c rune) ([]Phoneme, int) {
	if c == 'h' {
		return []Phoneme{}, 1
	}
	return nil, 0
}