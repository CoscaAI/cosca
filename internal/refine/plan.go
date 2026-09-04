package refine

// RefinementPlan is the output of the P2 phase-1 (plan) step: a Proposal plus
// the Baseline snapshot captured at plan time and the I4 TraceID of the run.
// Apply comparisons rely on Baseline to detect stale writes.
type RefinementPlan struct {
	ID         string        `json:"id"`
	Proposal   Proposal      `json:"proposal"`
	TraceID    string        `json:"trace_id"`
	RollbackOf string        `json:"rollback_of,omitempty"`
	Baseline   StateSnapshot `json:"baseline"`
}

// PlanRefinement runs the P2 phase-1 (plan). It captures a Baseline snapshot of
// the shared state at plan time so that applyRefinement can detect stale writes
// (no-clobber) when it re-reads the state later.
//
// The Proposal (summary/rationale/edits/expectedOutcome) may come from ANY
// auxiliary, non-deterministic source — an external editor, a heuristic, a
// developer request — but Plan itself is deterministic and pure over its
// inputs; it never mutates the state. The decision/application path stays
// zero-LLM (I1).
//
// A non-empty TraceID supplies the I4 provenance that Apply will require before
// promoting the pass (fail-closed I2).
func PlanRefinement(state *State, id string, proposal Proposal, traceID string, rollbackOf string) *RefinementPlan {
	if id == "" {
		id = NewID()
	}
	// Baseline is always captured on this path (default-não-clobber).
	return &RefinementPlan{
		ID:         id,
		Proposal:   proposal,
		TraceID:    traceID,
		RollbackOf: rollbackOf,
		Baseline:   state.Snapshot(),
	}
}
