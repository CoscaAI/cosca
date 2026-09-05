//
// `cosca shadow` — Cognitive Shadow Mode (ADR-033): o cérebro observa o
// próprio processo de deliberação (ADR-032) SEM ganhar autoridade.
//
// Esta é a superfície para consumir o relatório-ouro do Shadow mode: o "decide
// e REGISTRA" contrafactual (o que o Kernel teria respondido/escalado) fica em
// `.cosca/shadow/records.jsonl` (append-only, same padrão do cost, ADR-015).
//
// Subcomandos:
//   - `cosca shadow` / `cosca shadow summary` — resumo agregado (distribuição de
//     decisões, taxa de escalada, auto-resolução, confiança média).
//   - `cosca shadow report` — relatório-ouro (auto-resolução %, escalada,
//     histograma de confiança p/ calibrar thresholds).
//   - `cosca shadow list --request <id> [--agent <name>]` — observações
//     individuais de um request.
//
// Lê `.cosca/shadow/records.jsonl`. Somente leitura — não altera nenhum estado.
// Reuse de `internal/shadow` (Summary/Report/List) — nenhuma lógica nova de
// agregação aqui.
//

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/shadow"
)

// NewShadowCommand cria a árvore de comandos `cosca shadow`.
func NewShadowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shadow",
		Short: "Cognitive Shadow Mode — relatório-ouro da deliberação contrafactual (ADR-033)",
		Long: `Cognitive Shadow Mode (ADR-033): o cérebro observa o próprio processo de
deliberação SEM ganhar autoridade. Para cada execução, o Shadow registra qual
TERIA sido a decisão do Kernel (contrafactual): a taxonomia (ShadowDecision),
o "teria escalado?", a confiança, as evidências e o motivo.

Lê o log append-only em .cosca/shadow/records.jsonl (gerado por 'cosca run'
com orchestration.deliberation.shadow_mode: true). Somente leitura — não
altera nenhum estado.

Sem subcomando, equivale a 'cosca shadow summary' (resumo agregado).`,
		Example: `  cosca shadow
  cosca shadow summary
  cosca shadow report
  cosca shadow list --request <id> [--agent cosca-backend]
  cosca shadow --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runShadowSummary(cmd)
		},
	}

	cmd.AddCommand(newShadowSummaryCommand())
	cmd.AddCommand(newShadowReportCommand())
	cmd.AddCommand(newShadowListCommand())

	return cmd
}

// newShadowSummaryCommand cria o subcomando `cosca shadow summary`.
func newShadowSummaryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Resumo agregado da deliberação contrafactual",
		Long: `Resumo agregado das observações do Cognitive Shadow Mode: distribuição de
decisões (ShadowDecision), taxa de escalada (teria chamado a LLM), taxa de
auto-resolução (EMIT_OK), confiança e convergência médias.

Somente leitura. Requer orchestration.deliberation.shadow_mode: true no
config e execuções instrumentadas por 'cosca run'.`,
		Example: `  cosca shadow summary
  cosca shadow summary --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runShadowSummary(cmd)
		},
	}
	return cmd
}

// newShadowReportCommand cria o subcomando `cosca shadow report`.
func newShadowReportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Relatório-ouro da deliberação contrafactual (ADR-033 §3.7)",
		Long: `Relatório-ouro do Don (ADR-033 §3.7): a taxa de auto-resolução (porcentagem
dos casos em que o COSCA teria respondido SEM LLM, i.e. EMIT_OK), a taxa de
escalada, e o histograma de confiança — que calibra os thresholds de emissão
(emit_threshold, reservation_threshold).

Somente leitura. Requer shadow_mode habilitado e execuções instrumentadas.`,
		Example: `  cosca shadow report
  cosca shadow report --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runShadowReport(cmd)
		},
	}
	return cmd
}

// newShadowListCommand cria o subcomando `cosca shadow list`.
func newShadowListCommand() *cobra.Command {
	var (
		requestID  string
		agentCheck string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lista observações de um request",
		Long: `Lista as observações do Cognitive Shadow Mode registradas para um request.
Cada linha é um ShadowTrace: a taxonomia da decisão contrafactual, a confiança
e o motivo. Filtro opcional por agent.

