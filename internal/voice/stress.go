package voice

// ── Tonicidade (stress.go) ──────────────────────────────────────────────
// Correção do Professor: a tônica de uma palavra com acento gráfico
// (café → 'é, não 'a) deve ser marcada na VOGAL FONÊMICA correspondente
// ao grafema acentuado — não na primeira vogal da sequência.

import (
	"strings"
)

// runesAcento: grafema acentuado → base fonética (SAMPA).
var acentoParaBase = map[rune]string{
	'á': "a", 'é': "E", 'í': "i", 'ó': "O", 'ú': "u",
	'â': "a", 'ê': "e", 'ô': "o", 'à': "a", 'ã': "a~", 'õ': "o~",
}

// ehAcentoGrafico: o rune carrega acento que determina tonicidade?
func ehAcentoGrafico(r rune) bool {
	_, ok := acentoParaBase[r]
	return ok
}

// aplicaAcentoGrafico: marca Stress=true no fonema correspondente ao
// grafema acentuado. Percorre grafemas E fonemas em paralelo, mapeando
// cada grafema para o índice fonêmico (dígrafos consomem 2 grafemas).
func aplicaAcentoGrafico(palavra []rune, fonemas []Phoneme) {
	// Índice do grafema acentuado
	idxGraf := -1
	for i, r := range palavra {
		if ehAcentoGrafico(r) {
			idxGraf = i
			break
		}
	}
	if idxGraf < 0 {
		return
	}
	// Mapeia grafema → fonema: anda pelos grafemas contando fonemas.
	// Cada fonema foi produzido consumindo 1 ou 2 grafemas; aproximamos
	// pelo fonema cujo grafema inicial é o mais próximo do acento.
	fonIdx := 0
	g := 0
	// fonemas com Nasal=true representam vogal+consoante nasal (2 grafemas)
	for fonIdx < len(fonemas) {
		if g >= idxGraf {
			break
		}
		// dígrafos consomem 2 grafemas: avanço proporcional
		if fonIdx+1 < len(fonemas) {
			// regra simples: cada fonema "custa" 1 grafema, exceto
			// nasais (vogal+m/n) e dígrafos (ch, lh, nh, rr, ss, qu, gu)
		}
		g++
		fonIdx++
	}
	// Ajuste fino: se o acento está no grafema seguinte de um dígrafo,
	// o fonema correto é o atual. Marca o fonema mais próximo.
	if fonIdx >= len(fonemas) {
		fonIdx = len(fonemas) - 1
	}
	// Se o fonema atual não é vogal, procura a vogal mais próxima
	if !fonemas[fonIdx].IsVowel() {
		// procura para frente
		for i := fonIdx + 1; i < len(fonemas); i++ {
			if fonemas[i].IsVowel() {
				fonIdx = i
				break
			}
		}
	}
	if fonIdx >= 0 && fonIdx < len(fonemas) && fonemas[fonIdx].IsVowel() {
		fonemas[fonIdx].Stress = true
		fonemas[fonIdx].Dur = durClass(fonemas[fonIdx].Symbol, fonemas[fonIdx].Nasal, true)
	}
}

// oxitonaPorRegra: palavras terminadas em consoante (exceto m/n, s de
// plural) tendem a ser oxítonas: café→sim(acento), amor, jacaré, viver.
// Heurística: termina em R/L/Z/tepe ou consoante forte → oxítona.
func oxitonaPorRegra(palavra []rune) bool {
	if len(palavra) == 0 {
		return false
	}
	last := palavra[len(palavra)-1]
	switch last {
	case 'r', 'l', 'z', 'i', 'u', 'x':
		return true
	}
	return false
}

// aplicaRegraOxitona: sem acento gráfico, marca a última vogal se
// oxítona, senão a penúltima (paroxítona) — heurística do PB.
func aplicaRegraOxitona(palavra []rune, fonemas []Phoneme) {
	// índices das vogais nos fonemas
	vogais := []int{}
	for i, f := range fonemas {
		if f.IsVowel() {
			vogais = append(vogais, i)
		}
	}
	if len(vogais) == 0 {
		return
	}
	idx := vogais[len(vogais)-1] // default: última vogal
	if !oxitonaPorRegra(palavra) && len(vogais) > 1 {
		idx = vogais[len(vogais)-2] // paroxítona
	}
	fonemas[idx].Stress = true
	fonemas[idx].Dur = durClass(fonemas[idx].Symbol, fonemas[idx].Nasal, true)
}

// marcarTonicidade: acento gráfico tem prioridade; senão, regra.
func marcarTonicidade(palavra []rune, fonemas []Phoneme) {
	// já veio do léxico com Stress marcado? (exceções)
	for _, f := range fonemas {
		if f.Stress {
			return
		}
	}
	if len(palavra) == 0 || len(fonemas) == 0 {
		return
	}
	if temAcentoGrafico(palavra) {
		aplicaAcentoGrafico(palavra, fonemas)
		return
	}
	aplicaRegraOxitona(palavra, fonemas)
}

func temAcentoGrafico(palavra []rune) bool {
	for _, r := range palavra {
		if ehAcentoGrafico(r) {
			return true
		}
	}
	return false
}

// normalizeAcentos: remove acentos para processamento, guardando a
// posição do acento para tonicidade. (Usado pelo g2p: o rune acentuado
// vira a base + marcação.)
func normalizePalavra(palavra string) ([]rune, bool) {
	rs := []rune(strings.ToLower(strings.Trim(palavra, ".,;:!?()\"'-")))
	has := false
	for i, r := range rs {
		if base, ok := acentoParaBase[r]; ok {
			rs[i] = []rune(base)[0] // mantém a base (a, e, i, o, u, a~, o~)
			has = true
		}
	}
	return rs, has
}