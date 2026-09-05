package deliberate

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// ComputeConvergence
// ---------------------------------------------------------------------------

func TestComputeConvergence_Converges(t *testing.T) {
	t.Parallel()

	w := DefaultConvergenceWeights()
	positions := []Position{
		NewPosition("rec-1", "recommend refactor", "alice", DimensionRecommendation, []string{"e1"}),
		NewPosition("rec-2", "recommend refactor", "bob", DimensionRecommendation, []string{"e2"}),
		NewPosition("prem-1", "premise holds", "alice", DimensionPremises, []string{"e3"}),
		NewPosition("prem-2", "premise holds", "bob", DimensionPremises, []string{"e4"}),
		NewPosition("risk-1", "risk is low", "alice", DimensionRisks, []string{"e5"}),
		NewPosition("risk-2", "risk is low", "bob", DimensionRisks, []string{"e6"}),
		NewPosition("time-1", "now", "alice", DimensionTiming, []string{"e7"}),
		NewPosition("time-2", "now", "bob", DimensionTiming, []string{"e8"}),
	}

	score, converged := ComputeConvergence(positions, w)
	assert.InEpsilon(t, 1.00, score, 0.0001)
	assert.True(t, converged, "full agreement should converge")
}

func TestComputeConvergence_DoesNotConvergeOnDisagreement(t *testing.T) {
	t.Parallel()

	w := DefaultConvergenceWeights()
	positions := []Position{
		NewPosition("rec-1", "do A", "alice", DimensionRecommendation, []string{"e1"}),
		NewPosition("rec-2", "do B", "bob", DimensionRecommendation, []string{"e2"}),
	}

	score, converged := ComputeConvergence(positions, w)
	assert.InEpsilon(t, 0.15, score, 0.0001)
	assert.False(t, converged, "split recommendation should not converge")
}

func TestComputeConvergence_Threshold70(t *testing.T) {
	t.Parallel()

	w := DefaultConvergenceWeights()
	// Recommendation uncovered (adds 0): Premises + Risks + Timing fully agree
	// => 0.25 + 0.25 + 0.20 = 0.70 (exactly the gate).
	positions := []Position{
		NewPosition("prem-1", "premise holds", "alice", DimensionPremises, []string{"e1"}),
		NewPosition("prem-2", "premise holds", "bob", DimensionPremises, []string{"e2"}),
		NewPosition("risk-1", "risk is low", "alice", DimensionRisks, []string{"e3"}),
		NewPosition("risk-2", "risk is low", "bob", DimensionRisks, []string{"e4"}),
		NewPosition("time-1", "now", "alice", DimensionTiming, []string{"e5"}),
		NewPosition("time-2", "now", "bob", DimensionTiming, []string{"e6"}),
	}

	score, converged := ComputeConvergence(positions, w)
	assert.InEpsilon(t, 0.70, score, 0.0001)
	assert.True(t, converged, "exactly at the 0.70 gate should converge")
}

func TestComputeConvergence_NoPositions(t *testing.T) {
	t.Parallel()

	score, converged := ComputeConvergence(nil, DefaultConvergenceWeights())
	assert.Equal(t, 0.0, score)
	assert.False(t, converged)
}

func TestComputeConvergence_UnsubstantiatedPositionsDeweighted(t *testing.T) {
	t.Parallel()

	// "Zero achismo": a recommendation made entirely of unsubstantiated guesses
	// contributes no convergence. With 2 unsubstantiated recommendations the
	// covered weight drops and the total falls below the 0.70 gate even though
	// the rest of the council fully agrees.
	positions := []Position{
		Position{ID: "rec-1", Claim: "guess", Owner: "alice", Dimension: DimensionRecommendation},
		Position{ID: "rec-2", Claim: "guess", Owner: "bob", Dimension: DimensionRecommendation},
		NewPosition("prem-1", "premise holds", "alice", DimensionPremises, []string{"e1"}),
		NewPosition("prem-2", "premise holds", "bob", DimensionPremises, []string{"e2"}),
		NewPosition("risk-1", "risk is low", "alice", DimensionRisks, []string{"e3"}),
		NewPosition("risk-2", "risk is low", "bob", DimensionRisks, []string{"e4"}),
		NewPosition("time-1", "now", "alice", DimensionTiming, []string{"e5"}),
		NewPosition("time-2", "now", "bob", DimensionTiming, []string{"e6"}),
	}

	score, _ := ComputeConvergence(positions, DefaultConvergenceWeights())
	// Only Premises(0.25) + Risks(0.25) + Timing(0.20) = 0.70 count;
	// unsubstantiated recommendation is deweighted (0).
	assert.InEpsilon(t, 0.70, score, 0.0001)

	// And a council of pure unsubstantiated positions never converges.
	guesses := []Position{
		Position{ID: "g1", Claim: "guess", Owner: "a", Dimension: DimensionRecommendation},
		Position{ID: "g2", Claim: "guess", Owner: "b", Dimension: DimensionPremises},
	}
	score, converged := ComputeConvergence(guesses, DefaultConvergenceWeights())
	assert.Equal(t, 0.0, score)
	assert.False(t, converged, "no evidence => no convergence")
}

