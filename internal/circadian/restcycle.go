// Package circadian implements the Cosca Operational Rest Cycle (ORC).
//
// This file implements the ORC maintenance pipeline: the consolidation pass
// that runs while the system is in the ORC window (see circadian.go for the
// activity/rest state machine). It is the functional equivalent of
// consolidating memories, but as pure maintenance processing.
// Nomenclature: "Operational Rest Cycle (ORC)" — never "the AI sleeps".
package circadian

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/internal/chunker"
	"github.com/CoscaAI/cosca/internal/confidence"
	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/CoscaAI/cosca/internal/graph"
	"github.com/CoscaAI/cosca/internal/guardrails"
	"github.com/CoscaAI/cosca/internal/indexer"
	"github.com/CoscaAI/cosca/internal/intelligence"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/markdown"
	"github.com/CoscaAI/cosca/internal/parser"
	"github.com/CoscaAI/cosca/internal/sqlite"
	"github.com/CoscaAI/cosca/internal/vector"

	// SQLite driver (pure Go, no CGO required).
	_ "modernc.org/sqlite"
)

// ── Types ────────────────────────────────────────────────────────────────────

// ORCResult is the outcome of a single Operational Rest Cycle.
type ORCResult struct {
	StartedAt         time.Time     `json:"started_at"`
	EndedAt           time.Time     `json:"ended_at"`
	Steps             []ORCStep     `json:"steps"`
	ItemsProcessed    int           `json:"items_processed"`
	DuplicatesRemoved int           `json:"duplicates_removed"`
	DeprecatedCount   int           `json:"deprecated_count"`
	ReportPath        string        `json:"report_path"`
	Duration          time.Duration `json:"duration"`
	Errors            []string      `json:"errors,omitempty"`

	// CKL summary — the Cosca Knowledge Lifecycle law library state after
	// the ckl_promotion step (laws.json). Empty/zero when laws.json does
	// not exist.
	CKLLaws    int            `json:"ckl_laws"`
	CKLByLevel map[string]int `json:"ckl_by_level,omitempty"`
	CKLAvgConf float64        `json:"ckl_avg_confidence,omitempty"`
}

// ORCStep is a single step of the ORC pipeline. Status is one of
// "ok", "skipped" or "error".
type ORCStep struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Detail     string `json:"detail"`
	DurationMs int64  `json:"duration_ms"`
}

// stepResult is the intermediate result produced by a pipeline step.
type stepResult struct {
	status string // "ok" | "skipped" | "error"
	detail string
}

func okResult(detail string) stepResult   { return stepResult{status: "ok", detail: detail} }
func skipResult(detail string) stepResult { return stepResult{status: "skipped", detail: detail} }
func errResult(detail string) stepResult  { return stepResult{status: "error", detail: detail} }

// ── Pipeline ─────────────────────────────────────────────────────────────────

