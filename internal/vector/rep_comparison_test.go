// Package vector — campanha FASE 7 (L347): recall comparativo int16 vs int8
// vs float32 (oracle), no MESMO dataset sintético gaussiano do int8_recall_test.
// Responde a pergunta 2 do professor: qual representação preserva melhor o
// ranking? (pergunta 1 = velocidade no dot16_test; pergunta 3 = bytes/elem.)
package vector

import (
	"math"
	"math/rand"
	"testing"
)

// TestRepRecallComparison — int16 e int8 contra o oracle float32.
func TestRepRecallComparison(t *testing.T) {
	const dim, count, queries, k = 768, 5000, 50, 10
	rng := rand.New(rand.NewSource(20260817))

	// Dataset gaussiano L2-normalizado (mesmo gerador do int8_recall_test).
	rows := make([][]float64, count)
	for i := range rows {
		v := make([]float64, dim)
		var sq float64
		for d := 0; d < dim; d++ {
			v[d] = rng.NormFloat64()
			sq += v[d] * v[d]
		}
		inv := 1 / math.Sqrt(sq)
		for d := 0; d < dim; d++ {
			v[d] *= inv
		}
		rows[i] = v
	}
	// Queries: row + ruído σ=0.05 (vizinhos reais — como o recall int8).
	qrng := rand.New(rand.NewSource(99))
	qset := make([][]float64, queries)
	for i := range qset {
		base := rows[qrng.Intn(count)]
		v := make([]float64, dim)
		var sq float64
		for d, x := range base {
			v[d] = x + qrng.NormFloat64()*0.05
			sq += v[d] * v[d]
		}
		inv := 1 / math.Sqrt(sq)
		for d := range v {
			v[d] *= inv
		}
		qset[i] = v
	}

	// Pré-quantiza rows e queries (int8 grade 127, int16 grade 1024).
	q8 := make([][]int8, count)
	q16 := make([][]int16, count)
	sums := make([]int32, count)
	for i, v := range rows {
		r8 := make([]int8, dim)
		r16 := make([]int16, dim)
		var s8 int32
		for d, x := range v {
			r8[d] = quantize8(float32(x))
			r16[d] = quantize16(float32(x))
			s8 += int32(r8[d])
		}
		q8[i], q16[i], sums[i] = r8, r16, s8
	}

	// Oracle float32: score = dot/(‖q‖·‖r‖) exato.
	scoreExact := func(q, r []float64) float64 {
		var dot, nq, nr float64
		for d := range q {
			dot += q[d] * r[d]
			nq += q[d] * q[d]
			nr += r[d] * r[d]
		}
		if nq == 0 || nr == 0 {
			return 0
		}
		return dot / (math.Sqrt(nq) * math.Sqrt(nr))
	}
	oracle := make([][]int, queries)
	for i, q := range qset {
		sc := make([]float64, count)
		for j, r := range rows {
			sc[j] = scoreExact(q, r)
		}
		oracle[i] = topK(sc, k)
	}

	// int8: score = dot8/(127²·‖q‖·‖r‖); int16: dot16/(1024²·‖q‖·‖r‖).
	var r8_1, r8_10, r16_1, r16_10, e8, e16, ndcg8, ndcg16 float64
	qb := make([]uint8, dim)
	q16q := make([]int16, dim)
	for i, q := range qset {
		var nq float64
		for d, x := range q {
			nq += x * x
			qb[d] = uint8(int(quantize8(float32(x))) + 128)
			q16q[d] = quantize16(float32(x))
		}
		norm := math.Sqrt(nq)
		sc8 := make([]float64, count)
		sc16 := make([]float64, count)
		for j := range rows {
			sc8[j] = float64(dot8BiasDot(qb, q8[j], sums[j])) / (16129 * norm)
			sc16[j] = float64(dot16(q16q, q16[j])) / (1048576 * norm)
		}
		top8 := topK(sc8, k)
		top16 := topK(sc16, k)
		r8_1 += recallAt(top8, oracle[i], 1)
		r8_10 += recallAt(top8, oracle[i], 10)
		r16_1 += recallAt(top16, oracle[i], 1)
		r16_10 += recallAt(top16, oracle[i], 10)
		ndcg8 += ndcgAt(top8, oracle[i], 10)
		ndcg16 += ndcgAt(top16, oracle[i], 10)
		for _, id := range top8 {
			e8 = math.Max(e8, math.Abs(sc8[id]-scoreExact(q, rows[id])))
		}
		for _, id := range top16 {
			e16 = math.Max(e16, math.Abs(sc16[id]-scoreExact(q, rows[id])))
		}
	}
	n := float64(queries)
	t.Logf("── FASE 7: recall comparativo (sintético gaussiano, %d queries, dim %d) ──", queries, dim)
	t.Logf("int8 : recall@1=%.4f @10=%.4f NDCG@10=%.4f maxerr=%.6f", r8_1/n, r8_10/n, ndcg8/n, e8)
	t.Logf("int16: recall@1=%.4f @10=%.4f NDCG@10=%.4f maxerr=%.6f", r16_1/n, r16_10/n, ndcg16/n, e16)

	if r16_10/n < 0.97 {
		t.Errorf("int16 recall@10 = %.4f, queremos >= 0.97", r16_10/n)
	}
	if r16_1/n < 0.99 {
		t.Errorf("int16 recall@1 = %.4f, queremos >= 0.99", r16_1/n)
	}
	if e16 > 0.002 {
		t.Errorf("int16 maxerr = %.6f, queremos <= 0.002 (grade 2^10)", e16)
	}
}