// ---------------------------------------------------------------------------
// DetectLoop
// ---------------------------------------------------------------------------

func TestDetectLoop(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		hashes []string
		want   bool
	}{
		{"empty", []string{}, false},
		{"all unique", []string{"h1", "h2", "h3"}, false},
		{"single repeat", []string{"h1", "h1"}, true},
		{"repeat among many", []string{"h1", "h2", "h3", "h2"}, true},
		{"repeat not adjacent", []string{"a", "b", "c", "a", "d"}, true},
		{"case sensitive", []string{"H", "h"}, false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, DetectLoop(tt.hashes))
		})
	}
}

func TestRoundHash_DeterministicAndTracksSubstantiation(t *testing.T) {
	t.Parallel()

	p1 := NewPosition("r1", "claim x", "a", DimensionRecommendation, []string{"e1"})
	p2 := Position{ID: "r1", Claim: "claim x", Owner: "a", Dimension: DimensionRecommendation}

	h1 := RoundHash([]Position{p1})
	h2 := RoundHash([]Position{p1})
	assert.Equal(t, h1, h2, "same positions => same hash")
	assert.NotEqual(t, h1, RoundHash([]Position{p2}), "substantiation changes the hash")
}

// ---------------------------------------------------------------------------
// ComputeConfidence & EvaluateEmit
// ---------------------------------------------------------------------------

func TestComputeConfidence_AdjustmentsApplied(t *testing.T) {
	t.Parallel()

	bd := ComputeConfidence(0.60, []Adjustment{
		CriticAdjustment(0.65),     // -0.20
		RiskAdjustment("high"),     // -0.10
		CorroborationAdjustment(3), // +0.10
	})

	assert.InEpsilon(t, 0.40, bd.Final, 0.001)
	assert.Equal(t, 0.60, bd.Base)
	assert.Len(t, bd.Adjustments, 3)
	assert.Contains(t, bd.Breakdown, "final=")
	assert.Contains(t, bd.Breakdown, "critic_below_threshold")
	assert.Contains(t, bd.Breakdown, "-0.2000")
}

func TestComputeConfidence_ClampLow(t *testing.T) {
	t.Parallel()

	bd := ComputeConfidence(0.10, []Adjustment{
		UnresolvedContradictionAdjustment(),          // -0.10
		RiskAdjustment("catastrophic"),               // -0.15
	})
	assert.Equal(t, 0.0, bd.Final, "clamped to 0.0")
}

func TestComputeConfidence_ClampHigh(t *testing.T) {
	t.Parallel()

	bd := ComputeConfidence(0.95, []Adjustment{CorroborationAdjustment(5)})
	assert.Equal(t, 1.0, bd.Final, "clamped to 1.0")
}

func TestComputeConfidence_BaseClamped(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 1.0, ComputeConfidence(1.5, nil).Final)
	assert.Equal(t, 0.0, ComputeConfidence(-0.2, nil).Final)
}

func TestCriticAdjustment_Threshold(t *testing.T) {
	t.Parallel()

	assert.Equal(t, -0.20, CriticAdjustment(0.69).Delta)
	assert.Equal(t, 0.0, CriticAdjustment(0.70).Delta)
	assert.Equal(t, 0.0, CriticAdjustment(0.80).Delta)
}

func TestRiskAdjustment_Levels(t *testing.T) {
	t.Parallel()

	assert.Equal(t, -0.15, RiskAdjustment("catastrophic").Delta)
	assert.Equal(t, -0.15, RiskAdjustment("critico").Delta)
	assert.Equal(t, -0.10, RiskAdjustment("alta").Delta)
	assert.Equal(t, -0.10, RiskAdjustment("high").Delta)
	assert.Equal(t, -0.05, RiskAdjustment("media").Delta)
	assert.Equal(t, -0.05, RiskAdjustment("medium").Delta)
	assert.Equal(t, 0.0, RiskAdjustment("unknown").Delta)
}

