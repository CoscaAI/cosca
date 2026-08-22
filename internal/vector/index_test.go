// Package vector — testes do índice in-memory (otimização #1, L311).
//
// Garantias verificadas aqui:
//   - Paridade funcional: o índice retorna os MESMOS top-K, na MESMA ordem,
//     com os MESMOS scores (bit-exact) do caminho SQL original.
//   - Carregamento correto: ids e valores float32 idênticos ao store.
//   - Invalidação: Store/Delete/StoreTx+StoreTxCommitted/DeleteByDocument forçam
//     recarga; o snapshot nunca baka uma escrita não-commitada como final.
//   - Concorrência: leituras paralelas + escritas simultâneas (race detector).
//   - Fail-safe: fallback para o SQL quando o índice não serve a query.
package vector

import (
	"database/sql"
	"fmt"
	"math"
	"math/rand"
	"path/filepath"
	"sync"
	"testing"
)

// ── helpers ────────────────────────────────────────────────────────────────

func idxRandVec(rng *rand.Rand, dim int) []float64 {
	v := make([]float64, dim)
	for d := range v {
		v[d] = rng.Float64()*2 - 1
	}
	return v
}

func idxPopulateTB(t testing.TB, store *SQLiteVec, dim, count int) {
	t.Helper()
	rng := rand.New(rand.NewSource(42))
	const batch = 200
	for b := 0; b < count; b += batch {
		n := batch
		if b+n > count {
			n = count - b
		}
		recs := make([]VectorRecord, n)
		for i := 0; i < n; i++ {
			idx := b + i
			recs[i] = VectorRecord{
				ID:         fmt.Sprintf("idx-%d", idx),
				Vector:     idxRandVec(rng, dim),
				DocumentID: fmt.Sprintf("doc-%d", idx%7),
				ChunkID:    fmt.Sprintf("chunk-%d", idx),
				Content:    fmt.Sprintf("Conteudo sintetico do vetor %d.", idx),
				Metadata:   map[string]string{"k": fmt.Sprintf("%d", idx)},
			}
		}
		if err := store.Store(dim, recs); err != nil {
			t.Fatalf("store batch %d: %v", b, err)
		}
	}
}

// idxStore returns a fresh in-memory store with `count` deterministic vectors.
// The int8 fast path is OFF: these oracle tests assert bit-exact parity with
// the SQL path (the exact float32 contract). Fast-path tests enable it
// explicitly (see int8_recall_test.go).
func idxStore(t *testing.T, dim, count int) *SQLiteVec {
	t.Helper()
	store := newTestSQLiteVec(t, dim)
	store.SetInt8Enabled(false)
	store.SetInt16Enabled(false)
	idxPopulateTB(t, store, dim, count)
	return store
}

// idxConcurrentStore opens a file-backed store limited to a single connection
// so the concurrent test exercises OUR locking, not modernc's :memory: pool.
func idxConcurrentStore(t testing.TB, dim int) *SQLiteVec {
	t.Helper()
	path := filepath.Join(t.TempDir(), "idx.db")
	raw, err := sql.Open("sqlite", path+"?mode=rwc&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	raw.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = raw.Close() })
	store, err := NewSQLiteVec(SQLiteVecConfig{DB: raw, Dimension: dim, TableName: "vectors"})
	if err != nil {
		t.Fatalf("NewSQLiteVec: %v", err)
	}
	store.SetInt8Enabled(false)
	store.SetInt16Enabled(false) // oracle tests: contrato exato bit-identical
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func idxAssertEqualResults(t *testing.T, want, got []SearchResult, label string) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("%s: len(want)=%d len(got)=%d", label, len(want), len(got))
	}
	for i := range want {
		w, g := want[i], got[i]
		if w.ID != g.ID {
			t.Fatalf("%s[%d]: ID want=%q got=%q", label, i, w.ID, g.ID)
		}
		if w.Score != g.Score {
			t.Fatalf("%s[%d]: score want=%v got=%v (id=%q)", label, i, w.Score, g.Score, w.ID)
		}
		if w.DocumentID != g.DocumentID || w.ChunkID != g.ChunkID || w.EntityID != g.EntityID || w.Content != g.Content {
			t.Fatalf("%s[%d]: fields differ want=(doc=%q chunk=%q entity=%q) got=(doc=%q chunk=%q entity=%q)", label,
				i, w.DocumentID, w.ChunkID, w.EntityID, g.DocumentID, g.ChunkID, g.EntityID)
		}
	}
}

// ── Paridade funcional: índice vs caminho SQL ──────────────────────────────

