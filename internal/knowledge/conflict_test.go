//
// Tests for the first-class conflict entity (CONFLICT-XXX) —
// internal/knowledge/conflict.go.
//
// Covers:
//   - Add increments CONFLICT-001, CONFLICT-002, ...; Get round-trips the record
//   - List filters by status (open/resolved)
//   - Resolve sets status + ResolvedAt (and is idempotent)
//   - OpenConflictsForItem returns the open conflicts of an item
//   - NormalizeConflictID canonical forms
//   - HasOpenConflict helper (pure, does not mutate the item)
//   - StatusWithConflicts: CONFLICTING when there is an open conflict,
//     otherwise the item's own status (opt-in, no regression on the engine)
//
// Regression is covered by running the existing epistemic tests: this suite
// never mutates KnowledgeItem/PromotionEngine behavior.

package knowledge

import (
	"path/filepath"
	"testing"
	"time"
)

// newConflictStoreTest cria uma ConflictStore em um diretório temporário.
func newConflictStoreTest(t *testing.T) *ConflictStore {
	t.Helper()
	store, err := NewConflictStore(filepath.Join(t.TempDir(), "conflict.db"))
	if err != nil {
		t.Fatalf("NewConflictStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// addTestConflict insere um conflito e retorna o registro persistido.
func addTestConflict(t *testing.T, store *ConflictStore, c ConflictRecord) *ConflictRecord {
	t.Helper()
	id, err := store.Add(c)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	rec, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get %s: %v", id, err)
	}
	if rec == nil {
		t.Fatalf("Get %s: record not found", id)
	}
	return rec
}

func TestConflictStore_AddIncrementsAndGet(t *testing.T) {
	store := newConflictStoreTest(t)

	a := addTestConflict(t, store, ConflictRecord{
		ClaimA:      "K-27:E-101",
		ClaimB:      "K-27:E-203",
		ItemID:      "K-27",
		Description: "fonte A diz X, fonte B diz o contrário",
	})
	if a.ID != "CONFLICT-001" {
		t.Errorf("first conflict ID = %q, want CONFLICT-001", a.ID)
	}
	if a.Status != ConflictOpen {
		t.Errorf("default status = %q, want open", a.Status)
	}
	if a.ClaimA != "K-27:E-101" || a.ClaimB != "K-27:E-203" || a.ItemID != "K-27" {
		t.Errorf("record round-trip mismatch: %+v", a)
	}
	if a.DetectedAt.IsZero() {
		t.Error("DetectedAt should be filled when zero input")
	}

	b := addTestConflict(t, store, ConflictRecord{
		ClaimA: "K-01:F001",
		ClaimB: "K-01:red-team-A1",
		ItemID: "K-01",
	})
	if b.ID != "CONFLICT-002" {
		t.Errorf("second conflict ID = %q, want CONFLICT-002", b.ID)
	}

	// IDs nunca são reutilizados.
	c, err := store.Add(ConflictRecord{ClaimA: "x", ClaimB: "y", ItemID: "K-02"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if c != "CONFLICT-003" {
		t.Errorf("third conflict ID = %q, want CONFLICT-003", c)
	}
}

func TestConflictStore_AddValidation(t *testing.T) {
	store := newConflictStoreTest(t)

	if _, err := store.Add(ConflictRecord{ClaimA: "x", ClaimB: "y"}); err == nil {
		t.Error("Add without item_id should fail")
	}
	if _, err := store.Add(ConflictRecord{ItemID: "K-01", ClaimB: "y"}); err == nil {
		t.Error("Add without claim_a should fail")
	}
	if _, err := store.Add(ConflictRecord{ItemID: "K-01", ClaimA: "x"}); err == nil {
		t.Error("Add without claim_b should fail")
	}
	if _, err := store.Add(ConflictRecord{ItemID: "K-01", ClaimA: "x", ClaimB: "y", Status: "bogus"}); err == nil {
		t.Error("Add with invalid status should fail")
	}
}

func TestConflictStore_ListFiltersByStatus(t *testing.T) {
	store := newConflictStoreTest(t)

	addTestConflict(t, store, ConflictRecord{ClaimA: "K-01:E1", ClaimB: "K-01:E2", ItemID: "K-01"})
	addTestConflict(t, store, ConflictRecord{ClaimA: "K-02:E1", ClaimB: "K-02:E2", ItemID: "K-02"})
	if err := store.Resolve("CONFLICT-001"); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	all, err := store.List()
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if len(all) != 2 {
		t.Errorf("List() = %d records, want 2", len(all))
	}

	open, err := store.List(ConflictOpen)
	if err != nil {
		t.Fatalf("List(open): %v", err)
	}
	if len(open) != 1 || open[0].ID != "CONFLICT-002" {
		t.Errorf("List(open) = %+v, want only CONFLICT-002", open)
	}

	resolved, err := store.List(ConflictResolved)
	if err != nil {
		t.Fatalf("List(resolved): %v", err)
	}
	if len(resolved) != 1 || resolved[0].ID != "CONFLICT-001" {
		t.Errorf("List(resolved) = %+v, want only CONFLICT-001", resolved)
	}
	if resolved[0].ResolvedAt.IsZero() {
		t.Error("resolved record should carry ResolvedAt")
	}
}

func TestConflictStore_ResolveSetsStatusAndResolvedAt(t *testing.T) {
	store := newConflictStoreTest(t)

	addTestConflict(t, store, ConflictRecord{ClaimA: "K-01:E1", ClaimB: "K-01:E2", ItemID: "K-01"})

	rec, _ := store.Get("CONFLICT-001")
	if rec.Status != ConflictOpen {
		t.Fatalf("before resolve status = %q, want open", rec.Status)
	}
	if !rec.ResolvedAt.IsZero() {
		t.Fatal("before resolve ResolvedAt should be zero")
	}

	if err := store.Resolve("CONFLICT-001"); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	after, err := store.Get("CONFLICT-001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if after.Status != ConflictResolved {
		t.Errorf("after resolve status = %q, want resolved", after.Status)
	}
	if after.ResolvedAt.IsZero() {
		t.Error("after resolve ResolvedAt should be set")
	}

	// Idempotente: resolver de novo é no-op.
	before := after.ResolvedAt
	if err := store.Resolve("CONFLICT-001"); err != nil {
		t.Errorf("Resolve again should be a no-op, got error: %v", err)
	}
	again, _ := store.Get("CONFLICT-001")
	if !again.ResolvedAt.Equal(before) {
		t.Errorf("second Resolve changed ResolvedAt %v → %v", before, again.ResolvedAt)
	}

	// Conflito inexistente → erro.
	if err := store.Resolve("CONFLICT-999"); err == nil {
		t.Error("Resolve of a missing conflict should fail")
	}
}

func TestConflictStore_OpenConflictsForItem(t *testing.T) {
	store := newConflictStoreTest(t)

	addTestConflict(t, store, ConflictRecord{ClaimA: "K-01:E1", ClaimB: "K-01:E2", ItemID: "K-01"})
	addTestConflict(t, store, ConflictRecord{ClaimA: "K-01:E3", ClaimB: "K-01:E4", ItemID: "K-01"})
	addTestConflict(t, store, ConflictRecord{ClaimA: "K-02:E1", ClaimB: "K-02:E2", ItemID: "K-02"})
	if err := store.Resolve("CONFLICT-001"); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	open, err := store.OpenConflictsForItem("K-01")
	if err != nil {
		t.Fatalf("OpenConflictsForItem: %v", err)
	}
	if len(open) != 1 || open[0].ID != "CONFLICT-002" {
		t.Errorf("OpenConflictsForItem(K-01) = %+v, want only CONFLICT-002 (open)", open)
	}
	for _, c := range open {
		if c.ItemID != "K-01" {
			t.Errorf("OpenConflictsForItem(K-01) returned conflict for item %q", c.ItemID)
		}
	}

	// Item sem conflito aberto → lista vazia.
	none, err := store.OpenConflictsForItem("K-99")
	if err != nil {
		t.Fatalf("OpenConflictsForItem(K-99): %v", err)
	}
	if len(none) != 0 {
		t.Errorf("OpenConflictsForItem(K-99) = %+v, want empty", none)
	}
}

func TestNormalizeConflictID(t *testing.T) {
	valid := map[string]string{
		"CONFLICT-102": "CONFLICT-102",
		"conflict-102": "CONFLICT-102",
		"CONFLICT-7":   "CONFLICT-007",
		"102":          "CONFLICT-102",
		" 1 ":          "CONFLICT-001",
	}
	for in, want := range valid {
		got, err := NormalizeConflictID(in)
		if err != nil {
			t.Errorf("NormalizeConflictID(%q): unexpected error %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("NormalizeConflictID(%q) = %q, want %q", in, got, want)
		}
	}
	for _, in := range []string{"", "CONFLICT", "CONFLICT-X", "abc", "-1"} {
		if _, err := NormalizeConflictID(in); err == nil {
			t.Errorf("NormalizeConflictID(%q) should fail", in)
		}
	}
	if NextConflictID(102) != "CONFLICT-102" {
		t.Errorf("NextConflictID(102) = %q, want CONFLICT-102", NextConflictID(102))
	}
	if NextConflictID(7) != "CONFLICT-007" {
		t.Errorf("NextConflictID(7) = %q, want CONFLICT-007", NextConflictID(7))
	}
}

func TestHasOpenConflict(t *testing.T) {
	item := &KnowledgeItem{ID: "K-27", Status: StatusKnown}
	open := []ConflictRecord{
		{ID: "CONFLICT-001", ItemID: "K-01", Status: ConflictOpen},
		{ID: "CONFLICT-002", ItemID: "K-27", Status: ConflictOpen},
		{ID: "CONFLICT-003", ItemID: "K-27", Status: ConflictResolved},
	}

	if !item.HasOpenConflict(open) {
		t.Error("K-27 should have an open conflict (CONFLICT-002)")
	}
	if item.HasOpenConflict(open[:1]) {
		t.Error("K-27 should NOT have an open conflict when only K-01 is present")
	}

	var nilItem *KnowledgeItem
	if nilItem.HasOpenConflict(open) {
		t.Error("nil item should never have an open conflict")
	}
}

func TestStatusWithConflicts(t *testing.T) {
	openK27 := []ConflictRecord{
		{ID: "CONFLICT-001", ItemID: "K-27", Status: ConflictOpen},
	}
	noOpen := []ConflictRecord{
		{ID: "CONFLICT-002", ItemID: "K-27", Status: ConflictResolved},
		{ID: "CONFLICT-003", ItemID: "K-01", Status: ConflictOpen},
	}

	known := &KnowledgeItem{ID: "K-27", Status: StatusKnown}
	if got := StatusWithConflicts(known, openK27); got != StatusConflicting {
		t.Errorf("K-27 with open conflict = %q, want CONFLICTING", got)
	}
	if got := StatusWithConflicts(known, noOpen); got != StatusKnown {
		t.Errorf("K-27 without open conflict = %q, want KNOWN (item's own status)", got)
	}
	if got := StatusWithConflicts(known, nil); got != StatusKnown {
		t.Errorf("K-27 with no conflicts = %q, want KNOWN", got)
	}

	// Item sem status próprio → UNKNOWN quando não há conflito.
	plain := &KnowledgeItem{ID: "K-28"}
	if got := StatusWithConflicts(plain, noOpen); got != StatusUnknown {
		t.Errorf("K-28 (no status, no conflict) = %q, want UNKNOWN", got)
	}
	// Item com conflito aberto mas sem status próprio → CONFLICTING sobrepõe.
	plainK27 := &KnowledgeItem{ID: "K-27"}
	if got := StatusWithConflicts(plainK27, openK27); got != StatusConflicting {
		t.Errorf("K-27 open conflict should override empty status: got %q", got)
	}

	// nil item → UNKNOWN (defensivo).
	if got := StatusWithConflicts(nil, openK27); got != StatusUnknown {
		t.Errorf("nil item = %q, want UNKNOWN", got)
	}

	// O helper NÃO muta o item (opt-in, sem regressão).
	if known.Status != StatusKnown {
		t.Errorf("StatusWithConflicts mutated item.Status to %q, want KNOWN", known.Status)
	}
	if !known.LastVerified.IsZero() {
		t.Error("StatusWithConflicts mutated LastVerified")
	}
}

// TestStatusWithConflicts_NoEngineMutation garante que o helper não altera o
// comportamento de KnowledgeItem/PromotionEngine — itens continuam a ser
// promovidos normalmente independentemente de conflitos.
func TestStatusWithConflicts_NoEngineMutation(t *testing.T) {
	engine := NewPromotionEngine()
	if err := engine.Register(&KnowledgeItem{
		ID:    "K-50",
		Title: "Lei conflitante",
		Evidence: []Evidence{
			{ID: "e1", Kind: "test", Source: "s", Description: "d", Timestamp: time.Now()},
			{ID: "e2", Kind: "test", Source: "s", Description: "d", Timestamp: time.Now()},
			{ID: "e3", Kind: "test", Source: "s", Description: "d", Timestamp: time.Now()},
		},
		Confidence: 0.95,
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	it, _ := engine.Get("K-50")
	if it.Level != LevelLearning {
		t.Errorf("engine promoted K-50 to %q, want learning (no regression)", it.Level)
	}
	if it.Status != StatusSupported {
		t.Errorf("engine status = %q, want SUPPORTED (no regression)", it.Status)
	}

	// O conflito só aparece via o helper opt-in; o engine permanece intocado.
	open := []ConflictRecord{{ID: "CONFLICT-001", ItemID: "K-50", Status: ConflictOpen}}
	if got := StatusWithConflicts(it, open); got != StatusConflicting {
		t.Errorf("StatusWithConflicts = %q, want CONFLICTING", got)
	}
	after, _ := engine.Get("K-50")
	if after.Status != StatusSupported {
		t.Errorf("engine status after helper = %q, want SUPPORTED (helper must not write)", after.Status)
	}
}
