package skilleval

import (
	"context"
	"fmt"
	"math"
)

// Runner executes the agent (or skill body) for one benchmark trial and returns
// the produced output.
type Runner func(ctx context.Context) (output string, err error)

const (
	// MinTrials is the default number of trials per condition. It is also the
	// minimum evidence level below which a Delta is never marked a candidate,
	// so a small sample cannot produce a confident promotion decision.
	MinTrials = 5
)

// RunCondition runs `runner` for `trials` rounds and aggregates the per-case
// graded results into a robust Condition. See RunAB for the pair-wise harness.
//
// Robustness rules:
//   - A runner that returns an error degrades to a "fail" (score 0) for every
//     case in that trial, recorded in GradingResult.Feedback, and the loop
//     continues. A transient agent failure never aborts the benchmark.
//   - A scorer that returns an error degrades to the neutral score (0.5),
//     recorded in Feedback, and the loop continues. A malformed judge verdict
//     never aborts the benchmark.
//   - The aggregate follows Condition: median + IQR for score, median for
//     tokens and time, and evals.BootstrapStd for the mean's dispersion. The
//     mean itself is never reported.
//
// It returns a context error only when ctx is cancelled before a trial starts.
func RunCondition(ctx context.Context, scorer Scorer, runner Runner, cases []SkillCase, trials int) (Condition, error) {
	if trials <= 0 {
		trials = MinTrials
	}
	if runner == nil {
		runner = func(context.Context) (string, error) { return "", nil }
	}

	results := make([]GradingResult, 0, trials*len(cases))
	for t := 0; t < trials; t++ {
		if err := ctx.Err(); err != nil {
			return Condition{}, err
		}

		output, rerr := runner(ctx)
		if rerr != nil {
			// A failing runner degrades to a fail (score 0) per case without
			// aborting the loop.
			for _, c := range cases {
				results = append(results, failResult(c.Rubric, "runner failed: "+rerr.Error()))
			}
			continue
		}

		for _, c := range cases {
			var gr GradingResult
			if scorer == nil {
				gr = neutralResult(c.Rubric, "scorer is nil; treating as neutral")
			} else {
				r, gerr := scorer.Grade(ctx, c.Rubric, output)
				if gerr != nil {
					gr = neutralResult(c.Rubric, "scorer failed: "+gerr.Error())
				} else {
					gr = r
				}
			}
			results = append(results, gr)
		}
	}
	return BuildCondition(results), nil
}

// RunAB runs the two arms of a skill A/B experiment — the candidate body
// (`with`) and the baseline/old body (`without`) — and classifies whether the
// candidate is a promotion-worthy result.
//
// The classification (see abCandidate) requires:
//   - sufficient evidence (trials >= MinTrials),
//   - median(with) >= median(without), and
//   - distinguishable IQRs (non-overlapping, or overlap <= 25% of the smaller
//     IQR width). A non-overlapping or barely-overlapping pair is a robust
//     separation; a large common IQR region is inconclusive.
//
// A regression gate may further veto the result: when `gate` is non-nil and
// returns a *GateResult with Passed == false, IsCandidate becomes false
// regardless of the A/B signal. A nil `gate` function means "no gate": it does
// not block, and is recorded as benchmark.Gate == nil.
func RunAB(ctx context.Context, scorer Scorer, with, without Runner, cases []SkillCase, trials int, gate func(context.Context) *GateResult) (*SkillBenchmark, error) {
	if trials <= 0 {
		trials = MinTrials
	}

	withCond, err := RunCondition(ctx, scorer, with, cases, trials)
	if err != nil {
		return nil, fmt.Errorf("run with arm: %w", err)
	}
	withoutCond, err := RunCondition(ctx, scorer, without, cases, trials)
	if err != nil {
		return nil, fmt.Errorf("run without arm: %w", err)
	}

	delta := abDelta(withCond, withoutCond, trials)
	bench := &SkillBenchmark{
		With:    withCond,
		Without: withoutCond,
		Delta:   delta,
	}
	bench.IsCandidate = delta.Candidate
	bench.Gate = nil

	if gate != nil {
		if g := gate(ctx); g != nil {
			bench.Gate = g
			bench.IsCandidate = bench.IsCandidate && g.Passed
		}
	}
	return bench, nil
}

// abDelta derives the movement between the two arms and marks whether the A/B
// evidence alone (before the gate) calls the candidate arm a winner.
func abDelta(with, without Condition, trials int) Delta {
	return Delta{
		Score:     with.MedianScore - without.MedianScore,
		ScoreStd:  combinedStd(with.MeanBootstrap, without.MeanBootstrap),
		Candidate: abCandidate(with, without, trials),
	}
}

// abCandidate applies the falsifiable A/B criterion. Without enough trials the
// evidence is too thin to support a promotion, so it always reports false.
func abCandidate(with, without Condition, trials int) bool {
	if trials < MinTrials {
		return false
	}
	if !(with.MedianScore >= without.MedianScore) {
		return false
	}
	return iqrsNonOverlapping(with.IQRS, without.IQRS) || iqrOverlapRatio(with.IQRS, without.IQRS) <= 0.25
}

// combinedStd combines the bootstrap std of two independent arm means, i.e.
// sqrt(sigma_with^2 + sigma_without^2) — the dispersion of the movement.
func combinedStd(a, b float64) float64 {
	return math.Sqrt(a*a + b*b)
}

// iqrsNonOverlapping reports whether the two IQR intervals are fully separated
// (one sits entirely above the other, with no hint of overlap).
func iqrsNonOverlapping(a, b [2]float64) bool {
	return a[1] < b[0] || b[1] < a[0]
}

// iqrOverlapRatio reports the fraction of the smaller IQR's width that is
// shared with the other IQR, in [0,1]. A value of 0 means the intervals are
// fully separated; a value of 1 means one interval is entirely inside the
// other. Degenerate (point) IQRs are treated as fully overlapping when they
// intersect.
func iqrOverlapRatio(a, b [2]float64) float64 {
	lo := math.Max(a[0], b[0])
	hi := math.Min(a[1], b[1])
	if hi <= lo {
		return 0
	}
	widthA := a[1] - a[0]
	widthB := b[1] - b[0]
	denom := widthA
	if widthB < denom {
		denom = widthB
	}
	if denom <= 0 {
		return 1
	}
	return (hi - lo) / denom
}

// failResult builds a GradingResult for a runner that failed: score 0 for the
// whole case, with the failure noted in Feedback.
func failResult(rubric []string, note string) GradingResult {
	feedback := make([]string, len(rubric))
	for i := range feedback {
		feedback[i] = note
	}
	return GradingResult{
		Total:    len(rubric),
		Score:    0,
		Feedback: feedback,
	}
}

// neutralResult builds a GradingResult for a scorer that could not judge: it
// degrades to the neutral score instead of aborting, with the reason noted in
// Feedback.
func neutralResult(rubric []string, note string) GradingResult {
	feedback := make([]string, len(rubric))
	for i := range feedback {
		feedback[i] = note
	}
	return GradingResult{
		Total:    len(rubric),
		Score:    DefaultNeutralScore,
		Feedback: feedback,
	}
}
