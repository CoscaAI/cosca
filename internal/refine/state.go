package refine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// State is the shared, mutable harness/agent state that the refinement machine
// edits. Entries are keyed by Kind→ID; Records is the append-only audit log of
// every autoedit (P1). Immutable base entries (constitution / base system
// prompt / a skill's Genome.Identity) are tracked explicitly and REJECTED by
// the refinement boundary (P3) — never by convention.
//
// State is safe for concurrent applicators within a single process (RWMutex).
// Cross-process safety (the P2 antistale-write guard) is provided at the
// plan/apply boundary by the Baseline snapshot, not by holding this lock across
// processes.
type State struct {
	mu        sync.RWMutex
	entries   map[Kind]map[string]Entry
	immutable map[string]bool    // "kind:id" → true for base entries
	records   []OnlineRefinementRecord
	seq       int // append-only journal sequence (Version of the next record)
}

// NewState returns an empty refinement state.
func NewState() *State {
	return &State{
		entries:   map[Kind]map[string]Entry{},
		immutable: map[string]bool{},
		records:   []OnlineRefinementRecord{},
	}
}

// Set registers (or overwrites) an entry. It is the BOOTSTRAP primitive — it
// does not enforce immutability (the refinement boundary does); use it to seed
// the state, then MarkImmutable to pin a base entry.
func (s *State) Set(kind Kind, id string, e Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setLocked(kind, id, e)
}

// Delete removes an entry outright (bootstrap primitive, no immutability check).
func (s *State) Delete(kind Kind, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleteLocked(kind, id)
}

// Get reads an entry (snapshot copy).
func (s *State) Get(kind Kind, id string) (Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getLocked(kind, id)
}

// Entries returns a copy of all entries of a kind.
func (s *State) Entries(kind Kind) map[string]Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := map[string]Entry{}
	for id, e := range s.entries[kind] {
		out[id] = cloneEntry(e)
	}
	return out
}

// MarkImmutable pins an entry as a base that the refinement boundary may never
// create/update/delete (P3). It is the codified guard: after this, any edit
// targeting "kind:id" is rejected in code.
func (s *State) MarkImmutable(kind Kind, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.immutable[keyOf(kind, id)] = true
}

// Immutable reports whether "kind:id" is pinned as a base entry.
func (s *State) Immutable(kind Kind, id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isImmutableLocked(kind, id)
}

// Records returns the append-only audit log (deep copy).
func (s *State) Records() []OnlineRefinementRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]OnlineRefinementRecord, len(s.records))
	copy(out, s.records)
	return out
}

// Snapshot captures a deep copy of the entries at a point in time. It is the
// Baseline captured at plan time (P2): apply compares the live entries against
// it and rejects stale writes.
func (s *State) Snapshot() StateSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := StateSnapshot{}
	for k, m := range s.entries {
		mk := make(map[string]Entry, len(m))
		for id, e := range m {
			mk[id] = cloneEntry(e)
		}
		out[k] = mk
	}
	return out
}

// ValidateEdit is the public P3 + P2 structural guard. It returns a typed
// *ImmutableBaseEditError when the edit targets a base entry, or a descriptive
// error for an unsupported action/kind or a malformed edit.
func (s *State) ValidateEdit(edit Edit) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.validateEditLocked(edit)
}

// validateEditLocked assumes the caller holds at least a read lock.
func (s *State) validateEditLocked(edit Edit) error {
	if !edit.Action.Valid() {
		return fmt.Errorf("refine: unsupported action %q", edit.Action)
	}
	if !edit.Kind.Valid() {
		return fmt.Errorf("refine: unsupported kind %q", edit.Kind)
	}
	id := resolvedID(edit)

	// P3 — immutable base guard (codified, not by convention). Applies to
	// create/update/delete alike: a base entry can never be created-then-
	// deleted, updated, or otherwise touched by the refinement boundary.
	if s.isImmutableLocked(edit.Kind, id) {
		return &ImmutableBaseEditError{
			Kind:   edit.Kind,
			ID:     id,
			Reason: "entry is an immutable base (constitution / base system prompt / Genome.Identity)",
		}
	}
	if edit.Action != ActionCreate && id == "" {
		return fmt.Errorf("refine: %s requires id", edit.Action)
	}
	if edit.Action != ActionDelete && edit.Content == "" {
		return fmt.Errorf("refine: %s requires content", edit.Action)
	}
	return nil
}