Somente leitura. Requer shadow_mode habilitado e execuções instrumentadas.`,
		Example: `  cosca shadow list --request <id>
  cosca shadow list --request <id> --agent cosca-backend
  cosca shadow list --request <id> --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			store := shadow.ForCoscaDir(resolveCoscaDir())

			traces, err := store.List(requestID)
			if err != nil {
				return fmt.Errorf("shadow list failed: %w", err)
			}
			traces = filterShadowByAgent(traces, agentCheck)

			if useJSON {
				return printJSON(cmd, traces)
			}

			printShadowList(formatter, traces)
			return nil
		},
	}

	cmd.Flags().StringVar(&requestID, "request", "", "request_id a listar (obrigatório)")
	cmd.Flags().StringVar(&agentCheck, "agent", "", "filtrar por agent (ex.: cosca-backend)")
	_ = cmd.MarkFlagRequired("request")

	return cmd
}

// runShadowSummary executa o resumo agregado (comando raiz e subcomando summary).
func runShadowSummary(cmd *cobra.Command) error {
	formatter := GetFormatter(cmd)
	useJSON := IsJSONOutput(cmd)

	store := shadow.ForCoscaDir(resolveCoscaDir())

	sum, err := store.Summary()
	if err != nil {
		return fmt.Errorf("shadow summary failed: %w", err)
	}

	if useJSON {
		return printJSON(cmd, sum)
	}

	printShadowSummary(formatter, sum)
	return nil
}

// runShadowReport executa o relatório-ouro.
func runShadowReport(cmd *cobra.Command) error {
	formatter := GetFormatter(cmd)
	useJSON := IsJSONOutput(cmd)

	store := shadow.ForCoscaDir(resolveCoscaDir())

	rep, err := store.Report()
	if err != nil {
		return fmt.Errorf("shadow report failed: %w", err)
	}

	if useJSON {
		return printJSON(cmd, rep)
	}

	printShadowReport(formatter, rep)
	return nil
}

// filterShadowByAgent filtra as observações pelo agent dado.
func filterShadowByAgent(traces []shadow.ShadowTrace, agent string) []shadow.ShadowTrace {
	if agent == "" {
		return traces
	}
	out := make([]shadow.ShadowTrace, 0, len(traces))
	for _, t := range traces {
		if t.Agent == agent {
			out = append(out, t)
		}
	}
	return out
}

// printShadowSummary renderiza o resumo agregado em texto.
func printShadowSummary(formatter *OutputFormatter, sum shadow.Summary) {
	if sum.Total == 0 {
		formatter.Warning("Nenhuma observação do Shadow registrada (.cosca/shadow/records.jsonl vazio ou não existe).")
		formatter.Bullet("Ative `orchestration.deliberation.shadow_mode: true` no config e rode `cosca run \"...\"` para instrumentar.")
		formatter.Bullet("Depois `cosca shadow` para ver o resumo.")
		return
	}

	formatter.Header("Cognitive Shadow Mode — Resumo (ADR-033)")
	formatter.KeyValue("Observações", fmt.Sprintf("%d", sum.Total))
	formatter.KeyValue("Auto-resolução (EMIT_OK)", fmt.Sprintf("%.1f%%", sum.SelfResolveRate*100))
	formatter.KeyValue("Taxa de escalada (teria chamado a LLM)", fmt.Sprintf("%.1f%%", sum.EscalationRate*100))
	formatter.KeyValue("Confiança média", fmt.Sprintf("%.3f", sum.AvgConfidence))
	formatter.KeyValue("Convergência média", fmt.Sprintf("%.3f", sum.AvgConvergence))

	formatter.Println("")
	formatter.Header("Distribuição de decisões")
	if len(sum.Decisions) == 0 {
		formatter.Bullet("— (nenhuma decisão amostrada)")
	} else {
		for _, d := range []shadow.ShadowDecision{
			shadow.EMIT_OK,
			shadow.EMIT_WITH_RESERVATIONS,
			shadow.RETRIEVAL_INSUFFICIENT,
			shadow.ESCALATE,
		} {
			count := sum.Decisions[d.String()]
			pct := 0.0
			if sum.Total > 0 {
				pct = float64(count) / float64(sum.Total) * 100
			}
			formatter.KeyValue(d.String(), fmt.Sprintf("%d (%.1f%%)", count, pct))
		}
		// Decisões fora da taxonomia conhecida (robustez a versões futuras).
		unknown := sum.Decisions
		for _, d := range []shadow.ShadowDecision{
			shadow.EMIT_OK,
			shadow.EMIT_WITH_RESERVATIONS,
			shadow.RETRIEVAL_INSUFFICIENT,
			shadow.ESCALATE,
		} {
			delete(unknown, d.String())
		}
		for k, count := range unknown {
			formatter.KeyValue(k, fmt.Sprintf("%d", count))
		}
	}

	formatter.Println("")
	formatter.Bullet("O Shadow registra o CONTRAFACTUAL: o que o Kernel teria respondido/escalado, sem aplicar autoridade.")
}

