package indexer

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"testing"

	"github.com/CoscaAI/cosca/internal/chunker"
	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/CoscaAI/cosca/internal/sqlite"
	"github.com/stretchr/testify/require"
)

// ── Dedup pré-embed ──────────────────────────────────────────────────────
//
// A campanha 2026-08-21 exige que a deduplicação aconteça ANTES de chamar o
// provider de embedding: conteúdo já presente no estoque (com vetor) reutiliza
// o vetor existente e NÃO invoca o provider. Estes testes provam a economia de
// chamadas e a equivalência do resultado.

// countingProvider é um provider fake que conta quantos textos foram enviados
// ao "modelo" — a métrica central do dedup pré-embed.
type countingProvider struct {
	mu       sync.Mutex
	calls    int
	texts    []string
	dimension int
}

func (p *countingProvider) GenerateEmbedding(_ context.Context, text string) (*embeddings.EmbeddingResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	p.texts = append(p.texts, text)
	return &embeddings.EmbeddingResult{Vector: fakeVector(text, 4), Model: "fake", Dimensions: 4}, nil
}

func (p *countingProvider) GenerateEmbeddings(_ context.Context, texts []string) ([]*embeddings.EmbeddingResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	p.texts = append(p.texts, texts...)
	results := make([]*embeddings.EmbeddingResult, len(texts))
	for i, t := range texts {
		results[i] = &embeddings.EmbeddingResult{Vector: fakeVector(t, 4), Model: "fake", Dimensions: 4}
	}
	return results, nil
}

func (p *countingProvider) Model() string            { return "fake" }
func (p *countingProvider) Dimensions() int          { return 4 }
func (p *countingProvider) Name() string             { return "fake" }
func (p *countingProvider) Close() error             { return nil }

func (p *countingProvider) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

func fakeVector(text string, dim int) []float64 {
	v := make([]float64, dim)
	for i := range v {
		v[i] = float64(len(text) + i)
	}
	return v
}

// newDedupTestIndexer cria um indexer com DB real em memória (tabelas chunks e
// vectors) e um provider fake contador. O vetor canônico é inserido
// diretamente no DB para simular conteúdo já indexado em uma passada anterior.
func newDedupTestIndexer(t *testing.T, seedContent string) (*Indexer, *countingProvider) {
	t.Helper()

	// IMPORTANTE: Path ":memory:" + pool de conexões criaria um banco SEPARADO
	// por conexão (a tabela criada numa conexão seria invisível na outra).
	// MaxOpenConns=1 força UMA única conexão reutilizada — requisito do :memory:.
	db, err := sqlite.Open(sqlite.Config{Path: ":memory:", AutoMigrate: false, MaxOpenConns: 1, MaxIdleConns: 1})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	// Tabelas mínimas necessárias para a consulta de dedup (JOIN chunks×vectors).
	_, err = db.Exec(`CREATE TABLE chunks (
		id TEXT PRIMARY KEY, document_id TEXT NOT NULL DEFAULT '',
		content TEXT NOT NULL DEFAULT '', hash TEXT NOT NULL DEFAULT '',
		heading TEXT NOT NULL DEFAULT '', section_type TEXT NOT NULL DEFAULT '',
		position INTEGER NOT NULL DEFAULT 0, token_count INTEGER NOT NULL DEFAULT 0,
		metadata_json TEXT NOT NULL DEFAULT '{}'
	)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE vectors (
		id TEXT PRIMARY KEY, vector BLOB NOT NULL,
		metadata TEXT NOT NULL DEFAULT '{}', document_id TEXT NOT NULL DEFAULT '',
		chunk_id TEXT NOT NULL DEFAULT '', entity_id TEXT NOT NULL DEFAULT '',
		content TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`)
	require.NoError(t, err)

	// Seed: um chunk canônico com vetor já armazenado.
	if seedContent != "" {
		_, err = db.Exec(`INSERT INTO chunks (id, document_id, content, hash) VALUES ('canon-1', 'doc-seed', ?, 'h1')`, seedContent)
		require.NoError(t, err)
		vec := vectorBlob(t, fakeVector(seedContent, 4))
		_, err = db.Exec(`INSERT INTO vectors (id, vector, document_id, chunk_id, content) VALUES ('v-canon-1', ?, 'doc-seed', 'canon-1', ?)`, vec, seedContent)
		require.NoError(t, err)
	}

	prov := &countingProvider{dimension: 4}
	reg := embeddings.GetRegistry()
	reg.Register("fake", func(_ context.Context, _ *embeddings.Config) (embeddings.Provider, error) {
		return prov, nil
	}, "fake", 1)
	// O registry só usa o provider como "selected" após Select — sem isso,
	// GenerateEmbeddings retorna "no embedding provider selected".
	err = reg.Select(context.Background(), embeddings.ProviderRegistryConfig{
		Primary: "fake",
		Model:   "fake",
	})
	require.NoError(t, err)

	idx := New(DefaultConfig(), nil, nil, nil, reg, nil, nil, db, nil)
	idx.cfg.RequireEmbeddings = false
	idx.cfg.BuildGraph = false
	return idx, prov
}

