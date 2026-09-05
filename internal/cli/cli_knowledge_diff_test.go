//
// Tests for `cosca knowledge diff <pkg> <from> <to>`.
//
// Cobre:
//   - Registro do subcomando no grupo knowledge
//   - diff populado: seções NEW/CHANGED/DEPRECATED/REMOVED, risk + affected,
//     aviso pt-BR do Don quando o risco é medium+
//   - diff vazio (nenhum VersionDiffs) → summary
//   - --json
//   - Formatter injection pattern (newContextWithFormatter) — os testes
//     injetam o formatter no contexto do root command
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

// writePrismaManifest grava um manifesto prisma com VersionDiffs "6.0->6.1"
// (a fonte determinística do diff) na store do diretório root.
func writePrismaManifest(t *testing.T, root string, diffs map[string][]knowledge.DiffEntry) {
	t.Helper()
	pkg := knowledge.KnowledgePackage{
		ID: "prisma", Kind: knowledge.PackageKindLibrary, Ecosystem: "typescript",
		Versions: []string{"6.x"}, Sources: []string{"official-docs"},
		KnowledgeLevel: knowledge.PackageKnowledgeNone,
		Status:         knowledge.PackageStatusManifest,
		VersionDiffs:   diffs,
	}
	if err := knowledge.NewPackageStore(root).Add(pkg); err != nil {
		t.Fatalf("Add prisma manifesto: %v", err)
	}
}

// writeLawsItems grava KnowledgeItems do CKL em <root>/.cosca/knowledge/laws.json.
func writeLawsItems(t *testing.T, root string, items []*knowledge.KnowledgeItem) {
	t.Helper()
	engine := knowledge.NewPromotionEngine()
	for _, it := range items {
		if err := engine.Register(it); err != nil {
			t.Fatalf("register %s: %v", it.ID, err)
		}
	}
	if err := engine.Save(root + "/.cosca/knowledge/laws.json"); err != nil {
		t.Fatalf("save laws.json: %v", err)
	}
}

// prismaDiffFixture monta o cenário do Don: prisma 6.0→6.1 com
// new/changed/deprecated/removed e 3 KnowledgeItems afetados (K-182/K-219/K-441).
func prismaDiffFixture(t *testing.T, root string) {
	t.Helper()
	writePrismaManifest(t, root, map[string][]knowledge.DiffEntry{
		"6.0->6.1": {
			{Kind: knowledge.DiffKindNew, Symbol: "Resource X", Detail: "novo recurso de deploy"},
			{Kind: knowledge.DiffKindChanged, Symbol: "schema.migrations", Detail: "novo formato do arquivo"},
			{Kind: knowledge.DiffKindDeprecated, Symbol: "API A", Detail: "será removida na 7.0"},
			{Kind: knowledge.DiffKindRemoved, Symbol: "API B", Detail: "substituída por Resource X"},
		},
	})
	writeLawsItems(t, root, []*knowledge.KnowledgeItem{
		{
			ID: "K-182", Title: "Prisma datasource usa API B",
			Evidence: []knowledge.Evidence{{Source: "prisma", Description: "API B no datasource"}},
		},
		{
			ID: "K-219", Title: "Migrações prisma",
			Evidence: []knowledge.Evidence{{Source: "prisma", Description: "schema.migrations formato atual"}},
		},
		{
			ID: "K-441", Title: "Padrão de seed prisma",
			Evidence: []knowledge.Evidence{{Source: "prisma", Description: "API B e API A em uso"}},
		},
		{
			ID: "K-999", Title: "Regra de segurança",
			Evidence: []knowledge.Evidence{{Source: "auditoria-seguranca-2026-07-31", Description: "nunca root"}},
		},
	})
}

// =============================================================================
// Registration
// =============================================================================

func TestKnowledgeDiffCommand_RegisteredInKnowledge(t *testing.T) {
	cmd := NewKnowledgeCommand()
	found := map[string]bool{}
	for _, sub := range cmd.Commands() {
		found[sub.Name()] = true
	}
	if !found["diff"] {
		t.Error("knowledge subcommand \"diff\" não registrado")
	}
}

// =============================================================================
// `knowledge diff` — populado + aviso pt-BR do Don
// =============================================================================

