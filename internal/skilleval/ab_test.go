package skilleval

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	abCase1Rubric = []string{"returns 200", "logs error"}
	abCase2Rubric = []string{"retries once", "exponential backoff"}
	abCases       = []SkillCase{
		{ID: "c1", Task: "handle request", Rubric: abCase1Rubric},
		{ID: "c2", Task: "handle retry", Rubric: abCase2Rubric},
	}
	abGoodOutput   = "returns 200 logs error retries once exponential backoff"
	abBadOutput    = "no relevant signal here"
	withGoodRunner = func(context.Context) (string, error) { return abGoodOutput, nil }
	badRunner      = func(context.Context) (string, error) { return abBadOutput, nil }
)

func TestRunAB_WithBetter_PassesGate(t *testing.T) {
	var s StaticScorer
	gate := func(context.Context) *GateResult {
		return &GateResult{CatalogAudit: true, RegTests: true, RegDelta: true, Passed: true}
	}

	b, err := RunAB(context.Background(), s, withGoodRunner, badRunner, abCases, 5, gate)
	require.NoError(t, err)
	require.NotNil(t, b)

	assert.True(t, b.Delta.Candidate)
	assert.True(t, b.IsCandidate)
	assert.InDelta(t, 1.0, b.Delta.Score, 1e-9)
	require.NotNil(t, b.Gate)
	assert.True(t, b.Gate.Passed)
	assert.InDelta(t, 1.0, b.With.MedianScore, 1e-9)
	assert.InDelta(t, 0.0, b.Without.MedianScore, 1e-9)
}

func TestRunAB_WithoutBetter_NotCandidate(t *testing.T) {
	var s StaticScorer
	gate := func(context.Context) *GateResult { return &GateResult{Passed: true} }

	// with arm is the bad one, without arm is the good one.
	b, err := RunAB(context.Background(), s, badRunner, withGoodRunner, abCases, 5, gate)
	require.NoError(t, err)
	require.NotNil(t, b)

	assert.False(t, b.IsCandidate)
	assert.False(t, b.Delta.Candidate)
	assert.InDelta(t, -1.0, b.Delta.Score, 1e-9)
}

func TestRunAB_GateFails_NotCandidate(t *testing.T) {
	var s StaticScorer
	gate := func(context.Context) *GateResult { return &GateResult{Passed: false} }

	b, err := RunAB(context.Background(), s, withGoodRunner, badRunner, abCases, 5, gate)
	require.NoError(t, err)
	require.NotNil(t, b)

	// The A/B signal is strong...
	assert.True(t, b.Delta.Candidate)
	// ...but the regression gate vetoes the promotion.
	assert.False(t, b.IsCandidate)
	require.NotNil(t, b.Gate)
	assert.False(t, b.Gate.Passed)
}

func TestRunAB_NilGate_DoesNotBlock(t *testing.T) {
	var s StaticScorer

	b, err := RunAB(context.Background(), s, withGoodRunner, badRunner, abCases, 5, nil)
	require.NoError(t, err)
	require.NotNil(t, b)

	assert.True(t, b.Delta.Candidate)
	assert.True(t, b.IsCandidate)
	assert.Nil(t, b.Gate)
}

func TestRunAB_RunnerError_DoesNotAbort(t *testing.T) {
	var s StaticScorer
	calls := 0
	withFlaky := func(context.Context) (string, error) {
		calls++
		if calls == 3 {
			return "", errors.New("agent crashed")
		}
		return abGoodOutput, nil
	}

	b, err := RunAB(context.Background(), s, withFlaky, badRunner, abCases, 5, nil)
	require.NoError(t, err)
	require.NotNil(t, b)

	// The failing trial degrades to score 0, but the four good trials dominate
	// the robust median, so the with arm still scores high.
	assert.True(t, b.With.MedianScore > 0.5)
	assert.True(t, b.Delta.Candidate)
}

func TestRunCondition_AlwaysFail_ZeroScore(t *testing.T) {
	var s StaticScorer
	alwaysFail := func(context.Context) (string, error) { return "", errors.New("boom") }

	cond, err := RunCondition(context.Background(), s, alwaysFail, abCases, 5)
	require.NoError(t, err)
	assert.Equal(t, 0.0, cond.MedianScore)
	assert.Equal(t, 0.0, cond.SolveRate)
}

