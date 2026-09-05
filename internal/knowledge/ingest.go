package knowledge

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// IngestStatus descreve o resultado de uma ingestão de learning block.
type IngestStatus string

const (
	// IngestStatusIngested indica que o bloco foi indexado com sucesso.
	IngestStatusIngested IngestStatus = "ingested"
	// IngestStatusAlreadyIngested indica que um documento com o mesmo
	// SHA256 (conteúdo) já existe — dedup por conteúdo, nada duplicado.
	IngestStatusAlreadyIngested IngestStatus = "already_ingested"
	// IngestStatusSkippedNotPersistent indica que o classificador NÃO
	// persistiu o bloco (transiente/baixa confiança) — fail-closed, não indexa.
	IngestStatusSkippedNotPersistent IngestStatus = "skipped_not_persistent"
)

// IngestResult reporta o desfecho de IngestLearningBlock.
type IngestResult struct {
	Status   IngestStatus `json:"status"`
	Hash     string       `json:"hash"`
	Scope    string       `json:"scope,omitempty"`
	Kind     string       `json:"kind,omitempty"`
	Reason   string       `json:"reason,omitempty"`
	Document string       `json:"document_id,omitempty"`
}

// IngestLearningBlock é o gatilho automático de ingestão de um learning block
// recém-registrado (ou de um bloco pré-existente referenciado por path).
//
// Fluxo: lê o bloco → classifica (fail-closed) → verifica idempotência por
// SHA256 (o nome do arquivo é <hash>.md = hash do conteúdo) → indexa com a
// proveniência semântica (scope/origin/kind/agent) mesclada em metadata_json.
//
// O bloco JÁ tem hash = nome do arquivo. Se um documento com esse hash já
// existe no knowledge base → retorna AlreadyIngested sem duplicar. Se o
// classificador não persistir o bloco → retorna SkippedNotPersistent e NÃO
// indexa (fail-closed: nunca indexar o que não foi classificado).
//
// scope é decidido SEMPRE pelo classificador (path = proveniência física;
// scope = jurisdição semântica). A regra de ouro é preservada: ingestão de um
// aprendizado de projeto (origem não-clock) define scope=project; scope=global
// é reservado ao cérebro embarcado (internal/embed/cosca/**) ou a promoção
// explícita. Ingestão NUNCA promove project→global.
func (e *Engine) IngestLearningBlock(ctx context.Context, blockPath string, agent string) (*IngestResult, error) {
	e.mu.RLock()
	if !e.initialized {
		e.mu.RUnlock()
		return nil, fmt.Errorf("knowledge engine not initialized")
	}
	if e.mdParser == nil {
		e.mu.RUnlock()
		return nil, fmt.Errorf("markdown parser not available")
	}
	e.mu.RUnlock()

	// 1. Ler o conteúdo do bloco.
	content, err := os.ReadFile(blockPath)
	if err != nil {
		return nil, fmt.Errorf("read block: %w", err)
	}

	// 2. O bloco é nomeado pelo SHA256 do conteúdo (<hash>.md).
	sum := sha256.Sum256(content)
	hash := hex.EncodeToString(sum[:])
	base := strings.TrimSuffix(filepath.Base(blockPath), filepath.Ext(blockPath))
	if base != hash {
		return nil, fmt.Errorf("block path %s: filename %q != sha256(%q)", blockPath, base, hash)
	}

	// 3. Parse do frontmatter (para o classificador).
	doc, perr := e.mdParser.Parse(blockPath, string(content))
	var fm map[string]any
	if perr == nil && doc != nil && doc.Frontmatter.Data != nil {
		fm = doc.Frontmatter.Data
	} else {
		fm = map[string]any{}
	}

	// 4. Classificação determinística (fail-closed).
	cls := ClassifyDoc(blockPath, fm, string(content), agent)
	if !cls.Persistent {
		return &IngestResult{
			Status: IngestStatusSkippedNotPersistent,
			Hash:   hash,
			Scope:  cls.Scope,
			Kind:   cls.Kind,
			Reason: cls.Reason,
		}, nil
	}

	// 5. Idempotência por conteúdo (SHA256): não duplicar o mesmo aprendizado.
	if docID, ok := e.documentIDByHash(hash); ok {
		return &IngestResult{
			Status:   IngestStatusAlreadyIngested,
			Hash:     hash,
			Scope:    cls.Scope,
			Kind:     cls.Kind,
			Document: docID,
		}, nil
	}

	// 6. Proveniência semântica => metadata_json (merge no indexer).
	meta := map[string]any{
		"scope":     cls.Scope,
		"origin":    origin(blockPath),
		"kind":      cls.Kind,
		"epistemic": string(EpistemicFor(cls.Kind)), // FASE 4 — classe epistêmica estrutural
	}
	if agent != "" {
		meta["agent"] = agent
	}
	if proj := filepath.Base(e.cfg.RootDir); proj != "" && proj != "." {
		meta["project"] = proj
	}

	if err := e.indexer.IndexDocumentWithMeta(ctx, blockPath, meta); err != nil {
		return nil, fmt.Errorf("ingest block: %w", err)
	}

	docID, _ := e.documentIDByHash(hash)
	return &IngestResult{
		Status:   IngestStatusIngested,
		Hash:     hash,
		Scope:    cls.Scope,
		Kind:     cls.Kind,
		Reason:   cls.Reason,
		Document: docID,
	}, nil
}

// documentIDByHash retorna o id do documento que já possui o hash de conteúdo
// informado (idempotência por SHA256). Ok=false quando nenhum documento existe.
func (e *Engine) documentIDByHash(hash string) (string, bool) {
	e.mu.RLock()
	db := e.db
	e.mu.RUnlock()
	if db == nil {
		return "", false
	}
	var id string
	err := db.QueryRow(`SELECT id FROM documents WHERE hash = ? LIMIT 1`, hash).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false
		}
		return "", false
	}
	return id, true
}

// origin deriva a proveniência física (origin) do path: qual canal de fonte
// deu origem ao documento. Usado como metadata de provenance.
func origin(path string) string {
	norm := slashPath(path)
	switch {
	case strings.Contains(norm, "/.opencode/cosca/memory/agent/"):
		return "opencode"
	case strings.Contains(norm, "/.cosca/fallback/memory/agent/"):
		return "fallback"
	case strings.Contains(norm, "/internal/embed/cosca/"):
		return "embed"
	case strings.Contains(norm, "/docs/"):
		return "docs"
	default:
		return "filesystem"
	}
}
