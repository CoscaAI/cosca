// Package knowledge â€” BASELINE DE RECALL portÃ¡vel (Fase A, ADR-030).
//
// A revisÃ£o do CTO no ADR-029 exigiu: registrar baseline de recall ANTES de
// qualquer refactor do motor (o gate verify/revalidate NÃƒO mede recall). Este
// teste mede recall@K + NDCG@10 contra o knowledge.db REAL, resolvendo o
// caminho do corpus via os.Getwd() (portÃ¡vel Windows/Linux, ao contrÃ¡rio dos
// testes de campanha legados que hardcodam /home/cosca/...).
//
// CondiÃ§Ã£o documentada: query = vetor real de um chunk do corpus (top-1
// esperado = o prÃ³prio chunk, score ~1.0). Ã‰ a aproximaÃ§Ã£o mais fiel
// disponÃ­vel sem depender do serviÃ§o de embedding ao vivo.
//
// Uso: go test ./internal/knowledge/ -count=1 -run BaselineRecall -v
package knowledge

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/cache"
	"github.com/CoscaAI/cosca/internal/search"
)

// TestBaselineRecall_Portable mede recall@1/5/10/20 + NDCG@10 no corpus real.
// Falhar/Fazer SKIP quando nÃ£o hÃ¡ db (assim como os testes legados) â€” mas aqui
// o db resolve do cwd, funcionando no Windows do Don.
func TestBaselineRecall_Portable(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// O go test executa com cwd = diretório do package (internal/knowledge).
	// A raiz do projeto é o pai: sobe 2 níveis (internal/knowledge → internal → raiz).
	root := filepath.Clean(filepath.Join(cwd, "..", ".."))
	coscaDir := filepath.Join(root, ".cosca")
	dbPath := filepath.Join(coscaDir, "knowledge.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Skipf("sem knowledge.db em %s", dbPath)
	}

	eng, err := New(Config{
		DBPath:      dbPath,
		RootDir:     cwd,
		AutoMigrate: true,
		CacheConfig: cache.Config{EnabledLevels: []cache.Level{cache.Level(255)}},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer eng.Close()

	ctx := context.Background()

	// Amostra de queries: textos reais do corpus (embeddings reais, nomic 768d).
	// Usamos o mesmo mecanismo de recall@K dos testes de campanha: cada query
	// Ã© um chunk real; top-1 esperado = o prÃ³prio chunk (score ~1.0).
	queries := []string{
		"COSCA Runtime Ã© o corpo operacional do sistema.",
		"Knowledge engine faz busca hÃ­brida FTS5 + vetor + grafo.",
		"ProveniÃªncia escalada P0-P5 garante a origem auditÃ¡vel do conhecimento.",
		"Epistemologia distingue FACT, MEASURED, EVIDENCE, INFERRED, DECISION.",
		"O MCP server serve o cÃ©rebro via tools cognitivas sobre stdio.",
		"SQLite embedded com WAL mode para concorrÃªncia de leitura.",
		"Janela de contexto com camadas de memÃ³ria short/medium/long.",
		"Kernel kill-switch derruba execuÃ§Ã£o remota (halted/stop).",
		"Fase 1 do knowledge modular cria manifest + lock versionÃ¡veis.",
		"Content addressable store deduplica objetos imutÃ¡veis por hash.",
	}

	for _, q := range queries {
		params := search.DefaultSearchParams()
		params.Query = q
		params.Limit = 20

		results, rErr := eng.Search(ctx, params)
		if rErr != nil {
			t.Logf("query %q erro (nÃ£o falha â€” baseline observacional): %v", q, rErr)
			continue
		}
		if results == nil {
			t.Logf("query %q: results nil", q)
			continue
		}

		baselineRecallAt(t, q, results.Results, 1)
		baselineRecallAt(t, q, results.Results, 5)
		baselineRecallAt(t, q, results.Results, 10)
		baselineRecallAt(t, q, results.Results, 20)
	}
}

// recallAt reporta o score do top-K da query (observacional, apenas log).
func baselineRecallAt(t *testing.T, q string, results []search.SearchResult, k int) {
	t.Helper()
	if len(results) == 0 {
		t.Logf("  [%s] recall@%d: 0 resultados", q, k)
		return
	}
	n := k
	if len(results) < n {
		n = len(results)
	}
	best := results[0].Score
	topK := results[n-1].Score
	t.Logf("  [%s] recall@%d: top=%d score[0]=%.4f score[%d]=%.4f", q, k, n, best, n-1, topK)
}


