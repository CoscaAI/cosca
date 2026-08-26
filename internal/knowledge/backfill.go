package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// FASE 5 — Backfill epistêmico (aditivo, idempotente, reversível).
//
// O legado do knowledge.db foi indexado ANTES da Fase 2/4: os documentos não
// têm `metadata_json.epistemic` (verificado na 4.2: 0/2387 docs, 0/38854
// chunks). Este backfill POVOA a classe epistêmica DETERMINÍSTICA de cada
// documento existente — sem re-indexar, sem tocar chunks/vectors/grafo, sem
// migração destrutiva. Corre pela Fase 2 (classificador) + Fase 4
// (KnowledgeEpistemic) + regra de ouro de scope.
//
// Garantias ("do jeito certo"):
//   - ADITIVO: merge no metadata_json; nunca remove chave existente;
//   - IDEMPOTENTE: documento que já tem `epistemic` é pulado (already_labeled);
//   - FAIL-CLOSED: transiente / baixa confiança → não classifica (fica como está);
//   - REVERSÍVEL: é só metadata (remove as chaves = volta ao estado anterior);
//   - PROVENIÊNCIA: cada item registra kind/scope/confidence para auditoria.
type EpistemicBackfillItem struct {
	ID         string  `json:"id"`
	Path       string  `json:"path"`
	Kind       string  `json:"kind,omitempty"`
	Scope      string  `json:"scope,omitempty"`
	Epistemic  string  `json:"epistemic,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
	Status     string  `json:"status"`
}

// Status do item no backfill.
const (
	BackfillStatusUpdated    = "updated"               // epistemic aplicado (ou dry-run apontaria)
	BackfillStatusAlready    = "already_labeled"       // já tinha epistemic, pulado
	BackfillStatusMissing    = "missing_file"          // arquivo não pôde ser lido
	BackfillStatusNotPersist = "skipped_not_persistent" // classificador não persistiu (fail-closed)
	BackfillStatusNoKind     = "skipped_no_kind"       // sem kind classificável
)

// EpistemicBackfillReport é o resumo da operação (fácil de auditar).
type EpistemicBackfillReport struct {
	Scanned        int                     `json:"scanned"`
	Updated        int                     `json:"updated"`
	AlreadyLabeled int                     `json:"already_labeled"`
	MissingFile    int                     `json:"missing_file"`
	NotPersistent  int                     `json:"skipped_not_persistent"`
	NoKind         int                     `json:"skipped_no_kind"`
	ByEpistemic    map[string]int          `json:"by_epistemic"`
	Items          []EpistemicBackfillItem `json:"items"`
}

// BackfillEpistemic percorre os documentos existentes no banco e povoa
// `metadata_json.epistemic` (+kind/scope) de forma aditiva e idempotente.
//
// dryRun=true NÃO escreve nada — apenas calcula e reporta. Use sempre dryRun
// primeiro (o espelho/validação), depois a aplicação real.
func (e *Engine) BackfillEpistemic(ctx context.Context, dryRun bool) (*EpistemicBackfillReport, error) {
	e.mu.RLock()
	db := e.db
	initialized := e.initialized
	mdParser := e.mdParser
	e.mu.RUnlock()

	if !initialized || db == nil {
		return nil, fmt.Errorf("knowledge engine not initialized")
	}

	report := &EpistemicBackfillReport{ByEpistemic: map[string]int{}}

	rows, err := db.Query(`SELECT id, path, metadata_json FROM documents ORDER BY path`)
	if err != nil {
		return nil, fmt.Errorf("query documents: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var id, path string
		var metadataJSON nullStr
		if err := rows.Scan(&id, &path, &metadataJSON); err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}

		report.Scanned++
		meta := decodeMetadata(metadataJSON.String())

		// Idempotência: já tem epistemic → pula (já classificado/proveniente).
		if _, ok := meta["epistemic"]; ok {
			report.AlreadyLabeled++
			report.Items = append(report.Items, EpistemicBackfillItem{ID: id, Path: path, Status: BackfillStatusAlready})
			continue
		}

		content, rerr := os.ReadFile(path)
		if rerr != nil {
			report.MissingFile++
			report.Items = append(report.Items, EpistemicBackfillItem{ID: id, Path: path, Status: BackfillStatusMissing})
			continue
		}

		var fm map[string]any
		if mdParser != nil {
			if d, perr := mdParser.Parse(path, string(content)); perr == nil && d != nil && d.Frontmatter.Data != nil {
				fm = d.Frontmatter.Data
			}
		}
		if fm == nil {
			fm = map[string]any{}
		}

		agent := agentFromPath(path)
		cls := ClassifyDoc(path, fm, string(content), agent)
		if !cls.Persistent {
			report.NotPersistent++
			report.Items = append(report.Items, EpistemicBackfillItem{ID: id, Path: path, Status: BackfillStatusNotPersist, Kind: cls.Kind, Scope: cls.Scope, Confidence: cls.Confidence})
			continue
		}
		if cls.Kind == "" {
			report.NoKind++
			report.Items = append(report.Items, EpistemicBackfillItem{ID: id, Path: path, Status: BackfillStatusNoKind})
			continue
		}

		epi := string(EpistemicFor(cls.Kind))

		// Merge ADITIVO: só define as chaves quando ausentes (nunca sobrescreve).
		if _, ok := meta["kind"]; !ok {
			meta["kind"] = cls.Kind
		}
		if _, ok := meta["scope"]; !ok {
			meta["scope"] = cls.Scope
		}
		meta["epistemic"] = epi

		if origin := origin(path); origin != "" {
			if _, ok := meta["origin"]; !ok {
				meta["origin"] = origin
			}
		}
		if agent != "" {
			if _, ok := meta["agent"]; !ok {
				meta["agent"] = agent
			}
		}

		encoded := encodeMetadata(meta)

		if !dryRun {
			if _, err := db.Exec(
				`UPDATE documents SET metadata_json = ? WHERE id = ?`, encoded, id); err != nil {
				return nil, fmt.Errorf("update metadata %s: %w", path, err)
			}
		}

		report.Updated++
		report.ByEpistemic[epi]++
		report.Items = append(report.Items, EpistemicBackfillItem{ID: id, Path: path, Kind: cls.Kind, Scope: cls.Scope, Epistemic: epi, Confidence: cls.Confidence, Status: BackfillStatusUpdated})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate documents: %w", err)
	}

	return report, nil
}

// decodeMetadata faz o parse de metadata_json (NULL/"{}" tolerados).
func decodeMetadata(raw string) map[string]any {
	m := map[string]any{}
	if raw == "" {
		return m
	}
	_ = json.Unmarshal([]byte(raw), &m)
	return m
}

// encodeMetadata serializa o map de metadata de volta a JSON compacto.
func encodeMetadata(m map[string]any) string {
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// agentFromPath deriva o nome do agente dono de um path de memória de agente
// (.../memory/agent/<nome>/...). Empty para fontes não-agente.
func agentFromPath(path string) string {
	parts := strings.Split(slashPath(path), "/memory/agent/")
	if len(parts) < 2 {
		return ""
	}
	rest := strings.TrimPrefix(parts[1], "/")
	if i := strings.Index(rest, "/"); i > 0 {
		return rest[:i]
	}
	return ""
}

// nullStr aceita o scan de um metadata_json possivelmente NULL.
type nullStr string

func (j *nullStr) Scan(v any) error {
	switch t := v.(type) {
	case nil:
		*j = ""
	case []byte:
		*j = nullStr(string(t))
	case string:
		*j = nullStr(t)
	default:
		*j = nullStr(fmt.Sprint(t))
	}
	return nil
}

func (j nullStr) String() string { return string(j) }
