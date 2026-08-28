package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/performance"
)

// processStarted é capturado no pacote init — o mais cedo possível no runtime Go
// (após bootstrap do runtime, antes de main()). Usado para medir o boot real.
var processStarted = time.Now()

// perfReport é o relatório estruturado de `cosca perf`.
type perfReport struct {
	Measurement string            `json:"measurement" yaml:"measurement"`
	Name        string            `json:"name" yaml:"name"`
	Samples     int               `json:"samples" yaml:"samples"`
	MedianMs    float64           `json:"median_ms" yaml:"median_ms"`
	P95Ms       float64           `json:"p95_ms" yaml:"p95_ms"`
	MinMs       float64           `json:"min_ms" yaml:"min_ms"`
	MaxMs       float64           `json:"max_ms" yaml:"max_ms"`
	BudgetMs    float64           `json:"budget_ms" yaml:"budget_ms"`
	Decision    string            `json:"decision" yaml:"decision"`
	Epistemic   string            `json:"epistemic_class" yaml:"epistemic_class"`
	Provenance  performance.Provenance `json:"provenance" yaml:"provenance"`
}

// NewPerfCommand cria a árvore de comandos `cosca perf`.
func NewPerfCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "perf",
		Short: "Contratos de desempenho da família (ex.: cold-start)",
		Long: `Contratos de desempenho (Performance Brain F9). Mede o cold-path como
SLO de primeira classe — MEASURED, nunca FACT (L319). Determinístico (I1).

Subcomandos:
  cold-start   Mede boot → primeiro resultado e valida contra um orçamento.`,
		Example: `  cosca perf cold-start
  cosca perf cold-start --budget 250 --json`,
	}

	cmd.AddCommand(NewPerfColdStartCommand())
	return cmd
}

// NewPerfColdStartCommand cria `cosca perf cold-start`.
func NewPerfColdStartCommand() *cobra.Command {
	var budgetMs float64
	var storePath string

	cmd := &cobra.Command{
		Use:   "cold-start",
		Short: "Medir cold-start (boot → primeiro resultado) e validar contra orçamento",
		Long: `Mede o tempo decorrido desde o início do processo até o primeiro
resultado (boot + init de config + dispatch), grava como MEASURED no histórico
(append-only) e decide PASS/WARN/FAIL contra o orçamento de forma determinística
(I1 — sem LLM).

A mediana do histórico é a régua (L316: nunca declarar por uma execução única).
Epistemologia L319: isto é MEASURED, não FACT.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			// Amostra do boot deste processo (boot → chegada neste handler).
			sample := performance.NewColdStartSample("cosca.cold-start", time.Since(processStarted), "cosca perf cold-start")

			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getwd: %w", err)
			}
			coscaDir := filepath.Join(dir, ".cosca")
			if storePath == "" {
				storePath = filepath.Join(coscaDir, "performance", "coldstart.jsonl")
			}
			_ = os.MkdirAll(filepath.Dir(storePath), 0o755)

			store := performance.NewColdStartStore(storePath)
			if err := store.Record(sample); err != nil {
				formatter.Warning("Falha ao persistir amostra (continua em memória): " + err.Error())
			}

			history, err := store.LoadAll()
			if err != nil {
				return fmt.Errorf("load cold-start history: %w", err)
			}
			summary := performance.SummarizeColdStart(history)
			decision := performance.EvaluateColdStart(summary, budgetMs)

			report := perfReport{
				Measurement: "cold-start",
				Name:        sample.Name,
				Samples:     summary.Samples,
				MedianMs:    summary.MedianMs,
				P95Ms:       summary.P95Ms,
				MinMs:       summary.MinMs,
				MaxMs:       summary.MaxMs,
				BudgetMs:    budgetMs,
				Decision:    string(decision),
				Epistemic:   string(summary.Epistemic),
				Provenance:  sample.Provenance,
			}

			if useJSON {
				return printJSON(cmd, report)
			}

			formatter.Header("Cold-Start Contract (boot → primeiro resultado)")
			formatter.KeyValue("Medida", report.Measurement)
			formatter.KeyValue("Amostras", fmt.Sprintf("%d", report.Samples))
			formatter.KeyValue("Mediana", fmt.Sprintf("%.0f ms", report.MedianMs))
			formatter.KeyValue("P95", fmt.Sprintf("%.0f ms", report.P95Ms))
			formatter.KeyValue("Mín/Máx", fmt.Sprintf("%.0f / %.0f ms", report.MinMs, report.MaxMs))
			formatter.KeyValue("Orçamento", fmt.Sprintf("%.0f ms", report.BudgetMs))
			formatter.KeyValue("Classe epistêmica", string(report.Epistemic))
			formatter.KeyValue("Veredicto", strings.ToUpper(string(decision)))
			formatter.KeyValue("Histórico", storePath)

			if decision == performance.ColdStartFail {
				formatter.Warning("Cold-start acima de 2× o orçamento — regressão possível no caminho de boot.")
			} else if decision == performance.ColdStartWarn {
				formatter.Warning("Cold-start acima do orçamento — acompanhar antes de virar regressão.")
			}

			return nil
		},
	}

	cmd.Flags().Float64Var(&budgetMs, "budget", 500, "orçamento de cold-start em ms (mediana)")
	cmd.Flags().StringVar(&storePath, "store", "", "caminho do histórico JSONL (default: .cosca/performance/coldstart.jsonl)")
	return cmd
}
