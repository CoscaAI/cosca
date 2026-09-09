//
// Tests for the `cosca db repair` command (internal/cli/db_repair.go).
//
// Cobre os modos read-only (--check e --dry-run) do ADR-043 §8: o apply real
// re-executa o binário e é exercitado no nível do pacote staterepair (com
// recipe runner injetável) — aqui, o apply nunca roda dentro do teste para não
// recursar o binário de teste.

package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// jsonValidObject devolve true quando s é um objeto JSON válido.
func jsonValidObject(t *testing.T, s string) bool {
	t.Helper()
	var v map[string]interface{}
	return json.Unmarshal([]byte(s), &v) == nil
}

// seedDBRepairSQLite cria um SQLite válido com dados em path.
func seedDBRepairSQLite(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS docs (
		id TEXT PRIMARY KEY, path TEXT NOT NULL, content TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO docs (id, path, content) VALUES (?, ?, ?)`)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	for i := 0; i < 200; i++ {
		if _, err := stmt.Exec(strings.Repeat("x", 32)+itoaTest(i), "/doc/"+itoaTest(i), strings.Repeat("conteúdo ", 100)); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	_ = stmt.Close()
	_ = tx.Commit()
}

// itoaTest converte int para string decimal (evita import strconv no hot path).
func itoaTest(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}

// dbRepairPageSize lê o page size do header SQLite (bytes 16-17 big-endian).
func dbRepairPageSize(t *testing.T, path string) int64 {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	head := make([]byte, 18)
	if _, err := f.ReadAt(head, 0); err != nil {
		t.Fatalf("read header: %v", err)
	}
	ps := int64(head[16])<<8 | int64(head[17])
	if ps == 1 {
		return 65536
	}
	return ps
}

// corruptDBRepairSQLite escreve bytes lixo no meio de um sqlite válido.
func corruptDBRepairSQLite(t *testing.T, path string) {
	t.Helper()
	pageSize := dbRepairPageSize(t, path)
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open rw: %v", err)
	}
	defer f.Close()
	garbage := bytes.Repeat([]byte{0x5A}, 512)
	if _, err := f.WriteAt(garbage, 2*pageSize); err != nil {
		t.Fatalf("corrupt: %v", err)
	}
	_ = f.Sync()
}

// setupDBRepairProject cria um projeto temporário com `.cosca` e um
// session.db válido (ou corrompido), e faz `cd` para o projeto. Devolve o dir.
func setupDBRepairProject(t *testing.T, corrupt bool) string {
	t.Helper()
	origin, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir, err := os.MkdirTemp("", "cosca-dbrepair-")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) }) // roda por último (LIFO)
	t.Cleanup(func() { _ = os.Chdir(origin) })  // roda primeiro (LIFO)

	coscaDir := filepath.Join(dir, ".cosca")
	if err := os.MkdirAll(coscaDir, 0o755); err != nil {
		t.Fatalf("mkdir .cosca: %v", err)
	}
	db := filepath.Join(coscaDir, "session.db")
	seedDBRepairSQLite(t, db)
	if corrupt {
		corruptDBRepairSQLite(t, db)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	return dir
}

// treeSnapshotDBRepair lista os caminhos relativos sob root (para provar que
// dry-run não escreve nada).
func treeSnapshotDBRepair(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
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
	sort.Strings(out)
	return out
}

// executeDBRepair executa `cosca db repair <args...>` via NewRootCommand e
// devolve stdout e o erro do Execute() (sem initConfig/telemetria).
func executeDBRepair(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCommand()
	root.PersistentPreRunE = nil // evita initConfig/telemetria; determinístico
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	formatter := NewOutputFormatter(buf, OutputFormatText, false, false, false)
	root.SetContext(newContextWithFormatter(context.Background(), formatter))
	root.SetArgs(append([]string{"db", "repair"}, args...))
	err := root.Execute()
	return buf.String(), err
}

// TestDBRepair_RegisteredUnderDB prova que `repair` é subcomando de `db`.
func TestDBRepair_RegisteredUnderDB(t *testing.T) {
	root := NewRootCommand()
	var dbCmd *cobra.Command
	for _, sub := range root.Commands() {
		if sub.Name() == "db" {
			dbCmd = sub
			break
		}
	}
	if dbCmd == nil {
		t.Fatal("db subcommand not found")
	}
	found := false
	for _, sub := range dbCmd.Commands() {
		if sub.Name() == "repair" {
			found = true
			break
		}
	}
	if !found {
		t.Error("repair subcommand not registered under db")
	}
}

// TestDBRepair_Check_Healthy_ExitsZero: banco válido → exit 0 e relatório ok.
func TestDBRepair_Check_Healthy_ExitsZero(t *testing.T) {
	dir := setupDBRepairProject(t, false)
	coscaDir := filepath.Join(dir, ".cosca")

	out, err := executeDBRepair(t, "--db", "session", "--check", "--data-dir", coscaDir)
	if err != nil {
		t.Fatalf("db repair --check (saudável) deveria ter exit 0, got: %v\noutput:\n%s", err, out)
	}
	if code := ExitCode(err); code != 0 {
		t.Fatalf("esperava exit 0, got %d\noutput:\n%s", code, out)
	}
	for _, want := range []string{"DB Repair", "session", "ok"} {
		if !strings.Contains(out, want) {
			t.Errorf("output deveria conter %q; output:\n%s", want, out)
		}
	}
}

// TestDBRepair_Check_Corrupt_ExitsNonZero: banco doente → exit != 0 e relatório
// "doente".
func TestDBRepair_Check_Corrupt_ExitsNonZero(t *testing.T) {
	dir := setupDBRepairProject(t, true)
	coscaDir := filepath.Join(dir, ".cosca")

	out, err := executeDBRepair(t, "--db", "session", "--check", "--data-dir", coscaDir)
	if err == nil {
		t.Fatalf("db repair --check (doente) deveria ter exit != 0, got nil\noutput:\n%s", out)
	}
	if code := ExitCode(err); code == 0 {
		t.Fatalf("esperava exit != 0, got %d (err=%v)\noutput:\n%s", code, err, out)
	}
	if !strings.Contains(out, "doente") {
		t.Errorf("output deveria conter 'doente'; output:\n%s", out)
	}
}

// TestDBRepair_DryRun_WritesNothing: dry-run em banco corrompido reporta o
// plano e NÃO cria nenhum arquivo (critério de aceite 5 do ADR-043 §8).
func TestDBRepair_DryRun_WritesNothing(t *testing.T) {
	dir := setupDBRepairProject(t, true)
	coscaDir := filepath.Join(dir, ".cosca")

	before := treeSnapshotDBRepair(t, coscaDir)

	out, err := executeDBRepair(t, "--db", "session", "--dry-run", "--data-dir", coscaDir)
	if err == nil {
		t.Fatalf("dry-run de banco doente deveria sinalizar (exit != 0), got nil\noutput:\n%s", out)
	}
	if !strings.Contains(out, "plano") {
		t.Errorf("output deveria conter o plano; output:\n%s", out)
	}

	after := treeSnapshotDBRepair(t, coscaDir)
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Errorf("dry-run escreveu arquivos:\nantes:\n%s\ndepois:\n%s",
			strings.Join(before, "\n"), strings.Join(after, "\n"))
	}
	// Nenhum diretório de backup/quarentena nem ledger sidecar.
	for _, forbidden := range []string{
		filepath.Join(coscaDir, "backups"),
		filepath.Join(coscaDir, "quarantine"),
		filepath.Join(coscaDir, "session.db.repair-attempts.json"),
	} {
		if _, statErr := os.Stat(forbidden); statErr == nil {
			t.Errorf("dry-run criou %s — proibido", forbidden)
		}
	}
}

// TestDBRepair_MissingModeError: nenhum modo → erro de uso.
func TestDBRepair_MissingModeError(t *testing.T) {
	setupDBRepairProject(t, false)

	_, err := executeDBRepair(t, "--db", "session")
	if err == nil {
		t.Fatal("db repair sem modo deveria falhar")
	}
	if !strings.Contains(err.Error(), "--check") {
		t.Errorf("erro deveria nomear os modos; got: %v", err)
	}
}

// TestDBRepair_MutuallyExclusiveModes: --dry-run --apply juntos → erro.
func TestDBRepair_MutuallyExclusiveModes(t *testing.T) {
	setupDBRepairProject(t, false)

	_, err := executeDBRepair(t, "--db", "session", "--dry-run", "--apply")
	if err == nil {
		t.Fatal("modos mutuamente exclusivos deveriam falhar")
	}
	if !strings.Contains(err.Error(), "mutuamente exclusivos") {
		t.Errorf("erro deveria explicar a exclusividade; got: %v", err)
	}
}

// TestDBRepair_InvalidKindRejected: --db chain é rejeitado (nunca auto).
func TestDBRepair_InvalidKindRejected(t *testing.T) {
	setupDBRepairProject(t, false)

	_, err := executeDBRepair(t, "--db", "chain", "--check")
	if err == nil {
		t.Fatal("--db chain deveria ser rejeitado (chain nunca é reparável)")
	}
	if !strings.Contains(err.Error(), "inválida") {
		t.Errorf("erro deveria dizer classe inválida; got: %v", err)
	}
}

// TestDBRepair_JSONOutputValido: --json devolve JSON parseável.
func TestDBRepair_JSONOutputValido(t *testing.T) {
	dir := setupDBRepairProject(t, true)
	coscaDir := filepath.Join(dir, ".cosca")

	out, err := executeDBRepair(t, "--db", "session", "--check", "--json", "--data-dir", coscaDir)
	if err == nil {
		t.Fatal("--check em banco doente deveria ter exit != 0 mesmo em JSON")
	}
	if !jsonValidObject(t, out) {
		t.Errorf("saída não é JSON válido:\n%s", out)
	}
}
