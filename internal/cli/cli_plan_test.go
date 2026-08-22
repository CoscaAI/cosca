//
// Tests for `cosca plan` and `cosca approve` (internal/cli/plan.go e
// internal/cli/approve.go).
//
// Covers:
//   - Registration of `plan` and `approve` in the root command
//   - `plan` generates the Don's plan (text) against a real package
//   - `plan` fails with a clear error when the target expands to no files
//   - `plan --json` emits valid JSON with the plan fields
//   - `approve` flow: confirm with "y" → runs `go test` on a small package →
//     records the approval in the audit markdown file
//   - `approve --dry-run` shows what would be done without executing anything
//   - `approve` decline ("n") cancels without running tests or writing audit
//
// NOTE: tests that chdir() must NOT run in parallel — they mutate the
// process working directory.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot localiza a raiz do repositório (onde está o go.mod) a partir do
// diretório do pacote de teste.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}

// =============================================================================
// Registration — `plan` e `approve` na raiz
// =============================================================================

func TestPlanApproveCommands_RegisteredInRoot(t *testing.T) {
	cmd := NewRootCommand()
	registered := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range []string{"plan", "approve"} {
		if !registered[name] {
			t.Errorf("subcommand %q not registered in root command", name)
		}
	}
}

func TestPlanCommand_Properties(t *testing.T) {
	cmd := NewPlanCommand()
	if cmd == nil {
		t.Fatal("NewPlanCommand returned nil")
	}
	if !strings.HasPrefix(cmd.Use, "plan") {
		t.Errorf("expected Use to start with 'plan', got %q", cmd.Use)
	}
	if cmd.Short != "Estimar um plano de execução antes da aprovação" {
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
	if !cmd.Flags().Changed("type") && cmd.Flags().Lookup("type") == nil {
		t.Error("expected --type flag")
	}
	if cmd.Flags().Lookup("agent") == nil {
		t.Error("expected --agent flag")
	}
	if cmd.Flags().Lookup("json") == nil {
		t.Error("expected --json flag")
	}
}

func TestApproveCommand_Properties(t *testing.T) {
	cmd := NewApproveCommand()
	if cmd == nil {
		t.Fatal("NewApproveCommand returned nil")
	}
	if !strings.HasPrefix(cmd.Use, "approve") {
		t.Errorf("expected Use to start with 'approve', got %q", cmd.Use)
	}
	if cmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
	if cmd.Flags().Lookup("plan") == nil {
		t.Error("expected --plan flag")
	}
	if cmd.Flags().Lookup("dry-run") == nil {
		t.Error("expected --dry-run flag")
	}
}

// =============================================================================
// `cosca plan` — gera o plano
// =============================================================================

func TestPlanCommand_GeneratesPlan(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	rootDir := repoRoot(t)
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir to repo root: %v", err)
	}

	globalFlags = GlobalFlags{}
	cmd := NewRootCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"plan", "--target", "internal/kernel/*.go"})

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}
	output := buf.String()
	for _, want := range []string{
		"Plano de Execução",
		"Arquivos afetados",
		"Testes previstos",
		"Migrações",
		"Rollback",
		"Tempo estimado",
		"Confiança",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("plan output missing %q; output:\n%s", want, output)
		}
	}
}

// =============================================================================
// `cosca plan` — target sem arquivos → erro claro
// =============================================================================

func TestPlanCommand_NoFiles(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	globalFlags = GlobalFlags{}
	cmd := NewRootCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"plan", "--target", "internal/kernel/*.go"})

	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected error when target expands to no files")
	}
	if !strings.Contains(err.Error(), "nenhum arquivo encontrado para o target: internal/kernel/*.go") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// =============================================================================
// `cosca plan --json` — JSON válido com campos
// =============================================================================

func TestPlanCommand_JSON(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	rootDir := repoRoot(t)
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir to repo root: %v", err)
	}

	globalFlags = GlobalFlags{}
	cmd := NewRootCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"plan", "--target", "internal/kernel/*.go", "--json"})

	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}

	var plan map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &plan); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
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
// `cosca approve` — fluxo completo (y → testes → audit)
// =============================================================================

// approvalTestPlanJSON é um plano no formato emitido por `cosca plan --json`
// (ExecutionPlan sem json tags → chaves PascalCase).
const approvalTestPlanJSON = `{
	"FilesAffected": 1,
	"Files": ["tiny/tiny.go"],
	"TestsExpected": 1,
	"TestPackages": ["tiny"],
	"Migrations": [],
	"RollbackAvailable": true,
	"RollbackDetail": "git revert",
	"EstimatedMinutes": 5,
	"RiskLevel": "baixo",
	"ConfidencePercent": 90,
	"Method": "heurístico"
}`

