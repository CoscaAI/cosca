// Tests for the universal Trace ID + structured event ledger (internal/trace).
//
// Covers:
//   - NewID format (prefix TRACE- + date + 4 hex), uniqueness (100 no collision)
//   - Parse valid/invalid
//   - Store Append/Get (ordered)/Latest; append-only (no Update/Delete exposed)
//   - DetectDivergence: same sequence → none; divergence at position 2 → [2];
//     new action mid-sequence flagged; end-extension → not flagged (normal)

package trace

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// =============================================================================
// NewID / Parse
// =============================================================================

func TestNewID_Format(t *testing.T) {
	re := regexp.MustCompile(`^TRACE-\d{8}-[0-9A-F]{8}$`)
	for i := 0; i < 50; i++ {
		id := NewID()
		if !re.MatchString(id.String()) {
			t.Fatalf("NewID() format inválido: %q", id)
		}
		// A data deve ser hoje, no fuso UTC.
		datePart := strings.TrimPrefix(id.String(), "TRACE-")[:8]
		if len(datePart) != 8 {
			t.Fatalf("NewID() data inesperada: %q", id)
		}
	}
}

func TestNewID_Uniqueness(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id := NewID().String()
		if seen[id] {
			t.Fatalf("NewID() colidiu: %s", id)
		}
		seen[id] = true
	}
}

func TestParse_Valid(t *testing.T) {
	for _, s := range []string{"TRACE-20260802-7F92A1B3", "trace-20260802-7f92a1b3", "  TRACE-20260802-A1B2C3D4  "} {
		id, ok := Parse(s)
		if !ok {
			t.Fatalf("Parse(%q) deveria ser válido", s)
		}
		// Normaliza para maiúsculo no formato canônico.
		want := strings.ToUpper(strings.TrimSpace(s))
		if id.String() != want {
			t.Fatalf("Parse(%q) = %q, esperava %q", s, id, want)
		}
	}
}

func TestParse_Invalid(t *testing.T) {
	for _, s := range []string{
		"", "TRACE-", "TRACE-20260802", "TRACE-20260802-", "TRACE-20260802-ZZZ1ZZZZ",
		"TRACE-2026080-7F92A1B3", "TRACE-202608023-7F92A1B3", "TRACE-20260802-7F9",
		"TRACE-20260802-7F922", "X-TRACE-20260802-7F92A1B3", "7F92A1B3", "trace-20260802-7f92a1b3-extra",
	} {
		if _, ok := Parse(s); ok {
			t.Fatalf("Parse(%q) deveria ser inválido", s)
		}
	}
}

// =============================================================================
// Store — Append/Get/Latest, append-only
// =============================================================================

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(filepath.Join(t.TempDir(), "trace.db"))
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestStore_AppendAndGetOrdered(t *testing.T) {
	s := newTestStore(t)
	id := NewID().String()

	// Registra fora de ordem de timestamp para provar que Get ordena por tempo.
	evs := []Event{
		{TraceID: id, Actor: "kernel", Action: "TASK_STARTED", Timestamp: 100, Result: "running"},
		{TraceID: id, Actor: "agent-x", Action: "TEST_FAILED", Timestamp: 300, Result: "failed"},
		{TraceID: id, Actor: "kernel", Action: "PLAN_CREATED", Timestamp: 200, Result: "success"},
	}
	for _, e := range evs {
		if err := s.Append(e); err != nil {
			t.Fatalf("Append(%v): %v", e, err)
		}
	}

	got, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get(): %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("Get() len = %d, esperava 3", len(got))
	}
	// Ordenado por timestamp: TASK_STARTED(100) → PLAN_CREATED(200) → TEST_FAILED(300),
	// independente da ordem de inserção.
	if got[0].Timestamp != 100 || got[1].Timestamp != 200 || got[2].Timestamp != 300 {
		t.Fatalf("Get() fora de ordem por timestamp: %+v", got)
	}
	if got[0].Action != "TASK_STARTED" || got[1].Action != "PLAN_CREATED" || got[2].Action != "TEST_FAILED" {
		t.Fatalf("Get() com ações em ordem errada: %+v", got)
	}
	// Defaults aplicados no Append.
	for _, g := range got {
		if g.Environment == "" {
			t.Errorf("Environment deveria ter default preenchido: %+v", g)
		}
		if g.TraceID != id {
			t.Errorf("TraceID divergente: %q", g.TraceID)
		}
	}
}

