package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	// skillsCatalogName é o catálogo de skills (excluído do inventário real).
	skillsCatalogName = "SKILLS_CATALOG.md"
	// skillsInventoryMarker marca o início do Inventário Real no catálogo.
	skillsInventoryMarker = "## 🗂️ Inventário Real"
)

// NewSkillsCommand cria o comando `cosca skills` — gestão do catálogo de
// skills embutido no cérebro (internal/embed/cosca/skills/).
func NewSkillsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Gestão do catálogo de skills",
	}
	cmd.AddCommand(newSkillsSyncCommand())
	return cmd
}

// newSkillsSyncCommand cria o subcomando `cosca skills sync` — regenera o
// Inventário Real do SKILLS_CATALOG.md a partir dos arquivos em disco.
func newSkillsSyncCommand() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Regenera o Inventário Real do SKILLS_CATALOG.md a partir do disco",
		Long: `Regenera a seção "Inventário Real" do SKILLS_CATALOG.md a partir dos
arquivos reais em internal/embed/cosca/skills/. Tudo que vem antes do
Inventário Real (aviso de legado, seções obsoletas) é preservado intacto; a
partir do marcador até o fim do arquivo é substituído pelo estado real do disco.

  --dry-run   imprime a nova tabela sem alterar o arquivo.`,
		Example: `  cosca skills sync
  cosca skills sync --dry-run`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Caminho base relativo ao cwd, como faz embed.go.
			return runSkillsSync(cmd, "internal/embed/cosca/skills", time.Now(), dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "imprime a nova tabela sem escrever no arquivo")
	return cmd
}

// runSkillsSync executa o sync (modo normal ou dry-run) sobre um diretório de
// skills raiz. Separado do cobra para permitir testes com diretórios temporários.
func runSkillsSync(cmd *cobra.Command, root string, now time.Time, dryRun bool) error {
	section, total, cats, err := renderSkillsInventorySection(root, now)
	if err != nil {
		return err
	}

	if dryRun {
		fmt.Fprintln(cmd.OutOrStdout(), section)
		fmt.Fprintf(cmd.OutOrStdout(), "DRY-RUN: %d arquivos, %d categorias — nada escrito\n", total, cats)
		return nil
	}

	catalog := filepath.Join(root, skillsCatalogName)
	old, err := os.ReadFile(catalog)
	if err != nil {
		return fmt.Errorf("SKILLS_CATALOG.md não encontrado em %s: %w", root, err)
	}

	idx := strings.Index(string(old), skillsInventoryMarker)
	if idx < 0 {
		return fmt.Errorf("seção %q não encontrada em %s", skillsInventoryMarker, catalog)
	}
	// Preserva tudo que vem ANTES do Inventário Real; substitui o resto.
	prefix := string(old)[:idx]
	if err := os.WriteFile(catalog, []byte(prefix+section), 0o644); err != nil {
		return fmt.Errorf("falha ao escrever %s: %w", catalog, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "sync OK: %d arquivos, %d categorias em %s\n", total, cats, catalog)
	return nil
}

// renderSkillsInventorySection monta a seção completa do Inventário Real
// (marcador + preâmbulo + tabela + rodapé) a partir do estado do disco.
func renderSkillsInventorySection(root string, now time.Time) (string, int, int, error) {
	table, total, cats, err := generateSkillsInventory(root)
	if err != nil {
		return "", 0, 0, err
	}

	date := now.Format("2006-01-02")
	var b strings.Builder
	fmt.Fprintf(&b, "%s (gerado dos diretórios — %s)\n", skillsInventoryMarker, date)
	b.WriteString("\n")
	fmt.Fprintf(&b, "> O catálogo abaixo reflete o estado REAL dos arquivos de skill (%d arquivos, %d categorias).\n", total, cats)
	b.WriteString("> **Este inventário substitui as contagens anteriores que estavam defasadas.**\n")
	b.WriteString("> **FONTE DA VERDADE**: este inventário é gerado do disco (`find . -name '*.md' -not -name 'SKILLS_CATALOG.md'`) e bate 1:1 com os arquivos.\n")
	b.WriteString("\n")
	b.WriteString(table)
	b.WriteString("\n")
	fmt.Fprintf(&b, "> **Total real: %d arquivos de skill em %d categorias** | **Atualizado**: %s\n", total, cats, date)
	return b.String(), total, cats, nil
}

// generateSkillsInventory percorre o diretório de skills e gera a tabela
// markdown do Inventário Real. Categoria = primeiro nível de diretório; nome =
// caminho relativo à categoria sem extensão .md. Exclui SKILLS_CATALOG.md.
func generateSkillsInventory(root string) (table string, total, cats int, err error) {
	files := map[string][]string{} // categoria → nomes (relativos à categoria, sem .md)
	err = filepath.Walk(root, func(path string, info os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".md") || info.Name() == skillsCatalogName {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) < 2 {
			return nil // arquivo fora de categoria (na raiz do catálogo)
		}
		name := strings.TrimSuffix(filepath.Join(parts[1:]...), ".md")
		files[parts[0]] = append(files[parts[0]], name)
		return nil
	})
	if err != nil {
		return "", 0, 0, err
	}

	categories := make([]string, 0, len(files))
	for c := range files {
		categories = append(categories, c)
	}
	sort.Strings(categories)

	var b strings.Builder
	b.WriteString("| Categoria | Skills | Arquivos |\n")
	b.WriteString("|-----------|--------|----------|\n")
	total = 0
	for _, c := range categories {
		names := files[c]
		sort.Strings(names)
		fmt.Fprintf(&b, "| **%s** | %d | %s |\n", c, len(names), strings.Join(names, ", "))
		total += len(names)
	}
	cats = len(categories)
	return b.String(), total, cats, nil
}
