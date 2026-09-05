// F2 do ADR-019 — FUSÃO DE SINAIS para similaridade de código (sem LLM).
//
// O Embed (F1) é um sinal: cosseno lexical sobre feature-hashing. Um único
// sinal é fraco (colisões de hash, perde ordem). Aqui compomos VÁRIOS sinais
// independentes em uma só pontuação, todos determinísticos (I1):
//
//	s1 — cosseno do embedding lexical (unigram)      (F1)
//	s2 — cosseno do embedding por BIGRAMA            (ordem de tokens)
//	s3 — Jaccard via MinHash (conjunto de shingles)  (robusto a colisão)
//
// FuseSimilarity combina com pesos (default 0.4/0.3/0.3). Determinístico
// (I1), zero LLM, zero rede. A saída é um score [0,1].
package codeembed

import (
	"hash/fnv"
)

// minHashSeeds são seeds fixos (determinismo I1) para as k funções de minhash.
var minHashSeeds = makeMinHashSeeds(64)

func makeMinHashSeeds(k int) []uint32 {
	out := make([]uint32, k)
	for i := range out {
		// Seeds primos-ish, fixos (nunca aleatório).
		out[i] = 0x9E3779B9 * uint32(i+1) * 2654435761
	}
	return out
}

// Shingles devolve os shingles de 1 e 2 gramas dos tokens (ordem preservada).
func shingles(tokens []string) []string {
	sh := make([]string, 0, len(tokens)*2)
	for i := 0; i < len(tokens); i++ {
		sh = append(sh, tokens[i])
		if i+1 < len(tokens) {
			sh = append(sh, tokens[i]+"\x00"+tokens[i+1])
		}
	}
	return sh
}

// MinHashSketch devolve a assinatura MinHash de um texto (lista de tokens).
// Assinatura de comprimento fixo; a estimativa de Jaccard entre dois textos é
// a fração de posições iguais. Determinístico (seeds fixo).
func MinHashSketch(code string) []uint32 {
	tokens := Tokenize(code)
	sh := shingles(tokens)
	sig := make([]uint32, len(minHashSeeds))
	for i, seed := range minHashSeeds {
		m := uint32(^uint32(0))
		for _, s := range sh {
			h := fnv32Seeded(seed, s)
			if h < m {
				m = h
			}
		}
		sig[i] = m
	}
	return sig
}

// Jaccard estima a similaridade de conjunto (Jaccard) entre duas assinaturas
// MinHash: fração de posições iguais em [0,1]. Assinaturas de tamanhos
// diferentes devolvem 0.
func Jaccard(a, b []uint32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	eq := 0
	for i := range a {
		if a[i] == b[i] {
			eq++
		}
	}
	return float64(eq) / float64(len(a))
}

// BigramEmbed é o embedding de F1 mas por BIGRAMAS (unigram + bigram) — captura
// a ordem dos tokens, complementando o unigram.
func BigramEmbed(code string, dim int) []float64 {
	if dim <= 0 {
		dim = DefaultDim
	}
	vec := make([]float64, dim)
	for _, sh := range shingles(Tokenize(code)) {
		h := fnv32(sh)
		bucket := int(h) % dim
		sign := 1.0
		if h&(1<<31) != 0 {
			sign = -1.0
		}
		vec[bucket] += sign
	}
	return Normalize(vec)
}

// FuseSimilarity combina os 3 sinais (unigram, bigram, MinHash) numa pontuação
// ponderada [0,1]. `weights` com 3 entradas opcionais (default 0.4/0.3/0.3).
func FuseSimilarity(a, b string, dim int, weights ...float64) float64 {
	s1 := Similarity(Embed(a, dim), Embed(b, dim))
	s2 := Similarity(BigramEmbed(a, dim), BigramEmbed(b, dim))
	s3 := Jaccard(MinHashSketch(a), MinHashSketch(b))

	w := [3]float64{0.4, 0.3, 0.3}
	if len(weights) >= 3 {
		copy(w[:], weights[:3])
	}
	return clamp01(s1*w[0] + s2*w[1] + s3*w[2])
}

// Clamp / normalização de fusão.
func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// fnv32Seeded é FNV-1a com seed no início (para as k funções de minhash).
func fnv32Seeded(seed uint32, s string) uint32 {
	h := fnv.New32a()
	// Mistura o seed com os primeiros bytes para distinguir as k funções.
	_, _ = h.Write([]byte{byte(seed), byte(seed >> 8), byte(seed >> 16), byte(seed >> 24)})
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}