func TestStore_Append_InvalidTraceID(t *testing.T) {
	s := newTestStore(t)
	if err := s.Append(Event{TraceID: "not-a-trace", Action: "X"}); err == nil {
		t.Fatal("Append com Trace ID inválido deveria falhar")
	}
}

func TestStore_Get_UnknownTrace_ReturnsEmpty(t *testing.T) {
	s := newTestStore(t)
	got, err := s.Get(NewID().String())
	if err != nil {
		t.Fatalf("Get(): %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Get() de trace desconhecido deveria ser vazio, got %d", len(got))
	}
}

func TestStore_Latest(t *testing.T) {
	s := newTestStore(t)
	idA := NewID().String()
	idB := NewID().String()

	// B mais recente (timestamp maior).
	_ = s.Append(Event{TraceID: idA, Actor: "kernel", Action: "PLAN_CREATED", Timestamp: 100})
	_ = s.Append(Event{TraceID: idB, Actor: "don", Action: "APPROVED", Timestamp: 400})
	_ = s.Append(Event{TraceID: idA, Actor: "agent-x", Action: "TASK_STARTED", Timestamp: 200})

	all, err := s.Latest(0)
	if err != nil {
		t.Fatalf("Latest(): %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("Latest() len = %d, esperava 3", len(all))
	}
	// DESC por timestamp.
	if all[0].Action != "APPROVED" || all[1].Action != "TASK_STARTED" || all[2].Action != "PLAN_CREATED" {
		t.Fatalf("Latest() fora de ordem DESC: %+v", all)
	}

	two, err := s.Latest(2)
	if err != nil {
		t.Fatalf("Latest(2): %v", err)
	}
	if len(two) != 2 || two[0].Action != "APPROVED" || two[1].Action != "TASK_STARTED" {
		t.Fatalf("Latest(2) inesperado: %+v", two)
	}
}

func TestStore_AppendOnly(t *testing.T) {
	s := newTestStore(t)

	// O contrato append-only é garantido pela API: os únicos métodos de escrita
	// expostos são Append. Não deve existir Update nem Delete.
	if _, ok := any(s).(interface {
		Update(Event) error
	}); ok {
		t.Fatal("Store não deveria expor Update")
	}
	if _, ok := any(s).(interface {
		Delete(string) error
	}); ok {
		t.Fatal("Store não deveria expor Delete")
	}
}

func TestStore_ReopenExisting(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "trace.db")

	s1, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("primeiro NewStore(): %v", err)
	}
	id := NewID().String()
	if err := s1.Append(Event{TraceID: id, Actor: "kernel", Action: "TASK_STARTED"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	_ = s1.Close()

	// Reabre a mesma base — os eventos persistidos devem continuar lá.
	s2, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("segundo NewStore(): %v", err)
	}
	defer s2.Close()
	got, err := s2.Get(id)
	if err != nil {
		t.Fatalf("Get() após reabertura: %v", err)
	}
	if len(got) != 1 || got[0].Action != "TASK_STARTED" {
		t.Fatalf("eventos perdidos após reabertura: %+v", got)
	}
}

// =============================================================================
// Sequence / DetectDivergence
// =============================================================================

func TestSequenceFromEvents(t *testing.T) {
	id := NewID().String()
	events := []Event{
		{TraceID: id, Action: "PLAN_CREATED", Timestamp: 100},
		{TraceID: id, Action: "", Timestamp: 200}, // sem action — ignorado
		{TraceID: id, Action: "TASK_STARTED", Timestamp: 300},
	}
	seq := SequenceFromEvents(events)
	if len(seq.Actions) != 2 || seq.Actions[0] != "PLAN_CREATED" || seq.Actions[1] != "TASK_STARTED" {
		t.Fatalf("SequenceFromEvents() inesperado: %+v", seq.Actions)
	}
}

func TestDetectDivergence_None(t *testing.T) {
	prev := []Sequence{{Actions: []string{"A", "B", "C", "D"}}}
	cur := Sequence{Actions: []string{"A", "B", "C", "D"}}
	if div := DetectDivergence(prev, cur); len(div) != 0 {
		t.Fatalf("sequências iguais não deveriam divergir: %v", div)
	}
}

