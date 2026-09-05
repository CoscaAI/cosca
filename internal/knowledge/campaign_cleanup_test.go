// Package knowledge — campanha performance, L359: DRY-RUN da limpeza
// histórica (READ-ONLY — simulação em memória, NENHUMA transformação).
//
// Pré-requisitos da limpeza (regra do professor): snapshot ✓ (L357) · dry-run
// (este) · contagem antes/depois · backup (pendente, antes da execução) ·
// validação contra o oracle. A transformação real NÃO é feita aqui.
//
// Simula: (1) remoção dos triviais históricos (filtro T1-T4, L352);
// (2) deduplicação exata por conteúdo (hash do texto bruto) — 1 canônico +
// referências. Mede: contagens, slabs, diversidade do top-10, recall
// (int8/int16 vs oracle float32 no corpus LIMPO).
package knowledge

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/cache"
	"github.com/CoscaAI/cosca/internal/chunker"
)

// TestCampaignCleanupDryRun — a simulação completa da limpeza (L359).
func TestCampaignCleanupDryRun(t *testing.T) {
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

	const dim = 768

	// Carrega vetores com conteúdo.
	rows, err := eng.db.Conn().Query(
		"SELECT id, content, vector FROM vectors WHERE length(vector) = ? ORDER BY rowid", dim*4)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type vrow struct {
		id      string
		content string
		vec     []float64
	}
	var vecs []vrow
	for rows.Next() {
		var id, content string
		var blob []byte
		if err := rows.Scan(&id, &content, &blob); err != nil {
			t.Fatal(err)
		}
		v := make([]float64, dim)
		for d := 0; d < dim; d++ {
			v[d] = float64(math.Float32frombits(binary.LittleEndian.Uint32(blob[d*4:])))
		}
		vecs = append(vecs, vrow{id: id, content: content, vec: v})
	}
	nBefore := len(vecs)

	// ── 1. FILTRO TRIVIAIS (T1-T4) ──
	var kept []vrow
	trivialCount := 0
	for _, v := range vecs {
		if ok, _ := chunker.IsTrivial(v.content); ok {
			trivialCount++
			continue
		}
		kept = append(kept, v)
	}
	nAfterTrivial := len(kept)

	// ── 2. DEDUP por conteúdo (hash bruto) ──
	seen := map[[32]byte]bool{}
	var deduped []vrow
	dedupCount := 0
	for _, v := range kept {
		h := sha256.Sum256([]byte(v.content))
		if seen[h] {
			dedupCount++
			continue
		}
		seen[h] = true
		deduped = append(deduped, v)
	}
	nFinal := len(deduped)

	// ── 3. SLABS ──
	slab := func(n int, bytes int) float64 { return float64(n*bytes) / 1e6 }

	// ── 4. DIVERSIDADE top-10 (oracle float32, 200 queries) ──
	const nq = 200
	if nq > len(vecs) {
		t.Fatalf("poucos vetores: %d", len(vecs))
	}
	queries := vecs[:nq]
	normQ := make([]float64, nq)
	for i, q := range queries {
		var n float64
		for _, x := range q.vec {
			n += x * x
		}
		normQ[i] = math.Sqrt(n)
	}
	normR := make([]float64, len(vecs))
	for i, v := range vecs {
		var n float64
		for _, x := range v.vec {
			n += x * x
		}
		normR[i] = math.Sqrt(n)
	}
	// Diversidade ANTES (top-10 do oracle no corpus completo).
	divBefore := 0.0
	for qi, q := range queries {
		sc := make([]float64, len(vecs))
		for j, v := range vecs {
			sc[j] = scoreFloat(q.vec, v.vec, normQ[qi], normR[j])
		}
		top := topK(sc, 10)
		contents := map[[32]byte]bool{}
		for _, idx := range top {
			contents[sha256.Sum256([]byte(vecs[idx].content))] = true
		}
		divBefore += float64(len(contents))
	}
	// Diversidade DEPOIS (oracle no corpus limpo).
	normD := make([]float64, len(deduped))
	for i, v := range deduped {
		var n float64
		for _, x := range v.vec {
			n += x * x
		}
		normD[i] = math.Sqrt(n)
	}
	divAfter := 0.0
	for qi, q := range queries {
		sc := make([]float64, len(deduped))
		for j, v := range deduped {
			sc[j] = scoreFloat(q.vec, v.vec, normQ[qi], normD[j])
		}
		top := topK(sc, 10)
		contents := map[[32]byte]bool{}
		for _, idx := range top {
			contents[sha256.Sum256([]byte(deduped[idx].content))] = true
		}
		divAfter += float64(len(contents))
	}

	// ── 5. RECALL no corpus LIMPO (int8/int16 vs oracle limpo) ──
	sumR8 := make([]int32, len(deduped))
	for i, v := range deduped {
		var s int32
		for _, x := range v.vec {
			s += int32(quant8(x))
		}
		sumR8[i] = s
	}
	var jac8, jac16, r10_8, r10_16, ndcg8, ndcg16 float64
	for qi, q := range queries {
		scF := make([]float64, len(deduped))
		sc8 := make([]float64, len(deduped))
		sc16 := make([]float64, len(deduped))
		for j, v := range deduped {
			scF[j] = scoreFloat(q.vec, v.vec, normQ[qi], normD[j])
			sc8[j] = scoreInt8(q.vec, v.vec, sumR8[j], normQ[qi], normD[j])
			sc16[j] = scoreInt16(q.vec, v.vec, normQ[qi], normD[j])
		}
		topF := topK(scF, 50)
		top8 := topK(sc8, 50)
		top16 := topK(sc16, 50)
		jac8 += jaccardTop(topF, top8)
		jac16 += jaccardTop(topF, top16)
		r10_8 += recallAt(top8, topF, 10)
		r10_16 += recallAt(top16, topF, 10)
		ndcg8 += ndcgAt(top8, topF, 10)
		ndcg16 += ndcgAt(top16, topF, 10)
	}
	nf := float64(nq)

	// ── RELATÓRIO ──
	t.Logf("── L359 DRY-RUN da limpeza histórica (READ-ONLY) ──")
	t.Logf("vetores: antes=%d  pós-triviais=%d (-%d, %.1f%%)  pós-dedup=%d (-%d, %.1f%%)",
		nBefore, nAfterTrivial, nBefore-nAfterTrivial,
		float64(nBefore-nAfterTrivial)*100/float64(nBefore),
		nFinal, nBefore-nFinal, float64(nBefore-nFinal)*100/float64(nBefore))
	t.Logf("triviais removíveis=%d  duplicatas de conteúdo=%d", trivialCount, dedupCount)
	t.Logf("slabs: float32 %.1f→%.1fMB  int8 %.1f→%.1fMB  int16 %.1f→%.1fMB  (budget 80MB)",
		slab(nBefore, 3072), slab(nFinal, 3072), slab(nBefore, 768), slab(nFinal, 768),
		slab(nBefore, 1536), slab(nFinal, 1536))
	t.Logf("diversidade top-10 (conteúdos únicos): antes=%.2f/10  depois=%.2f/10",
		divBefore/nf, divAfter/nf)
	t.Logf("recall no corpus LIMPO: int8 jaccard=%.4f recall@10=%.4f NDCG=%.4f | int16 jaccard=%.4f recall@10=%.4f NDCG=%.4f",
		jac8/nf, r10_8/nf, ndcg8/nf, jac16/nf, r10_16/nf, ndcg16/nf)
	t.Logf("latência estimada pós-limpeza: scan int8 %.1f%% do atual (∝ N)",
		float64(nFinal)*100/float64(nBefore))
}