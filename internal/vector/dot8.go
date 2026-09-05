// Package vector — dot int8 com bias (L339).
//
// O kernel AVX2 (dot8_amd64.s) computa Σ qb·rb com qb unsigned (q+128) e rb
// signed. A função dot8BiasDot aplica a correção −128·Σr, devolvendo o dot
// exato em int32. n % 32 != 0 cai no caminho puro (portátil).

//go:build amd64 && !purego

package vector

// dot8BiasAVX2 soma produtos i8 (qb unsigned × rb signed), 32 por passo.
// Implementação em dot8_amd64.s. Requer n % 32 == 0 e n > 0.
func dot8BiasAVX2(qb *uint8, rb *int8, n int) uint32

// dot8BiasPure — referência portátil (mesma semântica: qb unsigned × rb signed).
func dot8BiasPure(qb []uint8, rb []int8) int64 {
	var acc int64
	for i := range qb {
		acc += int64(qb[i]) * int64(rb[i])
	}
	return acc
}

// dot8BiasDot computa dot(q, r) exato a partir da representação com bias:
// qb[i] = q[i]+128, rb = r, sumR = Σ r. Corrige −128·sumR.
func dot8BiasDot(qb []uint8, rb []int8, sumR int32) int32 {
	if len(qb) > 0 && len(qb)%16 == 0 {
		return int32(dot8BiasAVX2(&qb[0], &rb[0], len(qb))) - 128*sumR
	}
	return int32(dot8BiasPure(qb, rb) - 128*int64(sumR))
}
