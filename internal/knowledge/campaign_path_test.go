// Package knowledge — campanha de performance, FASE 1 (L343).
//
// Mede o caminho REAL do knowledge search contra o knowledge.db de produção:
// quantos vetores existem no store, quantos candidatos léxicos resolvem e
// quantos vetores são DE FATO escaneados por query. A pergunta que a campanha
// responde: o full-scan do índice int8 (1M) importa se a produção só entrega
// ~37k candidatos ao kernel?
package knowledge

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/cache"
	"github.com/CoscaAI/cosca/internal/search"
	"github.com/CoscaAI/cosca/internal/vector"
)

// TestCampaignPathReal — distribuição do caminho real (FASE 1).
// Roda N queries reais contra o knowledge.db da casa e coleta SearchMetrics
// por query via MetricsSink. Queries reais = conteúdo de chunks do próprio db
// (amostra determinística), para não depender de embedding externo.
func TestCampaignPathReal(t *testing.T) {
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
		// SEM cache: EnabledLevels com nível inexistente desativa todos os
		// backends (levelEnabled nunca casa) — a FASE 1 mede o caminho real,
		// não hits de cache persistente.
		CacheConfig: cache.Config{EnabledLevels: []cache.Level{cache.Level(255)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.Init(); err != nil {
		t.Fatal(err)
	}
	defer eng.Close()

	ctx := context.Background()

	// Queries reais: primeiro N chunks do db (determinístico).
	chunks, err := sampleChunkQueries(eng, 400)
	if err != nil {
		t.Fatal(err)
	}

	var ms []vector.SearchMetrics
	eng.SetMetricsSink(func(m vector.SearchMetrics) {
		ms = append(ms, m)
	})

	// Cache frio: queries diferentes (não repete — o cache de 5min distorceria).
	for i, q := range chunks {
		p := search.DefaultSearchParams()
		p.Query = q
		p.Limit = 10
		_, err := eng.Search(ctx, p)
		if err != nil {
			t.Fatalf("query %d: %v", i, err)
		}
	}

	t.Logf("queries enviadas=%d  métricas coletadas=%d", len(chunks), len(ms))
	reportPathMetrics(t, ms)
}

// sampleChunkQueries extrai textos reais de chunks para usar como queries.
// Filtra conteúdo substancial (len > 40): chunks com 1-2 chars são lixo de
// migração e não representam consultas reais de usuário.
func sampleChunkQueries(eng *Engine, n int) ([]string, error) {
	rows, err := eng.db.Conn().QueryContext(context.Background(),
		"SELECT content FROM chunks WHERE length(content) > 40 ORDER BY rowid LIMIT ?", n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		if len(c) > 200 {
			c = c[:200]
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// reportPathMetrics agrega e imprime a distribuição do caminho real.
func reportPathMetrics(t *testing.T, ms []vector.SearchMetrics) {
	t.Helper()
	if len(ms) == 0 {
		t.Fatal("nenhuma métrica coletada — o sink não foi chamado?")
	}

	var total, idx, cand, filt, sqlFail int
	var scannedSum, latSum int64
	var maxScanned int
	hist := map[string]int{}
	for _, m := range ms {
		total++
		switch {
		case m.MetadataCandidates == 0 && m.ScannedVectors == m.TotalVectors && m.ScannedVectors > 0:
			idx++
		case m.MetadataCandidates > 0:
			cand++
		case m.ScannedVectors == 0 && m.TotalVectors > 0:
			sqlFail++
		default:
			filt++
		}
		scannedSum += int64(m.ScannedVectors)
		latSum += int64(m.Latency)
		if m.ScannedVectors > maxScanned {
			maxScanned = m.ScannedVectors
		}
		hist[bucket(m.ScannedVectors)]++
	}

	buckets := []string{"0-10k", "10k-50k", "50k-100k", "100k-500k", "500k-1M"}
	has := false
	for _, b := range buckets {
		if hist[b] > 0 {
			has = true
			break
		}
	}
	t.Logf("── FASE 1: caminho real (%d queries, db real) ──", total)
	t.Logf("full-scan índice=%d  candidatos=%d  filtro=%d  sqlfail=%d", idx, cand, filt, sqlFail)
	t.Logf("scanned médio=%d  p95=%d  max=%d  TotalVectors=%d",
		scannedSum/int64(len(ms)), p95Scanned(ms), maxScanned, ms[0].TotalVectors)
	t.Logf("latency média=%s  p95=%s", time.Duration(latSum/int64(len(ms))), p95Latency(ms))
	if !has {
		t.Log("distribuição: (todos fora dos buckets padrão)")
	}
	for _, b := range buckets {
		if hist[b] > 0 {
			t.Logf("  %-12s %5d", b, hist[b])
		}
	}
	if idx > 0 {
		t.Logf("→ %d queries atingiram o fast path int8 (%d%%)", idx, idx*100/len(ms))
	}
	if cand > 0 {
		t.Logf("→ %d queries usaram o caminho híbrido bounded (float32!) (%d%%)", cand, cand*100/len(ms))
	}
}

func bucket(n int) string {
	switch {
	case n <= 10000:
		return "0-10k"
	case n <= 50000:
		return "10k-50k"
	case n <= 100000:
		return "50k-100k"
	case n <= 500000:
		return "100k-500k"
	default:
		return "500k-1M"
	}
}

func p95Scanned(ms []vector.SearchMetrics) int64 {
	vs := make([]int, len(ms))
	for i, m := range ms {
		vs[i] = m.ScannedVectors
	}
	sort.Ints(vs)
	return int64(vs[len(vs)*95/100])
}

func p95Latency(ms []vector.SearchMetrics) time.Duration {
	vs := make([]int64, len(ms))
	for i, m := range ms {
		vs[i] = int64(m.Latency)
	}
	sort.Slice(vs, func(a, b int) bool { return vs[a] < vs[b] })
	return time.Duration(vs[len(vs)*95/100])
}
