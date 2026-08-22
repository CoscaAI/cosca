// Package vector — prova do kernel AVX2 int8 (L339): correção exata vs
// referência e benchmark de limite da máquina (30M vetores, 16 threads).
package vector

import (
	"math/rand"
	"runtime"
	"testing"
)

// ── CORREÇÃO — kernel asm deve ser bit-exato vs referência int64 ──────────

func TestDot8BiasAVX2Correctness(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for _, dim := range []int{32, 64, 96, 768, 768 * 3} {
		q := make([]int8, dim)
		r := make([]int8, dim)
		for i := range q {
			q[i] = int8(rng.Intn(255) - 127)
			r[i] = int8(rng.Intn(255) - 127)
		}
		qb := make([]uint8, dim)
		var sumR int32
		for i := range r {
			qb[i] = uint8(int(q[i]) + 128)
			sumR += int32(r[i])
		}
		got := dot8BiasDot(qb, r, sumR)
		var want int64
		for i := range q {
			want += int64(q[i]) * int64(r[i])
		}
		if int64(got) != want {
			t.Fatalf("dim %d: got %d want %d", dim, got, want)
		}
	}
}

// Caso-limite: valores extremos (127/-127) — vpmaddubsw nunca satura
// (127·127 = 16129 < 32767), mas o acúmulo i32 precisa estar certo.
func TestDot8BiasAVX2Extremes(t *testing.T) {
	const dim = 768
	q := make([]int8, dim)
	r := make([]int8, dim)
	for i := 0; i < dim; i++ {
		q[i] = 127
		r[i] = -127
	}
	qb := make([]uint8, dim)
	var sumR int32
	for i := range r {
		qb[i] = uint8(int(q[i]) + 128)
		sumR += int32(r[i])
	}
	got := dot8BiasDot(qb, r, sumR)
	want := int64(127) * int64(-127) * dim
	if int64(got) != want {
		t.Fatalf("extremes: got %d want %d", got, want)
	}
}

// ── LIMITE DA MÁQUINA — dataset 30M×768 (23GB), 16 workers, kernel AVX2 ───

func BenchmarkLimitInt8AVX2_N30M_Dim768(b *testing.B) {
	if testing.Short() {
		b.Skip("dataset de 23GB em modo curto")
	}
	const n, dim = 30_000_000, 768

	// row base (padrão determinístico) + soma por row.
	row0 := make([]int8, dim)
	var sumR int32
	for d := range row0 {
		row0[d] = int8((d*7+3)%255) - 127
		sumR += int32(row0[d])
	}

	// 23.04GB de rows — preenchimento por blocos (memmove, rápido).
	rows := make([]int8, n*dim)
	batch := make([]int8, dim*10000)
	for i := 0; i < 10000; i++ {
		copy(batch[i*dim:], row0)
	}
	for i := 0; i < n/10000; i++ {
		copy(rows[i*len(batch):], batch)
	}
	sums := make([]int32, n)
	for i := range sums {
		sums[i] = sumR
	}

	// query biasada (768 uint8).
	qb := make([]uint8, dim)
	for d := range qb {
		qb[d] = uint8(d%200 + 28) // 28..227 → q ∈ [-100, 99]
	}

	const workers = 16 // NumCPU do 5700X3D
	chunk := n / workers

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results := make(chan int32, workers)
		for w := 0; w < workers; w++ {
			go func(lo, hi int) {
				var best int32
				for r := lo; r < hi; r++ {
					rb := rows[r*dim : r*dim+dim]
					s := dot8BiasDot(qb, rb, sums[r])
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
	b.ReportMetric(float64(n*dim*1)*float64(b.N)/b.Elapsed().Seconds()/1e9, "GB/s")
	b.ReportMetric(float64(n*dim*4)*float64(b.N)/b.Elapsed().Seconds()/1e9, "GB/s-equiv-float32")
	b.ReportMetric(float64(runtime.NumCPU()), "cores")
}
