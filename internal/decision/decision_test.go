package decision

//
// Testes da primitiva GENÉRICA de decisões (F1, ADR-015) — event-sourced,
// stdlib-only. Cobrem:
//   - Append + latest-winner (estado derivado do log);
//   - Supersede (estado → superseded; desconhecida → erro);
//   - Redact (estado → redacted; desconhecida → erro);
//   - ByTask (filtro por tarefa);
//   - Latest(kind) e List (ordem de criação);
//   - round-trip com um store em memória (as decisões sobrevivem à re-hidratação,
//     provando que o estado é derivado do log e não de um campo mutado).
//

import (
	"testing"
	"time"
)

// memoryStore é um DecisionStore em memória (append-only) usado nos testes —
// implementa o contrato sem importar nada de domínio, provando que a primitiva
// fala apenas com a interface. Simula um log durável idempotente.
type memoryStore struct {
	events []DecisionEvent
}

func (m *memoryStore) AppendEvent(ev DecisionEvent) error {
	for _, e := range m.events {
		if e.ID == ev.ID {
			return nil // idempotente por ID
		}
	}
	m.events = append(m.events, ev)
	return nil
}
func (m *memoryStore) Events() ([]DecisionEvent, error) {
	return append([]DecisionEvent(nil), m.events...), nil
}
func (m *memoryStore) Save(d Decision) error { return m.AppendEvent(NewAppendEvent(d)) }
func (m *memoryStore) ByTask(taskID string) ([]Decision, error) {
	all := Derive(m.events)
	out := make([]Decision, 0, len(all))
	for _, d := range all {
		if d.TaskID == taskID {
			out = append(out, d)
		}
	}
	return out, nil
}
func (m *memoryStore) List() ([]Decision, error) { return Derive(m.events), nil }

// dec monta uma decisão de teste com campos mínimos.
func dec(kind, taskID, actor string) Decision {
	return Decision{
		Kind:      kind,
		TaskID:    taskID,
		Actor:     actor,
		Rationale: "teste",
		Context:   map[string]string{"k": "v"},
		Evidence:  []string{"e1", "e2"},
		Timestamp: time.Now().UTC(),
	}
}

// TestAppendLatestWinner — Append nasce active; o estado efetivo é derivado do
// log. Uma decisão superseded/redacted reflete o último evento que a toca.
func TestAppendLatestWinner(t *testing.T) {
	l := NewDecisionLog(nil)

	id, err := l.Append(dec("start_strategy", "", "bot"))
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if id == "" {
		t.Fatal("Append não devolveu ID")
	}
	st, ok := l.Latest("start_strategy")
	if !ok {
		t.Fatal("Latest não achou a decisão")
	}
	if st.Status != StatusActive {
		t.Errorf("decisão recém-criada deveria ser active, got %q", st.Status)
	}
	if st.Rationale != "teste" || st.Context["k"] != "v" || len(st.Evidence) != 2 {
		t.Errorf("conteúdo não preservado: %+v", st)
	}

	// Supersede → o MESMO ID passa a superseded (latest-winner do log).
	if err := l.Supersede(id); err != nil {
		t.Fatalf("Supersede: %v", err)
	}
	st, _ = l.Latest("start_strategy")
	if st.Status != StatusSuperseded {
		t.Errorf("após supersede, status deveria ser superseded, got %q", st.Status)
	}

	// Append de outra decisão (ID novo) → active, não afeta a anterior.
	id2, _ := l.Append(dec("start_strategy", "", "bot"))
	if id2 == id {
		t.Fatal("decide duplicado não deveria reusar ID")
	}
	if st, _ := l.Latest("start_strategy"); st.Status != StatusActive || st.ID != id2 {
		t.Errorf("latest deveria ser a decisão nova active (id=%s), got id=%s status=%s", id2, st.ID, st.Status)
	}
}

// TestAppendDuplicateID — append de um ID já registrado → ErrDuplicateID
// (a decisão é criada uma vez; substituir se faz por nova decisão + supersede).
func TestAppendDuplicateID(t *testing.T) {
	l := NewDecisionLog(nil)
	id, _ := l.Append(dec("owm_bypass", "", "bot"))
	if _, err := l.Append(Decision{ID: id, Kind: "owm_bypass"}); err == nil {
		t.Fatalf("append com ID duplicado deveria falhar")
	}
}

// TestSupersedeUnknown — supersede de uma decisão inexistente → erro.
func TestSupersedeUnknown(t *testing.T) {
	l := NewDecisionLog(nil)
	if err := l.Supersede("nao-existe"); err == nil {
		t.Fatal("supersede de decisão desconhecida deveria falhar")
	}
}

// TestRedact — redact marca a decisão como redacted; desconhecida → erro.
func TestRedact(t *testing.T) {
	l := NewDecisionLog(nil)
	id, _ := l.Append(dec("risk_adjust", "task-1", "autopilot"))
	if err := l.Redact(id); err != nil {
		t.Fatalf("Redact: %v", err)
	}
	st, ok := l.Latest("risk_adjust")
	if !ok || st.Status != StatusRedacted {
		t.Errorf("após redact, status deveria ser redacted, got %q", st.Status)
	}
	if err := l.Redact("nao-existe"); err == nil {
		t.Fatal("redact de decisão desconhecida deveria falhar")
	}
}

