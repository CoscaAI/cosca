//
// `cosca gate catalog` — GATE DE CATÁLOGO (generate-and-diff).
//
// Implementa o gate do catálogo do framework Cosca. O catálogo é o conjunto de
// colunas (agents/skills/engines/departments) que devem ter INDEX.md, o
// frontmatter canônico (name/description/level) em SKILL.md/PROMPT.md e os
// cross-references internos válidos. É o "catálogo descoberto" que os docs
// (+155 diretórios) precisam manter coerente.
//
// O model é generate-and-diff, determinístico, com DUAS operações ortogonais:
//
//   --generate          regenera o snapshot canônico (.opencode/cosca/catalog.manifest)
//                       com a lista ordenada de INDEX esperados + nomes canônicos.
//                       O snapshot é o CONTRATO (versionado no git).
//   --check  (default)  DIF de DRIFT: compara a árvore viva com o snapshot.
//                       Deve PASSAR (drift=0) e enforçar SÓ novo drift no futuro.
//                       NÃO roda os 3 invariantes. exit non-zero em caso de drift.
//   --audit             Roda os 3 invariantes (index-missing, frontmatter,
//                       dangling-link) como DÉBITO NÃO-BLOQUEANTE: imprime
//                       contagens + exemplos e exit 0 (a menos que use --strict).
//
// Flags:
//   --strict   com --audit: encontrar violação => exit 1 (bloqueante).
//   --summary  imprime 1 linha curta p/ o hook (ex: "catalog: OK" ou
//              "catalog: DRIFT (n)" / "catalog: AUDIT (n)").
//
// Subcomandos de `cosca gate`:
//   catalog --check     Dif de drift do snapshot (deve passar)
//   catalog --audit     Auditoria dos 3 invariantes (débito não-bloqueante)
//   catalog --generate  Regenera o snapshot canônico catalog.manifest
//

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/catalog"
)

// catalogRootFromCWD resolve o .opencode/cosca do diretório de trabalho.
func catalogRootFromCWD() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	return filepath.Join(cwd, ".opencode", "cosca"), nil
}

// NewGateCatalogCommand cria a subárvore `cosca gate catalog`.
func NewGateCatalogCommand() *cobra.Command {
	var (
		generate bool
		check    bool
		audit    bool
		strict   bool
		summary  bool
	)

	cmd := &cobra.Command{
		Use:   "catalog",
		Short: "GATE DE CATÁLOGO — snapshot generate-and-diff (--check) + auditoria de invariantes (--audit)",
		Long: `GATE DE CATÁLOGO do framework Cosca (generate-and-diff, determinístico).

DUAS OPERAÇÕES ORTOGONAIS:

  --check  (default)  DIF DE DRIFT do snapshot canônico .opencode/cosca/catalog.manifest.
                      Compara a árvore viva com o snapshot commitado. Deve PASSAR
                      (drift=0) e enforçar SÓ novo drift no futuro. NÃO avalia os
                      invariantes A/B/C. Se houver drift, falha e orienta rodar
                      'cosca gate catalog --generate' + commitar.

  --audit             AUDITORIA DOS 3 INVARIANTES como débito NÃO-BLOQUEANTE:
                        A. INDEX.md      coluna de agents/skills/engines/departments
                                         sem INDEX.md.
                        B. frontmatter   SKILL.md/PROMPT.md sem name/description
                                         (ou level fora de 1-5).
                        C. cross-refs    link relativo em *.md apontando para alvo
                                         inexistente.
                      Reporta contagens + exemplos e faz exit 0 (a menos que use
                      --strict). A dívida vira backlog visível, não trava a esteira.

  --generate          Regenera o contrato canônico .opencode/cosca/catalog.manifest
                      (lista ordenada de INDEX esperados + nomes canônicos).

Flags:
  --strict    com --audit: encontrar violação => exit 1 (bloqueante).
  --summary   imprime 1 linha curta p/ o hook (ex: "catalog: OK").

Exemplos:
  cosca gate catalog --check
  cosca gate catalog --check --summary
  cosca gate catalog --audit
  cosca gate catalog --audit --strict
  cosca gate catalog --generate`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			root, err := catalogRootFromCWD()
			if err != nil {
				return err
			}

			switch {
			case generate:
				return runCatalogGenerate(cmd, formatter, useJSON, root)
			case audit:
				return runCatalogAudit(cmd, formatter, useJSON, strict, summary, root)
			default: // --check (default) = drift
				return runCatalogCheck(cmd, formatter, useJSON, strict, summary, root)
			}
		},
	}

	cmd.Flags().BoolVar(&generate, "generate", false, "regenerar o snapshot canônico .opencode/cosca/catalog.manifest (em vez de checar)")
	cmd.Flags().BoolVar(&check, "check", false, "verificar DRIFT do snapshot (default; exit non-zero em drift)")
	cmd.Flags().BoolVar(&audit, "audit", false, "auditar os 3 invariantes como débito não-bloqueante (em vez de checar drift)")
	cmd.Flags().BoolVar(&strict, "strict", false, "com --audit: tornar achados bloqueantes (exit 1)")
	cmd.Flags().BoolVar(&summary, "summary", false, "imprimir 1 linha curta de resumo (p/ hook)")
	return cmd
}

