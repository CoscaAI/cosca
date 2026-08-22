//
// Tests for the `cosca session` command tree (internal/cli/session.go).
//
// Covers:
//   - Registration of `session` in the root command
//   - Command properties + arg constraints (search/lineage exigem 1 arg;
//     index não aceita args)
//   - `session index` reindexa um projeto temp e reporta a contagem
//   - `session search` imprime tabela de ocorrências (sessão, papel, quando,
//     conteúdo truncado); sem ocorrências → "Nenhuma ocorrência"
//   - Formatter injetado via newContextWithFormatter (padrão do CLI)
//
// NOTE: os testes fazem chdir() e NÃO rodam em paralelo.

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// chdirSessionTemp muda para um diretório temporário e restaura o cwd no
// cleanup. Os comandos de sessão operam em <tmp>/.cosca/session.db.
func chdirSessionTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Chdir registra o restore do cwd DEPOIS do RemoveAll do TempDir
	// (cleanups rodam em LIFO) — no Windows não dá para remover o
	// diretório que é o CWD do processo.
	t.Chdir(dir)
	globalFlags = GlobalFlags{}
	return dir
}

// runSessionCommand executa o RunE de um subcomando com formatter injetado em
// um buffer (padrão de injeção de formatter do CLI).
func runSessionCommand(t *testing.T, cmd *cobra.Command, args []string) (string, error) {
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

// writeSessionFile grava um jsonl de sessão em <dir>/.cosca/sessions/.
func writeSessionFile(t *testing.T, dir, name, content string) {
	t.Helper()
	sessDir := filepath.Join(dir, ".cosca", "sessions")
	if err := os.MkdirAll(sessDir, 0o700); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessDir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write session %s: %v", name, err)
	}
}

// sessionVoiceJSONL é a sessão de teste: meta + 3 mensagens + 1 usage.
const sessionVoiceJSONL = `{"type":"meta","id":"voice","model":"deepseek","agent":"default","created_at":"2026-08-01T22:45:34Z","updated_at":"2026-08-02T08:13:44Z"}
{"type":"message","role":"user","content":"Ola, tudo bem?","timestamp":"2026-08-02T08:13:44.21377824Z"}
{"type":"message","role":"assistant","content":"Salve, chef. O último assunto foi o CKL, chef.","timestamp":"2026-08-02T08:13:44.21377824Z"}
{"type":"usage","prompt_tokens":1,"completion_tokens":2,"total_tokens":3}
{"type":"message","role":"user","content":"Kernel, tu fala normal.","timestamp":"2026-08-02T08:13:44.21377824Z"}
`

// =============================================================================
// Registration — `session` no root
// =============================================================================

func TestSessionCommand_RegisteredInRoot(t *testing.T) {
	cmd := NewRootCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "session" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("session subcommand not registered in root command")
	}
}

func TestSessionCommand_Properties(t *testing.T) {
	cmd := NewSessionCommand()
	if cmd == nil {
		t.Fatal("NewSessionCommand returned nil")
	}
	if cmd.Use != "session" {
		t.Errorf("expected Use='session', got %q", cmd.Use)
	}
	if cmd.Short == "" || cmd.Long == "" {
		t.Error("expected non-empty Short/Long description")
	}
	expected := []string{"index", "search", "lineage", "register", "ready", "status"}
	if len(cmd.Commands()) != len(expected) {
		t.Errorf("expected %d subcommands, got %d", len(expected), len(cmd.Commands()))
	}
	registered := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("missing session subcommand: %s", name)
		}
	}
}

func TestSessionCommand_ArgConstraints(t *testing.T) {
	search := NewSessionSearchCommand()
	if err := search.Args(search, nil); err == nil {
		t.Error("search should require a phrase")
	}
	if err := search.Args(search, []string{"CKL"}); err != nil {
		t.Errorf("search with one arg should be allowed: %v", err)
	}
	if err := search.Args(search, []string{"a", "b"}); err == nil {
		t.Error("search with two args should fail")
	}

	lineage := NewSessionLineageCommand()
	if err := lineage.Args(lineage, nil); err == nil {
		t.Error("lineage should require an id")
	}
	if err := lineage.Args(lineage, []string{"cosca-voice"}); err != nil {
		t.Errorf("lineage with one arg should be allowed: %v", err)
	}

	index := NewSessionIndexCommand()
	if err := index.Args(index, []string{"extra"}); err == nil {
		t.Error("index with args should fail")
	}
}

// =============================================================================
// index + search — fluxo completo via CLI
// =============================================================================

func TestSessionIndexReportsCount(t *testing.T) {
	dir := chdirSessionTemp(t)
	writeSessionFile(t, dir, "voice.jsonl", sessionVoiceJSONL)

	out, err := runSessionCommand(t, NewSessionIndexCommand(), nil)
	if err != nil {
		t.Fatalf("session index: %v", err)
	}
	if !strings.Contains(out, "3 mensagem") {
		t.Errorf("index output missing count: %q", out)
	}
	if _, err := os.Stat(filepath.Join(dir, ".cosca", "session.db")); err != nil {
		t.Fatalf("session.db not created: %v", err)
	}
}

