// Package knowledge — campanha performance, FASE B (L349): caracterização
// da deduplicação do corpus. READ-ONLY — nenhuma alteração de produção,
// nenhuma remoção, nenhuma escrita. Categorias separadas SEM misturar:
//
//	CAT1 duplicata exata de CONTEÚDO (sha256 do text)
//	CAT2 duplicata exata de VETOR (sha256 do blob)
//	CAT3 mesmo documento (das duplicatas de vetor) vs cross-documento
//	CAT4 mesmo chunk (duplicatas que apontam para o mesmo chunk_id)
//	CAT5 vetor igual com conteúdo DIFERENTE
//	CAT6 quase-duplicata (cosseno ≥ 0.99)
//
// + órfãos (chunks sem vetor) e simulação de remoção EM MEMÓRIA (dataset
// derivado — nada é removido do corpus real; scoring em Go puro, mesmo
// algoritmo de quantização do índice).
package knowledge

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/cache"
	"github.com/CoscaAI/cosca/internal/vector"
)

type dedupChunk struct {
	id         string
	documentID string
	content    string
	hasVector  bool
}

type dedupVec struct {
	id         string
	documentID string
	chunkID    string
	content    string
	vec        []float64
	normSq     float64
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// quant8 e dot8Bias replicam o algoritmo do índice (L339) para a simulação —
// o mesmo erro de quantização, sem tocar o pacote vector (fronteira
// EXPERIMENTAL/PRODUÇÃO rígida).
func quant8(v float64) int8 {
	f := v * 127
	if f >= 127 {
		return 127
	}
	if f <= -127 {
		return -127
	}
	return int8(math.Round(f))
}

func scoreInt8(q, r []float64, sumR int32, normQ, normR float64) float64 {
	var acc int64
	for i := range q {
		qb := int64(quant8(q[i])) + 128
		acc += qb * int64(quant8(r[i]))
	}
	dot := float64(acc-128*int64(sumR)) / (16129 * normQ * normR)
	return dot
}

func scoreFloat(q, r []float64, normQ, normR float64) float64 {
	var dot float64
	for i := range q {
		dot += q[i] * r[i]
	}
	if normQ == 0 || normR == 0 {
		return 0
	}
	return dot / (normQ * normR)
}

func topK(sc []float64, k int) []int {
	idx := make([]int, len(sc))
	for i := range idx {
		idx[i] = i
	}
	for i := 1; i < len(idx); i++ {
		for j := i; j > 0 && sc[idx[j]] > sc[idx[j-1]]; j-- {
			idx[j], idx[j-1] = idx[j-1], idx[j]
		}
	}
	return idx[:k]
}

func jaccardTop(a, b []int) float64 {
	set := map[int]bool{}
	for _, x := range a {
		set[x] = true
	}
	inter, union := 0, len(set)
	for _, x := range b {
		if set[x] {
			inter++
		} else {
			union++
		}
	}
	if union == 0 {
		return 1
	}
	return float64(inter) / float64(union)
}

func uniqueCount(ids []string) int {
	seen := map[string]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	return len(seen)
}

// TestCampaignDedupCharacterization — o experimento B completo (read-only).
func TestCampaignDedupCharacterization(t *testing.T) {
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

	// ── Carregar chunks (todos) ──
	chunkRows, err := eng.db.Conn().Query("SELECT id, document_id, content FROM chunks")
	if err != nil {
		t.Fatal(err)
	}
	chunks := map[string]*dedupChunk{}
	var chunkList []*dedupChunk
	for chunkRows.Next() {
		var id, doc, content string
		if err := chunkRows.Scan(&id, &doc, &content); err != nil {
			t.Fatal(err)
		}
		c := &dedupChunk{id: id, documentID: doc, content: content}
		chunks[id] = c
		chunkList = append(chunkList, c)
	}
	chunkRows.Close()

	// ── Carregar vetores ──
	vecRows, err := eng.db.Conn().Query("SELECT id, document_id, chunk_id, content, vector FROM vectors")
	if err != nil {
		t.Fatal(err)
	}
	var vecs []*dedupVec
	for vecRows.Next() {
		var id, doc, chunkID, content string
		var blob []byte
		if err := vecRows.Scan(&id, &doc, &chunkID, &content, &blob); err != nil {
			t.Fatal(err)
		}
		v := &dedupVec{id: id, documentID: doc, chunkID: chunkID, content: content}
		if len(blob) == dim*4 {
			v.vec = make([]float64, dim)
			for d := 0; d < dim; d++ {
				v.vec[d] = float64(math.Float32frombits(binary.LittleEndian.Uint32(blob[d*4:])))
			}
			for _, x := range v.vec {
				v.normSq += x * x
			}
		}
		vecs = append(vecs, v)
		if c, ok := chunks[chunkID]; ok {
			c.hasVector = true
		}
	}
	vecRows.Close()

	// ── CAT1: conteúdo duplicado (todos os chunks) ──
	contentGroups := map[[32]byte][]string{}
	for _, c := range chunkList {
		h := sha256.Sum256([]byte(c.content))
		contentGroups[h] = append(contentGroups[h], c.id)
	}
	dupContent, dupContentVec := 0, 0
	for _, ids := range contentGroups {
		if len(ids) > 1 {
			dupContent += len(ids) - 1
			groupHasVec := false
			for _, id := range ids {
				if chunks[id].hasVector {
					groupHasVec = true
					dupContentVec++
				}
			}
			_ = groupHasVec
		}
	}

	// ── CAT2: vetor duplicado ──
	vecGroups := map[[32]byte][]*dedupVec{}
	for _, v := range vecs {
		h := sha256.Sum256([]byte(fmtBlob(v.vec)))
		vecGroups[h] = append(vecGroups[h], v)
	}
	dupVec := 0
	intraDoc, crossDoc, sameChunk, diffContent := 0, 0, 0, 0
	maxCopies, groupsOver10, copiesOver10 := 0, 0, 0
	for _, g := range vecGroups {
		if len(g) > maxCopies {
			maxCopies = len(g)
		}
		if len(g) > 10 {
			groupsOver10++
			copiesOver10 += len(g)
		}
		if len(g) < 2 {
			continue
		}
		extra := len(g) - 1
		dupVec += extra
		docs := map[string]bool{}
		chunkIDs := map[string]bool{}
		contents := map[string]bool{}
		for _, v := range g {
			docs[v.documentID] = true
			chunkIDs[v.chunkID] = true
			contents[v.content] = true
		}
		if len(docs) == 1 {
			intraDoc += extra
		} else {
			crossDoc += extra
		}
		if len(chunkIDs) == 1 {
			sameChunk += extra
		}
		if len(contents) > 1 {
			diffContent += extra
		}
	}

	// ── ÓRFÃOS ──
	orphans, orphanDupOfIndexed := 0, 0
	for _, c := range chunkList {
		if !c.hasVector {
			orphans++
			h := sha256.Sum256([]byte(c.content))
			for _, id := range contentGroups[h] {
				if chunks[id].hasVector {
					orphanDupOfIndexed++
					break
				}
			}
		}
	}

	// ── Diversidade top-10 (oracle float32, 200 queries) + simulação ──
	// Queries = as MESMAS da FASE 8 (ORDER BY rowid LIMIT 200) para as
	// medições serem comparáveis (o db muda entre execuções — serve/watcher).
	const nq = 200
	qRows, err := eng.db.Conn().Query(
		"SELECT id, vector FROM vectors WHERE length(vector) = ? ORDER BY rowid LIMIT ?", dim*4, nq)
	if err != nil {
		t.Fatal(err)
	}
	queryIdx := map[string]bool{}
	var queries []*dedupVec
	for qRows.Next() {
		var id string
		var blob []byte
		if err := qRows.Scan(&id, &blob); err != nil {
			t.Fatal(err)
		}
		for _, v := range vecs {
			if v.id == id {
				queries = append(queries, v)
				queryIdx[id] = true
				break
			}
		}
	}
	qRows.Close()
	if len(queries) != nq {
		t.Fatalf("queries=%d want %d", len(queries), nq)
	}

	// Pré-computa normas.
	normQ := make([]float64, len(queries))
	for i, q := range queries {
		var n float64
		for _, x := range q.vec {
			n += x * x
		}
		normQ[i] = math.Sqrt(n)
	}
	normR := make([]float64, len(vecs))
	sumR := make([]int32, len(vecs))
	for i, v := range vecs {
		normR[i] = math.Sqrt(v.normSq)
		var s int32
		for _, x := range v.vec {
			s += int32(quant8(x))
		}
		sumR[i] = s
	}

	// Oracle completo (float32) e int8 completo (mesma metodologia FASE 8).
	var divCompleto, jacCompleto float64
	divContent := 0.0
	for i, q := range queries {
		scF := make([]float64, len(vecs))
		sc8 := make([]float64, len(vecs))
		for j, v := range vecs {
			scF[j] = scoreFloat(q.vec, v.vec, normQ[i], normR[j])
			sc8[j] = scoreInt8(q.vec, v.vec, sumR[j], normQ[i], normR[j])
		}
		topF := topK(scF, 50)
		top8 := topK(sc8, 50)
		jacCompleto += jaccardTop(topF, top8)
		// diversidade top-10 do oracle: ids únicos E conteúdos únicos
		ids := make([]string, 10)
		contents := map[[32]byte]bool{}
		for k, idx := range topF[:10] {
			ids[k] = vecs[idx].id
			contents[sha256.Sum256([]byte(vecs[idx].content))] = true
		}
		divCompleto += float64(uniqueCount(ids))
		divContent += float64(len(contents))
	}

	// ── SIMULAÇÃO de remoção (memória): 1 vetor por grupo ──
	var distinctVecs []*dedupVec
	for _, g := range vecGroups {
		distinctVecs = append(distinctVecs, g[0])
	}
	normD := make([]float64, len(distinctVecs))
	sumD := make([]int32, len(distinctVecs))
	for i, v := range distinctVecs {
		normD[i] = math.Sqrt(v.normSq)
		var s int32
		for _, x := range v.vec {
			s += int32(quant8(x))
		}
		sumD[i] = s
	}
	var jacDedup, divDedup float64
	for i, q := range queries {
		scF := make([]float64, len(distinctVecs))
		sc8 := make([]float64, len(distinctVecs))
		for j, v := range distinctVecs {
			scF[j] = scoreFloat(q.vec, v.vec, normQ[i], normD[j])
			sc8[j] = scoreInt8(q.vec, v.vec, sumD[j], normQ[i], normD[j])
		}
		topF := topK(scF, 50)
		top8 := topK(sc8, 50)
		jacDedup += jaccardTop(topF, top8)
		ids := make([]string, 10)
		for k, idx := range topF[:10] {
			ids[k] = distinctVecs[idx].id
		}
		divDedup += float64(uniqueCount(ids))
	}

	// ── Relatório ──
	nf := float64(nq)
	// CONTROLE: jaccard do ÍNDICE (FASE 8) para as MESMAS queries, na MESMA
	// execução — valida o Go puro contra o caminho de produção.
	store := eng.vecStore.(*vector.SQLiteVec)
	store.SetInt8Enabled(false)
	var jacIndex float64
	for i, q := range queries {
		o, err := store.Search(q.vec, 50)
		if err != nil {
			t.Fatal(err)
		}
		store.SetInt8Enabled(true)
		r, err := store.Search(q.vec, 50)
		if err != nil {
			t.Fatal(err)
		}
		store.SetInt8Enabled(false)
		oid := map[string]bool{}
		for _, x := range o {
			oid[x.ID] = true
		}
		inter, union := 0, len(oid)
		for _, x := range r {
			if oid[x.ID] {
				inter++
			} else {
				union++
			}
		}
		if union > 0 {
			jacIndex += float64(inter) / float64(union)
		}
		_ = i
	}
	t.Logf("CONTROLE índice (FASE 8): jaccard@50=%.4f  vs Go puro=%.4f", jacIndex/nf, jacCompleto/nf)
	t.Logf("── B: deduplicação (READ-ONLY, db real) ──")
	t.Logf("chunks=%d (com vetor=%d, órfãos=%d)  vetores=%d  distintos=%d",
		len(chunkList), len(chunkList)-orphans, orphans, len(vecs), len(distinctVecs))
	t.Logf("CAT1 conteúdo duplicado=%d (%.1f%% dos chunks) — desses, com vetor=%d",
		dupContent, float64(dupContent)*100/float64(len(chunkList)), dupContentVec)
	t.Logf("CAT2 vetor duplicado=%d (%.1f%% dos vetores)", dupVec, float64(dupVec)*100/float64(len(vecs)))
	t.Logf("CAT2b distribuição de cópias: máx=%d cópias  grupos>10 cópias=%d (%d vetores, %.1f%% do índice)",
		maxCopies, groupsOver10, copiesOver10, float64(copiesOver10)*100/float64(len(vecs)))
	t.Logf("CAT3 intra-documento=%d (%.1f%%)  cross-documento=%d (%.1f%%)",
		intraDoc, float64(intraDoc)*100/float64(maxInt(dupVec, 1)), crossDoc, float64(crossDoc)*100/float64(maxInt(dupVec, 1)))
	t.Logf("CAT4 mesmo chunk=%d  CAT5 vetor-igual-com-conteúdo-diferente=%d", sameChunk, diffContent)
	t.Logf("órfãos=%d (%.1f%% dos chunks) — órfãos que duplicam conteúdo já indexado=%d (%.1f%% dos órfãos)",
		orphans, float64(orphans)*100/float64(len(chunkList)), orphanDupOfIndexed, float64(orphanDupOfIndexed)*100/float64(maxInt(orphans, 1)))
	t.Logf("── impacto (200 queries reais) ──")
	t.Logf("oracle completo:   jaccard@50 int8=%.4f  diversidade top-10 ids=%.2f  CONTEÚDOS=%.2f",
		jacCompleto/nf, divCompleto/nf, divContent/nf)
	t.Logf("após dedup (sim.): jaccard@50 int8=%.4f  diversidade top-10 ids=%.2f/10", jacDedup/nf, divDedup/nf)
	t.Logf("índice: %d vetores (%.1f%% menor) → slab int8 %.1fMB (vs %.1fMB) — L3 budget 80MB: %s",
		len(distinctVecs),
		float64(len(vecs)-len(distinctVecs))*100/float64(len(vecs)),
		float64(len(distinctVecs)*dim)/1e6,
		float64(len(vecs)*dim)/1e6,
		func() string {
			if float64(len(distinctVecs)*dim)/1e6 < 80 {
				return "OK (dentro)"
			}
			return "excede"
		}())
	t.Logf("latência estimada pós-dedup: scan %.1f%% do atual (bandwidth-bound ⇒ ∝ N)",
		float64(len(distinctVecs))*100/float64(len(vecs)))
}

// fmtBlob serializa o vetor como float32 little-endian (mesmo formato do db).
func fmtBlob(v []float64) []byte {
	if v == nil {
		return nil
	}
	b := make([]byte, len(v)*4)
	for i, x := range v {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(float32(x)))
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
