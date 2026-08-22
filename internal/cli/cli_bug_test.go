//
// Tests for the `cosca bug` command tree (internal/cli/bug.go).
//
// Covers:
//   - Registration of `bug` (and its 4 subcommands) in the root command
//   - Command properties and arg constraints (register/match exigem
//     --component/--error; family exige exatamente 1 argumento)
//   - `bug register` → cria BUG-0001; o MESMO fingerprint de novo → incrementa
//     TimesSeen (reincidência #2) sem duplicar
//   - `bug register` com erro diferente → nova família (BUG-0002)
//   - `bug match` / `bug family` → agrupam a mesma família
//   - `bug list` → tabela (id, família, component:error, vezes, status)
//   - JSON output + formatter injection pattern
//
// NOTE: os testes fazem chdir() e NÃO rodam em paralelo. Formatter injetado
// via newContextWithFormatter (padrão do CLI).

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/kernel"
)

// chdirBugTemp muda para um diretório temporário e restaura o cwd no cleanup.
// Os comandos do bug operam em <tmp>/.cosca/bug.db.
func chdirBugTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Chdir registra o restore do cwd DEPOIS do RemoveAll do TempDir
	// (cleanups rodam em LIFO) — no Windows não dá para remover o
	// diretório que é o CWD do processo.
	t.Chdir(dir)
	globalFlags = GlobalFlags{}
	return dir
}

// runBugCommand executa o RunE de um subcomando com formatter injetado em um
// buffer (padrão de injeção de formatter do CLI).
func runBugCommand(t *testing.T, cmd *cobra.Command, args []string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true) // noColor
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	err := cmd.RunE(cmd, args)
	return buf.String(), err
}

// bugDBPath retorna o caminho da base de bugs do projeto.
func bugDBPath(dir string) string {
	return filepath.Join(dir, ".cosca", "bug.db")
}

// =============================================================================
// Registration — `bug` no root
// =============================================================================

func TestBugCommand_RegisteredInRoot(t *testing.T) {
	cmd := NewRootCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "bug" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("bug subcommand not registered in root command")
	}
}

func TestBugCommand_Properties(t *testing.T) {
	cmd := NewBugCommand()
	if cmd == nil {
		t.Fatal("NewBugCommand returned nil")
	}
	if cmd.Use != "bug" {
		t.Errorf("expected Use='bug', got %q", cmd.Use)
	}
	if cmd.Short == "" || cmd.Long == "" {
		t.Error("expected non-empty Short/Long description")
	}
	expected := []string{"register", "match", "family", "list"}
	if len(cmd.Commands()) != len(expected) {
		t.Errorf("expected %d subcommands, got %d", len(expected), len(cmd.Commands()))
	}
	registered := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("missing bug subcommand: %s", name)
		}
	}
}

func TestBugCommand_ArgConstraints(t *testing.T) {
	family := NewBugFamilyCommand()
	if err := family.Args(family, nil); err == nil {
		t.Error("family with no args should fail")
	}
	if err := family.Args(family, []string{"runtime.backup:permission_denied"}); err != nil {
		t.Errorf("family with one arg should be allowed: %v", err)
	}
	if err := family.Args(family, []string{"a", "b"}); err == nil {
		t.Error("family with two args should fail")
	}

	// register/match exigem --component e --error.
	for _, name := range []string{"component", "error"} {
		ann := NewBugRegisterCommand().Flags().Lookup(name).Annotations[cobra.BashCompOneRequiredFlag]
		if len(ann) != 1 || ann[0] != "true" {
			t.Errorf("register must mark --%s as required", name)
		}
		ann = NewBugMatchCommand().Flags().Lookup(name).Annotations[cobra.BashCompOneRequiredFlag]
		if len(ann) != 1 || ann[0] != "true" {
			t.Errorf("match must mark --%s as required", name)
		}
	}
}

// =============================================================================
// Ciclo: register → incrementa → família diferente → match/family/list
// =============================================================================

