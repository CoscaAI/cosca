package evals

import (
	"context"
	"sort"
	"time"
)

// ─── Ablation harness (deterministic instrument) ────────────────────────────
//
// The harness separates the experiment from the subject: it runs a Solver
// against a black-box Oracle and measures the discovery process, without any
// opinion about the solver's internals. It is fully deterministic given a
// deterministic solver+oracle, so the harness itself can be verified by tests
// before an LLM solver is ever plugged in.
//
// The single experimental variable is whether the solver receives the history
// of past attempts (managed) or an empty history (alone). See ABLATION_PROTOCOL.md.

// Attempt is one candidate the solver submitted to the oracle.
type Attempt struct {
	Candidate string
	Valid     bool
	// Signal is the anonymous per-property feedback (e.g. "t1=PASS t2=FAIL"),
	// empty when the oracle returns only VALID/INVALID. It gives the solver a
	// gradient to climb without revealing the secret.
	Signal string
	// Hypothesis is the solver's stated reasoning/prediction for this attempt
	// (the "why" behind the intervention). Empty when the solver did not
	// provide one. It is the raw material for distinguishing hypothesis-driven
	// experimentation from trial-and-error.
	Hypothesis string
	// Elapsed is the time since the trial started.
	Elapsed time.Duration
}

// Verifier is a black-box oracle: it reports whether a candidate is valid and,
// optionally, an anonymous signal (empty = VALID/INVALID only). It never
// reveals the secret, the reference solution, or how to fix an invalid
// candidate.
type Verifier interface {
	Verify(ctx context.Context, candidate string) (valid bool, signal string, err error)
}

// Solver proposes candidates. history is the memory of past attempts; a managed
// solver uses it, an amnesiac solver ignores it. It returns the candidate and
// (optionally) the stated hypothesis behind it. Returning ("", "", nil) signals
// the solver has nothing further to try.
type Solver interface {
	Next(ctx context.Context, history []Attempt) (candidate string, hypothesis string, err error)
}

// SolverFunc adapts a function to Solver.
type SolverFunc func(ctx context.Context, history []Attempt) (candidate string, hypothesis string, err error)

func (f SolverFunc) Next(ctx context.Context, history []Attempt) (string, string, error) {
	return f(ctx, history)
}

// VerifierFunc adapts a function to Verifier.
type VerifierFunc func(ctx context.Context, candidate string) (bool, string, error)

func (f VerifierFunc) Verify(ctx context.Context, candidate string) (bool, string, error) {
	return f(ctx, candidate)
}

// TrialResult is the outcome of one solver run against the oracle.
type TrialResult struct {
	// Solved reports whether the solver found a valid candidate within budget.
	Solved bool
	// AttemptCount is the total number of candidates submitted.
	AttemptCount int
	// Discarded is the number of invalid candidates.
	Discarded int
	// TimeToSolve is the elapsed time to the first valid candidate (zero if
	// never solved).
	TimeToSolve time.Duration
	// Attempts is the raw candidate log (evidence/audit — "guardar tudo").
	Attempts []Attempt
	// Distinct is the number of UNIQUE candidates tried. It is the exploration
	// signal: an amnesiac solver that repeats one candidate has Distinct=1 even
	// with a large AttemptCount — the "H1→H1→H1" failure mode.
	Distinct int
}

// RunTrial runs a solver against an oracle until it solves, gives up, or the
// budget is exhausted. It is the atomic unit of the experiment.
func RunTrial(ctx context.Context, oracle Verifier, solver Solver, budget int) TrialResult {
	start := time.Now()
	tr := TrialResult{}
	var history []Attempt

	for tr.AttemptCount < budget {
		candidate, hypothesis, err := solver.Next(ctx, history)
		if err != nil {
			break
		}
		if candidate == "" {
			break // solver gave up
		}

		valid, signal, err := oracle.Verify(ctx, candidate)
		if err != nil {
			break
		}

		a := Attempt{Candidate: candidate, Valid: valid, Signal: signal, Hypothesis: hypothesis, Elapsed: time.Since(start)}
		history = append(history, a)
		tr.Attempts = append(tr.Attempts, a)
		tr.AttemptCount++
		if !valid {
			tr.Discarded++
		} else {
			tr.Solved = true
			tr.TimeToSolve = a.Elapsed
			break
		}
	}

	seen := map[string]struct{}{}
	for _, a := range tr.Attempts {
		seen[a.Candidate] = struct{}{}
	}
	tr.Distinct = len(seen)
	return tr
}

