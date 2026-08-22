package audit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/audit"
)

// newDecisionStoreTest abre um DecisionStore temporário para testes.
func newDecisionStoreTest(t *testing.T) *audit.DecisionStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "audit.db")
	store, err := audit.NewDecisionStore(dbPath)
	if err != nil {
		t.Fatalf("NewDecisionStore() error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// TestDecisionStore_ReopenExisting verifica que a base pode ser reaberta sem
// perder a tabela decision_log (auto-migração idempotente).
func TestDecisionStore_ReopenExisting(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "audit.db")

	s1, err := audit.NewDecisionStore(dbPath)
	if err != nil {
		t.Fatalf("first NewDecisionStore() error: %v", err)
	}
	s1.Close()

	s2, err := audit.NewDecisionStore(dbPath)
	if err != nil {
		t.Fatalf("second NewDecisionStore() error: %v", err)
	}
	defer func() { _ = s2.Close() }()
}

// TestDecisionStore_SharesAuditDB verifica que o DecisionStore usa a mesma
// base do audit.Store (.cosca/audit.db) sem conflitar com a tabela audit_logs.
func TestDecisionStore_SharesAuditDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "audit.db")

	auditStore, err := audit.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}
	defer func() { _ = auditStore.Close() }()

	decStore, err := audit.NewDecisionStore(dbPath)
	if err != nil {
		t.Fatalf("NewDecisionStore() error: %v", err)
	}
	defer func() { _ = decStore.Close() }()

	// Audit continua funcionando normalmente com a tabela nova presente.
	if err := auditStore.Record(&audit.AuditEntry{Action: "test", Status: "success"}); err != nil {
		t.Fatalf("audit Record() after decision store: %v", err)
	}

	// A trilha de decisão também funciona na mesma base.
	id, err := decStore.Record(audit.DecisionRecord{Input: "x", Result: "passed", Status: "approved"})
	if err != nil {
		t.Fatalf("decision Record() after audit store: %v", err)
	}
	if id != "D-0001" {
		t.Errorf("decision id = %q, want D-0001", id)
	}
}

// TestDecisionRecord_AssignsIncrementingIDs verifica que Record atribui IDs
// incrementais D-0001, D-0002, ...
func TestDecisionRecord_AssignsIncrementingIDs(t *testing.T) {
	store := newDecisionStoreTest(t)

	id1, err := store.Record(audit.DecisionRecord{Input: "primeira", Result: "passed", Status: "approved"})
	if err != nil {
		t.Fatalf("Record #1 error: %v", err)
	}
	id2, err := store.Record(audit.DecisionRecord{Input: "segunda", Result: "failed", Status: "approved"})
	if err != nil {
		t.Fatalf("Record #2 error: %v", err)
	}

	if id1 != "D-0001" {
		t.Errorf("id1 = %q, want D-0001", id1)
	}
	if id2 != "D-0002" {
		t.Errorf("id2 = %q, want D-0002", id2)
	}
}

// TestDecisionRecord_IDAndTimestampPreserved verifica que ID/Timestamp
// explicitamente definidos são preservados.
func TestDecisionRecord_IDAndTimestampPreserved(t *testing.T) {
	store := newDecisionStoreTest(t)

	id, err := store.Record(audit.DecisionRecord{
		DecisionID: "D-1842",
		Input:      "x",
		Timestamp:  1712345678,
		Status:     "executed",
	})
	if err != nil {
		t.Fatalf("Record() error: %v", err)
	}
	if id != "D-1842" {
		t.Errorf("id = %q, want D-1842", id)
	}

	got, err := store.Get("D-1842")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.Timestamp != 1712345678 {
		t.Errorf("timestamp = %d, want 1712345678", got.Timestamp)
	}
}

