// Package vector — dot int16 AVX2 (FASE 7, L347).
//
// O kernel dot16AVX2 soma produtos i16×i16 em i32 (VPMADDWD), sem bias —
// diferente do int8 (que precisa do bias q+128). Escala 2^10 (1024): o
// produto máximo por elemento 1023² = 1.046M; a SOMA TOTAL dim×1023² ≤ 2^31
// exige dim ≤ 2048 (o acumulador tem 8 lanes i32 e a redução final soma tudo).
// Resolução 10 bits —
// 16× melhor que int8, com 2 bytes/elem (2× a bandwidth).

//go:build amd64 && !purego

package vector

import "math"

// dot16AVX2 soma produtos i16×i16 (32 por passo, YMM). Requer n%32==0, n>0.
func dot16AVX2(a *int16, b *int16, n int) int32

// dot16Pure — referência portátil (i64, sem estouro).
func dot16Pure(a, b []int16) int64 {
	var acc int64
	for i := range a {
		acc += int64(a[i]) * int64(b[i])
	}
	return acc
}

// dot16 computa Σa·b com o kernel AVX2 quando n%32==0 (i32), senão puro (i64).
// Escala 2^10: quem chama divide por (1024²·‖q‖·‖r‖) para o score de cosseno.
func dot16(a, b []int16) int64 {
	if len(a) > 0 && len(a)%32 == 0 {
		return int64(dot16AVX2(&a[0], &b[0], len(a)))
	}
	return dot16Pure(a, b)
}

// quantize16 converte float32 para int16 na grade 2^10: round(v·1024) clampado.
func quantize16(v float32) int16 {
	f := float64(v) * 1024
	if f >= 1024 {
		return 1023
	}
	if f <= -1024 {
		return -1023
	}
	return int16(math.Round(f))
}
