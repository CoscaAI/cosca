package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/pipeline"
	"github.com/CoscaAI/cosca/internal/workflows"
)

// ─── Pipeline Command ────────────────────────────────────────────────────────

// NewPipelineCommand creates the `cosca pipeline` command and its subcommands.
func NewPipelineCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Manage and run orchestration pipelines",
		Long: `Manage and execute Cosca orchestration pipelines.

Pipelines combine multiple agents and skills into structured workflows.
They are sourced from the workflow definitions in your Cosca configuration.

Subcommands:
  list            List available pipelines
  run <name>      Execute a pipeline with given input`,
		Example: `  cosca pipeline list
  cosca pipeline run code-review
  cosca pipeline run code-review --prompt "Review the auth module"`,
	}

	cmd.AddCommand(
		NewPipelineListCommand(),
		NewPipelineRunCommand(),
	)

	return cmd
}

// ─── Pipeline List ───────────────────────────────────────────────────────────

// NewPipelineListCommand creates the `cosca pipeline list` subcommand.
func NewPipelineListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available pipelines",
		Long:  `List all pipelines available from the workflow definitions in your Cosca project.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := workflows.NewManager(coscaDir)
			if mgr == nil {
				return fmt.Errorf("workflow manager not available")
			}

			pipelineList := mgr.List()

			if useJSON {
				return printJSON(cmd, pipelineList)
			}

			if len(pipelineList) == 0 {
				formatter.Warning("No pipelines found. Pipelines are created from workflow definitions in .cosca/workflows/")
				return nil
			}

			formatter.Header(fmt.Sprintf("Available Pipelines (%d)", len(pipelineList)))

			headers := []string{"Name", "Description", "Steps", "Status"}
			rows := make([][]string, 0, len(pipelineList))

			for _, p := range pipelineList {
				status := "active"
				if !p.Enabled {
					status = "disabled"
				}
				rows = append(rows, []string{p.Name, p.Description, fmt.Sprintf("%d", p.Steps), status})
			}

			formatter.Table(headers, rows)
			return nil
		},
	}

	return cmd
}

// ─── Pipeline Run ────────────────────────────────────────────────────────────

// NewPipelineRunCommand creates the `cosca pipeline run` subcommand.
func NewPipelineRunCommand() *cobra.Command {
	var (
		promptOverride string
		streamFlag     bool
		resumeFlag     bool
	)

	cmd := &cobra.Command{
		Use:   "run <name>",
		Short: "Execute a pipeline",
		Long: `Execute a named pipeline using the AI Orchestration Engine.

The pipeline name must correspond to a workflow defined in your Cosca
configuration (.cosca/workflows/). Each step in the pipeline dispatches
to the appropriate agent.

