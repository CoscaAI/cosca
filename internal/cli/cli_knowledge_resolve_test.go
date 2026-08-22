//
// Tests for `cosca knowledge resolve <task> [--dir .]` — Tarefa → Knowledge
// Resolver → Gap Detection (aquisição adaptativa).
//
// Cobre:
//   - Registro do subcomando no grupo knowledge
//   - Resolução suficiente: dep verified + versão coberta →
//     "Conhecimento suficiente — continua"
//   - Resolução com lacuna: prisma Versions ["6.0"] + dep prisma@6.1 →
//     tabela (pacote, versão, API, razão, fontes) + próxima ação "adquirir"
//   - --json (ResolveResult completo)
//   - Formatter injection pattern (newContextWithFormatter)
//
// NOTE: os testes fazem chdir() e NÃO rodam em paralelo.
//

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/knowledge"
)

// resolveFixture monta o projeto fake com package.json e o manifesto prisma.
// prismaVersions controla a família rastreada (["6.x"] → suficiente,
// ["6.0"] → lacuna 6.1).
func resolveFixture(t *testing.T, root string, prismaVersions []string) {
	t.Helper()
	pkgJSON := `{"name":"fake-project","dependencies":{"prisma":"6.1"}}`
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(pkgJSON), 0o600); err != nil {
		t.Fatalf("escrever package.json: %v", err)
	}
	if err := knowledge.NewPackageStore(root).Add(knowledge.KnowledgePackage{
		ID: "prisma", Kind: knowledge.PackageKindLibrary, Ecosystem: "typescript",
		Versions:       prismaVersions,
		Sources:        []string{"official-docs"},
		KnowledgeLevel: knowledge.PackageKnowledgeValidated,
		Status:         knowledge.PackageStatusValidated,
	}); err != nil {
		t.Fatalf("Add manifesto prisma: %v", err)
	}
}

// =============================================================================
// Registration
// =============================================================================

func TestKnowledgeResolveCommand_RegisteredInKnowledge(t *testing.T) {
	cmd := NewKnowledgeCommand()
	found := map[string]bool{}
	for _, sub := range cmd.Commands() {
		found[sub.Name()] = true
	}
	if !found["resolve"] {
		t.Error("knowledge subcommand \"resolve\" não registrado")
	}
}

// =============================================================================
// `knowledge resolve` — suficiente
// =============================================================================

func TestKnowledgeResolve_Sufficient(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()
	resolveFixture(t, root, []string{"6.x"})

	out, err := executeKnowledgePackage(t, root, "", "resolve", "create transaction")
	if err != nil {
		t.Fatalf("knowledge resolve: %v\n%s", err, out)
	}

	if !strings.Contains(out, "Knowledge Resolver —") {
		t.Errorf("output deveria ter o cabeçalho do resolver; output:\n%s", out)
	}
	if !strings.Contains(out, "Conhecimento suficiente — continua") {
		t.Errorf("output suficiente errado; output:\n%s", out)
	}
}

// =============================================================================
// `knowledge resolve` — lacuna (prisma 6.0 ✓ / 6.1 ✗)
// =============================================================================

func TestKnowledgeResolve_Gap(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()
	resolveFixture(t, root, []string{"6.0"})

	out, err := executeKnowledgePackage(t, root, "", "resolve", "create transaction")
	if err != nil {
		t.Fatalf("knowledge resolve (lacuna): %v\n%s", err, out)
	}

	// Cabeçalhos da tabela.
	for _, want := range []string{"Pacote", "Versão", "API", "Razão", "Fontes sugeridas"} {
		if !strings.Contains(out, want) {
			t.Errorf("tabela deveria conter %q; output:\n%s", want, out)
		}
	}

	// A lacuna: prisma 6.1 / transaction + razão + fontes.
	for _, want := range []string{
		"prisma", "6.1", "transaction",
		"não coberto", "official-docs", "official-repository", "release-notes",
		"adquirir: cosca knowledge add prisma",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output deveria conter %q; output:\n%s", want, out)
		}
	}
}

// =============================================================================
// `knowledge resolve --json`
// =============================================================================

func TestKnowledgeResolve_JSON(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()
	resolveFixture(t, root, []string{"6.0"})

	out, err := executeKnowledgePackage(t, root, "", "resolve", "create transaction", "--json")
	if err != nil {
		t.Fatalf("knowledge resolve --json: %v\n%s", err, out)
	}

	var res knowledge.ResolveResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("--json deveria emitir ResolveResult válido: %v\n%s", err, out)
	}
	if res.Sufficient {
		t.Errorf("resolução com 6.0 não deveria ser suficiente: %+v", res)
	}
	if len(res.Gaps) != 1 {
		t.Fatalf("JSON deveria ter 1 gap, got %d", len(res.Gaps))
	}
	g := res.Gaps[0]
	if g.PackageID != "prisma" || g.Version != "6.1" {
		t.Errorf("gap JSON errado: %+v", g)
	}
	if !strings.HasPrefix(res.NextAction, "adquirir: cosca knowledge add prisma") {
		t.Errorf("next_action JSON errado: %q", res.NextAction)
	}

	// Suficiente em JSON.
	root2 := t.TempDir()
	resolveFixture(t, root2, []string{"6.x"})
	out, err = executeKnowledgePackage(t, root2, "", "resolve", "create transaction", "--json")
	if err != nil {
		t.Fatalf("knowledge resolve --json (suficiente): %v\n%s", err, out)
	}
	var ok knowledge.ResolveResult
	if err := json.Unmarshal([]byte(out), &ok); err != nil {
		t.Fatalf("--json (suficiente) deveria emitir ResolveResult válido: %v\n%s", err, out)
	}
	if !ok.Sufficient || len(ok.Gaps) != 0 || ok.NextAction != "continua" {
		t.Errorf("JSON suficiente errado: %+v", ok)
	}
}

// =============================================================================
// Formatter injection pattern (context)
// =============================================================================

func TestKnowledgeResolve_FormatterInjection(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()
	resolveFixture(t, root, []string{"6.0"})

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
	cmd.SetIn(strings.NewReader(""))
	formatter := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	cmd.SetContext(newContextWithFormatter(context.Background(), formatter))
	cmd.SetArgs([]string{"knowledge", "resolve", "create transaction"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute com formatter injetado: %v", err)
	}
	if !strings.Contains(buf.String(), "Knowledge Resolver") {
		t.Errorf("formatter injetado deveria receber o output; got:\n%s", buf.String())
	}
}

// compile-time guard.
var _ = NewKnowledgeResolveCommand()
