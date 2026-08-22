package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/evals"
	"github.com/CoscaAI/cosca/internal/pipeline"
)

// ─── Eval Command Group ────────────────────────────────────────────────────

// NewEvalCommand creates the `cosca eval` command group. It runs benchmark
// suites against the REAL autonomous pipeline (buildPipelineWiring): planning
// produces tasks, steps execute through the step runner, DoD evaluates.
func NewEvalCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "eval",
		Short: "Run benchmark suites against the Cosca pipeline",
		Long: `Run benchmark suites against the real Cosca pipeline.

A suite is a YAML file (or one of the bundled suites under
internal/embed/cosca/evals/suites/) declaring benchmark cases. Every case is
executed through the REAL pipeline wiring — planning produces tasks, the step
runner executes them, the recovery loop handles failures, and the Definition
of Done evaluates the result. Scripted verification commands run after each
case and reports are persisted to .cosca/evals/reports/.

Subcommands:
  list [suite]    List benchmark cases in a suite
  run <suite>     Run a suite against the real pipeline
  smoke           Run the bundled smoke suite
  report <suite>  Show the last persisted run report

Examples:
  cosca eval list
  cosca eval list esteira
  cosca eval run smoke --case plan-feature-development --timeout 30
  cosca eval run esteira --json
  cosca eval smoke`,
	}

	cmd.AddCommand(
		newEvalListCommand(),
		newEvalRunCommand(),
		newEvalSmokeCommand(),
		newEvalReportCommand(),
		newEvalOracleCommand(),
		newEvalAblationCommand(),
	)

	return cmd
}

// ─── eval list ─────────────────────────────────────────────────────────────

func newEvalListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list [suite]",
		Short: "List benchmark cases in a suite",
		Long: `List the benchmark cases declared in a suite.

The suite argument is a path to a YAML suite file or the name of a bundled
suite (smoke, esteira). Defaults to the bundled smoke suite.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := "smoke"
			if len(args) > 0 {
				name = args[0]
			}

			suite, err := evals.LoadSuite(name)
			if err != nil {
				return err
			}

			formatter := GetFormatter(cmd)
			if IsJSONOutput(cmd) {
				return printJSON(cmd, suite)
			}

			formatter.Header(fmt.Sprintf("Suite: %s (%d cases)", suite.Suite, len(suite.Cases)))
			if m := suite.Metadata; m.Author != "" || m.Description != "" || m.Difficulty != "" || m.Category != "" {
				formatter.Header("Suite Metadata")
				if m.Author != "" {
					formatter.KeyValue("Author", m.Author)
				}
				if m.Description != "" {
					formatter.KeyValue("Description", m.Description)
				}
				if m.Difficulty != "" {
					formatter.KeyValue("Difficulty", m.Difficulty)
				}
				if m.Category != "" {
					formatter.KeyValue("Category", m.Category)
				}
				if len(m.Tags) > 0 {
					formatter.KeyValue("Tags", strings.Join(m.Tags, ", "))
				}
				if m.Created != "" {
					formatter.KeyValue("Created", m.Created)
				}
			}
			headers := []string{"ID", "Workflow", "Difficulty", "Category", "Prompt", "Steps", "Verify"}
			rows := make([][]string, 0, len(suite.Cases))
			for _, c := range suite.Cases {
				rows = append(rows, []string{
					c.ID,
					c.Workflow,
					suite.CaseDifficulty(c),
					suite.CaseCategory(c),
					truncateStr(c.Prompt, 50),
					strings.Join(evals.ResolveSteps(c), ","),
					strings.Join(c.Verify, "; "),
				})
			}
			formatter.Table(headers, rows)
			return nil
		},
	}
}

// ─── eval run / eval smoke ─────────────────────────────────────────────────

func newEvalRunCommand() *cobra.Command {
	var (
		onlyCase string
		timeoutS int
	)

	cmd := &cobra.Command{
		Use:   "run <suite>",
		Short: "Run a benchmark suite against the real pipeline",
		Long: `Run a benchmark suite against the real Cosca pipeline.

The suite argument is a path to a YAML suite file or the name of a bundled
suite (smoke, esteira). Each case is executed through buildPipelineWiring and
reports its real outcome. The report is persisted to
.cosca/evals/reports/ and printed to stdout (or emitted as JSON with --json).

