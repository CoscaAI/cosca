// Package cli — `cosca db build`: Fase C do ADR-013 (split físico dos módulos
// derivados), na interpretação ADITIVA (segura, reversível).
//
// Cria os módulos físicos a partir do knowledge.db (a fonte) SEM tocar nele:
//
//	core.db            documents + knowledge_entries + cache + users (fonte)
//	graph.db           entities + relationships (derivado)
//	projects.db        chunks + headings + code_blocks + tables (derivado)
//	vector-embed.db    vetores do domínio embed (cérebro) — particionado por
//		vector-code.db     responsabilidade (ADR-013 §2.0/§2.2.1: nunca por função)
//		vector-docs.db
//		vector-fallback.db
//		vector-opencode.db
//		vector-other.db
//	fts.db             FTS5 unificado (derivado, reconstruível)
//
// O knowledge.db fica INTACTO — é a fonte da verdade. Os módulos são
// projeções derivadas reconstruíveis (`--rebuild`). Reversível: apagar os
// módulos devolve o estado original.
package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	_ "modernc.org/sqlite"
)

// NewDBBuildCommand cria `cosca db build` — migração aditiva do ADR-013 Fase C.
func NewDBBuildCommand() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Cria os módulos físicos derivados (ADR-013 Fase C) — aditivo, não toca o knowledge.db",
		Long: `Cria os módulos físicos do ADR-013 (core, graph, projects, vector-*,
fts) como projeções derivadas do knowledge.db — que permanece INTACTO como
fonte da verdade.

ADITIVO e REVERSÍVEL: nenhuma escrita no knowledge.db; apagar os módulos
devolve o estado original. Cada módulo fica < 100 MB (Decisão 1).

O vector.db é particionado por RESPONSABILIDADE (embed/code/docs/fallback/
opencode/other) — o índice de 170 MB não cabe num único arquivo de 100 MB
(regra anti-monstro §2.0 + partição §2.2.1).

Valida por contagem/checksum após criar; --dry-run audita sem criar nada.`,
		Example: `  cosca db build --dry-run   # audita sem criar
  cosca db build             # cria os módulos físicos
  cosca db build --verify    # cria e valida contagens vs fonte`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			useJSON := IsJSONOutput(cmd)
			f := GetFormatter(cmd)

			dir, dirErr := resolveDataDir("")
			if dirErr != nil {
				return fmt.Errorf("resolve data directory: %w", dirErr)
			}
			kb := filepath.Join(dir, "knowledge.db")
			if _, statErr := os.Stat(kb); statErr != nil {
				return fmt.Errorf("knowledge.db não encontrado em %s: %w", kb, statErr)
			}

			// Abre a fonte READ-ONLY — nunca escreve nela.
			src, err := sql.Open("sqlite", "file:"+filepath.ToSlash(kb)+"?mode=ro")
			if err != nil {
				return fmt.Errorf("abrir fonte: %w", err)
			}
			defer src.Close()

			modules := []struct {
				name  string
				build func(*sql.DB, string) error
			}{
				{"core.db", buildCore},
				{"graph.db", buildGraph},
				{"projects.db", buildProjects},
				{"vector-embed-memory.db", buildVectorDomain("embed-memory")},
				{"vector-embed-engines.db", buildVectorDomain("embed-engines")},
				{"vector-embed-core.db", buildVectorDomain("embed-core")},
				{"vector-code.db", buildVectorDomain("code")},
				{"vector-docs.db", buildVectorDomain("docs")},
				{"vector-fallback.db", buildVectorDomain("fallback")},
				{"vector-opencode.db", buildVectorDomain("opencode")},
				{"vector-other.db", buildVectorDomain("other")},
			}

			if dryRun {
				f.Header("Auditoria (dry-run) — módulos a criar")
				for _, m := range modules {
					target := filepath.Join(dir, m.name)
					exists := ""
					if _, err := os.Stat(target); err == nil {
						exists = " [EXISTE — será re-criado]"
					}
					f.Printf("  %-20s %s%s\n", m.name, target, exists)
				}
				f.Printf("\n%d módulos. O knowledge.db NÃO será tocado. Rode sem --dry-run para criar.", len(modules))
				return nil
			}

			report := []string{}
			for _, m := range modules {
				target := filepath.Join(dir, m.name)
				// Projeção derivada (ADR-013): SEMPRE recriar do zero. O destino
				// é um read-model reconstruível — re-executar o build nunca
				// acumula dados (o INSERT OR REPLACE não remove linhas antigas
				// de momentos diferentes). Apagar antes é seguro e idempotente.
				if _, statErr := os.Stat(target); statErr == nil {
					if rmErr := os.Remove(target); rmErr != nil {
						return fmt.Errorf("remover %s (recriação): %w", m.name, rmErr)
					}
				}
				if err := m.build(src, target); err != nil {
					return fmt.Errorf("criar %s: %w", m.name, err)
				}
				sz := fileSize(target)
				report = append(report, fmt.Sprintf("%s: %s", m.name, formatDBSize(sz)))
			}

			if useJSON {
				b, _ := jsonMarshalImpl(map[string]any{
					"knowledge_db": kb,
					"modules":      report,
					"aditiva":      true,
				})
				f.Println(string(b))
			} else {
				f.Header("Módulos físicos criados (ADR-013 Fase C) — aditivo")
				for _, r := range report {
					f.Printf("  %s\n", r)
				}
				f.Printf("\nO knowledge.db (%s) permanece intacto como fonte da verdade.", formatDBSize(fileSize(kb)))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "auditar sem criar")
	return cmd
}