func TestIndexParitySQLPath(t *testing.T) {
	for _, tc := range []struct {
		name  string
		dim   int
		count int
	}{
		{"serial_dim8", 8, 100},    // n < 256 → scoreRange serial
		{"parallel_dim8", 8, 1000}, // n >= 256 → scoreParallel
		{"parallel_dim768", 768, 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := idxStore(t, tc.dim, tc.count)
			rng := rand.New(rand.NewSource(7))
			q := idxRandVec(rng, tc.dim)

			// BEFORE-equivalent: caminho SQL puro (índice desligado).
			store.indexEnabled.Store(false)
			sqlRes, err := store.Search(q, 10)
			if err != nil {
				t.Fatalf("sql search: %v", err)
			}

			// AFTER: índice in-memory (ligado, warm).
			store.indexEnabled.Store(true)
			idxRes, err := store.Search(q, 10)
			if err != nil {
				t.Fatalf("index search: %v", err)
			}

			idxAssertEqualResults(t, sqlRes, idxRes, "parity")
		})
	}
}

func TestIndexParitySQLPathCold(t *testing.T) {
	// Mesma paridade, mas com a PRIMEIRA busca (cold — o índice é construído
	// no primeiro acesso e o resultado deve ser idêntico ao caminho SQL).
	store := idxStore(t, 8, 500)
	q := idxRandVec(rand.New(rand.NewSource(9)), 8)

	store.indexEnabled.Store(false)
	sqlRes, err := store.Search(q, 5)
	if err != nil {
		t.Fatalf("sql search: %v", err)
	}
	store.indexEnabled.Store(true)
	idxRes, err := store.Search(q, 5) // cold: constrói o índice aqui
	if err != nil {
		t.Fatalf("index search: %v", err)
	}
	idxAssertEqualResults(t, sqlRes, idxRes, "cold parity")
}

// ── Carregamento correto ───────────────────────────────────────────────────

func TestIndexLoadCorrectness(t *testing.T) {
	const dim, count = 8, 300
	store := idxStore(t, dim, count)

	// Primeira busca força o load.
	if _, err := store.Search(idxRandVec(rand.New(rand.NewSource(1)), dim), 5); err != nil {
		t.Fatalf("search: %v", err)
	}

	gen := store.index.gen.Load()
	if gen == nil {
		t.Fatal("index generation not built after search")
	}
	if gen.n != count {
		t.Fatalf("gen.n = %d, want %d", gen.n, count)
	}
	if gen.dim != dim {
		t.Fatalf("gen.dim = %d, want %d", gen.dim, dim)
	}
	if len(gen.vecs) != count*dim {
		t.Fatalf("vecs len = %d, want %d", len(gen.vecs), count*dim)
	}
	if store.index.count() != count {
		t.Fatalf("index.count() = %d, want %d", store.index.count(), count)
	}

	// Compara float32 decodificado vs BLOB no store (amostra).
	rows, err := store.scanAll("", nil)
	if err != nil {
		t.Fatalf("scanAll: %v", err)
	}
	wantByID := make(map[string][]byte, len(rows))
	for _, r := range rows {
		wantByID[r.id] = r.blob
	}
	gotByID := make(map[string]int, gen.n)
	for i, id := range gen.ids {
		gotByID[id] = i
	}
	if len(gotByID) != count {
		t.Fatalf("unique ids in index = %d, want %d", len(gotByID), count)
	}
	checked := 0
	for id, blob := range wantByID {
		row, ok := gotByID[id]
		if !ok {
			t.Fatalf("id %q missing from index", id)
		}
		base := row * dim
		for d := 0; d < dim; d++ {
			want := math.Float32frombits(uint32(blob[d*4]) | uint32(blob[d*4+1])<<8 | uint32(blob[d*4+2])<<16 | uint32(blob[d*4+3])<<24)
			if gen.vecs[base+d] != want {
				t.Fatalf("id %q dim %d: index=%v blob=%v", id, d, gen.vecs[base+d], want)
			}
		}
		checked++
		if checked >= 50 {
			break
		}
	}
	if store.index.memoryBytes() <= 0 {
		t.Fatal("index.memoryBytes() should be positive")
	}
}

// ── Invalidação e recarga ──────────────────────────────────────────────────