Flags:
  --case <id>     run only the case with this id
  --timeout <s>   per-case timeout in seconds (overrides suite defaults)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEvalSuite(cmd, args[0], onlyCase, timeoutS)
		},
	}

	cmd.Flags().StringVar(&onlyCase, "case", "", "run only the case with this id")
	cmd.Flags().IntVar(&timeoutS, "timeout", 0, "per-case timeout in seconds (overrides suite defaults)")

	return cmd
}

func newEvalSmokeCommand() *cobra.Command {
	var (
		onlyCase string
		timeoutS int
	)

	cmd := &cobra.Command{
		Use:   "smoke",
		Short: "Run the bundled smoke suite",
		Long: `Run the bundled cosca-esteira-smoke suite against the real pipeline.

The smoke suite checks the pipeline MECHANISM — planning produces tasks,
steps execute through the real step runner, DoD evaluates — not code quality.
When the chat provider is down the execution case fails gracefully with a
clear error instead of hanging.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runEvalSuite(cmd, "smoke", onlyCase, timeoutS)
		},
	}

	cmd.Flags().StringVar(&onlyCase, "case", "", "run only the case with this id")
	cmd.Flags().IntVar(&timeoutS, "timeout", 0, "per-case timeout in seconds (overrides suite defaults)")

	return cmd
}

// runEvalSuite loads a suite and executes it through the real pipeline
// wiring, persisting a report and printing per-case results.
func runEvalSuite(cmd *cobra.Command, suiteRef string, onlyCase string, timeoutS int) error {
	suite, err := evals.LoadSuite(suiteRef)
	if err != nil {
		return err
	}

	if onlyCase != "" && suite.Case(onlyCase) == nil {
		return fmt.Errorf("case %q not found in suite %q (available: %s)",
			onlyCase, suite.Suite, strings.Join(suite.ListCases(), ", "))
	}

	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	// Real shared wiring: orchestrator → runner → planner → stepRunner →
	// recoveryLoop → DoD. Never a fake.
	wiring, err := buildPipelineWiring(dir)
	if err != nil {
		return fmt.Errorf("pipeline wiring: %w", err)
	}
	defer wiring.Close()
	runner := &pipelineCaseRunner{wiring: wiring, workDir: dir}

	var timeout time.Duration
	if timeoutS > 0 {
		timeout = time.Duration(timeoutS) * time.Second
	}

	opts := evals.RunOptions{
		ProjectDir:  dir,
		Timeout:     timeout,
		OnlyCaseID:  onlyCase,
		StripCanary: true,
		Progress: func(format string, args ...any) {
			fmt.Fprintf(os.Stderr, "  "+format+"\n", args...)
		},
	}

	formatter := GetFormatter(cmd)
	if !IsJSONOutput(cmd) {
		formatter.Header(fmt.Sprintf("Eval Suite: %s", suite.Suite))
	}

	report, err := evals.RunSuite(cmd.Context(), suite, runner, opts)
	if err != nil {
		return fmt.Errorf("eval run: %w", err)
	}

	// Persist the report (0600 perms, best effort).
	reportsDir := filepath.Join(dir, ".cosca", "evals", "reports")
	saved, saveErr := report.Save(reportsDir)
	if saveErr != nil {
		fmt.Fprintf(os.Stderr, "  warning: could not persist report: %v\n", saveErr)
	}

	if IsJSONOutput(cmd) {
		return printJSON(cmd, report)
	}

	printEvalReport(formatter, report)
	if saved != "" {
		formatter.KeyValue("Report", saved)
	}

	// Case failures are RESULTS, not command errors: the report above (and
	// the persisted JSON) carries them. Exiting non-zero here would make the
	// jail parent re-run the whole command through its no-root fallback.
	if report.Failed > 0 || report.Errors > 0 || report.TimedOut > 0 {
		fmt.Fprintf(os.Stderr, "  [eval] %s\n", report.SummaryLine())
		fmt.Fprintf(os.Stderr, "  [eval] run completed with failures — see report above (or use --json / the persisted report).\n")
	}
	return nil
}

// ─── eval report ───────────────────────────────────────────────────────────

func newEvalReportCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "report <suite>",
		Short: "Show the last persisted run report for a suite",
		Long: `Show the most recent persisted report for a suite.

The suite argument is the suite name (e.g. cosca-esteira-smoke), a suite file
reference, or a bare bundled name. Reports are read from
.cosca/evals/reports/.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolve working directory: %w", err)
			}
			reportsDir := filepath.Join(dir, ".cosca", "evals", "reports")

			name := args[0]
			if s, err := evals.LoadSuite(name); err == nil {
				name = s.Suite
			}

			report, err := evals.LatestReport(reportsDir, name)
			if err != nil {
				return err
			}

			formatter := GetFormatter(cmd)
			if IsJSONOutput(cmd) {
				return printJSON(cmd, report)
			}
			printEvalReport(formatter, report)
			return nil
		},
	}
}

