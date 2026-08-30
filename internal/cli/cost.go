//
// `cosca cost` — Token Efficiency: relatório de Useful Work / Tokens por
// agente/task (ADR-031, Frente 1).
//
// Lê o log append-only em `.cosca/cost/records.jsonl`, agrega por
// (agent_id, task_id) e reporta, para cada execução: tokens_total, as
// dimensões do vetor de valor (knowledge_gain, task_progress, artifact_value,
// evidence_gain, decision_gain) e a métrica Useful Work / Tokens.
//
// SAÍDA DECOMPOSTA: as execuções que gastaram muito aparecem primeiro (tokens
// desc), com o vetor de valor — e, se a decomposição do uso (contexto/tools/
// history) já existir, com o "onde estão os tokens". Fase 0: essa decomposição
// ainda NÃO existe no motor (é a Fase 0.1) — o relatório deixa isso explícito.
//
// Somente leitura — não altera nenhum estado.
//

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/cost"
)

// NewCostCommand cria a árvore de comandos `cosca cost`.
func NewCostCommand() *cobra.Command {
	var (
		agentFilter string
		taskFilter  string
		raw         bool
	)

	cmd := &cobra.Command{
		Use:   "cost",
		Short: "Token Efficiency — relatório de Useful Work / Tokens por agente/task",
		Long: `Token Efficiency (ADR-031): para cada execução registrada, mostra
tokens_total, as dimensões do vetor de valor (knowledge_gain, task_progress,
artifact_value, evidence_gain, decision_gain) e a métrica Useful Work / Tokens
(dimensões somadas ÷ tokens consumidos).

Lê o log append-only em .cosca/cost/records.jsonl (gerado por 'cosca run') e
agrega por (agent_id, task_id). Somente leitura — não altera nenhum estado.

SAÍDA DECOMPOSTA: as execuções que gastaram muito aparecem primeiro (tokens
desc), com o vetor de valor. Em Fase 0 a decomposição do uso (contexto/tools/
history) ainda não existe no motor — o relatório informa isso explicitamente
(a Fase 0.1 seria decompor o uso).

Limitação honesta: se nenhuma execução foi registrada (log não existe), o
relatório mostra 0 execuções e orienta a rodar 'cosca run' para instrumentar.`,
		Example: `  cosca cost
  cosca cost --agent cosca-backend
  cosca cost --task T-4821
  cosca cost --json
  cosca cost --raw`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			coscaDir := resolveCoscaDir()
			store := cost.ForCoscaDir(coscaDir)

			rep, err := buildCostData(store, agentFilter, taskFilter)
			if err != nil {
				return fmt.Errorf("cost report failed: %w", err)
			}

			if useJSON {
				return printJSON(cmd, rep)
			}

			printCostReport(formatter, rep, raw)
			return nil
		},
	}

	cmd.Flags().StringVar(&agentFilter, "agent", "", "filtrar por agent_id (ex.: cosca-backend)")
	cmd.Flags().StringVar(&taskFilter, "task", "", "filtrar por task_id (ex.: T-4821)")
	cmd.Flags().BoolVar(&raw, "raw", false, "mostrar cada execução (não só o agregado)")

	// Subcomando `cosca cost tasks` — agregação CAUSAL por task: as execuções
	// do mesmo task_id (write_file + build) agrupadas em uma TaskRecord com
	// Executions[] aninhadas. É a unidade de observabilidade que o Auto-Audit
	// precisa para responder "qual tarefa gerou o artefato e qual validou".
	cmd.AddCommand(newCostTasksCommand())

	return cmd
}

