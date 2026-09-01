// Package cli — `cosca db verify`: valida a integridade do split (ADR-013).
//
// Compara as contagens dos módulos físicos (core/graph/projects/vector-*.db)
// com a fonte (knowledge.db). É o cinto de segurança do split aditivo: se um
// módulo divergir da fonte (reindex, corrupção, drift), o verify acusa ANTES
// de a busca semântica operar com dados errados.
//
//	c cosca db verify              # compara tudo, exit 0 se íntegro
//	c cosca db verify --json       # saída estruturada
//
// READ-ONLY: só lê (mode=ro). Nunca escreve, migra ou apaga.
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

// dbVerifyCounts guarda as contagens de uma base (fonte ou módulo).
type dbVerifyCounts struct {
	documents     int
	chunks        int
	entities      int
	relationships int
	vectors       int
}

// NewDBVerifyCommand cria `cosca db verify`.
func NewDBVerifyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Valida a integridade dos módulos físicos vs a fonte (ADR-013)",
		Long: `Valida a integridade do split físico do ADR-013: compara as
contagens dos módulos (core/graph/projects/vector-*.db) com o knowledge.db
(a fonte da verdade). Exit 0 = íntegro; exit != 0 = divergência detectada.

READ-ONLY: todas as conexões em mode=ro. Nunca escreve, migra ou apaga.`,
		Example: `  cosca db verify
  cosca db verify --json`,
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

			// Contagens da fonte.
			src, err := sql.Open("sqlite", "file:"+filepath.ToSlash(kb)+"?mode=ro")
			if err != nil {
				return fmt.Errorf("abrir fonte: %w", err)
			}
			defer src.Close()

			var srcCounts dbVerifyCounts
			src.QueryRow("SELECT COUNT(*) FROM documents").Scan(&srcCounts.documents)
			src.QueryRow("SELECT COUNT(*) FROM chunks").Scan(&srcCounts.chunks)
			src.QueryRow("SELECT COUNT(*) FROM entities").Scan(&srcCounts.entities)
			src.QueryRow("SELECT COUNT(*) FROM relationships").Scan(&srcCounts.relationships)
			src.QueryRow("SELECT COUNT(*) FROM vectors").Scan(&srcCounts.vectors)

			// Módulos.
			mods := []struct {
				file  string
				table string
			}{
				{"core.db", "documents"},
				{"graph.db", "entities"},
				{"graph.db", "relationships"},
				{"projects.db", "chunks"},
			}

			type row struct {
				Module    string `json:"module"`
				Table     string `json:"table"`
				ModuleCnt int    `json:"module_count"`
				SourceCnt int    `json:"source_count"`
				OK        bool   `json:"ok"`
			}
			var rows []row
			allOK := true

			for _, m := range mods {
				path := filepath.Join(dir, m.file)
				var n int
				if _, statErr := os.Stat(path); statErr != nil {
					allOK = false
					rows = append(rows, row{Module: m.file, Table: m.table, ModuleCnt: -1, SourceCnt: srcFor(&srcCounts, m.table), OK: false})
					continue
				}
				db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
				if err != nil {
					allOK = false
					rows = append(rows, row{Module: m.file, Table: m.table, ModuleCnt: -1, SourceCnt: srcFor(&srcCounts, m.table), OK: false})
					continue
				}
				if err := db.QueryRow("SELECT COUNT(*) FROM " + m.table).Scan(&n); err != nil {
					_ = db.Close()
					allOK = false
					rows = append(rows, row{Module: m.file, Table: m.table, ModuleCnt: -1, SourceCnt: srcFor(&srcCounts, m.table), OK: false})
					continue
				}
				_ = db.Close()
				srcCnt := srcFor(&srcCounts, m.table)
				ok := n == srcCnt
				if !ok {
					allOK = false
				}
				rows = append(rows, row{Module: m.file, Table: m.table, ModuleCnt: n, SourceCnt: srcCnt, OK: ok})
			}

			// Vetores: somar todas as partições vector-*.db.
			vecTotal := 0
			vecMatches, _ := filepath.Glob(filepath.Join(dir, "vector-*.db"))
			sort.Strings(vecMatches)
			for _, p := range vecMatches {
				db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(p)+"?mode=ro")
				if err != nil {
					continue
				}
				var n int
				if err := db.QueryRow("SELECT COUNT(*) FROM vectors").Scan(&n); err == nil {
					vecTotal += n
				}
				_ = db.Close()
			}
			vecOK := vecTotal == srcCounts.vectors
			if !vecOK {
				allOK = false
			}
			rows = append(rows, row{Module: "vector-*.db (soma)", Table: "vectors", ModuleCnt: vecTotal, SourceCnt: srcCounts.vectors, OK: vecOK})

			if useJSON {
				b, _ := jsonMarshalImpl(map[string]any{
					"knowledge_db": kb,
					"modules":      rows,
					"integra":      allOK,
				})
				f.Println(string(b))
				return nil
			}

			f.Header("DB Verify — integridade do split (ADR-013)")
			f.KeyValue("fonte", kb)
			f.Println("")
			for _, r := range rows {
				status := "✅"
				mark := "OK"
				if !r.OK {
					status = "❌"
					mark = "DIVERGE"
				}
				if r.ModuleCnt < 0 {
					status = "⚠️"
					mark = "ausente"
				}
				f.Printf("  %s %-24s %-14s módulo=%-8d fonte=%-8d %s\n",
					status, r.Module, r.Table, r.ModuleCnt, r.SourceCnt, mark)
			}
			f.Println("")
			if allOK {
				f.Print("Split íntegro — módulos equivalentes à fonte.")
				return nil
			}
			f.Print("DIVERGÊNCIA detectada — rode 'cosca db build' para reconstruir os módulos.")
			return fmt.Errorf("db verify: divergência nos módulos do split")
		},
	}
	return cmd
}

// srcFor devolve a contagem da fonte para uma tabela.
func srcFor(c *dbVerifyCounts, table string) int {
	switch table {
	case "documents":
		return c.documents
	case "chunks":
		return c.chunks
	case "entities":
		return c.entities
	case "relationships":
		return c.relationships
	case "vectors":
		return c.vectors
	}
	return -1
}
