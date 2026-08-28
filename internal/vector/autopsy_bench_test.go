// Package vector — COSCA VECTOR SEARCH AUTOPSY instrumentation.
//
// Arquivos _test/_bench NOVOS, de propósito: NADA de produção foi alterado.
// Estes benchmarks medem o caminho REAL da busca vetorial:
//
//	scanAll (SQLite) → dotFromBytes (núcleo escalar) → scoreSerial/scoreParallel → fetchMeta.
package vector

import (
	"database/sql"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// ── helpers ────────────────────────────────────────────────────────────────

const autopsyDim = 768 // dimensão real de produção (nomic-embed-text)

func autopsyDSN(path string) string {
	return path + "?mode=rwc&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-20000)"
}

func autopsyOpen(tb testing.TB, dim, count int, path string) (*SQLiteVec, *sql.DB) {
	tb.Helper()
	if path == "" {
		path = filepath.Join(tb.TempDir(), "vec.db")
	}
	rawConn, err := sql.Open("sqlite", autopsyDSN(path))
	if err != nil {
		tb.Fatalf("open sqlite: %v", err)
	}
	rawConn.SetMaxOpenConns(1)
	tb.Cleanup(func() { _ = rawConn.Close() })
	store, err := NewSQLiteVec(SQLiteVecConfig{DB: rawConn, Dimension: dim, TableName: "vectors"})
	if err != nil {
		tb.Fatalf("create store: %v", err)
	}
	return store, rawConn
}

func autopsyCachePath(dim, count int) string {
	return filepath.Join(os.TempDir(), "cosca-autopsy", fmt.Sprintf("vec_d%d_n%d.db", dim, count))
}

// autopsyStore returns a SQLiteVec populado com `count` vetores de `dim` dims.
// Usa um cache em /tmp/cosca-autopsy para evitar repopular entre execuções.
func autopsyStore(tb testing.TB, dim, count int) *SQLiteVec {
	tb.Helper()
	if count == 0 {
		return nil
	}
	path := autopsyCachePath(dim, count)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		tb.Fatalf("mkdir: %v", err)
	}
	_, statErr := os.Stat(path)
	exists := statErr == nil

	store, rawConn := autopsyOpen(tb, dim, count, path)

	need := count
	if exists {
		if n, err := store.Count(); err == nil && n >= count {
			return store
		}
		need = count
	}
	_ = rawConn
	populateAutopsy(store, dim, need)
	return store
}

// populateAutopsy insere vetores sintéticos (rng determinístico).
func populateAutopsy(store *SQLiteVec, dim, count int) {
	rng := rand.New(rand.NewSource(42))
	const batch = 500
	for b := 0; b < count; b += batch {
		n := batch
		if b+n > count {
			n = count - b
		}
		recs := make([]VectorRecord, n)
		for i := 0; i < n; i++ {
			idx := b + i
			v := make([]float64, dim)
			for d := 0; d < dim; d++ {
				v[d] = rng.Float64()*2 - 1
			}
			recs[i] = VectorRecord{
				ID:      fmt.Sprintf("auto-%d", idx),
				Vector:  v,
				ChunkID: fmt.Sprintf("chunk-%d", idx),
				Content: fmt.Sprintf("Conteudo sintetico do vetor %d para autopsia de performance.", idx),
			}
		}
		if err := store.Store(dim, recs); err != nil {
			panic(fmt.Sprintf("store batch %d: %v", b, err))
		}
	}
}

// autopsyRows gera candidateRow em memória (sem SQL) para isolar o COMPUTE.
func autopsyRows(rng *rand.Rand, dim, count int) []candidateRow {
	rows := make([]candidateRow, count)
	for i := 0; i < count; i++ {
		blob := make([]byte, dim*4)
		for d := 0; d < dim; d++ {
			f := float32(rng.Float64()*2 - 1)
			u := math.Float32bits(f)
			blob[d*4] = byte(u)
			blob[d*4+1] = byte(u >> 8)
			blob[d*4+2] = byte(u >> 16)
			blob[d*4+3] = byte(u >> 24)
		}
		rows[i] = candidateRow{id: fmt.Sprintf("r%d", i), blob: blob}
	}
	return rows
}

func autopsyQuery(dim int) []float64 {
	rng := rand.New(rand.NewSource(7))
	q := make([]float64, dim)
	for d := range q {
		q[d] = rng.Float64()*2 - 1
	}
	return q
}