// =============================================================================
// Internal locked helpers (caller holds s.mu)
// =============================================================================

func (s *State) setLocked(kind Kind, id string, e Entry) {
	if s.entries[kind] == nil {
		s.entries[kind] = map[string]Entry{}
	}
	e.ID = id
	e.Kind = kind
	s.entries[kind][id] = cloneEntry(e)
}

func (s *State) deleteLocked(kind Kind, id string) {
	if m := s.entries[kind]; m != nil {
		delete(m, id)
	}
}

func (s *State) getLocked(kind Kind, id string) (Entry, bool) {
	m := s.entries[kind]
	if m == nil {
		return Entry{}, false
	}
	e, ok := m[id]
	if !ok {
		return Entry{}, false
	}
	return cloneEntry(e), true
}

func (s *State) isImmutableLocked(kind Kind, id string) bool {
	return s.immutable[keyOf(kind, id)]
}

func (s *State) appendRecordLocked(rec OnlineRefinementRecord) {
	s.records = append(s.records, rec)
	s.seq++
}

func (s *State) recordSeqLocked() int {
	return s.seq + 1
}

// =============================================================================
// StateSnapshot — the Baseline captured at plan time (P2)
// =============================================================================

// StateSnapshot is a point-in-time deep copy of the entries: kind → id → Entry.
// It is the Baseline that applyRefinement compares against to detect stale
// writes (no-clobber).
type StateSnapshot map[Kind]map[string]Entry

// Get reads an entry from the snapshot.
func (sn StateSnapshot) Get(kind Kind, id string) (Entry, bool) {
	m := sn[kind]
	if m == nil {
		return Entry{}, false
	}
	e, ok := m[id]
	return e, ok
}

// =============================================================================
// Journal — append-only record store (P1 persistence)
// =============================================================================

// Journal is an append-only, file-backed log of OnlineRefinementRecords. It
// mirrors the discipline of internal/evolution.StageRollStore: each record is a
// JSON object on its own line, appended under lock, so the log is never
// overwritten — audit-safe. An empty path keeps the journal volatile (memory-
// only). For durable, tamper-evident, cross-process CAS of the ENTRIES, the
// internal/ledger (P1–P8) is the natural backing store; the journal is the
// typed, append-only companion for the record trail.
type Journal struct {
	mu   sync.Mutex
	path string
}

// NewJournal creates a journal at path (empty path = volatile).
func NewJournal(path string) *Journal {
	return &Journal{path: path}
}

// Append appends a record to the journal (append-only). Volatile journals no-op.
func (j *Journal) Append(rec OnlineRefinementRecord) error {
	if j == nil || j.path == "" {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(j.path), 0o755); err != nil {
		return fmt.Errorf("refine: journal mkdir: %w", err)
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("refine: journal marshal: %w", err)
	}
	f, err := os.OpenFile(j.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("refine: journal open: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("refine: journal write: %w", err)
	}
	return nil
}

// Load returns all records in append order. A missing/volatile journal returns
// nil. Corrupted lines are skipped (never fatal).
func (j *Journal) Load() ([]OnlineRefinementRecord, error) {
	if j == nil || j.path == "" {
		return nil, nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	data, err := os.ReadFile(j.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("refine: journal load: %w", err)
	}
	var out []OnlineRefinementRecord
	for _, line := range splitLines(string(data)) {
		if line == "" {
			continue
		}
		var rec OnlineRefinementRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue // linha corrompida: ignora, não derruba
		}
		out = append(out, rec)
	}
	return out, nil
}

func cloneEntry(e Entry) Entry {
	if e.Metadata != nil {
		m := make(map[string]string, len(e.Metadata))
		for k, v := range e.Metadata {
			m[k] = v
		}
		e.Metadata = m
	}
	return e
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