// printEvalReport renders a report as a table plus per-case failure detail.
func printEvalReport(formatter *OutputFormatter, report *evals.Report) {
	if m := report.Metadata; m.Author != "" || m.Description != "" || m.Difficulty != "" || m.Category != "" {
		formatter.Header("Suite Metadata")
		if m.Author != "" {
			formatter.KeyValue("Author", m.Author)
		}
		if m.Description != "" {
			formatter.KeyValue("Description", m.Description)
		}
		if m.Difficulty != "" {
			formatter.KeyValue("Difficulty", m.Difficulty)
		}
		if m.Category != "" {
			formatter.KeyValue("Category", m.Category)
		}
		if len(m.Tags) > 0 {
			formatter.KeyValue("Tags", strings.Join(m.Tags, ", "))
		}
		if m.Created != "" {
			formatter.KeyValue("Created", m.Created)
		}
	}

	headers := []string{"Case", "Status", "Duration", "Reward", "Result"}
	rows := make([][]string, 0, len(report.Cases))
	for _, c := range report.Cases {
		rows = append(rows, []string{
			c.ID,
			string(c.Status),
			c.Duration,
			fmt.Sprintf("[reward %.2f/%.2f]", c.Reward, c.RewardExpectation),
			c.Summary,
		})
	}
	formatter.Table(headers, rows)

	for _, c := range report.Cases {
		if c.Status == evals.StatusPassed {
			continue
		}
		formatter.Println("")
		formatter.Header(fmt.Sprintf("Case %s (%s)", c.ID, c.Status))
		if c.Error != "" {
			formatter.Error(c.Error)
		}
		if c.Summary != "" {
			formatter.KeyValue("Summary", c.Summary)
		}
		for _, vr := range c.VerifyResults {
			if vr.OK {
				formatter.Success("verify: " + vr.Command)
			} else {
				formatter.Error("verify: " + vr.Command)
				if vr.Error != "" {
					formatter.KeyValue("  error", vr.Error)
				}
				if vr.OutputTail != "" {
					formatter.KeyValue("  output", truncateStr(vr.OutputTail, 200))
				}
			}
		}
		if c.DoD != "" {
			fmt.Fprint(os.Stdout, c.DoD)
		}
	}

	formatter.Println("")
	formatter.Header("Summary")
	formatter.KeyValue("Total", fmt.Sprintf("%d", report.Total))
	formatter.KeyValue("Passed", fmt.Sprintf("%d", report.Passed))
	formatter.KeyValue("Failed", fmt.Sprintf("%d", report.Failed))
	formatter.KeyValue("Errors", fmt.Sprintf("%d", report.Errors))
	formatter.KeyValue("Timed Out", fmt.Sprintf("%d", report.TimedOut))
	formatter.KeyValue("Pass Rate", fmt.Sprintf("%.2f", report.PassRate))
	formatter.KeyValue("Reward", fmt.Sprintf("mean %.2f | max %.2f | min %.2f | sum %.2f",
		report.RewardMean, report.RewardMax, report.RewardMin, report.RewardSum))
	formatter.KeyValue("Metrics", fmt.Sprintf("accuracy %.2f | precision %.2f | recall %.2f | F1 %.2f | MCC %.2f | ±%.2f (bootstrap, n=%d)",
		report.Metrics.Accuracy, report.Metrics.Precision, report.Metrics.Recall, report.Metrics.F1, report.Metrics.MatthewsCorr, report.Metrics.BootstrapStd, evals.DefaultBootstrapSamples))
}

// ─── Real pipeline runner ──────────────────────────────────────────────────

// pipelineCaseRunner executes an eval case through the REAL pipeline wiring.
// It satisfies evals.ExecRunner so the harness stays decoupled and testable.
type pipelineCaseRunner struct {
	wiring  *pipelineWiring
	workDir string
}

var _ evals.ExecRunner = (*pipelineCaseRunner)(nil)

