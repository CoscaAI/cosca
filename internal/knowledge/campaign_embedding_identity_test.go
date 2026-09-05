// Package knowledge — campanha performance, L375: EXPERIMENTO da identidade
// de embeddings (autorizado pelo Don/professor após a auditoria L374).
//
// AMBIENTE A (produção): os vetores EXISTENTES do índice real.
// AMBIENTE B (limpo): re-embed dos MESMOS conteúdos com o nomic explícito
// (ollama.Register + EmbeddingProvider=ollama + nomic-embed-text) — sem
// tocar o índice real (o B vive só no processo).
//
// Comparação A[i]↔B[i] (cosine) + queries CRUZADAS (query A → índice real vs
// query B → índice real): se os dois espaços são o mesmo, cosine ~1.0 e o
// top-k é o mesmo. Se o índice tem mistura de providers (a hipótese da
// L368), parte dos A[i] terá cosine BAIXO contra o B (nomic).
package knowledge

import (
	"context"
	"database/sql"
	"encoding/binary"
	"math"
	"os"
	"testing"

	"github.com/CoscaAI/cosca/internal/cache"
	"github.com/CoscaAI/cosca/internal/providers/ollama"
	"github.com/CoscaAI/cosca/internal/vector"
	_ "modernc.org/sqlite"
)

func TestCampaignEmbeddingIdentity(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	realDB := "/home/cosca/Documents/cosca/.cosca/knowledge.db"
	if _, err := os.Stat(realDB); err != nil {
		t.Skip("sem knowledge.db")
	}
	ollama.Register() // AMBIENTE B: provider explícito

	eng, err := New(Config{
		DBPath:            realDB,
		RootDir:           "/home/cosca/Documents/cosca",
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

	// Amostra: 60 antigos (rowid asc) + 60 novos (rowid desc) — cobre a
	// possível mistura (originais vs nomic L364).
	rows, err := eng.db.Conn().Query(
		"SELECT c.content, v.vector FROM chunks c JOIN vectors v ON v.chunk_id = c.id "+
			"WHERE c.is_trivial = 0 AND length(c.content) > 40 ORDER BY c.rowid ASC LIMIT 60")
	if err != nil {
		t.Fatal(err)
	}
	type pair struct {
		content string
		vecA    []float64
	}
	var pairs []pair
	for rows.Next() {
		var content string
		var blob []byte
		if err := rows.Scan(&content, &blob); err != nil {
			t.Fatal(err)
		}
		if len(blob) != 768*4 {
			continue
		}
		v := make([]float64, 768)
		for d := 0; d < 768; d++ {
			v[d] = float64(math.Float32frombits(binary.LittleEndian.Uint32(blob[d*4:])))
		}
		pairs = append(pairs, pair{content: content, vecA: v})
	}
	rows.Close()
	rows2, err := eng.db.Conn().Query(
		"SELECT c.content, v.vector FROM chunks c JOIN vectors v ON v.chunk_id = c.id "+
			"WHERE c.is_trivial = 0 AND length(c.content) > 40 ORDER BY c.rowid DESC LIMIT 60")
	if err != nil {
		t.Fatal(err)
	}
	for rows2.Next() {
		var content string
		var blob []byte
		if err := rows2.Scan(&content, &blob); err != nil {
			t.Fatal(err)
		}
		if len(blob) != 768*4 {
			continue
		}
		v := make([]float64, 768)
		for d := 0; d < 768; d++ {
			v[d] = float64(math.Float32frombits(binary.LittleEndian.Uint32(blob[d*4:])))
		}
		pairs = append(pairs, pair{content: content, vecA: v})
	}
	rows2.Close()
	t.Logf("amostra: %d pares (60 antigos + 60 novos)", len(pairs))

	// AMBIENTE B: re-embed de cada conteúdo com o nomic (só no processo).
	ctx := context.Background()
	cosines := make([]float64, 0, len(pairs))
	var lowCosine []float64
	for i, p := range pairs {
		emb, err := eng.embRegistry.GenerateEmbedding(ctx, p.content)
		if err != nil {
			t.Fatalf("embed %d: %v", i, err)
		}
		c := cosine(p.vecA, emb.Vector)
		cosines = append(cosines, c)
		if c < 0.99 {
			lowCosine = append(lowCosine, c)
		}
	}
	var sum float64
	lowCount := 0
	for _, c := range cosines {
		sum += c
		if c < 0.99 {
			lowCount++
		}
	}
	t.Logf("── L375 IDENTIDADE DE EMBEDDINGS (A: índice real vs B: nomic explícito) ──")
	t.Logf("cosine A[i]↔B[i]: média=%.4f  min=%.4f  <0.99: %d/%d (%.1f%%)",
		sum/float64(len(cosines)), minFloat(cosines), lowCount, len(cosines),
		float64(lowCount)*100/float64(len(cosines)))
	if len(lowCosine) > 0 {
		t.Logf("  amostra de cosines baixos (possível mistura): %.4f, %.4f, %.4f...",
			lowCosine[0], lowCosine[1], lowCosine[2])
	}

	// Queries CRUZADAS: para 20 pares, buscar com A[i] e com B[i] no índice
	// real — o top-k deve ser ~o mesmo se os espaços são compatíveis.
	store := eng.vecStore.(*vector.SQLiteVec)
	var overlapSum, recallSum float64
	cross := 0
	for i := 0; i < len(pairs) && i < 20; i++ {
		emb, err := eng.embRegistry.GenerateEmbedding(ctx, pairs[i].content)
		if err != nil {
			t.Fatal(err)
		}
		resA, err := store.Search(pairs[i].vecA, 10)
		if err != nil {
			t.Fatal(err)
		}
		resB, err := store.Search(emb.Vector, 10)
		if err != nil {
			t.Fatal(err)
		}
		setA := map[string]bool{}
		for _, r := range resA {
			setA[r.ID] = true
		}
		overlap := 0
		for _, r := range resB {
			if setA[r.ID] {
				overlap++
			}
		}
		overlapSum += float64(overlap)
		recallSum += float64(overlap) / 10
		cross++
	}
	t.Logf("queries cruzadas (%d): top-10 overlap médio=%.2f/10  recall@10=%.4f",
		cross, overlapSum/float64(cross), recallSum/float64(cross))

	// Verdict.
	if lowCount == 0 && recallSum/float64(cross) >= 0.95 {
		t.Logf("PASS: índice coerente com o nomic (sem mistura); espaços compatíveis")
	} else if lowCount > 0 {
		t.Logf("FAIL/INCONCLUSIVE: %d vetores com cosine < 0.99 vs nomic — MISTURA detectada", lowCount)
	} else {
		t.Logf("INCONCLUSIVE: recall cruzado baixo — investigar")
	}
}

func cosine(a, b []float64) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func minFloat(vs []float64) float64 {
	m := 1.0
	for _, v := range vs {
		if v < m {
			m = v
		}
	}
	return m
}

var _ = sql.ErrNoRows
