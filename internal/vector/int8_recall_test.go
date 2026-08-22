// Package vector — recall e erro do fast path int8 (L339).
//
// O caminho quantizado (kernel AVX2, 1 byte/elem) é APROXIMADO por design:
// o dot usa a grade int8 (round(v·127) clampado) enquanto a normalização usa
// as normas exatas. Este teste mede o preço disso em dados realistas
// (gaussiano L2-normalizado, como embeddings de produção):
//   - recall@10: fração do top-10 exato (float32) preservada pelo top-10 int8;
//   - erro de score: maior diferença absoluta entre os dois caminhos.
package vector

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
)

// int8GaussPopulate preenche o store com `count` vetores gaussianos
// L2-normalizados (distribuição típica de embeddings) — o pior caso
// realista para quantização (valores densos, não esparsos). Retorna as rows
// para gerar queries ruidosas (vizinhos reais).
func int8GaussPopulate(t testing.TB, store *SQLiteVec, dim, count int) [][]float64 {
	t.Helper()
	rng := rand.New(rand.NewSource(20260817))
	rows := make([][]float64, 0, count)
	const batch = 200
	for b := 0; b < count; b += batch {
		n := batch
		if b+n > count {
			n = count - b
		}
		recs := make([]VectorRecord, n)
		for i := 0; i < n; i++ {
			idx := b + i
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
			rows = append(rows, v)
			recs[i] = VectorRecord{
				ID:         fmt.Sprintf("g-%d", idx),
				Vector:     v,
				DocumentID: fmt.Sprintf("doc-%d", idx%11),
				ChunkID:    fmt.Sprintf("chunk-%d", idx),
				Content:    fmt.Sprintf("Vetor gaussiano %d.", idx),
			}
		}
		if err := store.Store(dim, recs); err != nil {
			t.Fatalf("store batch %d: %v", b, err)
		}
	}
	return rows
}

// int8NoisyQuery gera uma query com vizinhos reais: uma row do dataset mais
// ruído gaussiano (σ=0.05), normalizada. É o caso de uso real — a consulta
// está perto de alguns itens e longe do resto — onde o ranking tem gap
// distinto e o recall mede o que importa.
func int8NoisyQuery(rng *rand.Rand, rows [][]float64) []float64 {
	base := rows[rng.Intn(len(rows))]
	v := make([]float64, len(base))
	var sq float64
	for d, x := range base {
		v[d] = x + rng.NormFloat64()*0.05
		sq += v[d] * v[d]
	}
	inv := 1 / math.Sqrt(sq)
	for d := range v {
		v[d] *= inv
	}
	return v
}

// TestInt8RecallAndScoreError mede recall@10 e erro de score do fast path
// int8 contra o caminho exato float32, no MESMO índice (flag alternada).
// O default do produto (int8 ON) é o que está sendo validado aqui: o store é
// criado SEM DisableInt8.
func TestInt8RecallAndScoreError(t *testing.T) {
	const dim, count, queries, k = 768, 5000, 50, 10
	store := newTestSQLiteVec(t, dim) // int8 ON (default do produto)
	rows := int8GaussPopulate(t, store, dim, count)

	// Baseline exato: mesma geração, flag desligada (não invalida o índice).
	store.SetInt8Enabled(false)
	store.SetInt16Enabled(false)
	rng := rand.New(rand.NewSource(99))
	qset := make([][]float64, queries)
	for i := range qset {
		qset[i] = int8NoisyQuery(rng, rows)
	}
	exact := make([][]SearchResult, queries)
	for i, q := range qset {
		res, err := store.Search(q, k)
		if err != nil {
			t.Fatalf("exact search %d: %v", i, err)
		}
		exact[i] = res
	}

	// Fast path: mesma geração (snapshot válido), flag ligada.
	store.SetInt8Enabled(true)
	var recall1, recall5, recall10, maxErr float64
	for i, q := range qset {
		res, err := store.Search(q, k)
		if err != nil {
			t.Fatalf("int8 search %d: %v", i, err)
		}
		for _, lim := range []struct {
			n    int
			acc  *float64
			name string
		}{{1, &recall1, "1"}, {5, &recall5, "5"}, {10, &recall10, "10"}} {
			hit := 0
			for _, e := range exact[i][:lim.n] {
				for _, g := range res {
					if g.ID == e.ID {
						hit++
						break
					}
				}
			}
			*lim.acc += float64(hit) / float64(lim.n)
		}
		for _, g := range res {
			for _, e := range exact[i] {
				if g.ID == e.ID {
					if d := math.Abs(g.Score - e.Score); d > maxErr {
						maxErr = d
					}
					break
				}
			}
		}
	}
	nq := float64(queries)
	t.Logf("recall@1=%.4f recall@5=%.4f recall@10=%.4f  max abs score err=%.6f  (dim=%d n=%d q=%d)",
		recall1/nq, recall5/nq, recall10/nq, maxErr, dim, count, queries)

	// Limiares (medidos, L341): o top-1 (vizinho real) é preservado ~100%;
	// o recall@10 paga ~12% nos quase-empates (ranks 2-10 com gap ~0.01, da
	// mesma ordem do erro de quantização). Quem precisar de exatidão total
	// desliga a flag (fallback float32) — ou usamos int16 (2 bytes/elem,
	// ~26M vec/s, recall ~100%).
	if recall1/nq < 0.99 {
		t.Fatalf("recall@1 = %.4f, queremos >= 0.99", recall1/nq)
	}
	if recall10/nq < 0.85 {
		t.Fatalf("recall@10 = %.4f, queremos >= 0.85", recall10/nq)
	}
	if maxErr > 0.02 {
		t.Fatalf("max abs score err = %.6f, queremos <= 0.02", maxErr)
	}
}