func TestDetectDivergence_NoPrevious(t *testing.T) {
	if div := DetectDivergence(nil, Sequence{Actions: []string{"A", "B"}}); len(div) != 0 {
		t.Fatalf("sem histórico não deveria haver divergência: %v", div)
	}
}

func TestDetectDivergence_AtPosition2(t *testing.T) {
	prev := []Sequence{{Actions: []string{"A", "B", "C", "D"}}}
	cur := Sequence{Actions: []string{"A", "B", "X", "D"}}
	div := DetectDivergence(prev, cur)
	if len(div) != 1 || div[0] != 2 {
		t.Fatalf("divergência na posição 2 esperada, got %v", div)
	}
}

func TestDetectDivergence_NewActionMidSequence(t *testing.T) {
	// Inserção inesperada no meio da sequência: "X" na posição 2.
	// A heurística posicional sinaliza o ponto da inserção (posição 2) e, pelo
	// deslocamento, qualquer posição seguinte cuja ação não bate mais com o
	// histórico na mesma posição — aqui a posição 3 também difere (C vs D).
	prev := []Sequence{{Actions: []string{"A", "B", "C", "D"}}}
	cur := Sequence{Actions: []string{"A", "B", "X", "C", "D"}}
	div := DetectDivergence(prev, cur)
	if len(div) == 0 {
		t.Fatalf("ação nova no meio deveria divergir")
	}
	if div[0] != 2 {
		t.Fatalf("ponto da inserção (posição 2) deveria ser o primeiro marcado, got %v", div)
	}
}

func TestDetectDivergence_ExtensionAtEnd_NotFlagged(t *testing.T) {
	prev := []Sequence{{Actions: []string{"A", "B", "C"}}}
	cur := Sequence{Actions: []string{"A", "B", "C", "D", "E"}}
	if div := DetectDivergence(prev, cur); len(div) != 0 {
		t.Fatalf("extensão ao final é normal (prefix-extension), got %v", div)
	}
}

func TestDetectDivergence_ShorterCurrent(t *testing.T) {
	prev := []Sequence{{Actions: []string{"A", "B", "C", "D"}}}
	cur := Sequence{Actions: []string{"A", "B"}}
	if div := DetectDivergence(prev, cur); len(div) != 0 {
		t.Fatalf("execução mais curta (prefixo) não deveria divergir, got %v", div)
	}
}

func TestDetectDivergence_MultiplePrevious(t *testing.T) {
	// Duas execuções anteriores: na posição 2 fizeram C e C; na posição 3 D e E.
	prev := []Sequence{
		{Actions: []string{"A", "B", "C", "D"}},
		{Actions: []string{"A", "B", "C", "E"}},
	}
	// "B" diverge na posição 2 (nenhuma anterior fez B lá).
	cur := Sequence{Actions: []string{"A", "B", "X", "D"}}
	div := DetectDivergence(prev, cur)
	if len(div) != 1 || div[0] != 2 {
		t.Fatalf("divergência na posição 2 esperada com múltiplos anteriores, got %v", div)
	}

	// "C" na posição 2 é aceitável — aparece em ambas as anteriores.
	ok := Sequence{Actions: []string{"A", "B", "C", "D"}}
	if div := DetectDivergence(prev, ok); len(div) != 0 {
		t.Fatalf("sequência que coincide com uma anterior não deveria divergir, got %v", div)
	}
}

// =============================================================================
// Concorrência leve no Append (thread-safe)
// =============================================================================

func TestStore_ConcurrentAppend(t *testing.T) {
	s := newTestStore(t)
	id := NewID().String()

	const n = 50
	done := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			done <- s.Append(Event{
				TraceID: id,
				Actor:   "agent-x",
				Action:  fmt.Sprintf("STEP_%d", i),
				Details: fmt.Sprintf("passo %d", i),
			})
		}(i)
	}
	for i := 0; i < n; i++ {
		if err := <-done; err != nil {
			t.Fatalf("Append concorrente: %v", err)
		}
	}

	got, err := s.Get(id)
	if err != nil {
		t.Fatalf("Get(): %v", err)
	}
	if len(got) != n {
		t.Fatalf("Get() len = %d, esperava %d", len(got), n)
	}
}
