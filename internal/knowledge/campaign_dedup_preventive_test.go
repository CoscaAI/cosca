// Package knowledge — campanha performance, L371: TESTE da hipótese do dedup
// preventivo (experimento autorizado pelo Don — READ-ONLY no estoque REAL).
//
// Proteção: o teste roda em um db TEMPORÁRIO isolado (cópia via VACUUM INTO)
// — o estoque real NUNCA é tocado. Nenhuma implementação; apenas a evidência.
//
// OBSERVED   (L367): dedup_of foi limpeza única (L360); ingestão sem prevenção.
// HYPOTHESIS: documento novo com conteúdo duplicado gera vetor SEM dedup.
// TEST:      indexar um arquivo cujo conteúdo é idêntico a um chunk existente
//            (no db temp) e verificar se o vetor é criado com dedup_of vazio.
package knowledge

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/cache"
	_ "modernc.org/sqlite"
)

func TestCampaignDedupPreventive(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	realDB := "/home/cosca/Documents/cosca/.cosca/knowledge.db"
	if _, err := os.Stat(realDB); err != nil {
		t.Skip("sem knowledge.db")
	}

	// ── 1. db TEMPORÁRIO isolado (cópia consistente do real) ──
	tmp := t.TempDir()
	tempDB := filepath.Join(tmp, "knowledge.db")
	src, err := sql.Open("sqlite", realDB+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := src.Exec("VACUUM INTO ?", tempDB); err != nil {
		src.Close()
		t.Fatalf("cópia temp: %v", err)
	}
	src.Close()
	t.Logf("db temporário isolado: %s", tempDB)

	// ── 2. conhecimento no TEMP ──
	eng, err := New(Config{
		DBPath:      tempDB,
		RootDir:     tmp,
		AutoMigrate: true,
		CacheConfig: cache.Config{EnabledLevels: []cache.Level{cache.Level(255)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.Init(); err != nil {
		t.Fatal(err)
	}
	defer eng.Close()

	// ── 3. conteúdo real de um chunk existente (com vetor, não trivial) ──
	var dupContent string
	if err := eng.db.Conn().QueryRow(
		"SELECT c.content FROM chunks c JOIN vectors v ON v.chunk_id = c.id "+
			"WHERE c.is_trivial = 0 AND length(c.content) > 80 ORDER BY c.rowid LIMIT 1",
	).Scan(&dupContent); err != nil {
		t.Fatalf("chunk de referência: %v", err)
	}
	t.Logf("conteúdo de referência (%d chars): %.60s...", len(dupContent), dupContent)

	// ── 4. arquivo novo com o MESMO conteúdo ──
	docPath := filepath.Join(tmp, "doc-duplicado.md")
	if err := os.WriteFile(docPath, []byte(dupContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// ── 5. antes ──
	var nBefore, nChunksBefore int
	if err := eng.db.Conn().QueryRow("SELECT count(*) FROM vectors").Scan(&nBefore); err != nil {
		t.Fatal(err)
	}
	if err := eng.db.Conn().QueryRow("SELECT count(*) FROM chunks WHERE content = ?", dupContent).Scan(&nChunksBefore); err != nil {
		t.Fatal(err)
	}

	// ── 6. indexar o documento duplicado (ESCRITA SÓ NO TEMP) ──
	if err := eng.IndexDocument(context.Background(), docPath); err != nil {
		t.Fatalf("index: %v", err)
	}

	// ── 7. depois + verificação ──
	var nAfter, nChunksAfter int
	if err := eng.db.Conn().QueryRow("SELECT count(*) FROM vectors").Scan(&nAfter); err != nil {
		t.Fatal(err)
	}
	if err := eng.db.Conn().QueryRow("SELECT count(*) FROM chunks WHERE content = ?", dupContent).Scan(&nChunksAfter); err != nil {
		t.Fatal(err)
	}
	var dedupOf string
	_ = eng.db.Conn().QueryRow(
		"SELECT dedup_of FROM chunks WHERE content = ? ORDER BY rowid DESC LIMIT 1", dupContent,
	).Scan(&dedupOf)

	t.Logf("── L371 TESTE da hipótese do dedup preventivo (db temp isolado) ──")
	t.Logf("vetores: %d → %d  chunks-duplicados: %d → %d  dedup_of do novo: %q",
		nBefore, nAfter, nChunksBefore, nChunksAfter, dedupOf)
	if nAfter > nBefore {
		t.Logf("EVIDENCE: vetor NOVO criado sem dedup — hipótese CONFIRMADA (dedup_of=%q)",
			dedupOf)
	} else if nAfter == nBefore {
		t.Logf("EVIDENCE: nenhum vetor novo — há proteção? (hipótese REFUTADA)")
	} else {
		t.Logf("EVIDENCE: comportamento inesperado (nAfter < nBefore) — investigar")
	}
}