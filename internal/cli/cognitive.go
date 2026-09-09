//
// `cosca cognitive` — auto-inspeção do Kernel (métricas da L13, em código).
//
// Mede a QUALIDADE das decisões do Kernel (taxa de reversão, categorias de
// causa raiz, cobertura de proteção) e a PROFUNDIDADE de planejamento
// (horizonte). É a Lição L13 em número: "não é zero reversões; é zero
// reversões da MESMA classe" — medir é a vacina contra o loop que matou o
// "outro Kernel".
//
// Subcomandos:
//   quality   Qualidade de decisão (taxa de sucesso/reversão, categorias, cobertura)
//   horizon   Profundidade preditiva de um plano (--steps N)
//

package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/cognitive"
)

// NewCognitiveCommand cria a árvore de comandos `cosca cognitive`.
func NewCognitiveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cognitive",
		Short: "Auto-inspeção do Kernel — qualidade de decisão (L13) e horizonte",
		Long: `Auto-inspeção do Kernel: mede a qualidade das decisões (taxa de reversão,
categorias de causa raiz, cobertura de proteção) e a profundidade de
planejamento (horizonte). É a Lição L13 em número — a disciplina que a casa
construiu, agora com métrica.`,
		Example: `  cosca cognitive quality
  cosca cognitive horizon --steps 10`,
	}

	cmd.AddCommand(
		NewCognitiveQualityCommand(),
		NewCognitiveHorizonCommand(),
	)
	return cmd
}

// NewCognitiveQualityCommand cria `cosca cognitive quality`.
func NewCognitiveQualityCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "quality",
		Short: "Qualidade de decisão do Kernel (taxa de reversão, categorias, cobertura)",
		Long: `Métrica de qualidade de decisão (B3/L13): taxa de sucesso, taxa de
reversão, distribuição por categoria de causa raiz e cobertura de proteção.
A meta da L13: zero reversões da MESMA classe — não zero reversões no total.
Aqui você informa o registro de decisões (em breve automático).`,
		Example: `  cosca cognitive quality`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			// Registro de decisões do Kernel. Por ora, exemplo/dados de demonstração
			// (a captura automática do trace/decisions será o próximo passo).
			records := []cognitive.DecisionRecord{}
			protected := map[string]bool{cognitive.CatBypassGovernance: true}

			q := cognitive.MeasureDecisionQuality(records, protected)

			if useJSON {
				return printJSON(cmd, q)
			}

			formatter.Header("QUALIDADE DE DECISÃO (L13/B3) — Kernel")
			formatter.KeyValue("Decisões", strconv.Itoa(q.Decisions))
			formatter.KeyValue("Reversões", strconv.Itoa(q.Reverted))
			formatter.KeyValue("Taxa de sucesso", fmt.Sprintf("%.1f%%", q.SuccessRate*100))
			formatter.KeyValue("Taxa de reversão", fmt.Sprintf("%.1f%%", q.RevertRate*100))
			formatter.KeyValue("Cobertura de proteção", fmt.Sprintf("%.0f%% (meta: ≥80%)", q.ProtectionCoverage*100))
			formatter.Warning("Meta da L13: zero reversões da MESMA classe. Medir é a vacina contra o loop.")
			return nil
		},
	}
}

// NewCognitiveHorizonCommand cria `cosca cognitive horizon --steps N`.
func NewCognitiveHorizonCommand() *cobra.Command {
	var steps int
	cmd := &cobra.Command{
		Use:   "horizon",
		Short: "Profundidade preditiva de um plano (horizonte)",
		Long: `Mede o horizonte (profundidade preditiva) de um plano: quantos passos de
consequência a cadeia projeta. 0-2 = míope (resolve sintoma, cria problemas
downstream) · 15+ = visionário (antecipa cascata). É a meta-métrica: não o que
você planeja, mas ATÉ ONDE você planeja.`,
		Example: `  cosca cognitive horizon --steps 10`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			depth := cognitive.PlanDepth(steps)
			level := cognitive.HorizonLevel(depth)

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"steps": steps, "depth": depth, "level": level,
				})
			}

			formatter.Header(fmt.Sprintf("HORIZONTE — %d passo(s)", steps))
			formatter.KeyValue("Nível", fmt.Sprintf("%d/4", depth))
			formatter.KeyValue("Leitura", level)
			formatter.Warning("Quanto mais profundo o horizonte, melhor o planejamento — e menos problemas downstream.")
			return nil
		},
	}
	cmd.Flags().IntVar(&steps, "steps", 3, "quantos passos de consequência o plano projeta (0-25+)")
	return cmd
}
