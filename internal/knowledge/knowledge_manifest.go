package knowledge

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
)

// FASE "MANIFESTO" — o índice do índice, versionado e à prova de adulteração.
//
// O knowledge.db (254MB) é um CACHE DERIVADO (vetores+grafo+FTS) e está fora do
// git (re-gerável via cosca knowledge index/classify/vectors-backfill). Para o
// estado do conhecimento NÃO se perder sem inflar o repo, este manifesto é o
// "índice do índice": um arquivo PEQUENO (path + hash do conteúdo + classe
// epistêmica + escopo + kind de cada documento) que registra no GIT o estado
// real do conhecimento. A fonte (docs/memória/embed) + este manifesto regeneram
// e VERIFICAM o índice de forma determinística — o estado não se perde e é
// rastreável (uma adulteração do índice aparece como divergência path/hash/classe).
type KnowledgeManifestEntry struct {
	Path       string  `json:"path"`
	Hash       string  `json:"hash"`
	Epistemic  string  `json:"epistemic,omitempty"`
	Scope      string  `json:"scope,omitempty"`
	Kind       string  `json:"kind,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

type KnowledgeManifest struct {
	GeneratedAt time.Time                `json:"generated_at"`
	Count       int                      `json:"count"`
	Entries     []KnowledgeManifestEntry `json:"entries"`
}

// BuildKnowledgeManifest lê todos os documentos indexados (path + hash de
// conteúdo + metadata_json com a classe epistêmica/scope/kind do FASE 4/5.1) e
// produz o manifesto rastreável. É read-only sobre o estado atual do índice.
func (e *Engine) BuildKnowledgeManifest(ctx context.Context) (*KnowledgeManifest, error) {
	e.mu.RLock()
	db := e.db
	initialized := e.initialized
	rootDir := e.cfg.RootDir
	e.mu.RUnlock()
	if !initialized || db == nil {
		return nil, fmt.Errorf("knowledge engine not initialized")
	}

	rows, err := db.Query(`SELECT path, hash, metadata_json FROM documents ORDER BY path`)
	if err != nil {
		return nil, fmt.Errorf("query documents: %w", err)
	}
	defer rows.Close()

	man := &KnowledgeManifest{GeneratedAt: time.Now().UTC()}
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var path, hash string
		var metadataJSON nullStr
		if err := rows.Scan(&path, &hash, &metadataJSON); err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}
		meta := decodeMetadata(metadataJSON.String())
		rel := path
		if rootDir != "" && rootDir != "." {
			if r, err := filepath.Rel(rootDir, path); err == nil {
				rel = r
			}
		}
		entry := KnowledgeManifestEntry{Path: filepath.ToSlash(rel), Hash: hash}
		if v, ok := meta["epistemic"].(string); ok {
			entry.Epistemic = v
		}
		if v, ok := meta["scope"].(string); ok {
			entry.Scope = v
		}
		if v, ok := meta["kind"].(string); ok {
			entry.Kind = v
		}
		if c, ok := meta["confidence"].(float64); ok {
			entry.Confidence = c
		}
		man.Entries = append(man.Entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	man.Count = len(man.Entries)
	return man, nil
}