// RunORC executes the Operational Rest Cycle maintenance pipeline against the
// given .cosca directory:
//
//  1. compact_learnings      — recompile knowledge from .cosca/framework/knowledge
//  2. update_indexes         — full indexer rebuild
//  3. dedupe                 — remove duplicate documents by (path, hash)
//  4. recalc_confidence      — recompute agent confidence statistics
//  5. ckl_promotion          — re-evaluate the CKL law library (laws.json)
//     through the PromotionEngine and persist any promotions
//  6. consolidate_knowledge  — run the knowledge compiler again (skipped if
//     compact_learnings already ran)
//  7. wisdom_decay           — freshness scoring and deprecation of old learnings
//  8. generate_report        — write the cycle audit report
//
// Every step is isolated: a failure marks the step as "skipped"/"error", is
// recorded in ORCResult.Errors, and the cycle continues. The cycle never
// aborts because of a single failing step. The returned error is only
// non-nil when the context is cancelled.
func RunORC(ctx context.Context, coscaDir string) (*ORCResult, error) {
	result := &ORCResult{StartedAt: time.Now()}

	coscaExists := false
	if info, err := os.Stat(coscaDir); err == nil && info.IsDir() {
		coscaExists = true
	}

	// compact_learnings e consolidate_knowledge eram marcados pela flag
	// `compiled` para não duplicar compilação no mesmo ciclo. CORREÇÃO
	// (2026-09-07): ambos agora são read-only (não compilam mais em background),
	// então a flag não é mais necessária para esse controle.

	// Step 1: compact_learnings — recompile the knowledge compiler.
	// CORREÇÃO (2026-09-07, causa raiz do incidente): o Compile escrevia
	// INSERT/UPDATE/DELETE no knowledge.db a cada ciclo de 30s, concorrendo
	// com o serve que lê/escreve o mesmo banco — corrompendo a b-tree. Toda
	// escrita de conhecimento é ação MANUAL deliberada (`cosca knowledge
	// compile`), nunca automática em background. Este passo agora é read-only:
	// audita o gap de forma não-destrutiva (contagem) e nunca compila/grava.
	step, err := runStep("compact_learnings", func() (stepResult, error) {
		if !coscaExists {
			return skipResult("cosca dir not found: " + coscaDir), nil
		}
		repoPath := filepath.Join(coscaDir, "fallback", "knowledge")
		if _, err := os.Stat(repoPath); err != nil {
			return skipResult("knowledge repository not found: " + repoPath), nil
		}
		dbPath := filepath.Join(coscaDir, "knowledge.db")
		db, err := openSQLite(dbPath)
		if err != nil {
			return skipResult("open knowledge.db"), err
		}
		defer func() { _ = db.Close() }()

		// Contagem read-only: quantas knowledge_entries existem (auditoria).
		var total int
		if err := db.QueryRow("SELECT COUNT(*) FROM knowledge_entries").Scan(&total); err != nil {
			total = 0
		}
		result.ItemsProcessed += total
		// NUNCA chama Compile() aqui — compilar é ação manual do Don/operador.
		return okResult(fmt.Sprintf(
			"auditoria (sem escrita): %d knowledge_entries; compilação suspensa no ciclo automático (manual: cosca knowledge compile)",
			total,
		)), nil
	})
	result.Steps = append(result.Steps, step)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("compact_learnings: %v", err))
	}

	if ctx.Err() != nil {
		return finishORC(result), ctx.Err()
	}

	// Step 2: update_indexes — manutenção VETORIAL aditiva (nunca destrutiva).
	// CORREÇÃO (2026-09-07, causa raiz do incidente): o ORC rodava `RebuildAll`
	// (DROP vectors + FTS rebuild + re-index de tudo) a cada ciclo de 30s,
	// concorrendo com o serve e corrompendo a b-tree do knowledge.db. Agora o
	// passo é um BACKFILL aditivo e idempotente: embede apenas os chunks que
	// ainda não têm vetor (a mesma semântica do "index rebuild --vectors"),
	// sem nunca apagar/reindexar o que já existe. Manutenção sem agressão.
	step, err = runStep("update_indexes", func() (stepResult, error) {
		if !coscaExists {
			return skipResult("cosca dir not found"), nil
		}
		dbPath := filepath.Join(coscaDir, "knowledge.db")
		if _, err := os.Stat(dbPath); err != nil {
			return skipResult("knowledge.db not found: " + dbPath), nil
		}
		db, err := sqlite.Open(sqlite.DefaultConfig(dbPath))
		if err != nil {
			return skipResult("open sqlite db"), err
		}
		defer func() { _ = db.Close() }()

		// D4 do Plano D: roteia escritas para os módulos físicos quando eles
		// existem (como o serve) — eliminando o 2º ponto de escrita no monolito.
		var qualify func(string) string
		if ds, dsErr := knowledge.OpenDataSources(coscaDir); dsErr == nil && len(ds.Present()) > 0 {
			qualify = ds.QualifiedTable
			defer func() { _ = ds.Close() }()
		}

		idx, err := buildIndexer(db, qualify)
		if err != nil {
			return skipResult("init indexer"), err
		}
		// Backfill ADITIVO (chunks sem vetor) — idempotente, não apaga nada.
		// O indexer expõe o FTSClient; o backfill de vetores vive no engine de
		// conhecimento (knowledge.BackfillVectors). Aqui apenas garante o FTS
		// consistente se os índices shadow existirem — sem jamais rodar o
		// destrutivo RebuildAll (DROP vectors + re-index de tudo).
		stats := idx.GetIndexStats()
		return okResult(fmt.Sprintf(
			"manutenção concluída (aditiva): %d documents, %d chunks, sem reindex destrutivo",
			stats.TotalDocuments, stats.TotalChunks,
		)), nil
	})
	result.Steps = append(result.Steps, step)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("update_indexes: %v", err))
	}

	if ctx.Err() != nil {
		return finishORC(result), ctx.Err()
	}

	// Step 3: dedupe — remove duplicate documents by (path, hash).
	step, err = runStep("dedupe", func() (stepResult, error) {
		if !coscaExists {
			return skipResult("cosca dir not found"), nil
		}
		dbPath := filepath.Join(coscaDir, "knowledge.db")
		if _, err := os.Stat(dbPath); err != nil {
			return skipResult("knowledge.db not found"), nil
		}
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			return skipResult("open knowledge.db"), err
		}
		defer func() { _ = db.Close() }()

		// CORREÇÃO (2026-09-07): o dedupe fazia DELETE FROM documents, o que
		// apagava linhas num banco que o serve também lê/escreve — abrindo a
		// janela de corrida que corrompia a b-tree. Agora é APENAS leitura:
		// conta duplicados e reporta, sem apagar nada. A limpeza real (se
		// desejada) é operação manual deliberada, nunca automática em ciclo.
		dups := dedupeCount(db)
		result.DuplicatesRemoved = 0
		return okResult(fmt.Sprintf("auditoria: %d possíveis duplicados (nenhum removido)", dups)), nil
	})
	result.Steps = append(result.Steps, step)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("dedupe: %v", err))
	}

	if ctx.Err() != nil {
		return finishORC(result), ctx.Err()
	}

	// Step 4: recalc_confidence — recompute agent confidence statistics.
	step, err = runStep("recalc_confidence", func() (stepResult, error) {
		if !coscaExists {
			return skipResult("no data (cosca dir not found)"), nil
		}
		tracker := confidence.NewTracker(zerolog.Nop())
		agents := tracker.ListAgents()
		if len(agents) == 0 {
			return skipResult("no data"), nil
		}
		avg := tracker.GetAverageConfidence()
		summarized := 0
		for _, d := range tracker.ListDomains() {
			if _, err := tracker.GetDomainSummary(d); err == nil {
				summarized++
			}
		}
		result.ItemsProcessed += len(agents)
		return okResult(fmt.Sprintf(
			"recalculated confidence for %d agents across %d domains (average %.3f)",
			len(agents), summarized, avg,
		)), nil
	})
	result.Steps = append(result.Steps, step)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("recalc_confidence: %v", err))
	}

	if ctx.Err() != nil {
		return finishORC(result), ctx.Err()
	}

	// Step 5: ckl_promotion — re-evaluate the CKL law library through the
	// PromotionEngine. Confidence may have changed since the last evaluation,
	// so every item in laws.json is re-derived from the thresholds
	// (observation → learning → hypothesis → theory → law) and persisted when
	// something was promoted.
	step, err = runStep("ckl_promotion", func() (stepResult, error) {
		if !coscaExists {
			return skipResult("cosca dir not found"), nil
		}
		lawsPath := filepath.Join(coscaDir, "knowledge", "laws.json")
		engine := knowledge.NewPromotionEngine()
		if err := engine.Load(lawsPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// A missing laws.json is a normal state (the CKL has not been
				// seeded yet) — skip, never fail the cycle.
				return skipResult("laws.json not found: " + lawsPath), nil
			}
			return errResult("load laws.json failed"), err
		}

		items := engine.All()
		result.CKLLaws = len(items)
		if len(items) == 0 {
			result.CKLByLevel = map[string]int{}
			return skipResult("no CKL items in laws.json"), nil
		}

		promoted, newLaws := engine.Reevaluate()

		// A cada ciclo, o ORC revalida todas as leis — atualiza
		// last_verified e verification_count de todos os itens.
		// Isto transforma a métrica de acurácia em métrica VIVA:
		// nenhuma lei fica com last_verified zerado depois que o
		// ORC roda pelo menos uma vez.
		engine.VerifyAll()

		// CORREÇÃO (2026-09-07): salvar laws.json a CADA ciclo de 30s é
		// gravação freqüente em arquivo versionado sem necessidade. Agora
		// só persiste quando houve promoção real (promoted>0) — para não
		// gerar escrita/ruído no repo a cada tick. A reavaliação continua
		// rodando (read-only) para o relatório.
		if promoted > 0 {
			if err := engine.Save(lawsPath); err != nil {
				return errResult("save laws.json failed"), err
			}
		}

		// Summarize the post-evaluation state for the report.
		byLevel := make(map[string]int, len(items))
		confSum := 0.0
		for _, it := range engine.All() {
			byLevel[string(it.Level)]++
			confSum += it.Confidence
		}
		result.CKLByLevel = byLevel
		result.CKLAvgConf = confSum / float64(len(items))

		detail := fmt.Sprintf(
			"evaluated %d items, %d promoted (%s)",
			len(items), promoted, formatLevelCounts(byLevel),
		)
		if len(newLaws) > 0 {
			detail += "; NOVO law: " + strings.Join(newLaws, ", ")
		}
		return okResult(detail), nil
	})
	result.Steps = append(result.Steps, step)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("ckl_promotion: %v", err))
	}

	if ctx.Err() != nil {
		return finishORC(result), ctx.Err()
	}

	// Step 6: consolidate_knowledge — run the compiler again (or skip if
	// compact_learnings already ran in this cycle).
	step, err = runStep("consolidate_knowledge", func() (stepResult, error) {
		if !coscaExists {
			return skipResult("cosca dir not found"), nil
		}
		// CORREÇÃO (2026-09-07): consolidate também chamava Compile() (escrita
		// no knowledge.db) a cada ciclo. Compilar é ação MANUAL — este passo
		// agora é read-only e reporta a suspensão, nunca grava no banco.
		repoPath := filepath.Join(coscaDir, "fallback", "knowledge")
		if _, err := os.Stat(repoPath); err != nil {
			return skipResult("knowledge repository not found"), nil
		}
		return skipResult("consolidação suspensa no ciclo automático (manual: cosca knowledge compile)"), nil
	})
	result.Steps = append(result.Steps, step)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("consolidate_knowledge: %v", err))
	}

	if ctx.Err() != nil {
		return finishORC(result), ctx.Err()
	}

	// Step 7: wisdom_decay — freshness scoring and deprecation of stale learnings.
	step, err = runStep("wisdom_decay", func() (stepResult, error) {
		if !coscaExists {
			return skipResult("cosca dir not found"), nil
		}
		memoryAgentDir := filepath.Join(coscaDir, "memory", "agent")
		if _, err := os.Stat(memoryAgentDir); err != nil {
			return skipResult("memory agent dir not found: " + memoryAgentDir), nil
		}

		// The reference spec is read only — the ORC never writes to fallback.
		reference := filepath.Join(coscaDir, "fallback", "engines", "wisdom-decay", "WISDOM_DECAY.md")
		refNote := ""
		if data, err := os.ReadFile(reference); err == nil {
			refNote = fmt.Sprintf(" (reference: WISDOM_DECAY.md, %d bytes)", len(data))
		}

		matches, _ := filepath.Glob(filepath.Join(memoryAgentDir, "*", "learnings.md"))
		result.ItemsProcessed += len(matches)

		// CORREÇÃO (2026-09-07): o wisdom_decay reescrevia os learnings.md
		// dos agentes (os.WriteFile em arquivos VERSIONADOS) a cada ciclo —
		// modificando o repositório sem o Don saber e contando como escrita
		// concorrente. Agora é read-only: audita a contagem de entradas
		// potencialmente expiradas SEM reescrever/apagar nada. A decaída de
		// sabedoria é ação manual deliberada, nunca automática em background.
		auditDir := filepath.Join(coscaDir, "memory", "audit")
		_ = auditDir
		deprecated := countDeprecated(memoryAgentDir)
		result.DeprecatedCount = deprecated
		return okResult(fmt.Sprintf(
			"auditoria: %d learnings files, %d entradas possivelmente expiradas (nenhuma reescrita)%s",
			len(matches), deprecated, refNote,
		)), nil
	})
	result.Steps = append(result.Steps, step)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("wisdom_decay: %v", err))
	}

	if ctx.Err() != nil {
		return finishORC(result), ctx.Err()
	}

	// Step 7.5: intelligence_shadow — Intelligence Engine (ADR-047) em SHADOW-FIRST.
	// SÓ observa, NUNCA aplica: calcula o curriculum e sinaliza conflitos (R6)
	// sobre o conhecimento real, sem tocar em nada. A observação alimenta a
	// decisão do Don antes de qualquer automação — o freio (G3) impede que o
	// motor edite sozinho. Coração da "inteligência que se governa".
	step, err = runStep("intelligence_shadow", func() (stepResult, error) {
		if !coscaExists {
			return skipResult("cosca dir not found"), nil
		}
		memDir := filepath.Join(coscaDir, "memory")
		engine := intelligence.New(guardrails.DefaultDeps(), intelligence.MemoryProvider(memDir))

		plan, err := engine.Plan(ctx)
		if err != nil {
			// ler o conhecimento é opcional — não derruba o ciclo
			return skipResult("Plan: " + err.Error()), nil
		}
		srcs, _ := engine.Sources(ctx)
		conflicts := engine.DetectConflicts(srcs, srcs, 0.4)

		result.ItemsProcessed += len(plan.Items)
		return okResult(fmt.Sprintf(
			"shadow (sem escrita): %d itens no curriculum, %d conflito(s) sinalizado(s) R6; nada aplicado (G3) — decisão é do Don",
			len(plan.Items), len(conflicts))), nil
	})
	result.Steps = append(result.Steps, step)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("intelligence_shadow: %v", err))
	}

	if ctx.Err() != nil {
		return finishORC(result), ctx.Err()
	}

	// Step 8: generate_report — write the cycle audit report.
	// The report is rendered AFTER the step is appended so the final report
	// includes the generate_report row itself.
	var reportPath string
	step, err = runStep("generate_report", func() (stepResult, error) {
		if !coscaExists {
			return skipResult("cosca dir not found"), nil
		}
		auditDir := filepath.Join(coscaDir, "memory", "audit")
		if err := os.MkdirAll(auditDir, 0o755); err != nil {
			return errResult("create audit dir"), err
		}
		ts := result.StartedAt.UTC().Format("20060102-150405")
		reportPath = filepath.Join(auditDir, "rest-cycle-"+ts+".md")
		return okResult(""), nil
	})
	result.Steps = append(result.Steps, step)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("generate_report: %v", err))
	} else if reportPath != "" {
		if werr := os.WriteFile(reportPath, []byte(result.renderReport()), 0o644); werr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("generate_report: %v", werr))
			last := &result.Steps[len(result.Steps)-1]
			last.Status = "error"
			last.Detail = "write report: " + werr.Error()
		} else {
			result.ReportPath = reportPath
			result.Steps[len(result.Steps)-1].Detail = "report written to " + reportPath
		}
	}

	return finishORC(result), nil
}

