package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/pipeline"
	"github.com/CoscaAI/cosca/internal/workflows"
)

// NewWorkflowCommand creates the `cosca workflow` command and its subcommands.
func NewWorkflowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Manage Cosca workflows",
		Long: `Manage and execute Cosca workflows.

Workflows define structured processes for common tasks like code review,
architecture decisions, testing, deployment, and more.

Subcommands:
  list             List available workflows
  show <name>      Show workflow details
  run <name>       Execute a workflow
  search <query>   Search workflows
`,
		Example: `  cosca workflow list
  cosca workflow show code-review
  cosca workflow run code-review
  cosca workflow search "database"`,
	}

	cmd.AddCommand(
		NewWorkflowListCommand(),
		NewWorkflowShowCommand(),
		NewWorkflowRunCommand(),
		NewWorkflowSearchCommand(),
		NewWorkflowGraphRunCommand(),
	)

	return cmd
}

// NewWorkflowListCommand creates the `cosca workflow list` subcommand.
func NewWorkflowListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available workflows",
		Long:  `List all workflows available in the Cosca system.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := workflows.NewManager(coscaDir)
			if mgr == nil {
				return fmt.Errorf("workflow manager not available")
			}

			workflowList := mgr.List()

			if useJSON {
				return printJSON(cmd, workflowList)
			}

			if len(workflowList) == 0 {
				formatter.Warning("Nenhum workflow encontrado. Workflows built-in estao disponiveis via o Cosca Framework.")
				return nil
			}

			formatter.Header(fmt.Sprintf("Available Workflows (%d)", len(workflowList)))

			headers := []string{"Name", "Description", "Steps", "Status"}
			rows := make([][]string, 0, len(workflowList))

			for _, w := range workflowList {
				status := "active"
				if !w.Enabled {
					status = "disabled"
				}
				rows = append(rows, []string{w.Name, w.Description, fmt.Sprintf("%d", w.Steps), status})
			}

			formatter.Table(headers, rows)
			return nil
		},
	}

	return cmd
}

// NewWorkflowShowCommand creates the `cosca workflow show` subcommand.
func NewWorkflowShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show workflow details",
		Long:  `Display detailed information about a specific workflow.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := workflows.NewManager(coscaDir)
			if mgr == nil {
				return fmt.Errorf("workflow manager not available")
			}

			wf, err := mgr.Get(args[0])
			if err != nil {
				return fmt.Errorf("workflow %q not found: %w", args[0], err)
			}

			if useJSON {
				return printJSON(cmd, wf)
			}

			formatter.Header(fmt.Sprintf("Workflow: %s", wf.Name))
			formatter.KeyValue("Name", wf.Name)
			formatter.KeyValue("Description", wf.Description)
			formatter.KeyValue("Version", wf.Version)
			formatter.KeyValue("Status", wf.Status)
			formatter.KeyValue("Steps", fmt.Sprintf("%d", wf.Steps))

			formatter.Println("")
			formatter.Header("Steps")
			for i, step := range wf.StepList {
				formatter.KeyValue(fmt.Sprintf("Step %d", i+1), step.Name)
				formatter.KeyValue("  Description", step.Description)
				formatter.KeyValue("  Agent", step.Agent)
				formatter.KeyValue("  Timeout", step.Timeout)
				formatter.Println("")
			}

			if len(wf.Inputs) > 0 {
				formatter.Header("Inputs")
				for _, input := range wf.Inputs {
					formatter.Bullet(fmt.Sprintf("%s (%s, required: %v)", input.Name, input.Type, input.Required))
				}
			}

			if len(wf.Outputs) > 0 {
				formatter.Println("")
				formatter.Header("Outputs")
				for _, output := range wf.Outputs {
					formatter.Bullet(fmt.Sprintf("%s (%s)", output.Name, output.Type))
				}
			}

			return nil
		},
	}

	return cmd
}

