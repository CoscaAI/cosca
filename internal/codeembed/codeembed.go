// Package codeembed — embedding de CÓDIGO determinístico (F1 do ADR-019).
//
// Mineração codebase-memory-mcp: "busca semântica REAL sobre código com custo
// ZERO-LLM e zero rede". Aqui o Cosca produz um letor de código SEM modelo e
// SEM rede — puramente determinístico (I1), fail-closed (I2).
//
// Técnica: signed feature-hashing sobre tokens de código (camelCase/snake-aware)
// → distribui cada token em um bucket de dimensão fixa com sinal, normaliza L2.
// É determinístico (mesmo input → mesmo vetor), sem LLM, sem blob pré-treinado.
// A qualidade é LEXICAL/estrutural (base para a fusão de sinais do F2).
//
// Compõe com o substrato existente: produz []float64 compatível com
// internal/vector.Store (VectorRecord.Vector) e a int8 via QuantizeInt8 (a
// mesma linha do dot8/dot16 SIMD int8 do internal/vector).
package codeembed

import (
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

// DefaultDim é a dimensão padrão do vetor de código (potência de 2 para que o
// bucket de hashing distribua bem).
const DefaultDim = 512

// Tokenize extrai tokens de código: identificadores (camelCase/snake
// divididos), números e palavras, minúsculos. Ignora operadores/pontuação de
// um char (ruído semântico) e separa contexto neles. V1: aproximação por
// caracteres não-letra/dígito.
func Tokenize(code string) []string {
	var tokens []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			tokens = append(tokens, strings.ToLower(string(cur)))
			cur = cur[:0]
		}
	}
	for _, r := range []rune(code) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			// Quebra em rótulo camelCase: minúscula → maiúscula.
			if len(cur) > 0 && unicode.IsLower(cur[len(cur)-1]) && unicode.IsUpper(r) {
				flush()
			}
			cur = append(cur, r)
		case r == '_':
			flush()
		case unicode.IsSpace(r):
			flush()
		default:
			// Operador/pontuação: separa contexto (não vira token).
			flush()
		}
	}
	flush()
	return tokens
}

// Embed produz um vetor determinístico de dimensão `dim` para o trecho de
// código, normalizado (L2). `dim` <= 0 usa DefaultDim. Determinístico (I1):
// nenhuma aleatoriedade, nenhum modelo, nenhuma rede.
func Embed(code string, dim int) []float64 {
	if dim <= 0 {
		dim = DefaultDim
	}
	vec := make([]float64, dim)
	for _, tok := range Tokenize(code) {
		h := fnv32(tok)
		bucket := int(h) % dim
		sign := 1.0
		if h&(1<<31) != 0 {
			sign = -1.0
		}
		// peso = tamanho informativo do token (mais de uma letra vale mais).
		weight := 1.0
		if len(tok) > 3 {
			weight = 2.0
		}
		vec[bucket] += sign * weight
	}
	return Normalize(vec)
}

// Similarity devolve a similaridade de cosseno entre dois embeddings (mesma
// dimensão). Dimensões diferentes devolvem 0.
func Similarity(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// Normalize devolve o vetor normalizado (L2). Vetor zero permanece zero.
func Normalize(v []float64) []float64 {
	var norm float64
	for _, x := range v {
		norm += x * x
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		return v
	}
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = x / norm
	}
	return out
}

// QuantizeInt8 converte um vetor float64 ([−1,1]) em int8 (escala −127..127),
// representação consumida pelo caminho dot8/dot16 SIMD do internal/vector.
// Determinístico.
func QuantizeInt8(v []float64) []int8 {
	out := make([]int8, len(v))
	for i, x := range v {
		scale := x * 127
		out[i] = int8(math.Round(scale))
	}
	return out
}

// fnv32 é um hash estável (não muda entre execuções → I1). FNV-1a 32 bit.
func fnv32(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}
