//
// Tests for `cosca skills sync` — regeneração do Inventário Real do
// SKILLS_CATALOG.md a partir do disco (internal/cli/skills.go).
//
// Covers:
//   - generateSkillsInventory: agrupa por categoria (1º nível), exclui
//     SKILLS_CATALOG.md, ordena, total/categorias corretos
//   - sync --dry-run: gera a tabela esperada e NÃO escreve no arquivo
//   - sync normal: preserva o prefixo antes do marcador e é idempotente
//
// Os testes usam t.TempDir() e não alteram o repositório.

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// fixedSyncNow é a data fixa usada nos testes para o rodapé determinístico.
var fixedSyncNow = time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)

// writeSkillsFixture cria uma árvore de skills de exemplo num diretório
// temporário: 2 categorias, uma com subpasta, e um SKILLS_CATALOG.md (que deve
// ser excluído do inventário).
func writeSkillsFixture(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"backend/go/GO_API_IMPLEMENTATION.md": "# Go API\n",
		"backend/GRPC_IMPLEMENTATION.md":      "# gRPC\n",
		"ai/EMBEDDING_PIPELINE.md":            "# Embedding\n",
		"ai/PROMPT_ENGINEERING.md":            "# Prompt\n",
		"ai/PROVIDER_DISCOVERY.md":            "# Provider\n",
		"SKILLS_CATALOG.md":                   "# Cosca SKILLS CATALOG\n",
	}
	for rel, content := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// runSkillsSyncCommand executa runSkillsSync com um cobra.Command cuja saída é
// capturada num buffer.
func runSkillsSyncCommand(root string, dryRun bool, now time.Time) (string, error) {
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := runSkillsSync(cmd, root, now, dryRun)
	return buf.String(), err
}

// writeCatalogWithInventory grava um SKILLS_CATALOG.md com um prefixo de legado
// e um Inventário Real antigo (a ser substituído).
func writeCatalogWithInventory(t *testing.T, root string) {
	t.Helper()
	catalog := "> **Version**: 1.0 | **Status**: active\n" +
		">\n" +
		"## 🗑️ LEGADO OBSOLETO — não usar\n" +
		"> conteudo legado preservado\n" +
		"\n---\n" +
		"\n" +
		"## 🗂️ Inventário Real (gerado dos diretórios — 2020-01-01)\n" +
		"\n" +
		"| Categoria | Skills | Arquivos |\n" +
		"|-----------|--------|----------|\n" +
		"| **stale** | 1 | STALE_SKILL |\n" +
		"\n" +
		"> **Total real: 1 arquivos de skill em 1 categorias** | **Atualizado**: 2020-01-01\n"
	if err := os.WriteFile(filepath.Join(root, skillsCatalogName), []byte(catalog), 0o644); err != nil {
		t.Fatal(err)
	}
}

// =============================================================================
// generateSkillsInventory
// =============================================================================

func TestSkillsSync_DryRun_GeneratesExpectedTable(t *testing.T) {
	root := t.TempDir()
	writeSkillsFixture(t, root)

	table, total, cats, err := generateSkillsInventory(root)
	if err != nil {
		t.Fatalf("generateSkillsInventory: %v", err)
	}

	// SKILLS_CATALOG.md excluído → 6 .md escritos, 5 skills.
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if cats != 2 {
		t.Errorf("categorias = %d, want 2", cats)
	}

	// Cabeçalho da tabela presente.
	for _, h := range []string{"| Categoria | Skills | Arquivos |", "|-----------|--------|----------|"} {
		if !strings.Contains(table, h) {
			t.Errorf("tabela sem cabeçalho %q:\n%s", h, table)
		}
	}

	// Linha da categoria ai (sem subpasta) na ordem alfabética exata.
	wantAI := "| **ai** | 3 | EMBEDDING_PIPELINE, PROMPT_ENGINEERING, PROVIDER_DISCOVERY |"
	if !strings.Contains(table, wantAI) {
		t.Errorf("tabela deve conter %q:\n%s", wantAI, table)
	}

	// Linha da categoria backend inclui o nome com caminho relativo da subpasta.
	// filepath.Join mantém a asserção correta em qualquer SO (separador nativo).
	relPath := filepath.Join("go", "GO_API_IMPLEMENTATION")
	if !strings.Contains(table, relPath) {
		t.Errorf("tabela deve conter caminho relativo %s:\n%s", relPath, table)
	}

	// Nenhuma linha pode referenciar o catálogo.
	if strings.Contains(table, "SKILLS_CATALOG") {
		t.Errorf("tabela não pode conter SKILLS_CATALOG:\n%s", table)
	}

	// dry-run não escreve no arquivo.
	before, err := os.ReadFile(filepath.Join(root, skillsCatalogName))
	if err != nil {
		t.Fatal(err)
	}
	out, err := runSkillsSyncCommand(root, true, fixedSyncNow)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !strings.Contains(out, "DRY-RUN: 5 arquivos, 2 categorias — nada escrito") {
		t.Errorf("dry-run deve reportar contagem: %q", out)
	}
	if !strings.Contains(out, "EMBEDDING_PIPELINE") {
		t.Errorf("dry-run deve imprimir a tabela: %q", out)
	}
	after, err := os.ReadFile(filepath.Join(root, skillsCatalogName))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("dry-run NÃO deve alterar o SKILLS_CATALOG.md")
	}
}