// NewWorkflowRunCommand creates the `cosca workflow run` subcommand.
func NewWorkflowRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <name>",
		Short: "Execute a workflow",
		Long: `Execute a workflow by name with optional input parameters.
The run is durable: completed steps are recorded in an append-only event log
(.cosca/durable-events/) and replayed on resume. If a previous run of this
workflow was interrupted, use --resume to continue from the first incomplete
step instead of starting over.`,
		Example: `  cosca workflow run code-review
  cosca workflow run deploy --env production
  cosca workflow run code-review --resume`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)
			resumeFlag, _ := cmd.Flags().GetBool("resume")

			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}

			// Real shared wiring: runner → stepRunner → recovery → DoD.
			wiring, err := buildPipelineWiring(dir)
			if err != nil {
				return fmt.Errorf("pipeline wiring: %w", err)
			}
			defer wiring.Close()

			wf, err := wiring.manager.Get(args[0])
			if err != nil {
				return fmt.Errorf("workflow %q not found: %w", args[0], err)
			}
			if wiring.manager.IsStub(wf) {
				return fmt.Errorf("workflow %q is a stub (0 steps) — not executable", wf.Name)
			}

			formatter.Header(fmt.Sprintf("Executing workflow %s", wf.Name))

			// Convert the workflow's steps into a real pipeline plan.
			plan := workflowToPlan(wf)
			if plan.ID == "" {
				plan.ID = "WF-" + wf.Name
			}
			wiring.stepRunner.SetPlanID(plan.ID)

			// ── Durable resume: replay completed steps from the event log ──
			if _, resumeErr := maybeResumeDurable(wiring, formatter, plan, resumeFlag); resumeErr != nil {
				return fmt.Errorf("workflow resume failed: %w", resumeErr)
			}

			result, runErr := wiring.stepRunner.RunPlanWithProgress(cmd.Context(), plan,
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
				return fmt.Errorf("workflow execution failed: %w", runErr)
			}

			// Definition of Done enforcement: exit non-zero with a report
			// when the workflow did not pass all DoD checks.
			dodReport := wiring.DoD().Validate(cmd.Context(), plan)

			if useJSON {
				return printJSON(cmd, struct {
					Name   string             `json:"name"`
					Done   int                `json:"done"`
					Total  int                `json:"total"`
					Failed int                `json:"failed"`
					DoD    pipeline.DoDReport `json:"dod"`
				}{
					Name: wf.Name,
					Done: result.TasksDone, Total: result.TasksTotal, Failed: result.TasksFailed,
					DoD: *dodReport,
				})
			}

			if result.TasksFailed > 0 {
				fmt.Fprintln(os.Stderr)
				fmt.Fprint(os.Stderr, dodReport.FormatReport())
				return fmt.Errorf("workflow %s failed: %d/%d steps failed", wf.Name, result.TasksFailed, result.TasksTotal)
			}

			if !dodReport.AllPassed {
				fmt.Fprintln(os.Stderr)
				fmt.Fprint(os.Stderr, dodReport.FormatReport())
				return fmt.Errorf("workflow %s did not pass all DoD checks", wf.Name)
			}

			formatter.Success(fmt.Sprintf("Workflow %s completed", wf.Name))
			formatter.KeyValue("Steps Completed", fmt.Sprintf("%d/%d", result.TasksDone, result.TasksTotal))
			fmt.Fprint(os.Stdout, dodReport.FormatReport())

			return nil
		},
	}

	cmd.Flags().Bool("resume", false, "resume a stale durable run (replay completed steps, continue from first incomplete)")

	return cmd
}

// NewWorkflowSearchCommand creates the `cosca workflow search` subcommand.
func NewWorkflowSearchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search workflows",
		Long:  `Search for workflows by name, description, or tags.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := workflows.NewManager(coscaDir)
			if mgr == nil {
				return fmt.Errorf("workflow manager not available")
			}

			results, err := mgr.Search(args[0])
			if err != nil {
				return fmt.Errorf("workflow search failed: %w", err)
			}

			if useJSON {
				return printJSON(cmd, results)
			}

			if len(results) == 0 {
				formatter.Warning(fmt.Sprintf("No workflows found matching %q", args[0]))
				return nil
			}

			formatter.Header(fmt.Sprintf("Workflow Search Results for %q", args[0]))
			for _, r := range results {
				formatter.KeyValue(r.Name, r.Description)
			}
			formatter.KeyValue("Total", fmt.Sprintf("%d", len(results)))

			return nil
		},
	}

	return cmd
}
