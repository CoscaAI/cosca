package learning

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/CoscaAI/cosca/internal/embeddings"
)

// EmbedText gera o embedding do texto (título + tags) e devolve como BLOB
// float32 little-endian — pronto para armazenar na coluna embedding do vault.
func EmbedText(ctx context.Context, reg *embeddings.ProviderRegistry, text string) ([]byte, error) {
	if reg == nil {
		return nil, fmt.Errorf("learning: embedding registry nil")
	}
	res, err := reg.GenerateEmbedding(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("learning: embed: %w", err)
	}
	blob := make([]byte, len(res.Vector)*4)
	for i, v := range res.Vector {
		binary.LittleEndian.PutUint32(blob[i*4:], math.Float32bits(float32(v)))
	}
	return blob, nil
}

// SetEmbedding grava o BLOB de embedding de um trigger (por block_hash).
func SetEmbedding(db *sql.DB, blockHash string, blob []byte) error {
	_, err := db.Exec(`UPDATE triggers SET embedding = ? WHERE block_hash = ?`, blob, blockHash)
	if err != nil {
		return fmt.Errorf("learning: set embedding: %w", err)
	}
	return nil
}

// SemanticSearch busca por similaridade de cosseno sobre os embeddings do vault.
// O vault é pequeno (centenas de triggers) — carregar tudo e comparar em memória
// é rápido e honesto. Retorna os triggers ordenados por similaridade desc.
func SemanticSearch(ctx context.Context, reg *embeddings.ProviderRegistry, db *sql.DB, query string, limit int) ([]Trigger, error) {
	if limit <= 0 {
		limit = 20
	}
	qVec, err := EmbedText(ctx, reg, query)
	if err != nil {
		return nil, err
	}
	q := decodeVec(qVec)

	rows, err := db.Query(`SELECT id, agent, vault, date, title, level, tags, hash16, block_hash, chain_prev, embedding FROM triggers`)
	if err != nil {
		return nil, fmt.Errorf("learning: semantic scan: %w", err)
	}
	defer rows.Close()

	type scored struct {
		tr  Trigger
		sim float64
	}
	var results []scored
	for rows.Next() {
		var t Trigger
		var v string
		var blob []byte
		if err := rows.Scan(&t.ID, &t.Agent, &v, &t.Date, &t.Title, &t.Level,
			&t.Tags, &t.Hash16, &t.BlockHash, &t.ChainPrev, &blob); err != nil {
			return nil, fmt.Errorf("learning: semantic scan row: %w", err)
		}
		t.Vault = VaultName(v)
		if len(blob) == 0 {
			continue // sem embedding — não comparável
		}
		sim := cosine(q, decodeVec(blob))
		results = append(results, scored{tr: t, sim: sim})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Ordena por similaridade desc (insertion sort — vaults pequenos).
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].sim > results[j-1].sim; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}

	out := make([]Trigger, 0, min(limit, len(results)))
	for i := 0; i < len(results) && i < limit; i++ {
		out = append(out, results[i].tr)
	}
	return out, nil
}

// decodeVec converte BLOB float32 LE → []float64.
func decodeVec(blob []byte) []float64 {
	n := len(blob) / 4
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		out[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(blob[i*4:])))
	}
	return out
}