func TestRunAB_FewTrials_NotCandidate(t *testing.T) {
	var s StaticScorer
	gate := func(context.Context) *GateResult { return &GateResult{Passed: true} }

	// 3 < MinTrials (5): even though the with arm looks better, the evidence is
	// too thin to mark a candidate.
	b, err := RunAB(context.Background(), s, withGoodRunner, badRunner, abCases, 3, gate)
	require.NoError(t, err)
	require.NotNil(t, b)

	assert.False(t, b.Delta.Candidate)
	assert.False(t, b.IsCandidate)
	assert.InDelta(t, 1.0, b.With.MedianScore, 1e-9)
}

func TestRunAB_FewTrialsNilGate(t *testing.T) {
	var s StaticScorer

	b, err := RunAB(context.Background(), s, withGoodRunner, badRunner, abCases, 3, nil)
	require.NoError(t, err)
	require.NotNil(t, b)

	assert.Nil(t, b.Gate)
	assert.False(t, b.Delta.Candidate)
	assert.False(t, b.IsCandidate)
}

func TestAbCandidate_MedianAndIQR_NonOverlapping(t *testing.T) {
	with := Condition{MedianScore: 0.9, IQRS: [2]float64{0.8, 1.0}}
	without := Condition{MedianScore: 0.2, IQRS: [2]float64{0.1, 0.3}}

	assert.True(t, abCandidate(with, without, MinTrials))
	// Reversed order: median(with) < median(without) — not a candidate.
	assert.False(t, abCandidate(without, with, MinTrials))
}

func TestAbCandidate_HeavyOverlap_NotCandidate(t *testing.T) {
	with := Condition{MedianScore: 0.5, IQRS: [2]float64{0.3, 0.7}}
	without := Condition{MedianScore: 0.45, IQRS: [2]float64{0.35, 0.55}}

	// Median improved but IQRs overlap heavily — inconclusive, so not a candidate.
	assert.False(t, abCandidate(with, without, MinTrials))
}

func TestAbCandidate_FewTrials_NotCandidate(t *testing.T) {
	with := Condition{MedianScore: 0.9, IQRS: [2]float64{0.8, 1.0}}
	without := Condition{MedianScore: 0.2, IQRS: [2]float64{0.1, 0.3}}

	assert.False(t, abCandidate(with, without, MinTrials-1))
}

func TestIqrOverlapRatio(t *testing.T) {
	cases := []struct {
		name string
		a, b [2]float64
		want float64
	}{
		{"fully separated", [2]float64{0.8, 1.0}, [2]float64{0.1, 0.3}, 0.0},
		{"touching at point", [2]float64{1, 2}, [2]float64{2, 3}, 0.0},
		{"half overlap", [2]float64{0, 1}, [2]float64{0.5, 1.5}, 0.5},
		{"fully contained", [2]float64{0, 1}, [2]float64{0.25, 0.75}, 1.0},
		{"identical", [2]float64{0.5, 0.5}, [2]float64{0.5, 0.5}, 0.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.InDelta(t, tc.want, iqrOverlapRatio(tc.a, tc.b), 1e-9)
		})
	}
}

func TestIqrsNonOverlapping(t *testing.T) {
	assert.True(t, iqrsNonOverlapping([2]float64{0.8, 1.0}, [2]float64{0.1, 0.3}))
	assert.True(t, iqrsNonOverlapping([2]float64{1, 1}, [2]float64{0, 0}))
	assert.False(t, iqrsNonOverlapping([2]float64{1, 2}, [2]float64{2, 3})) // touching
	assert.False(t, iqrsNonOverlapping([2]float64{0.3, 0.7}, [2]float64{0.35, 0.55}))
}

func TestRunCondition_DefaultTrials(t *testing.T) {
	var s StaticScorer
	// trials <= 0 defaults to MinTrials; here it must still produce a Condition.
	cond, err := RunCondition(context.Background(), s, withGoodRunner, abCases, 0)
	require.NoError(t, err)
	assert.InDelta(t, 1.0, cond.MedianScore, 1e-9)
}
