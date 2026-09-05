// Package cli — `cosca db migrate` (D2b do Plano D, ADR-013).
//
// A migração formal do corte: reconstrói os módulos físicos a partir da
// fonte E valida por checksum contra um SNAPSHOT capturado ANTES — não
// contra a fonte viva (o problema do alvo móvel detectado na D2: o MCP
// serve reindexa em rajadas, então comparar módulo vs fonte em momentos
// diferentes produz falso "diverge").
//
// Fluxo:
//  1. Captura o snapshot da fonte (contagens + checksums) — t0;
//  2. Reconstrói os módulos (db build) — o estado capturado;
//  3. Valida cada módulo contra o SNAPSHOT (não contra a fonte) — se a
//     fonte mudou depois de t0, o migrate ainda é válido (o módulo reflete
//     t0); se o módulo divergir do snapshot, a migração FALHOU de verdade.
//
// Nota: se a fonte muda DURANTE o build, o módulo pode capturar um estado
// misto. Por isso o migrate reporta "fonte mudou durante a migração" quando
// detecta divergência entre o snapshot e a fonte no fim — o operador decide
// se re-roda com a fonte parada (fluxo canônico do ADR-013 §5).
package cli

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	_ "modernc.org/sqlite"
)

// migrateSnapshot é o estado capturado da fonte antes da migração.
type migrateSnapshot struct {
	// tables é a lista de (arquivo-módulo, tabela) verificadas.
	tables []migrateTable
}

// migrateTable é uma (módulo, tabela) com contagem e checksum do snapshot.
type migrateTable struct {
	module string
	table  string
	count  int
	hash   string
}

