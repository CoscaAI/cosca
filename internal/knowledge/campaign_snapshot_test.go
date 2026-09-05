// Package knowledge — campanha performance, L357: SNAPSHOT pós-correção
// (read-only — NENHUMA transformação). Fotografia do corpus com a fábrica
// corrigida (L356) e o filtro de triviais ativo (L353), ANTES de qualquer
// limpeza histórica. Registra horário/versão (db é mutante — a comparação
// com L349/L352 é informativa, não exata).
package knowledge

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/cache"
	"github.com/CoscaAI/cosca/internal/chunker"
	"github.com/CoscaAI/cosca/internal/vector"
)

// TestCampaignSnapshotPostL356 — a fotografia completa (L357).
func TestCampaignSnapshotPostL356(t *testing.T) {
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

	ts := time.Now().Format("2006-01-02 15:04:05")

	// ── 1. FOTOGRAFIA ──
	rows, err := eng.db.Conn().Query(
		"SELECT c.id, c.content, EXISTS(SELECT 1 FROM vectors v WHERE v.chunk_id = c.id) FROM chunks c")
	if err != nil {
		t.Fatal(err)
	}
	type chunkInfo struct {
		content   string
		hasVector bool
	}
	var chunks []chunkInfo
	for rows.Next() {
		var id string
		var c chunkInfo
		if err := rows.Scan(&id, &c.content, &c.hasVector); err != nil {
			t.Fatal(err)
		}
		chunks = append(chunks, c)
	}
	rows.Close()

	// Vetores + duplicatas + tamanhos.
	vecRows, err := eng.db.Conn().Query("SELECT vector, document_id FROM vectors WHERE length(vector) = 768*4")
	if err != nil {
		t.Fatal(err)
	}
	var vecBlobs []string
	vecDocs := map[string]string{}
	for vecRows.Next() {
		var blob []byte
		var doc string
		if err := vecRows.Scan(&blob, &doc); err != nil {
			t.Fatal(err)
		}
		vecBlobs = append(vecBlobs, string(blob))
		vecDocs[fmt.Sprintf("%p", &blob)] = doc
	}
	vecRows.Close()
	_ = vecDocs

	// Duplicatas por hash do blob.
	dupMap := map[string]int{}
	for _, b := range vecBlobs {
		dupMap[b]++
	}
	distinct, dupVectors := len(dupMap), 0
	for _, c := range dupMap {
		if c > 1 {
			dupVectors += c - 1
		}
	}

	// Triviais (filtro L353) entre os chunks.
	trivTotal, trivVec, trivOrphan := 0, 0, 0
	for _, c := range chunks {
		if ok, _ := chunker.IsTrivial(c.content); ok {
			trivTotal++
			if c.hasVector {
				trivVec++
			} else {
				trivOrphan++
			}
		}
	}
	withVec, orphans := 0, 0
	for _, c := range chunks {
		if c.hasVector {
			withVec++
		} else {
			orphans++
		}
	}

	// ── 2. DISTRIBUIÇÃO de tamanho (chunks com vetor) ──
	var sizes []int
	for _, c := range chunks {
		if c.hasVector {
			sizes = append(sizes, len(c.content))
		}
	}
	sort.Ints(sizes)
	pct := func(p float64) int {
		if len(sizes) == 0 {
			return 0
		}
		return sizes[int(float64(len(sizes)-1)*p)]
	}

	// ── 3. SLABS ──
	nVec := len(vecBlobs)
	slabF32 := float64(nVec * 3072) / 1e6
	slabI8 := float64(nVec * 768) / 1e6
	slabI16 := float64(nVec * 1536) / 1e6

	t.Logf("── L357 SNAPSHOT pós-correção (L356) — %s ──", ts)
	t.Logf("corpus: chunks=%d  com vetor=%d  órfãos=%d  vetores=%d  distintos=%d",
		len(chunks), withVec, orphans, nVec, distinct)
	t.Logf("duplicatas=%d (%.1f%%)  triviais=%d (com vetor=%d, órfãos=%d)",
		dupVectors, float64(dupVectors)*100/float64(nVec), trivTotal, trivVec, trivOrphan)
	t.Logf("distribuição tamanho (chars): p50=%d p95=%d p99=%d max=%d",
		pct(0.50), pct(0.95), pct(0.99), sizes[len(sizes)-1])
	t.Logf("slabs: float32=%.1fMB  int8=%.1fMB  int16=%.1fMB  (budget L3=80MB)",
		slabF32, slabI8, slabI16)

	// ── 4-5. RECALL determinístico (oracle float32 vs int8 vs int16) ──
	snapshotRecall(t, eng)

	// ── 6. LATÊNCIA de produção (índice, int8 ON) ──
	latencyProduction(t, eng)

	// ── 7. DETERMINISMO do top-K ──
	determinismTopK(t, eng)
}

