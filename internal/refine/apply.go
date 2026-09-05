package refine

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// AppliedEdit is the outcome of a single edit during apply. Before/After are
// carried for audit and for Rollback (P1: reconstruct the prior state).
type AppliedEdit struct {
	Edit
	Applied bool
	Error   string
	Before  *Entry
	After   *Entry
}

// ApplyResult is the outcome of a refinement pass. It carries the per-edit
// applied/before/after trail and the append-only record that was written.
type ApplyResult struct {
	ID              string
	Summary         string
	Rationale       string
	ExpectedOutcome string
	AppliedEdits    []AppliedEdit
	Record          OnlineRefinementRecord
	RollbackOf      string
}

// errNilState / errNilPlan are internal fail-fast helpers.
var (
	errNilState = errors.New("refine: nil state")
	errNilPlan  = errors.New("refine: nil plan")
)

// Apply is the P2 phase-2 (apply) of the refinement pipeline. It RE-READS the
// shared state (passed as `state`) and, for each proposed edit:
//
//  1. P3 — rejects any edit that reaches an immutable base entry (typed
//     *ImmutableBaseEditError), applied:false.
//  2. P2 — compares the live Before against the Baseline captured at plan time;
//     on divergence rejects with applied:false and the
//     "entry changed during refinement planning" reason (no-clobber). Edits
//     already applied within this same pass are exempt (chained intra-proposal
//     edits are allowed).
//  3. Otherwise applies the edit, appending the before/after to the result.
//
// A promoted pass always requires a non-empty Evidence (rationale) and a
// TraceID (I4) — fail-closed (I2): Apply refuses to run without them, so the
// audit log can never contain an unjustified, unattributed autoedit.
func Apply(state *State, plan *RefinementPlan) (*ApplyResult, error) {
	if state == nil {
		return nil, errNilState
	}
	if plan == nil {
		return nil, errNilPlan
	}
	// P1 provenance invariant (fail-closed I2/I4): no justification, no write.
	if strings.TrimSpace(plan.Proposal.Rationale) == "" {
		return nil, ErrMissingEvidence
	}
	if strings.TrimSpace(plan.TraceID) == "" {
		return nil, ErrMissingTraceID
	}
	if plan.ID == "" {
		plan.ID = NewID()
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	proposalModified := map[string]bool{}
	applied := make([]AppliedEdit, 0, len(plan.Proposal.Edits))

	for _, e := range plan.Proposal.Edits {
		id := resolvedID(e)
		edit := e
		edit.ID = id

		// P3 — immutable base guard (codified).
		if verr := state.validateEditLocked(edit); verr != nil {
			applied = append(applied, AppliedEdit{Edit: edit, Applied: false, Error: verr.Error()})
			continue
		}

		before, exists := state.getLocked(edit.Kind, id)
		key := keyOf(edit.Kind, id)

		// P2 — baseline-conflict guard (no-clobber). Skip keys already changed
		// within this pass so a plan may chain edits on one entry.
		if plan.Baseline != nil && !proposalModified[key] &&
			!reconcilable(&before, exists, plan.Baseline, edit.Kind, id) {
			applied = append(applied, AppliedEdit{
				Edit: edit, Before: entryPtr(before, exists), Applied: false,
				Error: (&EntryChangedDuringPlanError{Kind: edit.Kind, ID: id}).Error(),
			})
			continue
		}

		switch edit.Action {
		case ActionDelete:
			if !exists {
				applied = append(applied, AppliedEdit{Edit: edit, Applied: false, Error: "entry not found"})
				continue
			}
			state.deleteLocked(edit.Kind, id)
			applied = append(applied, AppliedEdit{Edit: edit, Before: &before, Applied: true})
		case ActionCreate:
			if exists {
				applied = append(applied, AppliedEdit{Edit: edit, Before: &before, Applied: false, Error: "entry already exists"})
				continue
			}
			after := newEntry(edit)
			state.setLocked(edit.Kind, id, after)
			applied = append(applied, AppliedEdit{Edit: edit, After: &after, Applied: true})
		case ActionUpdate:
			if !exists {
				applied = append(applied, AppliedEdit{Edit: edit, Applied: false, Error: "entry not found"})
				continue
			}
			after := updatedEntry(before, edit)
			state.setLocked(edit.Kind, id, after)
			applied = append(applied, AppliedEdit{Edit: edit, Before: &before, After: &after, Applied: true})
		}
		proposalModified[key] = true
	}

	record := OnlineRefinementRecord{
		ID:        plan.ID,
		Trigger:   plan.Proposal.Summary,
		Changes:   changesOf(applied),
		Evidence:  plan.Proposal.Rationale,
		Outcome:   plan.Proposal.ExpectedOutcome,
		Version:   state.recordSeqLocked(),
		CreatedAt: time.Now().UTC(),
		TraceID:   plan.TraceID,
	}
	state.appendRecordLocked(record)

	return &ApplyResult{
		ID:              plan.ID,
		Summary:         plan.Proposal.Summary,
		Rationale:       plan.Proposal.Rationale,
		ExpectedOutcome: plan.Proposal.ExpectedOutcome,
		AppliedEdits:    applied,
		Record:          record,
		RollbackOf:      plan.RollbackOf,
	}, nil
}

// Rollback reconstructs the prior state from the Before/After carried on each
// applied edit (P1), in reverse order. It rebuilds the exact prior Entry
// (content, title, version, timestamps) rather than going through a fresh
// update — a faithful reconstruction. It enforces the immutable guard (P3) so a
// rollback can never resurrect-then-mutate a base entry, and it adds an
// antistale check: if the live entry no longer matches the After we recorded,
// a concurrent writer touched it, so the undo is refused (no-clobber).
func Rollback(state *State, outcome *ApplyResult, traceID string) (*ApplyResult, error) {
	if state == nil {
		return nil, errNilState
	}
	if outcome == nil {
		return nil, errors.New("refine: rollback requires a non-nil outcome")
	}
	if strings.TrimSpace(traceID) == "" {
		return nil, ErrMissingTraceID
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	applied := make([]AppliedEdit, 0, len(outcome.AppliedEdits))
	for i := len(outcome.AppliedEdits) - 1; i >= 0; i-- {
		ae := outcome.AppliedEdits[i]
		if !ae.Applied {
			continue
		}
		id := ae.ID

		// P3 — a rollback must not touch an immutable base either.
		if state.isImmutableLocked(ae.Kind, id) {
			applied = append(applied, AppliedEdit{Edit: ae.Edit, Applied: false,
				Error: (&ImmutableBaseEditError{Kind: ae.Kind, ID: id,
					Reason: "rollback targets an immutable base"}).Error()})
			continue
		}

		switch {
		case ae.Before != nil && ae.After != nil:
			// update was applied → restore the exact prior entry.
			cur, ok := state.getLocked(ae.Kind, id)
			if !ok || !reflect.DeepEqual(cur, *ae.After) {
				applied = append(applied, AppliedEdit{Edit: ae.Edit, Before: ae.Before, Applied: false,
					Error: (&EntryChangedDuringPlanError{Kind: ae.Kind, ID: id}).Error()})
				continue
			}
			restored := cloneEntry(*ae.Before)
			state.setLocked(ae.Kind, id, restored)
			applied = append(applied, AppliedEdit{Edit: ae.Edit, Before: ae.After, After: &restored, Applied: true})
		case ae.Before != nil && ae.After == nil:
			// delete was applied → recreate the prior entry.
			if _, ok := state.getLocked(ae.Kind, id); ok {
				applied = append(applied, AppliedEdit{Edit: ae.Edit, Before: ae.Before, Applied: false,
					Error: (&EntryChangedDuringPlanError{Kind: ae.Kind, ID: id}).Error()})
				continue
			}
			restored := cloneEntry(*ae.Before)
			state.setLocked(ae.Kind, id, restored)
			applied = append(applied, AppliedEdit{Edit: ae.Edit, After: &restored, Applied: true})
		case ae.Before == nil && ae.After != nil:
			// create was applied → delete it.
			cur, ok := state.getLocked(ae.Kind, id)
			if !ok || !reflect.DeepEqual(cur, *ae.After) {
				applied = append(applied, AppliedEdit{Edit: ae.Edit, Before: ae.After, Applied: false,
					Error: (&EntryChangedDuringPlanError{Kind: ae.Kind, ID: id}).Error()})
				continue
			}
			state.deleteLocked(ae.Kind, id)
			applied = append(applied, AppliedEdit{Edit: ae.Edit, Before: ae.After, Applied: true})
		}
	}

	rec := OnlineRefinementRecord{
		ID:        NewID(),
		Trigger:   "rollback " + outcome.ID,
		Changes:   changesOf(applied),
		Evidence:  "restores shared state from refinement " + outcome.ID,
		Outcome:   "faulty refinement edits reverted",
		Version:   state.recordSeqLocked(),
		CreatedAt: time.Now().UTC(),
		TraceID:   traceID,
	}
	state.appendRecordLocked(rec)

	return &ApplyResult{
		ID:              rec.ID,
		Summary:         "rollback " + outcome.ID,
		Rationale:       "restores shared state from refinement " + outcome.ID,
		ExpectedOutcome: "faulty refinement edits reverted",
		AppliedEdits:    applied,
		Record:          rec,
		RollbackOf:      outcome.ID,
	}, nil
}

// reconcilable reports whether the live entry (live/ok) still matches the
// baseline captured at plan time. Absent↔absent is compatible; any presence
// mismatch (someone created or deleted during planning) or content/version
// divergence is a stale write (P2).
func reconcilable(live *Entry, liveOK bool, base StateSnapshot, kind Kind, id string) bool {
	baseEntry, baseOK := base.Get(kind, id)
	if liveOK != baseOK {
		return false // one side exists and the other did not at plan time
	}
	if !liveOK {
		return true // both absent
	}
	return reflect.DeepEqual(*live, baseEntry)
}

func newEntry(e Edit) Entry {
	now := time.Now().UTC()
	id := resolvedID(e)
	return Entry{
		ID: id, Kind: e.Kind, Title: e.Title, Content: e.Content,
		Version: 1, CreatedAt: now, UpdatedAt: now,
	}
}

func updatedEntry(before Entry, e Edit) Entry {
	after := cloneEntry(before)
	if e.Title != "" {
		after.Title = e.Title
	}
	if e.Content != "" {
		after.Content = e.Content
	}
	after.Version = before.Version + 1
	after.UpdatedAt = time.Now().UTC()
	return after
}

func entryPtr(e Entry, ok bool) *Entry {
	if !ok {
		return nil
	}
	e = cloneEntry(e)
	return &e
}

func changesOf(applied []AppliedEdit) []string {
	var out []string
	for _, ae := range applied {
		if !ae.Applied {
			continue
		}
		out = append(out, fmt.Sprintf("%s %s:%s", ae.Action, ae.Kind, ae.ID))
	}
	return out
}

// NewID mints a refinement id in the canonical `refine_<unixnano>` format.
func NewID() string {
	return fmt.Sprintf("refine_%d", time.Now().UTC().UnixNano())
}