// TestDecisionRecord_Defaults verifica que Status/Timestamp ganham defaults.
func TestDecisionRecord_Defaults(t *testing.T) {
	store := newDecisionStoreTest(t)

	id, err := store.Record(audit.DecisionRecord{Input: "x"})
	if err != nil {
		t.Fatalf("Record() error: %v", err)
	}

	got, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got.Status != "executed" {
		t.Errorf("status = %q, want executed", got.Status)
	}
	if got.Timestamp == 0 {
		t.Error("timestamp should default to now")
	}
}

// TestDecisionGet_ReturnsFullRecord verifica que Get devolve o registro
// completo, incluindo as listas de knowledge/laws/evidence.
func TestDecisionGet_ReturnsFullRecord(t *testing.T) {
	store := newDecisionStoreTest(t)

	id, err := store.Record(audit.DecisionRecord{
		Input:         "Aplicar a Lei P9 no workflow de deploy",
		KnowledgeUsed: []string{"K-18", "K-91"},
		LawsApplied:   []string{"L-07", "L-13"},
		Evidence:      []string{"E-182", "E-201"},
		Provider:      "ollama",
		Model:         "llama3.1",
		Approval:      "Don / Gate 0",
		Result:        "passed",
		Rollback:      "git revert bdaef4c",
		Status:        "approved",
	})
	if err != nil {
		t.Fatalf("Record() error: %v", err)
	}

	got, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got == nil {
		t.Fatal("Get returned nil")
	}

	if got.DecisionID != id {
		t.Errorf("DecisionID = %q, want %q", got.DecisionID, id)
	}
	if got.Input != "Aplicar a Lei P9 no workflow de deploy" {
		t.Errorf("Input = %q", got.Input)
	}
	if len(got.KnowledgeUsed) != 2 || got.KnowledgeUsed[0] != "K-18" || got.KnowledgeUsed[1] != "K-91" {
		t.Errorf("KnowledgeUsed = %v, want [K-18 K-91]", got.KnowledgeUsed)
	}
	if len(got.LawsApplied) != 2 || got.LawsApplied[0] != "L-07" || got.LawsApplied[1] != "L-13" {
		t.Errorf("LawsApplied = %v, want [L-07 L-13]", got.LawsApplied)
	}
	if len(got.Evidence) != 2 || got.Evidence[0] != "E-182" || got.Evidence[1] != "E-201" {
		t.Errorf("Evidence = %v, want [E-182 E-201]", got.Evidence)
	}
	if got.Provider != "ollama" || got.Model != "llama3.1" {
		t.Errorf("Provider/Model = %q/%q", got.Provider, got.Model)
	}
	if got.Approval != "Don / Gate 0" {
		t.Errorf("Approval = %q, want 'Don / Gate 0'", got.Approval)
	}
	if got.Result != "passed" || got.Status != "approved" {
		t.Errorf("Result/Status = %q/%q", got.Result, got.Status)
	}
	if got.Rollback != "git revert bdaef4c" {
		t.Errorf("Rollback = %q", got.Rollback)
	}
}

// TestDecisionGet_NotFound verifica que Get devolve nil para ID inexistente.
func TestDecisionGet_NotFound(t *testing.T) {
	store := newDecisionStoreTest(t)

	got, err := store.Get("D-9999")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for non-existent id, got %+v", got)
	}
}

// TestDecisionGet_NormalizesInputID verifica que Get aceita formas
// não-canônicas ("d-2", "0002") e normaliza para D-0002.
func TestDecisionGet_NormalizesInputID(t *testing.T) {
	store := newDecisionStoreTest(t)

	if _, err := store.Record(audit.DecisionRecord{Input: "a", Status: "executed"}); err != nil {
		t.Fatalf("Record #1 error: %v", err)
	}
	if _, err := store.Record(audit.DecisionRecord{Input: "b", Status: "executed"}); err != nil {
		t.Fatalf("Record #2 error: %v", err)
	}

	for _, variant := range []string{"d-0002", "D-2", "0002"} {
		got, err := store.Get(variant)
		if err != nil {
			t.Fatalf("Get(%q) error: %v", variant, err)
		}
		if got == nil || got.DecisionID != "D-0002" {
			t.Errorf("Get(%q) = %v, want D-0002", variant, got)
		}
	}

	if _, err := store.Get("D-abc"); err == nil {
		t.Error("expected error for invalid id D-abc")
	}
}

