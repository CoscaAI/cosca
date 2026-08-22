//
// `cosca decision` — Trilha de decisão (Decision Trace).
//
// Explicabilidade de decisão: toda tarefa/decisão da Cosca deve conseguir
// responder "Por que fiz isso?" com um rastro estruturado e auditável:
//
//	Decision ID, Input, Knowledge used, Laws applied, Evidence, Provider,
//	Model, Aprovação humana (Don / Gate 0), Resultado e Rollback.
//
// O rastro vive na tabela `decision_log` da MESMA base do audit
// (.cosca/audit.db). É ouro para auditoria.
//
// Subcomandos:
//   list                  Tabela (id, quando, input truncado, status)
//   explain <id>          Trilha completa estruturada (--json para JSON)
//
// NOTE: decisões são SEMPRE append-only — nada aqui edita/apaga rastros.
//

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/audit"
)

// resolveDecisionStore abre a trilha de decisão do projeto atual
// (<projeto>/.cosca/audit.db — mesma base do audit).
func resolveDecisionStore() (*audit.DecisionStore, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getwd: %w", err)
	}
	return audit.NewDecisionStore(filepath.Join(dir, ".cosca", "audit.db"))
}

// NewDecisionCommand cria a árvore de comandos `cosca decision`.
func NewDecisionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decision",
		Short: "Trilha de decisão (Decision Trace) — explicabilidade auditável de cada decisão",
		Long: `Trilha de decisão (Decision Trace) — explicabilidade auditável.

Toda decisão/tarefa da Cosca deve conseguir responder "Por que fiz isso?" com
um rastro estruturado e auditável: Decision ID, Input, Knowledge used, Laws
applied, Evidence, Provider, Model, Aprovação humana (Don / Gate 0), Resultado
e Rollback. O rastro é gravado na tabela decision_log da base do audit
(.cosca/audit.db).

Subcomandos:
  list                  Tabela das decisões recentes (id, quando, input, status)
  explain <id>          Trilha completa da decisão em formato estruturado
                        (--json para JSON bruto)`,
		Example: `  cosca decision list
  cosca decision list --limit 50
  cosca decision explain D-0001
  cosca decision explain D-0001 --json`,
	}

	cmd.AddCommand(
		NewDecisionListCommand(),
		NewDecisionExplainCommand(),
	)
	return cmd
}

// NewDecisionListCommand cria `cosca decision list`.
func NewDecisionListCommand() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lista as decisões recentes da trilha (id, quando, input, status)",
		Long: `Lista as decisões recentes em uma tabela (Decision ID, quando,
input truncado, status). Rastro append-only — apenas leitura.`,
		Example: `  cosca decision list
  cosca decision list --limit 50
  cosca decision list --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			store, err := resolveDecisionStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			records, err := store.List(limit)
			if err != nil {
				return err
			}

			if useJSON {
				return printJSON(cmd, records)
			}
			if len(records) == 0 {
				formatter.Warning("Nenhuma decisão registrada ainda — rode \"cosca approve\" para gerar o primeiro rastro.")
				return nil
			}

			rows := make([][]string, 0, len(records))
			for _, r := range records {
				rows = append(rows, []string{
					r.DecisionID,
					time.Unix(r.Timestamp, 0).Format("2006-01-02 15:04:05"),
					truncateDecisionInput(r.Input, 60),
					r.Status,
				})
			}
			formatter.Header(fmt.Sprintf("Trilha de decisão — %d decisão(ões)", len(records)))
			formatter.Table([]string{"Decision ID", "Quando", "Input", "Status"}, rows)
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "número máximo de decisões a listar")
	return cmd
}

// NewDecisionExplainCommand cria `cosca decision explain <id>`.
func NewDecisionExplainCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "explain <id>",
		Short: "Explica uma decisão — trilha completa estruturada (\"Por que fiz isso?\")",
		Long: `Imprime a trilha completa de uma decisão, no formato exato do Don:
Decision ID, Input, Knowledge used, Laws applied, Evidence, Provider, Model,
Aprovação humana (Don / Gate 0), Resultado e Rollback.

  --json   emite o rastro bruto em JSON (mesmos campos da trilha).`,
		Example: `  cosca decision explain D-0001
  cosca decision explain D-0001 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			store, err := resolveDecisionStore()
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()

			norm, err := audit.NormalizeDecisionID(args[0])
			if err != nil {
				return err
			}

			rec, err := store.Get(norm)
			if err != nil {
				return err
			}
			if rec == nil {
				return fmt.Errorf("decisão %s não encontrada na trilha (nada foi apagado — a trilha é append-only)", norm)
			}

			if useJSON {
				return printJSON(cmd, rec)
			}

			formatter.Header(fmt.Sprintf("DECISION TRACE %s", rec.DecisionID))
			formatter.KeyValue("Decision ID", rec.DecisionID)
			formatter.KeyValue("Input", rec.Input)
			formatter.KeyValue("Knowledge usado", joinDecisionRefs(rec.KnowledgeUsed))
			formatter.KeyValue("Leis aplicadas", joinDecisionRefs(rec.LawsApplied))
			formatter.KeyValue("Evidências", joinDecisionRefs(rec.Evidence))
			formatter.KeyValue("Provider", rec.Provider)
			formatter.KeyValue("Modelo", rec.Model)
			formatter.KeyValue("Aprovação humana", rec.Approval)
			formatter.KeyValue("Resultado", rec.Result)
			formatter.KeyValue("Rollback", rec.Rollback)
			formatter.KeyValue("Quando", time.Unix(rec.Timestamp, 0).Format(time.RFC3339))
			formatter.KeyValue("Status", rec.Status)
			return nil
		},
	}
}

// joinDecisionRefs junta as referências (K-18, L-07, E-182) separadas por
// espaço, como no formato do Don ("Knowledge used: K-18 K-91").
func joinDecisionRefs(refs []string) string {
	return strings.Join(refs, " ")
}

// truncateDecisionInput reduz o input para o tamanho dado (com "…") — usado
// na tabela de listagem para não estourar a largura.
func truncateDecisionInput(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return strings.TrimSpace(s[:max-1]) + "…"
}
