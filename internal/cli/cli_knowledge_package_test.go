//
// Tests for `cosca knowledge add` + `cosca knowledge packages`
// (internal/cli/knowledge.go — Knowledge Package manifesto).
//
// Cobre:
//   - Registro de `add` e `packages` (list/show/status) no grupo knowledge
//   - `knowledge add prisma`: manifesto criado em .cosca/knowledge/packages/
//     com status "manifest", ecosystem typescript, nota KNOWLEDGE ≠ DEPENDENCY
//   - `knowledge add github:pgx/pgx`: id pgx, repository pgx/pgx, ecosystem go
//   - Duplicado → erro (manifesto existente nunca é sobrescrito)
//   - `knowledge packages list` / `show <id>` / `status <id>`
//   - Formatter injection pattern (newContextWithFormatter) + SetIn para o
//     prompt de versões não travar nos testes (default ["*"])
//
// NOTE: os testes fazem chdir() e NÃO rodam em paralelo.
//

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/knowledge"
)

// executeKnowledgePackage roda `cosca knowledge <args...>` com cwd em root e
// stdin injetado (default "*" no prompt de versões). Mesmo shape do
// executeKnowledgeStatus (cli_knowledge_status_test.go).
func executeKnowledgePackage(t *testing.T, root, stdin string, args ...string) (string, error) {
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
	cmd.SetIn(strings.NewReader(stdin))
	formatter := NewOutputFormatter(buf, OutputFormatText, false, false, true) // noColor
	cmd.SetContext(newContextWithFormatter(context.Background(), formatter))
	cmd.SetArgs(append([]string{"knowledge"}, args...))
	err = cmd.Execute()
	return buf.String(), err
}

// readManifest lê o manifesto <id>.json do diretório raiz.
func readManifest(t *testing.T, root, id string) *knowledge.KnowledgePackage {
	t.Helper()
	data, err := os.ReadFile(root + "/.cosca/knowledge/packages/" + id + ".json")
	if err != nil {
		t.Fatalf("ler manifesto %s: %v", id, err)
	}
	var p knowledge.KnowledgePackage
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("parsear manifesto %s: %v", id, err)
	}
	return &p
}

// =============================================================================
// Registration
// =============================================================================

func TestKnowledgePackageCommand_RegisteredInKnowledge(t *testing.T) {
	cmd := NewKnowledgeCommand()
	found := map[string]bool{}
	for _, sub := range cmd.Commands() {
		found[sub.Name()] = true
	}
	for _, name := range []string{"add", "packages"} {
		if !found[name] {
			t.Errorf("knowledge subcommand %q não registrado", name)
		}
	}
}

func TestKnowledgePackagesCommand_Subcommands(t *testing.T) {
	cmd := NewKnowledgePackagesCommand()
	if cmd == nil {
		t.Fatal("NewKnowledgePackagesCommand returned nil")
	}
	if cmd.Use != "packages" {
		t.Errorf("expected Use='packages', got %q", cmd.Use)
	}
	expected := map[string]bool{"list": false, "show": false, "status": false}
	for _, sub := range cmd.Commands() {
		expected[sub.Name()] = true
	}
	for name, ok := range expected {
		if !ok {
			t.Errorf("packages subcommand %q não registrado", name)
		}
	}
}

// =============================================================================
// `knowledge add`
// =============================================================================

func TestKnowledgeAdd_CreatesManifest(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()

	out, err := executeKnowledgePackage(t, root, "\n", "add", "prisma")
	if err != nil {
		t.Fatalf("knowledge add prisma: %v\n%s", err, out)
	}

	// Draft mostrado.
	for _, want := range []string{"Knowledge Package detectado", "prisma", "typescript", "library"} {
		if !strings.Contains(out, want) {
			t.Errorf("draft deveria conter %q; output:\n%s", want, out)
		}
	}

	// Nota de separação explícita — KNOWLEDGE ≠ DEPENDENCY ≠ CODE.
	if !strings.Contains(out, "NADA foi instalado no projeto") {
		t.Errorf("output deveria dizer que nada foi instalado; output:\n%s", out)
	}
	if !strings.Contains(out, "KNOWLEDGE ≠ DEPENDENCY ≠ CODE") {
		t.Errorf("output deveria citar a separação KNOWLEDGE ≠ DEPENDENCY ≠ CODE; output:\n%s", out)
	}

	// Manifesto persistido.
	p := readManifest(t, root, "prisma")
	if p.Status != knowledge.PackageStatusManifest {
		t.Errorf("status deveria ser manifest, got %q", p.Status)
	}
	if p.Ecosystem != "typescript" {
		t.Errorf("ecosystem deveria ser typescript, got %q", p.Ecosystem)
	}
	if p.Kind != knowledge.PackageKindLibrary {
		t.Errorf("kind deveria ser library, got %q", p.Kind)
	}
	if p.KnowledgeLevel != knowledge.PackageKnowledgeNone {
		t.Errorf("knowledge_level deveria ser none, got %q", p.KnowledgeLevel)
	}
	if len(p.Versions) != 1 || p.Versions[0] != "*" {
		t.Errorf("versões default deveriam ser [*], got %v", p.Versions)
	}
	if p.Repository != "" {
		t.Errorf("prisma simples não deveria ter repository, got %q", p.Repository)
	}
}

