package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/chat"
	chatprovider "github.com/CoscaAI/cosca/internal/chat/provider"
	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/evals"
)

// newEvalAblationCommand creates `cosca eval ablation` — the metacognition
// ablation harness. Default (deterministic demo) runs a hidden-number problem
// with a deterministic solver; --llm runs the SAME harness with the real LLM
// as the subject (proposing candidates through a minimal, MAG-free chat call).
func newEvalAblationCommand() *cobra.Command {
	var (
		useLLM bool
		trials int
		budget int
	)
	cmd := &cobra.Command{
		Use:   "ablation",
		Short: "Run the metacognition ablation (alone vs managed)",
		Long: `Run the metacognition ablation harness: the solver must discover a hidden
candidate through a black-box oracle that answers only VALID/INVALID.

Two conditions, same solver, same oracle:
  alone   — the solver receives an EMPTY history each step (amnesia).
  managed — the solver receives the full history (memory of past attempts).

The harness reports the solved rate and median attempts (with IQR) per
condition, and whether managed won. See internal/evals/ABLATION_PROTOCOL.md.

By default it runs a deterministic demo (secret number, deterministic solver),
which doubles as a smoke test of the harness. Use --llm to run the real LLM
subject through a minimal chat call (no MAG, no routing — the only variable
is memory).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			var oracle evals.Verifier
			var problem string
			var solver evals.Solver
			if useLLM {
				// The real subject: a discovery problem with a reasoning gradient
				// (property-level feedback), so memory can plausibly help.
				oracle = evals.VerifierFunc(invariantOracle)
				problem = "Find an integer n (1 <= n <= 200) that satisfies ALL the hidden properties.\n" +
					"The verifier reports three anonymous tests (t1, t2, t3) as PASS or FAIL.\n" +
					"Discover an n where all three are PASS."
				propose, err := minimalLLMProposer()
				if err != nil {
					return err
				}
				solver = &evals.LLMSolver{
					Problem:   problem,
					Propose:   propose,
					ParseBoth: evals.ParseStructured,
				}
			} else {
				oracle = evals.VerifierFunc(func(_ context.Context, candidate string) (bool, string, error) {
					return candidate == "42", "", nil
				})
				solver = evals.SolverFunc(demoScanSolver)
			}

			cmp := evals.RunAblation(ctx, oracle, solver, trials, budget)

			formatter := GetFormatter(cmd)
			if IsJSONOutput(cmd) {
				return printJSON(cmd, cmp)
			}

			subject := "demo (deterministic solver)"
			if useLLM {
				subject = "real LLM — invariant discovery (prime≡3 mod 7, >100)"
			}
			formatter.Header(fmt.Sprintf("Metacognition Ablation — %s", subject))
			printCondition(formatter, cmp.Alone)
			printCondition(formatter, cmp.Managed)
			formatter.Println("")
			if cmp.ManagedWins() {
				formatter.Success("Managed won — memory improved the discovery process.")
			} else if cmp.Managed.SolvedRate > 0 && cmp.Alone.SolvedRate > 0 && cmp.Managed.MedianAttempts >= cmp.Alone.MedianAttempts {
				formatter.Warning("Managed did NOT improve attempts — memory not contributing.")
			} else {
				formatter.Warning("Inconclusive — inspect the per-condition metrics.")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&useLLM, "llm", false, "use the real LLM as the subject (minimal chat call)")
	cmd.Flags().IntVar(&trials, "trials", 5, "number of trials per condition")
	cmd.Flags().IntVar(&budget, "budget", 200, "max candidate submissions per trial")
	return cmd
}

func printCondition(f *OutputFormatter, c evals.ConditionResult) {
	f.KeyValue(fmt.Sprintf("%s — solved", c.Label), fmt.Sprintf("%.0f%%", c.SolvedRate*100))
	f.KeyValue(fmt.Sprintf("%s — median attempts", c.Label), fmt.Sprintf("%d (IQR %d–%d)", c.MedianAttempts, c.IQRAttempts[0], c.IQRAttempts[1]))
	f.KeyValue(fmt.Sprintf("%s — median distinct candidates", c.Label), fmt.Sprintf("%d", c.MedianDistinct))
}

// demoScanSolver scans 1..100 in order, skipping candidates already tried. With
// an empty history it always returns "1" (it cannot remember), so the amnesiac
// condition loops forever while the managed condition converges in 42 steps.
func demoScanSolver(_ context.Context, history []evals.Attempt) (string, string, error) {
	tried := map[string]bool{}
	for _, a := range history {
		tried[a.Candidate] = true
	}
	for i := 1; i <= 100; i++ {
		c := strconv.Itoa(i)
		if !tried[c] {
			return c, "", nil
		}
	}
	return "", "", nil
}

// minimalLLMProposer returns an evals.Proposer backed by a MINIMAL chat call:
// the selected provider directly, with no orchestration (no MAG, no agent
// routing, no memory). This is essential for the ablation — the only memory the
// subject sees is the history the harness passes, never the Cosca's own memory.
// Temperature 0 reduces nondeterminism (best effort; providers may not honor it
// fully).
func minimalLLMProposer() (evals.Proposer, error) {
	if cfg, err := config.Load(); err == nil {
		loadChatEnv(cfg)
	}

	registry := chat.GetRegistry()
	if err := chatprovider.RegisterChatProviders(registry, nil); err != nil {
		return nil, fmt.Errorf("register chat providers: %w", err)
	}
	selCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := registry.Select(selCtx, chat.DefaultChatRegistryConfig()); err != nil {
		return nil, fmt.Errorf("select chat provider: %w", err)
	}

	return func(ctx context.Context, prompt string) (string, error) {
		resp, err := registry.Chat(ctx, []chat.Message{{Role: chat.RoleUser, Content: prompt}}, chat.ChatOptions{Temperature: 0})
		if err != nil {
			return "", err
		}
		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("no choices in response")
		}
		return resp.Choices[0].Message.Content, nil
	}, nil
}

// invariantOracle is a hidden-invariant verifier: the secret is the conjunction
// of three properties over an integer n — (t1) n ≡ 3 (mod 7), (t2) n is prime,
// (t3) n > 100. It returns the anonymous per-property signal (t1=PASS t2=FAIL
// t3=PASS) so the solver gets a gradient without ever learning the properties.
// Solutions include 101, 199, … — the solver must discover one.
func invariantOracle(_ context.Context, candidate string) (bool, string, error) {
	n, err := strconv.Atoi(strings.TrimSpace(candidate))
	if err != nil {
		return false, "t1=FAIL t2=FAIL t3=FAIL", nil
	}
	t1 := n%7 == 3
	t2 := isPrime(n)
	t3 := n > 100
	state := func(b bool) string {
		if b {
			return "PASS"
		}
		return "FAIL"
	}
	signal := fmt.Sprintf("t1=%s t2=%s t3=%s", state(t1), state(t2), state(t3))
	return t1 && t2 && t3, signal, nil
}

// isPrime reports whether n is a prime number (trial division).
func isPrime(n int) bool {
	if n < 2 {
		return false
	}
	for d := 2; d*d <= n; d++ {
		if n%d == 0 {
			return false
		}
	}
	return true
}