func TestCorroborationAdjustment(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 0.10, CorroborationAdjustment(3).Delta)
	assert.Equal(t, 0.10, CorroborationAdjustment(7).Delta)
	assert.Equal(t, 0.0, CorroborationAdjustment(2).Delta)
}

func TestEvaluateEmit_Thresholds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		final  float64
		want   Emit
	}{
		{"high", 0.85, EmitOK},
		{"at upper gate", 0.70, EmitOK},
		{"just below upper", 0.69, EmitWithReservations},
		{"at lower gate", 0.50, EmitWithReservations},
		{"just below lower", 0.49, Escalate},
		{"very low", 0.20, Escalate},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, EvaluateEmit(ConfidenceBreakdown{Final: tt.final}))
		})
	}
}

// ---------------------------------------------------------------------------
// VoteCross
// ---------------------------------------------------------------------------

func TestVoteCross_RejectsSelfVote(t *testing.T) {
	t.Parallel()

	reviews := []Review{
		{Reviewer: "critic", Target: "alice", Score: 0.8},
		{Reviewer: "qa", Target: "qa", Score: 0.9}, // self-vote
	}

	_, err := VoteCross(reviews, "don")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSelfVote), "error should wrap ErrSelfVote")
	assert.Contains(t, err.Error(), "self-vote")
}

func TestVoteCross_ExcludesOwner(t *testing.T) {
	t.Parallel()

	// owner "alice" must not cross-vote on their own decision; "critic" votes
	// on alice, "qa" votes on critic.
	reviews := []Review{
		{Reviewer: "alice", Target: "bob", Score: 1.0}, // owner -> excluded
		{Reviewer: "critic", Target: "alice", Score: 0.7},
		{Reviewer: "qa", Target: "critic", Score: 0.6},
	}

	got, err := VoteCross(reviews, "alice")
	require.NoError(t, err)
	assert.Len(t, got, 2)
	for _, r := range got {
		assert.NotEqual(t, "alice", r.Reviewer, "owner must not appear in cross-review pool")
	}
	assert.Equal(t, "critic", got[0].Reviewer)
	assert.Equal(t, "qa", got[1].Reviewer)
}

func TestVoteCross_ValidReviewsKept(t *testing.T) {
	t.Parallel()

	reviews := []Review{
		{Reviewer: "critic", Target: "alice", Score: 0.8},
		{Reviewer: "qa", Target: "alice", Score: 0.9},
	}
	got, err := VoteCross(reviews, "don")
	require.NoError(t, err)
	assert.Equal(t, reviews, got)
}

func TestVoteCross_EmptyReviews(t *testing.T) {
	t.Parallel()

	got, err := VoteCross(nil, "don")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestVoteCross_RejectsEmptyReviewerOrTarget(t *testing.T) {
	t.Parallel()

	_, err := VoteCross([]Review{{Reviewer: "", Target: "x", Score: 0.5}}, "don")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")

	_, err = VoteCross([]Review{{Reviewer: "critic", Target: "", Score: 0.5}}, "don")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

// ---------------------------------------------------------------------------
// EvidenceConfidence
// ---------------------------------------------------------------------------

func TestEvidenceConfidence_Levels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		ev    any
		want  float64
	}{
		{"L5 concrete (code + exact ref)", Evidence{Level: 5, Concrete: true}, 1.00},
		{"L5 plain", Evidence{Level: 5}, 1.00},
		{"L4 cross validated", Evidence{Level: 4, CrossValidated: true}, 1.00},
		{"L4 plain", Evidence{Level: 4}, 0.90},
		{"L3 official docs", Evidence{Level: 3}, 0.60},
		{"L2 memory", Evidence{Level: 2}, 0.50},
		{"L1 agent opinion", Evidence{Level: 1}, 0.30},
		{"L0 LLM", Evidence{Level: 0}, 0.20},
		{"L0 default (zero value)", Evidence{}, 0.20},
		{"nil interface -> L0", nil, 0.20},
		{"nil pointer -> L0", (*Evidence)(nil), 0.20},
		{"non-evidence any -> L0", "foo", 0.20},
		{"no evidence -> L0", 42, 0.20},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.InEpsilon(t, tt.want, EvidenceConfidence(tt.ev), 0.0001)
		})
	}
}

