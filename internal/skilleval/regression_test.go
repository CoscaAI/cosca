package skilleval

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// regHoldout is a holdout set whose rubric conditions are all "fix: ..."
// markers, so StaticScorer scores a body by how many fix-marker conditions its
// output contains — "more fix: => higher score", matching the fixture contract.
var regHoldout = []SkillCase{
	{ID: "h1", Task: "handle retry", Rubric: []string{"fix: alpha", "fix: beta"}},
	{ID: "h2", Task: "handle backoff", Rubric: []string{"fix: gamma", "fix: delta"}},
}

// goodRegBody satisfies every holdout rubric condition.
const goodRegBody = "fix: alpha fix: beta fix: gamma fix: delta"

// regRunner returns a deterministic Runner that produces a fixed output body.
func regRunner(body string) Runner {
	return func(context.Context) (string, error) { return body, nil }
}

// regBenchmark runs a single body over the holdout with the scorer (as a
// self-baseline, With == Without) and returns the benchmark whose With arm
// holds the body's holdout median score. Borrows RunAB so the fixture genuinely
// grades the body with StaticScorer over the holdout cases.
func regBenchmark(t *testing.T, scorer Scorer, body string, holdout []SkillCase) *SkillBenchmark {
	t.Helper()
	b, err := RunAB(context.Background(), scorer, regRunner(body), regRunner(body), holdout, MinTrials, nil)
	require.NoError(t, err)
	require.NotNil(t, b)
	return b
}

func TestRegressionGate_RejectsCandidateThatRegressesOnHoldout(t *testing.T) {
	var scorer StaticScorer
	gate := NewRegressionGate(DefaultRegressionThreshold)

	// Baseline scores high on the holdout; the candidate (an overfit body that
	// "improved" on train but contains none of the holdout fix markers) scores
	// 0 on the holdout. This is the classic train-improves-but-holdout-regresses
	// trap that the gate exists to reject.
	baseline := regBenchmark(t, scorer, goodRegBody, regHoldout)
	candidate := regBenchmark(t, scorer, "no relevant fix markers at all", regHoldout)

	assert.Greater(t, baseline.With.MedianScore, candidate.With.MedianScore)

	res, err := gate.CheckRegression("demo-skill", baseline, candidate, regHoldout, scorer)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.False(t, res.Passed, "a candidate that regresses more than the threshold must be rejected")
	assert.False(t, res.RegDelta)
	assert.True(t, res.RegTests)
	assert.True(t, res.CatalogAudit)
	require.NotEmpty(t, res.Details)
	assert.Contains(t, strings.Join(res.Details, "\n"), "reject")
}

func TestRegressionGate_PassesCandidateThatImprovesOnHoldout(t *testing.T) {
	var scorer StaticScorer
	gate := NewRegressionGate(DefaultRegressionThreshold)

	// Baseline body only satisfies one fix-marker condition; the candidate body
	// satisfies all of them, so it improves on the holdout. More "fix:" markers
	// => higher StaticScorer score.
	baseline := regBenchmark(t, scorer, "fix: alpha", regHoldout)
	candidate := regBenchmark(t, scorer, goodRegBody, regHoldout)

	assert.Greater(t, candidate.With.MedianScore, baseline.With.MedianScore)

	res, err := gate.CheckRegression("demo-skill", baseline, candidate, regHoldout, scorer)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.True(t, res.Passed, "an improving candidate must pass the regression gate")
	assert.True(t, res.RegDelta)
	assert.True(t, res.RegTests)
	assert.True(t, res.CatalogAudit)
	assert.Contains(t, strings.Join(res.Details, "\n"), "pass")
}