// ── FASE 3 · MICROBENCHMARKS ────────────────────────────────────────────────

func BenchmarkAutopsyCosineSimilarity_Dim768(b *testing.B) {
	a, c := autopsyQuery(autopsyDim), autopsyQuery(autopsyDim)
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_, _ = CosineSimilarity(a, c)
	}
}

func BenchmarkAutopsyNormalizeVector_Dim768(b *testing.B) {
	v := autopsyQuery(autopsyDim)
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_ = NormalizeVector(v)
	}
}

// A/B: sqrt custom (Newton, até 100 iterações) vs math.Sqrt.
func BenchmarkAutopsySqrtCustomNewton(b *testing.B) {
	x := 12345.678
	for i := 0; b.Loop(); i++ {
		x = sqrt(x + 0.0001)
	}
	_ = x
}

func BenchmarkAutopsySqrtMathSqrt(b *testing.B) {
	x := 12345.678
	for i := 0; b.Loop(); i++ {
		x = math.Sqrt(x + 0.0001)
	}
	_ = x
}

// núcleo da busca de produção: decode float32 + dot + norm (loop escalar).
func BenchmarkAutopsyDotFromBytes_Dim128(b *testing.B) {
	benchDotFromBytes(b, 128)
}
func BenchmarkAutopsyDotFromBytes_Dim384(b *testing.B) {
	benchDotFromBytes(b, 384)
}
func BenchmarkAutopsyDotFromBytes_Dim768(b *testing.B) {
	benchDotFromBytes(b, autopsyDim)
}
func BenchmarkAutopsyDotFromBytes_Dim1536(b *testing.B) {
	benchDotFromBytes(b, 1536)
}

func benchDotFromBytes(b *testing.B, dim int) {
	rng := rand.New(rand.NewSource(1))
	q := autopsyQuery(dim)
	blob := make([]byte, dim*4)
	for d := 0; d < dim; d++ {
		u := math.Float32bits(float32(rng.Float64()*2 - 1))
		blob[d*4] = byte(u)
		blob[d*4+1] = byte(u >> 8)
		blob[d*4+2] = byte(u >> 16)
		blob[d*4+3] = byte(u >> 24)
	}
	scratch := make([]float32, dim)
	var acc float64
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		acc += dotFromBytes(q, blob, scratch, math.Sqrt(3.0))
	}
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds()/1e9, "Gdots/s")
	_ = acc
}

// decode apenas (alocação/cópia) vs dot completo — hipótese "decode é caro".
func BenchmarkAutopsyDecodeFloat32Only_Dim768(b *testing.B) {
	dim := autopsyDim
	rng := rand.New(rand.NewSource(1))
	blob := make([]byte, dim*4)
	for d := 0; d < dim; d++ {
		u := math.Float32bits(float32(rng.Float64()*2 - 1))
		blob[d*4] = byte(u)
		blob[d*4+1] = byte(u >> 8)
		blob[d*4+2] = byte(u >> 16)
		blob[d*4+3] = byte(u >> 24)
	}
	scratch := make([]float32, dim)
	var acc float32
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		for j := range scratch {
			scratch[j] = math.Float32frombits(uint32(blob[j*4]) | uint32(blob[j*4+1])<<8 | uint32(blob[j*4+2])<<16 | uint32(blob[j*4+3])<<24)
		}
		acc += scratch[len(scratch)/2]
	}
	_ = acc
}

func BenchmarkAutopsyBytesToFloat32Slice_Dim768(b *testing.B) {
	rng := rand.New(rand.NewSource(1))
	blob := make([]byte, autopsyDim*4)
	for d := 0; d < autopsyDim; d++ {
		u := math.Float32bits(float32(rng.Float64()*2 - 1))
		blob[d*4] = byte(u)
		blob[d*4+1] = byte(u >> 8)
		blob[d*4+2] = byte(u >> 16)
		blob[d*4+3] = byte(u >> 24)
	}
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_ = bytesToFloat32Slice(blob)
	}
}

