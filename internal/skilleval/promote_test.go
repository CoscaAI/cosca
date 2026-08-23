package skilleval

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// promoteDataset builds a dataset big enough to yield a 50/25/25 split with a
// non-empty holdout. Every case rewards the same marker token, so the split is
// deterministic and whichever subset lands in the holdout grades the two bodies
// identically: a baseline/candidate body is scored purely by whether it
// contains (or omits) that marker.
func promoteDataset(n int, marker string) []SkillCase {
	cases := make([]SkillCase, 0, n)
	for i := 0; i < n; i++ {
		cases = append(cases, SkillCase{
			ID:     "p" + strconv.Itoa(i),
			Task:   "case " + strconv.Itoa(i),
			Rubric: []string{marker},
		})
	}
	return cases
}

func TestPromotionGate_RejectsCandidateThatRegressesOnHoldout(t *testing.T) {
	// The baseline satisfies the holdout marker; the candidate (an overfit body
	// that "improved" on train/val but drops the holdout marker) scores 0 on the
	// holdout. This is the classic train-improves-but-holdout-regresses trap the
	// integrated gate exists to reject.
	cases := promoteDataset(12, "fix: holdout")
	baselineBody := "fix: holdout"
	candidateBody := "evolved only; dropped the holdout marker"

	pr, err := NewPromotionGate(DefaultRegressionThreshold, DefaultSplitRatio, DefaultPromotionSeed).
		Evaluate(context.Background(), "demo-skill", cases, baselineBody, candidateBody, StaticScorer{}, MinTrials)
	require.NoError(t, err)
	require.NotNil(t, pr)
	require.NotNil(t, pr.Gate)
	require.NotNil(t, pr.Split)
	require.NotEmpty(t, pr.Split.Holdout)

	// The gate verdict is a rejection: the candidate regressed on the holdout.
	assert.False(t, pr.Gate.Passed, "a candidate that regresses on the holdout must NOT be promoted")
	assert.False(t, pr.Gate.RegDelta)
	assert.Less(t, pr.RegDelta, float64(-DefaultRegressionThreshold), "regression delta must exceed the tolerance")
	assert.Greater(t, pr.BaselineHoldout.With.MedianScore, pr.CandidateHoldout.With.MedianScore)
	assert.Contains(t, strings.Join(pr.Gate.Details, "\n"), "reject")
	assert.Contains(t, strings.Join(pr.Gate.Details, "\n"), "holdout")
}

func TestPromotionGate_PassesCandidateThatImprovesOnHoldout(t *testing.T) {
	cases := promoteDataset(12, "fix: holdout")
	baselineBody := "no relevant marker"
	candidateBody := "fix: holdout"

	pr, err := NewPromotionGate(DefaultRegressionThreshold, DefaultSplitRatio, DefaultPromotionSeed).
		Evaluate(context.Background(), "demo-skill", cases, baselineBody, candidateBody, StaticScorer{}, MinTrials)
	require.NoError(t, err)
	require.NotNil(t, pr)
	require.NotNil(t, pr.Gate)

	assert.True(t, pr.Gate.Passed, "a candidate that improves on the holdout must be promotable")
	assert.True(t, pr.Gate.RegDelta)
	assert.Greater(t, pr.RegDelta, float64(DefaultRegressionThreshold), "improvement delta should be above tolerance")
	assert.Greater(t, pr.CandidateHoldout.With.MedianScore, pr.BaselineHoldout.With.MedianScore)
	assert.Contains(t, strings.Join(pr.Gate.Details, "\n"), "pass")
}