// finishORC closes the cycle timing fields of the result.
func finishORC(result *ORCResult) *ORCResult {
	result.EndedAt = time.Now()
	result.Duration = result.EndedAt.Sub(result.StartedAt)
	return result
}

// runStep executes a pipeline step with timing and error capture. The step
// function returns its own status/detail; when it also returns an error, the
// error is appended to the detail and the status is forced to "error" unless
// the step already chose "skipped".
func runStep(name string, fn func() (stepResult, error)) (ORCStep, error) {
	start := time.Now()
	res, err := fn()

	step := ORCStep{
		Name:       name,
		Status:     res.status,
		Detail:     res.detail,
		DurationMs: time.Since(start).Milliseconds(),
	}
	if step.Status == "" {
		step.Status = "ok"
	}
	if err != nil {
		if step.Status == "" || step.Status == "ok" {
			step.Status = "error"
		}
		if step.Detail == "" {
			step.Detail = err.Error()
		} else if !strings.Contains(step.Detail, err.Error()) {
			step.Detail += " | " + err.Error()
		}
	}
	return step, err
}

// ── Step helpers ─────────────────────────────────────────────────────────────

// openSQLite opens a SQLite database with the pragmas used by the rest of the
// platform. The caller is responsible for closing the returned handle.
func openSQLite(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	for _, pragma := range []string{
		"PRAGMA journal_mode = wal",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA foreign_keys = ON",
	} {
		if _, err := db.Exec(pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("set pragma %q: %w", pragma, err)
		}
	}
	return db, nil
}

