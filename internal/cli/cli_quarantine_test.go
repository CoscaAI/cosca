//
// Tests for the `cosca quarantine` command tree (internal/cli/quarantine.go).
//
// Covers:
//   - Registration of `quarantine` in the root command
//   - Command properties + arg constraints (validate/promote/discard exigem id)
//   - `quarantine add` → creates Q-0001 with status pending (t.TempDir, nunca o .cosca real)
//   - `quarantine list` → tabela (id, status, título, criada em); vazio → aviso
//   - `quarantine validate` → pending → validating
//   - `quarantine promote --to K-06` → validating → promoted + PromotedTo
//   - `quarantine discard` → arquivado em .cosca/quarantine/archive/
//   - JSON output (add/list)
//   - O ciclo completo add → list → validate → promote / discard
//
// NOTE: os testes fazem chdir() (mutam o cwd do processo) e NÃO rodam em
// paralelo. Formatter injetado via newContextWithFormatter (padrão do CLI).

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

	"github.com/CoscaAI/cosca/internal/quarantine"
)

// chdirQuarantineTemp muda para um diretório temporário e restaura o cwd no
// cleanup. Os comandos da quarentena operam em <tmp>/.cosca/quarantine.
func chdirQuarantineTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Chdir registra o restore do cwd DEPOIS do RemoveAll do TempDir
	// (cleanups rodam em LIFO) — no Windows não dá para remover o
	// diretório que é o CWD do processo.
	t.Chdir(dir)
	return dir
}

// runQuarantineCommand executa o RunE de um subcomando com formatter injetado
// em um buffer (padrão de injeção de formatter do CLI).
func runQuarantineCommand(t *testing.T, cmd *cobra.Command, args []string) (string, error) {
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

// quarantineFilePath retorna o caminho do arquivo ativo da proposal.
func quarantineFilePath(dir, id string) string {
	return filepath.Join(dir, ".cosca", "quarantine", id+".json")
}

// =============================================================================
// Registration — `quarantine` no root
// =============================================================================

func TestQuarantineCommand_RegisteredInRoot(t *testing.T) {
	cmd := NewRootCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "quarantine" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("quarantine subcommand not registered in root command")
	}
}

func TestQuarantineCommand_Properties(t *testing.T) {
	cmd := NewQuarantineCommand()
	if cmd == nil {
		t.Fatal("NewQuarantineCommand returned nil")
	}
	if cmd.Use != "quarantine" {
		t.Errorf("expected Use='quarantine', got %q", cmd.Use)
	}
	if cmd.Short == "" || cmd.Long == "" {
		t.Error("expected non-empty Short/Long description")
	}
	expected := []string{"add", "list", "validate", "promote", "discard"}
	if len(cmd.Commands()) != len(expected) {
		t.Errorf("expected %d subcommands, got %d", len(expected), len(cmd.Commands()))
	}
	registered := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("missing quarantine subcommand: %s", name)
		}
	}
}

func TestQuarantine_ArgConstraints(t *testing.T) {
	validate := NewQuarantineValidateCommand()
	if err := validate.Args(validate, nil); err == nil {
		t.Error("validate should require an id")
	}
	if err := validate.Args(validate, []string{"Q-0001"}); err != nil {
		t.Errorf("validate with one arg should be allowed: %v", err)
	}

	promote := NewQuarantinePromoteCommand()
	if err := promote.Args(promote, []string{"Q-0001", "Q-0002"}); err == nil {
		t.Error("promote with two args should fail")
	}

	discard := NewQuarantineDiscardCommand()
	if err := discard.Args(discard, nil); err == nil {
		t.Error("discard should require an id")
	}
}

// =============================================================================
// `quarantine list` — vazio
// =============================================================================

func TestQuarantineList_Empty(t *testing.T) {
	chdirQuarantineTemp(t)
	out, err := runQuarantineCommand(t, NewQuarantineListCommand(), nil)
	if err != nil {
		t.Fatalf("quarantine list (empty): %v", err)
	}
	if !strings.Contains(out, "Quarentena vazia") {
		t.Errorf("expected empty-state warning, got: %q", out)
	}
}

// =============================================================================
// Ciclo completo: add → list → validate → promote/discard
// =============================================================================

