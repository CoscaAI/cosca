//
// `cosca acquisition budget` — Orçamento de aquisição: contabilizar fontes,
// arquivos, rede, tempo e tokens de IA antes de continuar a busca de evidência
// externa.
//
// Regra do Don: "Knowledge Acquisition Budget: max sources 8, max files 100,
// max network 20 MB, max time 30 sec, max AI tokens 8k. Se não conseguir
// evidência suficiente: 'Não consegui validar com confiança dentro do
// orçamento. Preciso da sua decisão.'" Isso impede o sistema de entrar numa
// espiral: não sei → busca → não sabe → busca mais → contexto explode →
// custo explode.
//
// Comando apenas de leitura/diagnóstico — não altera nenhum estado.
//
// Subcomandos:
//   default                                   Mostra o orçamento de aquisição
//                                             padrão
//   check --sources N --files N --network-mb N --time D
//                                             Dry-run: um consumo proposto
//                                             caberia no orçamento padrão?
//                                             (dentro/acima)
//

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/acquisition"
)

// NewAcquisitionCommand cria a árvore de comandos `cosca acquisition`.
func NewAcquisitionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "acquisition",
		Short: "Orçamento de aquisição — contabiliza fontes/arquivos/rede/tempo/IA antes de buscar evidência externa",
		Long: `Orçamento de aquisição de evidência externa.
Em subdiretório (subcomando) budget, ` + "`cosca acquisition budget`" + ` contabiliza
o consumo da busca: fontes, arquivos, rede, tempo e tokens de IA — e impede o
sistema de entrar numa espiral: não sei → busca → não sabe → busca mais →
contexto explode → custo explode.

Comando somente de leitura (diagnóstico): não altera nenhum estado.

Subcomandos:
  default                        Mostra o orçamento de aquisição padrão
  budget check --sources N       Dry-run: o consumo proposto cabe no orçamento?
                                 (dentro/acima), com --files/--network-mb/--time
                                 opcionais`,
		Example: `  cosca acquisition budget default
  cosca acquisition budget check --sources 3 --files 2 --network-mb 1 --time 1.2s
  cosca acquisition budget check --sources 10 --files 5 --network-mb 3 --time 1.2s
  cosca acquisition budget check --sources 10 --files 5 --network-mb 3 --time 1.2s --json`,
	}

	cmd.AddCommand(
		NewAcquisitionBudgetCommand(),
	)
	return cmd
}

// NewAcquisitionBudgetCommand cria `cosca acquisition budget`.
func NewAcquisitionBudgetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "budget",
		Short: "Orçamento de aquisição (Fontes: 8, Arquivos: 100, Rede: 20MB, Tempo: 30s, IA: 8k)",
		Long: `Orçamento de aquisição de evidência externa do Don:
{Fontes: 8, Arquivos: 100, Rede: 20MB, Tempo: 30s, IA: 8k}. O sistema busca
evidência apenas dentro desses limites; se não conseguir validar com
confiança, devolve: "Não consegui validar com confiança dentro do orçamento.
Preciso da sua decisão."

Comando somente de leitura (diagnóstico): não altera nenhum estado.

Subcomandos:
  default                        Mostra o orçamento de aquisição padrão
  check --sources N --files N    Dry-run: o consumo proposto cabe no orçamento?
    --network-mb N --time D         (dentro/acima)`,
		Example: `  cosca acquisition budget default
  cosca acquisition budget check --sources 3 --files 2 --network-mb 1 --time 1.2s
  cosca acquisition budget check --sources 10 --files 5 --network-mb 3 --time 1.2s`,
	}

	cmd.AddCommand(
		NewAcquisitionBudgetDefaultCommand(),
		NewAcquisitionBudgetCheckCommand(),
	)
	return cmd
}

