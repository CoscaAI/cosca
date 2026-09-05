//
// Tests for internal/knowledge/thinking.go — "How to think": classificação
// determinística de afirmações (FACT/EVIDENCE/INFERENCE/ASSUMPTION/
// HYPOTHESIS/UNKNOWN) + ClaimStore (SQLite .cosca/claims.db).
//
// Covers:
//   - ClaimKind.Valid / Description
//   - Classify matrix: todas as combinações (evidence, validated, derived,
//     tested) → tipo esperado, cobrindo a ordem de precedência
//     (FACT > EVIDENCE > INFERENCE > HYPOTHESIS > ASSUMPTION > UNKNOWN)
//   - Bandeira "assumida explicitamente" (ExplicitlyAssumed) que sobrepõe
//   - Afirmação vazia → UNKNOWN
//   - ClassifyWithReason (razão pt-BR de cada decisão)
//   - IsTrustworthy: FACT true; EVIDENCE >= 0.7 true; demais false
//   - ClaimStore Add/Get/List (normalização de ID, round-trip, ordem)
//   - ClassifyAndAdd persiste
//   - Validação do Add (kind/statement/confidence)
//   - NextClaimID / NormalizeClaimID
//

package knowledge

import (
	"path/filepath"
	"testing"
)

// =============================================================================
// ClaimKind
// =============================================================================

func TestClaimKind_Valid(t *testing.T) {
	for _, k := range []ClaimKind{
		ClaimFact, ClaimEvidence, ClaimInference,
		ClaimAssumption, ClaimHypothesis, ClaimUnknown,
	} {
		if !k.Valid() {
			t.Errorf("%s should be valid", k)
		}
	}
	if (ClaimKind("BOGUS")).Valid() {
		t.Error("BOGUS should not be valid")
	}
	if (ClaimKind("")).Valid() {
		t.Error("empty kind should not be valid")
	}
}

func TestClaimKind_Description(t *testing.T) {
	want := map[ClaimKind]string{
		ClaimFact:       "estabelecido com evidência validada",
		ClaimEvidence:   "observação registrada",
		ClaimInference:  "derivada de outra afirmação",
		ClaimAssumption: "assumida sem evidência",
		ClaimHypothesis: "não confirmada, testável",
		ClaimUnknown:    "sem base",
	}
	for k, d := range want {
		if got := k.Description(); got != d {
			t.Errorf("%s.Description() = %q, want %q", k, got, d)
		}
	}
	if got := (ClaimKind("X")).Description(); got == "" {
		t.Error("unknown kind description should not be empty")
	}
}

// =============================================================================
// Classify — matriz determinística (precedência)
// =============================================================================

func TestClassify_Matrix(t *testing.T) {
	tests := []struct {
		name      string
		evidence  int
		validated bool
		derived   bool
		tested    bool
		want      ClaimKind
	}{
		// Sem evidência.
		{"sem sinal", 0, false, false, false, ClaimAssumption},
		{"testada", 0, false, false, true, ClaimHypothesis},
		{"derivada", 0, false, true, false, ClaimInference},
		{"derivada + testada", 0, false, true, true, ClaimInference},
		{"validada sem evidência", 0, true, false, false, ClaimAssumption},
		{"validada sem evidência + testada", 0, true, false, true, ClaimHypothesis},
		{"validada sem evidência + derivada", 0, true, true, false, ClaimInference},
		{"validada sem evidência + derivada + testada", 0, true, true, true, ClaimInference},
		// Com evidência (não validada): EVIDENCE > INFERENCE > HYPOTHESIS.
		{"evidência", 1, false, false, false, ClaimEvidence},
		{"evidência + testada", 1, false, false, true, ClaimEvidence},
		{"evidência + derivada", 1, false, true, false, ClaimEvidence},
		{"evidência + derivada + testada", 1, false, true, true, ClaimEvidence},
		// Com evidência validada: FACT > tudo.
		{"fato", 1, true, false, false, ClaimFact},
		{"fato + testada", 1, true, false, true, ClaimFact},
		{"fato + derivada", 1, true, true, false, ClaimFact},
		{"fato + derivada + testada", 1, true, true, true, ClaimFact},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := Classify("afirmação", tt.evidence, tt.validated, tt.derived, tt.tested)
			if got != tt.want {
				t.Errorf("Classify(%d,%v,%v,%v) = %s, want %s",
					tt.evidence, tt.validated, tt.derived, tt.tested, got, tt.want)
			}
		})
	}
}

func TestClassify_ExplicitAssumptionOverridesEverything(t *testing.T) {
	c := &Classifier{ExplicitlyAssumed: true}
	// --assumed sobrepõe até FACT (evidência validada).
	if got := c.Classify("afirmação", 5, true, false, false); got != ClaimAssumption {
		t.Errorf("assumed+fact = %s, want ASSUMPTION", got)
	}
	// E também o default sem evidência.
	if got := c.Classify("afirmação", 0, false, false, false); got != ClaimAssumption {
		t.Errorf("assumed default = %s, want ASSUMPTION", got)
	}
	// Sem a bandeira, a mesma entrada é FACT.
	if got := (&Classifier{}).Classify("afirmação", 5, true, false, false); got != ClaimFact {
		t.Errorf("without assumed, want FACT, got %s", got)
	}
}

