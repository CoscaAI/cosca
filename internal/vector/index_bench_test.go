// Package vector — benchmarks do índice in-memory (otimização #1, L311).
//
// BEFORE vs AFTER sobre o MESMO dataset (cache /tmp/cosca-autopsy):
//   - BEFORE (caminho SQL puro): BenchmarkIndexSQLPath_* — índice DESLIGADO,
//     exatamente o comportamento pré-otimização (scanAll + score + fetchMeta).
//   - AFTER (índice in-memory): BenchmarkIndexSearch_* — busca quente sobre o
//     slab float32 contíguo (SoA), sem re-decodificar via SQLite.
//
// Também mede: custo de LOAD (uma vez) vs SEARCH (cada query), cold (primeira
// busca pós-load) vs warm, e escalabilidade de paralelismo (1/4/8/16 workers).
package vector

import (
	"math"
	"testing"
)

// ── BEFORE — caminho SQL puro (índice desligado) ───────────────────────────

func BenchmarkIndexSQLPath_N10000_Dim768(b *testing.B)  { benchSQLPath(b, 10000, autopsyDim, 10) }
func BenchmarkIndexSQLPath_N100000_Dim768(b *testing.B) { benchSQLPath(b, 100000, autopsyDim, 10) }
func BenchmarkIndexSQLPath_N1000000_Dim768(b *testing.B) {
	if testing.Short() {
		b.Skip("large store in short mode")
	}
	benchSQLPath(b, 1000000, autopsyDim, 10)
}

func benchSQLPath(b *testing.B, count, dim, limit int) {
	store := autopsyStore(b, dim, count)
	store.indexEnabled.Store(false)
	q := autopsyQuery(dim)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := store.Search(q, limit)
		if err != nil {
			b.Fatalf("search: %v", err)
		}
		_ = res
	}
	b.ReportMetric(float64(count)*float64(b.N)/b.Elapsed().Seconds()/1e6, "Mvec/s")
}

// ── AFTER — índice in-memory (quente) ──────────────────────────────────────

func BenchmarkIndexSearch_N10000_Dim768(b *testing.B)  { benchIndexWarm(b, 10000, autopsyDim, 10) }
func BenchmarkIndexSearch_N100000_Dim768(b *testing.B) { benchIndexWarm(b, 100000, autopsyDim, 10) }
func BenchmarkIndexSearch_N1000000_Dim768(b *testing.B) {
	if testing.Short() {
		b.Skip("large store in short mode")
	}
	benchIndexWarm(b, 1000000, autopsyDim, 10)
}

func benchIndexWarm(b *testing.B, count, dim, limit int) {
	store := autopsyStore(b, dim, count)
	store.indexEnabled.Store(true)
	q := autopsyQuery(dim)
	// warm: constrói o snapshot uma vez (fora do timer).
	if _, err := store.Search(q, limit); err != nil {
		b.Fatalf("warm search: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := store.Search(q, limit)
		if err != nil {
			b.Fatalf("search: %v", err)
		}
		_ = res
	}
	b.ReportMetric(float64(count)*float64(b.N)/b.Elapsed().Seconds()/1e6, "Mvec/s")
}

// ── COLD — primeira busca após invalidação (load + search) ────────────────

func BenchmarkIndexCold_N100000_Dim768(b *testing.B) {
	if testing.Short() {
		b.Skip("large store in short mode")
	}
	store := autopsyStore(b, autopsyDim, 100000)
	store.indexEnabled.Store(true)
	q := autopsyQuery(autopsyDim)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.index.invalidate()
		res, err := store.Search(q, 10) // reconstrói o snapshot + busca
		if err != nil {
			b.Fatalf("search: %v", err)
		}
		_ = res
	}
}

// ── LOAD — custo de construção do índice (uma vez) ─────────────────────────

