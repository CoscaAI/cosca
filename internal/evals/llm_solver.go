package evals

import (
	"context"
	"fmt"
	"strings"
)

// Proposer asks a language model (or any agent) for a response to a prompt. It
// is the decoupling point between the solver and the model: tests substitute a
// deterministic fake, production wires the real pipeline runner.
type Proposer func(ctx context.Context, prompt string) (string, error)

// LLMSolver is the real subject of the ablation: it proposes candidate
// solutions by prompting an LLM. It implements Solver faithfully — the only
// thing that changes between the two ablation conditions is the history it is
// given, which buildPrompt reflects verbatim into the prompt.
//
// The prompt NEVER contains the oracle's secret, the reference solution, or a
// hint about how to solve. It contains only: the public problem statement, the
// solver's own past candidates and their VALID/INVALID results, and the request
// for the next candidate.
type LLMSolver struct {
	// Problem is the public problem statement (what the agent is told).
	Problem string
	// Propose sends a prompt to the model and returns its raw response.
	Propose Proposer
	// Parse extracts the candidate string from the raw response. Defaults to
	// trimming whitespace (the whole response is the candidate).
	Parse func(response string) string
	// ParseBoth extracts (hypothesis, candidate) from a structured response
	// (HYPOTHESIS:/CANDIDATE: lines). When set, it takes precedence over Parse
	// and captures the solver's stated reasoning alongside the candidate.
	ParseBoth func(response string) (hypothesis string, candidate string)
}

// Next builds the prompt from history and returns the candidate and, when
// ParseBoth is set, the stated hypothesis behind it.
func (s *LLMSolver) Next(ctx context.Context, history []Attempt) (string, string, error) {
	resp, err := s.Propose(ctx, s.buildPrompt(history))
	if err != nil {
		return "", "", err
	}
	if s.ParseBoth != nil {
		hypothesis, candidate := s.ParseBoth(resp)
		return candidate, hypothesis, nil
	}
	if s.Parse != nil {
		return s.Parse(resp), "", nil
	}
	return strings.TrimSpace(resp), "", nil
}

// buildPrompt renders the prompt for a given history. An empty history yields a
// bare "propose a candidate" prompt (the amnesiac/alone condition); a non-empty
// history yields the same prompt plus the record of past attempts (the managed
// condition). Nothing else differs.
func (s *LLMSolver) buildPrompt(history []Attempt) string {
	var b strings.Builder
	b.WriteString("You are solving a discovery problem by proposing candidate solutions to a black-box verifier.\n")
	b.WriteString("The verifier answers only VALID or INVALID — it will never tell you how to fix a candidate.\n\n")

	b.WriteString("PROBLEM:\n")
	b.WriteString(s.Problem)
	b.WriteString("\n\n")

	if len(history) > 0 {
		b.WriteString("Your previous attempts and their results:\n")
		for i, a := range history {
			result := "VALID"
			if !a.Valid {
				result = "INVALID"
			}
			if a.Signal != "" {
				result = a.Signal // richer gradient (e.g. t1=PASS t2=FAIL)
			}
			b.WriteString(fmt.Sprintf("  attempt %d: %q -> %s\n", i+1, a.Candidate, result))
		}
		b.WriteString("\n")
	}

	b.WriteString("Propose your NEXT candidate. Respond with exactly two lines:\n")
	b.WriteString("HYPOTHESIS: <your reasoning — what you expect and why>\n")
	b.WriteString("CANDIDATE: <the candidate value>\n")
	return b.String()
}

// ParseLastLine is a Parse helper for simple candidates: it returns the last
// non-empty line of the response, trimmed. Useful when the model may add prose
// around a one-line candidate.
func ParseLastLine(response string) string {
	lines := strings.Split(strings.TrimSpace(response), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if s := strings.TrimSpace(lines[i]); s != "" {
			return s
		}
	}
	return ""
}

// ParseStructured extracts (hypothesis, candidate) from a two-line structured
// response of the form "HYPOTHESIS: ..." and "CANDIDATE: ...". It is tolerant of
// extra prose and missing lines: a missing field yields an empty string. The
// hypothesis is the model's STATED reasoning (its claimed justification), not
// ground-truth internal state — a limitation to record, not to hide.
func ParseStructured(response string) (hypothesis string, candidate string) {
	for _, line := range strings.Split(response, "\n") {
		line = strings.TrimSpace(line)
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "HYPOTHESIS:"):
			hypothesis = strings.TrimSpace(line[len("HYPOTHESIS:"):])
		case strings.HasPrefix(upper, "CANDIDATE:"):
			candidate = strings.TrimSpace(line[len("CANDIDATE:"):])
		}
	}
	return hypothesis, candidate
}
