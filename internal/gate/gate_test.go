//
// Tests for the Gate transition engine (internal/gate/gate.go).
//
// Covers:
//   - TransitionTable: matriz CanTransition (editor pode plan→approving mas
//     NÃO approving→approved; don/admin podem aprovar; specialist não executa)
//   - Create atribui G-XXXX começando em plan
//   - Move registra transição + atualiza estado; negação (papel) deixa o
//     estado inalterado
//   - Ledger devolve todas as transições em ordem
//   - Ciclo completo new → approving → approved → executed + rollback
//
// A base SQLite vive em t.TempDir() — nunca no .cosca real do projeto.

package gate

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newTestStore abre um GateStore em um diretório temporário.
func newTestStore(t *testing.T) *GateStore {
	t.Helper()
	store, err := NewGateStore(filepath.Join(t.TempDir(), "gate.db"))
	if err != nil {
		t.Fatalf("NewGateStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// =============================================================================
// TransitionTable — matriz CanTransition
// =============================================================================

func TestCanTransition_Allowed(t *testing.T) {
	allowed := []struct {
		from, to State
		role     string
	}{
		// plan → approving: editor+
		{StatePlan, StateApproving, RoleEditor},
		{StatePlan, StateApproving, RoleAdmin},
		{StatePlan, StateApproving, RoleDon},
		// approving → approved: SOMENTE don/admin (o approver)
		{StateApproving, StateApproved, RoleDon},
		{StateApproving, StateApproved, RoleAdmin},
		// approving → rejected: don/admin
		{StateApproving, StateRejected, RoleDon},
		{StateApproving, StateRejected, RoleAdmin},
		// approved → executed: SOMENTE don/admin (approver ≠ executor)
		{StateApproved, StateExecuted, RoleDon},
		{StateApproved, StateExecuted, RoleAdmin},
		// executed → rolled-back: don/admin
		{StateExecuted, StateRolledBack, RoleDon},
		{StateExecuted, StateRolledBack, RoleAdmin},
	}
	for _, c := range allowed {
		if !CanTransition(c.from, c.to, c.role) {
			t.Errorf("CanTransition(%s, %s, %q) = false, want true",
				c.from, c.to, c.role)
		}
	}
}

func TestCanTransition_Denied(t *testing.T) {
	denied := []struct {
		from, to State
		role     string
	}{
		// editor NÃO pode aprovar.
		{StatePlan, StateApproving, RoleSpecialist}, // executor não planeja/encaminha
		{StateApproving, StateApproved, RoleEditor}, // editor não aprova
		{StateApproving, StateApproved, RoleSpecialist},
		{StateApproving, StateRejected, RoleEditor}, // editor não rejeita
		// executor (specialist) NÃO executa — approver ≠ executor.
		{StateApproved, StateExecuted, RoleSpecialist},
		{StateApproved, StateExecuted, RoleEditor},
		{StateExecuted, StateRolledBack, RoleSpecialist},
		{StateExecuted, StateRolledBack, RoleEditor},
		// viewer nunca move nada.
		{StatePlan, StateApproving, "viewer"},
		{StateApproving, StateApproved, "viewer"},
		// transições inexistentes: ninguém pode pular etapas.
		{StatePlan, StateApproved, RoleDon},
		{StatePlan, StateExecuted, RoleDon},
		{StateApproving, StateExecuted, RoleDon},
		{StateApproved, StateRolledBack, RoleDon},
		{StateRejected, StateApproved, RoleDon},
	}
	for _, c := range denied {
		if CanTransition(c.from, c.to, c.role) {
			t.Errorf("CanTransition(%s, %s, %q) = true, want false",
				c.from, c.to, c.role)
		}
	}
}

func TestParseState(t *testing.T) {
	valid := map[string]State{
		"plan":        StatePlan,
		"APPROVING":   StateApproving,
		"approved":    StateApproved,
		"executed":    StateExecuted,
		"rejected":    StateRejected,
		"rolled-back": StateRolledBack,
	}
	for in, want := range valid {
		got, err := ParseState(in)
		if err != nil {
			t.Errorf("ParseState(%q): unexpected error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseState(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := ParseState("aprovado"); err == nil {
		t.Error("ParseState of an unknown state should fail")
	}
}

// =============================================================================
// Create
// =============================================================================

func TestCreate_AssignsGAndStartsAtPlan(t *testing.T) {
	store := newTestStore(t)

	id, err := store.Create("plano-deploy-v2", RoleDon)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !strings.HasPrefix(id, "G-") || len(id) != len("G-0001") {
		t.Errorf("id = %q, want G-XXXX format", id)
	}
	if id != "G-0001" {
		t.Errorf("first id = %q, want G-0001", id)
	}

	rec, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec == nil {
		t.Fatal("record not found after Create")
	}
	if rec.PlanRef != "plano-deploy-v2" {
		t.Errorf("PlanRef = %q, want plano-deploy-v2", rec.PlanRef)
	}
	if rec.State != StatePlan {
		t.Errorf("State = %q, want plan", rec.State)
	}
	if len(rec.Transitions) != 0 {
		t.Errorf("Transitions = %v, want empty on create", rec.Transitions)
	}

	// IDs sequenciais.
	id2, err := store.Create("outro-plano", RoleDon)
	if err != nil {
		t.Fatalf("Create #2: %v", err)
	}
	if id2 != "G-0002" {
		t.Errorf("second id = %q, want G-0002", id2)
	}
}

func TestCreate_RequiresPlanRef(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.Create("", RoleDon); err == nil {
		t.Error("Create with empty planRef should fail")
	}
}

// =============================================================================
// Move — ciclo completo + negação
// =============================================================================

func TestMove_FullLifecycle(t *testing.T) {
	store := newTestStore(t)
	id, err := store.Create("plano", RoleDon)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	steps := []struct {
		to   State
		by   string
		role string
	}{
		{StateApproving, "editor-ana", RoleEditor},
		{StateApproved, RoleDon, RoleDon},
		{StateExecuted, RoleDon, RoleDon},
		{StateRolledBack, RoleAdmin, RoleAdmin},
	}
	for _, s := range steps {
		if err := store.Move(id, s.to, s.by, s.role); err != nil {
			t.Fatalf("Move to %s by %s: %v", s.to, s.by, err)
		}
	}

	rec, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec.State != StateRolledBack {
		t.Errorf("State = %q, want rolled-back", rec.State)
	}
	if len(rec.Transitions) != 4 {
		t.Fatalf("Transitions = %d, want 4", len(rec.Transitions))
	}
	// A primeira transição foi feita pelo editor e registrada no ledger.
	if rec.Transitions[0].From != StatePlan || rec.Transitions[0].To != StateApproving {
		t.Errorf("transition[0] = %+v, want plan→approving", rec.Transitions[0])
	}
	if rec.Transitions[0].By != "editor-ana" {
		t.Errorf("transition[0].By = %q, want editor-ana", rec.Transitions[0].By)
	}
	if rec.Transitions[3].To != StateRolledBack || rec.Transitions[3].By != RoleAdmin {
		t.Errorf("transition[3] = %+v, want rolled-back by admin", rec.Transitions[3])
	}
}

func TestMove_InvalidRoleLeavesStateUnchanged(t *testing.T) {
	store := newTestStore(t)
	id, err := store.Create("plano", RoleDon)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Editor encaminha plan→approving (permitido).
	if err := store.Move(id, StateApproving, "editor-ana", RoleEditor); err != nil {
		t.Fatalf("plan→approving by editor: %v", err)
	}

	// Editor NÃO pode aprovar.
	err = store.Move(id, StateApproved, "editor-ana", RoleEditor)
	if err == nil {
		t.Fatal("editor approving should be denied")
	}
	if !strings.Contains(err.Error(), "papel 'editor' não pode mover approving→approved") {
		t.Errorf("denial message = %q", err.Error())
	}

	rec, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec.State != StateApproving {
		t.Errorf("State = %q, want approving (unchanged after denial)", rec.State)
	}
	if len(rec.Transitions) != 1 {
		t.Errorf("Transitions = %d, want 1 (denied move must not be recorded)", len(rec.Transitions))
	}

	// Specialist (executor) NÃO pode executar um plano aprovado.
	if err := store.Move(id, StateApproved, RoleDon, RoleDon); err != nil {
		t.Fatalf("approving→approved by don: %v", err)
	}
	err = store.Move(id, StateExecuted, "exec-bot", RoleSpecialist)
	if err == nil {
		t.Fatal("specialist executing should be denied")
	}
	if !strings.Contains(err.Error(), "papel 'specialist' não pode mover approved→executed") {
		t.Errorf("denial message = %q", err.Error())
	}
}

func TestMove_InvalidTransitionFails(t *testing.T) {
	store := newTestStore(t)
	id, err := store.Create("plano", RoleDon)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// plan → executed não existe na máquina (ninguém pode pular etapas).
	err = store.Move(id, StateExecuted, RoleDon, RoleDon)
	if err == nil {
		t.Fatal("plan→executed should be an invalid transition")
	}
	if !strings.Contains(err.Error(), "transição inválida plan→executed") {
		t.Errorf("invalid transition message = %q", err.Error())
	}

	rec, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec.State != StatePlan || len(rec.Transitions) != 0 {
		t.Errorf("state/transitions changed after invalid move: state=%q n=%d",
			rec.State, len(rec.Transitions))
	}
}

func TestMove_SameStateIsNoOp(t *testing.T) {
	store := newTestStore(t)
	id, err := store.Create("plano", RoleDon)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.Move(id, StatePlan, RoleDon, RoleDon); err != nil {
		t.Errorf("same-state move should be a no-op, got err: %v", err)
	}
	rec, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(rec.Transitions) != 0 {
		t.Errorf("same-state move must not record a transition, got %d", len(rec.Transitions))
	}
}

func TestMove_UnknownGate(t *testing.T) {
	store := newTestStore(t)
	if err := store.Move("G-9999", StateApproving, RoleDon, RoleDon); err == nil {
		t.Error("moving an unknown gate should fail")
	}
}

// =============================================================================
// Ledger — imutabilidade e ordem
// =============================================================================

func TestLedger_AllTransitionsInOrder(t *testing.T) {
	store := newTestStore(t)
	id, err := store.Create("plano", RoleDon)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := store.Move(id, StateApproving, "editor-ana", RoleEditor); err != nil {
		t.Fatalf("move 1: %v", err)
	}
	if err := store.Move(id, StateApproved, RoleDon, RoleDon); err != nil {
		t.Fatalf("move 2: %v", err)
	}
	if err := store.Move(id, StateExecuted, RoleDon, RoleDon); err != nil {
		t.Fatalf("move 3: %v", err)
	}

	ledger, err := store.Ledger(id)
	if err != nil {
		t.Fatalf("Ledger: %v", err)
	}
	if len(ledger) != 3 {
		t.Fatalf("ledger len = %d, want 3", len(ledger))
	}
	want := []State{StateApproving, StateApproved, StateExecuted}
	for i, lt := range ledger {
		if lt.To != want[i] {
			t.Errorf("ledger[%d].To = %q, want %q", i, lt.To, want[i])
		}
		if lt.At.IsZero() {
			t.Errorf("ledger[%d].At is zero", i)
		}
	}
	// Ledger é uma cópia imutável: mutar a fatia retornada não altera o store.
	ledger[0].To = StateRejected
	rec, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec.Transitions[0].To != StateApproving {
		t.Error("mutating the returned ledger leaked into the store")
	}
}

func TestLedger_UnknownGate(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.Ledger("G-4242"); err == nil {
		t.Error("ledger of an unknown gate should fail")
	}
}

// =============================================================================
// List
// =============================================================================

func TestList_NewestFirst(t *testing.T) {
	store := newTestStore(t)
	id1, err := store.Create("p1", RoleDon)
	if err != nil {
		t.Fatalf("Create 1: %v", err)
	}
	if _, err := store.Create("p2", RoleDon); err != nil {
		t.Fatalf("Create 2: %v", err)
	}
	// Move id1 para que seu updated_at seja mais recente que o de id2.
	if err := store.Move(id1, StateApproving, RoleDon, RoleDon); err != nil {
		t.Fatalf("Move: %v", err)
	}

	recs, err := store.List(10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("List len = %d, want 2", len(recs))
	}
	if recs[0].ID != id1 {
		t.Errorf("List[0].ID = %q, want %q (newest first)", recs[0].ID, id1)
	}
}

func TestList_Empty(t *testing.T) {
	store := newTestStore(t)
	recs, err := store.List(10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("List on empty store = %d, want 0", len(recs))
	}
}

// =============================================================================
// Persistência + NormalizeID
// =============================================================================

func TestStore_PersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "gate.db")

	store, err := NewGateStore(dbPath)
	if err != nil {
		t.Fatalf("NewGateStore: %v", err)
	}
	id, err := store.Create("plano", RoleDon)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.Move(id, StateApproving, "editor-ana", RoleEditor); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := NewGateStore(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = reopened.Close() }()

	rec, err := reopened.Get(id)
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if rec == nil {
		t.Fatal("record lost after reopen")
	}
	if rec.State != StateApproving || len(rec.Transitions) != 1 {
		t.Errorf("state/transitions after reopen = %q/%d, want approving/1",
			rec.State, len(rec.Transitions))
	}

	// IDs continuam sequenciais após reabrir.
	id2, err := reopened.Create("outro", RoleDon)
	if err != nil {
		t.Fatalf("Create after reopen: %v", err)
	}
	if id2 != "G-0002" {
		t.Errorf("id after reopen = %q, want G-0002", id2)
	}
}

func TestNormalizeID(t *testing.T) {
	cases := map[string]string{
		"G-0001": "G-0001",
		"g-0001": "G-0001",
		"G-1":    "G-0001",
		"0001":   "G-0001",
		"1842":   "G-1842",
	}
	for in, want := range cases {
		got, err := NormalizeID(in)
		if err != nil {
			t.Errorf("NormalizeID(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("NormalizeID(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := NormalizeID("X-0001"); err == nil {
		t.Error("NormalizeID of a non-gate id should fail")
	}
}

// =============================================================================
// fmtTime deve produzir largura fracionária FIXA (sortable) — regressão para
// o bug do RFC3339Nano (largura variável quebra ORDER BY updated_at DESC).
// =============================================================================

func TestFmtTimeSortable_FixedWidth(t *testing.T) {
	cases := []time.Time{
		time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 28, 12, 0, 0, 500000000, time.UTC),
		time.Date(2026, 8, 28, 12, 0, 1, 1, time.UTC),
	}
	for _, c := range cases {
		s := fmtTime(c)
		idx := strings.Index(s, ".")
		if idx < 0 {
			t.Fatalf("%q: sem ponto fracionário", s)
		}
		frac := s[idx+1 : idx+10]
		if len(frac) != 9 {
			t.Fatalf("fração não é fixa (esperava 9 dígitos): %q", s)
		}
	}
}

func TestFmtTimeLexicographicMatchesTime(t *testing.T) {
	before := time.Date(2026, 8, 28, 12, 0, 0, 500000000, time.UTC)
	after := time.Date(2026, 8, 28, 12, 0, 0, 500000001, time.UTC)
	sb := fmtTime(before)
	sa := fmtTime(after)
	// No código antigo (RFC3339Nano) sb=".5Z" > sa=".500000001Z" → ordena errado.
	if !(sb < sa) {
		t.Fatalf("ordem lexicográfica não segue o tempo: %q < %q = %v (bug de largura variável)", sb, sa, sb < sa)
	}
}
