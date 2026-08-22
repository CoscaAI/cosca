// Package knowledge — campanha performance, L369: RECALL REAL de produção
// (experimento 2 da revisão L367). Verifica a composição do índice e mede o
// recall com o CAMINHO COMPLETO: texto → embed nomic (via provider ollama)
// → busca → o próprio chunk está no top-K?
//
// Se os vetores do índice foram gerados com o nomic: o self-match (embed do
// próprio texto) deve dar score ~1.0 no top-1. Se uma fração do índice veio
// de outro provider (test-deterministic/TF-IDF), o self-match dessa fração
// será BAIXO — confirmando a mistura (L368).
package knowledge

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/cache"
	"github.com/CoscaAI/cosca/internal/providers/ollama"
	"github.com/CoscaAI/cosca/internal/vector"
)

// TestCampaignRealRecall — recall real via o caminho completo (nomic).
func TestCampaignRealRecall(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	ollama.Register() // registra o provider de embeddings (produção usa)

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

	// 60 chunks (30 "antigos" por rowid + 30 "novos" por rowid desc).
	rows, err := eng.db.Conn().Query(
		"SELECT id, content FROM chunks c WHERE is_trivial = 0 AND dedup_of = '' " +
			"AND EXISTS (SELECT 1 FROM vectors v WHERE v.chunk_id = c.id) ORDER BY c.rowid LIMIT 30")
	if err != nil {
		t.Fatal(err)
	}
	var oldChunks []string
	oldIDs := map[string]bool{}
	for rows.Next() {
		var id, content string
		if err := rows.Scan(&id, &content); err != nil {
			t.Fatal(err)
		}
		oldChunks = append(oldChunks, content)
		oldIDs[id] = true
	}
	rows.Close()
	rows2, err := eng.db.Conn().Query(
		"SELECT id, content FROM chunks c WHERE is_trivial = 0 AND dedup_of = '' " +
			"AND EXISTS (SELECT 1 FROM vectors v WHERE v.chunk_id = c.id) ORDER BY c.rowid DESC LIMIT 30")
	if err != nil {
		t.Fatal(err)
	}
	var newChunks []string
	for rows2.Next() {
		var id, content string
		if err := rows2.Scan(&id, &content); err != nil {
			t.Fatal(err)
		}
		newChunks = append(newChunks, content)
	}
	rows2.Close()

	// Self-match: embed o texto → busca → o top-1 deve ser ~1.0 (o próprio).
	selfMatch := func(contents []string, label string) {
		store := eng.vecStore.(*vector.SQLiteVec)
		hits, total := 0, 0
		var scoreSum float64
		for _, c := range contents {
			if len(c) < 40 || len(c) > 3000 {
				continue
			}
			emb, err := eng.embRegistry.GenerateEmbedding(context.Background(), c)
			if err != nil {
				t.Fatalf("embed: %v", err)
			}
			res, err := store.Search(emb.Vector, 5)
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			total++
			// O próprio chunk deve ter score ~1.0 no top-1 (o texto exato
			// do chunk, embedado com o MESMO provider).
			scoreSum += res[0].Score
			if res[0].Score > 0.9 {
				hits++
			}
		}
		t.Logf("self-match %s (n=%d): top1-score médio=%.4f  hits>0.9=%d/%d",
			label, total, scoreSum/float64(total), hits, total)
	}
	t.Logf("── L369 RECALL REAL (nomic via fluxo padrão) ──")
	selfMatch(oldChunks, "originais")
	selfMatch(newChunks, "novos (nomic L364)")
}
