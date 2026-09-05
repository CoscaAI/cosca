// Package memory — Semantic Memory Retrieval (o gap §4.3 do INDEX.md C2).
//
// ANTES: a memória era recuperada por CAMINHO (path-first) + FTS5 — nunca por
// SIGNIFICADO. O INDEX.md da memória semântica (embed) é um documento
// derivado, não um mecanismo de busca. Este arquivo implementa o RETRIEVAL
// SEMÂNTICO: embebe os registros de memória (nomic-embed-text, 768-dim) e
// busca por similaridade de cosseno — "search by meaning, not path".
//
// Arquitetura:
//   - semantic.db: índice vetorial próprio (id, text, vector BLOB, layer,
//     agent, created_at) — separado do index.db FTS5 (que continua intacto);
//   - Index(record): embebe e armazena/atualiza o vetor do registro;
//   - SearchSemantic(query): embebe a query e retorna os top-K por cosseno;
//   - Drop(id): remove o vetor de um registro removido.
//
// É uma camada COMPLEMENTAR ao FileStore/SQLiteIndex — não substitui nada.
package memory

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/CoscaAI/cosca/internal/privfile"

	_ "modernc.org/sqlite"
)

// SemanticResult é um hit do retrieval semântico.
type SemanticResult struct {
	ID      string  `json:"id"`
	Content string  `json:"content"`
	Layer   string  `json:"layer"`
	Agent   string  `json:"agent,omitempty"`
	Score   float64 `json:"score"` // similaridade de cosseno (0..1)
}

// SemanticMemory é o índice vetorial da memória (retrieval por significado).
type SemanticMemory struct {
	db       *sql.DB
	embedder embeddings.Provider
	dim      int
}

// OpenSemanticMemory abre (ou cria) o índice semântico da memória.
// `embedder` é o provider de embeddings (ollama/nomic-embed-text). Se nil,
// o índice abre mas o SearchSemantic retorna erro claro (sem provider).
func OpenSemanticMemory(dataDir string, embedder embeddings.Provider) (*SemanticMemory, error) {
	path := filepath.Join(dataDir, "semantic.db")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("mkdir semantic: %w", err)
	}
	if err := privfile.EnsurePrivateDBFile(path); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open semantic db: %w", err)
	}
	for _, p := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}
	ddl := `
	CREATE TABLE IF NOT EXISTS semantic_memory (
		id         TEXT PRIMARY KEY,
		content    TEXT NOT NULL,
		vector     BLOB NOT NULL,
		layer      TEXT NOT NULL DEFAULT '',
		agent      TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_semantic_layer ON semantic_memory(layer);
	`
	if _, err := db.Exec(ddl); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create semantic schema: %w", err)
	}

	dim := 768 // nomic-embed-text default
	if embedder != nil {
		if d := embedder.Dimensions(); d > 0 {
			dim = d
		}
	}
	return &SemanticMemory{db: db, embedder: embedder, dim: dim}, nil
}

// Close fecha o índice semântico.
func (s *SemanticMemory) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Index embebe um registro de memória e armazena o vetor (UPSERT pelo ID).
func (s *SemanticMemory) Index(ctx context.Context, rec MemoryRecord) error {
	if s == nil || s.db == nil {
		return nil
	}
	if s.embedder == nil {
		return fmt.Errorf("semantic memory: embedding provider não disponível")
	}
	if rec.Content == "" {
		return nil // nada a embedar
	}

	res, err := s.embedder.GenerateEmbedding(ctx, rec.Content)
	if err != nil {
		return fmt.Errorf("embed memory %s: %w", rec.ID, err)
	}
	vec := encodeVector(res.Vector)
	if len(vec) == 0 {
		return nil
	}
	_, err = s.db.Exec(`
		INSERT INTO semantic_memory (id, content, vector, layer, agent)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			content=excluded.content, vector=excluded.vector,
			layer=excluded.layer, agent=excluded.agent,
			created_at=datetime('now')`,
		rec.ID, rec.Content, vec, string(rec.Layer), rec.Agent,
	)
	if err != nil {
		return fmt.Errorf("store semantic %s: %w", rec.ID, err)
	}
	return nil
}

// SearchSemantic busca memória por SIGNIFICADO: embebe a query e retorna os
// top-K registros por similaridade de cosseno. Filtros opcionais de camada e
// agente (vazio = todos).
func (s *SemanticMemory) SearchSemantic(ctx context.Context, query string, limit int, layer, agent string) ([]SemanticResult, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if s.embedder == nil {
		return nil, fmt.Errorf("semantic memory: embedding provider não disponível")
	}
	if limit <= 0 {
		limit = 5
	}
	if query == "" {
		return nil, nil
	}

	// Embebe a query (a capability real — mesmo smoke test do setup).
	res, err := s.embedder.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	qVec := res.Vector

	// Carrega todos os vetores do escopo e calcula cosseno (brute-force —
	// memória é pequena; ANN é evolução futura, como documentado no C2 §4.2).
	q := "SELECT id, content, vector, layer, agent FROM semantic_memory WHERE 1=1"
	var args []any
	if layer != "" {
		q += " AND layer = ?"
		args = append(args, layer)
	}
	if agent != "" {
		q += " AND agent = ?"
		args = append(args, agent)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("query semantic: %w", err)
	}
	defer rows.Close()

	var results []SemanticResult
	for rows.Next() {
		var id, content, layerS, agentS string
		var vecBlob []byte
		if err := rows.Scan(&id, &content, &vecBlob, &layerS, &agentS); err != nil {
			continue
		}
		vec := decodeVector(vecBlob)
		if len(vec) != len(qVec) {
			continue // dimensão incompatível — skip (não derruba)
		}
		score := cosine(qVec, vec)
		results = append(results, SemanticResult{
			ID: id, Content: content, Layer: layerS, Agent: agentS, Score: score,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate semantic: %w", err)
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// Drop remove o vetor de um registro (quando a memória é apagada).
func (s *SemanticMemory) Drop(id string) error {
	if s == nil || s.db == nil {
		return nil
	}
	_, err := s.db.Exec("DELETE FROM semantic_memory WHERE id = ?", id)
	return err
}

// Count devolve o total de vetores indexados.
func (s *SemanticMemory) Count() (int, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM semantic_memory").Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// ── helpers ────────────────────────────────────────────────────────────────

// encodeVector serializa []float64 para BLOB little-endian.
func encodeVector(v []float64) []byte {
	if len(v) == 0 {
		return nil
	}
	buf := make([]byte, len(v)*8)
	for i, f := range v {
		binary.LittleEndian.PutUint64(buf[i*8:], math.Float64bits(f))
	}
	return buf
}

// decodeVector desserializa BLOB little-endian para []float64.
func decodeVector(b []byte) []float64 {
	if len(b) == 0 || len(b)%8 != 0 {
		return nil
	}
	out := make([]float64, len(b)/8)
	for i := range out {
		out[i] = math.Float64frombits(binary.LittleEndian.Uint64(b[i*8:]))
	}
	return out
}

// cosine calcula a similaridade de cosseno entre dois vetores.
func cosine(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
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