func TestKnowledgeDiff_Populated(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()
	prismaDiffFixture(t, root)

	out, err := executeKnowledgePackage(t, root, "", "diff", "prisma", "6.0", "6.1")
	if err != nil {
		t.Fatalf("knowledge diff prisma 6.0 6.1: %v\n%s", err, out)
	}

	// Cabeçalho e versões.
	for _, want := range []string{"Knowledge Diff — prisma", "6.0 → 6.1"} {
		if !strings.Contains(out, want) {
			t.Errorf("output deveria conter %q; output:\n%s", want, out)
		}
	}

	// Seções com os símbolos e detalhes.
	for _, want := range []string{
		"NEW", "+ Resource X", "novo recurso de deploy",
		"CHANGED", "~ schema.migrations", "novo formato do arquivo",
		"DEPRECATED", "- API A", "será removida na 7.0",
		"REMOVED", "- API B", "substituída por Resource X",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output deveria conter %q; output:\n%s", want, out)
		}
	}

	// Risk medium (3 itens afetados) + affected knowledge + impacto.
	if !strings.Contains(out, "RISK: medium") {
		t.Errorf("risco deveria ser medium; output:\n%s", out)
	}
	if !strings.Contains(out, "3 padrões existentes podem ser afetados") {
		t.Errorf("summary medium deveria aparecer; output:\n%s", out)
	}
	for _, want := range []string{"K-182", "K-219", "K-441", "AFFECTED KNOWLEDGE"} {
		if !strings.Contains(out, want) {
			t.Errorf("output deveria conter %q; output:\n%s", want, out)
		}
	}
	// K-999 não referencia o pacote → NÃO afetada.
	if strings.Contains(out, "K-999") {
		t.Errorf("K-999 não deveria estar afetada; output:\n%s", out)
	}
	if !strings.Contains(out, "Impacto no projeto") {
		t.Errorf("output deveria mostrar impacto no projeto; output:\n%s", out)
	}

	// Aviso pt-BR do Don (risco medium+).
	warning := "Chef, a atualização altera uma API utilizada em 2 pontos. Não atualizei nada. Preparei a análise."
	if !strings.Contains(out, warning) {
		t.Errorf("output deveria conter o aviso do Don; output:\n%s", out)
	}
}

func TestKnowledgeDiff_EmptyNoDiffRecorded(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()
	writePrismaManifest(t, root, nil) // sem version_diffs

	out, err := executeKnowledgePackage(t, root, "", "diff", "prisma", "6.0", "6.1")
	if err != nil {
		t.Fatalf("knowledge diff (empty): %v\n%s", err, out)
	}
	if !strings.Contains(out, "nenhuma mudança registrada entre 6.0 e 6.1") {
		t.Errorf("diff vazio deveria informar summary; output:\n%s", out)
	}
	if !strings.Contains(out, "Risk") || !strings.Contains(out, "none") {
		t.Errorf("diff vazio deveria mostrar risk none; output:\n%s", out)
	}
	if !strings.Contains(out, "NEW") && !strings.Contains(out, "CHANGED") {
		// ok — nenhuma seção
	} else {
		t.Errorf("diff vazio não deveria ter seções NEW/CHANGED; output:\n%s", out)
	}
}

func TestKnowledgeDiff_UnknownPackage(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()

	_, err := executeKnowledgePackage(t, root, "", "diff", "next", "6.0", "6.1")
	if err == nil || !strings.Contains(err.Error(), "não encontrado") {
		t.Fatalf("diff de pacote inexistente deveria falhar, got %v", err)
	}
}

func TestKnowledgeDiff_InvalidArgs(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()

	if _, err := executeKnowledgePackage(t, root, "", "diff", "prisma", "6.0"); err == nil {
		t.Error("diff com 2 args deveria falhar (exigem 3)")
	}
	if _, err := executeKnowledgePackage(t, root, "", "diff", "prisma", "6.0", "6.0"); err == nil {
		t.Error("diff com versões iguais deveria falhar")
	}
}

// =============================================================================
// `knowledge diff --json`
// =============================================================================

func TestKnowledgeDiff_JSON(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()
	prismaDiffFixture(t, root)

	out, err := executeKnowledgePackage(t, root, "", "diff", "prisma", "6.0", "6.1", "--json")
	if err != nil {
		t.Fatalf("knowledge diff --json: %v\n%s", err, out)
	}

	var diff knowledge.KnowledgeDiff
	if err := json.Unmarshal([]byte(out), &diff); err != nil {
		t.Fatalf("--json deveria emitir KnowledgeDiff válido: %v\n%s", err, out)
	}
	if diff.PackageID != "prisma" || diff.FromVersion != "6.0" || diff.ToVersion != "6.1" {
		t.Errorf("metadados do JSON errados: %+v", diff)
	}
	if diff.RiskLevel != knowledge.RiskMedium {
		t.Errorf("risk JSON deveria ser medium, got %q", diff.RiskLevel)
	}
	wantAffected := []string{"K-182", "K-219", "K-441"}
	if len(diff.AffectedKnowledge) != 3 {
		t.Fatalf("affected JSON errado: %v", diff.AffectedKnowledge)
	}
	for i := range wantAffected {
		if diff.AffectedKnowledge[i] != wantAffected[i] {
			t.Errorf("affected[%d] = %q, want %q", i, diff.AffectedKnowledge[i], wantAffected[i])
		}
	}
	if diff.ProjectImpact != 2 {
		t.Errorf("project_impact JSON deveria ser 2, got %d", diff.ProjectImpact)
	}
	if len(diff.Entries) != 4 {
		t.Errorf("entries JSON deveria ter 4, got %d", len(diff.Entries))
	}
}

// =============================================================================
// Formatter injection pattern (context) — sem formatter injetado o comando
// deve funcionar com o default.
// =============================================================================

func TestKnowledgeDiff_FormatterInjection(t *testing.T) {
	globalFlags = GlobalFlags{}
	root := t.TempDir()
	prismaDiffFixture(t, root)

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
	cmd.SetArgs([]string{"knowledge", "diff", "prisma", "6.0", "6.1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute com formatter injetado: %v", err)
	}
	if !strings.Contains(buf.String(), "Knowledge Diff — prisma") {
		t.Errorf("formatter injetado deveria receber o output; got:\n%s", buf.String())
	}
}

// compile-time guards.
var _ = NewKnowledgeDiffCommand()
