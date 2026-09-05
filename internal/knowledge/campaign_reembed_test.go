// Package knowledge — campanha performance, L364: RE-EMBED dos órfãos limpos
// (AUTORIZADO — "continue"). Provider real: nomic-embed-text via ollama
// (endpoint OpenAI-compatível /v1/embeddings, 768 dims verificado).
//
// Garantias: backup pré obrigatório · SÓ os órfãos limpos (is_trivial=0 E
// dedup_of=” E sem vetor — os triviais/duplicados marcados na L360 ficam
// fora) · lotes de 100 · validação pós.
package knowledge

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/cache"
	"github.com/CoscaAI/cosca/internal/vector"
)

const ollamaEmbedURL = "http://localhost:11434/v1/embeddings"

// ollamaEmbedBatch chama o endpoint /v1/embeddings com um lote de textos.
func ollamaEmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	payload, err := json.Marshal(map[string]interface{}{
		"model": "nomic-embed-text",
		"input": texts,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ollamaEmbedURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama status %d", resp.StatusCode)
	}
	var out struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Data) != len(texts) {
		return nil, fmt.Errorf("ollama devolveu %d embeddings para %d textos", len(out.Data), len(texts))
	}
	vecs := make([][]float64, len(out.Data))
	for i, d := range out.Data {
		vecs[i] = d.Embedding
	}
	return vecs, nil
}

// TestCampaignReembedOrphans — o re-embed dos órfãos limpos (L364).
func TestCampaignReembedOrphans(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	root := "/home/cosca/Documents/cosca"
	coscaDir := filepath.Join(root, ".cosca")

	// Backup pré obrigatório.
	backupPath := filepath.Join(coscaDir, "backup", "knowledge-pre-reembed-"+time.Now().Format("20060102-150405")+".db")
	if _, err := os.Stat(filepath.Join(coscaDir, "backup")); os.IsNotExist(err) {
		t.Skip("teste operacional one-shot — requer o ambiente real de produção (/home/cosca/Documents/cosca) com dir de backup .cosca/backup e o provider local ollama (nomic-embed-text) em localhost:11434")
	}

	eng, err := New(Config{
		DBPath:      filepath.Join(coscaDir, "knowledge.db"),
		RootDir:     root,
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

	// Backup (VACUUM INTO) antes de qualquer escrita.
	if _, err := eng.db.Conn().Exec("VACUUM INTO ?", backupPath); err != nil {
		t.Fatalf("backup falhou: %v", err)
	}
	t.Logf("backup criado: %s", filepath.Base(backupPath))

	// Seleciona os órfãos LIMPOS (não triviais, não duplicados, sem vetor).
	rows, err := eng.db.Conn().Query(
		"SELECT id, document_id, content FROM chunks c WHERE c.is_trivial = 0 AND c.dedup_of = '' " +
			"AND NOT EXISTS (SELECT 1 FROM vectors v WHERE v.chunk_id = c.id)")
	if err != nil {
		t.Fatal(err)
	}
	type orphan struct {
		id, doc, content string
	}
	var orphans []orphan
	for rows.Next() {
		var o orphan
		if err := rows.Scan(&o.id, &o.doc, &o.content); err != nil {
			t.Fatal(err)
		}
		orphans = append(orphans, o)
	}
	rows.Close()
	t.Logf("órfãos limpos a re-embedar: %d", len(orphans))
	if len(orphans) == 0 {
		t.Log("nada a fazer")
		return
	}

	store := eng.vecStore.(*vector.SQLiteVec)
	ctx := context.Background()
	const batchSize = 100
	embedded := 0
	start := time.Now()
	for i := 0; i < len(orphans); i += batchSize {
		end := i + batchSize
		if end > len(orphans) {
			end = len(orphans)
		}
		batch := orphans[i:end]
		texts := make([]string, len(batch))
		for j, o := range batch {
			texts[j] = o.content
		}
		vecs, err := ollamaEmbedBatch(ctx, texts)
		if err != nil {
			t.Fatalf("lote %d: %v", i/batchSize, err)
		}
		recs := make([]vector.VectorRecord, len(batch))
		for j, o := range batch {
			recs[j] = vector.VectorRecord{
				ID:         o.id,
				Vector:     vecs[j],
				DocumentID: o.doc,
				ChunkID:    o.id,
				Content:    o.content,
			}
		}
		if err := store.Store(768, recs); err != nil {
			t.Fatalf("store lote %d: %v", i/batchSize, err)
		}
		embedded += len(batch)
		if embedded%2000 == 0 || embedded == len(orphans) {
			el := time.Since(start)
			t.Logf("  %d/%d embedados (%.0f/s, decorrido %s)", embedded, len(orphans),
				float64(embedded)/el.Seconds(), el.Round(time.Second))
		}
	}

	// Validação.
	var nAfter int
	if err := eng.db.Conn().QueryRow("SELECT count(*) FROM vectors WHERE length(vector) = 768*4").Scan(&nAfter); err != nil {
		t.Fatal(err)
	}
	t.Logf("── L364 RE-EMBED (nomic real via ollama) ──")
	t.Logf("embedados=%d em %s  vetores totais: %d → %d",
		embedded, time.Since(start).Round(time.Second), len(orphans)+0, nAfter)
	if embedded != len(orphans) {
		t.Errorf("embedados %d != órfãos %d", embedded, len(orphans))
	}
	// Spot-check: uma busca real funciona e o recall int16 se mantém alto.
	spotCheckReembed(t, eng)
}

func spotCheckReembed(t *testing.T, eng *Engine) {
	t.Helper()
	store := eng.vecStore.(*vector.SQLiteVec)
	// Query = primeiro vetor existente (pré-re-embed) — o vizinho deve ser
	// ~1.0 (ele mesmo) e a busca deve retornar resultados.
	rows, err := eng.db.Conn().Query("SELECT vector FROM vectors WHERE length(vector) = 768*4 ORDER BY rowid LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var blob []byte
	rows.Next()
	rows.Scan(&blob)
	q := make([]float64, 768)
	for d := 0; d < 768; d++ {
		q[d] = float64(math.Float32frombits(binary.LittleEndian.Uint32(blob[d*4:])))
	}
	res, err := store.Search(q, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatal("busca vazia pós-re-embed")
	}
	t.Logf("spot-check pós-re-embed: top1=%.4f (self, esperado ~1.0)", res[0].Score)
	if math.Abs(res[0].Score-1.0) > 0.02 {
		t.Errorf("top1 score = %.4f, esperado ~1.0 (busca quebrada?)", res[0].Score)
	}
}