func TestQuarantineFullCycle(t *testing.T) {
	dir := chdirQuarantineTemp(t)

	// add
	add := NewQuarantineAddCommand()
	_ = add.ParseFlags([]string{"--title", "Cache de embeddings", "--content", "Propor LRU no indexer", "--source", "agent-x"})
	out, err := runQuarantineCommand(t, add, nil)
	if err != nil {
		t.Fatalf("quarantine add: %v", err)
	}
	if !strings.Contains(out, "Q-0001") {
		t.Errorf("add output missing Q-0001: %q", out)
	}
	if !strings.Contains(out, "pending") {
		t.Errorf("add output missing status pending: %q", out)
	}
	if _, err := os.Stat(quarantineFilePath(dir, "Q-0001")); err != nil {
		t.Fatalf("Q-0001.json not written: %v", err)
	}

	// list (populado)
	out, err = runQuarantineCommand(t, NewQuarantineListCommand(), nil)
	if err != nil {
		t.Fatalf("quarantine list: %v", err)
	}
	if !strings.Contains(out, "Q-0001") || !strings.Contains(out, "Cache de embeddings") {
		t.Errorf("list missing the added proposal: %q", out)
	}

	// validate
	out, err = runQuarantineCommand(t, NewQuarantineValidateCommand(), []string{"Q-0001"})
	if err != nil {
		t.Fatalf("quarantine validate: %v", err)
	}
	if !strings.Contains(out, "validating") {
		t.Errorf("validate output missing status validating: %q", out)
	}

	// promote
	promote := NewQuarantinePromoteCommand()
	_ = promote.ParseFlags([]string{"--to", "K-06"})
	out, err = runQuarantineCommand(t, promote, []string{"Q-0001"})
	if err != nil {
		t.Fatalf("quarantine promote: %v", err)
	}
	if !strings.Contains(out, "K-06") || !strings.Contains(out, "promovida") {
		t.Errorf("promote output missing target/confirmation: %q", out)
	}
	if !strings.Contains(out, "manual/aprovada") {
		t.Errorf("promote output must document the manual/approved promotion step: %q", out)
	}

	// discard (uma segunda proposal)
	add2 := NewQuarantineAddCommand()
	_ = add2.ParseFlags([]string{"--title", "Ideia rejeitada", "--content", "Texto descartável"})
	if _, err := runQuarantineCommand(t, add2, nil); err != nil {
		t.Fatalf("quarantine add #2: %v", err)
	}
	if _, err := runQuarantineCommand(t, NewQuarantineValidateCommand(), []string{"Q-0002"}); err != nil {
		t.Fatalf("quarantine validate #2: %v", err)
	}
	out, err = runQuarantineCommand(t, NewQuarantineDiscardCommand(), []string{"Q-0002"})
	if err != nil {
		t.Fatalf("quarantine discard: %v", err)
	}
	if !strings.Contains(out, "arquivada") {
		t.Errorf("discard output missing archival confirmation: %q", out)
	}
	// arquivo movido para archive/, não mais no ativo.
	if _, err := os.Stat(quarantineFilePath(dir, "Q-0002")); !os.IsNotExist(err) {
		t.Errorf("Q-0002.json should have left the active dir (err=%v)", err)
	}
	archived := filepath.Join(dir, ".cosca", "quarantine", "archive", "Q-0002.json")
	if _, err := os.Stat(archived); err != nil {
		t.Errorf("archived Q-0002.json must exist: %v", err)
	}
}

// =============================================================================
// JSON output
// =============================================================================

func TestQuarantineAdd_JSON(t *testing.T) {
	chdirQuarantineTemp(t)
	globalFlags = GlobalFlags{JSON: true}

	add := NewQuarantineAddCommand()
	_ = add.ParseFlags([]string{"--title", "T", "--content", "C", "--source", "s"})
	out, err := runQuarantineCommand(t, add, nil)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	var prop quarantine.Proposal
	if err := json.Unmarshal([]byte(out), &prop); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if prop.ID != "Q-0001" || prop.Status != quarantine.StatusPending {
		t.Errorf("unexpected json proposal: %+v", prop)
	}
}

// =============================================================================
// Transições inválidas via CLI
// =============================================================================

func TestQuarantinePromote_WithoutValidationFails(t *testing.T) {
	chdirQuarantineTemp(t)
	add := NewQuarantineAddCommand()
	_ = add.ParseFlags([]string{"--title", "T", "--content", "C"})
	if _, err := runQuarantineCommand(t, add, nil); err != nil {
		t.Fatalf("add: %v", err)
	}
	promote := NewQuarantinePromoteCommand()
	_ = promote.ParseFlags([]string{"--to", "K-06"})
	if _, err := runQuarantineCommand(t, promote, []string{"Q-0001"}); err == nil {
		t.Error("promote of a pending proposal should fail via CLI")
	}
}

// compile-time guard: the command constructors must match the cobra contract.
var (
	_ *cobra.Command = NewQuarantineCommand()
	_ *cobra.Command = NewQuarantineAddCommand()
	_ *cobra.Command = NewQuarantineListCommand()
	_ *cobra.Command = NewQuarantineValidateCommand()
	_ *cobra.Command = NewQuarantinePromoteCommand()
	_ *cobra.Command = NewQuarantineDiscardCommand()
)
