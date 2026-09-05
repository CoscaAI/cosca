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
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	var withChecksum bool
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Valida a integridade dos módulos físicos vs a fonte (ADR-013)",
		Long: `Valida a integridade do split físico do ADR-013: compara as
contagens dos módulos (core/graph/projects/vector-*.db) com o knowledge.db
(a fonte da verdade). Exit 0 = íntegro; exit != 0 = divergência detectada.

Com --checksum: além das contagens, compara o HASH de conteúdo de cada
tabela (linhas ordenadas pela PK, todas as colunas) — a prova de que a
cópia foi FIEL, não só no volume (D2 do Plano D).

READ-ONLY: todas as conexões em mode=ro. Nunca escreve, migra ou apaga.`,
		Example: `  cosca db verify
  cosca db verify --checksum
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

			// D2 do Plano D: checksum de conteúdo (prova de cópia FIEL, não
			// só de volume). Compara o hash das linhas ordenadas pela PK,
			// com todas as colunas, entre módulo e fonte.
			if withChecksum {
				for _, m := range mods {
					path := filepath.Join(dir, m.file)
					if _, statErr := os.Stat(path); statErr != nil {
						continue // ausente já reportado na contagem
					}
					srcHash, hErr := tableChecksum(src, m.table)
					if hErr != nil {
						f.Printf("  ⚠️ checksum fonte.%s: %v\n", m.table, hErr)
						continue
					}
					db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
					if err != nil {
						continue
					}
					modHash, hErr := tableChecksum(db, m.table)
					_ = db.Close()
					if hErr != nil {
						f.Printf("  ⚠️ checksum %s.%s: %v\n", m.file, m.table, hErr)
						continue
					}
					if srcHash != modHash {
						allOK = false
						f.Printf("  ❌ CHECKSUM %-20s %-14s diverge (fonte=%s módulo=%s)\n",
							m.file, m.table, srcHash[:12], modHash[:12])
					} else {
						f.Printf("  ✅ CHECKSUM %-20s %-14s idêntico (%s)\n",
							m.file, m.table, srcHash[:12])
					}
				}
			}

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
	cmd.Flags().BoolVar(&withChecksum, "checksum", false, "comparar também o hash de conteúdo de cada tabela (prova de cópia fiel)")
	return cmd
}

// tableChecksum calcula o SHA-256 das linhas de uma tabela ordenadas pela
// primeira coluna (a PK), serializando todas as colunas — a prova de que
// duas bases têm o MESMO conteúdo, não só o mesmo volume. Determinístico:
// mesma tabela + mesma ordenação = mesmo hash.
func tableChecksum(db *sql.DB, table string) (string, error) {
	cols, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return "", err
	}
	var colNames []string
	for cols.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt interface{}
		if err := cols.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			cols.Close()
			return "", err
		}
		colNames = append(colNames, name)
	}
	cols.Close()
	if len(colNames) == 0 {
		return "", fmt.Errorf("tabela %s sem colunas", table)
	}

	colList := strings.Join(colNames, ", ")
	orderCol := colNames[0] // PK (convenção do schema: primeira coluna)
	rows, err := db.Query("SELECT " + colList + " FROM " + table + " ORDER BY " + orderCol)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	h := sha256.New()
	args := make([]any, len(colNames))
	for i := range args {
		args[i] = new(any)
	}
	for rows.Next() {
		if err := rows.Scan(args...); err != nil {
			return "", err
		}
		for _, a := range args {
			v := *(a.(*any))
			switch tv := v.(type) {
			case nil:
				h.Write([]byte{0})
			case []byte:
				h.Write([]byte{1})
				h.Write(tv)
			default:
				h.Write([]byte{2})
				h.Write([]byte(fmt.Sprintf("%v", tv)))
			}
		}
		h.Write([]byte{0xFF}) // separador de linha
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
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