func TestRegressionGate_StableCandidatePassesAtZeroDelta(t *testing.T) {
	var scorer StaticScorer
	gate := NewRegressionGate(DefaultRegressionThreshold)

	base := regBenchmark(t, scorer, goodRegBody, regHoldout)
	cand := regBenchmark(t, scorer, goodRegBody, regHoldout)

	res, err := gate.CheckRegression("demo-skill", base, cand, regHoldout, scorer)
	require.NoError(t, err)
	require.NotNil(t, res)

	// No movement is a pass: delta 0 >= -0.02.
	assert.InDelta(t, 0.0, cand.With.MedianScore-base.With.MedianScore, 1e-9)
	assert.True(t, res.Passed)
	assert.True(t, res.RegDelta)
}

func TestRegressionGate_DeltaWithinTolerance_AtExactThreshold(t *testing.T) {
	// Hand-built benchmarks let us pin the delta exactly at the boundary. We use
	// quarter fractions (0.5 and 0.25 are exactly representable in binary) so the
	// subtraction is exact and the boundary is not blurred by float error.
	// A delta equal to -threshold is NOT a regression (it is >= -threshold).
	baseline := &SkillBenchmark{With: Condition{MedianScore: 0.50}}
	candidate := &SkillBenchmark{With: Condition{MedianScore: 0.25}}
	delta := candidate.With.MedianScore - baseline.With.MedianScore
	assert.InDelta(t, -0.25, delta, 1e-9)

	gate := NewRegressionGate(0.25)
	res, err := gate.CheckRegression("demo-skill", baseline, candidate, regHoldout, StaticScorer{})
	require.NoError(t, err)
	assert.True(t, res.Passed, "a delta of exactly -threshold is within tolerance")
	assert.True(t, res.RegDelta)
}

func TestRegressionGate_DeltaBeyondThreshold_Rejected(t *testing.T) {
	baseline := &SkillBenchmark{With: Condition{MedianScore: 0.80}}
	candidate := &SkillBenchmark{With: Condition{MedianScore: 0.70}}
	delta := candidate.With.MedianScore - baseline.With.MedianScore
	assert.InDelta(t, -0.10, delta, 1e-9)

	gate := NewRegressionGate(DefaultRegressionThreshold)
	res, err := gate.CheckRegression("demo-skill", baseline, candidate, regHoldout, StaticScorer{})
	require.NoError(t, err)
	assert.False(t, res.Passed, "a delta beyond -threshold must be rejected")
	assert.False(t, res.RegDelta)
}

func TestRegressionGate_CustomThreshold(t *testing.T) {
	baseline := &SkillBenchmark{With: Condition{MedianScore: 0.80}}
	candidate := &SkillBenchmark{With: Condition{MedianScore: 0.40}}
	delta := candidate.With.MedianScore - baseline.With.MedianScore
	assert.InDelta(t, -0.40, delta, 1e-9)

	// A loose custom threshold of 0.5 tolerates the 0.4 loss.
	gate := NewRegressionGate(0.5)
	res, err := gate.CheckRegression("demo-skill", baseline, candidate, regHoldout, StaticScorer{})
	require.NoError(t, err)
	assert.True(t, res.Passed, "custom threshold 0.5 must tolerate a 0.4 loss")

	// The strict default (0.02) rejects the same 0.4 loss.
	strict := NewRegressionGate(DefaultRegressionThreshold)
	strictRes, err := strict.CheckRegression("demo-skill", baseline, candidate, regHoldout, StaticScorer{})
	require.NoError(t, err)
	assert.False(t, strictRes.Passed)
}

func TestRegressionGate_DeltaRecordedInDetails(t *testing.T) {
	baseline := &SkillBenchmark{With: Condition{MedianScore: 0.80}}
	candidate := &SkillBenchmark{With: Condition{MedianScore: 0.55}}

	gate := NewRegressionGate(DefaultRegressionThreshold)
	res, err := gate.CheckRegression("demo-skill", baseline, candidate, regHoldout, StaticScorer{})
	require.NoError(t, err)

	joined := strings.Join(res.Details, "\n")
	assert.Contains(t, joined, "-0.2500")
	assert.Contains(t, joined, "0.8000")
	assert.Contains(t, joined, "0.5500")
	assert.Contains(t, joined, "2 case(s)")
	assert.False(t, res.Passed)
}

