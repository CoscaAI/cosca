// Package vector — fallback portátil do dot int8 (sem AVX2).

//go:build !amd64 || purego

package vector

// dot8BiasPure — referência portátil (qb unsigned × rb signed).
func dot8BiasPure(qb []uint8, rb []int8) int64 {
	var acc int64
	for i := range qb {
		acc += int64(qb[i]) * int64(rb[i])
	}
	return acc
}

// dot8BiasDot computa dot(q, r) exato a partir da representação com bias.
func dot8BiasDot(qb []uint8, rb []int8, sumR int32) int32 {
	return int32(dot8BiasPure(qb, rb) - 128*int64(sumR))
}
