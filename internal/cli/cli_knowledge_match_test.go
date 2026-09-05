//
// Tests for `cosca knowledge match [dir]` — Projeto → Dependency Detection →
// Knowledge Matching.
//
// Cobre:
//   - Registro do subcomando no grupo knowledge
//   - Tabela Dependência | Pacote | Status | Ação sugerida (prisma verified,
//     zod partial, gin missing) + linha de resumo pt-BR
//   - --json (lista de DependencyMatch)
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

// writeMatchManifest grava um manifesto de Knowledge Package na store do root.
func writeMatchManifest(t *testing.T, root string, pkg knowledge.KnowledgePackage) {
	t.Helper()
	if err := knowledge.NewPackageStore(root).Add(pkg); err != nil {
		t.Fatalf("Add manifesto %s: %v", pkg.ID, err)
	}
}

// matchFixture monta o projeto fake com deps [prisma, zod, gin] e a store com
// prisma validated + zod manifest (gin ausente).
func matchFixture(t *testing.T, root string) {
	t.Helper()
	pkgJSON := `{"name":"fake-project","dependencies":{"prisma":"6.1","zod":"^3.23","gin":"1"}}`
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(pkgJSON), 0o600); err != nil {
		t.Fatalf("escrever package.json: %v", err)
	}
	writeMatchManifest(t, root, knowledge.KnowledgePackage{
		ID: "prisma", Kind: knowledge.PackageKindLibrary, Ecosystem: "typescript",
		Versions: []string{"6.x"}, Sources: []string{"official-docs"},
		KnowledgeLevel: knowledge.PackageKnowledgeValidated,
		Status:         knowledge.PackageStatusValidated,
	})
	writeMatchManifest(t, root, knowledge.KnowledgePackage{
		ID: "zod", Kind: knowledge.PackageKindLibrary, Ecosystem: "typescript",
		Versions: []string{"3.x"}, Sources: []string{"official-docs"},
		KnowledgeLevel: knowledge.PackageKnowledgeNone,
		Status:         knowledge.PackageStatusManifest,
	})
}

// =============================================================================
// Registration
// =============================================================================

func TestKnowledgeMatchCommand_RegisteredInKnowledge(t *testing.T) {
	cmd := NewKnowledgeCommand()
	found := map[string]bool{}
	for _, sub := range cmd.Commands() {
		found[sub.Name()] = true
	}
	if !found["match"] {
		t.Error("knowledge subcommand \"match\" não registrado")
	}
}

// =============================================================================
// `knowledge match` — tabela + resumo
// =============================================================================

func TestKnowledgeMatch_TableAndSummary(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()
	matchFixture(t, root)

	out, err := executeKnowledgePackage(t, root, "", "match")
	if err != nil {
		t.Fatalf("knowledge match: %v\n%s", err, out)
	}

	// Cabeçalho do projeto (o nome é o diretório raiz detectado).
	if !strings.Contains(out, "Knowledge Matching —") {
		t.Errorf("output deveria ter o cabeçalho do projeto; output:\n%s", out)
	}

	// Cabeçalhos da tabela.
	for _, want := range []string{"Dependência", "Pacote", "Status", "Ação sugerida"} {
		if !strings.Contains(out, want) {
			t.Errorf("tabela deveria conter %q; output:\n%s", want, out)
		}
	}

	// Status por dependência.
	for _, want := range []string{
		"prisma", "✓ verificado",
		"zod", "◐ parcial", "complete a aquisição",
		"gin", "✗ sem conhecimento", "ofereça: cosca knowledge add gin",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output deveria conter %q; output:\n%s", want, out)
		}
	}

	// Resumo: 3 dependências · 1 verificado · 1 sem conhecimento.
	if !strings.Contains(out, "3 dependências · 1 com conhecimento verificado · 1 sem conhecimento") {
		t.Errorf("resumo errado; output:\n%s", out)
	}
}

func TestKnowledgeMatch_NoDeps(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()

	out, err := executeKnowledgePackage(t, root, "", "match")
	if err != nil {
		t.Fatalf("knowledge match (sem deps): %v\n%s", err, out)
	}
	if !strings.Contains(out, "Nenhuma dependência detectada") {
		t.Errorf("projeto sem deps deveria avisar; output:\n%s", out)
	}
}

// =============================================================================
// `knowledge match --json`
// =============================================================================

func TestKnowledgeMatch_JSON(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()
	matchFixture(t, root)

	out, err := executeKnowledgePackage(t, root, "", "match", "--json")
	if err != nil {
		t.Fatalf("knowledge match --json: %v\n%s", err, out)
	}

	var matches []knowledge.DependencyMatch
	if err := json.Unmarshal([]byte(out), &matches); err != nil {
		t.Fatalf("--json deveria emitir []DependencyMatch válido: %v\n%s", err, out)
	}
	if len(matches) != 3 {
		t.Fatalf("JSON deveria ter 3 matches, got %d", len(matches))
	}

	byDep := map[string]knowledge.DependencyMatch{}
	for _, m := range matches {
		byDep[m.Dependency] = m
	}
	if byDep["prisma"].Status != knowledge.MatchVerified || byDep["prisma"].PackageID != "prisma" {
		t.Errorf("prisma JSON errado: %+v", byDep["prisma"])
	}
	if byDep["zod"].Status != knowledge.MatchPartial || byDep["zod"].PackageID != "zod" {
		t.Errorf("zod JSON errado: %+v", byDep["zod"])
	}
	if byDep["gin"].Status != knowledge.MatchMissing || byDep["gin"].PackageID != "" {
		t.Errorf("gin JSON errado: %+v", byDep["gin"])
	}
	if byDep["prisma"].Note != "conhecimento verificado" {
		t.Errorf("note JSON errado: %+v", byDep["prisma"])
	}
}

// =============================================================================
// Formatter injection pattern (context)
// =============================================================================

func TestKnowledgeMatch_FormatterInjection(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()
	matchFixture(t, root)

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
	cmd.SetArgs([]string{"knowledge", "match"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute com formatter injetado: %v", err)
	}
	if !strings.Contains(buf.String(), "Knowledge Matching") {
		t.Errorf("formatter injetado deveria receber o output; got:\n%s", buf.String())
	}
}

// compile-time guard.
var _ = NewKnowledgeMatchCommand()
