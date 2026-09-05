package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/evals"
)

// newEvalOracleCommand creates the `cosca eval oracle` command group — the
// black-box verifier an agent queries during a discovery task. It exposes only
// the minimal signal (VALID/INVALID or anonymous PASS/FAIL); the secret
// verify commands and the reference solution are never revealed.
func newEvalOracleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "oracle",
		Short: "Interact with the secret oracle (black-box verifier)",
		Long: `Submit candidate solutions to the secret oracle of a benchmark case.

The oracle lives OUTSIDE the public suite, in .cosca/evals/secrets/<id>.yaml
(0600, never committed). It runs the secret verify commands against the
current workspace and returns only the minimal signal allowed by the case's
feedback level — never the commands, the reference solution, or how to fix a
failing candidate.

Each submission is appended to .cosca/evals/submissions/<id>.jsonl and feeds
the DiscoveryMetrics of the run report.`,
	}
	cmd.AddCommand(newEvalOracleSubmitCommand())
	return cmd
}

// newEvalOracleSubmitCommand creates `cosca eval oracle submit <case-id>`.
func newEvalOracleSubmitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "submit <case-id>",
		Short: "Submit the current workspace as a candidate and receive VALID/INVALID",
		Long: `Submit the current workspace as a candidate solution to the case's secret
oracle. The oracle runs the secret verify commands (which the caller never
sees) and prints the minimal signal: VALID/INVALID, or t1=PASS t2=FAIL for
pass_fail/property/constraint feedback.

Example:
  cosca eval oracle submit index-optimization`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			caseID := args[0]
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolve working directory: %w", err)
			}

			secret, err := evals.LoadSecret(dir, caseID)
			if err != nil {
				return err
			}
			if secret == nil {
				return fmt.Errorf("no secret for case %q (expected .cosca/evals/secrets/%s.yaml)", caseID, caseID)
			}

			feedback := secret.Feedback
			if feedback == "" {
				feedback = evals.FeedbackValidInvalid
			}

			verdict, err := evals.SubmitToOracle(cmd.Context(), dir, secret, feedback, 30*time.Second)
			if err != nil {
				return err
			}

			// Record the submission for DiscoveryMetrics.
			log := evals.NewSubmissionLog(dir, caseID)
			idx, err := log.Append(evals.Submission{
				At:     time.Now().UTC(),
				Valid:  verdict.Valid,
				Signal: verdict.Signal,
			})
			if err != nil {
				return err
			}

			formatter := GetFormatter(cmd)
			if IsJSONOutput(cmd) {
				return printJSON(cmd, map[string]any{
					"submission":  idx,
					"valid":       verdict.Valid,
					"signal":      verdict.Signal,
					"per_command": verdict.PerCommand,
				})
			}
			formatter.Println(verdict.Signal)
			return nil
		},
	}
}