func TestRegressionGate_HoldoutDisjointFromTrain(t *testing.T) {
	var scorer StaticScorer

	// A realistic flow: build a dataset, split it, and confirm the holdout the
	// gate runs on never overlaps the train split. The regression gate is the
	// holdout guard, so a case used for fitness (train) must never enter it.
	// Every case shares one rubric marker ("fix: common") so the bodies can be
	// graded identically regardless of which subset becomes the holdout.
	dataset := make([]SkillCase, 24)
	for i := range dataset {
		dataset[i] = SkillCase{ID: "c" + strconv.Itoa(i), Rubric: []string{"fix: common"}}
	}
	split, err := SplitEval("demo-skill", dataset, DefaultSplitRatio, 77)
	require.NoError(t, err)
	require.NotEmpty(t, split.Holdout)

	// Baseline satisfies the shared marker on the holdout; the overfit candidate
	// contains none of it, so it regresses on the holdout and must be rejected.
	baseline := regBenchmark(t, scorer, "fix: common", split.Holdout)
	candidate := regBenchmark(t, scorer, "only train-relevant text", split.Holdout)

	res, err := NewRegressionGate(DefaultRegressionThreshold).
		CheckRegression("demo-skill", baseline, candidate, split.Holdout, scorer)
	require.NoError(t, err)
	assert.False(t, res.Passed)

	// Prove the holdout the gate consumed is disjoint from train and val: the
	// gate only ever sees holdout evidence (the benchmarks are built from the
	// holdout and the holdout slice is passed in), never a train case.
	trainSet := idSet(split.Train)
	valSet := idSet(split.Val)
	for _, c := range split.Holdout {
		_, inTrain := trainSet[c.ID]
		_, inVal := valSet[c.ID]
		assert.False(t, inTrain, "holdout case %s must not be in train", c.ID)
		assert.False(t, inVal, "holdout case %s must not be in val", c.ID)
	}
}

func TestRegressionGate_EmptyHoldoutFailsClosed(t *testing.T) {
	baseline := &SkillBenchmark{With: Condition{MedianScore: 0.8}}
	candidate := &SkillBenchmark{With: Condition{MedianScore: 0.8}}

	_, err := NewRegressionGate(DefaultRegressionThreshold).
		CheckRegression("demo-skill", baseline, candidate, nil, StaticScorer{})
	require.Error(t, err)
}

func TestRegressionGate_NilScorerFailsClosed(t *testing.T) {
	baseline := &SkillBenchmark{With: Condition{MedianScore: 0.8}}
	candidate := &SkillBenchmark{With: Condition{MedianScore: 0.8}}

	_, err := NewRegressionGate(DefaultRegressionThreshold).
		CheckRegression("demo-skill", baseline, candidate, regHoldout, nil)
	require.Error(t, err)
}

func TestRegressionGate_NilBenchmarkFailsClosed(t *testing.T) {
	_, err := NewRegressionGate(DefaultRegressionThreshold).
		CheckRegression("demo-skill", nil, &SkillBenchmark{With: Condition{MedianScore: 0.8}}, regHoldout, StaticScorer{})
	require.Error(t, err)

	_, err = NewRegressionGate(DefaultRegressionThreshold).
		CheckRegression("demo-skill", &SkillBenchmark{With: Condition{MedianScore: 0.8}}, nil, regHoldout, StaticScorer{})
	require.Error(t, err)
}

func TestRegressionGate_ZeroThresholdUsesDefault(t *testing.T) {
	var scorer StaticScorer
	gate := &RegressionGate{} // Threshold 0 => default 0.02

	baseline := regBenchmark(t, scorer, goodRegBody, regHoldout)
	candidate := regBenchmark(t, scorer, goodRegBody, regHoldout)
	res, err := gate.CheckRegression("demo-skill", baseline, candidate, regHoldout, scorer)
	require.NoError(t, err)
	assert.True(t, res.Passed)
}