// TestDecisionList_ReturnsRecentOrdered verifica que List devolve as decisões
// mais recentes primeiro (timestamp DESC).
func TestDecisionList_ReturnsRecentOrdered(t *testing.T) {
	store := newDecisionStoreTest(t)

	// timestamps crescentes: 1 = mais antiga, 3 = mais recente.
	for i := 1; i <= 3; i++ {
		_, err := store.Record(audit.DecisionRecord{
			Input:     "decisão-" + string(rune('A'+i-1)),
			Timestamp: int64(i),
			Status:    "executed",
		})
		if err != nil {
			t.Fatalf("Record() error: %v", err)
		}
	}

	records, err := store.List(10)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("len = %d, want 3", len(records))
	}
	if records[0].Timestamp != 3 || records[1].Timestamp != 2 || records[2].Timestamp != 1 {
		t.Errorf("List order = %v, want [3 2 1]", []int64{records[0].Timestamp, records[1].Timestamp, records[2].Timestamp})
	}
}

// TestDecisionList_Limit verifica o limite da listagem.
func TestDecisionList_Limit(t *testing.T) {
	store := newDecisionStoreTest(t)

	for i := 1; i <= 5; i++ {
		_, err := store.Record(audit.DecisionRecord{Input: "x", Status: "executed"})
		if err != nil {
			t.Fatalf("Record() error: %v", err)
		}
	}

	records, err := store.List(2)
	if err != nil {
		t.Fatalf("List(2) error: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("List(2) len = %d, want 2", len(records))
	}

	// limit 0 → default 20.
	records, err = store.List(0)
	if err != nil {
		t.Fatalf("List(0) error: %v", err)
	}
	if len(records) != 5 {
		t.Errorf("List(0) len = %d, want 5 (all)", len(records))
	}
}

// TestDecisionList_Empty verifica que List devolve lista vazia (não nil).
func TestDecisionList_Empty(t *testing.T) {
	store := newDecisionStoreTest(t)

	records, err := store.List(10)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if records == nil {
		t.Fatal("List() returned nil slice")
	}
	if len(records) != 0 {
		t.Errorf("len = %d, want 0", len(records))
	}
}

// TestDecisionCount verifica o contador total de decisões.
func TestDecisionCount(t *testing.T) {
	store := newDecisionStoreTest(t)

	total, err := store.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	if total != 0 {
		t.Errorf("Count = %d, want 0", total)
	}

	for i := 0; i < 4; i++ {
		_, _ = store.Record(audit.DecisionRecord{Input: "x", Status: "executed"})
	}
	total, err = store.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	if total != 4 {
		t.Errorf("Count = %d, want 4", total)
	}
}

// TestNextDecisionID_Format verifica o formato zero-padded (largura 4).
func TestNextDecisionID_Format(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{1, "D-0001"},
		{7, "D-0007"},
		{42, "D-0042"},
		{1842, "D-1842"},
		{10000, "D-10000"},
	}
	for _, c := range cases {
		if got := audit.NextDecisionID(c.n); got != c.want {
			t.Errorf("NextDecisionID(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

// TestDecisionStore_NewDecisionStoreFailsOnBadPath verifica que um caminho
// inválido retorna erro (sem pânico) — usado pelo best-effort do approve.
func TestDecisionStore_NewDecisionStoreFailsOnBadPath(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	bad := filepath.Join(blocker, "sub", "audit.db")

	store, err := audit.NewDecisionStore(bad)
	if err == nil {
		_ = store.Close()
		t.Fatalf("expected error for bad path %q", bad)
	}
}
