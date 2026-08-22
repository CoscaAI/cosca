// Package knowledge — campanha performance, L360: EXECUÇÃO da limpeza
// histórica (AUTORIZADA pelo Don após L359). Backup prévio obrigatório
// (.cosca/backup/knowledge-pre-cleanup-*.db). Reversível via backup.
//
// Operação (nada é irreversível):
//  1. Migração leve: chunks.is_trivial / chunks.dedup_of (marcação).
//  2. Marcação: triviais (T1-T4) e duplicatas de conteúdo (1 canônico).
//  3. Remoção dos VETORES de lixo via store.Delete (derivados, re-embedáveis;
//     os CHUNKS permanecem com a provenance — nada de conteúdo é perdido).
//  4. Validação: contagens vs dry-run (L359: ~9.674) + recall spot-check.
package knowledge

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/cache"
	"github.com/CoscaAI/cosca/internal/chunker"
	"github.com/CoscaAI/cosca/internal/vector"
)

func TestCampaignCleanupExecute(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	root := "/home/cosca/Documents/cosca"
	coscaDir := filepath.Join(root, ".cosca")

	// Pré-requisito: backup existe.
	backups, err := filepath.Glob(filepath.Join(coscaDir, "backup", "knowledge-pre-cleanup-*.db"))
	if err != nil || len(backups) == 0 {
		t.Skip("teste operacional one-shot — requer backup real pré-existente do ambiente de produção (/home/cosca/Documents/cosca/.cosca/backup/knowledge-pre-cleanup-*.db); sem backup a regra 'backup antes de transformar' impede a execução")
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

	conn := eng.db.Conn()
	var nBefore int
	if err := conn.QueryRow("SELECT count(*) FROM vectors WHERE length(vector) = 768*4").Scan(&nBefore); err != nil {
		t.Fatal(err)
	}

	// ── 1. Migração leve (idempotente) ──
	cols := map[string]bool{}
	rows, err := conn.Query("PRAGMA table_info(chunks)")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		cols[name] = true
	}
	rows.Close()
	if !cols["is_trivial"] {
		if _, err := conn.Exec("ALTER TABLE chunks ADD COLUMN is_trivial INTEGER DEFAULT 0"); err != nil {
			t.Fatal(err)
		}
	}
	if !cols["dedup_of"] {
		if _, err := conn.Exec("ALTER TABLE chunks ADD COLUMN dedup_of TEXT DEFAULT ''"); err != nil {
			t.Fatal(err)
		}
	}

	// ── 2. Marcação de triviais (T1-T4) ──
	cRows, err := conn.Query("SELECT id, content FROM chunks WHERE is_trivial = 0")
	if err != nil {
		t.Fatal(err)
	}
	var trivialIDs []string
	var trivialVecIDs []string
	for cRows.Next() {
		var id, content string
		if err := cRows.Scan(&id, &content); err != nil {
			t.Fatal(err)
		}
		if ok, _ := chunker.IsTrivial(content); ok {
			trivialIDs = append(trivialIDs, id)
		}
	}
	cRows.Close()
	trivialCount := len(trivialIDs)
	for i := 0; i < len(trivialIDs); i += 500 {
		batch := trivialIDs[i:minInt(i+500, len(trivialIDs))]
		ph := ""
		args := make([]interface{}, 0, len(batch))
		for j, id := range batch {
			if j > 0 {
				ph += ","
			}
			ph += "?"
			args = append(args, id)
		}
		if _, err := conn.Exec("UPDATE chunks SET is_trivial = 1 WHERE id IN ("+ph+")", args...); err != nil {
			t.Fatal(err)
		}
	}
	// Vetores dos triviais (a remover).
	vRows, err := conn.Query("SELECT id FROM vectors WHERE chunk_id IN (SELECT id FROM chunks WHERE is_trivial = 1)")
	if err != nil {
		t.Fatal(err)
	}
	for vRows.Next() {
		var id string
		if err := vRows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		trivialVecIDs = append(trivialVecIDs, id)
	}
	vRows.Close()

	// ── 3. Marcação de duplicatas de conteúdo (com vetor, não triviais) ──
	dRows, err := conn.Query(
		"SELECT v.chunk_id, c.content FROM vectors v JOIN chunks c ON c.id = v.chunk_id " +
			"WHERE c.is_trivial = 0 AND length(v.vector) = 768*4 ORDER BY v.rowid")
	if err != nil {
		t.Fatal(err)
	}
	type dupe struct {
		chunkID string
		canon   string
	}
	var dupes []dupe
	seen := map[[32]byte]string{}
	for dRows.Next() {
		var chunkID, content string
		if err := dRows.Scan(&chunkID, &content); err != nil {
			t.Fatal(err)
		}
		h := sha256.Sum256([]byte(content))
		if canon, ok := seen[h]; ok {
			dupes = append(dupes, dupe{chunkID: chunkID, canon: canon})
		} else {
			seen[h] = chunkID
		}
	}
	dRows.Close()
	for _, d := range dupes {
		if _, err := conn.Exec("UPDATE chunks SET dedup_of = ? WHERE id = ?", d.canon, d.chunkID); err != nil {
			t.Fatal(err)
		}
	}
	// Vetores das duplicatas não-canônicas (a remover).
	var dupeVecIDs []string
	uRows, err := conn.Query(
		"SELECT id FROM vectors WHERE chunk_id IN (SELECT id FROM chunks WHERE dedup_of != '')")
	if err != nil {
		t.Fatal(err)
	}
	for uRows.Next() {
		var id string
		if err := uRows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		dupeVecIDs = append(dupeVecIDs, id)
	}
	uRows.Close()

	// ── 4. Remoção dos vetores de lixo (via store → version bump) ──
	store := eng.vecStore.(*vector.SQLiteVec)
	toDelete := append(trivialVecIDs, dupeVecIDs...)
	if err := store.Delete(toDelete); err != nil {
		t.Fatal(err)
	}

	// ── 5. Validação ──
	var nAfter int
	if err := conn.QueryRow("SELECT count(*) FROM vectors WHERE length(vector) = 768*4").Scan(&nAfter); err != nil {
		t.Fatal(err)
	}
	t.Logf("── L360 EXECUÇÃO da limpeza (autorizada; backup=%s) ──", filepath.Base(backups[0]))
	t.Logf("triviais marcados=%d (vetores removidos=%d)  duplicatas marcadas=%d (vetores removidos=%d)",
		trivialCount, len(trivialVecIDs), len(dupes), len(dupeVecIDs))
	t.Logf("vetores: %d → %d  (-%d, %.1f%%)  [dry-run L359 esperava ~9.674]",
		nBefore, nAfter, nBefore-nAfter, float64(nBefore-nAfter)*100/float64(nBefore))
	if nAfter < 9000 || nAfter > 10500 {
		t.Errorf("nAfter=%d fora do esperado (~9.674)", nAfter)
	}
	// Spot-check: recall oracle no corpus limpo (int8/int16 determinístico).
	cleanupRecall(t, eng)
}

