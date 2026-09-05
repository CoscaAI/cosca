//
// Tests for `cosca delegate` (internal/cli/delegate.go).
//
// Covers:
//   - Registration of `delegate` in the root command
//   - `delegate` com --yes mostra o plano (formato do Don) e emite o contrato
//     de delegação (agente, escopo, tarefa e plano resumido)
//   - `delegate` com target que não expande → erro claro
//   - `delegate --json` emite JSON válido com o contrato de delegação
//   - `delegate` aprovado registra a aprovação em approvals-{data}.md
//
// NOTE: os testes chdir() para um diretório temporário que contém uma cópia
// de um pacote real do repositório — não devem rodar em paralelo.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/trace"
)

// copyPackageToTemp copia um pacote real do repositório (todos os arquivos do
// diretório) para um diretório temporário, preservando o caminho relativo para
// que os globs do --target continuem resolvendo.
func copyPackageToTemp(t *testing.T, srcRel string) string {
	t.Helper()
	rootDir := repoRoot(t)
	src := filepath.Join(rootDir, filepath.FromSlash(srcRel))
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("read package %s: %v", srcRel, err)
	}
	dst := t.TempDir()
	rel := filepath.FromSlash(srcRel)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		full := filepath.Join(dst, rel, e.Name())
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
	writeTestFile(t, filepath.Join(dst, "go.mod"), "module coscatest\n\ngo 1.25.0\n")
	return dst
}

// =============================================================================
// Registration — `delegate` na raiz
// =============================================================================

func TestDelegateCommand_RegisteredInRoot(t *testing.T) {
	cmd := NewRootCommand()
	registered := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range []string{"delegate"} {
		if !registered[name] {
			t.Errorf("subcommand %q not registered in root command", name)
		}
	}
}

func TestDelegateCommand_Properties(t *testing.T) {
	cmd := NewDelegateCommand()
	if cmd == nil {
		t.Fatal("NewDelegateCommand returned nil")
	}
	if !strings.HasPrefix(cmd.Use, "delegate") {
		t.Errorf("expected Use to start with 'delegate', got %q", cmd.Use)
	}
	if cmd.Short != "Delegar uma tarefa com plano de execução prévio" {
		t.Errorf("unexpected Short: %q", cmd.Short)
	}
	if cmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
	targetFlag := cmd.Flags().Lookup("target")
	if targetFlag == nil {
		t.Fatal("expected --target flag")
	}
	if _, ok := targetFlag.Annotations["cobra_annotation_bash_completion_one_required_flag"]; !ok {
		t.Error("expected --target to be a required flag")
	}
	if cmd.Flags().Lookup("type") == nil {
		t.Error("expected --type flag")
	}
	if cmd.Flags().Lookup("agent") == nil {
		t.Error("expected --agent flag")
	}
	if cmd.Flags().Lookup("task") == nil {
		t.Error("expected --task flag")
	}
	if cmd.Flags().Lookup("yes") == nil {
		t.Error("expected --yes flag")
	}
	if cmd.Flags().Lookup("json") == nil {
		t.Error("expected --json flag")
	}
}

// =============================================================================
// `cosca delegate` — mostra o plano e emite o contrato (com --yes)
// =============================================================================

func TestDelegateCommand_ShowsPlan(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	dir := copyPackageToTemp(t, "internal/estimator")
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	globalFlags = GlobalFlags{}
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"delegate", "--target", "internal/estimator/*.go", "--yes"})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v\noutput:\n%s", err, buf.String())
	}
	output := buf.String()
	for _, want := range []string{
		"Plano de Execução",
		"Arquivos afetados",
		"Testes previstos",
		"Migrações",
		"Rollback",
		"Tempo estimado",
		"Risco da alteração",
		"Confiança",
		"Aprovar e delegar",
		"CONTRATO DE DELEGAÇÃO",
		"Agente executor",
		"cosca-backend",
		"Escopo",
		"Aprovação registrada em:",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("delegate output missing %q; output:\n%s", want, output)
		}
	}
}

// =============================================================================
// `cosca delegate` — target sem arquivos → erro claro
// =============================================================================

func TestDelegateCommand_NoFiles(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	globalFlags = GlobalFlags{}
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"delegate", "--target", "internal/kernel/*.go"})

	err := root.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected error when target expands to no files")
	}
	if !strings.Contains(err.Error(), "nenhum arquivo encontrado para o target: internal/kernel/*.go") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// =============================================================================
// `cosca delegate --json` — contrato de delegação em JSON válido
// =============================================================================

func TestDelegateCommand_JSON(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	dir := copyPackageToTemp(t, "internal/estimator")
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	globalFlags = GlobalFlags{}
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"delegate", "--target", "internal/estimator/*.go",
		"--task", "Integrar o estimator no fluxo do kernel", "--json", "--yes"})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v\noutput:\n%s", err, buf.String())
	}

	var contract map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &contract); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	for _, key := range []string{"status", "agent", "target", "plan"} {
		if _, ok := contract[key]; !ok {
			t.Errorf("JSON contract missing field %q; keys: %v", key, contract)
		}
	}
	if status, ok := contract["status"].(string); !ok || status != "approved" {
		t.Errorf("expected status=approved, got %v", contract["status"])
	}
	if agent, ok := contract["agent"].(string); !ok || agent != "cosca-backend" {
		t.Errorf("expected agent=cosca-backend, got %v", contract["agent"])
	}
	if task, ok := contract["task"].(string); !ok || task != "Integrar o estimator no fluxo do kernel" {
		t.Errorf("expected task set, got %v", contract["task"])
	}
	plan, ok := contract["plan"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected plan object; keys: %v", contract)
	}
	for _, key := range []string{
		"FilesAffected", "Files", "TestPackages", "TestsExpected",
		"Migrations", "EstimatedMinutes", "RiskLevel", "ConfidencePercent",
	} {
		if _, ok := plan[key]; !ok {
			t.Errorf("JSON plan missing field %q; keys: %v", key, plan)
		}
	}
	if n, ok := plan["FilesAffected"].(float64); !ok || n < 1 {
		t.Errorf("expected FilesAffected >= 1, got %v", plan["FilesAffected"])
	}
}