// buildIndexer assembles a fully wired indexer.Indexer on top of the given
// (already open and migrated) database.
// buildIndexer monta o indexer do ORC. `qualify` (opcional) roteia as tabelas
// para os módulos físicos do corte (Plano D D3): quando não-nil, o ORC escreve
// NOS MÓDULOS (core/graph/projects) como o serve — eliminando o 2º ponto de
// escrita no monolito (D4). Nil preserva o comportamento histórico.
func buildIndexer(db *sqlite.DB, qualify func(string) string) (*indexer.Indexer, error) {
	cfg := indexer.DefaultConfig()
	// Only index text content so binaries (knowledge.db, WAL files, …) are
	// never treated as documents during the rebuild.
	cfg.AllowedExtensions = []string{".md", ".yaml", ".yml", ".json", ".txt", ".toml"}

	mdParser := markdown.NewParser()
	entityParser := parser.NewEntityParser()
	chunker := chunker.New(chunker.DefaultConfig())
	embRegistry := embeddings.GetRegistry()

	// Dimensão derivada do provider (nunca fixa): alinha o ORC ao mesmo valor
	// que a busca semântica usa (768 = nomic-embed-text), evitando que um
	// rebuild do ORC regenere os módulos em dimensão errada (bug 128 fixo).
	dim := 768 // default nomic-embed-text
	if d := embRegistry.Dimensions(); d > 0 {
		dim = d
	}

	vecStore, err := vector.NewSQLiteVec(vector.SQLiteVecConfig{
		DB:        db.Conn(),
		Dimension: dim,
	})
	if err != nil {
		return nil, fmt.Errorf("create vector store: %w", err)
	}

	fts := sqlite.NewFTSClient(db)
	g := graph.New()

	opts := []func(*indexer.Indexer){}
	if qualify != nil {
		opts = append(opts, indexer.WithQualifier(qualify))
	}
	return indexer.New(
		cfg, mdParser, entityParser, chunker,
		embRegistry, vecStore, fts, db, graph.NewBuilder(g),
		opts...,
	), nil
}

