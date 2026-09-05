//
// `cosca budget` — Budget cognitivo: contabilizar tokens/tempo/custo antes de
// chamar IA.
//
// Regra do Don: "Você pode criar um 'budget cognitivo'. Antes de chamar IA:
// Cognitive Budget {Tokens: 8k, Time: 20s, Cost: $0.05}. O sistema tenta
// primeiro: local knowledge → external deterministic search → cached evidence
// → existing solution. Só então chama o modelo. Se conseguir resolver sem IA:
// AI calls: 0, Cost: $0."
//
// Comando apenas de leitura/diagnóstico — não altera nenhum estado.
//
// Subcomandos:
//   default                          Mostra o budget cognitivo padrão
//   check --tokens N --time D --cost C   Dry-run: um consumo proposto caberia
//                                        no budget padrão? (dentro/acima)
//

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/engine"
)

// NewBudgetCommand cria a árvore de comandos `cosca budget`.
func NewBudgetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "budget",
		Short: "Budget cognitivo — contabiliza tokens/tempo/custo antes de chamar IA",
		Long: `Budget cognitivo: contabilizar tokens/tempo/custo antes de chamar IA.

Antes de chamar IA: Cognitive Budget {Tokens: 8k, Time: 20s, Cost: $0.05}. O
sistema tenta primeiro: local knowledge → external deterministic search →
cached evidence → existing solution. Só então chama o modelo — e apenas
enquanto o budget permitir. Se conseguir resolver sem IA: AI calls: 0,
Cost: $0.

Comando somente de leitura (diagnóstico): não altera nenhum estado.

Subcomandos:
  default                        Mostra o budget cognitivo padrão
  check --tokens N --time D      Dry-run: o consumo proposto cabe no budget?
                                 (dentro/acima), com --cost opcional`,
		Example: `  cosca budget default
  cosca budget check --tokens 500 --time 1.2s
  cosca budget check --tokens 500 --time 1.2s --cost 0.01
  cosca budget check --tokens 9000 --time 1.2s --json`,
	}

	cmd.AddCommand(
		NewBudgetDefaultCommand(),
		NewBudgetCheckCommand(),
	)
	return cmd
}

// NewBudgetDefaultCommand cria `cosca budget default`.
func NewBudgetDefaultCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "default",
		Short: "Mostra o budget cognitivo padrão (Tokens: 8k, Tempo: 20s, Custo: $0.05)",
		Long: `Mostra os valores padrão do budget cognitivo do Don:
Tokens: 8.000 | Tempo: 20s | Custo: $0.05 — os limites usados antes de
autorizar uma chamada de IA.`,
		Example: `  cosca budget default
  cosca budget default --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			b := engine.DefaultCognitiveBudget()

			if IsJSONOutput(cmd) {
				return printJSON(cmd, b)
			}

			formatter.Header("Budget cognitivo (default)")
			formatter.KeyValue("MaxTokens", fmt.Sprintf("%d", b.MaxTokens))
			formatter.KeyValue("MaxTempo", b.MaxDuration.String())
			formatter.KeyValue("MaxCusto", fmt.Sprintf("$%.2f", b.MaxCost))
			formatter.Bullet("Estratégia: local knowledge → external deterministic search → cached evidence → existing solution; só então o modelo — e só enquanto o budget permitir.")
			formatter.Bullet("Se resolver sem IA: AI calls: 0, Cost: $0.")
			return nil
		},
	}
}

// NewBudgetCheckCommand cria `cosca budget check --tokens N --time D`.
func NewBudgetCheckCommand() *cobra.Command {
	var (
		tokens int
		timeD  time.Duration
		cost   float64
	)

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Dry-run: um consumo proposto (--tokens, --time, --cost) cabe no budget padrão?",
		Long: `Dry-run de diagnóstico: dado um consumo proposto em --tokens,
--time e --cost, informa se ele cabe (dentro) ou estoura (acima) o budget
cognitivo padrão {Tokens: 8k, Tempo: 20s, Custo: $0.05}, e quais dimensões
estourariam. Somente leitura — não altera nada.`,
		Example: `  cosca budget check --tokens 500 --time 1.2s
  cosca budget check --tokens 9000 --time 1.2s
  cosca budget check --tokens 500 --time 30s --cost 0.01 --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			b := engine.DefaultCognitiveBudget()

			tracker := engine.NewBudgetTracker(b)
			tracker.Record(tokens, timeD, cost)
			exceeded := tracker.Exceeded()

			overBy := make([]string, 0, 3)
			if tokens > b.MaxTokens {
				overBy = append(overBy, "tokens")
			}
			if timeD > b.MaxDuration {
				overBy = append(overBy, "tempo")
			}
			if cost > b.MaxCost {
				overBy = append(overBy, "custo")
			}

			if IsJSONOutput(cmd) {
				return printJSON(cmd, map[string]interface{}{
					"within":     !exceeded,
					"tokens":     tokens,
					"time":       timeD.String(),
					"cost":       cost,
					"max_tokens": b.MaxTokens,
					"max_time":   b.MaxDuration.String(),
					"max_cost":   b.MaxCost,
					"over_by":    overBy,
				})
			}

			formatter.Header("Budget check (default)")
			formatter.KeyValue("Tokens", fmt.Sprintf("%d / %d", tokens, b.MaxTokens))
			formatter.KeyValue("Tempo", fmt.Sprintf("%s / %s", timeD, b.MaxDuration))
			formatter.KeyValue("Custo", fmt.Sprintf("$%.4f / $%.4f", cost, b.MaxCost))
			if exceeded {
				formatter.Error(fmt.Sprintf("Acima do orçamento — estouraria: %s", strings.Join(overBy, ", ")))
				return nil
			}
			formatter.Success("Dentro do orçamento")
			return nil
		},
	}

	cmd.Flags().IntVar(&tokens, "tokens", 0, "tokens estimados do consumo")
	cmd.Flags().DurationVar(&timeD, "time", 0, "tempo estimado do consumo (ex.: 1.2s, 30s)")
	cmd.Flags().Float64Var(&cost, "cost", 0, "custo estimado em USD (ex.: 0.01)")
	return cmd
}