// runCatalogGenerate escreve o snapshot canônico catalog.manifest.
func runCatalogGenerate(cmd *cobra.Command, formatter *OutputFormatter, useJSON bool, root string) error {
	text, err := catalog.Generate(root)
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(root, catalog.ManifestFileName)
	if err := os.WriteFile(manifestPath, []byte(text), 0o644); err != nil {
		return fmt.Errorf("write manifest %s: %w", manifestPath, err)
	}

	if useJSON {
		return printJSON(cmd, map[string]interface{}{
			"generated": true,
			"file":      catalog.ManifestFileName,
			"root":      root,
		})
	}
	formatter.Success(fmt.Sprintf("Snapshot canônico gerado ✓ — %s", catalog.ManifestFileName))
	formatter.KeyValue("Arquivo", manifestPath)
	formatter.KeyValue("Regras", "check = DRIFT do snapshot | audit = A: INDEX.md | B: name/description/level | C: cross-refs")
	return nil
}

// runCatalogCheck roda o DIF DE DRIFT do snapshot (default). Deve passar.
func runCatalogCheck(cmd *cobra.Command, formatter *OutputFormatter, useJSON, strict, summary bool, root string) error {
	rep, err := catalog.CheckDrift(root)
	if err != nil {
		return err
	}

	if summary {
		fmt.Fprintln(cmd.OutOrStdout(), driftSummaryLine(rep))
		return catalogExit(rep.Pass)
	}

	if useJSON {
		if err := printJSON(cmd, rep); err != nil {
			return err
		}
	} else {
		printDriftReport(formatter, rep)
	}

	if !rep.Pass {
		return catalogExit(false)
	}
	return nil
}

// runCatalogAudit roda os 3 invariantes como débito não-bloqueante.
func runCatalogAudit(cmd *cobra.Command, formatter *OutputFormatter, useJSON, strict, summary bool, root string) error {
	rep, err := catalog.AuditInvariants(root)
	if err != nil {
		return err
	}

	if summary {
		fmt.Fprintln(cmd.OutOrStdout(), auditSummaryLine(rep))
		if strict && !rep.Pass {
			return catalogExit(false)
		}
		return nil
	}

	if useJSON {
		if err := printJSON(cmd, rep); err != nil {
			return err
		}
	} else {
		printAuditReport(formatter, rep)
	}

	// Não-bloqueante por padrão; --strict transforma achados em exit 1.
	if strict && !rep.Pass {
		return catalogExit(false)
	}
	return nil
}

// catalogExit traduz um booleano de sucesso em um ExitCodeError (exit 1) ou nil.
func catalogExit(pass bool) error {
	if pass {
		return nil
	}
	return ExitCodeError{Code: 1}
}

// driftSummaryLine é a 1 linha curta do --check para o hook.
func driftSummaryLine(rep *catalog.Report) string {
	if rep.Pass {
		return "catalog: OK"
	}
	return fmt.Sprintf("catalog: DRIFT (%d)", len(rep.Violations))
}

// auditSummaryLine é a 1 linha curta do --audit para o hook.
func auditSummaryLine(rep *catalog.Report) string {
	if rep.Pass {
		return "catalog: OK"
	}
	return fmt.Sprintf("catalog: AUDIT (%d)", len(rep.Violations))
}