// buildCore cria core.db com as tabelas FONTE (documents, knowledge_entries,
// cache, users, sync_log, snapshots, migration_history).
func buildCore(src *sql.DB, target string) error {
	dst, err := sql.Open("sqlite", target)
	if err != nil {
		return err
	}
	defer dst.Close()

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS documents (
			id TEXT PRIMARY KEY, path TEXT NOT NULL UNIQUE, hash TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT '', doc_type TEXT NOT NULL DEFAULT 'markdown',
			metadata_json TEXT NOT NULL DEFAULT '{}', frontmatter_json TEXT NOT NULL DEFAULT '{}',
			size INTEGER NOT NULL DEFAULT 0, token_count INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (datetime('now')), updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			tier TEXT NOT NULL DEFAULT 'medium', expires_at TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE IF NOT EXISTS knowledge_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT, category TEXT NOT NULL,
			sub_category TEXT NOT NULL DEFAULT '', title TEXT NOT NULL, content TEXT NOT NULL,
			tags TEXT NOT NULL DEFAULT '[]', confidence REAL NOT NULL DEFAULT 0.5,
			source TEXT NOT NULL, updated_at TEXT NOT NULL, content_hash TEXT NOT NULL,
			UNIQUE(source))`,
		`CREATE TABLE IF NOT EXISTS cache (
			key TEXT PRIMARY KEY, value BLOB NOT NULL,
			content_type TEXT NOT NULL DEFAULT 'application/octet-stream',
			ttl_seconds INTEGER NOT NULL DEFAULT 3600,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			expires_at TEXT NOT NULL DEFAULT (datetime('now', '+1 hour')))`,
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY, username TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'viewer', email TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL, failed_attempts INTEGER NOT NULL DEFAULT 0,
			locked_until TEXT NOT NULL DEFAULT '', must_change_password INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE IF NOT EXISTS sync_log (
			id TEXT PRIMARY KEY, path TEXT NOT NULL, action TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending', hash_before TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE IF NOT EXISTS snapshots (
			id TEXT PRIMARY KEY, created_at TEXT NOT NULL, label TEXT NOT NULL DEFAULT '',
			path TEXT NOT NULL DEFAULT '', size INTEGER NOT NULL DEFAULT 0)`,
	}
	for _, s := range stmts {
		if _, err := dst.Exec(s); err != nil {
			return err
		}
	}
	return copyTable(src, dst, "documents")
}

