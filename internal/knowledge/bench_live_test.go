package knowledge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/search"
)

// BenchLiveSearch mede a busca REAL em produção (índice in-memory ativo).
// Não é um teste de assert — é uma régua de medição (F9). Uso:
//
//	go test ./internal/knowledge/ -run TestLiveSearchLatency -v -count=1
func TestLiveSearchLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	// A casa roda em /home/cosca/Documents/cosca (o .cosca fica na raiz).
	root := "/home/cosca/Documents/cosca"
	coscaDir := filepath.Join(root, ".cosca")
	if _, err := os.Stat(filepath.Join(coscaDir, "knowledge.db")); err != nil {
		t.Skip("sem knowledge.db — rodando em outro contexto")
	}

	eng, err := New(Config{
		DBPath:      filepath.Join(coscaDir, "knowledge.db"),
		RootDir:     root,
		AutoMigrate: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.Init(); err != nil {
		t.Fatal(err)
	}
	defer eng.Close()

	ctx := context.Background()
	params := search.DefaultSearchParams()
	params.Query = "vector search performance"
	params.Limit = 5

	if _, err := eng.Search(ctx, params); err != nil { // warm (índice carrega)
		t.Fatal(err)
	}

	const n = 30
	var total time.Duration
	minT, maxT := time.Hour, time.Duration(0)
	for i := 0; i < n; i++ {
		t0 := time.Now()
		if _, err := eng.Search(ctx, params); err != nil {
			t.Fatal(err)
		}
		d := time.Since(t0)
		total += d
		if d < minT {
			minT = d
		}
		if d > maxT {
			maxT = d
		}
	}
	avg := total / n
	t.Logf("SEARCH LIVE [60K vetores, in-memory]: avg=%.2fms min=%.2fms max=%.2fms (n=%d)",
		float64(avg.Microseconds())/1000, float64(minT.Microseconds())/1000,
		float64(maxT.Microseconds())/1000, n)
}