// =============================================================================
// `cosca delegate` — aprovação registrada no audit
// =============================================================================

func TestDelegateCommand_ApprovalRecorded(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	dir := copyPackageToTemp(t, "internal/estimator")
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	globalFlags = GlobalFlags{}
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"delegate", "--target", "internal/estimator/*.go",
		"--task", "Integrar o estimator no fluxo do kernel", "--yes"})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v\noutput:\n%s", err, buf.String())
	}

	// O arquivo diário de aprovações deve existir com o bloco canônico do
	// `cosca approve` + o bloco de delegação.
	matches, _ := filepath.Glob(filepath.Join(dir, ".cosca", "memory", "audit", "approvals-*.md"))
	if len(matches) == 0 {
		t.Fatalf("expected approval audit file under .cosca/memory/audit; output:\n%s", buf.String())
	}
	content, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}
	for _, want := range []string{
		"Aprovação do Don",
		"Data/Hora",
		"approved",
		"Plano aprovado",
		"Plano de Execução",
		"Delegação",
		"Agente executor",
		"cosca-backend",
		"Escopo",
		"Tarefa",
		"Integrar o estimator no fluxo do kernel",
	} {
		if !strings.Contains(string(content), want) {
			t.Errorf("audit file missing %q; content:\n%s", want, content)
		}
	}
}

// =============================================================================
// `cosca delegate` — rejeição (sem --yes, resposta negativa) → nada registrado
// =============================================================================

func TestDelegateCommand_Rejected(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	dir := copyPackageToTemp(t, "internal/estimator")
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	globalFlags = GlobalFlags{}
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetIn(strings.NewReader("n\n"))
	root.SetArgs([]string{"delegate", "--target", "internal/estimator/*.go"})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Aprovar e delegar?") {
		t.Errorf("expected confirmation prompt; output:\n%s", output)
	}
	if !strings.Contains(output, "plano rejeitado — nada executado") {
		t.Errorf("expected rejection message; output:\n%s", output)
	}
	if strings.Contains(output, "CONTRATO DE DELEGAÇÃO") {
		t.Errorf("rejected delegation must not emit a contract; output:\n%s", output)
	}
	if matches, _ := filepath.Glob(filepath.Join(dir, ".cosca", "memory", "audit", "approvals-*.md")); len(matches) != 0 {
		t.Errorf("rejected delegation must not write an audit file; found %v", matches)
	}
}

// =============================================================================
// `cosca delegate --trace` — Trace ID universal + ledger (opt-in, zero
// side-effects sem a flag)
// =============================================================================

func TestDelegateCommand_TraceFlag(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	// Formatter-injection pattern (mesmo do cli_cv_test.go): injeta o
	// OutputFormatter no contexto para as saídas do comando.
	runDelegate := func(t *testing.T, dir string, args ...string) string {
		t.Helper()
		if err := os.Chdir(dir); err != nil {
			t.Fatalf("chdir: %v", err)
		}
		defer func() { _ = os.Chdir(origWd) }()
		globalFlags = GlobalFlags{}
		root := NewRootCommand()
		var buf bytes.Buffer
		root.SetOut(&buf)
		root.SetErr(&buf)
		root.SetContext(newContextWithFormatter(context.Background(),
			NewOutputFormatter(&buf, OutputFormatText, false, false, false)))
		root.SetArgs(args)
		if err := root.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("ExecuteContext returned error: %v\noutput:\n%s", err, buf.String())
		}
		return buf.String()
	}

	t.Run("with --trace creates trace db with PLAN_CREATED", func(t *testing.T) {
		dir := copyPackageToTemp(t, "internal/estimator")
		output := runDelegate(t, dir, "delegate",
			"--target", "internal/estimator/*.go", "--task", "Integrar o estimator", "--yes", "--trace")
		if !strings.Contains(output, "Trace: TRACE-") {
			t.Errorf("output missing Trace ID; output:\n%s", output)
		}

		storePath := filepath.Join(dir, ".cosca", "trace.db")
		if _, err := os.Stat(storePath); err != nil {
			t.Fatalf("expected trace.db to be created: %v", err)
		}
		store, err := trace.NewStore(storePath)
		if err != nil {
			t.Fatalf("open trace store: %v", err)
		}
		defer func() { _ = store.Close() }()

		events, err := store.Latest(100)
		if err != nil {
			t.Fatalf("latest trace events: %v", err)
		}
		actions := make(map[string]bool)
		for _, e := range events {
			actions[e.Action] = true
		}
		for _, want := range []string{"PLAN_CREATED", "APPROVED", "DELEGATED"} {
			if !actions[want] {
				t.Errorf("trace db missing %s event; events: %+v", want, events)
			}
		}
	})

	t.Run("without --trace no trace db is created", func(t *testing.T) {
		dir := copyPackageToTemp(t, "internal/estimator")
		runDelegate(t, dir, "delegate",
			"--target", "internal/estimator/*.go", "--yes")
		if _, err := os.Stat(filepath.Join(dir, ".cosca", "trace.db")); !os.IsNotExist(err) {
			t.Errorf("expected no trace.db without --trace (stat err=%v)", err)
		}
	})
}
