//
// Tests for the `cosca department` command tree (internal/cli/department.go).
//
// Covers:
//   - Registration of `department` (and its 5 subcommands) in the root command
//   - Command properties and arg constraints
//   - `department ask` output (DM-XXXX + ThreadID)
//   - `department answer` / `department resolve` append to the thread
//   - `department thread` audit-trail output
//   - `department list` output
//   - Unknown department → error pt-BR
//   - Store file created in the project (.cosca/department.db)
//
// NOTE: tests that chdir() must NOT run in parallel — they mutate the
// process working directory.

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// =============================================================================
// Registration — `department` in the root command
// =============================================================================

func TestDepartmentCommand_RegisteredInRoot(t *testing.T) {
	cmd := NewRootCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "department" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("department subcommand not registered in root command")
	}
}

func TestDepartmentCommand_Properties(t *testing.T) {
	cmd := NewDepartmentCommand()
	if cmd == nil {
		t.Fatal("NewDepartmentCommand returned nil")
	}
	if cmd.Use != "department" {
		t.Errorf("expected Use='department', got %q", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
	if cmd.Long == "" {
		t.Error("expected non-empty Long description")
	}

	expected := []string{"ask", "answer", "resolve", "thread", "list"}
	if len(cmd.Commands()) != len(expected) {
		t.Errorf("expected %d subcommands, got %d", len(expected), len(cmd.Commands()))
	}
	registered := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("missing department subcommand: %s", name)
		}
	}
}

func TestDepartmentCommand_ArgConstraints(t *testing.T) {
	ask := NewDepartmentAskCommand()
	if err := ask.Args(ask, []string{"extra"}); err == nil {
		t.Error("ask with args should fail")
	}
	if err := ask.Args(ask, nil); err != nil {
		t.Errorf("ask with no args should be allowed: %v", err)
	}

	answer := NewDepartmentAnswerCommand()
	if err := answer.Args(answer, nil); err == nil {
		t.Error("answer with no args should fail")
	}
	if err := answer.Args(answer, []string{"release-approval:20260802"}); err != nil {
		t.Errorf("answer with one arg should be allowed: %v", err)
	}

	resolve := NewDepartmentResolveCommand()
	if err := resolve.Args(resolve, nil); err == nil {
		t.Error("resolve with no args should fail")
	}

	thread := NewDepartmentThreadCommand()
	if err := thread.Args(thread, nil); err == nil {
		t.Error("thread with no args should fail")
	}

	list := NewDepartmentListCommand()
	if err := list.Args(list, nil); err != nil {
		t.Errorf("list with no args should be allowed: %v", err)
	}
}

// =============================================================================
// executeDepartment runs a `department` subcommand with cwd set to root.
// =============================================================================

func executeDepartment(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatal(err)
		}
	}()

	cmd := NewRootCommand()
	cmd.PersistentPreRunE = nil
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	formatter := NewOutputFormatter(buf, OutputFormatText, false, false, false)
	cmd.SetContext(newContextWithFormatter(context.Background(), formatter))
	cmd.SetArgs(append([]string{"department"}, args...))
	err = cmd.Execute()
	return buf.String(), err
}

// fakeDepartmentTree builds a fake .cosca project tree for CLI tests.
func fakeDepartmentTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeCLIFile(t, root, ".cosca/config.yaml", "cache:\n  enabled: true\n")
	return root
}

// =============================================================================
// `department ask`
// =============================================================================

func TestDepartmentAsk_Output(t *testing.T) {
	root := fakeDepartmentTree(t)
	out, err := executeDepartment(t, root, "ask", "--from", "executive", "--to", "security", "--topic", "release-approval", "--msg", "podemos liberar?")
	if err != nil {
		t.Fatalf("department ask: %v", err)
	}
	re := regexp.MustCompile(`DM-\d{8}-[0-9A-F]{4}`)
	if !re.MatchString(out) {
		t.Errorf("output sem ID de mensagem DM-XXXX: %q", out)
	}
	if !strings.Contains(out, "release-approval:"+todayUTC()) {
		t.Errorf("output sem a thread (<topic>:<data>): %q", out)
	}
	if !strings.Contains(out, "Pergunta enviada") || !strings.Contains(out, "Thread") {
		t.Errorf("output sem confirmação/thread: %q", out)
	}
}

func TestDepartmentAsk_Validation(t *testing.T) {
	root := fakeDepartmentTree(t)
	// Sem --from/--to → erro.
	if _, err := executeDepartment(t, root, "ask", "--topic", "t", "--msg", "m"); err == nil {
		t.Error("ask sem --from/--to deveria falhar")
	}
	// Sem --topic → erro.
	if _, err := executeDepartment(t, root, "ask", "--from", "developer", "--to", "security", "--msg", "m"); err == nil {
		t.Error("ask sem --topic deveria falhar")
	}
	// Sem --msg → erro.
	if _, err := executeDepartment(t, root, "ask", "--from", "developer", "--to", "security", "--topic", "t"); err == nil {
		t.Error("ask sem --msg deveria falhar")
	}
	// Departamento desconhecido → erro pt-BR.
	if _, err := executeDepartment(t, root, "ask", "--from", "finance", "--to", "security", "--topic", "t", "--msg", "m"); err == nil || !strings.Contains(err.Error(), "departamento desconhecido") {
		t.Errorf("ask com departamento desconhecido deveria falhar em pt-BR, got: %v", err)
	}
}

// =============================================================================
// `department answer` / `department resolve` + thread audit trail
// =============================================================================

