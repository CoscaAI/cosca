package knowledge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/search"
)

// TestRealBenchmark mede o caminho REAL de produção com múltiplos cenários:
// 1. Cache quente (mesma query repetida — o cache multi-tier de 5min atua)
// 2. Cache frio (queries diferentes — força o scan in-memory real)
// 3. Throughput (queries variadas em sequência)
func TestRealBenchmark(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	root := "/home/cosca/Documents/cosca"
	coscaDir := filepath.Join(root, ".cosca")
	if _, err := os.Stat(filepath.Join(coscaDir, "knowledge.db")); err != nil {
		t.Skip("sem knowledge.db")
	}
	eng, err := New(Config{DBPath: filepath.Join(coscaDir, "knowledge.db"), RootDir: root, AutoMigrate: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.Init(); err != nil {
		t.Fatal(err)
	}
	defer eng.Close()

	ctx := context.Background()
	queries := []string{
		"vector search performance",
		"hardware brain capability",
		"scene graph procedural",
		"memory bandwidth benchmark",
		"security sandbox jail",
		"cosca kernel memory",
		"audio video generation",
		"worker pool fabric",
		"epistemology fact measured",
		"storage nvme knowledge",
	}

	// ── 1. CACHE QUENTE (mesma query, 20x) ──
	q := search.DefaultSearchParams()
	q.Query = queries[0]
	q.Limit = 5
	eng.Search(ctx, q) // warm
	var hotTotal time.Duration
	hotMin, hotMax := time.Hour, time.Duration(0)
	for i := 0; i < 20; i++ {
		t0 := time.Now()
		eng.Search(ctx, q)
		d := time.Since(t0)
		hotTotal += d
		if d < hotMin { hotMin = d }
		if d > hotMax { hotMax = d }
	}
	t.Logf("CACHE QUENTE (mesma query): avg=%.3fms min=%.3fms max=%.3fms",
		float64((hotTotal/20).Microseconds())/1000, float64(hotMin.Microseconds())/1000, float64(hotMax.Microseconds())/1000)

	// ── 2. CACHE FRIO (queries diferentes, 10x cada) ──
	var coldTotal time.Duration
	coldMin, coldMax := time.Hour, time.Duration(0)
	nCold := 0
	for _, query := range queries {
		p := search.DefaultSearchParams()
		p.Query = query
		p.Limit = 5
		for i := 0; i < 10; i++ {
			t0 := time.Now()
			eng.Search(ctx, p)
			d := time.Since(t0)
			coldTotal += d
			nCold++
			if d < coldMin { coldMin = d }
			if d > coldMax { coldMax = d }
		}
	}
	t.Logf("CACHE FRIO (queries variadas): avg=%.3fms min=%.3fms max=%.3fms (n=%d)",
		float64((coldTotal/time.Duration(nCold)).Microseconds())/1000,
		float64(coldMin.Microseconds())/1000, float64(coldMax.Microseconds())/1000, nCold)

	// ── 3. THROUGHPUT (queries sequenciais variadas, sem warm entre) ──
	t0 := time.Now()
	total := 0
	for i := 0; i < 50; i++ {
		p := search.DefaultSearchParams()
		p.Query = queries[i%len(queries)]
		p.Limit = 5
		eng.Search(ctx, p)
		total++
	}
	elapsed := time.Since(t0)
	t.Logf("THROUGHPUT: %d queries em %.3fs = %.1f qps",
		total, elapsed.Seconds(), float64(total)/elapsed.Seconds())
}