func BenchmarkIndexLoad_N100000_Dim768(b *testing.B) {
	if testing.Short() {
		b.Skip("large store in short mode")
	}
	store := autopsyStore(b, autopsyDim, 100000)
	store.index.invalidate()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen, err := store.index.loadGeneration()
		if err != nil {
			b.Fatalf("load: %v", err)
		}
		if gen.n != 100000 {
			b.Fatalf("loaded %d, want 100000", gen.n)
		}
	}
	b.SetBytes(int64(100000 * autopsyDim * 4))
}

// ── COMPUTE puro do índice (sem fetchMeta) ─────────────────────────────────

func BenchmarkIndexScoreOnly_N100000_Dim768(b *testing.B) {
	benchIndexScoreOnly(b, 100000, autopsyDim, 10)
}

func benchIndexScoreOnly(b *testing.B, count, dim, limit int) {
	store := autopsyStore(b, dim, count)
	store.indexEnabled.Store(true)
	q := autopsyQuery(dim)
	if _, err := store.Search(q, limit); err != nil {
		b.Fatalf("warm search: %v", err)
	}
	gen := store.index.gen.Load()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = gen.score(q, limit)
	}
	b.ReportMetric(float64(count)*float64(b.N)/b.Elapsed().Seconds()/1e6, "Mvec/s")
}

// ── COMPUTE int8 (fast path AVX2, L339) vs float32 (exato) ────────────────

// BenchmarkIndexScoreOnly8_N1000000_Dim768 mede o fast path int8 no índice
// real (1M×768). O MESMO snapshot tem os dois slabs (float32 + int8): o
// score8 roda o kernel AVX2 sobre 1 byte/elem; o float32 é o fallback exato.
func BenchmarkIndexScoreOnly8_N100000_Dim768(b *testing.B) {
	benchIndexScoreOnly8(b, 100000, autopsyDim, 10)
}

func BenchmarkIndexScoreOnly8_N1000000_Dim768(b *testing.B) {
	if testing.Short() {
		b.Skip("large store in short mode")
	}
	benchIndexScoreOnly8(b, 1000000, autopsyDim, 10)
}

func benchIndexScoreOnly8(b *testing.B, count, dim, limit int) {
	store := autopsyStore(b, dim, count)
	store.indexEnabled.Store(true) // int8 é o default do produto (L341)
	q := autopsyQuery(dim)
	if _, err := store.Search(q, limit); err != nil {
		b.Fatalf("warm search: %v", err)
	}
	gen := store.index.gen.Load()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = gen.score8(q, limit)
	}
	b.ReportMetric(float64(count)*float64(b.N)/b.Elapsed().Seconds()/1e6, "Mvec/s")
}

// ── PARALELISMO — score do índice com N workers (100K, dim768) ────────────

func BenchmarkIndexWorkers_N100000_Dim768_W1(b *testing.B)  { benchIndexWorkers(b, 100000, 1) }
func BenchmarkIndexWorkers_N100000_Dim768_W4(b *testing.B)  { benchIndexWorkers(b, 100000, 4) }
func BenchmarkIndexWorkers_N100000_Dim768_W8(b *testing.B)  { benchIndexWorkers(b, 100000, 8) }
func BenchmarkIndexWorkers_N100000_Dim768_W16(b *testing.B) { benchIndexWorkers(b, 100000, 16) }

func benchIndexWorkers(b *testing.B, count, workers int) {
	store := autopsyStore(b, autopsyDim, count)
	store.indexEnabled.Store(true)
	q := autopsyQuery(autopsyDim)
	if _, err := store.Search(q, 10); err != nil {
		b.Fatalf("warm search: %v", err)
	}
	gen := store.index.gen.Load()
	var normA float64
	for _, v := range q {
		normA += v * v
	}
	sqrtNormA := math.Sqrt(normA)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = gen.scoreParallel(q, 10, workers, sqrtNormA)
	}
	b.ReportMetric(float64(count)*float64(b.N)/b.Elapsed().Seconds()/1e6, "Mvec/s")
}