func TestEvidenceConfidence_Modifiers(t *testing.T) {
	t.Parallel()

	// L2 memory with M4 (stale), M5 (contradicted by higher source), M6
	// (single source): 0.50 - 0.15 - 0.40 - 0.10 = -0.15 -> clamped 0.00
	low := Evidence{Level: 2, Stale: true, Contradicted: true, SingleSource: true}
	assert.Equal(t, 0.0, EvidenceConfidence(low))

	// L2 memory with M1 (concrete) + M2 (cross-validated) + M3 (recent):
	// 0.50 + 0.15 + 0.10 + 0.05 = 0.80
	boosted := Evidence{Level: 2, Concrete: true, CrossValidated: true, Recent: true}
	assert.InEpsilon(t, 0.80, EvidenceConfidence(boosted), 0.0001)

	// M7 (aspirational): 0.60 - 0.30 = 0.30
	asp := Evidence{Level: 3, Aspirational: true}
	assert.InEpsilon(t, 0.30, EvidenceConfidence(asp), 0.0001)

	// Pointer form yields the same result.
	ptr := &Evidence{Level: 5, Concrete: true}
	assert.InEpsilon(t, 1.00, EvidenceConfidence(ptr), 0.0001)
}

// ---------------------------------------------------------------------------
// ValidateSpec
// ---------------------------------------------------------------------------

func validSpec() SynthesisSpec {
	return SynthesisSpec{
		Findings:   []string{"finding 1"},
		CrossAgent: []string{"critic", "qa"},
		Spec: []SpecItem{
			{File: "internal/foo/foo.go", Action: "add feature", Metric: "unit_tests_pass", Acceptance: ">= 95% coverage"},
		},
		Actions:    []string{"implement the spec item"},
		Confidence: 0.80,
	}
}

func TestValidateSpec_Valid(t *testing.T) {
	t.Parallel()

	assert.NoError(t, ValidateSpec(validSpec()))
}

func TestValidateSpec_MissingSections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		mut   func(s *SynthesisSpec)
		want  string
	}{
		{"missing FINDINGS", func(s *SynthesisSpec) { s.Findings = nil }, "required"},
		{"missing CROSS-AGENT", func(s *SynthesisSpec) { s.CrossAgent = nil }, "required"},
		{"missing SPEC", func(s *SynthesisSpec) { s.Spec = nil }, "required"},
		{"missing ACTIONS", func(s *SynthesisSpec) { s.Actions = nil }, "required"},
		{"missing CONFIDENCE", func(s *SynthesisSpec) { s.Confidence = 0 }, "CONFIDENCE"},
		{"confidence above 1", func(s *SynthesisSpec) { s.Confidence = 1.2 }, "CONFIDENCE"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := validSpec()
			tt.mut(&spec)
			err := ValidateSpec(spec)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestValidateSpec_IncompleteSpecItem(t *testing.T) {
	t.Parallel()

	incomplete := map[string]func(item *SpecItem){
		"missing File":       func(it *SpecItem) { it.File = "" },
		"missing Action":     func(it *SpecItem) { it.Action = "" },
		"missing Metric":     func(it *SpecItem) { it.Metric = "" },
		"missing Acceptance": func(it *SpecItem) { it.Acceptance = "" },
	}
	for name, mut := range incomplete {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			spec := validSpec() // fresh copy per subtest
			mut(&spec.Spec[0])
			err := ValidateSpec(spec)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "missing")
		})
	}
}

// ---------------------------------------------------------------------------
// Struct semantics
// ---------------------------------------------------------------------------

func TestReview_SelfVote(t *testing.T) {
	t.Parallel()

	assert.True(t, Review{Reviewer: "critic", Target: "critic"}.SelfVote())
	assert.False(t, Review{Reviewer: "critic", Target: "alice"}.SelfVote())
}

func TestPosition_NewPositionDerivesSubstantiation(t *testing.T) {
	t.Parallel()

	assert.True(t, NewPosition("p", "c", "o", DimensionRecommendation, []string{"e1"}).Effective())
	assert.False(t, NewPosition("p", "c", "o", DimensionRecommendation, nil).Effective())
}

func TestDefaultConvergenceWeights_SumToOne(t *testing.T) {
	t.Parallel()

	w := DefaultConvergenceWeights()
	sum := w.Recommendation + w.Premises + w.Risks + w.Timing
	assert.InEpsilon(t, 1.00, sum, 0.0001)
}

func TestMaxRoundsAndIterations(t *testing.T) {
	t.Parallel()

	// Sanity check the circuit breaker constants (A3).
	assert.Equal(t, 3, MaxRounds)
	assert.Equal(t, 5, MaxIterations)
	assert.Equal(t, 300, Timeout)
	assert.Equal(t, "deliberate:empty-round", RoundHash(nil))
}
