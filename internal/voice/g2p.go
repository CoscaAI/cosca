package voice

// ── G2P: orquestrador do pipeline grafema→fonema ─────────────────────────
// Diretriz do Professor: cada estágio produz evidência própria.
//
//	TEXT → TOKENIZE → LEXICON → RULES → STRESS → PHONOLOGY → SAMPA
//
// O léxico responde primeiro (evidência LEXICON); o que não está no
// léxico passa pelas regras (FACT/INFERRED/FALLBACK); a tonicidade é
// marcada por acento gráfico (prioridade) ou regra oxítona/paroxítona.
// Pausas entre palavras são Phonemes DurPause — o planner de prosódia
// decide durações depois.

import (
	"strings"
)

// Tokenize: divide o texto em palavras (preserva ordem e pontuação).
func Tokenize(texto string) []string {
	return strings.FieldsFunc(strings.ToLower(texto), func(r rune) bool {
		return !isLetraPalavra(r)
	})
}

func isLetraPalavra(r rune) bool {
	return (r >= 'a' && r <= 'z') || r == 'ã' || r == 'õ' ||
		r == 'á' || r == 'é' || r == 'í' || r == 'ó' || r == 'ú' ||
		r == 'â' || r == 'ê' || r == 'ô' || r == 'à' || r == '\''
}

// fonemasPorRegras: aplica as regras ortográficas letra a letra.
// Trabalha com []rune (correção UTF-8 do Professor: ã, é, ô etc.).
func fonemasPorRegras(palavra []rune) []Phoneme {
	var out []Phoneme
	n := len(palavra)
	for i := 0; i < n; {
		c := palavra[i]
		var nx, nxx rune
		if i+1 < n {
			nx = palavra[i+1]
		}
		if i+2 < n {
			nxx = palavra[i+2]
		}

		// dígrafos e consoantes contextuais
		switch {
		case c == 'c' && nx == 'h':
			out = append(out, Phoneme{Symbol: "S", Evid: EvidenceFact})
			i += 2
			continue
		case c == 'l' && nx == 'h':
			out = append(out, Phoneme{Symbol: "L", Evid: EvidenceFact})
			i += 2
			continue
		case c == 'n' && nx == 'h':
			out = append(out, Phoneme{Symbol: "J", Evid: EvidenceFact})
			i += 2
			continue
		case c == 'r' && nx == 'r':
			out = append(out, Phoneme{Symbol: "R", Evid: EvidenceFact})
			i += 2
			continue
		case c == 's' && nx == 's':
			out = append(out, Phoneme{Symbol: "s", Evid: EvidenceFact})
			i += 2
			continue
		case c == 'x' && nx == 'c' && (nxx == 'e' || nxx == 'i'):
			out = append(out, Phoneme{Symbol: "s", Evid: EvidenceFact})
			i += 2
			continue
		}

		// regras com consumo correto de m/n (correção do Professor)
		if res, adv := regraNasal(c, nx, nxx, nx != 0 && isVogalLetra(nx)); res != nil {
			out = append(out, res...)
			i += adv
			continue
		}

		if res, adv := regraC(c, nx, nxx); res != nil {
			out = append(out, res...)
			i += adv
			continue
		}
		if res, adv := regraG(c, nx, nxx); res != nil {
			out = append(out, res...)
			i += adv
			continue
		}
		if res, adv := regraQ(c, nx, nxx); res != nil {
			out = append(out, res...)
			i += adv
			continue
		}
		if res, adv := regraX(palavra, i); res != nil {
			out = append(out, res...)
			i += adv
			continue
		}
		if res, adv := regraR(palavra, i); res != nil {
			out = append(out, res...)
			i += adv
			continue
		}
		if res, adv := regraL(c, nx, nx != 0 && isVogalLetra(nx)); res != nil {
			out = append(out, res...)
			i += adv
			continue
		}
		if res, adv := regraH(c); res != nil {
			out = append(out, res...)
			i += adv
			continue
		}
		if res, adv := regraS(c, rune(0), nx, i > 0 && isVogalLetra(palavra[i-1]), nx != 0 && isVogalLetra(nx)); res != nil {
			out = append(out, res...)
			i += adv
			continue
		}

		// vogais acentuadas (com base fonética correta)
		if base, ok := acentoParaBase[c]; ok {
			nasal := strings.HasSuffix(base, "~")
			out = append(out, Phoneme{
				Symbol: strings.TrimSuffix(base, "~"),
				Nasal:  nasal,
				Dur:    durClass(strings.TrimSuffix(base, "~"), nasal, false),
				Evid:   EvidenceFact,
			})
			i++
			continue
		}
		// vogais simples e consoantes
		if isVogalLetra(c) {
			out = append(out, Phoneme{Symbol: string(c), Evid: EvidenceFact})
			i++
			continue
		}
		switch c {
		case 'b', 'd', 'f', 'j', 'k', 'm', 'p', 't', 'v', 'z':
			sym := string(c)
			if c == 'j' {
				sym = "Z"
			}
			out = append(out, Phoneme{Symbol: sym, Evid: EvidenceFact})
		case 'g', 'l', 'n':
			out = append(out, Phoneme{Symbol: string(c), Evid: EvidenceFact})
		case 'q':
			out = append(out, Phoneme{Symbol: "k", Evid: EvidenceFact})
		case 'w':
			out = append(out, Phoneme{Symbol: "w", Evid: EvidenceFallback})
		case 'y':
			out = append(out, Phoneme{Symbol: "i", Evid: EvidenceFallback})
		default:
			// desconhecido: FALLBACK explícito (nunca silêncio)
			out = append(out, Phoneme{Symbol: string(c), Evid: EvidenceFallback})
		}
		i++
	}
	return out
}

