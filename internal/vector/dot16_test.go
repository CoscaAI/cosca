// Package vector — testes e benchmark do kernel dot16 AVX2 (FASE 7, L347).
package vector

import (
	"math/rand"
	"runtime"
	"testing"
)

// TestDot16AVX2Correctness valida o kernel contra a referência pura em dims
// variados (múltiplos de 32 — o contrato do kernel).
func TestDot16AVX2Correctness(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for _, dim := range []int{32, 64, 768, 1024, 1024} {
		a := make([]int16, dim)
		b := make([]int16, dim)
		for i := range a {
			a[i] = int16(rng.Intn(4097) - 1024)
			b[i] = int16(rng.Intn(4097) - 1024)
		}
		got := dot16(a, b)
		want := dot16Pure(a, b)
		if got != want {
			t.Fatalf("dim %d: got %d want %d", dim, got, want)
		}
	}
}

// TestDot16AVX2Extremes valida o limite de estouro: escala 1024, pares 2·1024².
func TestDot16AVX2Extremes(t *testing.T) {
	for _, dim := range []int{32, 768, 1024} {
		a := make([]int16, dim)
		b := make([]int16, dim)
		for i := range a {
			a[i], b[i] = 1023, 1023
		}
		got := dot16(a, b)
		want := dot16Pure(a, b)
		if got != want {
			t.Fatalf("extremes dim %d: got %d want %d (estouro i32?)", dim, got, want)
		}
	}
}

// TestQuantize16 valida a grade 2^11 e o clamp.
func TestQuantize16(t *testing.T) {
	for _, tc := range []struct {
		in   float32
		want int16
	}{
		{0, 0}, {0.5, 512}, {-0.5, -512}, {1, 1023}, {-1, -1023},
		{1.5, 1023}, {-2, -1023}, {1.0 / 1024, 1}, {-1.0 / 1024, -1},
	} {
		if got := quantize16(tc.in); got != tc.want {
			t.Errorf("quantize16(%v) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// ── LIMITE DA MÁQUINA — dataset 15M×768 int16 (23GB), 16 workers ──────────
// Compara o kernel int16 com os oráculos: int8 1M = 48.63 (RAM) / 100k = 80.82
// (L3). int16 esperado: ~metade (2 bytes/elem) em RAM, ~metade+ no L3.
func BenchmarkLimitInt16AVX2_N30M_Dim768(b *testing.B) {
	if testing.Short() {
		b.Skip("dataset de 23GB em modo curto")
	}
	const n, dim = 15_000_000, 768
	row0 := make([]int16, dim)
	for d := range row0 {
		row0[d] = int16(((d*7 + 3) % 255) - 127)
	}
	rows := make([]int16, n*dim)
	batch := make([]int16, dim*10000)
	for i := 0; i < 10000; i++ {
		copy(batch[i*dim:], row0)
	}
	for i := 0; i < n/10000; i++ {
		copy(rows[i*len(batch):], batch)
	}
	q := make([]int16, dim)
	for d := range q {
		q[d] = int16(((d*13 + 7) % 255) - 127)
	}

	const workers = 16
	chunk := n / workers

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		results := make(chan int32, workers)
		for w := 0; w < workers; w++ {
			go func(lo, hi int) {
				var best int32
				for r := lo; r < hi; r++ {
					rb := rows[r*dim : r*dim+dim]
					s := int32(dot16(q, rb))
					if s > best {
						best = s
					}
				}
				results <- best
			}(w*chunk, (w+1)*chunk)
		}
		var best int32
		for w := 0; w < workers; w++ {
			if v := <-results; v > best {
				best = v
			}
		}
		_ = best
	}
	b.ReportMetric(float64(n)*float64(b.N)/b.Elapsed().Seconds()/1e6, "Mvec/s")
	b.ReportMetric(float64(n*dim*2)*float64(b.N)/b.Elapsed().Seconds()/1e9, "GB/s")
	b.ReportMetric(float64(runtime.NumCPU()), "cores")
}
