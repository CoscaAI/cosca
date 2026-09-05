package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/pipeline"
)

// NewPipelineTestCommand creates the `cosca pipeline test` command.
func NewPipelineTestCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "pipeline-test",
		Short: "Testa o pipeline completo com mock runner (sem LLM)",
		Long: `Executa um plano de teste completo passando por todas as fases:
  Plan → StepRunner → Recovery → Reconciliation → Checkpoint → Event History.

Usa MockRunner — nenhuma chamada LLM real. Ideal para verificar
a infraestrutura de execução durável.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			// Setup history & checkpoint
			historyDir := filepath.Join(coscaDir, "history")
			history, err := pipeline.NewWorkflowHistory(historyDir)
			if err != nil {
				return fmt.Errorf("history: %w", err)
			}

			cpDir := filepath.Join(historyDir, "checkpoints")
			checkpoint, err := pipeline.NewCheckpointStore(cpDir)
			if err != nil {
				return fmt.Errorf("checkpoint: %w", err)
			}

			// Create mock runner
			runner := pipeline.NewMockRunner()

			// Create step runner with full durable execution
			recovery := pipeline.NewRecoveryLoop(nil) // no classifier needed for mock
			stepRunner := pipeline.NewDurableStepRunner(runner, history, checkpoint, recovery)

			// Plugin registry
			plugins := pipeline.NewPluginRegistry()
			pipeline.RegisterBuiltinPlugins(plugins, checkpoint)
			stepRunner.SetPlugins(plugins)

			// Create test plan
			plan := &pipeline.Plan{
				ID:         fmt.Sprintf("TEST-%s", time.Now().Format("150405")),
				Intent:     "Pipeline smoke test",
				IntentType: "test",
				Tasks: []*pipeline.TaskNode{
					{ID: "build-backend", Description: "Build Go backend", Agent: "cosca-backend", Status: pipeline.TaskPending},
					{ID: "run-tests", Description: "Run unit tests", Agent: "cosca-testing", Status: pipeline.TaskPending, DependsOn: []string{"build-backend"}},
					{ID: "lint-check", Description: "Run lint checks", Agent: "cosca-review", Status: pipeline.TaskPending, DependsOn: []string{"run-tests"}},
					{ID: "security-scan", Description: "Security audit", Agent: "cosca-security", Status: pipeline.TaskPending, DependsOn: []string{"lint-check"}},
					{ID: "docs-update", Description: "Update documentation", Agent: "cosca-documentation", Status: pipeline.TaskPending, DependsOn: []string{"run-tests"}},
				},
				EstimatedMinutes: 1,
				RiskLevel:        "low",
			}
			stepRunner.SetPlanID(plan.ID)

			formatter.Header("╔════════════════ PIPELINE TEST ════════════════")

			ctx := context.Background()
			start := time.Now()

			result, err := stepRunner.RunPlan(ctx, plan)
			if err != nil {
				return fmt.Errorf("pipeline failed: %w", err)
			}

			dur := time.Since(start)

			// Show results
			fmt.Fprintf(cmd.OutOrStdout(), "\n  📋 Plan: %s\n", plan.ID)
			fmt.Fprintf(cmd.OutOrStdout(), "  ⏱  Duration: %v\n", dur.Round(time.Millisecond))
			fmt.Fprintf(cmd.OutOrStdout(), "  ✅ Tasks done: %d/%d\n", result.TasksDone, result.TasksTotal)
			fmt.Fprintf(cmd.OutOrStdout(), "  ❌ Tasks failed: %d\n", result.TasksFailed)
			fmt.Fprintf(cmd.OutOrStdout(), "  🔄 Recovered from history: %d\n", result.Recovered)
			fmt.Fprintf(cmd.OutOrStdout(), "  📜 Events recorded: %d\n", len(result.Events))

			// Show task statuses
			fmt.Fprintf(cmd.OutOrStdout(), "\n  Tasks:\n")
			for _, t := range plan.Tasks {
				icon := "✅"
				if t.Status == pipeline.TaskFailed {
					icon = "❌"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "    %s  %-20s  %s\n", icon, t.ID, t.Status)
			}

			// Verify integrity
			verified, hashErr := history.VerifyIntegrity(plan.ID)
			if hashErr != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "\n  🔴 Hash chain: BROKEN — %v\n", hashErr)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "\n  🟢 Hash chain: VERIFIED (%d events)\n", verified)
			}

			// Load analytics
			store := pipeline.NewAnalyticsStore(history)
			analytics, _ := store.Load(plan.ID)
			if analytics != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  📊 Autonomy: %.0f%%\n", analytics.AutonomyScore*100)
			}

			// Test crash recovery
			fmt.Fprintf(cmd.OutOrStdout(), "\n  ─────── CRASH RECOVERY TEST ───────\n")
			hasHistory := stepRunner.HasHistory()
			fmt.Fprintf(cmd.OutOrStdout(), "  History exists: %v\n", hasHistory)

			if hasHistory {
				// Simulate crash by creating a new step runner and running again
				runner2 := pipeline.NewMockRunner()
				stepRunner2 := pipeline.NewDurableStepRunner(runner2, history, checkpoint, recovery)
				stepRunner2.SetPlugins(plugins)
				stepRunner2.SetPlanID(plan.ID)

				// All tasks should already be completed → recovered = all tasks
				result2, _ := stepRunner2.RunPlan(ctx, plan)
				fmt.Fprintf(cmd.OutOrStdout(), "  Recovered tasks: %d/%d\n", result2.Recovered, result2.TasksTotal)
				if result2.Recovered == result2.TasksTotal {
					fmt.Fprintf(cmd.OutOrStdout(), "  🟢 Crash recovery: TODAS tasks recuperadas\n")
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\n╚════════════════════════════════════════════\n")

			// Now check analytics
			fmt.Fprintf(cmd.OutOrStdout(), "\n  📊 Aggregate Analytics:\n")
			agg := &pipeline.AggregateAnalytics{}
			store2 := pipeline.NewAnalyticsStore(history)
			a2, _ := store2.Load(plan.ID)
			if a2 != nil {
				agg.Add(a2)
				fmt.Fprintf(cmd.OutOrStdout(), "    Plans: %d | Tasks: %d/%d | Recovery: %d | Autonomy: %.0f%%\n",
					agg.TotalPlans, agg.TasksDone, agg.TotalTasks, agg.RecoveriesSucceeded, agg.AvgAutonomyScore*100)
			}

			// Show qgate
			fmt.Fprintf(cmd.OutOrStdout(), "\n")
			gate := pipeline.NewGate(dir)
			gate.SetHistory(history)
			gateResult, _ := gate.Run(ctx)
			fmt.Fprint(cmd.OutOrStdout(), gateResult.Display())

			return nil
		},
	}
}