func cleanupRecall(t *testing.T, eng *Engine) {
	t.Helper()
	const dim, nq = 768, 100
	qRows, err := eng.db.Conn().Query(
		"SELECT vector FROM vectors WHERE length(vector) = ? ORDER BY rowid LIMIT ?", dim*4, nq)
	if err != nil {
		t.Fatal(err)
	}
	defer qRows.Close()
	var qs [][]float64
	for qRows.Next() {
		var blob []byte
		if err := qRows.Scan(&blob); err != nil {
			t.Fatal(err)
		}
		v := make([]float64, dim)
		for d := 0; d < dim; d++ {
			v[d] = float64(math.Float32frombits(binary.LittleEndian.Uint32(blob[d*4:])))
		}
		qs = append(qs, v)
	}
	allRows, err := eng.db.Conn().Query(
		"SELECT vector FROM vectors WHERE length(vector) = ? ORDER BY rowid", dim*4)
	if err != nil {
		t.Fatal(err)
	}
	defer allRows.Close()
	var all [][]float64
	for allRows.Next() {
		var blob []byte
		if err := allRows.Scan(&blob); err != nil {
			t.Fatal(err)
		}
		v := make([]float64, dim)
		for d := 0; d < dim; d++ {
			v[d] = float64(math.Float32frombits(binary.LittleEndian.Uint32(blob[d*4:])))
		}
		all = append(all, v)
	}
	var jac8, jac16 float64
	for _, q := range qs {
		normQ := math.Sqrt(dotSelf64(q))
		scF := make([]float64, len(all))
		sc8 := make([]float64, len(all))
		sc16 := make([]float64, len(all))
		for j, v := range all {
			nr := math.Sqrt(dotSelf64(v))
			scF[j] = scoreFloat(q, v, normQ, nr)
			var sumR int32
			for _, x := range v {
				sumR += int32(quant8(x))
			}
			sc8[j] = scoreInt8(q, v, sumR, normQ, nr)
			sc16[j] = scoreInt16(q, v, normQ, nr)
		}
		topF := topK(scF, 50)
		jac8 += jaccardTop(topF, topK(sc8, 50))
		jac16 += jaccardTop(topF, topK(sc16, 50))
	}
	nf := float64(len(qs))
	t.Logf("recall pós-limpeza (spot-check %d queries): int8 jaccard=%.4f  int16 jaccard=%.4f",
		len(qs), jac8/nf, jac16/nf)
}

func dotSelf64(v []float64) float64 {
	var s float64
	for _, x := range v {
		s += x * x
	}
	return s
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