// NewAcquisitionBudgetDefaultCommand cria `cosca acquisition budget default`.
func NewAcquisitionBudgetDefaultCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "default",
		Short: "Mostra o orçamento de aquisição padrão (Fontes: 8, Arquivos: 100, Rede: 20MB, Tempo: 30s, IA: 8k)",
		Long: `Mostra os valores padrão do orçamento de aquisição do Don:
Fontes: 8 | Arquivos: 100 | Rede: 20MB | Tempo: 30s | IA: 8k — os limites
usados antes de autorizar nova busca de evidência externa.`,
		Example: `  cosca acquisition budget default
  cosca acquisition budget default --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			b := acquisition.DefaultAcquisitionBudget()

			if IsJSONOutput(cmd) {
				return printJSON(cmd, b)
			}

			formatter.Header("Orçamento de aquisição (default)")
			formatter.KeyValue("MaxFontes", fmt.Sprintf("%d", b.MaxSources))
			formatter.KeyValue("MaxArquivos", fmt.Sprintf("%d", b.MaxFiles))
			formatter.KeyValue("MaxRede", fmt.Sprintf("%dMB", b.MaxNetworkMB))
			formatter.KeyValue("MaxTempo", b.MaxTime.String())
			formatter.KeyValue("MaxTokensIA", fmt.Sprintf("%dk", b.MaxAITokens/1000))
			formatter.Bullet("Se não conseguir evidência suficiente: \"Não consegui validar com confiança dentro do orçamento. Preciso da sua decisão.\" — impede a espiral: não sei → busca → não sabe → busca mais.")
			return nil
		},
	}
}

// NewAcquisitionBudgetCheckCommand cria `cosca acquisition budget check`.
func NewAcquisitionBudgetCheckCommand() *cobra.Command {
	var (
		sources   int
		files     int
		networkMB int64
		timeD     time.Duration
		aiTokens  int
	)

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Dry-run: um consumo proposto (--sources, --files, --network-mb, --time, --ai-tokens) cabe no orçamento padrão?",
		Long: `Dry-run de diagnóstico: dado um consumo proposto em --sources,
--files, --network-mb, --time e --ai-tokens, informa se ele cabe (dentro) ou
estoura (acima) o orçamento de aquisição padrão {Fontes: 8, Arquivos: 100,
Rede: 20MB, Tempo: 30s, IA: 8k}, e quais dimensões estourariam. Somente
leitura — não altera nada.`,
		Example: `  cosca acquisition budget check --sources 3 --files 2 --network-mb 1 --time 1.2s
  cosca acquisition budget check --sources 10 --files 5 --network-mb 3 --time 1.2s
  cosca acquisition budget check --sources 10 --files 5 --network-mb 3 --time 1.2s --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			b := acquisition.DefaultAcquisitionBudget()

			tracker := acquisition.NewAcquisitionTracker(b)
			for i := 0; i < sources; i++ {
				tracker.RecordSource()
			}
			for i := 0; i < files; i++ {
				tracker.RecordFile()
			}
			tracker.RecordBytes(networkMB << 20)
			tracker.RecordDuration(timeD)
			tracker.RecordAITokens(aiTokens)

			exceeded := tracker.Exceeded()
			overBy := tracker.WhichExceeded()

			if IsJSONOutput(cmd) {
				return printJSON(cmd, map[string]interface{}{
					"within":         !exceeded,
					"sources":        sources,
					"files":          files,
					"network_mb":     networkMB,
					"time":           timeD.String(),
					"ai_tokens":      aiTokens,
					"max_sources":    b.MaxSources,
					"max_files":      b.MaxFiles,
					"max_network_mb": b.MaxNetworkMB,
					"max_time":       b.MaxTime.String(),
					"max_ai_tokens":  b.MaxAITokens,
					"over_by":        overBy,
				})
			}

			formatter.Header("Orçamento de aquisição — check (default)")
			formatter.KeyValue("Fontes", fmt.Sprintf("%d / %d", sources, b.MaxSources))
			formatter.KeyValue("Arquivos", fmt.Sprintf("%d / %d", files, b.MaxFiles))
			formatter.KeyValue("Rede", fmt.Sprintf("%dMB / %dMB", networkMB, b.MaxNetworkMB))
			formatter.KeyValue("Tempo", fmt.Sprintf("%s / %s", timeD, b.MaxTime))
			formatter.KeyValue("Tokens IA", fmt.Sprintf("%d / %dk", aiTokens, b.MaxAITokens/1000))
			if exceeded {
				formatter.Error(fmt.Sprintf("Acima do orçamento — estouraria: %s", strings.Join(overBy, ", ")))
				return nil
			}
			formatter.Success("Dentro do orçamento")
			return nil
		},
	}

	cmd.Flags().IntVar(&sources, "sources", 0, "fontes estimadas do consumo")
	cmd.Flags().IntVar(&files, "files", 0, "arquivos estimados do consumo")
	cmd.Flags().Int64Var(&networkMB, "network-mb", 0, "tráfego de rede estimado em MiB (ex.: 3)")
	cmd.Flags().DurationVar(&timeD, "time", 0, "tempo estimado do consumo (ex.: 1.2s, 30s)")
	cmd.Flags().IntVar(&aiTokens, "ai-tokens", 0, "tokens de IA estimados (ex.: 1000)")
	return cmd
}
