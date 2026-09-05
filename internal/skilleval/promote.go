package skilleval

import (
	"context"
	"fmt"
	"strings"
)

// DefaultPromotionSeed is the fixed, cross-platform seed used by the integrated
// promotion gate. It is a constant (never random, never time-based) so the SAME
// skill always splits the SAME way across runs and machines. SplitEval already
// mixes the skill name into the shuffle stream, so a constant seed still
// decorrelates the partition between different skills while keeping a given
// skill's split reproducible.
const DefaultPromotionSeed int64 = 1

// PromotionResult is the integrated, anti-overfit verdict of the
// benchmarks-as-gates phase (FATIA 3.2). It bundles the deterministic split and
// the holdout regression verdict so a promotion decision can be made from a
// single auditable object:
//
//   - Split: the 50/25/25 partition (Train/Val/Holdout) the gate used. The
//     holdout is the ONLY set the regression verdict trusts.
//   - BaselineHoldout / CandidateHoldout: the two self-baseline benchmarks built
//     by grading the baseline and candidate bodies against the holdout cases.
//   - Gate: the RegressionGate verdict (RegDelta, Passed, Details). The holdout
//     never trains — it is a pure gate, so a candidate that improves on train
//     but regresses on holdout is rejected here.
//   - RegDelta: the numeric holdout score movement (candidate − baseline). It is
//     surfaced so the CLI can say precisely how far a blocked candidate fell on
//     the holdout.
type PromotionResult struct {
	Split            *EvalSplit
	BaselineHoldout  *SkillBenchmark
	CandidateHoldout *SkillBenchmark
	Gate             *GateResult
	RegDelta         float64
}

// PromotionGate is the integrated split + holdout regression gate. It is
// stateless and pure: Evaluate is deterministic over its inputs (no time, no
// randomness, no LLM) — the same (skill, dataset, bodies, scorer, trials) always
// yields the same verdict.
//
// It is deliberately separate from fitness:
//
//   - Fitness (the GEPA A/B over the full case set) answers "did the skill
//     improve the task?".
//   - This gate answers "did it break the rest?" by scoring baseline and
//     candidate ONLY on the disjoint holdout produced by SplitEval.
//
// The holdout NEVER trains: it is only ever used to grade the gate. Keeping it
// disjoint from the train set is the anti-overfit guarantee — a variant that
// overfits the train set is caught here (the classic train-improves-but-holdout-
// regresses trap).
type PromotionGate struct {
	// Threshold is the maximum tolerated holdout score loss. <= 0 falls back to
	// DefaultRegressionThreshold (0.02).
	Threshold float64
	// SplitRatio is the Train fraction used by SplitEval. <= 0 or >= 1 falls
	// back to DefaultSplitRatio (0.5), i.e. the 50/25/25 split.
	SplitRatio float64
	// Seed is the deterministic split seed. It is shared across runs so the
	// partition is reproducible; DefaultPromotionSeed is the value the CLI uses.
	Seed int64
}

// NewPromotionGate returns a PromotionGate with an explicit threshold, split
// ratio and seed. A zero or out-of-range threshold/ratio falls back to its
// default (0.02 tolerance, 0.5 train fraction).
func NewPromotionGate(threshold, splitRatio float64, seed int64) *PromotionGate {
	g := &PromotionGate{Threshold: threshold, SplitRatio: splitRatio, Seed: seed}
	if g.Threshold <= 0 {
		g.Threshold = DefaultRegressionThreshold
	}
	if g.SplitRatio <= 0 || g.SplitRatio >= 1 {
		g.SplitRatio = DefaultSplitRatio
	}
	return g
}

// effectiveThreshold resolves the gate's regression tolerance, falling back to
// the default (0.02) when the field is zero or negative.
func (g *PromotionGate) effectiveThreshold() float64 {
	if g == nil || g.Threshold <= 0 {
		return DefaultRegressionThreshold
	}
	return g.Threshold
}

// effectiveRatio resolves the SplitEval train fraction, falling back to the
// default (0.5) when the field is zero or out of range.
func (g *PromotionGate) effectiveRatio() float64 {
	if g == nil || g.SplitRatio <= 0 || g.SplitRatio >= 1 {
		return DefaultSplitRatio
	}
	return g.SplitRatio
}

// Evaluate splits `cases` (SplitEval, 50/25/25 by default), grades the baseline
// and candidate bodies ONLY over the disjoint holdout, and returns the combined
// verdict. The holdout is never used for fitness — it is a pure gate.
//
// Both bodies are scored as self-baselines (With == Without) over the holdout
// using the supplied scorer, so the gate reads each body's honest holdout
// signal. The scorer must be non-nil (a gate without a rubric cannot genuinely
// judge regression), and the dataset must be large enough to yield a holdout
// (SplitEval enforces MinSplitCases).
//
// The verdict is deterministic over its inputs, so a holdout rejection is
// reproducible and auditable.
func (g *PromotionGate) Evaluate(ctx context.Context, skill string, cases []SkillCase,
	baselineBody, candidateBody string, scorer Scorer, trials int) (*PromotionResult, error) {

	if strings.TrimSpace(skill) == "" {
		return nil, fmt.Errorf("skilleval: promotion gate needs a skill name")
	}
	if scorer == nil {
		return nil, fmt.Errorf("skilleval: promotion gate for %q needs a nil-free scorer", skill)
	}

	split, err := SplitEval(skill, cases, g.effectiveRatio(), g.Seed)
	if err != nil {
		return nil, err
	}
	if len(split.Holdout) == 0 {
		return nil, fmt.Errorf("skilleval: promotion gate for %q needs a non-empty holdout", skill)
	}

	baselineHoldout, err := RunAB(ctx, scorer, bodyRunner(baselineBody), bodyRunner(baselineBody), split.Holdout, trials, nil)
	if err != nil {
		return nil, fmt.Errorf("skilleval: grade baseline on holdout for %q: %w", skill, err)
	}
	candidateHoldout, err := RunAB(ctx, scorer, bodyRunner(candidateBody), bodyRunner(candidateBody), split.Holdout, trials, nil)
	if err != nil {
		return nil, fmt.Errorf("skilleval: grade candidate on holdout for %q: %w", skill, err)
	}

	gateResult, err := NewRegressionGate(g.effectiveThreshold()).
		CheckRegression(skill, baselineHoldout, candidateHoldout, split.Holdout, scorer)
	if err != nil {
		return nil, err
	}

	return &PromotionResult{
		Split:            split,
		BaselineHoldout:  baselineHoldout,
		CandidateHoldout: candidateHoldout,
		Gate:             gateResult,
		RegDelta:         candidateHoldout.With.MedianScore - baselineHoldout.With.MedianScore,
	}, nil
}

// bodyRunner is a deterministic Runner that emits a fixed body as the produced
// output. It lets the promotion gate genuinely grade a body with a scorer
// without invoking any agent or LLM — exactly like the CLI's staticRunner
// fixture.
func bodyRunner(body string) Runner {
	return func(context.Context) (string, error) { return body, nil }
}
