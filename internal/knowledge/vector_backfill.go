package knowledge

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/CoscaAI/cosca/internal/vector"
)

// FASE 5.3 — Backfill de vetores (cobertura completa do significado).
//
// Diagnóstico: o legado tem chunks indexados sem vetor (9854/38854, medidos na
// auditoria 5.3) — e NENHUM tem `chunks.embedding` reaproveitável. Ou seja, o
// significado desses chunks não existe no índice vetorial: a busca semântica não
// os alcança. Este backfill EMBEDE o conteúdo de cada chunk sem vetor e o
// armazena — derivado do conteúdo (não invento significado), idempotente (só os
// que faltam), aditivo (não toca nos existentes), não-destrutivo.
//
// dryRun=true NÃO embede nem escreve: só reporta o gap (auditoria segura).
// A geração real roda com o provider local (Ollama nomic-embed-text, 768-dim) —
// inferência LOCAL, compatível com a ALMA ("cofre fechado").
type VectorBackfillReport struct {
	ChunksTotal int               `json:"chunks_total"`
	Missing     int               `json:"missing"`
	Filled      int               `json:"filled"`
	Failed      int               `json:"failed"`
	ByArea      map[string]int    `json:"by_area"`
	DurationSec float64           `json:"duration_sec"`
}

func (e *Engine) BackfillVectors(ctx context.Context, dryRun bool) (*VectorBackfillReport, error) {
	e.mu.RLock()
	db := e.db
	initialized := e.initialized
	embRegistry := e.embRegistry
	vecStore := e.vecStore
	e.mu.RUnlock()

	if !initialized || db == nil {
		return nil, fmt.Errorf("knowledge engine not initialized")
	}
	if embRegistry == nil || vecStore == nil {
		return nil, fmt.Errorf("embedding registry/vector store unavailable (initialized?)")
	}

	rep := &VectorBackfillReport{ByArea: map[string]int{}}
	start := time.Now()

	rows, err := db.Query(`SELECT c.id, c.document_id, d.path, c.content
		FROM chunks c JOIN documents d ON c.document_id = d.id
		WHERE NOT EXISTS (SELECT 1 FROM vectors v WHERE v.chunk_id = c.id)
		ORDER BY c.document_id`)
	if err != nil {
		return nil, fmt.Errorf("query chunks without vectors: %w", err)
	}
	defer rows.Close()

	type chunk struct{ id, docID, path, content string }
	var missing []chunk
	for rows.Next() {
		var c chunk
		if err := rows.Scan(&c.id, &c.docID, &c.path, &c.content); err != nil {
			return nil, fmt.Errorf("scan chunk: %w", err)
		}
		missing = append(missing, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rep.Missing = len(missing)
	_ = db.QueryRow(`SELECT COUNT(*) FROM chunks`).Scan(&rep.ChunksTotal)
	for _, c := range missing {
		rep.ByArea[areaOf(c.path)]++
	}

	if dryRun || len(missing) == 0 {
		rep.DurationSec = time.Since(start).Seconds()
		return rep, nil
	}

	// Batch embedding (config batch_size=100). Processa em lotes e armazena.
	const batchSize = 100
	for i := 0; i < len(missing); i += batchSize {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		end := i + batchSize
		if end > len(missing) {
			end = len(missing)
		}
		batch := missing[i:end]
		texts := make([]string, len(batch))
		for j, c := range batch {
			texts[j] = c.content
		}
		results, err := embRegistry.GenerateEmbeddings(ctx, texts)
		if err != nil {
			log.Warn().Err(err).Int("chunks", len(batch)).Msg("embedding batch failed")
			rep.Failed += len(batch)
			continue
		}
		records := make([]vector.VectorRecord, 0, len(batch))
		for j, res := range results {
			if res == nil || len(res.Vector) == 0 {
				rep.Failed++
				continue
			}
			c := batch[j]
			records = append(records, vector.VectorRecord{
				ID:         uuid.New().String(),
				Vector:     res.Vector,
				DocumentID: c.docID,
				ChunkID:    c.id,
				Content:    c.content,
			})
		}
		if len(records) > 0 {
			dim := results[0].Dimensions
			if dim <= 0 {
				dim = len(records[0].Vector)
			}
			if err := vecStore.Store(dim, records); err != nil {
				log.Warn().Err(err).Int("records", len(records)).Msg("vector store failed")
				rep.Failed += len(records)
				continue
			}
			rep.Filled += len(records)
		}
	}

	rep.DurationSec = time.Since(start).Seconds()
	return rep, nil
}

// areaOf infere a área de um documento pelo segmento do seu path.
func areaOf(path string) string {
	n := strings.ToLower(slashPath(path))
	for _, a := range []string{"architecture", "adr", "fallback", "docs", "embed"} {
		if strings.Contains(n, a) {
			return a
		}
	}
	return "outro"
}