func TestClassify_EmptyStatementIsUnknown(t *testing.T) {
	if got := Classify("", 0, false, false, false); got != ClaimUnknown {
		t.Errorf("empty = %s, want UNKNOWN", got)
	}
	if got := Classify("   ", 0, false, false, false); got != ClaimUnknown {
		t.Errorf("blank = %s, want UNKNOWN", got)
	}
}

func TestClassifyWithReason(t *testing.T) {
	tests := []struct {
		name      string
		statement string
		evidence  int
		validated bool
		derived   bool
		tested    bool
		wantKind  ClaimKind
	}{
		{"fato", "afirmação", 2, true, false, false, ClaimFact},
		{"evidência", "afirmação", 1, false, false, false, ClaimEvidence},
		{"inferência", "afirmação", 0, false, true, false, ClaimInference},
		{"hipótese", "afirmação", 0, false, false, true, ClaimHypothesis},
		{"assunção", "afirmação", 0, false, false, false, ClaimAssumption},
		{"desconhecida", "", 0, false, false, false, ClaimUnknown},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			kind, reason := (&Classifier{}).ClassifyWithReason(
				tt.statement, tt.evidence, tt.validated, tt.derived, tt.tested)
			if kind != tt.wantKind {
				t.Errorf("kind = %s, want %s", kind, tt.wantKind)
			}
			if reason == "" {
				t.Error("reason should not be empty")
			}
		})
	}

	kind, reason := (&Classifier{ExplicitlyAssumed: true}).ClassifyWithReason("afirmação", 5, true, false, false)
	if kind != ClaimAssumption || reason == "" {
		t.Errorf("assumed override: kind = %s, reason = %q", kind, reason)
	}
}

// =============================================================================
// IsTrustworthy
// =============================================================================