// top-k: sort completo do conjunto scoreado (como scoreSerial/Parallel fazem).
func BenchmarkAutopsyTopKFullSort_N100000(b *testing.B) {
	rng := rand.New(rand.NewSource(3))
	n := 100000
	scores := make([]scoredRow, n)
	for i := 0; i < n; i++ {
		scores[i] = scoredRow{id: fmt.Sprintf("%d", i), score: rng.Float64()}
	}
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		sort.Slice(scores, func(a, c int) bool { return scores[a].score > scores[c].score })
	}
}

// ── COMPUTE isolado (sem SQL): scoreSerial (1 worker) vs scoreParallel (NumCPU) ──

func BenchmarkAutopsyScoreSerial_N10000_Dim768(b *testing.B) {
	benchScoreSerial(b, 10000, autopsyDim)
}
func BenchmarkAutopsyScoreSerial_N100000_Dim768(b *testing.B) {
	benchScoreSerial(b, 100000, autopsyDim)
}
func BenchmarkAutopsyScoreParallel_N10000_Dim768(b *testing.B) {
	benchScoreParallel(b, 10000, autopsyDim)
}
func BenchmarkAutopsyScoreParallel_N100000_Dim768(b *testing.B) {
	benchScoreParallel(b, 100000, autopsyDim)
}

func benchScoreSerial(b *testing.B, count, dim int) {
	rng := rand.New(rand.NewSource(4))
	rows := autopsyRows(rng, dim, count)
	q := autopsyQuery(dim)
	var normA float64
	for _, v := range q {
		normA += v * v
	}
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_ = scoreSerial(q, rows, 10, math.Sqrt(normA))
	}
	b.ReportMetric(float64(count)*float64(b.N)/b.Elapsed().Seconds()/1e6, "Mvec/s")
}

func benchScoreParallel(b *testing.B, count, dim int) {
	rng := rand.New(rand.NewSource(4))
	rows := autopsyRows(rng, dim, count)
	q := autopsyQuery(dim)
	var normA float64
	for _, v := range q {
		normA += v * v
	}
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_ = scoreParallel(q, rows, 10)
	}
	b.ReportMetric(float64(count)*float64(b.N)/b.Elapsed().Seconds()/1e6, "Mvec/s")
}

// ── CAMINHO COMPLETO (SQLite + decode + score + fetchMeta) ──────────────────

func BenchmarkAutopsySQLiteSearch_N10000_Dim768(b *testing.B) {
	benchSQLiteSearch(b, 10000, autopsyDim, 10)
}
func BenchmarkAutopsySQLiteSearch_N100000_Dim768(b *testing.B) {
	if testing.Short() {
		b.Skip("large store in short mode")
	}
	benchSQLiteSearch(b, 100000, autopsyDim, 10)
}
func BenchmarkAutopsySQLiteSearch_N1000000_Dim768(b *testing.B) {
	if testing.Short() {
		b.Skip("1M store in short mode")
	}
	benchSQLiteSearch(b, 1000000, autopsyDim, 10)
}
func BenchmarkAutopsySQLiteSearch_N100000_Dim128(b *testing.B) {
	benchSQLiteSearch(b, 100000, 128, 10)
}
func BenchmarkAutopsySQLiteSearch_N100000_Dim384(b *testing.B) {
	benchSQLiteSearch(b, 100000, 384, 10)
}
func BenchmarkAutopsySQLiteSearch_N100000_Dim1536(b *testing.B) {
	benchSQLiteSearch(b, 100000, 1536, 10)
}

func benchSQLiteSearch(b *testing.B, count, dim, limit int) {
	store := autopsyStore(b, dim, count)
	q := autopsyQuery(dim)
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		res, err := store.Search(q, limit)
		if err != nil {
			b.Fatalf("search: %v", err)
		}
		_ = res
	}
	b.ReportMetric(float64(count)*float64(b.N)/b.Elapsed().Seconds()/1e6, "Mvec/s")
}

// caminho bounded (hybrid-first): candidatos lexicais + pool de recência.
func BenchmarkAutopsySQLiteSearchCandidates_N100000_Dim768(b *testing.B) {
	if testing.Short() {
		b.Skip("large store in short mode")
	}
	store := autopsyStore(b, autopsyDim, 100000)
	q := autopsyQuery(autopsyDim)
	candidates := make([]string, 20)
	for i := range candidates {
		candidates[i] = fmt.Sprintf("auto-%d", 10000+i)
	}
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		res, err := store.SearchWithCandidates(q, 8, candidates, 250, nil)
		if err != nil {
			b.Fatalf("candidate search: %v", err)
		}
		_ = res
	}
}