// topK devolve os índices dos k maiores scores (descendente).
func topK(sc []float64, k int) []int {
	idx := make([]int, len(sc))
	for i := range idx {
		idx[i] = i
	}
	// selection parcial: ordena até k.
	for i := 1; i < len(idx); i++ {
		for j := i; j > 0 && sc[idx[j]] > sc[idx[j-1]]; j-- {
			idx[j], idx[j-1] = idx[j-1], idx[j]
		}
	}
	return idx[:k]
}

// recallAt: fração do top-lim do oracle presente no top-lim do candidato.
func recallAt(cand, oracle []int, lim int) float64 {
	hit := 0
	for _, o := range oracle[:lim] {
		for _, c := range cand[:lim] {
			if c == o {
				hit++
				break
			}
		}
	}
	return float64(hit) / float64(lim)
}

// ndcgAt: NDCG binário (relevância = está no oracle top-k).
func ndcgAt(cand, oracle []int, k int) float64 {
	inOracle := map[int]bool{}
	for _, o := range oracle {
		inOracle[o] = true
	}
	var dcg float64
	for j, c := range cand[:k] {
		if inOracle[c] {
			dcg += 1 / math.Log2(float64(j+2))
		}
	}
	var idcg float64
	for j := 0; j < k; j++ {
		idcg += 1 / math.Log2(float64(j+2))
	}
	return dcg / idcg
}

// BenchmarkDot16_L3_100k mede o int16 no regime cache (100k×768 = 153MB slab
// int16 — 1.6x o L3, então deve ficar entre RAM e cache).
func BenchmarkDot16_L3_100k(b *testing.B) {
	const n, dim = 100_000, 768
	row0 := make([]int16, dim)
	for d := range row0 {
		row0[d] = int16(((d*7 + 3) % 255) - 127)
	}
	rows := make([]int16, n*dim)
	for i := 0; i < n; i++ {
		copy(rows[i*dim:], row0)
	}
	q := make([]int16, dim)
	for d := range q {
		q[d] = int16(((d*13 + 7) % 255) - 127)
	}
	const workers = 16
	chunk := n / workers
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results := make(chan int32, workers)
		for w := 0; w < workers; w++ {
			go func(lo, hi int) {
				var best int32
				for r := lo; r < hi; r++ {
					s := int32(dot16(q, rows[r*dim:r*dim+dim]))
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
}