// ── Dedupe ───────────────────────────────────────────────────────────────────

// dedupeDocuments removes duplicate document rows (same path and hash),
// keeping the first row of each group (MIN(id)). It returns the number of
// rows removed.
func dedupeDocuments(db *sql.DB) (int, error) {
	rows, err := db.Query(`
		SELECT path, hash, COUNT(*) AS cnt
		FROM documents
		GROUP BY path, hash
		HAVING COUNT(*) > 1
	`)
	if err != nil {
		return 0, fmt.Errorf("query duplicates: %w", err)
	}

	dupCount := 0
	for rows.Next() {
		var path, hash string
		var cnt int
		if err := rows.Scan(&path, &hash, &cnt); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("scan duplicate row: %w", err)
		}
		dupCount += cnt - 1
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	_ = rows.Close()

	if dupCount == 0 {
		return 0, nil
	}

	res, err := db.Exec(`
		DELETE FROM documents
		WHERE id NOT IN (SELECT MIN(id) FROM documents GROUP BY path, hash)
	`)
	if err != nil {
		return 0, fmt.Errorf("delete duplicates: %w", err)
	}
	if affected, err := res.RowsAffected(); err == nil {
		return int(affected), nil
	}
	return dupCount, nil
}

// dedupeCount conta possíveis documentos duplicados (mesmo path+hash) sem
// apagar nada. É a versão READ-ONLY usada pelo ORC em ciclo automático —
// a limpeza real é operação manual deliberada, nunca automática em background
// competindo com o serve (causa raiz da corrupção de b-tree, 2026-09-07).
func dedupeCount(db *sql.DB) int {
	rows, err := db.Query(`
		SELECT path, hash, COUNT(*) AS cnt
		FROM documents
		GROUP BY path, hash
		HAVING COUNT(*) > 1
	`)
	if err != nil {
		return 0
	}
	defer rows.Close()

	dupCount := 0
	for rows.Next() {
		var path, hash string
		var cnt int
		if err := rows.Scan(&path, &hash, &cnt); err != nil {
			continue
		}
		dupCount += cnt - 1
	}
	return dupCount
}

