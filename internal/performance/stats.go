package performance

import (
	"math"
	"sort"
)

// =============================================================================
// Estatística — funções puras para agregação de medições (L316: nunca declarar
// vitória por uma execução; usar mean/stddev/percentis).
// =============================================================================

func mean(v []int64) int64 {
	if len(v) == 0 {
		return 0
	}
	var sum int64
	for _, x := range v {
		sum += x
	}
	return sum / int64(len(v))
}

func min(v []int64) int64 {
	if len(v) == 0 {
		return 0
	}
	m := v[0]
	for _, x := range v[1:] {
		if x < m {
			m = x
		}
	}
	return m
}

func max(v []int64) int64 {
	if len(v) == 0 {
		return 0
	}
	m := v[0]
	for _, x := range v[1:] {
		if x > m {
			m = x
		}
	}
	return m
}

// stddev calcula o desvio padrão populacional (n, não n-1 — amostra pequena
// não justifica correção de Bessel para esta régua).
func stddev(v []int64) float64 {
	if len(v) < 2 {
		return 0
	}
	m := mean(v)
	var sumSq float64
	for _, x := range v {
		d := float64(x - m)
		sumSq += d * d
	}
	return math.Sqrt(sumSq / float64(len(v)))
}

// percentile devolve o valor na posição p (0..1) após ordenar — interpolação
// linear entre os vizinhos mais próximos (régua robusta para p95/p99).
func percentile(v []int64, p float64) int64 {
	if len(v) == 0 {
		return 0
	}
	if len(v) == 1 {
		return v[0]
	}
	sorted := make([]int64, len(v))
	copy(sorted, v)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	pos := p * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	frac := pos - float64(lo)
	// math.Round evita truncamento por imprecisão de float (0.8*10 = 7.9999...).
	return sorted[lo] + int64(math.Round(frac*float64(sorted[hi]-sorted[lo])))
}