// TestInt8PadCorrectness valida o fast path com dims não-múltiplos de 16:
// o pad zerado não pode vazar para o score (kernel exige len%16==0). A
// verificação é recall@1 (o vizinho real tem score ~0.99, imune a quase-
// empates) + erro de score (pad vazado corromperia os scores por completo).
func TestInt8PadCorrectness(t *testing.T) {
	for _, dim := range []int{8, 100, 768, 769} {
		t.Run(fmt.Sprintf("dim%d", dim), func(t *testing.T) {
			const count, k = 300, 5
			store := newTestSQLiteVec(t, dim) // int8 ON
			rows := int8GaussPopulate(t, store, dim, count)
			q := int8NoisyQuery(rand.New(rand.NewSource(7)), rows)

			store.SetInt8Enabled(false)
			store.SetInt16Enabled(false)
			exact, err := store.Search(q, k)
			if err != nil {
				t.Fatalf("exact: %v", err)
			}
			store.SetInt8Enabled(true)
			fast, err := store.Search(q, k)
			if err != nil {
				t.Fatalf("int8: %v", err)
			}
			if len(fast) != len(exact) {
				t.Fatalf("len(fast)=%d want %d", len(fast), len(exact))
			}
			// recall@1: o top-1 exato deve ser o top-1 do fast path.
			if fast[0].ID != exact[0].ID {
				t.Fatalf("dim %d: top-1 int8=%q exato=%q — pad vazou no score?", dim, fast[0].ID, exact[0].ID)
			}
			// erro de score no top-1 (o mesmo id deve ter score ~igual).
			if d := math.Abs(fast[0].Score - exact[0].Score); d > 0.02 {
				t.Fatalf("dim %d: score err top-1 = %.6f (pad vazado?)", dim, d)
			}
		})
	}
}

// TestInt8QuantizeBoundaries valida o clamp da quantização.
func TestInt8QuantizeBoundaries(t *testing.T) {
	for _, tc := range []struct {
		in   float32
		want int8
	}{
		{0, 0}, {0.5, 64}, {-0.5, -64}, {1, 127}, {-1, -127},
		{1.5, 127}, {-2, -127}, {0.007874, 1}, {-0.007874, -1},
	} {
		if got := quantize8(tc.in); got != tc.want {
			t.Errorf("quantize8(%v) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// TestInt8QueryPad verifica o pad 128 do query (contribui 0 contra pad 0).
func TestInt8QueryPad(t *testing.T) {
	qb := quantizeQuery8([]float64{0.5, -0.5}, 16)
	if len(qb) != 16 {
		t.Fatalf("len=%d want 16", len(qb))
	}
	for d := 2; d < 16; d++ {
		if qb[d] != 128 {
			t.Fatalf("pad[%d]=%d want 128", d, qb[d])
		}
	}
}
