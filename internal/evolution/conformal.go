// conformal.go — Conformal Prediction (gema #4, ADR-022; mineração RuVector).
//
// Calibra a CONFIANÇA de uma decisão/recuperação ("quão certo é este hit"),
// determinístico (I1) e sem LLM. Recomenda/PREDIZ; o Cosca decide o que aceitar.
//
// Dado um conjunto de calibração (scores de hits "corretos"), computa-se um
// limiar conformal; um novo score é "coberto" se está acima dele. A p-value
// conformal dá uma SIGNIFICÂNCIA honesta (rank-based, exchangeability).
package evolution

import (
	"sort"
)

// ConformalThreshold devolve o quantil (1-alpha) dos scores de calibração —
// o limiar abaixo do qual um novo score é considerado ANÔMALO. Determinístico.
func ConformalThreshold(calibration []float64, alpha float64) float64 {
	if len(calibration) == 0 {
		return 0
	}
	if alpha <= 0 || alpha >= 1 {
		alpha = 0.05
	}
	cpy := append([]float64(nil), calibration...)
	sort.Float64s(cpy)
	q := alpha * float64(len(cpy)-1)
	lo := int(q)
	hi := int(q) + 1
	if hi >= len(cpy) {
		return cpy[len(cpy)-1]
	}
	frac := q - float64(lo)
	return cpy[lo]*(1-frac) + cpy[hi]*frac
}

// ConformalPValue devolve a p-value conformal de um novo score vs a calibração.
// p = (1 + #{s_cal >= score}) / (n+1). p <= alpha → o score é raro/anômalo.
// Determinstico (I1).
func ConformalPValue(calibration []float64, newScore float64) float64 {
	n := len(calibration)
	if n == 0 {
		return 0
	}
	count := 0
	for _, s := range calibration {
		if s >= newScore {
			count++
		}
	}
	return float64(1+count) / float64(n+1)
}

// Covers informa se um novo score está DENTRO da região conformal (não anômalo).
// p > alpha → coberto (confiável). p <= alpha → fora (anômalo).
func Covers(calibration []float64, newScore, alpha float64) bool {
	return ConformalPValue(calibration, newScore) > alpha
}