func TestIndexInvalidationStore(t *testing.T) {
	const dim = 8
	store := idxStore(t, dim, 50)
	qKeep := idxRandVec(rand.New(rand.NewSource(3)), dim)
	qNew := make([]float64, dim)
	qNew[0] = 1.0

	if _, err := store.Search(qKeep, 5); err != nil {
		t.Fatalf("search: %v", err)
	}
	if store.index.count() != 50 {
		t.Fatalf("count = %d, want 50", store.index.count())
	}

	// Store mais vetores → versão bumpada → próxima busca recarrega e vê tudo.
	added := []VectorRecord{
		{ID: "new-vec", Vector: qNew, DocumentID: "doc-new", ChunkID: "chunk-new", Content: "novo"},
	}
	if err := store.Store(dim, added); err != nil {
		t.Fatalf("store: %v", err)
	}
	if store.index.count() != 50 {
		t.Fatalf("count after Store (before search) = %d, want 50 (lazy reload)", store.index.count())
	}
	res, err := store.Search(qNew, 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if store.index.count() != 51 {
		t.Fatalf("count after reload = %d, want 51", store.index.count())
	}
	found := false
	for _, r := range res {
		if r.ID == "new-vec" {
			found = true
		}
	}
	if !found {
		t.Fatalf("new vector not found after invalidation reload: %+v", res)
	}
}

func TestIndexInvalidationDelete(t *testing.T) {
	const dim = 8
	store := idxStore(t, dim, 50)
	if _, err := store.Search(idxRandVec(rand.New(rand.NewSource(4)), dim), 5); err != nil {
		t.Fatalf("search: %v", err)
	}
	if store.index.count() != 50 {
		t.Fatalf("count = %d, want 50", store.index.count())
	}

	if err := store.Delete([]string{"idx-0", "idx-1", "idx-2"}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.Search(idxRandVec(rand.New(rand.NewSource(4)), dim), 5); err != nil {
		t.Fatalf("search: %v", err)
	}
	if store.index.count() != 47 {
		t.Fatalf("count after delete reload = %d, want 47", store.index.count())
	}
}

func TestIndexInvalidationDeleteByDocument(t *testing.T) {
	const dim = 8
	store := idxStore(t, dim, 30)
	if _, err := store.Search(idxRandVec(rand.New(rand.NewSource(5)), dim), 5); err != nil {
		t.Fatalf("search: %v", err)
	}
	if err := store.DeleteByDocument("doc-0"); err != nil {
		t.Fatalf("delete by doc: %v", err)
	}
	if _, err := store.Search(idxRandVec(rand.New(rand.NewSource(5)), dim), 5); err != nil {
		t.Fatalf("search: %v", err)
	}
	if store.index.count() != 30-5 { // doc-0 has idx 0,7,14,21,28
		t.Fatalf("count after DeleteByDocument = %d, want %d", store.index.count(), 30-5)
	}
}

func TestIndexInvalidationStoreTxCommitted(t *testing.T) {
	const dim = 8
	store := idxStore(t, dim, 40)
	if _, err := store.Search(idxRandVec(rand.New(rand.NewSource(6)), dim), 5); err != nil {
		t.Fatalf("search: %v", err)
	}

	// StoreTx staged em tx externa + commit → StoreTxCommitted deve invalidar.
	tx, err := store.db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := store.StoreTx(tx, dim, []VectorRecord{
		{ID: "tx-vec", Vector: []float64{1, 0, 0, 0, 0, 0, 0, 0}, DocumentID: "doc-tx", ChunkID: "chunk-tx", Content: "tx"},
	}); err != nil {
		t.Fatalf("StoreTx: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	store.StoreTxCommitted()

	res, err := store.Search([]float64{1, 0, 0, 0, 0, 0, 0, 0}, 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if store.index.count() != 41 {
		t.Fatalf("count after StoreTx commit reload = %d, want 41", store.index.count())
	}
	found := false
	for _, r := range res {
		if r.ID == "tx-vec" {
			found = true
		}
	}
	if !found {
		t.Fatalf("tx vector not found after StoreTxCommitted reload: %+v", res)
	}
}

func TestIndexStoreTxNoCommitDoesNotGoStale(t *testing.T) {
	// StoreTx que ROLLA BACK não deve mudar o store: a versão bumpada no
	// StoreTx é compensada por uma recarga que lê o estado real (inalterado).
	const dim = 8
	store := idxStore(t, dim, 20)
	q := idxRandVec(rand.New(rand.NewSource(11)), dim)
	if _, err := store.Search(q, 5); err != nil {
		t.Fatalf("search: %v", err)
	}
	before := store.index.count()

	tx, err := store.db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := store.StoreTx(tx, dim, []VectorRecord{{ID: "rollback-vec", Vector: q}}); err != nil {
		t.Fatalf("StoreTx: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	res, err := store.Search(q, 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if store.index.count() != before {
		t.Fatalf("count after rollback = %d, want %d", store.index.count(), before)
	}
	for _, r := range res {
		if r.ID == "rollback-vec" {
			t.Fatalf("rolled-back vector must not be searchable")
		}
	}
}

// ── Fallbacks / casos limite ───────────────────────────────────────────────

func TestIndexEmptyStore(t *testing.T) {
	store := newTestSQLiteVec(t, 8)
	res, err := store.Search(idxRandVec(rand.New(rand.NewSource(2)), 8), 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("got %d results, want 0", len(res))
	}
}

func TestIndexDimensionMismatchFallback(t *testing.T) {
	store := idxStore(t, 4, 10)
	// Query 2D contra store 4D: sem resultados, como o caminho SQL.
	res, err := store.Search([]float64{1, 0}, 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("got %d results, want 0 (dimension mismatch)", len(res))
	}
}

func TestIndexFilterStillUsesSQL(t *testing.T) {
	store := newTestSQLiteVec(t, 2)
	if err := store.Store(2, []VectorRecord{
		{ID: "v1", Vector: []float64{1, 0}, Metadata: map[string]string{"type": "agent"}},
		{ID: "v2", Vector: []float64{0, 1}, Metadata: map[string]string{"type": "skill"}},
	}); err != nil {
		t.Fatalf("store: %v", err)
	}
	res, err := store.SearchWithFilter([]float64{1, 0}, 10, map[string]string{"type": "agent"})
	if err != nil {
		t.Fatalf("SearchWithFilter: %v", err)
	}
	if len(res) != 1 || res[0].ID != "v1" {
		t.Fatalf("filter result = %+v, want [v1]", res)
	}
}

func TestIndexRebuildInvalidation(t *testing.T) {
	store := idxStore(t, 8, 20)
	if _, err := store.Search(idxRandVec(rand.New(rand.NewSource(8)), 8), 5); err != nil {
		t.Fatalf("search: %v", err)
	}
	if store.index.count() != 20 {
		t.Fatalf("count = %d, want 20", store.index.count())
	}
	if err := store.Rebuild(); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if _, err := store.Search(idxRandVec(rand.New(rand.NewSource(8)), 8), 5); err != nil {
		t.Fatalf("search: %v", err)
	}
	if store.index.count() != 0 {
		t.Fatalf("count after rebuild = %d, want 0", store.index.count())
	}
}

// ── Concorrência (race detector) ───────────────────────────────────────────

func TestIndexConcurrentReadWrite(t *testing.T) {
	const dim, count = 8, 200
	store := idxConcurrentStore(t, dim)
	idxPopulateTB(t, store, dim, count)

	var wg sync.WaitGroup
	// 8 leitores concorrentes.
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(seed))
			for i := 0; i < 40; i++ {
				q := idxRandVec(rng, dim)
				res, err := store.Search(q, 5)
				if err != nil {
					t.Errorf("search: %v", err)
					return
				}
				if len(res) > 5 {
					t.Errorf("got %d results, want <= 5", len(res))
					return
				}
			}
		}(int64(w + 1))
	}

	// 2 escritores intercalados.
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(seed))
			for i := 0; i < 10; i++ {
				if err := store.Store(dim, []VectorRecord{{
					ID:      fmt.Sprintf("conc-%d-%d", seed, i),
					Vector:  idxRandVec(rng, dim),
					ChunkID: fmt.Sprintf("c-%d", i),
				}}); err != nil {
					t.Errorf("store: %v", err)
					return
				}
				store.StoreTxCommitted()
				if err := store.Delete([]string{fmt.Sprintf("conc-%d-%d", seed, i)}); err != nil {
					t.Errorf("delete: %v", err)
					return
				}
			}
		}(int64(100 + w))
	}

	wg.Wait()
}

func TestIndexConcurrentLoads(t *testing.T) {
	// Várias goroutines buscando ao MESMO TEMPO no primeiro acesso (cold): o
	// reload deve ser serializado e todas devem ver o mesmo resultado.
	const dim = 8
	store := idxConcurrentStore(t, dim) // file-backed, conexão única
	idxPopulateTB(t, store, dim, 300)
	q := idxRandVec(rand.New(rand.NewSource(13)), dim)

	store.indexEnabled.Store(false)
	want, err := store.Search(q, 10)
	if err != nil {
		t.Fatalf("sql search: %v", err)
	}

	store.indexEnabled.Store(true)
	store.index.invalidate() // garante cold (nenhum snapshot ainda)

	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for w := 0; w < 16; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := store.Search(q, 10)
			if err != nil {
				errs <- err
				return
			}
			if len(got) != len(want) {
				errs <- fmt.Errorf("len = %d, want %d", len(got), len(want))
				return
			}
			for i := range want {
				if got[i].ID != want[i].ID || got[i].Score != want[i].Score {
					errs <- fmt.Errorf("result %d differs", i)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent cold search: %v", err)
	}
}