// TestByTask — ByTask devolve só as decisões da tarefa pedida.
func TestByTask(t *testing.T) {
	l := NewDecisionLog(nil)
	_, _ = l.Append(dec("complete", "task-1", "autopilot"))
	_, _ = l.Append(dec("continue", "task-1", "autopilot"))
	_, _ = l.Append(dec("complete", "task-2", "autopilot"))

	t1 := l.ByTask("task-1")
	if len(t1) != 2 {
		t.Fatalf("ByTask(task-1) deveria ter 2, got %d", len(t1))
	}
	for _, d := range t1 {
		if d.TaskID != "task-1" {
			t.Errorf("ByTask vazou decisão de outra tarefa: %+v", d)
		}
	}
	if len(l.ByTask("task-2")) != 1 {
		t.Errorf("ByTask(task-2) deveria ter 1")
	}
	if len(l.ByTask("sem-task")) != 0 {
		t.Errorf("ByTask de tarefa vazia deveria ser 0")
	}
}

// TestListAndLatest — List devolve em ordem de criação; Latest devolve a mais
// recente de um kind.
func TestListAndLatest(t *testing.T) {
	l := NewDecisionLog(nil)
	_, _ = l.Append(dec("owm_bypass", "", "autopilot"))
	_, _ = l.Append(dec("start_strategy", "", "bot"))
	_, _ = l.Append(dec("owm_bypass", "", "autopilot"))

	all := l.List()
	if len(all) != 3 {
		t.Fatalf("List deveria ter 3, got %d", len(all))
	}
	// Ordem de criação preservada.
	if all[0].Kind != "owm_bypass" || all[1].Kind != "start_strategy" || all[2].Kind != "owm_bypass" {
		t.Errorf("List fora da ordem de criação: %+v", all)
	}

	latest, ok := l.Latest("owm_bypass")
	if !ok || latest.ID != all[2].ID {
		t.Errorf("Latest(owm_bypass) deveria ser a 3ª decisão (id=%s), got id=%s ok=%v", all[2].ID, latest.ID, ok)
	}
	if _, ok := l.Latest("nunca"); ok {
		t.Errorf("Latest de kind inexistente deveria ser false")
	}
}

// TestDerivePure — Derive é a função pura de derivação (latest-winner) usada
// pelos stores: de uma lista de eventos, materializa o estado efetivo.
func TestDerivePure(t *testing.T) {
	ms := &memoryStore{}
	_ = ms.Save(dec("complete", "task-1", "autopilot"))
	evs, _ := ms.Events()

	derived := Derive(evs)
	if len(derived) != 1 {
		t.Fatalf("Derive len=%d, esperava 1", len(derived))
	}
	if derived[0].Status != StatusActive {
		t.Errorf("Derive deveria marcar active, got %q", derived[0].Status)
	}

	// Após um supersede evento, o derive reflete o estado.
	_ = ms.AppendEvent(DecisionEvent{ID: newID(), DecisionID: derived[0].ID, Op: OpSupersede, Timestamp: time.Now()})
	evs, _ = ms.Events()
	derived = Derive(evs)
	if derived[0].Status != StatusSuperseded {
		t.Errorf("Derive após supersede deveria marcar superseded, got %q", derived[0].Status)
	}
}

// TestStoreRoundTripRehydrate — persistir via store e RECRIAR o log a partir do
// store re-hidrata o estado (replay): as decisões e seus status sobrevivem,
// provando que a verdade está no log (eventos), não num campo mutado.
func TestStoreRoundTripRehydrate(t *testing.T) {
	ms := &memoryStore{}

	l1 := NewDecisionLog(ms)
	id, _ := l1.Append(dec("owm_bypass", "task-9", "autopilot"))
	_, _ = l1.Append(dec("start_strategy", "", "bot"))
	_ = l1.Supersede(id)

	// Novo log sobre o MESMO store (simula um restart): o estado é re-executado
	// a partir dos eventos persistidos.
	l2 := NewDecisionLog(ms)
	if l2.Count() != 2 {
		t.Fatalf("re-hidratado deveria ter 2 decisões, got %d", l2.Count())
	}
	st, _ := l2.Latest("owm_bypass")
	if st.Status != StatusSuperseded {
		t.Errorf("replay deveria restaurar superseded, got %q", st.Status)
	}
	if st.TaskID != "task-9" {
		t.Errorf("replay deveria restaurar task_id, got %q", st.TaskID)
	}
	if len(l2.ByTask("task-9")) != 1 {
		t.Errorf("replay ByTask(task-9) deveria ter 1, got %d", len(l2.ByTask("task-9")))
	}
}

// TestDecisionCloneIsIndependent — as decisões devolvidas são cópias: mutar o
// mapa de contexto não corrompe o estado interno do log.
func TestDecisionCloneIsIndependent(t *testing.T) {
	l := NewDecisionLog(nil)
	id, _ := l.Append(dec("complete", "task-1", "autopilot"))
	d, _ := l.Latest("complete")
	d.Context["k"] = "mutado"
	d.Evidence[0] = "mutado"
	st, _ := l.Latest("complete")
	if st.Context["k"] != "v" {
		t.Errorf("mutação no Context vazou para o estado: %q", st.Context["k"])
	}
	if st.Evidence[0] != "e1" {
		t.Errorf("mutação no Evidence vazou para o estado: %q", st.Evidence[0])
	}
	task1 := l.ByTask("task-1")
	if task1[0].ID != id {
		t.Errorf("ByTask perdeu o ID: %q", task1[0].ID)
	}
}
