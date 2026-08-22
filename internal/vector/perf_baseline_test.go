// Package vector — FASE 1 · BASELINE INVIOLÁVEL + FASE 15 · REGRESSION GUARD.
//
// Este arquivo é o harness de baseline do caminho InMemoryIndex (L311):
//   - full path (Search completo: snapshot + score + materialize) e
//   - score-only (gen.score: só o compute)
//
// para datasets {10K, 100K, 1M} × dim 768 × limits {1, 10, 50, 100}.
// Serve de baseline reprodutível e de guarda-regressão (FASE 15): qualquer
// otimização futura DEVE comparar contra estes números, e nenhuma regressão
// pode voltar sem ser detectada por este harness.
//
// O baseline documentado (hardware: AMD Ryzen 7 5700X3D, 16 threads) vive no
// relatório final da rodada de otimização; estes benchmarks são a medição.
package vector

import (
	"fmt"
	"math"
	"sort"
	"testing"
	"time"
)

var perfBaselineCounts = []int{10000, 100000, 1000000}
var perfBaselineLimits = []int{1, 10, 50, 100}

func perfSkip1M(b *testing.B, count int) {
	if count >= 1000000 && testing.Short() {
		b.Skip("1M store in short mode")
	}
}

// benchFullSearch mede o caminho completo e quente (snapshot + score +
// materialize via fetchMeta), com o índice ligado.
func benchFullSearch(b *testing.B, count, limit int) {
	perfSkip1M(b, count)
	store := autopsyStore(b, autopsyDim, count)
	store.indexEnabled.Store(true)
	q := autopsyQuery(autopsyDim)
	if _, err := store.Search(q, limit); err != nil {
		b.Fatalf("warm search: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := store.Search(q, limit)
		if err != nil {
			b.Fatalf("search: %v", err)
		}
		if len(res) > limit {
			b.Fatalf("results %d > limit %d", len(res), limit)
		}
	}
	b.ReportMetric(float64(count)*float64(b.N)/b.Elapsed().Seconds()/1e6, "Mvec/s")
	b.ReportMetric(float64(count)*4*float64(autopsyDim)*float64(b.N)/b.Elapsed().Seconds()/1e9, "MB/s")
}

// benchScoreOnly mede apenas o compute (gen.score) sobre um snapshot quente.
func benchScoreOnly(b *testing.B, count, limit int) {
	perfSkip1M(b, count)
	store := autopsyStore(b, autopsyDim, count)
	store.indexEnabled.Store(true)
	q := autopsyQuery(autopsyDim)
	if _, err := store.Search(q, limit); err != nil {
		b.Fatalf("warm search: %v", err)
	}
	gen := store.index.gen.Load()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := gen.score(q, limit)
		if len(res) > limit {
			b.Fatalf("results %d > limit %d", len(res), limit)
		}
	}
	b.ReportMetric(float64(count)*float64(b.N)/b.Elapsed().Seconds()/1e6, "Mvec/s")
	b.ReportMetric(float64(count)*4*float64(autopsyDim)*float64(b.N)/b.Elapsed().Seconds()/1e9, "MB/s")
}

// ── FASE 1 · baseline full path ────────────────────────────────────────────

func BenchmarkPerfBaselineSearch(b *testing.B) {
	for _, count := range perfBaselineCounts {
		for _, limit := range perfBaselineLimits {
			count, limit := count, limit
			b.Run(fmt.Sprintf("N%d/L%d", count, limit), func(b *testing.B) {
				benchFullSearch(b, count, limit)
			})
		}
	}
}

// ── FASE 1 · baseline score-only (compute puro, sem fetchMeta) ─────────────

func BenchmarkPerfBaselineScoreOnly(b *testing.B) {
	for _, count := range perfBaselineCounts {
		for _, limit := range perfBaselineLimits {
			count, limit := count, limit
			b.Run(fmt.Sprintf("N%d/L%d", count, limit), func(b *testing.B) {
				benchScoreOnly(b, count, limit)
			})
		}
	}
}

// ── FASE 1 · latência p50/p95/p99 + wall-clock do caminho completo ─────────

func TestPerfBaselineLatencyPercentiles(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	for _, tc := range []struct {
		count int
		limit int
	}{
		{10000, 10},
		{100000, 10},
		{100000, 100},
		{1000000, 10},
	} {
		store := autopsyStore(t, autopsyDim, tc.count)
		store.indexEnabled.Store(true)
		q := autopsyQuery(autopsyDim)
		if _, err := store.Search(q, tc.limit); err != nil {
			t.Fatalf("warm: %v", err)
		}

		const n = 40
		samples := make([]time.Duration, n)
		for i := 0; i < n; i++ {
			t0 := time.Now()
			res, err := store.Search(q, tc.limit)
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			samples[i] = time.Since(t0)
			_ = res
		}
		sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
		pct := func(p float64) time.Duration { return samples[int(float64(n-1)*p)] }
		t.Logf("N=%-7d L=%-3d mean=%9.2fms p50=%9.2fms p95=%9.2fms p99=%9.2fms min=%9.2fms",
			tc.count, tc.limit,
			float64(sumDur(samples))/float64(n)/1e6,
			float64(pct(0.50))/1e6, float64(pct(0.95))/1e6, float64(pct(0.99))/1e6,
			float64(samples[0])/1e6)
	}
}

// ── FASE 15 · guarda-regressão: limites documentados ───────────────────────

// TestPerfBaselineRegressionGuard trava os limiares do caminho quente. Se uma
// mudança futura violar estes limites, o guard acusa — impedindo o retorno do
// gargalo original. Os valores são ~2x o baseline medido (folga para ruído de
// hardware) e cobrem o regime N=10K..1M com limit=10.
func TestPerfBaselineRegressionGuard(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	// A régua de latência é calibrada SEM -race (overhead do detector ~20×
	// inflaria os limites e geraria falso positivo — o -race é usado apenas
	// para validar concorrência, nunca como régua de performance).
	if testingRace() {
		t.Skip("latency guard skipped under -race (performance ruler is race-free only)")
	}
	// limite por N (baseline warm ~1.3ms/10.4ms/109ms → guard 2.6ms/20ms/250ms)
	thresholds := map[int]time.Duration{
		10000:   2600 * time.Microsecond,
		100000:  20 * time.Millisecond,
		1000000: 250 * time.Millisecond,
	}
	const runs = 7
	for _, count := range []int{10000, 100000, 1000000} {
		store := autopsyStore(t, autopsyDim, count)
		store.indexEnabled.Store(true)
		q := autopsyQuery(autopsyDim)
		if _, err := store.Search(q, 10); err != nil {
			t.Fatalf("warm: %v", err)
		}
		// descarta outliers (primeira busca após warm) e tira a mediana.
		times := make([]time.Duration, runs)
		for i := 0; i < runs; i++ {
			t0 := time.Now()
			if _, err := store.Search(q, 10); err != nil {
				t.Fatalf("search: %v", err)
			}
			times[i] = time.Since(t0)
		}
		sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
		med := times[runs/2]
		lim := thresholds[count]
		if med > lim {
			t.Fatalf("regression guard: N=%d search med=%v > limit=%v — o gargalo voltou?", count, med, lim)
		}
		t.Logf("guard OK: N=%d med=%v (limit=%v)", count, med, lim)
	}
}

// sumDur é usado também pelos testes de latência (definido em
// autopsy_experiments_test.go; mantido aqui por simetria para o caso de o
// outro arquivo ser removido).
func perfSumDur(d []time.Duration) time.Duration {
	var s time.Duration
	for _, v := range d {
		s += v
	}
	return s
}

var _ = math.Sqrt