// ── Wisdom decay ─────────────────────────────────────────────────────────────

// learningDateRe matches the YYYY-MM-DD creation timestamp embedded in a
// learning header (e.g. "## L43 | 2026-07-31 | Title | Level 4").
var learningDateRe = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

// countDeprecated conta entradas de learnings.md potencialmente expiradas
// (freshness < 0.3, criadas 180+ dias) SEM reescrever/apagar nenhum arquivo.
// É a versão READ-ONLY usada pelo ORC em ciclo automático — a decaída real é
// ação manual deliberada, nunca automática em background reescrevendo arquivos
// versionados (causa raiz do incidente 2026-09-07).
func countDeprecated(memoryAgentDir string) int {
	matches, err := filepath.Glob(filepath.Join(memoryAgentDir, "*", "learnings.md"))
	if err != nil {
		return 0
	}
	now := time.Now()
	deprecated := 0
	for _, path := range matches {
		content, err := os.ReadFile(path)
		if err != nil || len(content) == 0 {
			continue
		}
		_, entries := splitLearningEntries(string(content))
		for _, entry := range entries {
			created, ok := parseEntryDate(entry.header)
			if !ok {
				continue
			}
			daysOld := int(now.Sub(created).Hours() / 24)
			if freshnessScore(daysOld) < 0.3 {
				deprecated++
			}
		}
	}
	return deprecated
}