// =============================================================================
// sync normal — preserva prefixo e é idempotente
// =============================================================================

func TestSkillsSync_IsIdempotent(t *testing.T) {
	root := t.TempDir()
	writeSkillsFixture(t, root)
	writeCatalogWithInventory(t, root)

	// 1ª sincronização.
	if _, err := runSkillsSyncCommand(root, false, fixedSyncNow); err != nil {
		t.Fatalf("primeira sync: %v", err)
	}
	first, err := os.ReadFile(filepath.Join(root, skillsCatalogName))
	if err != nil {
		t.Fatal(err)
	}

	// O prefixo (legado) antes do Inventário Real foi preservado.
	content := string(first)
	if !strings.Contains(content, "## 🗑️ LEGADO OBSOLETO — não usar") {
		t.Error("prefixo legado deve ser preservado")
	}
	if !strings.Contains(content, "conteudo legado preservado") {
		t.Error("conteúdo do legado deve ser preservado")
	}

	// O inventário antigo foi substituído pelo novo.
	if strings.Contains(content, "STALE_SKILL") {
		t.Error("inventário antigo deve ser substituído")
	}
	if !strings.Contains(content, "| **ai** | 3 | EMBEDDING_PIPELINE, PROMPT_ENGINEERING, PROVIDER_DISCOVERY |") {
		t.Errorf("novo inventário deve conter a categoria ai:\n%s", content)
	}
	if !strings.Contains(content, "**Atualizado**: 2026-08-17") {
		t.Errorf("rodapé deve usar a data fixa:\n%s", content)
	}

	// 2ª sincronização → conteúdo idêntico (idempotente).
	if _, err := runSkillsSyncCommand(root, false, fixedSyncNow); err != nil {
		t.Fatalf("segunda sync: %v", err)
	}
	second, err := os.ReadFile(filepath.Join(root, skillsCatalogName))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Error("segunda sync deve produzir conteúdo idêntico (idempotência)")
	}
}

// =============================================================================
// Erros e registros
// =============================================================================

func TestSkillsSync_MissingCatalogFails(t *testing.T) {
	root := t.TempDir()
	writeSkillsFixture(t, root)
	if err := os.Remove(filepath.Join(root, skillsCatalogName)); err != nil {
		t.Fatal(err)
	}
	if _, err := runSkillsSyncCommand(root, false, fixedSyncNow); err == nil {
		t.Error("sync sem SKILLS_CATALOG.md deve falhar com erro claro")
	}
}

func TestSkillsSync_MissingInventoryMarkerFails(t *testing.T) {
	root := t.TempDir()
	writeSkillsFixture(t, root)
	// catálogo SEM o marcador do Inventário Real.
	if err := os.WriteFile(filepath.Join(root, skillsCatalogName), []byte("# apenas legado\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runSkillsSyncCommand(root, false, fixedSyncNow); err == nil {
		t.Error("sync sem o marcador do Inventário Real deve falhar")
	}
}

func TestSkillsSync_RegisteredInSkillsCommand(t *testing.T) {
	cmd := NewSkillsCommand()
	if cmd.Name() != "skills" {
		t.Errorf("nome do comando = %q, want skills", cmd.Name())
	}
	registered := map[string]bool{}
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	if !registered["sync"] {
		t.Error("missing skills subcommand: sync")
	}
}
