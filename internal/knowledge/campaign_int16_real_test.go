// Package knowledge — campanha performance, A-sub (L349): recall REAL do
// int16 vs float32 (oracle), com o método determinístico validado na
// caracterização B (score em Go puro sobre snapshot único — o índice em
// produção tem desempate não-determinístico entre duplicatas, o que contamina
// jaccard/NDCG medidos via Search).
//
// Score int16: q16 = round(v·1024) clamp ±1023 (grade 2^10, mesmo algoritmo
// do kernel dot16AVX2 — FASE 7), score = Σq16·r16 / (1024²·‖q‖·‖r‖).
package knowledge

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/cache"
)

func quant16(v float64) int16 {
	f := v * 1024
	if f >= 1024 {
		return 1023
	}
	if f <= -1024 {
		return -1023
	}
	return int16(math.Round(f))
}

func scoreInt16(q, r []float64, normQ, normR float64) float64 {
	var acc int64
	for i := range q {
		acc += int64(quant16(q[i])) * int64(quant16(r[i]))
	}
	return float64(acc) / (1048576 * normQ * normR) // 1024² = 1048576
}

// TestCampaignInt16Real — recall real do int16 (A-sub), método determinístico.
func TestCampaignInt16Real(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	root := "/home/cosca/Documents/cosca"
	coscaDir := filepath.Join(root, ".cosca")
	if _, err := os.Stat(filepath.Join(coscaDir, "knowledge.db")); err != nil {
		t.Skip("sem knowledge.db")
	}
	eng, err := New(Config{
		DBPath:      filepath.Join(coscaDir, "knowledge.db"),
		RootDir:     root,
		AutoMigrate: true,
		CacheConfig: cache.Config{EnabledLevels: []cache.Level{cache.Level(255)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.Init(); err != nil {
		t.Fatal(err)
	}
	defer eng.Close()

	const dim, nq = 768, 200
	rows, err := eng.db.Conn().Query(
		"SELECT id, vector FROM vectors WHERE length(vector) = ? ORDER BY rowid LIMIT ?", dim*4, nq)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ids []string
	var vecs [][]float64
	for rows.Next() {
		var id string
		var blob []byte
		if err := rows.Scan(&id, &blob); err != nil {
			t.Fatal(err)
		}
		v := make([]float64, dim)
		for d := 0; d < dim; d++ {
			v[d] = float64(math.Float32frombits(binary.LittleEndian.Uint32(blob[d*4:])))
		}
		ids = append(ids, id)
		vecs = append(vecs, v)
	}
	if len(vecs) != nq {
		t.Fatalf("queries=%d want %d", len(vecs), nq)
	}

	// Scores determinísticos: oracle float32, int8, int16 — snapshot único.
	normQ := make([]float64, nq)
	for i, q := range vecs {
		var n float64
		for _, x := range q {
			n += x * x
		}
		normQ[i] = math.Sqrt(n)
	}
	allRows, err := eng.db.Conn().Query(
		"SELECT id, vector FROM vectors WHERE length(vector) = ? ORDER BY rowid", dim*4)
	if err != nil {
		t.Fatal(err)
	}
	defer allRows.Close()
	var allIDs []string
	var allVecs [][]float64
	for allRows.Next() {
		var id string
		var blob []byte
		if err := allRows.Scan(&id, &blob); err != nil {
			t.Fatal(err)
		}
		v := make([]float64, dim)
		for d := 0; d < dim; d++ {
			v[d] = float64(math.Float32frombits(binary.LittleEndian.Uint32(blob[d*4:])))
		}
		allIDs = append(allIDs, id)
		allVecs = append(allVecs, v)
	}
	normR := make([]float64, len(allVecs))
	sumR8 := make([]int32, len(allVecs))
	for i, v := range allVecs {
		var n float64
		var s int32
		for _, x := range v {
			n += x * x
			s += int32(quant8(x))
		}
		normR[i] = math.Sqrt(n)
		sumR8[i] = s
	}

	// Para cada query: rankings do oracle, int8, int16 (top-50).
	var r1_16, r5_16, r10_16, jac16, ndcg16, maxErr16, sumErr16 float64
	var r1_8, r10_8, jac8, ndcg8 float64
	top1Changed16, top1Empate16 := 0, 0
	for qi, q := range vecs {
		scF := make([]float64, len(allVecs))
		sc8 := make([]float64, len(allVecs))
		sc16 := make([]float64, len(allVecs))
		for j, v := range allVecs {
			scF[j] = scoreFloat(q, v, normQ[qi], normR[j])
			sc8[j] = scoreInt8(q, v, sumR8[j], normQ[qi], normR[j])
			sc16[j] = scoreInt16(q, v, normQ[qi], normR[j])
		}
		topF := topK(scF, 50)
		top8 := topK(sc8, 50)
		top16 := topK(sc16, 50)
		jac8 += jaccardTop(topF, top8)
		jac16 += jaccardTop(topF, top16)
		r1_8 += recallAt(top8, topF, 1)
		r10_8 += recallAt(top8, topF, 10)
		r1_16 += recallAt(top16, topF, 1)
		r5_16 += recallAt(top16, topF, 5)
		r10_16 += recallAt(top16, topF, 10)
		ndcg8 += ndcgAt(top8, topF, 10)
		ndcg16 += ndcgAt(top16, topF, 10)
		// top-1 do int16: mudou? empate (gap < 0.02) ou relevante?
		if top16[0] != topF[0] {
			top1Changed16++
			if math.Abs(sc16[top16[0]]-scF[topF[0]]) < 0.02 {
				top1Empate16++
			}
		}
		// erro de score do int16 (mesmos ids no top-50)
		for _, id16 := range top16 {
			for _, idF := range topF {
				if id16 == idF {
					d := math.Abs(sc16[id16] - scF[idF])
					if d > maxErr16 {
						maxErr16 = d
					}
					sumErr16 += d
					break
				}
			}
		}
	}
	nf := float64(nq)
	t.Logf("── A-sub: recall REAL do int16 (corpus, %d queries, dim 768, determinístico) ──", nq)
	t.Logf("int8 : recall@1=%.4f @10=%.4f  jaccard@50=%.4f  NDCG@10=%.4f",
		r1_8/nf, r10_8/nf, jac8/nf, ndcg8/nf)
	t.Logf("int16: recall@1=%.4f @5=%.4f @10=%.4f  jaccard@50=%.4f  NDCG@10=%.4f",
		r1_16/nf, r5_16/nf, r10_16/nf, jac16/nf, ndcg16/nf)
	t.Logf("int16 top-1 mudou: %d/%d (%.1f%%) — empates=%d  mudanças com gap>=0.02=%d",
		top1Changed16, nq, float64(top1Changed16)*100/nf, top1Empate16, top1Changed16-top1Empate16)
	t.Logf("int16 max err=%.6f  mean err=%.6f", maxErr16, sumErr16/float64(nq*50))
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

// ndcgAt: NDCG binário (relevância = está no oracle top-50).
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