// ConditionResult aggregates N trials of one condition with robust statistics
// (median + IQR), because LLM output is heavy-tailed and non-deterministic —
// the mean is not trustworthy at small N.
type ConditionResult struct {
	Label string
	// Trials is the raw per-trial outcome.
	Trials []TrialResult
	// SolvedRate is solved trials / total.
	SolvedRate float64
	// MedianAttempts is the median number of attempts over solved trials.
	MedianAttempts int
	// IQRAttempts is the [Q1, Q3] of attempts over solved trials.
	IQRAttempts [2]int
	// MedianTimeToSolve is the median time-to-solution over solved trials.
	MedianTimeToSolve time.Duration
	// MedianDistinct is the median number of unique candidates tried per trial.
	// It captures exploration independent of whether the solver solved.
	MedianDistinct int
}

// RunCondition runs N trials for a solver and aggregates them.
func RunCondition(ctx context.Context, oracle Verifier, solver Solver, label string, trials, budget int) ConditionResult {
	res := ConditionResult{Label: label}
	for i := 0; i < trials; i++ {
		res.Trials = append(res.Trials, RunTrial(ctx, oracle, solver, budget))
	}

	solved := 0
	var attempts []int
	var times []time.Duration
	var distinct []int
	for _, tr := range res.Trials {
		distinct = append(distinct, tr.Distinct)
		if tr.Solved {
			solved++
			attempts = append(attempts, tr.AttemptCount)
			times = append(times, tr.TimeToSolve)
		}
	}
	res.SolvedRate = float64(solved) / float64(trials)
	if len(attempts) > 0 {
		res.MedianAttempts = medianInt(attempts)
		res.IQRAttempts = [2]int{quantileInt(attempts, 0.25), quantileInt(attempts, 0.75)}
		res.MedianTimeToSolve = medianDuration(times)
	}
	res.MedianDistinct = medianInt(distinct)
	return res
}

// Compare holds the two conditions of an ablation run.
type Compare struct {
	Alone   ConditionResult `json:"alone"`
	Managed ConditionResult `json:"managed"`
}

// RunAblation runs the two conditions (amnesiac vs managed) against the same
// oracle and returns the comparison. The managed condition is simply the solver
// receiving the full history; the alone condition is the SAME solver receiving
// an empty history — the only variable that changes is memory.
func RunAblation(ctx context.Context, oracle Verifier, solver Solver, trials, budget int) Compare {
	alone := SolverFunc(func(ctx context.Context, _ []Attempt) (string, string, error) {
		return solver.Next(ctx, nil) // amnesia: no history
	})
	return Compare{
		Alone:   RunCondition(ctx, oracle, alone, "alone", trials, budget),
		Managed: RunCondition(ctx, oracle, solver, "managed", trials, budget),
	}
}

// ManagedWins reports whether the managed condition is strictly better: it
// solved (and alone did not), or both solved but managed used fewer median
// attempts. Returns false when managed did not solve at all.
func (c Compare) ManagedWins() bool {
	if c.Managed.SolvedRate == 0 {
		return false
	}
	if c.Alone.SolvedRate == 0 {
		return true
	}
	return c.Managed.MedianAttempts < c.Alone.MedianAttempts
}

// medianInt returns the median of a sorted-in-place copy of xs.
func medianInt(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	s := append([]int(nil), xs...)
	sort.Ints(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// quantileInt returns the q-th quantile (0 <= q <= 1) of xs.
func quantileInt(xs []int, q float64) int {
	if len(xs) == 0 {
		return 0
	}
	s := append([]int(nil), xs...)
	sort.Ints(s)
	idx := int(q * float64(len(s)-1))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(s) {
		idx = len(s) - 1
	}
	return s[idx]
}

// medianDuration returns the median duration of xs.
func medianDuration(xs []time.Duration) time.Duration {
	if len(xs) == 0 {
		return 0
	}
	s := append([]time.Duration(nil), xs...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}
