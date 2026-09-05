// Package knowledge — campanha performance, L368: BREAKDOWN da busca por
// fase (experimento 1 da revisão L367, read-only). Mede onde os ~2.8ms de
// uma busca de produção realmente vão: embed → FTS → vector+materialização
// → ranking/merge.
package knowledge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/cache"
	"github.com/CoscaAI/cosca/internal/search"
	"github.com/CoscaAI/cosca/internal/sqlite"
)

// TestCampaignSearchBreakdown — o breakdown da latência por fase (L368).
func TestCampaignSearchBreakdown(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	root := "/home/cosca/Documents/cosca"
	coscaDir := filepath.Join(root, ".cosca")
	if _, err := os.Stat(filepath.Join(coscaDir, "knowledge.db")); err != nil {
		t.Skip("sem knowledge.db")
	}
	eng, err := New(Config{
		DBPath:            filepath.Join(coscaDir, "knowledge.db"),
		RootDir:           root,
		AutoMigrate:       true,
		CacheConfig:       cache.Config{EnabledLevels: []cache.Level{cache.Level(255)}},
		EmbeddingProvider: "ollama",
		EmbeddingModel:    "nomic-embed-text",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.Init(); err != nil {
		t.Fatal(err)
	}
	defer eng.Close()

	// 30 queries de texto real (conteúdo de chunks).
	rows, err := eng.db.Conn().Query(
		"SELECT content FROM chunks WHERE length(content) > 40 ORDER BY rowid LIMIT 30")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var queries []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		if len(c) > 200 {
			c = c[:200]
		}
		queries = append(queries, c)
	}
	if len(queries) == 0 {
		t.Fatal("sem queries")
	}

	ctx := context.Background()
	// Warm-up: carrega o índice lazy + aquece o provider (fora do timer).
	if _, err := eng.embRegistry.GenerateEmbedding(ctx, queries[0]); err != nil {
		t.Fatalf("warm embed: %v", err)
	}
	if _, _, err := eng.fts.Search(sqlite.FTSSearchParams{Query: queries[0], Limit: 30}); err != nil {
		t.Fatalf("warm fts: %v", err)
	}
	if _, err := eng.vecStore.Search([]float64(make([]float64, 768)), 30); err != nil {
		t.Fatalf("warm vector: %v", err)
	}

	var dTotal, dEmbed, dFTS, dVector time.Duration
	n := 0
	for _, q := range queries {
		// 1. EMBED isolado (nomic real via ollama).
		t0 := time.Now()
		emb, err := eng.embRegistry.GenerateEmbedding(ctx, q)
		if err != nil {
			t.Fatalf("embed %q: %v", q[:20], err)
		}
		dEmbed += time.Since(t0)

		// 2. FTS isolado.
		t1 := time.Now()
		_, _, err = eng.fts.Search(sqlite.FTSSearchParams{Query: q, Limit: 30})
		if err != nil {
			t.Fatalf("fts: %v", err)
		}
		dFTS += time.Since(t1)

		// 3. Vector + materialização (índice int16 + fetch de colunas).
		t2 := time.Now()
		if _, err := eng.vecStore.Search(emb.Vector, 30); err != nil {
			t.Fatalf("vector: %v", err)
		}
		dVector += time.Since(t2)

		// 4. TOTAL (busca completa, cache off).
		t3 := time.Now()
		p := search.DefaultSearchParams()
		p.Query = q
		p.Limit = 30
		if _, err := eng.Search(ctx, p); err != nil {
			t.Fatalf("total: %v", err)
		}
		dTotal += time.Since(t3)
		n++
	}
	nf := float64(n)
	ms := func(d time.Duration) float64 { return float64(d) / nf / 1e6 }
	t.Logf("── L368 BREAKDOWN da busca (%d queries reais, nomic, cache off, warm) ──", n)
	t.Logf("embed:         %6.2f ms  (%5.1f%%)", ms(dEmbed), ms(dEmbed)*100/ms(dTotal))
	t.Logf("fts:           %6.2f ms  (%5.1f%%)", ms(dFTS), ms(dFTS)*100/ms(dTotal))
	t.Logf("vector+mat:    %6.2f ms  (%5.1f%%)", ms(dVector), ms(dVector)*100/ms(dTotal))
	rankMS := ms(dTotal) - (ms(dEmbed) + ms(dFTS) + ms(dVector))
	t.Logf("ranking/merge: %6.2f ms  (%5.1f%%)", rankMS, rankMS*100/ms(dTotal))
	t.Logf("TOTAL:         %6.2f ms  (scan do índice ≈ 0.6ms — L357)", ms(dTotal))
}