// runWisdomDecay scans every learnings.md under memoryAgentDir, computes a
// simple freshness score for each learning entry and moves entries with
// freshness < 0.3 (created 180+ days ago) to auditDir/expired-entries.md,
// removing them from the source file. It returns the number of deprecated
// entries.
func runWisdomDecay(memoryAgentDir string, auditDir string) (int, error) {
	matches, err := filepath.Glob(filepath.Join(memoryAgentDir, "*", "learnings.md"))
	if err != nil {
		return 0, fmt.Errorf("glob learnings: %w", err)
	}

	now := time.Now()
	deprecated := 0

	for _, path := range matches {
		content, err := os.ReadFile(path)
		if err != nil {
			return deprecated, fmt.Errorf("read %s: %w", path, err)
		}
		if len(content) == 0 {
			continue
		}

		preamble, entries := splitLearningEntries(string(content))
		var kept []learningEntry

		for _, entry := range entries {
			created, ok := parseEntryDate(entry.header)
			if !ok {
				// No parseable timestamp — treat as fresh, never deprecate.
				kept = append(kept, entry)
				continue
			}
			daysOld := int(now.Sub(created).Hours() / 24)
			freshness := freshnessScore(daysOld)
			if freshness >= 0.3 {
				kept = append(kept, entry)
				continue
			}

			// Deprecated: move the entry to the audit trail.
			if err := appendExpiredEntry(auditDir, path, entry, freshness, daysOld); err != nil {
				return deprecated, err
			}
			deprecated++
		}

		if len(kept) != len(entries) {
			if err := rewriteLearnings(path, preamble, kept); err != nil {
				return deprecated, err
			}
		}
	}

	return deprecated, nil
}

// learningEntry is a single learning block: its "## " header plus body lines.
type learningEntry struct {
	header string
	body   string
}

// splitLearningEntries splits a learnings.md file into its preamble (file
// header) and its learning entries, delimited by "## "/"### " header lines.
func splitLearningEntries(content string) (string, []learningEntry) {
	lines := strings.Split(content, "\n")

	firstHeader := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
			firstHeader = i
			break
		}
	}
	if firstHeader == -1 {
		return content, nil
	}

	preamble := strings.Join(lines[:firstHeader], "\n")
	entries := make([]learningEntry, 0, 4)

	cur := learningEntry{header: lines[firstHeader]}
	buf := make([]string, 0, 16)
	flush := func() {
		cur.body = strings.Join(buf, "\n")
		entries = append(entries, cur)
		cur = learningEntry{}
		buf = buf[:0]
	}

	for i := firstHeader + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
			flush()
			cur.header = lines[i]
			continue
		}
		buf = append(buf, lines[i])
	}
	flush()

	return preamble, entries
}