Examples:
  cosca pipeline run code-review
  cosca pipeline run deploy --prompt "Deploy to staging"
  cosca pipeline run test-suite --stream`,
		Args: func(cmd *cobra.Command, args []string) error {
			// A name is required unless --prompt/--stream drives execution.
			promptOverride, _ := cmd.Flags().GetString("prompt")
			streamFlag, _ := cmd.Flags().GetBool("stream")
			if len(args) == 0 && promptOverride == "" && !streamFlag {
				return fmt.Errorf("accepts 1 arg(s) (pipeline name), received 0; or provide --prompt")
			}
			if len(args) > 1 {
				return fmt.Errorf("accepts at most 1 arg(s), received %d", len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			pipelineName := ""
			if len(args) > 0 {
				pipelineName = args[0]
			}
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}

			// Real shared wiring: planner → stepRunner → recovery → DoD.
			wiring, err := buildPipelineWiring(dir)
			if err != nil {
				return fmt.Errorf("pipeline wiring: %w", err)
			}
			defer wiring.Close()

			// Prompt-only mode: no named pipeline definition to show.
			if pipelineName == "" {
				return runPipelinePrompt(cmd, wiring, formatter, promptOverride, resumeFlag)
			}

			wf, err := wiring.manager.Get(pipelineName)
			if err != nil {
				return fmt.Errorf("pipeline %q not found: %w; use 'cosca pipeline list' to see available pipelines", pipelineName, err)
			}

			if useJSON {
				// In JSON mode, show the pipeline definition.
				return printJSON(cmd, wf)
			}

			// 2. Display pipeline info.
			formatter.Header(fmt.Sprintf("Pipeline: %s", wf.Name))
			formatter.KeyValue("Description", wf.Description)
			formatter.KeyValue("Version", wf.Version)
			formatter.KeyValue("Status", wf.Status)
			formatter.KeyValue("Steps", fmt.Sprintf("%d", wf.Steps))

			if len(wf.StepList) > 0 {
				formatter.Println("")
				formatter.Header("Pipeline Steps")
				for i, step := range wf.StepList {
					formatter.KeyValue(fmt.Sprintf("Step %d", i+1), step.Name)
					formatter.KeyValue("  Agent", step.Agent)
					if step.Description != "" {
						formatter.KeyValue("  Description", step.Description)
					}
					if step.Timeout != "" {
						formatter.KeyValue("  Timeout", step.Timeout)
					}
				}
			}

			if len(wf.Inputs) > 0 {
				formatter.Println("")
				formatter.Header("Inputs")
				for _, input := range wf.Inputs {
					required := ""
					if input.Required {
						required = " (required)"
					}
					formatter.Bullet(fmt.Sprintf("%s (%s)%s", input.Name, input.Type, required))
				}
			}

			if len(wf.Outputs) > 0 {
				formatter.Println("")
				formatter.Header("Expected Outputs")
				for _, output := range wf.Outputs {
					formatter.Bullet(fmt.Sprintf("%s (%s)", output.Name, output.Type))
				}
			}

			// 3. Execute for REAL through the shared wiring.
			var plan *pipeline.Plan
			if promptOverride != "" || streamFlag {
				// Prompt/stream path: plan the prompt into a task DAG and run
				// it through the same planner → stepRunner → recovery loop as
				// `cosca terminal --task`.
				formatter.Println("")
				formatter.KeyValue("Executing prompt", promptOverride)
				plan = wiring.planner.Plan(promptOverride)
				if wiring.stepRunner.DurableLog() != nil {
					plan.ID = durablePromptRunID(promptOverride)
				} else if plan.ID == "" {
					plan.ID = "PIPE-" + wf.Name
				}
				return runPlanPipeline(cmd, wiring, formatter, plan, wf.Name, "pipeline", resumeFlag)
			}

			if wiring.manager.IsStub(wf) {
				return fmt.Errorf("pipeline %q is a stub (0 steps) — not executable", wf.Name)
			}
			plan = workflowToPlan(wf)
			if plan.ID == "" {
				plan.ID = "PIPE-" + wf.Name
			}
			return runPlanPipeline(cmd, wiring, formatter, plan, wf.Name, "pipeline", resumeFlag)
		},
	}

	cmd.Flags().StringVar(&promptOverride, "prompt", "", "input prompt to pass to the pipeline")
	cmd.Flags().BoolVar(&streamFlag, "stream", false, "enable streaming output")
	cmd.Flags().BoolVar(&resumeFlag, "resume", false, "resume a stale durable run (replay completed steps, continue from first incomplete)")

	return cmd
}

// runPipelinePrompt executes an ad-hoc prompt through the autonomous pipeline
// (planner → stepRunner → recovery → DoD) without a named pipeline.
func runPipelinePrompt(cmd *cobra.Command, wiring *pipelineWiring, formatter *OutputFormatter, prompt string, resumeFlag bool) error {
	if prompt == "" {
		return fmt.Errorf("pipeline run requires a pipeline name or --prompt")
	}
	formatter.Header("Pipeline (ad-hoc prompt)")
	formatter.KeyValue("Prompt", prompt)

	plan := wiring.planner.Plan(prompt)
	if wiring.stepRunner.DurableLog() != nil {
		// Durable execution: the run ID must be stable so the same prompt can
		// be resumed (completed steps replayed, interrupted work continued).
		plan.ID = durablePromptRunID(prompt)
	} else if plan.ID == "" {
		plan.ID = "PIPE-prompt"
	}
	return runPlanPipeline(cmd, wiring, formatter, plan, "ad-hoc", "pipeline", resumeFlag)
}

// runPlanPipeline executes a plan through the shared stepRunner, enforces DoD
// and reports progress. Returns an error (non-zero exit) on real failure or
// when DoD checks are not all passed.
func runPlanPipeline(cmd *cobra.Command, wiring *pipelineWiring, formatter *OutputFormatter, plan *pipeline.Plan, name, kind string, resumeFlag bool) error {
	wiring.stepRunner.SetPlanID(plan.ID)

	// ── Durable resume: replay completed steps from the event log ──
	if _, resumeErr := maybeResumeDurable(wiring, formatter, plan, resumeFlag); resumeErr != nil {
		return fmt.Errorf("%s resume failed: %w", kind, resumeErr)
	}

	result, runErr := wiring.stepRunner.RunPlanWithQueue(cmd.Context(), plan,
		func(stepName string, status pipeline.TaskStatus, done, total int) {
			symbol := "."
			switch status {
			case pipeline.TaskCompleted:
				symbol = "+"
			case pipeline.TaskFailed:
				symbol = "x"
			case pipeline.TaskRunning:
				symbol = ">"
			}
			fmt.Fprintf(os.Stderr, "    [%d/%d] %s %s (%s)\n", done, total, symbol, stepName, status)
		})
	if runErr != nil {
		return fmt.Errorf("%s execution failed: %w", kind, runErr)
	}

	dodReport := wiring.DoD().Validate(cmd.Context(), plan)

	formatter.Println("")
	if result.TasksFailed > 0 {
		fmt.Fprint(os.Stderr, dodReport.FormatReport())
		return fmt.Errorf("%s %s failed: %d/%d steps failed", kind, name, result.TasksFailed, result.TasksTotal)
	}
	if !dodReport.AllPassed {
		fmt.Fprint(os.Stderr, dodReport.FormatReport())
		return fmt.Errorf("%s %s did not pass all DoD checks", kind, name)
	}

	formatter.Success(fmt.Sprintf("%s %s completed successfully", kind, name))
	formatter.KeyValue("Steps Completed", fmt.Sprintf("%d/%d", result.TasksDone, result.TasksTotal))
	fmt.Fprint(os.Stdout, dodReport.FormatReport())

	return nil
}
