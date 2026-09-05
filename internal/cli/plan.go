//
// `cosca plan` — estima um Plano de Execução ANTES da aprovação.
//
// O comando é SOMENTE LEITURA: ele nunca modifica nada no projeto. Ele delega
// a estimativa ao pacote internal/estimator (ExecutionPlan/ExecutionScope/
// Estimate/String) e imprime o plano no formato exato do Don, ou em JSON
// (compatível com `cosca approve --plan`).
//

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/estimator"
)

// planTargetSeparator separa os múltiplos globs/caminhos do --target.
const planTargetSeparator = ","

// NewPlanCommand cria o comando `cosca plan`.
func NewPlanCommand() *cobra.Command {
	var (
		target     string
		changeType string
		agent      string
		asJSON     bool
	)

	cmd := &cobra.Command{
		Use:   "plan --target <glob|lista-de-arquivos>",
		Short: "Estimar um plano de execução antes da aprovação",
		Long: `Estima o Plano de Execução (pré-aprovação) que o Don vê antes de aprovar
qualquer execução: arquivos afetados, testes previstos, migrações, rollback,
tempo estimado, risco e confiança.

O comando é SOMENTE LEITURA: nada é alterado no projeto.

Flags:
  --target   glob ou lista de arquivos a analisar (ex: "internal/kernel/*.go"
             ou "internal/kernel/*.go,api/rest/**")
  --type     tipo da alteração: feature, fix, security, refactor, docs, test
  --agent    agente executor (usado no cálculo de confiança)
  --json     saída em JSON (formato aceito por "cosca approve --plan")`,
		Example: `  cosca plan --target "internal/kernel/*.go"
  cosca plan --target "internal/kernel/*.go,api/rest/**" --type refactor --agent cosca-backend
  cosca plan --target "internal/estimator/*.go" --json > plano.json
  cosca approve --plan plano.json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("falha ao obter o diretório de trabalho: %w", err)
			}

			scope := estimator.ExecutionScope{
				ProjectDir:  cwd,
				TargetFiles: splitPlanTarget(target),
				ChangeType:  changeType,
				Agent:       agent,
			}

			// Guarda: o target precisa expandir para pelo menos um arquivo.
			files, err := estimator.ExpandScope(scope)
			if err != nil {
				return err
			}
			if len(files) == 0 {
				return fmt.Errorf("nenhum arquivo encontrado para o target: %s", target)
			}

			plan, err := estimator.Estimate(scope)
			if err != nil {
				return fmt.Errorf("falha ao estimar o plano de execução: %w", err)
			}

			if asJSON || IsJSONOutput(cmd) {
				return printJSON(cmd, plan)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), plan.String())
			return err
		},
	}

	cmd.Flags().StringVar(&target, "target", "",
		`glob ou lista de arquivos a analisar (ex: "internal/kernel/*.go" ou "internal/kernel/*.go,api/rest/**")`)
	cmd.Flags().StringVar(&changeType, "type", "feature",
		"tipo da alteração (feature, fix, security, refactor, docs, test)")
	cmd.Flags().StringVar(&agent, "agent", "cosca-backend",
		"agente executor (usado no cálculo de confiança)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "saída em JSON")
	_ = cmd.MarkFlagRequired("target")
	return cmd
}

// splitPlanTarget separa o valor do --target em entradas individuais (globs
// ou caminhos literais), ignorando entradas vazias.
func splitPlanTarget(target string) []string {
	var out []string
	for _, part := range strings.Split(target, planTargetSeparator) {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