// reducaoVocalica: fonologia do PB — vogais átonas finais reduzem:
// o→u, e→i (ex.: "campo" → 'a~ p u, "mesa" → m 'e z a não reduz 'a').
// Aplica só em vogal NÃO tônica (a tônica nunca reduz).
func reducaoVocalica(fonemas []Phoneme) {
	if len(fonemas) == 0 {
		return
	}
	// acha a última vogal da palavra
	for i := len(fonemas) - 1; i >= 0; i-- {
		f := fonemas[i]
		if !f.IsVowel() || f.Stress || f.Nasal {
			if !f.IsVowel() {
				continue
			}
			if f.Stress || f.Nasal {
				break // tônica/nasal no fim: nada a reduzir
			}
		}
		switch f.Symbol {
		case "o":
			fonemas[i].Symbol = "u"
			fonemas[i].Dur = DurVowelShort
		case "e":
			fonemas[i].Symbol = "i"
			fonemas[i].Dur = DurVowelShort
		}
		break
	}
}

// G2P: pipeline completo → []Phoneme com pausas entre palavras.
// Compatível com a API anterior (SAMPA string), mas retorna estrutura rica.
func G2P(texto string) []Phoneme {
	var out []Phoneme
	palavras := Tokenize(texto)
	for i, p := range palavras {
		if i > 0 {
			out = append(out, Phoneme{Symbol: " ", Dur: DurPause, Evid: EvidenceFact})
		}
		rs := []rune(strings.ToLower(strings.Trim(p, ".,;:!?()\"'-")))
		if len(rs) == 0 {
			continue
		}
		// 1) léxico primeiro (evidência LEXICON)
		if lex, ok := Consultar(string(rs)); ok {
			out = append(out, lex...)
			continue
		}
		// 2) regras
		fonemas := fonemasPorRegras(rs)
		// 3) tonicidade (acento gráfico > regra)
		marcarTonicidade(rs, fonemas)
		// 4) fonologia: redução vocálica átona
		reducaoVocalica(fonemas)
		out = append(out, fonemas...)
	}
	return out
}

// G2PSAMPA: API de compatibilidade — []Phoneme → []string SAMPA
// (usada pelo demo e pelo synth).
func G2PSAMPA(texto string) []string {
	fonemas := G2P(texto)
	out := make([]string, 0, len(fonemas))
	for _, f := range fonemas {
		out = append(out, f.SAMPA())
	}
	return out
}