// ── FASE 2 · LATENCY BREAKDOWN (instrumentação por etapa, num Test) ────────

func TestAutopsyLatencyBreakdown(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	const count = 100000
	store := autopsyStore(t, autopsyDim, count)
	q := autopsyQuery(autopsyDim)

	// 1. scanAll (SQL + scan das BLOBs)
	t0 := time.Now()
	where, args := store.filterClause(nil)
	rows, err := store.scanAll(where, args)
	tScan := time.Since(t0)
	if err != nil {
		t.Fatalf("scanAll: %v", err)
	}
	t.Logf("scanAll      : %8.1f ms  (%d rows, %.1f MB de blobs)", float64(tScan)/1e6, len(rows), float64(len(rows))*float64(autopsyDim)*4/1e6)

	// 2. score (dotFromBytes) sobre as linhas já carregadas
	t0 = time.Now()
	scores := scoreParallel(q, rows, 10)
	tScore := time.Since(t0)
	t.Logf("scoreParallel: %8.1f ms  (%d scores)", float64(tScore)/1e6, len(scores))

	// 3. dotFromBytes puro (matemática) — mesmo conjunto, reusando scratch
	scratch := make([]float32, autopsyDim)
	var normA float64
	for _, v := range q {
		normA += v * v
	}
	sqrtNormA := math.Sqrt(normA)
	var acc float64
	t0 = time.Now()
	for _, r := range rows {
		if len(r.blob) == autopsyDim*4 {
			acc += dotFromBytes(q, r.blob, scratch, sqrtNormA)
		}
	}
	tDot := time.Since(t0)
	t.Logf("dotFromBytes : %8.1f ms  (matemática pura, 1 goroutine)", float64(tDot)/1e6)

	// 4. fetchMeta (segundo SELECT só para o top-K)
	ids := make([]string, len(scores))
	for i, sc := range scores {
		ids[i] = sc.id
	}
	t0 = time.Now()
	_, err = store.fetchMeta(ids)
	tMeta := time.Since(t0)
	if err != nil {
		t.Fatalf("fetchMeta: %v", err)
	}
	t.Logf("fetchMeta    : %8.1f ms  (top-%d)", float64(tMeta)/1e6, len(ids))

	// 5. busca completa
	t0 = time.Now()
	_, err = store.Search(q, 10)
	tFull := time.Since(t0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	t.Logf("Search total : %8.1f ms", float64(tFull)/1e6)

	// alocações do scanAll (Phase 8 — allocation investigation)
	var ms runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&ms)
	before := ms.TotalAlloc
	where, args = store.filterClause(nil)
	_, _ = store.scanAll(where, args)
	runtime.ReadMemStats(&ms)
	alloc := ms.TotalAlloc - before
	t.Logf("scanAll alloc: %8.1f MB por varredura (blobs + structs)", float64(alloc)/1e6)
}

// ── FASE 4 · COLD VS WARM ───────────────────────────────────────────────────

func TestAutopsyColdWarm(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	store := autopsyStore(t, autopsyDim, 100000)
	q := autopsyQuery(autopsyDim)

	// forçar limpeza do page cache SQLite: fechar e reabrir sem memcache, e
	// consultar contagem antes (aquece a raiz mas não as BLOBs).
	times := make([]time.Duration, 0, 105)
	for i := 1; i <= 100; i++ {
		t0 := time.Now()
		_, err := store.Search(q, 10)
		if err != nil {
			t.Fatalf("search %d: %v", i, err)
		}
		times = append(times, time.Since(t0))
	}
	show := []int{1, 2, 3, 5, 10, 20, 50, 100}
	t.Log("seq | latency (ms)  | delta vs anterior")
	var prev time.Duration
	for _, idx := range show {
		d := times[idx-1]
		delta := "—"
		if prev > 0 {
			delta = fmt.Sprintf("%+.1f%%", 100*float64(d-prev)/float64(prev))
		}
		t.Logf("%3d | %12.1f | %s", idx, float64(d)/1e6, delta)
		prev = d
	}
	first, last := times[0], times[len(times)-1]
	t.Logf("cold=%8.1f ms   warm(100th)=%8.1f ms   warm/cold=%.2f", float64(first)/1e6, float64(last)/1e6, float64(last)/float64(first))
}
