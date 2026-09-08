// health.go — healthcheck SQLite read-only (PRAGMA quick_check).
//
// Abre o banco em modo somente leitura (`mode=ro`, mesmo padrão do
// internal/dbhealth.openReadOnly) e roda `PRAGMA quick_check`. É a definição
// de "doente" usada pelo Repair: um banco que não abre como SQLite válido ou
// cujo quick_check não devolve "ok" está doente.
//
// 100% READ-ONLY: nunca escreve no arquivo do banco (o driver modernc em
// mode=ro recusa escrita no nível SQLite).
package staterepair

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite" // driver pure-Go SQLite — o driver do repo (dbhealth)
)

// quickCheckRowLimit é o número máximo de linhas de problema do quick_check
// incluídas no detalhe (um banco muito corrompido pode gerar milhares).
const quickCheckRowLimit = 3

// Health abre dbPath em mode=ro e roda `PRAGMA quick_check`.
//
// Devolve:
//   - ok=true quando o banco abre e o quick_check devolve apenas "ok";
//   - ok=false quando o arquivo não existe, não é um SQLite válido, ou o
//     quick_check acusa problema(s) — detail traz a razão legível;
//   - err preenchido apenas em falha de infraestrutura (stat/DSN/etc.).
func Health(dbPath string) (ok bool, detail string, err error) {
	if _, statErr := os.Stat(dbPath); statErr != nil {
		if os.IsNotExist(statErr) {
			return false, "arquivo não existe", nil
		}
		return false, "stat: " + statErr.Error(), nil
	}

	db, err := openReadOnlyHealth(dbPath)
	if err != nil {
		return false, "open: " + err.Error(), nil
	}
	defer db.Close()

	rows, err := db.Query("PRAGMA quick_check")
	if err != nil {
		return false, "quick_check: " + err.Error(), nil
	}
	defer rows.Close()

	var problems []string
	for rows.Next() {
		var result string
		if scanErr := rows.Scan(&result); scanErr != nil {
			problems = append(problems, "scan: "+scanErr.Error())
			continue
		}
		if strings.EqualFold(strings.TrimSpace(result), "ok") {
			continue
		}
		problems = append(problems, result)
		if len(problems) >= quickCheckRowLimit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return false, "quick_check rows: " + err.Error(), nil
	}
	if len(problems) > 0 {
		return false, "quick_check: " + strings.Join(problems, "; "), nil
	}
	return true, "quick_check ok", nil
}

// openReadOnlyHealth abre um banco SQLite em mode=ro de forma cross-platform
// (Windows usa barras invertidas -> file: URI com forward slashes). Valida com
// Ping para detectar arquivo que não é SQLite válido. Mesmo padrão de
// internal/dbhealth.openReadOnly.
func openReadOnlyHealth(absPath string) (*sql.DB, error) {
	dsn := "file:" + filepath.ToSlash(absPath) + "?mode=ro"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