// cosine calcula a similaridade de cosseno entre dois vetores.
func cosine(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
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

// HybridSearch funde FTS5 (palavra-chave) + semântica (entendimento) via RRF
// (Reciprocal Rank Fusion): 1/(k+rank) por fonte, k=60. O melhor dos dois —
// precisão lexical + recall semântico.
func HybridSearch(ctx context.Context, reg *embeddings.ProviderRegistry, db *sql.DB, query string, limit int) ([]Trigger, error) {
	if limit <= 0 {
		limit = 20
	}
	const k = 60

	// FTS5 arm.
	ftsResults, ftsErr := Search(db, SanitizeFTS(query), 50)
	// Semântica arm (best-effort: sem embeddings, só FTS5).
	semResults, semErr := SemanticSearch(ctx, reg, db, query, 50)

	scores := make(map[string]float64) // block_hash → score
	order := make([]string, 0)         // preserva ordem de primeira aparição

	add := func(results []Trigger, err error) {
		if err != nil {
			return
		}
		for i, t := range results {
			if _, ok := scores[t.BlockHash]; !ok {
				order = append(order, t.BlockHash)
			}
			scores[t.BlockHash] += 1.0 / (float64(k) + float64(i+1))
		}
	}
	add(ftsResults, ftsErr)
	add(semResults, semErr)

	// Ordena por score desc (insertion sort — vaults pequenos).
	hashes := append([]string(nil), order...)
	for i := 1; i < len(hashes); i++ {
		for j := i; j > 0 && scores[hashes[j]] > scores[hashes[j-1]]; j-- {
			hashes[j], hashes[j-1] = hashes[j-1], hashes[j]
		}
	}

	// Busca os triggers completos pelos hashes ordenados.
	out := make([]Trigger, 0, min(limit, len(hashes)))
	for _, h := range hashes {
		if len(out) >= limit {
			break
		}
		var t Trigger
		var v string
		err := db.QueryRow(`SELECT id, agent, vault, date, title, level, tags, hash16, block_hash, chain_prev
			FROM triggers WHERE block_hash = ?`, h).
			Scan(&t.ID, &t.Agent, &v, &t.Date, &t.Title, &t.Level, &t.Tags, &t.Hash16, &t.BlockHash, &t.ChainPrev)
		if err != nil {
			continue
		}
		t.Vault = VaultName(v)
		out = append(out, t)
	}
	return out, nil
}

// EmbeddingText monta o texto a ser embedado a partir de um trigger
// (título + tags — leve, sem conteúdo completo).
func EmbeddingText(t Trigger) string {
	return strings.TrimSpace(t.Title + " " + t.Tags)
}

// EmbedAll embeds every trigger that lacks an embedding, across all vaults.
// Idempotent: triggers with an embedding are skipped. Uses the batch API
// (nomic-embed-text via ollama, 768 dims). Enriches the embedded text with the
// block's Task field (context) — the embedding is always 768 floats, so richer
// input costs ZERO extra storage. Returns the count embedded.
func EmbedAll(ctx context.Context, reg *embeddings.ProviderRegistry, vaultDir, agentsRoot string) (int, error) {
	entries, err := os.ReadDir(vaultDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil // no vaults yet
		}
		return 0, fmt.Errorf("learning: read vault dir: %w", err)
	}

	embedded := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".db") {
			continue
		}
		vault := VaultName(strings.TrimSuffix(e.Name(), ".db"))
		db, err := OpenVault(vaultDir, vault)
		if err != nil {
			return embedded, err
		}

		// Triggers sem embedding.
		rows, err := db.Query(`SELECT block_hash, agent, title, tags FROM triggers WHERE embedding IS NULL`)
		if err != nil {
			db.Close()
			return embedded, fmt.Errorf("learning: select unembedded %s: %w", vault, err)
		}
		type pending struct {
			hash  string
			agent string
			title string
			tags  string
		}
		var pend []pending
		for rows.Next() {
			var p pending
			if err := rows.Scan(&p.hash, &p.agent, &p.title, &p.tags); err != nil {
				rows.Close()
				db.Close()
				return embedded, err
			}
			pend = append(pend, p)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			db.Close()
			return embedded, err
		}

		for _, p := range pend {
			text := strings.TrimSpace(p.title + " " + p.tags)
			// Enriquece com o conteúdo do bloco (Task + Learned — o sinal
			// semântico real). O embedding tem tamanho FIXO (768 floats),
			// então texto mais rico custa ZERO de armazenamento extra.
			if content := blockContent(agentsRoot, p.agent, p.hash); content != "" {
				text = text + " " + content
			}
			blob, err := EmbedText(ctx, reg, text)
			if err != nil {
				db.Close()
				return embedded, fmt.Errorf("learning: embed %s/%s: %w", vault, p.hash[:min(16, len(p.hash))], err)
			}
			if err := SetEmbedding(db, p.hash, blob); err != nil {
				db.Close()
				return embedded, err
			}
			embedded++
		}
		db.Close()
	}
	return embedded, nil
}

// blockContent lê o bloco imutável e devolve um trecho semântico (primeiros
// ~1500 chars do corpo, pulando o header PREV/ID/TIME/LEVEL/TAGS). Retorna ""
// se o bloco não existir.
func blockContent(agentsRoot, agent, hash string) string {
	path := filepath.Join(agentsRoot, agent, "blocks", hash+".md")
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	s := string(content)
	// Pula o header (até o primeiro "---").
	if idx := strings.Index(s, "---"); idx >= 0 {
		s = s[idx+3:]
	}
	s = strings.TrimSpace(s)
	if len(s) > 1500 {
		s = s[:1500]
	}
	return s
}

// ClearEmbeddings apaga todos os embeddings dos vaults (para re-embed com
// texto enriquecido). Idempotente — vaults sem embedding são válidos (FTS5).
func ClearEmbeddings(vaultDir string) error {
	entries, err := os.ReadDir(vaultDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("learning: read vault dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".db") {
			continue
		}
		db, err := OpenVault(vaultDir, VaultName(strings.TrimSuffix(e.Name(), ".db")))
		if err != nil {
			return err
		}
		if _, err := db.Exec(`UPDATE triggers SET embedding = NULL`); err != nil {
			db.Close()
			return fmt.Errorf("learning: clear embeddings %s: %w", e.Name(), err)
		}
		db.Close()
	}
	return nil
}

// blockTask extrai o campo Task do bloco imutável (contexto do aprendizado).
// Retorna "" se o bloco não existir ou não tiver Task.
func blockTask(agentsRoot, agent, hash string) string {
	path := filepath.Join(agentsRoot, agent, "blocks", hash+".md")
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	// Procura a linha "| **Task** | ... |" e pega o texto até o próximo "|".
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "| **Task** |") {
			parts := strings.SplitN(trimmed, "|", 4)
			if len(parts) >= 3 {
				task := strings.TrimSpace(parts[2])
				if len(task) > 300 {
					task = task[:300]
				}
				return task
			}
		}
	}
	return ""
}