// newCostTasksCommand cria o subcomando `cosca cost tasks` que agrega por
// TASK (unidade causal), preservando cada execução aninhada.
func newCostTasksCommand() *cobra.Command {
	var agentFilter, taskFilter string
	var raw bool

	cmd := &cobra.Command{
		Use:   "tasks",
		Short: "Agregação por TASK (unidade causal) — execuções aninhadas por task_id",
		Long: `Token Efficiency por TASK (ADR-031): agrupa as execuções por task_id,
preservando cada execução (write_file, build, inspect) dentro do TaskRecord.
É o que permite ao Auto-Audit enxergar a tarefa como um todo em vez de runs
separados sem vínculo causal.

Lê o log append-only em .cosca/cost/records.jsonl (gerado por 'cosca run').
Somente leitura — não altera nenhum estado.`,
		Example: `  cosca cost tasks
  cosca cost tasks --agent cosca-frontend
  cosca cost tasks --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			coscaDir := resolveCoscaDir()
			store := cost.ForCoscaDir(coscaDir)

			tasks, err := buildCostTasksData(store, agentFilter, taskFilter)
			if err != nil {
				return fmt.Errorf("cost tasks report failed: %w", err)
			}

			if useJSON {
				return printJSON(cmd, tasks)
			}

			printCostTasksReport(formatter, tasks, raw)
			return nil
		},
	}

	cmd.Flags().StringVar(&agentFilter, "agent", "", "filtrar por agent_id")
	cmd.Flags().StringVar(&taskFilter, "task", "", "filtrar por task_id")
	cmd.Flags().BoolVar(&raw, "raw", false, "mostrar cada execução aninhada")

	return cmd
}

// buildCostTasksData carrega os registros e agrega por TASK (causal).
func buildCostTasksData(store *cost.Store, agentFilter, taskFilter string) ([]cost.TaskRecord, error) {
	records, err := store.Load()
	if err != nil {
		return nil, err
	}
	filtered := filterCostRecords(records, agentFilter, taskFilter)
	return cost.AggregateTasks(filtered), nil
}

// printCostTasksReport renderiza o relatório causal por task em texto.
func printCostTasksReport(formatter *OutputFormatter, tasks []cost.TaskRecord, raw bool) {
	if len(tasks) == 0 {
		formatter.Warning("Nenhuma execução registrada (log .cosca/cost/records.jsonl vazio ou não existe).")
		return
	}

	formatter.Header("Token Efficiency por TASK — Useful Work / Tokens (ADR-031)")
	formatter.KeyValue("Tarefas", fmt.Sprintf("%d", len(tasks)))

	for _, t := range tasks {
		formatter.Println("")
		formatter.Header(fmt.Sprintf("Task: %s / %s", t.TaskID, t.AgentID))
		formatter.KeyValue("Execuções", fmt.Sprintf("%d", t.Runs))
		formatter.KeyValue("Tokens total", fmt.Sprintf("%d", t.TokensTotal))
		formatter.KeyValue("Duration", fmt.Sprintf("%dms", t.DurationMs))
		formatter.KeyValue("Useful Work", fmt.Sprintf("%.4f", t.UsefulWork()))
		formatter.KeyValue("Efficiency", fmt.Sprintf("%.8f", t.Efficiency()))
		printWorkDims(formatter, t.KnowledgeGain, t.TaskProgress, t.ArtifactValue, t.EvidenceGain, t.DecisionGain)

		if raw && len(t.Executions) > 0 {
			formatter.Println("")
			formatter.Header("Execuções")
			for i, e := range t.Executions {
				formatter.Println("")
				formatter.KeyValue(fmt.Sprintf("Execução %d", i+1), fmt.Sprintf("%s (%s)", e.Status, e.Phase))
				formatter.KeyValue("Tool", e.Tool)
				formatter.KeyValue("Tokens", fmt.Sprintf("%d/%d/%d", e.InputTokens, e.OutputTokens, e.TokensTotal))
				formatter.KeyValue("Duration", fmt.Sprintf("%dms", e.DurationMs))
				printWorkDims(formatter, e.KnowledgeGain, e.TaskProgress, e.ArtifactValue, e.EvidenceGain, e.DecisionGain)
			}
		}
	}

	formatter.Println("")
	formatter.Bullet("Métricas = Useful Work (dimensões somadas) ÷ tokens consumidos, por TAREFA (unidade causal).")
}

// (resolveCoscaDir é definido em circadian.go — reutilizado aqui.)

// buildCostData carrega os registros, filtra por agente/task e agrega.
// Separado do RunE para ser testável.
func buildCostData(store *cost.Store, agentFilter, taskFilter string) (*cost.Report, error) {
	records, err := store.Load()
	if err != nil {
		return nil, err
	}
	filtered := filterCostRecords(records, agentFilter, taskFilter)
	return cost.Aggregate(filtered), nil
}

// filterCostRecords filtra os registros pelos filtros de agente e task.
func filterCostRecords(records []cost.Record, agentFilter, taskFilter string) []cost.Record {
	if agentFilter == "" && taskFilter == "" {
		return records
	}
	out := make([]cost.Record, 0, len(records))
	for _, r := range records {
		if agentFilter != "" && r.AgentID != agentFilter {
			continue
		}
		if taskFilter != "" && r.TaskID != taskFilter {
			continue
		}
		out = append(out, r)
	}
	return out
}

// printCostReport renderiza o relatório em texto (formatter injetado).
func printCostReport(formatter *OutputFormatter, rep *cost.Report, raw bool) {
	if rep.Runs == 0 {
		formatter.Warning("Nenhuma execução registrada (o log .cosca/cost/records.jsonl não existe ou está vazio).")
		formatter.Bullet("Rode `cosca run \"...\"` para instrumentar e depois `cosca cost`.")
		formatter.Bullet("Ou use --agent / --task para filtrar execuções já registradas.")
		return
	}

	formatter.Header("Token Efficiency — Useful Work / Tokens (ADR-031)")

	decStatus := "NÃO (Fase 0.1: decompor o uso é o próximo passo)"
	if rep.Decomposed {
		decStatus = "SIM (contexto/tools/history presentes)"
	}
	formatter.KeyValue("Execuções registradas", fmt.Sprintf("%d", rep.Runs))
	formatter.KeyValue("Grupos (agente/task)", fmt.Sprintf("%d", len(rep.Grouped)))
	formatter.KeyValue("Decomposição do uso", decStatus)

	formatter.Println("")
	formatter.Header("TOTAL")
	formatter.KeyValue("Tokens total", fmt.Sprintf("%d", rep.Total.TokensTotal))
	formatter.KeyValue("Useful Work (soma das dimensões)", fmt.Sprintf("%.4f", rep.Total.UsefulWork()))
	formatter.KeyValue("Efficiency (Useful Work / tokens)", fmt.Sprintf("%.8f", rep.Total.Efficiency()))
	printWorkDims(formatter, rep.Total.KnowledgeGain, rep.Total.TaskProgress, rep.Total.ArtifactValue, rep.Total.EvidenceGain, rep.Total.DecisionGain)

	for _, g := range rep.Grouped {
		formatter.Println("")
		formatter.Header(fmt.Sprintf("%s / %s", g.AgentID, g.TaskID))
		formatter.KeyValue("Execuções", fmt.Sprintf("%d", g.Runs))
		formatter.KeyValue("Tokens total", fmt.Sprintf("%d", g.TokensTotal))
		formatter.KeyValue("Useful Work", fmt.Sprintf("%.4f", g.UsefulWork()))
		formatter.KeyValue("Efficiency", fmt.Sprintf("%.8f", g.Efficiency()))
		printWorkDims(formatter, g.KnowledgeGain, g.TaskProgress, g.ArtifactValue, g.EvidenceGain, g.DecisionGain)
	}

	// SAÍDA DECOMPOSTA — execuções individuais (vetor de valor por execução).
	if raw && len(rep.Commands) > 0 {
		formatter.Println("")
		formatter.Header("Execuções (decomposto por registro)")
		formatter.Bullet("Tokens totais por execução + vetor de valor. A decomposição base/contexto/tools/history não está preenchida em Fase 0.")
		for _, c := range rep.Commands {
			formatter.Println("")
			formatter.KeyValue("Execução", fmt.Sprintf("%s / %s", c.AgentID, c.TaskID))
			formatter.KeyValue("Tokens (input/output/total)", fmt.Sprintf("%d / %d / %d", c.InputTokens, c.OutputTokens, c.TokensTotal))
			if c.ContextTokens > 0 || c.ToolTokens > 0 || c.HistoryTokens > 0 {
				formatter.KeyValue("Contexto/Tools/History", fmt.Sprintf("%d / %d / %d", c.ContextTokens, c.ToolTokens, c.HistoryTokens))
			} else {
				formatter.KeyValue("Contexto/Tools/History", "— (não decomposto — Fase 0.1)")
			}
			formatter.KeyValue("Duration", fmt.Sprintf("%dms", c.DurationMs))
			formatter.KeyValue("Useful Work", fmt.Sprintf("%.4f", c.UsefulWork()))
			formatter.KeyValue("Efficiency", fmt.Sprintf("%.8f", c.Efficiency()))
			printWorkDims(formatter, c.KnowledgeGain, c.TaskProgress, c.ArtifactValue, c.EvidenceGain, c.DecisionGain)
		}
	}

	formatter.Println("")
	formatter.Bullet("Métrica = Useful Work (dimensões somadas) ÷ tokens consumidos.")
	formatter.Bullet("Duas execuções, mesma taxa de sucesso, eficiências diferentes → o sistema aprende qual agente é melhor para aquele tipo de tarefa (ADR-031).")
}

// printWorkDims mostra as 5 dimensões do vetor de valor, apenas as não-zero.
func printWorkDims(formatter *OutputFormatter, kg, tp float64, av, eg, dg int) {
	dims := make([]string, 0, 5)
	if kg != 0 {
		dims = append(dims, fmt.Sprintf("knowledge_gain=%.2f", kg))
	}
	if tp != 0 {
		dims = append(dims, fmt.Sprintf("task_progress=%.2f", tp))
	}
	if av != 0 {
		dims = append(dims, fmt.Sprintf("artifact_value=%d", av))
	}
	if eg != 0 {
		dims = append(dims, fmt.Sprintf("evidence_gain=%d", eg))
	}
	if dg != 0 {
		dims = append(dims, fmt.Sprintf("decision_gain=%d", dg))
	}
	if len(dims) == 0 {
		formatter.KeyValue("Dimensões de valor", "— (nenhuma registrada)")
		return
	}
	formatter.KeyValue("Dimensões de valor", strings.Join(dims, " | "))
}
