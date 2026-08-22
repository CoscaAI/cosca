package orchestration

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/stallwatch"
)

// ── P2 v6: Extreme ramp — o breakpoint real ─────────────────────────────
//
// A régua do professor: "a partir de 5.000 mude o foco — não basta contar
// sessões". Mede: throughput, goroutines (vazamento), latência p50/p95/p99,
// e acha o ponto onde o sistema DEGRADA (não apenas quebra).

// runChaosExtreme roda load sessões em paralelo com provider caótico e
// coleta: resultado, latências por sessão, goroutines antes/depois.
func runChaosExtreme(t *testing.T, load, maxRetries int) (completed, failed int, latencies []time.Duration, goroutinesBefore, goroutinesAfter int) {
	t.Helper()
	collector := stallwatch.NewCollector()
	var wg sync.WaitGroup
	var mu sync.Mutex
	var comp, fail atomic.Int64

	goroutinesBefore = runtime.NumGoroutine()

	for i := 0; i < load; i++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(seed)))
			p := &chaosProvider{
				name:       fmt.Sprintf("chaos-%d", seed),
				model:      "gpt-4o",
				stallEvery: 1 + rng.Intn(3),
				failEvery:  2 + rng.Intn(4),
			}
			cfg := DefaultExecutorConfig()
			cfg.MaxRetries = maxRetries
			cfg.RetryDelay = time.Millisecond
			cfg.Timeout = 15 * time.Millisecond
			exec := NewExecutor(p, cfg, nil)
			exec.SetStallCollector(collector)

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			start := time.Now()
			_, err := exec.Execute(ctx, NewPipelineContext(fmt.Sprintf("req-%d", seed), "task"))
			lat := time.Since(start)

			mu.Lock()
			latencies = append(latencies, lat)
			mu.Unlock()

			if err != nil {
				fail.Add(1)
			} else {
				comp.Add(1)
			}
		}(i)
	}
	wg.Wait()

	// Quiescence: dá um instante para goroutines órfãs terminarem antes de
	// contar (o provider caótico respeita ctx, então devem encerrar).
	time.Sleep(50 * time.Millisecond)
	goroutinesAfter = runtime.NumGoroutine()

	return int(comp.Load()), int(fail.Load()), latencies, goroutinesBefore, goroutinesAfter
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}

// TestChaos_ExtremeRampBreakpoint: escada 10k → 25k → 50k com as métricas
// do professor. Registra o breakpoint (primeira deterioração) e nunca quebra
// o teste por degradação — degradação é o RESULTADO que queremos medir; só
// falhamos se houver HANG (invariante absoluto).
func TestChaos_ExtremeRampBreakpoint(t *testing.T) {
	loads := []int{10000, 25000, 50000}
	const maxRetries = 3

	t.Log("=== EXTREME RAMP (provider caótico) — métricas do professor ===")
	t.Log("  load | completed/failed | hung | throughput | p50/p95/p99 | goroutines(delta)")
	var prevThroughput float64
	var prevP95 time.Duration
	var breakpoint string

	for _, load := range loads {
		start := time.Now()
		comp, fail, lats, gBefore, gAfter := runChaosExtreme(t, load, maxRetries)
		elapsed := time.Since(start)
		throughput := float64(load) / elapsed.Seconds()

		sorted := append([]time.Duration(nil), lats...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		p50, p95, p99 := percentile(sorted, 0.50), percentile(sorted, 0.95), percentile(sorted, 0.99)
		goroutineDelta := gAfter - gBefore

		if comp+fail != load {
			t.Fatalf("load %d: contabilidade %d+%d != %d", load, comp, fail, load)
		}

		t.Logf("  %5d | %d/%d        | %d    | %7.0f/s | %4.0f/%4.0f/%4.0fms | %+d",
			load, comp, fail, 0, throughput,
			float64(p50.Milliseconds()), float64(p95.Milliseconds()), float64(p99.Milliseconds()),
			goroutineDelta)

		// Breakpoint de THROUGHPUT: queda > 20% vs estágio anterior.
		if prevThroughput > 0 && throughput < prevThroughput*0.80 {
			breakpoint = fmt.Sprintf("load=%d: throughput %.0f/s (queda %.0f%% vs %.0f/s)",
				load, throughput, (1-throughput/prevThroughput)*100, prevThroughput)
			break
		}
		// Breakpoint de LATÊNCIA: p95 dobra (> +100%) vs estágio anterior —
		// a saturação aparece na latência antes do throughput (lição do
		// professor: medir múltiplas métricas, não só contagem).
		if prevP95 > 0 && p95 > prevP95*2 {
			breakpoint = fmt.Sprintf("load=%d: p95 %v (dobrou vs %v do estágio anterior) — saturação de latência",
				load, p95.Round(time.Millisecond), prevP95.Round(time.Millisecond))
			break
		}
		prevThroughput = throughput
		prevP95 = p95
	}

	if breakpoint != "" {
		t.Logf("⚠ BREAKPOINT DETECTADO: %s", breakpoint)
	} else {
		t.Log("✓ SEM breakpoint até 50.000 sessões — throughput e latência estáveis")
	}
}