func TestKnowledgeAdd_JavaEcosystem(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()

	for _, lib := range []string{"quarkus", "flyway"} {
		out, err := executeKnowledgePackage(t, root, "\n", "add", lib)
		if err != nil {
			t.Fatalf("knowledge add %s: %v\n%s", lib, err, out)
		}
		if !strings.Contains(out, "java") {
			t.Errorf("draft de %s deveria mostrar ecosystem java; output:\n%s", lib, out)
		}
		p := readManifest(t, root, lib)
		if p.ID != lib {
			t.Errorf("id deveria ser %s, got %q", lib, p.ID)
		}
		if p.Ecosystem != "java" {
			t.Errorf("ecosystem de %s deveria ser java, got %q", lib, p.Ecosystem)
		}
		if p.Kind != knowledge.PackageKindLibrary {
			t.Errorf("kind de %s deveria ser library, got %q", lib, p.Kind)
		}
	}
}

func TestKnowledgeAdd_RepoForm(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()

	out, err := executeKnowledgePackage(t, root, "6.x,7.x\n", "add", "github:pgx/pgx")
	if err != nil {
		t.Fatalf("knowledge add github:pgx/pgx: %v\n%s", err, out)
	}

	p := readManifest(t, root, "pgx")
	if p.Repository != "pgx/pgx" {
		t.Errorf("repository deveria ser pgx/pgx, got %q", p.Repository)
	}
	if p.Ecosystem != "go" {
		t.Errorf("ecosystem deveria ser go, got %q", p.Ecosystem)
	}
	if len(p.Versions) != 2 || p.Versions[0] != "6.x" || p.Versions[1] != "7.x" {
		t.Errorf("versões deveriam refletir o input, got %v", p.Versions)
	}
	if !strings.Contains(out, "pgx/pgx") {
		t.Errorf("draft deveria mostrar o repository; output:\n%s", out)
	}
}

func TestKnowledgeAdd_DuplicateRefuses(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()

	if _, err := executeKnowledgePackage(t, root, "\n", "add", "prisma"); err != nil {
		t.Fatalf("primeiro add: %v", err)
	}
	_, err := executeKnowledgePackage(t, root, "\n", "add", "prisma")
	if err == nil || !strings.Contains(err.Error(), "já registrado") {
		t.Fatalf("add duplicado deveria recusar, got %v", err)
	}
}

func TestKnowledgeAdd_EmptyArg(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()

	if _, err := executeKnowledgePackage(t, root, "\n", "add", ""); err == nil {
		t.Fatal("add com argumento vazio deveria falhar")
	}
}

// =============================================================================
// `knowledge packages list` / `show` / `status`
// =============================================================================

func TestKnowledgePackages_ListEmpty(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()

	out, err := executeKnowledgePackage(t, root, "", "packages", "list")
	if err != nil {
		t.Fatalf("packages list (empty): %v", err)
	}
	if !strings.Contains(out, "Nenhum Knowledge Package registrado") {
		t.Errorf("esperava estado vazio, got: %q", out)
	}
}

func TestKnowledgePackages_ListShowStatus(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()

	if _, err := executeKnowledgePackage(t, root, "\n", "add", "prisma"); err != nil {
		t.Fatalf("add prisma: %v", err)
	}
	if _, err := executeKnowledgePackage(t, root, "\n", "add", "github:pgx/pgx"); err != nil {
		t.Fatalf("add pgx: %v", err)
	}

	// list → tabela com prisma e pgx.
	out, err := executeKnowledgePackage(t, root, "", "packages", "list")
	if err != nil {
		t.Fatalf("packages list: %v", err)
	}
	for _, want := range []string{"prisma", "pgx", "typescript", "go", "manifest", "ID"} {
		if !strings.Contains(out, want) {
			t.Errorf("list deveria conter %q; output:\n%s", want, out)
		}
	}

	// show prisma → manifesto completo.
	out, err = executeKnowledgePackage(t, root, "", "packages", "show", "prisma")
	if err != nil {
		t.Fatalf("packages show prisma: %v", err)
	}
	for _, want := range []string{"Knowledge Package prisma", "Ecosystem", "typescript", "Versões", "Fontes", "Status"} {
		if !strings.Contains(out, want) {
			t.Errorf("show deveria conter %q; output:\n%s", want, out)
		}
	}

	// show pgx → repository presente.
	out, err = executeKnowledgePackage(t, root, "", "packages", "show", "pgx")
	if err != nil {
		t.Fatalf("packages show pgx: %v", err)
	}
	if !strings.Contains(out, "pgx/pgx") {
		t.Errorf("show pgx deveria conter o repository; output:\n%s", out)
	}

	// show inexistente → erro.
	if _, err := executeKnowledgePackage(t, root, "", "packages", "show", "next"); err == nil {
		t.Fatal("show de pacote inexistente deveria falhar")
	}

	// status prisma → resumo.
	out, err = executeKnowledgePackage(t, root, "", "packages", "status", "prisma")
	if err != nil {
		t.Fatalf("packages status prisma: %v", err)
	}
	for _, want := range []string{"Status — Knowledge Package prisma", "manifest", "none", "Adquirido em", "nunca", "Artefatos"} {
		if !strings.Contains(out, want) {
			t.Errorf("status deveria conter %q; output:\n%s", want, out)
		}
	}

	// JSON list.
	out, err = executeKnowledgePackage(t, root, "", "packages", "list", "--json")
	if err != nil {
		t.Fatalf("packages list --json: %v", err)
	}
	if !strings.Contains(out, `"id": "prisma"`) {
		t.Errorf("json list deveria conter prisma; output:\n%s", out)
	}
}

// compile-time guards.
var (
	_ = NewKnowledgeAddCommand()
	_ = NewKnowledgePackagesCommand()
	_ = NewKnowledgePackagesListCommand()
	_ = NewKnowledgePackagesShowCommand()
	_ = NewKnowledgePackagesStatusCommand()
)
