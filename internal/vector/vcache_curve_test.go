// Package vector — campanha de performance, FASE 3 (L345).
//
// Curva do V-Cache: throughput do fast path int8 por tamanho de dataset,
// de 10k a 1M vetores (dim 768). O objetivo é achar o JOELHO da curva — o
// ponto onde o slab deixa de caber no L3 (96MB nominal) e o regime muda de
// cache para RAM. Esse joelho vira o cache_budget EXPERIMENTAL do Cosca
// (não o datasheet de 96MB — epistemologia FACT → MEASURED → DECISION).
//
// Oráculos de referência (não alterados): int8/1M = 48.63 Mvec/s (RAM),
// int8/100k = 80.82 Mvec/s (L3). Este benchmark usa um generation SINTÉTICO
// mínimo (só os slabs do score8), então os números absolutos podem diferir
// levemente do índice real — o que importa é a FORMA da curva e o joelho.
package vector

import (
	"fmt"
	"testing"
)

// vcacheSizes — os tamanhos da curva (slab int8: N×768 bytes).
// 10k=7.7MB · 20k=15.4 · 40k=30.7 · 60k=46.1 · 80k=61.4 · 100k=76.8
// 120k=92.2 · 150k=115.2 · 200k=153.6 · 250k=192 · 500k=384 · 1M=768MB
var vcacheSizes = []int{10_000, 20_000, 40_000, 60_000, 80_000, 100_000, 120_000, 150_000, 200_000, 250_000, 500_000, 1_000_000}

// synthGen8 monta um generation int8 mínimo para o score8: rows com padrão
// determinístico (mesma fórmula do dot8_test), sums e nb computados, ids
// sintéticos. NÃO cria o slab float32 (o score8 não usa) — economiza 4x RAM.
func synthGen8(n, dim int) *indexGeneration {
	dimPad := (dim + 15) &^ 15
	vecs8 := make([]int8, n*dimPad)
	sums8 := make([]int32, n)
	nb := make([]float32, n)
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		base := i * dimPad
		var sum int32
		var nbsq float64
		for d := 0; d < dim; d++ {
			v := float64((d*7+3)%255) - 127 // v ∈ [-127, 127]
			q := int8(v)
			vecs8[base+d] = q
			sum += int32(q)
			nbsq += v * v
		}
		sums8[i] = sum
		nb[i] = float32(nbsq)
		ids[i] = fmt.Sprintf("r%d", i)
	}
	return &indexGeneration{
		ids:     ids,
		vecs8:   vecs8,
		sums8:   sums8,
		nb:      nb,
		dim:     dim,
		dimPad:  dimPad,
		n:       n,
	}
}

// synthQuery8 gera uma query int8 determinística (mesmo padrão das rows).
func synthQuery8(dim int) []float64 {
	q := make([]float64, dim)
	for d := 0; d < dim; d++ {
		q[d] = float64((d*13+7)%255) - 127
	}
	return q
}

// BenchmarkVCacheCurve_N — a curva completa do V-Cache (FASE 3).
// Cada tamanho: geração sintética + aquecimento (3 iterações fora do timer —
// páginas quentes, lição L340) + medição. Reporta MB do slab, Mvec/s e GB/s.
func BenchmarkVCacheCurve_N(b *testing.B) {
	if testing.Short() {
		b.Skip("curva completa em short mode")
	}
	const dim, limit, workers = 768, 10, 16
	for _, n := range vcacheSizes {
		b.Run(fmt.Sprintf("N%d", n), func(b *testing.B) {
			gen := synthGen8(n, dim)
			q := synthQuery8(dim)
			// Aquecimento: páginas quentes antes de medir (L340).
			for i := 0; i < 3; i++ {
				_ = gen.score8(q, limit)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = gen.score8(q, limit)
			}
			slabMB := float64(n*dim) / (1 << 20)
			vecPerSec := float64(n) * float64(b.N) / b.Elapsed().Seconds()
			b.ReportMetric(slabMB, "MB-slab")
			b.ReportMetric(vecPerSec/1e6, "Mvec/s")
			b.ReportMetric(vecPerSec*float64(dim)/1e9, "GB/s")
			b.SetBytes(int64(n * dim))
		})
	}
}