func TestClaimRecord_IsTrustworthy(t *testing.T) {
	tests := []struct {
		name string
		rec  ClaimRecord
		want bool
	}{
		{"fato", ClaimRecord{Kind: ClaimFact}, true},
		{"evidência forte", ClaimRecord{Kind: ClaimEvidence, Confidence: 0.7}, true},
		{"evidência acima do limite", ClaimRecord{Kind: ClaimEvidence, Confidence: 0.95}, true},
		{"evidência fraca", ClaimRecord{Kind: ClaimEvidence, Confidence: 0.6}, false},
		{"evidência zero", ClaimRecord{Kind: ClaimEvidence}, false},
		{"inferência", ClaimRecord{Kind: ClaimInference, Confidence: 0.9}, false},
		{"assunção", ClaimRecord{Kind: ClaimAssumption, Confidence: 0.9}, false},
		{"hipótese", ClaimRecord{Kind: ClaimHypothesis, Confidence: 0.9}, false},
		{"desconhecida", ClaimRecord{Kind: ClaimUnknown}, false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rec.IsTrustworthy(); got != tt.want {
				t.Errorf("IsTrustworthy() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// ClaimStore
// =============================================================================

func newTestClaimStore(t *testing.T) *ClaimStore {
	t.Helper()
	store, err := NewClaimStore(filepath.Join(t.TempDir(), "claims.db"))
	if err != nil {
		t.Fatalf("NewClaimStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestClaimStore_AddGetList(t *testing.T) {
	store := newTestClaimStore(t)

	id1, err := store.Add(ClaimRecord{Kind: ClaimFact, Statement: "Fato 1", Confidence: 0.9})
	if err != nil {
		t.Fatalf("Add 1: %v", err)
	}
	if id1 != "CL-0001" {
		t.Errorf("first id = %q, want CL-0001", id1)
	}

	id2, err := store.Add(ClaimRecord{
		Kind:       ClaimAssumption,
		Statement:  "Suposição",
		Supports:   []string{"K-01", "CL-0001"},
		Confidence: 0,
	})
	if err != nil {
		t.Fatalf("Add 2: %v", err)
	}
	if id2 != "CL-0002" {
		t.Errorf("second id = %q, want CL-0002", id2)
	}

	// Get pela forma canônica.
	rec, err := store.Get(id1)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec == nil {
		t.Fatal("Get returned nil")
	}
	if rec.Kind != ClaimFact || rec.Statement != "Fato 1" || rec.Confidence != 0.9 {
		t.Errorf("unexpected record: %+v", rec)
	}
	if rec.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}

	// Get pela forma normalizada (lowercase, sem padding).
	rec2, err := store.Get("cl-2")
	if err != nil {
		t.Fatalf("Get normalized: %v", err)
	}
	if rec2 == nil || rec2.ID != "CL-0002" {
		t.Fatalf("normalized get: got %+v", rec2)
	}
	if len(rec2.Supports) != 2 || rec2.Supports[0] != "K-01" || rec2.Supports[1] != "CL-0001" {
		t.Errorf("supports not round-tripped: %+v", rec2.Supports)
	}

	// Get de inexistente → nil, nil.
	rec3, err := store.Get("CL-9999")
	if err != nil {
		t.Fatalf("Get missing: %v", err)
	}
	if rec3 != nil {
		t.Errorf("expected nil for missing, got %+v", rec3)
	}

	// List: created_at DESC, id DESC → CL-0002 primeiro.
	records, err := store.List(10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("len(List) = %d, want 2", len(records))
	}
	if records[0].ID != "CL-0002" {
		t.Errorf("List order: first = %q, want CL-0002", records[0].ID)
	}
}

func TestClaimStore_ListLimit(t *testing.T) {
	store := newTestClaimStore(t)
	for i := 0; i < 5; i++ {
		if _, err := store.Add(ClaimRecord{Kind: ClaimEvidence, Statement: "obs"}); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	records, err := store.List(2)
	if err != nil {
		t.Fatalf("List(2): %v", err)
	}
	if len(records) != 2 {
		t.Errorf("len(List(2)) = %d, want 2", len(records))
	}

	// Limit <= 0 assume 20; > 1000 é truncado.
	if records, err = store.List(0); err != nil {
		t.Fatalf("List(0): %v", err)
	}
	if len(records) != 5 {
		t.Errorf("len(List(0)) = %d, want 5 (default 20)", len(records))
	}
	if records, err = store.List(5000); err != nil {
		t.Fatalf("List(5000): %v", err)
	}
	if len(records) != 5 {
		t.Errorf("len(List(5000)) = %d, want 5 (capped at 1000)", len(records))
	}
}

func TestClaimStore_ClassifyAndAddPersists(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "claims.db")
	store, err := NewClaimStore(dbPath)
	if err != nil {
		t.Fatalf("NewClaimStore: %v", err)
	}

	rec, err := store.ClassifyAndAdd("A API X rejeita tokens JWT", 2, true, false, false, 0.95)
	if err != nil {
		t.Fatalf("ClassifyAndAdd: %v", err)
	}
	if rec.Kind != ClaimFact {
		t.Errorf("kind = %s, want FACT", rec.Kind)
	}
	if rec.ID == "" {
		t.Error("ID should be assigned")
	}
	if rec.Confidence != 0.95 {
		t.Errorf("confidence = %v, want 0.95", rec.Confidence)
	}

	// Persistência: reabre a mesma base e encontra o registro.
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	store2, err := NewClaimStore(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer store2.Close()

	got, err := store2.Get(rec.ID)
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if got == nil {
		t.Fatal("claim not persisted")
	}
	if got.Kind != ClaimFact || got.Statement != "A API X rejeita tokens JWT" {
		t.Errorf("unexpected persisted record: %+v", got)
	}
}

func TestClaimStore_AddValidation(t *testing.T) {
	store := newTestClaimStore(t)

	if _, err := store.Add(ClaimRecord{Kind: "BOGUS", Statement: "x"}); err == nil {
		t.Error("expected error for invalid kind")
	}
	if _, err := store.Add(ClaimRecord{Kind: ClaimFact, Statement: "  "}); err == nil {
		t.Error("expected error for empty statement")
	}
	if _, err := store.Add(ClaimRecord{Kind: ClaimFact, Statement: "x", Confidence: 1.5}); err == nil {
		t.Error("expected error for out-of-range confidence")
	}
	if _, err := store.Add(ClaimRecord{Kind: ClaimFact, Statement: "x", Confidence: -0.1}); err == nil {
		t.Error("expected error for negative confidence")
	}
}

// =============================================================================
// IDs
// =============================================================================

func TestNextAndNormalizeClaimID(t *testing.T) {
	if got := NextClaimID(7); got != "CL-0007" {
		t.Errorf("NextClaimID(7) = %q, want CL-0007", got)
	}
	if got := NextClaimID(1024); got != "CL-1024" {
		t.Errorf("NextClaimID(1024) = %q, want CL-1024", got)
	}

	for _, in := range []string{"CL-0007", "cl-7", "CL-07", "CL-0007", "7"} {
		got, err := NormalizeClaimID(in)
		if err != nil {
			t.Errorf("NormalizeClaimID(%q): %v", in, err)
			continue
		}
		if got != "CL-0007" {
			t.Errorf("NormalizeClaimID(%q) = %q, want CL-0007", in, got)
		}
	}
	if _, err := NormalizeClaimID("abc"); err == nil {
		t.Error("expected error for non-numeric ID")
	}
	if _, err := NormalizeClaimID("-3"); err == nil {
		t.Error("expected error for negative ID")
	}
}
