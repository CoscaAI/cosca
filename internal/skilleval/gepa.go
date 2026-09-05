package skilleval

import (
	"context"
	"fmt"
)

// GEpaStep is a single, auditable row of the GEPA evolution trail. It records
// what happened in one iteration: the fitness of the best variant at that
// point, whether the regression gate allowed the variant, and the feedback the
// evaluation produced (which guides the next mutation).
type GEpaStep struct {
	Iter       int
	Fitness    float64
	GatePassed bool     // true when the variant passed the regression gate
	Feedback   []string // what evaluation flagged; guides the next mutation
}

// GEPAResult is the outcome of a feedback-guided evolutionary loop (RunGEPA).
// It carries the best variant found, its fitness and its dispersion, the number
// of iterations executed, an append-only per-iteration trail for auditability,
// and whether the loop actually landed on a variant that improved without
// regression (a gated candidate).
type GEPAResult struct {
	Best       *Genome    // best variant found
	Score      float64    // fitness of Best
	ScoreStd   float64    // bootstrap std of Best's score
	Iterations int        // iterations actually executed
	History    []GEpaStep // per-iteration trail (audit)
	Candidate  bool       // improved without regression (gate passed)
}

// RunGEPA executes the feedback-guided evolutionary search over a skill body.
//
// It starts from `base` (the current genome) whose identity is LOCKED: a
// variant always keeps base.Identity and only Body may change via the mutator.
// Each iteration:
//
//  1. The mutator rewrites the best body so far, guided by the feedback the
//     previous iteration's evaluation produced.
//  2. `evaluate(ctx, body)` runs the harness on the variant and returns an A/B
//     SkillBenchmark against the immutable baseline genome.
//  3. The variant is adopted only when it is a candidate (improved without
//     regression AND the regression gate passed) AND its fitness beats the
//     current best. Otherwise the best is kept, so the loop never regresses.
//
// Errors are non-fatal: a failed mutation or evaluation is recorded in the
// history as a step that left the best unchanged, and the loop continues. A nil
// mutator or evaluate degrades to such a failing step rather than panicking.
// The loop is capped by maxIter, checks ctx per iteration (a cancelled context
// returns the best-so-far with the context error), and always returns the best
// variant plus its metrics.
func RunGEPA(ctx context.Context, base *Genome, mutator Mutator,
	baselineScore float64, evaluate func(ctx context.Context, body string) (*SkillBenchmark, error),
	maxIter int) (*GEPAResult, error) {

	if base == nil {
		return nil, fmt.Errorf("skilleval: gepa requires a non-nil base genome")
	}
	if maxIter < 0 {
		maxIter = 0
	}
	if mutator == nil {
		mutator = func(context.Context, string, []string) (string, error) {
			return "", fmt.Errorf("skilleval: gepa mutator is nil")
		}
	}
	if evaluate == nil {
		evaluate = func(context.Context, string) (*SkillBenchmark, error) {
			return nil, fmt.Errorf("skilleval: gepa evaluate is nil")
		}
	}

	// Best starts as a deep copy of the base genome so the caller's base is
	// never mutated in place and the identity is guaranteed locked. Only Body
	// is ever replaced on adoption; Identity is copied once and never touched.
	best := &Genome{
		Identity: base.Identity,
		Body:     base.Body,
		Source:   base.Source,
	}
	bestScore := baselineScore
	bestStd := 0.0
	adoptedAny := false

	result := &GEPAResult{
		Best:    best,
		Score:   baselineScore,
		History: make([]GEpaStep, 0, maxIter),
	}

	// feedback carries the previous iteration's evaluation verdict and guides
	// the next mutation. It starts empty (no prior signal).
	var feedback []string

	for iter := 0; iter < maxIter; iter++ {
		if err := ctx.Err(); err != nil {
			result.Best = best
			result.Score = bestScore
			result.ScoreStd = bestStd
			result.Iterations = iter
			return result, err
		}

		// (1) Mutate the best body, guided by the previous feedback. A failed
		// mutation is recorded and never crashes the loop.
		variantBody, merr := mutator(ctx, best.Body, feedback)
		if merr != nil {
			note := "mutation failed: " + merr.Error()
			result.History = append(result.History, GEpaStep{
				Iter:       iter,
				Fitness:    bestScore,
				GatePassed: false,
				Feedback:   []string{note},
			})
			feedback = []string{note}
			continue
		}

		// (2) Evaluate the variant against the immutable baseline.
		bench, eerr := evaluate(ctx, variantBody)
		if eerr != nil {
			note := "evaluation failed: " + eerr.Error()
			result.History = append(result.History, GEpaStep{
				Iter:       iter,
				Fitness:    bestScore,
				GatePassed: false,
				Feedback:   []string{note},
			})
			feedback = []string{note}
			continue
		}
		if bench == nil {
			note := "evaluation returned a nil benchmark"
			result.History = append(result.History, GEpaStep{
				Iter:       iter,
				Fitness:    bestScore,
				GatePassed: false,
				Feedback:   []string{note},
			})
			feedback = []string{note}
			continue
		}

		// (3) Select: adopt only when the variant is a candidate (improved
		// without regression AND the regression gate passed) and its fitness
		// beats the current best. Otherwise keep the best; the loop never
		// regresses below the baseline.
		gatePassed := bench.Gate == nil || bench.Gate.Passed
		variantScore := bench.With.MedianScore
		variantStd := bench.With.MeanBootstrap

		adopted := bench.IsCandidate && gatePassed && variantScore > bestScore
		if adopted {
			best = withBody(base, variantBody)
			bestScore = variantScore
			bestStd = variantStd
			adoptedAny = true
		}

		feedback = deriveFeedback(bench)

		result.History = append(result.History, GEpaStep{
			Iter:       iter,
			Fitness:    bestScore,
			GatePassed: gatePassed,
			Feedback:   feedback,
		})
	}

	result.Best = best
	result.Score = bestScore
	result.ScoreStd = bestStd
	result.Iterations = len(result.History)
	result.Candidate = adoptedAny && bestScore > baselineScore
	return result, nil
}

// withBody mints a new variant genome: identity is locked to base.Identity and
// only the body is replaced. Source is set to the applied full SKILL.md
// (identity + body) so the variant carries provenance for semantic diffing.
// It is the only way a GEPA loop creates a variant, guaranteeing the identity
// can never leak into (or be duplicated by) the body.
func withBody(base *Genome, body string) *Genome {
	g := &Genome{Identity: base.Identity, Body: body}
	g.Source = g.Apply()
	return g
}

// deriveFeedback converts a benchmark into the feedback that guides the next
// mutation. It is deterministic over the benchmark — the same evaluation always
// yields the same guide — so a feedback-driven mutator can be reasoned about
// and tested without an LLM. The feedback reflects what the evaluation flagged:
// the score movement, whether the evidence separated, and whether the gate
// allowed the variant.
func deriveFeedback(bench *SkillBenchmark) []string {
	if bench == nil {
		return []string{"evaluation produced no benchmark"}
	}
	fb := make([]string, 0, 4)
	fb = append(fb, fmt.Sprintf("score %.4f vs baseline %.4f", bench.With.MedianScore, bench.Without.MedianScore))
	if bench.Delta.Candidate {
		fb = append(fb, "robust improvement: evidence separated")
	} else {
		fb = append(fb, "not a robust improvement: evidence overlapping")
	}
	switch {
	case bench.Gate == nil:
		fb = append(fb, "gate: not configured (allowed)")
	case bench.Gate.Passed:
		fb = append(fb, "gate: passed")
	default:
		fb = append(fb, "gate: failed")
	}
	return fb
}