func TestDepartmentConversation_FullFlow(t *testing.T) {
	root := fakeDepartmentTree(t)

	// 1. Executive pergunta a Security.
	askOut, err := executeDepartment(t, root, "ask", "--from", "executive", "--to", "security", "--topic", "release-approval", "--msg", "podemos liberar a versão?")
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	thread := "release-approval:" + todayUTC()
	if !strings.Contains(askOut, thread) {
		t.Fatalf("ask sem a thread esperada %q: %q", thread, askOut)
	}

	// 2. Security responde com os riscos.
	if out, err := executeDepartment(t, root, "answer", thread, "--from", "security", "--msg", "há 2 riscos críticos"); err != nil || !strings.Contains(out, "Resposta") {
		t.Fatalf("answer: %v out=%q", err, out)
	}
	// 3. Developer: um deles já foi corrigido.
	if _, err := executeDepartment(t, root, "answer", thread, "--from", "developer", "--msg", "um deles já foi corrigido"); err != nil {
		t.Fatalf("answer (developer): %v", err)
	}
	// 4. Security valida.
	if _, err := executeDepartment(t, root, "answer", thread, "--from", "security", "--msg", "validação passou"); err != nil {
		t.Fatalf("answer (security): %v", err)
	}
	// 5. Executive resolve.
	if out, err := executeDepartment(t, root, "resolve", thread, "--from", "executive", "--msg", "release aprovado"); err != nil || !strings.Contains(out, "Decisão") {
		t.Fatalf("resolve: %v out=%q", err, out)
	}

	// Trilha de auditoria completa — todas as 5 mensagens, em ordem.
	out, err := executeDepartment(t, root, "thread", thread)
	if err != nil {
		t.Fatalf("thread: %v", err)
	}
	if !strings.Contains(out, "THREAD "+thread) {
		t.Errorf("thread sem header: %q", out)
	}
	for _, want := range []string{
		"podemos liberar a versão?",
		"há 2 riscos críticos",
		"um deles já foi corrigido",
		"validação passou",
		"release aprovado",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("thread sem a mensagem %q: %q", want, out)
		}
	}
	if !strings.Contains(out, "Mensagens") || !strings.Contains(out, "5") {
		t.Errorf("thread sem a contagem de mensagens: %q", out)
	}
}

func TestDepartmentAnswer_Validation(t *testing.T) {
	root := fakeDepartmentTree(t)
	thread := "release-approval:" + todayUTC()
	// Thread inexistente → erro pt-BR.
	if _, err := executeDepartment(t, root, "answer", thread, "--from", "security", "--msg", "m"); err == nil || !strings.Contains(err.Error(), "não encontrada") {
		t.Errorf("answer de thread inexistente deveria falhar em pt-BR, got: %v", err)
	}
	// Sem --from → erro.
	if _, err := executeDepartment(t, root, "answer", thread, "--msg", "m"); err == nil {
		t.Error("answer sem --from deveria falhar")
	}
	// Sem --msg → erro.
	if _, err := executeDepartment(t, root, "answer", thread, "--from", "security"); err == nil {
		t.Error("answer sem --msg deveria falhar")
	}
}

// =============================================================================
// `department list`
// =============================================================================

func TestDepartmentList_EmptyAndPopulated(t *testing.T) {
	root := fakeDepartmentTree(t)

	out, err := executeDepartment(t, root, "list")
	if err != nil {
		t.Fatalf("department list (empty): %v", err)
	}
	if !strings.Contains(out, "Nenhuma conversa") {
		t.Errorf("expected empty-state warning, got: %q", out)
	}

	thread := "release-approval:" + todayUTC()
	if _, err := executeDepartment(t, root, "ask", "--from", "developer", "--to", "security", "--topic", "release-approval", "--msg", "podemos liberar?"); err != nil {
		t.Fatalf("ask: %v", err)
	}
	out, err = executeDepartment(t, root, "list", "--limit", "5")
	if err != nil {
		t.Fatalf("department list: %v", err)
	}
	if !strings.Contains(out, thread) || !strings.Contains(out, "podemos liberar?") {
		t.Errorf("list sem a mensagem registrada: %q", out)
	}
}

// =============================================================================
// `department` with no subcommand → help
// =============================================================================

func TestDepartmentCommand_NoSubcommand_ShowsHelp(t *testing.T) {
	root := fakeDepartmentTree(t)
	out, err := executeDepartment(t, root)
	if err != nil {
		t.Fatalf("department with no subcommand should show help without error: %v", err)
	}
	if !strings.Contains(out, "ask") || !strings.Contains(out, "resolve") {
		t.Errorf("expected help listing subcommands, got: %q", out)
	}
}

// O ledger append-only deve persistir em .cosca/department.db dentro do projeto.
func TestDepartment_StoreFileCreatedInProject(t *testing.T) {
	root := fakeDepartmentTree(t)
	if _, err := executeDepartment(t, root, "ask", "--from", "developer", "--to", "security", "--topic", "release-approval", "--msg", "podemos liberar?"); err != nil {
		t.Fatalf("department ask: %v", err)
	}
	storePath := filepath.Join(root, ".cosca", "department.db")
	if _, err := os.Stat(storePath); err != nil {
		t.Errorf("department.db não criado no projeto: %v", err)
	}
}

// todayUTC devolve a data de hoje no formato YYYYMMDD (UTC) — usado para
// compor a thread esperada nos testes (paridade com department.ThreadKey).
func todayUTC() string {
	return time.Now().UTC().Format("20060102")
}

// compile-time guard: the command constructors must match the cobra contract.
var (
	_ *cobra.Command = NewDepartmentCommand()
	_ *cobra.Command = NewDepartmentAskCommand()
	_ *cobra.Command = NewDepartmentAnswerCommand()
	_ *cobra.Command = NewDepartmentResolveCommand()
	_ *cobra.Command = NewDepartmentThreadCommand()
	_ *cobra.Command = NewDepartmentListCommand()
)