func TestSessionSearch_FormattedHits(t *testing.T) {
	dir := chdirSessionTemp(t)
	writeSessionFile(t, dir, "voice.jsonl", sessionVoiceJSONL)

	if _, err := runSessionCommand(t, NewSessionIndexCommand(), nil); err != nil {
		t.Fatalf("session index: %v", err)
	}

	out, err := runSessionCommand(t, NewSessionSearchCommand(), []string{"CKL"})
	if err != nil {
		t.Fatalf("session search: %v", err)
	}
	for _, want := range []string{"voice", "assistant", "CKL"} {
		if !strings.Contains(out, want) {
			t.Errorf("search output missing %q: %q", want, out)
		}
	}
}

func TestSessionSearch_NoOccurrence(t *testing.T) {
	dir := chdirSessionTemp(t)
	writeSessionFile(t, dir, "voice.jsonl", sessionVoiceJSONL)

	if _, err := runSessionCommand(t, NewSessionIndexCommand(), nil); err != nil {
		t.Fatalf("session index: %v", err)
	}

	out, err := runSessionCommand(t, NewSessionSearchCommand(), []string{"inexistente"})
	if err != nil {
		t.Fatalf("session search: %v", err)
	}
	if !strings.Contains(out, "Nenhuma ocorrência") {
		t.Errorf("expected 'Nenhuma ocorrência', got: %q", out)
	}
}

func TestSessionSearch_MalformedQueryViaCLI(t *testing.T) {
	dir := chdirSessionTemp(t)
	writeSessionFile(t, dir, "voice.jsonl", sessionVoiceJSONL)

	if _, err := runSessionCommand(t, NewSessionIndexCommand(), nil); err != nil {
		t.Fatalf("session index: %v", err)
	}

	// Aspas e dois-pontos na frase não podem quebrar a busca.
	out, err := runSessionCommand(t, NewSessionSearchCommand(), []string{`"CKL":`})
	if err != nil {
		t.Fatalf("session search on malformed query: %v", err)
	}
	if !strings.Contains(out, "CKL") {
		t.Errorf("sanitized search should still match CKL, got: %q", out)
	}
}

func TestSessionSearch_NoIndexYet(t *testing.T) {
	chdirSessionTemp(t) // sem sessões, sem índice

	out, err := runSessionCommand(t, NewSessionSearchCommand(), []string{"qualquer"})
	if err != nil {
		t.Fatalf("session search without index: %v", err)
	}
	if !strings.Contains(out, "Nenhuma ocorrência") {
		t.Errorf("expected 'Nenhuma ocorrência', got: %q", out)
	}
}

// =============================================================================
// lineage via CLI
// =============================================================================

func TestSessionLineage_Output(t *testing.T) {
	dir := chdirSessionTemp(t)
	writeSessionFile(t, dir, "root.jsonl",
		"{\"type\":\"meta\",\"id\":\"root\"}\n"+
			"{\"type\":\"message\",\"role\":\"user\",\"content\":\"raiz\",\"timestamp\":\"2026-08-02T08:00:00Z\"}\n")
	writeSessionFile(t, dir, "child.jsonl",
		"{\"type\":\"meta\",\"id\":\"child\",\"parent_session_id\":\"root\"}\n"+
			"{\"type\":\"message\",\"role\":\"user\",\"content\":\"filho\",\"timestamp\":\"2026-08-02T09:00:00Z\"}\n")

	if _, err := runSessionCommand(t, NewSessionIndexCommand(), nil); err != nil {
		t.Fatalf("session index: %v", err)
	}

	out, err := runSessionCommand(t, NewSessionLineageCommand(), []string{"root"})
	if err != nil {
		t.Fatalf("session lineage: %v", err)
	}
	for _, want := range []string{"root", "child", "2 sessão"} {
		if !strings.Contains(out, want) {
			t.Errorf("lineage output missing %q: %q", want, out)
		}
	}
}

func TestSessionLineage_NoDescendants(t *testing.T) {
	dir := chdirSessionTemp(t)
	writeSessionFile(t, dir, "orphan.jsonl",
		"{\"type\":\"meta\",\"id\":\"orphan\"}\n"+
			"{\"type\":\"message\",\"role\":\"user\",\"content\":\"sem pais\",\"timestamp\":\"2026-08-02T11:00:00Z\"}\n")

	if _, err := runSessionCommand(t, NewSessionIndexCommand(), nil); err != nil {
		t.Fatalf("session index: %v", err)
	}

	out, err := runSessionCommand(t, NewSessionLineageCommand(), []string{"orphan"})
	if err != nil {
		t.Fatalf("session lineage: %v", err)
	}
	if !strings.Contains(out, "não possui descendentes") {
		t.Errorf("expected graceful 'no descendants' message, got: %q", out)
	}
}

// compile-time guard: os argumentos dos comandos são construídos no padrão cobra.
var (
	_ *cobra.Command = NewSessionCommand()
	_ *cobra.Command = NewSessionIndexCommand()
	_ *cobra.Command = NewSessionSearchCommand()
	_ *cobra.Command = NewSessionLineageCommand()
)