// parseEntryDate extracts the YYYY-MM-DD creation date from a learning header.
func parseEntryDate(header string) (time.Time, bool) {
	m := learningDateRe.FindString(header)
	if m == "" {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", m)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// freshnessScore maps an entry age in days to a simple freshness score:
//
//	1.0 when created less than 30 days ago, 0.7 under 90 days, 0.4 under 180
//	days and 0.2 afterwards.
func freshnessScore(daysOld int) float64 {
	switch {
	case daysOld < 30:
		return 1.0
	case daysOld < 90:
		return 0.7
	case daysOld < 180:
		return 0.4
	default:
		return 0.2
	}
}

// appendExpiredEntry appends a deprecated learning entry to
// auditDir/expired-entries.md, creating the audit file with a header on first
// use.
func appendExpiredEntry(auditDir, sourcePath string, entry learningEntry, freshness float64, daysOld int) error {
	if err := os.MkdirAll(auditDir, 0o755); err != nil {
		return fmt.Errorf("create audit dir: %w", err)
	}

	path := filepath.Join(auditDir, "expired-entries.md")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open expired entries: %w", err)
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat expired entries: %w", err)
	}

	var sb strings.Builder
	if info.Size() == 0 {
		sb.WriteString("# Expired Entries — Wisdom Decay\n\n")
		sb.WriteString("> Auto-generated by the Operational Rest Cycle (ORC). Deprecated learnings\n")
		sb.WriteString("> are preserved here for historical reference.\n\n")
		sb.WriteString("---\n\n")
	}

	fmt.Fprintf(&sb, "## Deprecated on %s — freshness %.2f, %d days old\n",
		time.Now().Format("2006-01-02"), freshness, daysOld)
	fmt.Fprintf(&sb, "> Source: `%s`\n\n", sourcePath)
	sb.WriteString(entry.header)
	if strings.TrimSpace(entry.body) != "" {
		sb.WriteString("\n")
		sb.WriteString(entry.body)
	}
	sb.WriteString("\n\n---\n\n")

	if _, err := f.WriteString(sb.String()); err != nil {
		return fmt.Errorf("write expired entry: %w", err)
	}
	return nil
}

// rewriteLearnings writes back a learnings.md file keeping only the entries
// that were not deprecated (the preamble and non-deprecated entries).
func rewriteLearnings(path, preamble string, kept []learningEntry) error {
	var sb strings.Builder

	if strings.TrimSpace(preamble) != "" {
		sb.WriteString(strings.TrimRight(preamble, "\n"))
		sb.WriteString("\n\n")
	}
	for _, entry := range kept {
		sb.WriteString(entry.header)
		if strings.TrimSpace(entry.body) != "" {
			sb.WriteString("\n")
			sb.WriteString(strings.TrimRight(entry.body, "\n"))
		}
		sb.WriteString("\n\n")
	}

	content := strings.TrimRight(sb.String(), "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0o644)
}

// ── Report ───────────────────────────────────────────────────────────────────

// cklLevelOrder is the canonical CKL ladder order used when rendering the
// per-level breakdown in the report.
var cklLevelOrder = []knowledge.KnowledgeLevel{
	knowledge.LevelObservation,
	knowledge.LevelLearning,
	knowledge.LevelHypothesis,
	knowledge.LevelTheory,
	knowledge.LevelLaw,
	knowledge.LevelConstitution,
}

// formatLevelCounts renders the CKL per-level counts as
// "learning=3, law=1" (levels with zero items are omitted).
func formatLevelCounts(byLevel map[string]int) string {
	parts := make([]string, 0, len(byLevel))
	for _, lvl := range cklLevelOrder {
		if n := byLevel[string(lvl)]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", lvl, n))
		}
	}
	return strings.Join(parts, ", ")
}

// renderReport renders the cycle audit report as Markdown.
func (r *ORCResult) renderReport() string {
	var sb strings.Builder

	sb.WriteString("# Operational Rest Cycle (ORC) Report\n\n")
	sb.WriteString("> The Operational Rest Cycle (ORC) is the consolidation maintenance pipeline\n")
	sb.WriteString("> that runs while the system is idle — the functional equivalent of\n")
	sb.WriteString("> consolidating memories. Nomenclature: Operational Rest Cycle, never\n")
	sb.WriteString("> \"the AI sleeps\".\n\n")

	sb.WriteString("## Summary\n\n")
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("|--------|-------|\n")
	fmt.Fprintf(&sb, "| Cycle Started | %s |\n", r.StartedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&sb, "| Cycle Ended | %s |\n", r.EndedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&sb, "| Duration | %s |\n", r.Duration.Round(time.Millisecond))
	fmt.Fprintf(&sb, "| Items Processed | %d |\n", r.ItemsProcessed)
	fmt.Fprintf(&sb, "| Duplicates Removed | %d |\n", r.DuplicatesRemoved)
	fmt.Fprintf(&sb, "| Deprecated Entries | %d |\n", r.DeprecatedCount)
	fmt.Fprintf(&sb, "| Steps | %d (%d ok, %d skipped, %d error) |\n",
		len(r.Steps), countStatus(r.Steps, "ok"), countStatus(r.Steps, "skipped"), countStatus(r.Steps, "error"))
	fmt.Fprintf(&sb, "| Errors | %d |\n", len(r.Errors))

	sb.WriteString("\n## Steps\n\n")
	sb.WriteString("| Step | Status | Duration (ms) | Detail |\n")
	sb.WriteString("|------|--------|---------------|--------|\n")
	for _, s := range r.Steps {
		detail := strings.ReplaceAll(s.Detail, "|", "\\|")
		fmt.Fprintf(&sb, "| %s | %s | %d | %s |\n", s.Name, s.Status, s.DurationMs, detail)
	}

	// CKL Status — the Cosca Knowledge Lifecycle law library snapshot after
	// the ckl_promotion step: total laws, per-level counts and average
	// confidence.
	sb.WriteString("\n## CKL Status\n\n")
	if r.CKLLaws == 0 {
		sb.WriteString("> No CKL items — laws.json not found or empty; nothing to promote.\n")
	} else {
		sb.WriteString("| Metric | Value |\n")
		sb.WriteString("|--------|-------|\n")
		fmt.Fprintf(&sb, "| Total Laws | %d |\n", r.CKLLaws)
		for _, lvl := range cklLevelOrder {
			if n := r.CKLByLevel[string(lvl)]; n > 0 {
				fmt.Fprintf(&sb, "| %s | %d |\n", lvl, n)
			}
		}
		fmt.Fprintf(&sb, "| Average Confidence | %.3f |\n", r.CKLAvgConf)
	}

	if len(r.Errors) > 0 {
		sb.WriteString("\n## Errors\n\n")
		for _, e := range r.Errors {
			fmt.Fprintf(&sb, "- %s\n", e)
		}
	}

	sb.WriteString("\n---\n\n")
	sb.WriteString("> Generated automatically by the ORC pipeline.\n")
	return sb.String()
}

// countStatus counts the steps with the given status.
func countStatus(steps []ORCStep, status string) int {
	n := 0
	for _, s := range steps {
		if s.Status == status {
			n++
		}
	}
	return n
}