// printShadowReport renderiza o relatório-ouro em texto.
func printShadowReport(formatter *OutputFormatter, rep shadow.Report) {
	if rep.Total == 0 {
		formatter.Warning("Nenhuma observação do Shadow registrada (.cosca/shadow/records.jsonl vazio ou não existe).")
		formatter.Bullet("Ative `orchestration.deliberation.shadow_mode: true` no config e rode `cosca run \"...\"` para instrumentar.")
		formatter.Bullet("Depois `cosca shadow report` para ver o relatório-ouro.")
		return
	}

	formatter.Header("Cognitive Shadow — Relatório-Ouro (ADR-033 §3.7)")
	formatter.KeyValue("Observações", fmt.Sprintf("%d", rep.Total))
	formatter.KeyValue("Taxa de auto-resolução (EMIT_OK, sem LLM)", fmt.Sprintf("%.1f%%", rep.SelfResolveRate*100))
	formatter.KeyValue("Taxa de escalada (teria chamado a LLM)", fmt.Sprintf("%.1f%%", rep.EscalationRate*100))
	formatter.KeyValue("Confiança média", fmt.Sprintf("%.3f", rep.AvgConfidence))
	formatter.KeyValue("Convergência média", fmt.Sprintf("%.3f", rep.AvgConvergence))

	formatter.Println("")
	formatter.Header("Distribuição de decisões")
	if len(rep.Decisions) == 0 {
		formatter.Bullet("— (nenhuma decisão amostrada)")
	} else {
		for _, d := range []shadow.ShadowDecision{
			shadow.EMIT_OK,
			shadow.EMIT_WITH_RESERVATIONS,
			shadow.RETRIEVAL_INSUFFICIENT,
			shadow.ESCALATE,
		} {
			count := rep.Decisions[d.String()]
			pct := 0.0
			if rep.Total > 0 {
				pct = float64(count) / float64(rep.Total) * 100
			}
			formatter.KeyValue(d.String(), fmt.Sprintf("%d (%.1f%%)", count, pct))
		}
	}

	formatter.Println("")
	formatter.Header("Histograma de confiança (calibração de thresholds)")
	for _, b := range rep.Histogram {
		formatter.KeyValue(fmt.Sprintf("%.2f–%.2f", b.Low, b.High), fmt.Sprintf("%d", b.Count))
	}

	formatter.Println("")
	formatter.Bullet("Histograma útil para calibrar emit_threshold e reservation_threshold: confiança alta + auto-resolução baixa indica threshold alto demais; confiança baixa + auto-resolução alta indica risco de resposta sem garantia.")
}

// printShadowList renderiza as observações de um request em texto.
func printShadowList(formatter *OutputFormatter, traces []shadow.ShadowTrace) {
	if len(traces) == 0 {
		formatter.Warning("Nenhuma observação encontrada para os filtros pedidos.")
		formatter.Bullet("Verifique o request_id (--request) e o filtro --agent.")
		formatter.Bullet("Ou a sombra ainda não foi instrumentada (shadow_mode: true + cosca run).")
		return
	}

	formatter.Header("Cognitive Shadow — Observações (ADR-033)")
	formatter.KeyValue("Observações", fmt.Sprintf("%d", len(traces)))

	rows := make([][]string, 0, len(traces))
	for _, t := range traces {
		agent := t.Agent
		if agent == "" {
			agent = "KERNEL"
		}
		rows = append(rows, []string{
			t.RequestID,
			agent,
			string(t.Decision),
			fmt.Sprintf("%.3f", t.Confidence),
			t.Reason,
		})
	}
	formatter.Table(
		[]string{"RequestID", "Agent", "Decision", "Confidence", "Reason"},
		rows,
	)

	formatter.Println("")
	formatter.Bullet("O Shadow registra o CONTRAFACTUAL: a decisão que o Kernel TERIA tomado, sem autoridade.")
}
