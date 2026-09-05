//
// `cosca knowledge status` — estado epistemológico das leis do CKL.
//
// Sem argumentos: tabela de todas as leis com seu estado epistemológico
// (ID | Título | Status | Verificações). Com um ID: detalhe do estado de uma
// única lei (status, descrição, última verificação, contagens, evidências).
//
// Os estados são os 6 tipados do CKL — KNOWN, SUPPORTED, UNCERTAIN,
// CONFLICTING, UNKNOWN, STALE — para que o sistema saiba quando NÃO sabe
// (ex.: "Não possuo evidência suficiente para transformar isso em
// conhecimento."). Leis antigas sem status migram via DefaultStatus no load.
//
// Reutiliza o padrão do `law`: resolveLawsPath + loadLawsEngine + o seed
// idempotente da primeira execução (ensureSeededLaws).
//

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/knowledge"
)

// NewKnowledgeStatusCommand cria o comando `cosca knowledge status`.
func NewKnowledgeStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status [id]",
		Short: "Estado epistemológico das leis (KNOWN/SUPPORTED/UNCERTAIN/...)",
		Long: `Mostra o estado epistemológico de cada lei do Cosca Knowledge
Lifecycle (CKL).

O estado responde "o que o sistema sabe sobre esta lei?" — os 6 estados
tipados: KNOWN (conhecimento estabelecido), SUPPORTED (evidência parcial),
UNCERTAIN (evidência insuficiente), CONFLICTING (evidências contraditórias),
UNKNOWN (sem evidência) e STALE (expirado — requer revalidação). É como o
sistema sabe quando NÃO sabe.

Sem argumento, lista todas as leis numa tabela. Com um ID, mostra o detalhe
de uma única lei.

Persistência: .cosca/knowledge/laws.json (runtime, gitignored). Leis antigas
sem status migram automaticamente via DefaultStatus (learning → SUPPORTED,
law → KNOWN).
`,
		Example: `  cosca knowledge status
  cosca knowledge status K-01
  cosca knowledge status --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			path, err := resolveLawsPath()
			if err != nil {
				return err
			}

			// Primeira execução: semear as 5 leis reais (idempotente — um
			// arquivo existente nunca é sobrescrito).
			seeded, err := ensureSeededLaws(path)
			if err != nil {
				return err
			}

			engine, err := loadLawsEngine(path)
			if err != nil {
				return err
			}

			if seeded {
				formatter.Verbose(fmt.Sprintf(
					"Primeira execução: %d leis seed criadas em %s (runtime, gitignored)",
					len(engine.All()), path))
			}

			// Com ID: detalhe de uma única lei.
			if len(args) == 1 {
				item, ok := engine.Get(args[0])
				if !ok {
					return fmt.Errorf("lei %q não encontrada em %s", args[0], path)
				}
				return printStatusDetail(cmd, formatter, useJSON, item)
			}

			items := engine.All()

			if useJSON {
				return printJSON(cmd, items)
			}

			if len(items) == 0 {
				formatter.Warning("Nenhuma lei registrada ainda — use \"cosca knowledge law add-evidence <id> ...\".")
				return nil
			}

			formatter.Header(fmt.Sprintf("Estado epistemológico das leis (%d)", len(items)))
			rows := make([][]string, 0, len(items))
			for _, it := range items {
				rows = append(rows, []string{
					it.ID,
					it.Title,
					string(it.Status),
					fmt.Sprintf("%d", it.VerificationCount),
				})
			}
			formatter.Table([]string{"ID", "Título", "Status", "Verificações"}, rows)
			return nil
		},
	}
}

// printStatusDetail imprime o estado epistemológico de uma única lei (texto
// ou JSON), seguindo o padrão do `law show`.
func printStatusDetail(cmd *cobra.Command, formatter *OutputFormatter, useJSON bool, item *knowledge.KnowledgeItem) error {
	if useJSON {
		return printJSON(cmd, map[string]interface{}{
			"item":               item,
			"status_description": item.Status.Description(),
		})
	}

	lastVerified := "nunca"
	if !item.LastVerified.IsZero() {
		lastVerified = item.LastVerified.Format("2006-01-02")
	}

	formatter.Header(fmt.Sprintf("Lei %s — %s", item.ID, item.Title))
	formatter.KeyValue("Status", string(item.Status))
	formatter.KeyValue("Descrição", item.Status.Description())
	formatter.KeyValue("Última verificação", lastVerified)
	formatter.KeyValue("Verificações", fmt.Sprintf("%d", item.VerificationCount))
	formatter.KeyValue("Contradições", fmt.Sprintf("%d", item.ContradictionCount))
	formatter.KeyValue("Evidências", fmt.Sprintf("%d", len(item.Evidence)))
	return nil
}