// snapshotRecall — recall determinístico (reusa os métodos validados L349).
func snapshotRecall(t *testing.T, eng *Engine) {
	t.Helper()
	const dim, nq = 768, 200
	qRows, err := eng.db.Conn().Query(
		"SELECT id, vector FROM vectors WHERE length(vector) = ? ORDER BY rowid LIMIT ?", dim*4, nq)
	if err != nil {
		t.Fatal(err)
	}
	defer qRows.Close()
	var qids []string
	var qvecs [][]float64
	for qRows.Next() {
		var id string
		var blob []byte
		if err := qRows.Scan(&id, &blob); err != nil {
			t.Fatal(err)
		}
		v := make([]float64, dim)
		for d := 0; d < dim; d++ {
			v[d] = float64(math.Float32frombits(binary.LittleEndian.Uint32(blob[d*4:])))
		}
		qids = append(qids, id)
		qvecs = append(qvecs, v)
	}
	_ = qids

	allRows, err := eng.db.Conn().Query(
		"SELECT id, vector FROM vectors WHERE length(vector) = ? ORDER BY rowid", dim*4)
	if err != nil {
		t.Fatal(err)
	}
	defer allRows.Close()
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
		allVecs = append(allVecs, v)
	}
	normQ := make([]float64, nq)
	for i, q := range qvecs {
		var n float64
		for _, x := range q {
			n += x * x
		}
		normQ[i] = math.Sqrt(n)
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

	var jac8, jac16, r10_8, r10_16, ndcg8, ndcg16 float64
	for qi, q := range qvecs {
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
		r10_8 += recallAt(top8, topF, 10)
		r10_16 += recallAt(top16, topF, 10)
		ndcg8 += ndcgAt(top8, topF, 10)
		ndcg16 += ndcgAt(top16, topF, 10)
	}
	nf := float64(nq)
	t.Logf("recall determinístico: int8 jaccard=%.4f recall@10=%.4f NDCG=%.4f | int16 jaccard=%.4f recall@10=%.4f NDCG=%.4f",
		jac8/nf, r10_8/nf, ndcg8/nf, jac16/nf, r10_16/nf, ndcg16/nf)
}

// latencyProduction — latência média do scan int8 (índice real, produção).
func latencyProduction(t *testing.T, eng *Engine) {
	t.Helper()
	store := eng.vecStore.(*vector.SQLiteVec)
	store.SetInt8Enabled(true)
	rows, err := eng.db.Conn().Query(
		"SELECT vector FROM vectors WHERE length(vector) = 768*4 ORDER BY rowid LIMIT 50")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var total time.Duration
	n := 0
	for rows.Next() {
		var blob []byte
		if err := rows.Scan(&blob); err != nil {
			t.Fatal(err)
		}
		q := make([]float64, 768)
		for d := 0; d < 768; d++ {
			q[d] = float64(math.Float32frombits(binary.LittleEndian.Uint32(blob[d*4:])))
		}
		start := time.Now()
		if _, err := store.Search(q, 10); err != nil {
			t.Fatal(err)
		}
		total += time.Since(start)
		n++
	}
	t.Logf("latência produção (scan int8, 50 queries): média=%s", total/time.Duration(n))
}

// determinismTopK — mesma query N vezes: quantifica a variação do top-K.
func determinismTopK(t *testing.T, eng *Engine) {
	t.Helper()
	store := eng.vecStore.(*vector.SQLiteVec)
	store.SetInt8Enabled(true)
	var blob []byte
	if err := eng.db.Conn().QueryRow(
		"SELECT vector FROM vectors WHERE length(vector) = 768*4 ORDER BY rowid LIMIT 1").Scan(&blob); err != nil {
		t.Fatal(err)
	}
	q := make([]float64, 768)
	for d := 0; d < 768; d++ {
		q[d] = float64(math.Float32frombits(binary.LittleEndian.Uint32(blob[d*4:])))
	}
	const runs = 20
	first := ""
	diffRuns := 0
	rankings := map[string]int{}
	for i := 0; i < runs; i++ {
		res, err := store.Search(q, 10)
		if err != nil {
			t.Fatal(err)
		}
		var sb string
		for _, r := range res {
			sb += r.ID + "|"
		}
		rankings[sb]++
		if i == 0 {
			first = sb
		} else if sb != first {
			diffRuns++
		}
	}
	t.Logf("determinismo top-10 (20 runs da MESMA query): %d runs diferem da 1ª, %d rankings distintos",
		diffRuns, len(rankings))
}