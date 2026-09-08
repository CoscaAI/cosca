// helpers_test.go — helpers compartilhados dos testes do staterepair.
//
// seedSQLite cria um SQLite válido e grande (cabeçalho + páginas de dados
// suficientes para as janelas de amostra do fingerprint — 64 KiB head + 4 KiB
// tail — terem regiões disjuntas).
// corruptSQLiteMidFile corrompe bytes no MEIO de um sqlite válido (a página de
// dados de uma b-tree) para simular o banco doente sintético do ADR-043 §8.
package staterepair

import (
	"bytes"
	"database/sql"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	_ "modernc.org/sqlite" // driver pure-Go SQLite — mesmo do repo
)

// seedTargetBytes é o tamanho mínimo do arquivo semeado (~256 KiB): garante
// head 64 KiB + tail 4 KiB disjuntas e páginas de dados no meio.
const seedTargetBytes = 256 * 1024

// seedSQLite cria (ou recria) um banco SQLite válido em path com dados além de
// seedTargetBytes.
func seedSQLite(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	_ = os.Remove(path)

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite %s: %v", path, err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS payload (
		id INTEGER PRIMARY KEY,
		data BLOB
	)`); err != nil {
		t.Fatalf("create payload table: %v", err)
	}

	chunk := 4096
	payload := bytes.Repeat([]byte{0x42}, chunk)
	rows := seedTargetBytes/chunk + 16 // margem cobre overhead de página
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO payload (data) VALUES (?)`)
	if err != nil {
		t.Fatalf("prepare insert: %v", err)
	}
	for i := 0; i < rows; i++ {
		if _, err := stmt.Exec(payload); err != nil {
			_ = stmt.Close()
			_ = tx.Rollback()
			t.Fatalf("insert payload: %v", err)
		}
	}
	if err := stmt.Close(); err != nil {
		t.Fatalf("close stmt: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat seeded db: %v", err)
	}
	if fi.Size() < seedTargetBytes {
		t.Fatalf("seedSQLite criou só %d bytes (alvo %d)", fi.Size(), seedTargetBytes)
	}
}

// sqlitePageSize lê o page size do header SQLite (offset 16, big-endian).
func sqlitePageSize(t *testing.T, path string) int {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	head := make([]byte, 18)
	if _, err := f.ReadAt(head, 0); err != nil {
		t.Fatalf("read header: %v", err)
	}
	// Header SQLite: page size é um big-endian uint16 nos bytes 16-17; 1 =
	// "legacy 64 KiB".
	ps := int(head[16])<<8 | int(head[17])
	if ps == 1 {
		return 65536
	}
	return ps
}

// corruptSQLiteMidFile escreve bytes lixo no meio de um sqlite válido (na
// página de dados), deixando o header íntegro — o arquivo abre mas o
// quick_check acusa corrupção.
func corruptSQLiteMidFile(t *testing.T, path string) {
	t.Helper()
	pageSize := sqlitePageSize(t, path)
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open rw %s: %v", path, err)
	}
	defer f.Close()
	// Corrompe o início da página 3 (offset 2*page_size): região de dados no
	// MEIO do arquivo, longe do header e da última página.
	offset := int64(2 * pageSize)
	garbage := make([]byte, 512)
	for i := range garbage {
		garbage[i] = 0xA5
	}
	if _, err := f.WriteAt(garbage, offset); err != nil {
		t.Fatalf("write garbage at %d: %v", offset, err)
	}
	if err := f.Sync(); err != nil {
		t.Fatalf("sync: %v", err)
	}
}

// corruptFileBytes grava bytes num offset do arquivo (mesmo tamanho).
func corruptFileBytes(t *testing.T, path string, offset int64, data []byte) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open rw %s: %v", path, err)
	}
	defer f.Close()
	if _, err := f.WriteAt(data, offset); err != nil {
		t.Fatalf("write at %d: %v", offset, err)
	}
}

// readFileBytes devolve o conteúdo do arquivo.
func readFileBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}

// treeSnapshot devolve a lista ordenada de caminhos relativos de tudo que
// existe sob root (arquivos e diretórios). Usado para provar que o dry-run
// não escreve NADA.
func treeSnapshot(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	sort.Strings(out)
	return out
}

// assertFileExists falha o teste se path não existir.
func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("arquivo %s deveria existir: %v", path, err)
	}
}

// assertFileMissing falha o teste se path existir.
func assertFileMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("arquivo %s NÃO deveria existir", path)
	}
}
