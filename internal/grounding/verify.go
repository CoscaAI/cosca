package grounding

import (
	"math"
	"regexp"
	"strings"
)

// VerifyOptions controla a verificação heurística de um claim contra os chunks.
// Valores zero caem nos defaults documentados (o pacote é determinístico).
type VerifyOptions struct {
	// Threshold é a pontuação mínima para um claim ser considerado
	// "supported". Zero/negativo cai em DefaultVerifyThreshold.
	Threshold float64
	// NGramBonus é o peso do bônus de bigramas (n-gramas de ordem 2).
	// Zero cai em DefaultNGramBonus.
	NGramBonus float64
	// NumberBonus é o peso do bônus de números (termos numéricos do claim
	// presentes no chunk). Zero cai em DefaultNumberBonus.
	NumberBonus float64
}

// Defaults de verificação (constantes públicas para o chamador calibrar).
const (
	// DefaultVerifyThreshold é a pontuação mínima de suporte de um claim.
	DefaultVerifyThreshold = 0.40
	// DefaultNGramBonus é o peso default do bônus de bigramas.
	DefaultNGramBonus = 0.15
	// DefaultNumberBonus é o peso default do bônus de números.
	DefaultNumberBonus = 0.10
)

// tokenRe extrai "palavras e números" como tokens (Unicode letters+digits),
// ignorando pontuação e símbolos. A caixa é normalizada para minúsculas.
var tokenRe = regexp.MustCompile(`[\p{L}\p{N}]+`)

// numberTokenRe identifica um token que é puramente numérico.
var numberTokenRe = regexp.MustCompile(`^\p{N}+$`)

// EffectiveVerifyOptions resolve os defaults de um VerifyOptions zero/parcial.
func EffectiveVerifyOptions(o VerifyOptions) VerifyOptions {
	if o.Threshold <= 0 {
		o.Threshold = DefaultVerifyThreshold
	}
	if o.NGramBonus <= 0 {
		o.NGramBonus = DefaultNGramBonus
	}
	if o.NumberBonus <= 0 {
		o.NumberBonus = DefaultNumberBonus
	}
	return o
}

// tokenize normaliza um texto em uma lista de tokens (lowercase, sem
// pontuação). Determinística e sem LLM.
func tokenize(s string) []string {
	raw := tokenRe.FindAllString(strings.ToLower(s), -1)
	toks := make([]string, 0, len(raw))
	for _, t := range raw {
		if t != "" {
			toks = append(toks, t)
		}
	}
	return toks
}

// tokenSet devolve um set de tokens (para interseção O(1)).
func tokenSet(toks []string) map[string]struct{} {
	set := make(map[string]struct{}, len(toks))
	for _, t := range toks {
		set[t] = struct{}{}
	}
	return set
}

// bigrams devolve os bigramas consecutivos de uma lista de tokens. Menos de 2
// tokens produz zero bigramas.
func bigrams(toks []string) [][2]string {
	if len(toks) < 2 {
		return nil
	}
	out := make([][2]string, 0, len(toks)-1)
	for i := 0; i < len(toks)-1; i++ {
		out = append(out, [2]string{toks[i], toks[i+1]})
	}
	return out
}

// bigramSet devolve um set das chaves "a|b" dos bigramas.
func bigramSet(grams [][2]string) map[string]struct{} {
	set := make(map[string]struct{}, len(grams))
	for _, g := range grams {
		set[g[0]+"|"+g[1]] = struct{}{}
	}
	return set
}

// numericTokens devolve apenas os tokens que são puramente numéricos.
func numericTokens(toks []string) []string {
	out := make([]string, 0)
	for _, t := range toks {
		if numberTokenRe.MatchString(t) {
			out = append(out, t)
		}
	}
	return out
}

// VerifyClaim verifica UM claim contra os chunks recuperados usando
// token-overlap com bônus de n-gramas (bigramas) e de números.
//
// Para cada chunk calculamos uma pontuação:
//
//	score = baseCoverage + NGramBonus*bigramCoverage + NumberBonus*numberCoverage
//
// em que:
//   - baseCoverage  = |claimTokens ∩ chunkTokens| / |claimTokens|
//   - bigramCoverage= |claimBigrams ∩ chunkBigrams| / |claimBigrams|
//   - numberCoverage= |claimNumbers ∩ chunkNumbers| / |claimNumbers|
//
// O melhor chunk (maior score) é escolhido; supported = score >= Threshold.
// Quando nenhum chunk tem overlap, BestChunkIdx = -1, Score = 0 e Supported =
// false. Tudo é determinístico (nada de LLM, nada de tempo).
func VerifyClaim(query string, claim Claim, chunks []SourceChunk, opts VerifyOptions) ClaimVerdict {
	opts = EffectiveVerifyOptions(opts)

	v := ClaimVerdict{
		Claim:        claim,
		BestChunkIdx: -1,
		Score:        0,
		Supported:    false,
		MatchingTerms: nil,
	}

	claimTokens := tokenize(claim.Text)
	if len(claimTokens) == 0 {
		v.Claim.Substantiated = false
		return v
	}
	baseDenom := float64(len(claimTokens))

	grams := bigrams(claimTokens)
	var gramDenom float64
	if len(grams) > 0 {
		gramDenom = float64(len(grams))
	}
	numTokens := numericTokens(claimTokens)
	var numDenom float64
	if len(numTokens) > 0 {
		numDenom = float64(len(numTokens))
	}

	var bestScore float64
	var bestTerms []string

	for i, ch := range chunks {
		chunkTokens := tokenize(ch.Content)
		if len(chunkTokens) == 0 {
			continue
		}
		chSet := tokenSet(chunkTokens)
		chGrams := bigramSet(bigrams(chunkTokens))
		chNums := tokenSet(numericTokens(chunkTokens))

		// baseCoverage: fração dos tokens do claim presentes no chunk.
		var matched float64
		var matchedTerms []string
		for _, t := range claimTokens {
			if _, ok := chSet[t]; ok {
				matched++
				matchedTerms = append(matchedTerms, t)
			}
		}
		baseCoverage := matched / baseDenom

		// bigramCoverage: fração dos bigramas do claim presentes no chunk.
		var gramMatch float64
		if gramDenom > 0 {
			for _, g := range grams {
				if _, ok := chGrams[g[0]+"|"+g[1]]; ok {
					gramMatch++
				}
			}
		}
		var bigramCoverage float64
		if gramDenom > 0 {
			bigramCoverage = gramMatch / gramDenom
		}

		// numberCoverage: fração dos números do claim presentes no chunk.
		var numMatch float64
		if numDenom > 0 {
			for _, n := range numTokens {
				if _, ok := chNums[n]; ok {
					numMatch++
				}
			}
		}
		var numberCoverage float64
		if numDenom > 0 {
			numberCoverage = numMatch / numDenom
		}

		score := baseCoverage + opts.NGramBonus*bigramCoverage + opts.NumberBonus*numberCoverage
		if score > bestScore {
			bestScore = score
			bestTerms = matchedTerms
			v.BestChunkIdx = i
		}
	}

	v.Score = round4(bestScore)
	if v.Score >= opts.Threshold && v.BestChunkIdx >= 0 {
		v.Supported = true
	}
	v.MatchingTerms = bestTerms
	v.Claim.Substantiated = v.Supported
	return v
}

// round4 arredonda para 4 casas decimais (evita ruído de ponto flutuante).
func round4(f float64) float64 {
	return math.Round(f*10000) / 10000
}
