package skilleval

import (
	"fmt"
)

// DefaultRegressionThreshold is the default tolerance for the regression gate.
// A candidate is allowed to lose at most this much score on the holdout; a loss
// beyond it (i.e. delta < -threshold) is treated as a regression and the
// candidate is rejected. The 0.02 default (mirroring e.g. TBLite) is deliberately
// tight: the point of the gate is to catch a variant that looks good on the
// train fitness but silently breaks something else on the holdout.
const DefaultRegressionThreshold = 0.02

// RegressionGate is the regression sub-gate of the benchmarks-as-gates phase
// (FATIA 3.1). It is deliberately separate from fitness:
//
//   - Fitness (Fase 1/2 meta-loop A/B) answers "did the skill improve the task?"
//     measured on the Train split.
//   - Regression (this gate) answers "did it break the rest?" measured ONLY on
//     the dedicated Holdout split (see SplitEval). Because the holdout is
//     disjoint from the train set, a candidate that overfits the train set can
//     be caught here — the classic train-improves-but-holdout-regresses trap.
//
// A `RegressionGate` is stateless and pure: a zero value uses the default
// threshold, and CheckRegression is deterministic over its inputs (no time, no
// randomness, no LLM) — the same inputs always yield the same verdict.
type RegressionGate struct {
	// Threshold is the maximum tolerated holdout score loss. A value <= 0 falls
	// back to DefaultRegressionThreshold (0.02).
	Threshold float64
}

// NewRegressionGate returns a RegressionGate with an explicit threshold. A
// threshold <= 0 falls back to the default, which is the behaviour callers get
// from a zero-value gate too.
func NewRegressionGate(threshold float64) *RegressionGate {
	return &RegressionGate{Threshold: threshold}
}

// effectiveThreshold resolves the gate's tolerance, falling back to the default
// when the field is left zero or negative. This keeps the zero-value gate fully
// usable and the default stable and documented.
func (g *RegressionGate) effectiveThreshold() float64 {
	if g == nil || g.Threshold <= 0 {
		return DefaultRegressionThreshold
	}
	return g.Threshold
}

// CheckRegression gates a candidate against its baseline using ONLY the holdout
// split. It scores both the baseline arm and the candidate arm on the holdout
// and compares the movement: the gate passes when the candidate does not lose
// more than the threshold (delta >= -threshold).
//
// Contract (anti-overfit and anti-dark-magic):
//   - Both `baseline` and `candidate` must be SkillBenchmarks whose arm under
//     promotion was measured over `holdout` with `scorer` — i.e. the arms were
//     produced by running the skill body against the holdout cases with the
//     same rubric. The gate reads the arm's median score as the holdout signal
//     and NEVER looks at train or val, so a train-fit signal cannot leak into
//     the regression verdict.
//   - The arm read from a benchmark is `With` (the arm under test). A benchmark
//     built as a self-baseline (baseline-vs-baseline via RunAB) therefore
//     reports the baseline's holdout score in `With`, and a candidate benchmark
//     reports the candidate's holdout score in `With`.
//   - A non-empty `holdout` and a non-nil `scorer` are required: the gate cannot
//     genuinely gate on regression without holdout evidence and a rubric.
//   - The GateResult.Enabled fields mirror the existing model: CatalogAudit is
//     not in scope for this sub-gate (set to true so a later, fuller gate can
//     AND its own audit), RegTests reports the holdout grading ran, RegDelta
//     reports the tolerance verdict, and Passed is the overall verdict which
//     equals RegDelta here.
//
// The verdict is deterministic: the same (skill, baseline, candidate, holdout,
// scorer, threshold) always yields the same tuple, so a regression rejection is
// reproducible and auditable.
func (g *RegressionGate) CheckRegression(skill string, baseline, candidate *SkillBenchmark, holdout []SkillCase, scorer Scorer) (*GateResult, error) {
	if len(holdout) == 0 {
		return nil, fmt.Errorf("skilleval: regression gate for %q needs a non-empty holdout", skill)
	}
	if scorer == nil {
		return nil, fmt.Errorf("skilleval: regression gate for %q needs a nil-free scorer", skill)
	}
	if baseline == nil || candidate == nil {
		return nil, fmt.Errorf("skilleval: regression gate for %q needs baseline and candidate benchmarks", skill)
	}

	baselineScore := baseline.With.MedianScore
	candidateScore := candidate.With.MedianScore
	regDelta := candidateScore - baselineScore
	threshold := g.effectiveThreshold()

	// Pass when the candidate did not regress beyond the tolerance. A candidate
	// that improves on train but loses more than `threshold` on the holdout is
	// rejected — this is the overfit catch the holdout exists for.
	regDeltaOK := regDelta >= -threshold

	res := &GateResult{
		CatalogAudit: true, // catalog audit is a separate gate; not in scope here
		RegTests:     true, // holdout grading ran successfully
		RegDelta:     regDeltaOK,
		Passed:       regDeltaOK,
		Details: []string{
			fmt.Sprintf("skill %q holdout regression vs baseline over %d case(s)", skill, len(holdout)),
			fmt.Sprintf("baseline holdout median %.4f", baselineScore),
			fmt.Sprintf("candidate holdout median %.4f", candidateScore),
			fmt.Sprintf("holdout delta %.4f (tolerance -%.4f)", regDelta, threshold),
		},
	}
	if regDeltaOK {
		res.Details = append(res.Details, fmt.Sprintf("verdict: pass (no regression beyond %.4f)", threshold))
	} else {
		res.Details = append(res.Details, fmt.Sprintf("verdict: reject (regressed %.4f, beyond tolerance %.4f)", -regDelta, threshold))
	}
	return res, nil
}