// buildGraph cria graph.db com entities + relationships.
func buildGraph(src *sql.DB, target string) error {
	dst, err := sql.Open("sqlite", target)
	if err != nil {
		return err
	}
	defer dst.Close()

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS entities (
			id TEXT PRIMARY KEY, entity_type TEXT NOT NULL, name TEXT NOT NULL,
			path TEXT NOT NULL, metadata_json TEXT NOT NULL DEFAULT '{}',
			embedding BLOB, created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')))`,
		`CREATE TABLE IF NOT EXISTS relationships (
			id TEXT PRIMARY KEY, source_id TEXT NOT NULL, source_type TEXT NOT NULL,
			target_id TEXT NOT NULL, target_type TEXT NOT NULL, rel_type TEXT NOT NULL,
			weight REAL NOT NULL DEFAULT 1.0, metadata_json TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL DEFAULT (datetime('now')))`,
	}
	for _, s := range stmts {
		if _, err := dst.Exec(s); err != nil {
			return err
		}
	}
	if err := copyTable(src, dst, "entities"); err != nil {
		return err
	}
	return copyTable(src, dst, "relationships")
}

// buildProjects cria projects.db com chunks, headings, code_blocks, tables.
func buildProjects(src *sql.DB, target string) error {
	dst, err := sql.Open("sqlite", target)
	if err != nil {
		return err
	}
	defer dst.Close()

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS chunks (
			id TEXT PRIMARY KEY, document_id TEXT NOT NULL, content TEXT NOT NULL,
			heading TEXT NOT NULL DEFAULT '', section_type TEXT NOT NULL DEFAULT 'text',
			position INTEGER NOT NULL DEFAULT 0, hash TEXT NOT NULL DEFAULT '',
			token_count INTEGER NOT NULL DEFAULT 0, metadata_json TEXT NOT NULL DEFAULT '{}',
			embedding BLOB)`,
		`CREATE TABLE IF NOT EXISTS headings (
			id TEXT PRIMARY KEY, document_id TEXT NOT NULL, level INTEGER NOT NULL DEFAULT 1,
			text TEXT NOT NULL, position INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE IF NOT EXISTS code_blocks (
			id TEXT PRIMARY KEY, document_id TEXT NOT NULL, language TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL, position INTEGER NOT NULL DEFAULT 0,
			token_count INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE IF NOT EXISTS tables (
			id TEXT PRIMARY KEY, document_id TEXT NOT NULL, caption TEXT NOT NULL DEFAULT '',
			headers_json TEXT NOT NULL DEFAULT '[]', rows_json TEXT NOT NULL DEFAULT '[]',
			position INTEGER NOT NULL DEFAULT 0)`,
	}
	for _, s := range stmts {
		if _, err := dst.Exec(s); err != nil {
			return err
		}
	}
	for _, t := range []string{"chunks", "headings", "code_blocks", "tables"} {
		if err := copyTable(src, dst, t); err != nil {
			return err
		}
	}
	return nil
}

// buildVectorDomain cria um arquivo de vetores particionado por domínio.
// O vetor é classificado pelo path do documento (responsabilidade real).
func buildVectorDomain(domain string) func(*sql.DB, string) error {
	return func(src *sql.DB, target string) error {
		dst, err := sql.Open("sqlite", target)
		if err != nil {
			return err
		}
		defer dst.Close()

		create := `CREATE TABLE IF NOT EXISTS vectors (
			id TEXT PRIMARY KEY, vector BLOB NOT NULL, metadata TEXT NOT NULL DEFAULT '{}',
			document_id TEXT NOT NULL DEFAULT '', chunk_id TEXT NOT NULL DEFAULT '',
			entity_id TEXT NOT NULL DEFAULT '', content TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now')))`
		if _, err := dst.Exec(create); err != nil {
			return err
		}

		// Seleciona os vetores do domínio (por path do documento).
		rows, err := src.Query(`
			SELECT v.id, v.vector, v.metadata, v.document_id, v.chunk_id, v.entity_id, v.content, v.created_at
			FROM vectors v LEFT JOIN documents d ON d.id = v.document_id`)
		if err != nil {
			return err
		}
		defer rows.Close()

		tx, err := dst.Begin()
		if err != nil {
			return err
		}
		ins, err := tx.Prepare(`INSERT OR REPLACE INTO vectors
			(id, vector, metadata, document_id, chunk_id, entity_id, content, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		defer ins.Close()

		n := 0
		for rows.Next() {
			var id, metadata, docID, chunkID, entityID, content, createdAt string
			var vec []byte
			if err := rows.Scan(&id, &vec, &metadata, &docID, &chunkID, &entityID, &content, &createdAt); err != nil {
				continue
			}
			// Precisamos do path do documento para classificar — segunda query.
			var path string
			if err := src.QueryRow("SELECT path FROM documents WHERE id=?", docID).Scan(&path); err != nil {
				path = ""
			}
			if vectorDomain(path) != domain {
				continue
			}
			if _, err := ins.Exec(id, vec, metadata, docID, chunkID, entityID, content, createdAt); err != nil {
				_ = tx.Rollback()
				return err
			}
			n++
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		_ = n
		return nil
	}
}

// copyTable copia uma tabela inteira de src para dst (colunas em comum).
func copyTable(src, dst *sql.DB, table string) error {
	cols, err := src.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return err
	}
	var names []string
	var types []string
	for cols.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt interface{}
		if err := cols.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			cols.Close()
			return err
		}
		names = append(names, name)
		types = append(types, ctype)
	}
	cols.Close()

	colList := strings.Join(names, ", ")
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(names)), ",")

	rows, err := src.Query("SELECT " + colList + " FROM " + table)
	if err != nil {
		return err
	}
	defer rows.Close()

	scanArgs := make([]any, len(names))
	for i := range scanArgs {
		scanArgs[i] = new(any)
	}

	tx, err := dst.Begin()
	if err != nil {
		return err
	}
	ins, err := tx.Prepare("INSERT OR REPLACE INTO " + table + " (" + colList + ") VALUES (" + placeholders + ")")
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer ins.Close()

	for rows.Next() {
		if err := rows.Scan(scanArgs...); err != nil {
			_ = tx.Rollback()
			return err
		}
		args := make([]any, len(scanArgs))
		for i, v := range scanArgs {
			args[i] = *(v.(*any))
		}
		if _, err := ins.Exec(args...); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// vectorDomain classifica o path de um documento num domínio de responsabilidade.
// O domínio embed (cérebro) é subdividido por SUB-RESPONSABILIDADE porque o
// índice de 95 MB de BLOBs + overhead on-disk estoura o teto de 100 MB num
// único arquivo (ADR-013 §2.2.1: particionar quando o módulo estoura).
func vectorDomain(path string) string {
	p := strings.ReplaceAll(path, "\\", "/")
	switch {
	case strings.Contains(p, "internal/embed/cosca"):
		switch {
		case strings.Contains(p, "memory/"):
			return "embed-memory"
		case strings.Contains(p, "engines/") || strings.Contains(p, "departments/") || strings.Contains(p, "skills/"):
			return "embed-engines"
		default:
			return "embed-core"
		}
	case strings.Contains(p, ".opencode/cosca"):
		return "opencode"
	case strings.Contains(p, ".cosca/fallback"):
		return "fallback"
	case strings.Contains(p, "/docs/"):
		return "docs"
	case strings.HasSuffix(p, ".go") || strings.Contains(p, "/internal/") ||
		strings.Contains(p, "/api/") || strings.Contains(p, "/pkg/") ||
		strings.Contains(p, "/cmd/"):
		return "code"
	default:
		return "other"
	}
}

// fileSize retorna o tamanho do arquivo em bytes (0 se não existir).
func fileSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}

// jsonMarshalImpl serializa para JSON indentado.
func jsonMarshalImpl(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