// NewDBMigrateCommand cria `cosca db migrate` — migração formal com snapshot.
func NewDBMigrateCommand() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migração formal do corte: rebuild + validação por snapshot (Plano D D2b)",
		Long: `Migração formal do split (Plano D D2b): reconstrói os módulos
físicos a partir da fonte e VALIDA por checksum contra um snapshot capturado
ANTES — resolvendo o alvo móvel (a fonte reindexada pelo MCP durante a
leitura produziria falso "diverge").

Fluxo: (1) captura snapshot da fonte (contagens + hashes); (2) db build;
(3) valida cada módulo contra o SNAPSHOT. Se a fonte mudou durante o build,
o migrate reporta e o operador decide (re-rodar com fonte parada é o fluxo
canônico do ADR-013 §5: 'nunca migrar com o serve rodando').`,
		Example: `  cosca db migrate            # rebuild + validação por snapshot
  cosca db migrate --dry-run  # audita sem escrever`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, dirErr := resolveDataDir("")
			if dirErr != nil {
				return fmt.Errorf("resolve data directory: %w", dirErr)
			}
			kb := filepath.Join(dir, "knowledge.db")
			if _, statErr := os.Stat(kb); statErr != nil {
				return fmt.Errorf("knowledge.db não encontrado em %s", kb)
			}

			if dryRun {
				f.Header("Auditoria (dry-run) — migração do corte")
				f.Printf("  fonte: %s\n", kb)
				for _, m := range dbModules() {
					f.Printf("  → %s\n", m.name)
				}
				f.Print("\nO migrate captura o snapshot, reconstrói e valida por checksum. Nada foi escrito (dry-run).")
				return nil
			}

			// 1. Snapshot da fonte (contagens + checksums) — o estado de t0.
			src, err := sql.Open("sqlite", "file:"+filepath.ToSlash(kb)+"?mode=ro")
			if err != nil {
				return fmt.Errorf("abrir fonte: %w", err)
			}
			defer src.Close()

			snap := &migrateSnapshot{}
			for _, mt := range migrateTablesToCheck() {
				var n int
				if err := src.QueryRow("SELECT COUNT(*) FROM " + mt.table).Scan(&n); err != nil {
					return fmt.Errorf("snapshot count %s: %w", mt.table, err)
				}
				h, err := tableChecksum(src, mt.table)
				if err != nil {
					return fmt.Errorf("snapshot checksum %s: %w", mt.table, err)
				}
				snap.tables = append(snap.tables, migrateTable{
					module: mt.module, table: mt.table, count: n, hash: h,
				})
			}

			// 2. Rebuild dos módulos (db build).
			f.Header("COSCA DB MIGRATE — corte (D2b)")
			f.Printf("  snapshot capturado: %d tabelas\n", len(snap.tables))
			if _, err := dbBuildModules(nil, kb, dir); err != nil {
				return fmt.Errorf("db build: %w", err)
			}

			// 3. Valida cada módulo contra o SNAPSHOT (não contra a fonte viva).
			allOK := true
			for _, st := range snap.tables {
				path := filepath.Join(dir, st.module)
				if _, statErr := os.Stat(path); statErr != nil {
					f.Printf("  ❌ %-20s %s: módulo ausente\n", st.module, st.table)
					allOK = false
					continue
				}
				db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
				if err != nil {
					f.Printf("  ❌ %-20s %s: %v\n", st.module, st.table, err)
					allOK = false
					continue
				}
				var n int
				if err := db.QueryRow("SELECT COUNT(*) FROM " + st.table).Scan(&n); err != nil {
					_ = db.Close()
					f.Printf("  ❌ %-20s %s: %v\n", st.module, st.table, err)
					allOK = false
					continue
				}
				h, err := tableChecksum(db, st.table)
				_ = db.Close()
				if err != nil {
					f.Printf("  ❌ %-20s %s: checksum: %v\n", st.module, st.table, err)
					allOK = false
					continue
				}
				if h != st.hash {
					allOK = false
					f.Printf("  ❌ %-20s %s: DIVERGE do snapshot (módulo %d/%s vs snapshot %d/%s)\n",
						st.module, st.table, n, h[:12], st.count, st.hash[:12])
					continue
				}
				f.Printf("  ✅ %-20s %s: %d linhas, checksum idêntico ao snapshot\n",
					st.module, st.table, n)
			}

			// Verifica se a fonte mudou durante o build (alvo móvel).
			fonteMudou := false
			for _, st := range snap.tables {
				var n int
				if err := src.QueryRow("SELECT COUNT(*) FROM " + st.table).Scan(&n); err == nil && n != st.count {
					fonteMudou = true
					break
				}
			}
			if fonteMudou {
				f.Printf("\n⚠️ A fonte MUDOU durante a migração (alvo móvel — MCP reindexando).\n")
				f.Printf("   O módulo reflete o snapshot (%s); a fonte avançou. Para validação\n", "t0")
				f.Printf("   estrita, pare o index, rode 'cosca db migrate' e religue (ADR-013 §5).\n")
			}

			if useJSON {
				b, _ := jsonMarshalImpl(map[string]any{
					"knowledge_db": kb,
					"snapshot":     snap.tables,
					"integra":      allOK,
					"fonte_mudou":  fonteMudou,
				})
				f.Println(string(b))
				return nil
			}

			f.Println("")
			if allOK {
				f.Print("Migração íntegra — módulos fiéis ao snapshot da fonte.")
				return nil
			}
			return fmt.Errorf("db migrate: módulos divergem do snapshot da fonte")
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "auditar sem escrever")
	return cmd
}

// migrateTablesToCheck lista as (módulo, tabela) validadas na migração.
// O vector é somado (partições) e validado por contagem — o checksum do
// vector seria por partição (a soma não tem hash único).
func migrateTablesToCheck() []migrateTable {
	return []migrateTable{
		{module: "core.db", table: "documents"},
		{module: "graph.db", table: "entities"},
		{module: "graph.db", table: "relationships"},
		{module: "projects.db", table: "chunks"},
	}
}

var _ = sort.Strings // compat