// Execute plans the case (workflow-pinned or ad-hoc), applies the scripted
// steps, runs the plan through the durable step runner and evaluates DoD.
func (r *pipelineCaseRunner) Execute(ctx context.Context, c evals.Case, qp *evals.QuestionPolicy) (*evals.CaseOutcome, error) {
	// Question policy: the real pipeline currently has no ask_user hook, so
	// no question can be surfaced during execution. The policy is still
	// resolved and passed through so any runner that does ask (now or in
	// future) injects the scripted answers / applies the strategy; per spec,
	// "fail" only trips when a question is actually asked without answers.
	_ = qp

	var plan *pipeline.Plan
	if c.Workflow != "" {
		wf, err := r.wiring.manager.Get(c.Workflow)
		if err != nil {
			return nil, fmt.Errorf("case %s: workflow %q not found: %w", c.ID, c.Workflow, err)
		}
		if r.wiring.manager.IsStub(wf) {
			return nil, fmt.Errorf("case %s: workflow %q is a stub (0 steps) — not executable", c.ID, wf.Name)
		}
		plan = workflowToPlan(wf)
	} else {
		plan = r.wiring.planner.Plan(c.Prompt)
	}
	if plan.ID == "" {
		plan.ID = "EVAL-" + evalPlanSuffix(c.ID)
	}

	steps := evals.ResolveSteps(c)
	executed := []string{}
	var runResult *pipeline.RunPlanResult
	var dodReport *pipeline.DoDReport

	for _, step := range steps {
		switch step {
		case "wait_for_plan":
			if len(plan.Tasks) == 0 {
				return nil, fmt.Errorf("case %s: planning produced no tasks", c.ID)
			}
		case "approve_plan":
			// Gate 0 approval marker — granted so execution can proceed.
		case "wait_for_execution":
			r.wiring.stepRunner.SetPlanID(plan.ID)
			var err error
			runResult, err = r.wiring.stepRunner.RunPlanWithProgress(ctx, plan, r.progress)
			if err != nil {
				return nil, fmt.Errorf("case %s: execution failed: %w", c.ID, err)
			}
			dodReport = r.wiring.DoD().Validate(ctx, plan)
		default:
			return nil, fmt.Errorf("case %s: unknown step %q (want wait_for_plan, approve_plan, wait_for_execution)", c.ID, step)
		}
		executed = append(executed, step)
	}

	outcome := &evals.CaseOutcome{
		Status:    "passed",
		Steps:     executed,
		PlanTasks: len(plan.Tasks),
	}

	if runResult == nil {
		// Plan-only case: the pipeline mechanism stops after planning.
		outcome.Summary = fmt.Sprintf("plan-only: %d tasks planned", len(plan.Tasks))
		return outcome, nil
	}

	outcome.TasksDone = runResult.TasksDone
	outcome.TasksFailed = runResult.TasksFailed
	outcome.Summary = fmt.Sprintf("%d/%d tasks completed", runResult.TasksDone, runResult.TasksTotal)
	if runResult.TasksFailed > 0 {
		outcome.Status = "failed"
		outcome.Summary = fmt.Sprintf("%d/%d tasks failed", runResult.TasksFailed, runResult.TasksTotal)
		if failText := failedTaskReason(plan); failText != "" {
			outcome.Failure = failText
		}
	}

	if dodReport != nil {
		outcome.DoD = dodReport.FormatReport()
		if !dodReport.AllPassed {
			outcome.Status = "failed"
			outcome.Summary = "Definition of Done not fully passed"
			if outcome.Failure == "" {
				if failText := failedTaskReason(plan); failText != "" {
					outcome.Failure = failText
				}
			}
		}
	}

	return outcome, nil
}

// failedTaskReason returns the error text of the first failed task in a plan,
// or "" when there are no failed tasks.
func failedTaskReason(plan *pipeline.Plan) string {
	for _, t := range plan.Tasks {
		if t.Status == pipeline.TaskFailed && t.Result != nil && t.Result.Error != "" {
			return t.Result.Error
		}
	}
	return ""
}

// progress renders a real-time step indicator to stderr (never stdout, so
// JSON output stays clean).
func (r *pipelineCaseRunner) progress(stepName string, status pipeline.TaskStatus, done, total int) {
	symbol := "."
	switch status {
	case pipeline.TaskCompleted:
		symbol = "+"
	case pipeline.TaskFailed:
		symbol = "x"
	case pipeline.TaskRunning:
		symbol = ">"
	}
	fmt.Fprintf(os.Stderr, "      [%d/%d] %s %s (%s)\n", done, total, symbol, stepName, status)
}

// evalPlanSuffix derives a safe, unique plan suffix from a case ID.
func evalPlanSuffix(id string) string {
	clean := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, id)
	if clean == "" {
		clean = "case"
	}
	return clean + "-" + time.Now().Format("150405")
}