func TestPromotionGate_HoldoutNeverLeaksIntoTrain(t *testing.T) {
	// The integrated gate's contract is anti-overfit: the holdout it grades is
	// disjoint from the train (and val) set, so the regression verdict never
	// sees a case used for fitness. Verify the split the gate consumed.
	cases := promoteDataset(12, "fix: holdout")
	pr, err := NewPromotionGate(DefaultRegressionThreshold, DefaultSplitRatio, DefaultPromotionSeed).
		Evaluate(context.Background(), "demo-skill", cases, "fix: holdout", "fix: holdout", StaticScorer{}, MinTrials)
	require.NoError(t, err)
	require.NotNil(t, pr)
	require.NotNil(t, pr.Split)

	trainSet := idSet(pr.Split.Train)
	valSet := idSet(pr.Split.Val)
	for _, c := range pr.Split.Holdout {
		_, inTrain := trainSet[c.ID]
		_, inVal := valSet[c.ID]
		assert.False(t, inTrain, "holdout case %s must not be in train", c.ID)
		assert.False(t, inVal, "holdout case %s must not be in val", c.ID)
	}
}

func TestPromotionGate_StableCandidatePassesAtZeroDelta(t *testing.T) {
	cases := promoteDataset(12, "fix: holdout")
	// Same body -> no movement on the holdout (delta 0 >= -0.02) -> pass.
	pr, err := NewPromotionGate(DefaultRegressionThreshold, DefaultSplitRatio, DefaultPromotionSeed).
		Evaluate(context.Background(), "demo-skill", cases, "fix: holdout", "fix: holdout", StaticScorer{}, MinTrials)
	require.NoError(t, err)
	require.NotNil(t, pr)
	require.NotNil(t, pr.Gate)

	assert.InDelta(t, 0.0, pr.RegDelta, 1e-9)
	assert.True(t, pr.Gate.Passed)
	assert.True(t, pr.Gate.RegDelta)
}

func TestPromotionGate_TooSmallDataset_FailsClosed(t *testing.T) {
	// A dataset below MinSplitCases cannot yield a holdout; the gate must fail
	// closed (never silently skip the anti-overfit check).
	_, err := NewPromotionGate(DefaultRegressionThreshold, DefaultSplitRatio, DefaultPromotionSeed).
		Evaluate(context.Background(), "demo-skill", promoteDataset(2, "fix: holdout"), "fix: holdout", "fix: holdout", StaticScorer{}, MinTrials)
	require.Error(t, err)
}

func TestPromotionGate_NilScorer_FailsClosed(t *testing.T) {
	_, err := NewPromotionGate(DefaultRegressionThreshold, DefaultSplitRatio, DefaultPromotionSeed).
		Evaluate(context.Background(), "demo-skill", promoteDataset(12, "fix: holdout"), "fix: holdout", "fix: holdout", nil, MinTrials)
	require.Error(t, err)
}

func TestPromotionGate_DeterministicAcrossRuns(t *testing.T) {
	cases := promoteDataset(12, "fix: holdout")
	first, err := NewPromotionGate(DefaultRegressionThreshold, DefaultSplitRatio, DefaultPromotionSeed).
		Evaluate(context.Background(), "demo-skill", cases, "fix: holdout", "no marker", StaticScorer{}, MinTrials)
	require.NoError(t, err)
	second, err := NewPromotionGate(DefaultRegressionThreshold, DefaultSplitRatio, DefaultPromotionSeed).
		Evaluate(context.Background(), "demo-skill", cases, "fix: holdout", "no marker", StaticScorer{}, MinTrials)
	require.NoError(t, err)

	// Same (skill, seed) => identical split and identical gate verdict.
	assert.Equal(t, promoteSplitIDs(first.Split), promoteSplitIDs(second.Split))
	assert.Equal(t, first.Gate.Passed, second.Gate.Passed)
	assert.InDelta(t, first.RegDelta, second.RegDelta, 1e-9)
}

// promoteSplitIDs returns a stable slice of the three split sets so tests can
// compare partitions structurally.
func promoteSplitIDs(s *EvalSplit) []string {
	var ids []string
	for _, c := range s.Train {
		ids = append(ids, "t:"+c.ID)
	}
	for _, c := range s.Val {
		ids = append(ids, "v:"+c.ID)
	}
	for _, c := range s.Holdout {
		ids = append(ids, "h:"+c.ID)
	}
	return ids
}