func vectorBlob(t *testing.T, v []float64) []byte {
	t.Helper()
	// Mesma codificação do SQLiteVec (float32, little-endian, 4 bytes/valor).
	b := make([]byte, len(v)*4)
	for i, val := range v {
		bits := math.Float32bits(float32(val))
		binary.LittleEndian.PutUint32(b[i*4:], bits)
	}
	return b
}

// TestDedupPreEmbed_ReusesExistingVector prova que conteúdo idêntico ao do
// estoque NÃO gera chamada ao provider: o vetor é reutilizado.
func TestDedupPreEmbed_ReusesExistingVector(t *testing.T) {
	content := "seção genérica compartilhada entre documentos"
	idx, prov := newDedupTestIndexer(t, content)

	chunks := []chunker.Chunk{
		{ID: "new-1", DocumentID: "doc-a", Content: content, Position: 1, SectionType: "section"},
		{ID: "new-2", DocumentID: "doc-a", Content: "conteúdo totalmente novo aqui", Position: 2, SectionType: "section"},
	}

	vectors, missing, err := idx.embedChunksWithMissing(context.Background(), chunks)
	require.NoError(t, err)
	require.Zero(t, missing)
	require.Len(t, vectors, 2, "deve retornar vetores para ambos os chunks")

	// O provider foi chamado APENAS UMA vez (para o conteúdo novo) — o chunk
	// idêntico ao estoque foi deduplicado pré-embed.
	require.Equal(t, 1, prov.callCount(), "provider deve ser chamado 1x (apenas conteúdo novo)")
	require.Equal(t, 1, len(prov.texts), "apenas 1 texto enviado ao provider")
	require.Equal(t, "conteúdo totalmente novo aqui", prov.texts[0], "o texto enviado é o novo, não o duplicado")

	// Os vetores são idênticos para o mesmo content (determinismo do modelo).
	require.Equal(t, fakeVector(content, 4), vectors[0].Vector)
	require.Equal(t, "new-1", vectors[0].ChunkID, "o chunk novo reutiliza o vetor mas mantém o ID próprio")
	require.Equal(t, "new-2", vectors[1].ChunkID)
}

// TestDedupPreEmbed_NoSeed_EmbedsEverything: sem estoque prévio, todos os
// chunks são enviados ao provider (comportamento igual ao pré-campanha).
func TestDedupPreEmbed_NoSeed_EmbedsEverything(t *testing.T) {
	idx, prov := newDedupTestIndexer(t, "")

	chunks := []chunker.Chunk{
		{ID: "a1", DocumentID: "doc-a", Content: "texto um"},
		{ID: "a2", DocumentID: "doc-a", Content: "texto dois"},
	}

	vectors, missing, err := idx.embedChunksWithMissing(context.Background(), chunks)
	require.NoError(t, err)
	require.Len(t, vectors, 2)
	require.Zero(t, missing)
	require.Equal(t, 1, prov.callCount(), "sem estoque, uma chamada em batch para ambos")
	require.Len(t, prov.texts, 2)
}

// TestDedupPreEmbed_AllDuplicated: todos os chunks idênticos ao estoque →
// zero chamadas ao provider.
func TestDedupPreEmbed_AllDuplicated(t *testing.T) {
	content := "bloco repetido cem vezes"
	idx, prov := newDedupTestIndexer(t, content)

	chunks := []chunker.Chunk{
		{ID: "d1", DocumentID: "doc-a", Content: content},
		{ID: "d2", DocumentID: "doc-a", Content: content},
	}

	vectors, missing, err := idx.embedChunksWithMissing(context.Background(), chunks)
	require.NoError(t, err)
	require.Len(t, vectors, 2)
	require.Zero(t, missing)
	require.Equal(t, 0, prov.callCount(), "conteúdo 100% duplicado → zero chamadas ao provider")
	require.Len(t, prov.texts, 0)
	// IDs próprios preservados.
	require.Equal(t, "d1", vectors[0].ChunkID)
	require.Equal(t, "d2", vectors[1].ChunkID)
}

var _ = fmt.Sprintf
