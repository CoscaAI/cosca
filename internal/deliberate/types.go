// Package deliberate implements the deterministic meta-cognitive evidence-gated
// deliberation layer of the Cosca Mega Brain (ADR-011, Bloco 1, Fatia D1).
//
// The core is ZERO-LLM by design (P9 "a IA propõe, o sistema decide"): the model
// only fills the Positions (claim + evidence IDs); every arithmetic gate
// (convergence, confidence breakdown, emit/threshold, self-vote rejection and
// spec validation) is computed here, deterministically, with no network.
//
// The arithmetic and gates are EXTERNAL to the model, which corrects the
// confirmation/self-evaluation bias described in ADR-011 §1.1. Everything in
// this package is testable via `go test ./internal/deliberate/...` with no LLM.
//
// References:
//   - ADR-011: docs/adr/ADR-011-mega-brain-deliberation-and-plan-only.md (Bloco 1)
//   - CONFIDENCE_MODEL.md (L0–L5 + M1–M7): internal/embed/cosca/engines/evidence/
package deliberate

// Dimension identifies which aspect of a decision a Position addresses. The
// four dimensions are weighted by ConvergenceWeights (A3): a decision
// converges when the weighted agreement across these dimensions is >= 0.70.
type Dimension string

// The four council dimensions weighed during convergence (ADR-011 A3).
const (
	// DimensionRecommendation consolidates the "what to do" stance.
	DimensionRecommendation Dimension = "recommendation"
	// DimensionPremises consolidates the factual/rationale stance.
	DimensionPremises Dimension = "premises"
	// DimensionRisks consolidates the risk stance.
	DimensionRisks Dimension = "risks"
	// DimensionTiming consolidates the scheduling stance.
	DimensionTiming Dimension = "timing"
)

// Position is a stance taken by an agent in the deliberation council (A2).
//
// Evidence-gating ("zero achismo"): a position is only Substantiated when it
// carries at least one traceable evidence ID. A position with no evidence is
// deweighted — it contributes nothing to convergence and it is a documented
// "unsubstantiated" claim (never taken as fact).
//
// Dimension groups the position into one of the four convergence dimensions
// (recommendation / premises / risks / timing). It is an implementation
// extension required by ComputeConvergence to apply the per-dimension weights.
type Position struct {
	ID            string
	Claim         string
	EvidenceIDs   []string
	Owner         string
	Substantiated bool
	Dimension     Dimension
}

// NewPosition builds a Position and derives Substantiated from the evidence
// IDs. "Zero achismo": no evidence => not substantiated.
func NewPosition(id, claim, owner string, dimension Dimension, evidenceIDs []string) Position {
	return Position{
		ID:            id,
		Claim:         claim,
		Owner:         owner,
		Dimension:     dimension,
		EvidenceIDs:   evidenceIDs,
		Substantiated: len(evidenceIDs) > 0,
	}
}

// Effective reports whether the position counts toward convergence. A position
// is effective only when it is both marked substantiated AND carries evidence
// IDs — enforcing the "zero achismo" gate even if a caller sets the flag
// inconsistently. Unsubstantiated positions are ignored (deweighted).
func (p Position) Effective() bool {
	return p.Substantiated && len(p.EvidenceIDs) > 0
}

// Review is one reviewer's assessment of a target claim (A5). A review voting
// on itself (Target == Reviewer) is a SelfVote and is rejected by VoteCross:
// nobody escorts themself. Neither the critic nor the decision owner votes on
// their own position.
type Review struct {
	Reviewer string
	Target   string
	Score    float64
}

// SelfVote reports whether the review votes for itself (Target == Reviewer).
// Self-votes are invalid (A5) and rejected by VoteCross.
func (r Review) SelfVote() bool {
	return r.Target == r.Reviewer
}

// Decision is the deterministic result of a deliberation. It is what the
// system (not the model) emits after all arithmetic gates are applied (P9).
type Decision struct {
	Positions  []Position
	Converged  bool
	Confidence ConfidenceBreakdown
	Emit       Emit
}

// Emit is the emission verdict produced by the deterministic confidence gates
// (A4). The system decides whether to emit, emit with reservations, or
// escalate; the model only proposes.
type Emit string

// Emission verdicts (A4 thresholds: >=0.70 emit; 0.50–0.69 emit with
// reservations; <0.50 escalate). Escalate does NOT re-run deliberation
// (anti-loop, same threshold as ADR-011 §3.1).
const (
	// EmitOK means confidence >= 0.70: safe to emit the decision.
	EmitOK Emit = "emit_ok"
	// EmitWithReservations means 0.50 <= confidence < 0.70: emit but attach the
	// mitigation plan.
	EmitWithReservations Emit = "emit_with_reservations"
	// Escalate means confidence < 0.50: do not emit; escalate to the Don.
	Escalate Emit = "escalate"
)

// ConvergenceWeights are the A3 weights for the four convergence dimensions.
// They sum to 1.00: Recommendation 0.30, Premises 0.25, Risks 0.25, Timing 0.20.
type ConvergenceWeights struct {
	Recommendation float64
	Premises       float64
	Risks          float64
	Timing         float64
}

// DefaultConvergenceWeights returns the A3 defaults for ConvergenceWeights.
func DefaultConvergenceWeights() ConvergenceWeights {
	return ConvergenceWeights{
		Recommendation: 0.30,
		Premises:       0.25,
		Risks:          0.25,
		Timing:         0.20,
	}
}

// Adjustment is a single typed, traceable confidence adjustment (A4). Each
// adjustment carries a human-readable Reason and a deterministic Delta
// (negative to rebaixa, positive to corroborate). Adjustments are applied in
// order; the resulting total is clamped to [0,1].
type Adjustment struct {
	Reason string
	Delta  float64
}

// ConfidenceBreakdown is the audit-ready arithmetic confidence result (A4).
//
//	Final = clamp01(Base + sum(Adjustments[*].Delta))
//
// Breakdown is a deterministic human-readable trace of how Final was reached.
type ConfidenceBreakdown struct {
	Base        float64
	Adjustments []Adjustment
	Final       float64
	Breakdown   string
}

// SynthesisSpec is a plan-only, actionable SPEC (A7) — never a narrative
// summary. It requires the FINDINGS / CROSS-AGENT / SPEC / ACTIONS /
// CONFIDENCE sections, each validated by ValidateSpec.
type SynthesisSpec struct {
	Findings   []string
	CrossAgent []string
	Spec       []SpecItem
	Actions    []string
	Confidence float64
}

// SpecItem is one actionable, auditable work item (A7): file + action +
// metric + acceptance. A SpecItem with any empty field is invalid.
type SpecItem struct {
	File       string
	Action     string
	Metric     string
	Acceptance string
}
