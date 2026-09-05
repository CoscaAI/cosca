package evals

import (
	"context"
	"strings"
	"testing"
)

func TestBuildPromptEmptyHistory(t *testing.T) {
	s := &LLMSolver{Problem: "Find the hidden number."}
	p := s.buildPrompt(nil)

	if !strings.Contains(p, "Find the hidden number.") {
		t.Fatalf("prompt missing problem:\n%s", p)
	}
	if strings.Contains(p, "previous attempts") {
		t.Fatalf("empty history must not render a history section:\n%s", p)
	}
	if strings.Contains(p, "attempt 1") {
		t.Fatalf("empty history must not render attempts:\n%s", p)
	}
}

func TestBuildPromptWithHistory(t *testing.T) {
	s := &LLMSolver{Problem: "Find the hidden number."}
	history := []Attempt{
		{Candidate: "7", Valid: false},
		{Candidate: "13", Valid: false},
	}
	p := s.buildPrompt(history)

	if !strings.Contains(p, "previous attempts") {
		t.Fatalf("history must render a history section:\n%s", p)
	}
	if !strings.Contains(p, `attempt 1: "7" -> INVALID`) {
		t.Fatalf("history must render attempt 1:\n%s", p)
	}
	if !strings.Contains(p, `attempt 2: "13" -> INVALID`) {
		t.Fatalf("history must render attempt 2:\n%s", p)
	}
}

func TestBuildPromptNeverLeaksSecret(t *testing.T) {
	// The secret lives only in the oracle. The LLMSolver prompt must never
	// contain it — not in the problem, not as a hint.
	secret := "THE_SECRET_SOLUTION"
	s := &LLMSolver{Problem: "Find a solution satisfying the properties."}
	history := []Attempt{
		{Candidate: "wrong-1", Valid: false},
		{Candidate: "wrong-2", Valid: false},
	}
	p := s.buildPrompt(history)
	if strings.Contains(p, secret) {
		t.Fatalf("prompt must never contain the secret:\n%s", p)
	}
}

func TestLLMSolverNext(t *testing.T) {
	s := &LLMSolver{
		Problem: "Find the hidden number.",
		Propose: func(_ context.Context, prompt string) (string, error) {
			return "  42  \n", nil
		},
	}

	c, _, err := s.Next(context.Background(), nil)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if c != "42" {
		t.Fatalf("expected trimmed candidate 42, got %q", c)
	}
}

func TestLLMSolverNextWithParse(t *testing.T) {
	s := &LLMSolver{
		Problem: "Find the hidden number.",
		Propose: func(_ context.Context, prompt string) (string, error) {
			return "I think the answer is...\n42", nil
		},
		Parse: ParseLastLine,
	}

	c, _, err := s.Next(context.Background(), nil)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if c != "42" {
		t.Fatalf("expected parsed candidate 42, got %q", c)
	}
}

// TestLLMSolverManagedUsesHistory verifies the solver forwards the history it
// is given into the prompt (the managed condition), so the model can learn from
// past failures. This is the mechanism the ablation measures.
func TestLLMSolverManagedUsesHistory(t *testing.T) {
	var captured string
	s := &LLMSolver{
		Problem: "Find the hidden number.",
		Propose: func(_ context.Context, prompt string) (string, error) {
			captured = prompt
			return "42", nil
		},
	}

	history := []Attempt{{Candidate: "7", Valid: false}}
	if _, _, err := s.Next(context.Background(), history); err != nil {
		t.Fatalf("Next: %v", err)
	}
	if !strings.Contains(captured, `attempt 1: "7" -> INVALID`) {
		t.Fatalf("managed condition must forward history to the prompt:\n%s", captured)
	}
}

func TestParseStructured(t *testing.T) {
	resp := "HYPOTHESIS: I expect t1 to pass because n is a multiple of 7.\nCANDIDATE: 105"
	hyp, cand := ParseStructured(resp)
	if hyp != "I expect t1 to pass because n is a multiple of 7." {
		t.Fatalf("hypothesis mismatch: %q", hyp)
	}
	if cand != "105" {
		t.Fatalf("candidate mismatch: %q", cand)
	}
}

func TestParseStructuredMissingField(t *testing.T) {
	// A model may emit only the candidate; hypothesis must be empty, not an error.
	_, cand := ParseStructured("CANDIDATE: 42")
	if cand != "42" {
		t.Fatalf("candidate mismatch: %q", cand)
	}
	hyp, _ := ParseStructured("HYPOTHESIS: just a thought")
	if hyp != "just a thought" {
		t.Fatalf("hypothesis mismatch: %q", hyp)
	}
}

func TestLLMSolverCapturesHypothesis(t *testing.T) {
	s := &LLMSolver{
		Problem: "Find n.",
		Propose: func(_ context.Context, prompt string) (string, error) {
			return "HYPOTHESIS: trying a prime.\nCANDIDATE: 101", nil
		},
		ParseBoth: ParseStructured,
	}

	cand, hyp, err := s.Next(context.Background(), nil)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if cand != "101" || hyp != "trying a prime." {
		t.Fatalf("expected cand=101 hyp=%q, got cand=%q hyp=%q", "trying a prime.", cand, hyp)
	}
}

// TestRunTrialCapturesHypothesis proves the hypothesis flows from the solver
// into the Attempt log — the raw material for hypothesis-driven analysis.
func TestRunTrialCapturesHypothesis(t *testing.T) {
	oracle := VerifierFunc(func(_ context.Context, c string) (bool, string, error) {
		return c == "101", "", nil
	})
	solver := SolverFunc(func(_ context.Context, h []Attempt) (string, string, error) {
		return "101", "I predict 101 is prime and ≡3 mod 7", nil
	})

	tr := RunTrial(context.Background(), oracle, solver, 5)
	if !tr.Solved {
		t.Fatalf("expected solved")
	}
	if len(tr.Attempts) != 1 || tr.Attempts[0].Hypothesis != "I predict 101 is prime and ≡3 mod 7" {
		t.Fatalf("hypothesis not captured in attempt log: %+v", tr.Attempts)
	}
}