// printDriftReport renderiza o relatório humano do check de drift.
func printDriftReport(formatter *OutputFormatter, rep *catalog.Report) {
	formatter.Header(fmt.Sprintf("GATE CATÁLOGO (drifte do snapshot) — %s", rep.Root))

	if rep.Pass {
		formatter.Success("CATÁLOGO OK ✓ (drift=0)")
	} else {
		formatter.Error(fmt.Sprintf("CATÁLOGO COM DRIFT — %d violação(ões) ✗", len(rep.Violations)))
	}

	formatter.KeyValue("Colunas de catálogo", fmt.Sprintf("%d", rep.Stats.Columns))
	formatter.KeyValue("Snapshot presente", boolWord(rep.Stats.Manifest))

	if rep.Pass {
		return
	}

	const cap = 100
	formatter.Header(fmt.Sprintf("%s (%d)", kindLabel(catalog.KindManifestDrift), len(rep.Violations)))
	for i, v := range rep.Violations {
		if i >= cap {
			formatter.Bullet(fmt.Sprintf("… e mais %d violação(ões) deste tipo", len(rep.Violations)-cap))
			break
		}
		formatter.Bullet(fmt.Sprintf("%s — %s", v.Path, v.Detail))
	}
	formatter.Header("Próximos passos")
	formatter.Bullet("Há drift de contrato ✓")
	formatter.Bullet("Rode: cosca gate catalog --generate")
	formatter.Bullet("Depois: commit do .opencode/cosca/catalog.manifest")
}

// printAuditReport renderiza o relatório humano da auditoria (débito não-bloqueante).
func printAuditReport(formatter *OutputFormatter, rep *catalog.Report) {
	formatter.Header(fmt.Sprintf("AUDITORIA DE CATÁLOGO (débito não-bloqueante) — %s", rep.Root))

	byKind := map[string][]catalog.Violation{}
	for _, v := range rep.Violations {
		byKind[v.Kind] = append(byKind[v.Kind], v)
	}
	kinds := make([]string, 0, len(byKind))
	for k := range byKind {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)

	if rep.Pass {
		formatter.Success("SEM DÉBITO DE CATÁLOGO ✓")
	} else {
		formatter.Warning(fmt.Sprintf("DÉBITO DE CATÁLOGO — %d achado(s) ✗ (não-bloqueante; use --strict p/ travar)", len(rep.Violations)))
	}

	formatter.KeyValue("Arquivos .md", fmt.Sprintf("%d", rep.Stats.MdFiles))
	formatter.KeyValue("SKILL/PROMPT examinados", fmt.Sprintf("%d", rep.Stats.Prompts))
	formatter.KeyValue("Colunas de catálogo", fmt.Sprintf("%d", rep.Stats.Columns))
	formatter.KeyValue("Links relativos", fmt.Sprintf("%d", rep.Stats.Links))

	if rep.Pass {
		return
	}

	const cap = 100
	for _, kind := range kinds {
		vs := byKind[kind]
		formatter.Header(fmt.Sprintf("%s (%d)", kindLabel(kind), len(vs)))
		for i, v := range vs {
			if i >= cap {
				formatter.Bullet(fmt.Sprintf("… e mais %d violação(ões) deste tipo", len(vs)-cap))
				break
			}
			formatter.Bullet(fmt.Sprintf("%s — %s", v.Path, v.Detail))
		}
	}

	formatter.Header("Como tratar este débito")
	formatter.Bullet("É backlog de qualidade: os achados NÃO travam a esteira (exit 0 por padrão).")
	formatter.Bullet("Priorize por severidade no catálogo do QA; rode novamente após cada correção.")
	formatter.Bullet("Para travar em CI/qualidade, use: cosca gate catalog --audit --strict")
}

func kindLabel(kind string) string {
	switch kind {
	case catalog.KindIndexMissing:
		return "INDEX.md ausente"
	case catalog.KindFrontmatter:
		return "frontmatter inválido"
	case catalog.KindDanglingLink:
		return "cross-referência quebrada"
	case catalog.KindManifestDrift:
		return "drift do snapshot"
	default:
		return kind
	}
}

func boolWord(b bool) string {
	if b {
		return "presente"
	}
	return "ausente"
}