func TestBugFullCycle(t *testing.T) {
	dir := chdirBugTemp(t)

	// register → BUG-0001
	reg := NewBugRegisterCommand()
	_ = reg.ParseFlags([]string{"--component", "runtime.backup", "--error", "permission_denied",
		"--path", "/backups", "--phase", "snapshot", "--title", "backup falhou"})
	out, err := runBugCommand(t, reg, nil)
	if err != nil {
		t.Fatalf("bug register: %v", err)
	}
	for _, want := range []string{"BUG-0001", "runtime.backup:permission_denied:/backups:snapshot", "1x"} {
		if !strings.Contains(out, want) {
			t.Errorf("register output missing %q: %q", want, out)
		}
	}
	if _, err := os.Stat(bugDBPath(dir)); err != nil {
		t.Fatalf("bug.db not created: %v", err)
	}

	// MESMO fingerprint → reincidência #2, sem duplicar.
	reg = NewBugRegisterCommand()
	_ = reg.ParseFlags([]string{"--component", "runtime.backup", "--error", "permission_denied",
		"--path", "/backups", "--phase", "snapshot", "--title", "backup falhou de novo"})
	out, err = runBugCommand(t, reg, nil)
	if err != nil {
		t.Fatalf("bug register #2: %v", err)
	}
	for _, want := range []string{"reincidência #2", "BUG-0001", "2x"} {
		if !strings.Contains(out, want) {
			t.Errorf("revisit output missing %q: %q", want, out)
		}
	}

	// Erro diferente → nova família, BUG-0002.
	reg = NewBugRegisterCommand()
	_ = reg.ParseFlags([]string{"--component", "runtime.backup", "--error", "disk_full",
		"--path", "/backups", "--phase", "snapshot", "--title", "disco cheio"})
	out, err = runBugCommand(t, reg, nil)
	if err != nil {
		t.Fatalf("bug register #3: %v", err)
	}
	if !strings.Contains(out, "BUG-0002") {
		t.Errorf("different error should create BUG-0002: %q", out)
	}

	// match por component+error → só a família permission_denied (1 bug).
	match := NewBugMatchCommand()
	_ = match.ParseFlags([]string{"--component", "runtime.backup", "--error", "permission_denied"})
	out, err = runBugCommand(t, match, nil)
	if err != nil {
		t.Fatalf("bug match: %v", err)
	}
	for _, want := range []string{"Família runtime.backup:permission_denied", "BUG-0001", "2x"} {
		if !strings.Contains(out, want) {
			t.Errorf("match output missing %q: %q", want, out)
		}
	}
	if strings.Contains(out, "BUG-0002") {
		t.Errorf("match must not leak a different family: %q", out)
	}

	// family <key> → os bugs da família.
	family := NewBugFamilyCommand()
	out, err = runBugCommand(t, family, []string{"runtime.backup:permission_denied"})
	if err != nil {
		t.Fatalf("bug family: %v", err)
	}
	if !strings.Contains(out, "BUG-0001") || strings.Contains(out, "BUG-0002") {
		t.Errorf("family output should contain only BUG-0001: %q", out)
	}

	// list → 2 registros.
	list := NewBugListCommand()
	out, err = runBugCommand(t, list, nil)
	if err != nil {
		t.Fatalf("bug list: %v", err)
	}
	for _, want := range []string{"2 registro", "BUG-0001", "BUG-0002", "runtime.backup:permission_denied", "runtime.backup:disk_full"} {
		if !strings.Contains(out, want) {
			t.Errorf("list output missing %q: %q", want, out)
		}
	}
}

// =============================================================================
// JSON output
// =============================================================================

func TestBugRegister_JSON(t *testing.T) {
	chdirBugTemp(t)
	globalFlags = GlobalFlags{JSON: true}

	reg := NewBugRegisterCommand()
	_ = reg.ParseFlags([]string{"--component", "runtime.backup", "--error", "permission_denied",
		"--path", "/backups", "--phase", "snapshot"})
	out, err := runBugCommand(t, reg, nil)
	if err != nil {
		t.Fatalf("bug register --json: %v", err)
	}
	var rec kernel.BugRecord
	if err := json.Unmarshal([]byte(out), &rec); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if rec.ID != "BUG-0001" {
		t.Errorf("unexpected json bug id: %q", rec.ID)
	}
	if rec.Fingerprint.Key() != "runtime.backup:permission_denied:/backups:snapshot" {
		t.Errorf("unexpected fingerprint: %q", rec.Fingerprint.Key())
	}
	if rec.Family != "runtime.backup:permission_denied" || rec.TimesSeen != 1 {
		t.Errorf("unexpected family/times_seen: %q/%d", rec.Family, rec.TimesSeen)
	}
}

func TestBugList_JSON(t *testing.T) {
	chdirBugTemp(t)
	globalFlags = GlobalFlags{JSON: true}

	reg := NewBugRegisterCommand()
	_ = reg.ParseFlags([]string{"--component", "runtime.backup", "--error", "permission_denied"})
	if _, err := runBugCommand(t, reg, nil); err != nil {
		t.Fatalf("bug register: %v", err)
	}

	list := NewBugListCommand()
	out, err := runBugCommand(t, list, nil)
	if err != nil {
		t.Fatalf("bug list --json: %v", err)
	}
	var records []kernel.BugRecord
	if err := json.Unmarshal([]byte(out), &records); err != nil {
		t.Fatalf("list output is not valid JSON: %v\n%s", err, out)
	}
	if len(records) != 1 || records[0].ID != "BUG-0001" {
		t.Errorf("unexpected list: %+v", records)
	}
}

// compile-time guard: os comandos são construídos no padrão cobra.
var (
	_ *cobra.Command = NewBugCommand()
	_ *cobra.Command = NewBugRegisterCommand()
	_ *cobra.Command = NewBugMatchCommand()
	_ *cobra.Command = NewBugFamilyCommand()
	_ *cobra.Command = NewBugListCommand()
)
