// Package refine implements the ONLINE self-editing (refinement) protocol for
// the COSCA harness/agent shared state — the incremental, auditable autoedit
// that complements the OFFLINE evolution (internal/evolution ProofGate/Stage/
// Checkpoint being projection+proof) and the GEPA mutation loop
// (internal/skilleval Genome/guardrail/promote).
//
// It maps three patterns proven by the prime-agent reference onto the Cosca
// internals. All three are deterministic (I1) and take ZERO LLM on the critical
// path: the plan may be produced by an auxiliary/non-deterministic source, but
// decision + application are pure functions over the shared state.
//
//	P1  OnlineRefinementRecord — an append-only audit record per autoedit:
//	    {id, trigger, changes, evidence, outcome, version, created_at}.
//	    Evidence (the rationale) is the key field, Outcome is optional, and a
//	    PROMOTED (applied) autoedit always carries a non-empty Evidence plus a
//	    TraceID (I4 provenance). Rollback reconstructs the prior state from the
//	    Before/After carried on each applied edit.
//
//	P2  plan→apply separation + baseline-conflict guard. planRefinement
//	    PROPOSES edits (from any auxiliary source) and captures a Baseline
//	    snapshot of the shared state; applyRefinement RE-READS the shared state
//	    and, per edit, compares the live Before against the Baseline; if they
//	    diverged during the planning window the edit is REJECTED
//	    (applied:false, "entry changed during refinement planning"). This is the
//	    antistale-write guard, cross-process, default-no-clobber.
//
//	P3  Hard immutability guard at the refinement boundary. validateEdit
//	    rejects IN CODE any edit touching an immutable base entry (constitution /
//	    base system prompt / a skill's Genome.Identity), with applied:false and a
//	    typed reason — never by convention.
//
// Fail-closed (I2): a promoted autoedit without Evidence/TraceID, or an edit
// that hits an immutable/conflicting entry, is rejected and never promoted.
package refine

import (
	"fmt"
	"strings"
	"time"
	"unicode"
)

// BasePromptID is the canonical id of the base system prompt — the immutable
// constitution entry that no refinement may ever touch (P3). It mirrors the
// prime-agent `base_system_prompt` guard.
const BasePromptID = "base_system_prompt"

// Action is the operation a refinement edit performs.
type Action string

const (
	// ActionCreate mints a new entry.
	ActionCreate Action = "create"
	// ActionUpdate mutates an existing entry.
	ActionUpdate Action = "update"
	// ActionDelete removes an existing entry.
	ActionDelete Action = "delete"
)

// Valid reports whether a is a supported action.
func (a Action) Valid() bool {
	switch a {
	case ActionCreate, ActionUpdate, ActionDelete:
		return true
	}
	return false
}

// Kind classifies the refinement target (harness entry, memory, skill, ...).
type Kind string

const (
	KindPrompt   Kind = "prompt"
	KindMemory   Kind = "memory"
	KindSkill    Kind = "skill"
	KindSubagent Kind = "subagent"
)

// Valid reports whether k is a supported kind.
func (k Kind) Valid() bool {
	switch k {
	case KindPrompt, KindMemory, KindSkill, KindSubagent:
		return true
	}
	return false
}

// Entry is a refinement target record in the shared harness/agent state. It is
// the unit the plan proposes to create/update/delete and the unit the apply
// guard checks for stale writes. Entries are immutable once written: mutations
// produce a NEW Entry with an incremented Version (never an in-place edit).
type Entry struct {
	ID        string            `json:"id"`
	Kind      Kind              `json:"kind"`
	Title     string            `json:"title"`
	Content   string            `json:"content"`
	Version   int               `json:"version"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Edit is a single proposed change. Reason is the per-edit rationale (used in
// audit/diagnostics, never a substitute for the proposal Evidence).
type Edit struct {
	Action  Action `json:"action"`
	Kind    Kind   `json:"kind"`
	ID      string `json:"id,omitempty"`
	Title   string `json:"title,omitempty"`
	Content string `json:"content,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// Proposal is a batch of edits plus the evidence that justifies it. Rationale
// becomes the record's Evidence (P1 key field); ExpectedOutcome becomes its
// Outcome (optional).
type Proposal struct {
	Summary         string `json:"summary"`
	Rationale       string `json:"rationale"`
	Edits           []Edit `json:"edits"`
	ExpectedOutcome string `json:"expected_outcome,omitempty"`
}

// OnlineRefinementRecord is the append-only audit record of a single autoedit
// (P1). A promoted (applied) record is guaranteed by Apply to carry a
// non-empty Evidence and a non-empty TraceID.
type OnlineRefinementRecord struct {
	ID        string    `json:"id"`
	Trigger   string    `json:"trigger"`
	Changes   []string  `json:"changes"`
	Evidence  string    `json:"evidence"`
	Outcome   string    `json:"outcome,omitempty"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	TraceID   string    `json:"trace_id"`
}

// =============================================================================
// Typed errors
// =============================================================================

// EntryChangedDuringPlanError is the P2 antistale-write rejection: the live
// entry diverged from the baseline captured at plan time because a concurrent
// writer (another session/process) mutated the shared state during the
// refinement window. The edit is dropped, not clobbered.
type EntryChangedDuringPlanError struct {
	Kind Kind
	ID   string
}

func (e *EntryChangedDuringPlanError) Error() string {
	return fmt.Sprintf("entry changed during refinement planning: %s:%s", e.Kind, e.ID)
}

// ImmutableBaseEditError is the P3 rejection: an edit tried to touch an
// immutable base entry (constitution / base system prompt / Genome.Identity).
// Codified in validateEdit, never by convention.
type ImmutableBaseEditError struct {
	Kind   Kind
	ID     string
	Reason string
}

func (e *ImmutableBaseEditError) Error() string {
	return fmt.Sprintf("immutable base edit rejected: %s:%s (%s)", e.Kind, e.ID, e.Reason)
}

// Sentinel fail-closed errors for the P1 provenance invariant (I2/I4).
var (
	// ErrMissingEvidence is returned by Apply when a proposed autoedit carries
	// no Evidence (rationale). A promoted autoedit without justification is
	// never written.
	ErrMissingEvidence = fmt.Errorf("refine: autoedit requires a non-empty Evidence (rationale)")
	// ErrMissingTraceID is returned by Apply when a proposed autoedit carries no
	// TraceID (I4 provenance). Every autoedit must be attributable.
	ErrMissingTraceID = fmt.Errorf("refine: autoedit requires a TraceID (I4 provenance)")
)

// resolvedID computes the effective entry id for an edit: an explicit ID, or —
// for a create — a slug derived from the title (mirrors the prime-agent
// computedId). Update/delete require an explicit ID.
func resolvedID(e Edit) string {
	if e.ID != "" {
		return e.ID
	}
	if e.Action == ActionCreate {
		return Slug(e.Title, e.Kind)
	}
	return ""
}

// keyOf builds the canonical "kind:id" map key used for immutable tracking and
// the proposal-modified set.
func keyOf(k Kind, id string) string {
	return string(k) + ":" + id
}

// Slug derives a stable, filesystem-safe id from a title (lowercase, runs of
// non-alphanumerics become a single dash). Falls back to the kind when the
// title yields nothing.
func Slug(title string, kind Kind) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		s = strings.ToLower(string(kind))
	}
	return s
}