// createApprovalTestModule monta um módulo Go mínimo com um pacote pequeno e
// um teste rápido (para que `go test` não dependa de rede nem seja lento).
func createApprovalTestModule(t *testing.T, dir string) {
	t.Helper()
	writeTestFile(t, filepath.Join(dir, "go.mod"), "module coscatest\n\ngo 1.25.0\n")
	writeTestFile(t, filepath.Join(dir, "tiny", "tiny.go"),
		"package tiny\n\n// Answer responde 42.\nfunc Answer() int { return 42 }\n")
	writeTestFile(t, filepath.Join(dir, "tiny", "tiny_test.go"),
		"package tiny\n\nimport \"testing\"\n\nfunc TestAnswer(t *testing.T) {\n\tif Answer() != 42 {\n\t\tt.Fatal(\"wrong answer\")\n\t}\n}\n")
}

func TestApproveCommand_Flow(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	createApprovalTestModule(t, tmpDir)
	planPath := filepath.Join(tmpDir, "plan.json")
	writeTestFile(t, planPath, approvalTestPlanJSON)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	globalFlags = GlobalFlags{}
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetIn(strings.NewReader("y\n"))
	root.SetArgs([]string{"approve", "--plan", planPath})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v\noutput:\n%s", err, buf.String())
	}

	output := buf.String()
	for _, want := range []string{
		"Plano de Execução",
		"Confirmar aprovação?",
		"Executando testes dos pacotes afetados",
		"ok",
		"Testes passaram",
		"Aprovação registrada em:",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("approve output missing %q; output:\n%s", want, output)
		}
	}

	// O arquivo de auditoria deve existir e conter timestamp, plano e decisão.
	matches, _ := filepath.Glob(filepath.Join(tmpDir, ".cosca", "memory", "audit", "approvals-*.md"))
	if len(matches) == 0 {
		t.Fatalf("expected approval audit file under .cosca/memory/audit; output:\n%s", output)
	}
	content, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}
	for _, want := range []string{
		"Aprovação do Don",
		"Data/Hora",
		"approved",
		"passed",
		"Plano aprovado",
		"Plano de Execução",
	} {
		if !strings.Contains(string(content), want) {
			t.Errorf("audit file missing %q; content:\n%s", want, content)
		}
	}
}

func TestApproveCommand_DryRun(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.json")
	writeTestFile(t, planPath, approvalTestPlanJSON)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	globalFlags = GlobalFlags{}
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"approve", "--plan", planPath, "--dry-run"})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}

	output := buf.String()
	for _, want := range []string{
		"DRY-RUN",
		"go ./tiny/...",
		"Registro de auditoria:",
		"approvals-",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("dry-run output missing %q; output:\n%s", want, output)
		}
	}

	// Dry-run não pode executar nada nem gravar auditoria.
	if matches, _ := filepath.Glob(filepath.Join(tmpDir, ".cosca", "memory", "audit", "approvals-*.md")); len(matches) != 0 {
		t.Errorf("dry-run must not write an audit file; found %v", matches)
	}
}

func TestApproveCommand_Decline(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plan.json")
	writeTestFile(t, planPath, approvalTestPlanJSON)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	globalFlags = GlobalFlags{}
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetIn(strings.NewReader("n\n"))
	root.SetArgs([]string{"approve", "--plan", planPath})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Aprovação cancelada pelo Don") {
		t.Errorf("expected cancellation message; output:\n%s", output)
	}
	if strings.Contains(output, "Executando testes") {
		t.Errorf("declined approval must not run tests; output:\n%s", output)
	}
	if matches, _ := filepath.Glob(filepath.Join(tmpDir, ".cosca", "memory", "audit", "approvals-*.md")); len(matches) != 0 {
		t.Errorf("declined approval must not write an audit file; found %v", matches)
	}
}

// =============================================================================
// Helpers internos
// =============================================================================

func TestGoTestArgs(t *testing.T) {
	projectDir := "/proj"
	tests := []struct {
		name     string
		packages []string
		want     []string
	}{
		{"relative", []string{"tiny"}, []string{"./tiny/..."}},
		{"absolute inside project", []string{"/proj/internal/kernel"}, []string{"./internal/kernel/..."}},
		{"absolute outside project", []string{"/other/pkg"}, nil},
		{"empty and blank", []string{"", "  "}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := goTestArgs(projectDir, tt.packages)
			if len(got) != len(tt.want) {
				t.Fatalf("goTestArgs = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("goTestArgs[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
