package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/orchestration"
)

// ─── Metrics Command ──────────────────────────────────────────────────────────

// engineHolder holds a reference to the orchestration engine for metrics display.
// It is set by the run command when the engine is created.
var engineHolder *orchestration.Engine

// SetMetricsEngine stores the engine reference for use by the metrics command.
func SetMetricsEngine(e *orchestration.Engine) {
	engineHolder = e
}

// NewMetricsCommand creates the `cosca metrics` command.
func NewMetricsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "metrics",
		Short: "Show orchestration metrics",
		Long: `Display a snapshot of the current orchestration engine metrics.

This command shows aggregated statistics including:
  - Request counts and success rates
  - LLM call statistics (calls, tokens, errors, fallbacks)
  - Router decision breakdown
  - Memory-Augmented Generation (MAG) statistics
  - Per-stage timing breakdown`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			if engineHolder == nil {
				formatter.Warning("No orchestration engine is currently active.")
				formatter.Println("Metrics are only available after running a request via 'cosca run'.")
				return nil
			}

			snapshot := engineHolder.Metrics().Snapshot()

			if useJSON {
				return printJSON(cmd, snapshot)
			}

			printMetricsSnapshot(formatter, snapshot)
			return nil
		},
	}
}

// printMetricsSnapshot prints a formatted metrics snapshot.
func printMetricsSnapshot(formatter *OutputFormatter, snap orchestration.MetricsSnapshot) {
	formatter.Header("Orchestration Metrics")

	formatter.KeyValue("Timestamp", snap.Timestamp.Format("2006-01-02 15:04:05"))

	formatter.Header("Request Statistics")
	formatter.KeyValue("Total Requests", fmt.Sprintf("%d", snap.TotalRequests))
	formatter.KeyValue("Successful", fmt.Sprintf("%d", snap.SuccessfulRequests))
	formatter.KeyValue("Failed", fmt.Sprintf("%d", snap.FailedRequests))
	formatter.KeyValue("Success Rate", fmt.Sprintf("%.1f%%", snap.SuccessRate*100))
	formatter.KeyValue("Avg Duration", fmt.Sprintf("%.2fms", snap.AvgDurationMs))

	formatter.Header("LLM Statistics")
	formatter.KeyValue("Total LLM Calls", fmt.Sprintf("%d", snap.TotalLLMCalls))
	formatter.KeyValue("Total LLM Tokens", fmt.Sprintf("%d", snap.TotalLLMTokens))
	formatter.KeyValue("LLM Errors", fmt.Sprintf("%d", snap.LLMErrors))
	formatter.KeyValue("LLM Fallbacks", fmt.Sprintf("%d", snap.LLMFallbacks))

	formatter.Header("Router Breakdown")
	formatter.KeyValue("Explicit Hits", fmt.Sprintf("%d", snap.RouterExplicitHits))
	formatter.KeyValue("Keyword Hits", fmt.Sprintf("%d", snap.RouterKeywordHits))
	formatter.KeyValue("Search Hits", fmt.Sprintf("%d", snap.RouterSearchHits))
	formatter.KeyValue("Fallbacks", fmt.Sprintf("%d", snap.RouterFallbacks))

	formatter.Header("MAG (Memory-Augmented Generation)")
	formatter.KeyValue("Memories Retrieved", fmt.Sprintf("%d", snap.MAGMemoriesRetrieved))
	formatter.KeyValue("Memories Stored", fmt.Sprintf("%d", snap.MAGMemoriesStored))

	if len(snap.StageBreakdown) > 0 {
		formatter.Header("Stage Breakdown")
		headers := []string{"Stage", "Calls", "Successes", "Errors", "Error Rate", "Avg ms", "Min ms", "Max ms"}
		rows := make([][]string, 0, len(snap.StageBreakdown))
		for stage, stats := range snap.StageBreakdown {
			rows = append(rows, []string{
				stage,
				fmt.Sprintf("%d", stats.Calls),
				fmt.Sprintf("%d", stats.Successes),
				fmt.Sprintf("%d", stats.Errors),
				fmt.Sprintf("%.1f%%", stats.ErrorRate*100),
				fmt.Sprintf("%.2f", stats.AvgDuration),
				fmt.Sprintf("%.2f", stats.MinDuration),
				fmt.Sprintf("%.2f", stats.MaxDuration),
			})
		}
		formatter.Table(headers, rows)
	}
}
