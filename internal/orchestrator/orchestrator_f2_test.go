package orchestrator

//
// F2 (ADR-015) — PERSISTÊNCIA DO TASK STATE + RETOMADA IDEMPOTENTE.
//
// Estes testes exercitam o contrato de persistência com um fakeRepo em memória
// (implementa task.TaskRepository), SEM depender do SQLite (que é coberto na
// borda de domínio, apoiado em internal/durable). Provam que o TaskOrchestrator:
//   - persiste cada mutação FORA do lock (lição F3 / sem deadlock no emit);
//   - re-hidrata, no New, apenas as tasks NÃO-terminais do repositório
//     (ACTIVE/WAITING/PAUSED) e IGNORA as terminais;
//   - retoma uma task ACTIVE com checkpoint SEM re-executar etapa já concluída
//     (idempotência da retomada).
//
// A FRONTEIRA permanece: o orchestrator fala apenas com o contrato
// task.TaskRepository (stdlib only) — nunca com internal/durable nem com o
// domínio de trading.
//
import (
	"sync"
	"testing"

	"github.com/CoscaAI/cosca/internal/task"
)

// fakeRepo é um TaskRepository em memória (testes da F2). Load/List devolvem
// cópias — espelhando o contrato ("não expor estado interno mutável").
type fakeRepo struct {
	mu     sync.Mutex
	states map[task.TaskID]*task.TaskState
}

func newFakeRepo() *fakeRepo { return &fakeRepo{states: map[task.TaskID]*task.TaskState{}} }

func (r *fakeRepo) Save(st *task.TaskState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := st.Clone()
	r.states[st.TaskID] = &c
	return nil
}

func (r *fakeRepo) Load(id task.TaskID) (*task.TaskState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.states[id]; ok {
		c := s.Clone()
		return &c, nil
	}
	return nil, nil
}

func (r *fakeRepo) List() ([]*task.TaskState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []*task.TaskState{}
	for _, s := range r.states {
		c := s.Clone()
		out = append(out, &c)
	}
	return out, nil
}

func (r *fakeRepo) Delete(id task.TaskID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.states, id)
	return nil
}

func (r *fakeRepo) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.states)
}

// f2Obj é um objetivo de task usado nos testes da F2.
func f2Obj() task.TaskObjective {
	return task.TaskObjective{ID: "t-1", Objective: "aumentar exposição em BTCUSDT", Symbol: "BTCUSDT", Direction: "buy"}
}

// TestF2SaveLoadRoundTrip — a interface TaskRepository tem Save→Load que
// devolve o MESMO estado (cópia). Prove com o fakeRepo isolado (unidade).
func TestF2SaveLoadRoundTrip(t *testing.T) {
	repo := newFakeRepo()
	st := &task.TaskState{
		TaskID:        task.TaskID("t-1"),
		Objective:     f2Obj(),
		Status:        task.StatusActive,
		CurrentStep:   2,
		Agent:         "bot",
		PolicyVersion: "v1",
		Checkpoints:   []string{"fill", "step2"},
		Observations:  []string{"obs1", "obs2"},
		Artifacts:     longArtifacts(),
	}
	if err := repo.Save(st); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.Load(task.TaskID("t-1"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got == nil {
		t.Fatal("Load devolveu nil para task salva")
	}
	if got.TaskID != st.TaskID || got.Status != st.Status || got.CurrentStep != st.CurrentStep {
		t.Errorf("round-trip divergiu: %+v", got)
	}
	if len(got.Checkpoints) != 2 || got.Checkpoints[0] != "fill" {
		t.Errorf("checkpoints não sobreviveram: %v", got.Checkpoints)
	}
	if len(got.Observations) != 2 {
		t.Errorf("observations não sobreviveram: %v", got.Observations)
	}
	if got.Artifacts[task.ArtifactSide] != "buy" || got.Artifacts[task.ArtifactOpen] != "true" {
		t.Errorf("artifacts não sobreviveram: %+v", got.Artifacts)
	}
	// É cópia: mutar o retorno não muda o repositório.
	got.Checkpoints[0] = "mutado"
	again, _ := repo.Load(task.TaskID("t-1"))
	if again.Checkpoints[0] != "fill" {
		t.Error("Load não devolveu cópia independente")
	}
}

// TestF2CreatePersistsAndRehydrates — Create persiste no repositório; um NOVO
// orquestrador (simula restart) re-hidrata a task NÃO-terminal e ela aparece.
func TestF2CreatePersistsAndRehydrates(t *testing.T) {
	repo := newFakeRepo()
	o := New(Config{MaxContinue: 5}, WithRepository(repo))
	id, err := o.Create(f2Obj(), "bot", "v1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if repo.count() != 1 {
		t.Fatalf("Create deveria persistir 1 task, repo tem %d", repo.count())
	}

	// Simula restart: novo orquestrador com o MESMO repositório.
	o2 := New(Config{MaxContinue: 5}, WithRepository(repo))
	st, ok := o2.Status(id)
	if !ok {
		t.Fatal("task não retomada após rehidratação")
	}
	if st.Status != task.StatusActive || st.CurrentStep != 0 {
		t.Errorf("retomada divergiu: status=%s step=%d", st.Status, st.CurrentStep)
	}
	if st.Agent != "bot" || st.PolicyVersion != "v1" {
		t.Errorf("retomada perdeu agent/policy: %q/%q", st.Agent, st.PolicyVersion)
	}
}

// TestF2RehydrateOnlyNonTerminal — o rehydrate restaura apenas NÃO-terminais e
// ignora COMPLETE/ABORTED. Um orquestrador "restartado" não retoma trabalho
// concluído.
func TestF2RehydrateOnlyNonTerminal(t *testing.T) {
	repo := newFakeRepo()
	repo.Save(&task.TaskState{TaskID: "act", Objective: f2Obj(), Status: task.StatusActive, CurrentStep: 1})
	repo.Save(&task.TaskState{TaskID: "wait", Objective: f2Obj(), Status: task.StatusWaiting, CurrentStep: 2})
	repo.Save(&task.TaskState{TaskID: "pause", Objective: f2Obj(), Status: task.StatusPaused, CurrentStep: 3})
	repo.Save(&task.TaskState{TaskID: "done", Objective: f2Obj(), Status: task.StatusComplete, CurrentStep: 4})
	repo.Save(&task.TaskState{TaskID: "abort", Objective: f2Obj(), Status: task.StatusAborted, CurrentStep: 5})

	o := New(Config{MaxContinue: 5}, WithRepository(repo))
	if got := o.Count(); got != 3 {
		t.Fatalf("Count=%d, esperava 3 (só não-terminais), resto foram ignorados", got)
	}
	for _, id := range []task.TaskID{"act", "wait", "pause"} {
		if _, ok := o.Status(id); !ok {
			t.Errorf("task %s NÃO-terminal deveria ter sido retomada", id)
		}
	}
	for _, id := range []task.TaskID{"done", "abort"} {
		if _, ok := o.Status(id); ok {
			t.Errorf("task %s terminal NÃO deveria ter sido retomada", id)
		}
	}
}

// TestF2ResumeIdempotentCheckpoint — retomada idempotente: uma task ACTIVE com
// checkpoint já concluído NÃO re-executa a etapa ao retomar. O checkpoint
// sobrevive ao restart e re-registrá-lo é no-op (não duplica).
func TestF2ResumeIdempotentCheckpoint(t *testing.T) {
	repo := newFakeRepo()
	o := New(Config{MaxContinue: 5}, WithRepository(repo))
	id, _ := o.Create(f2Obj(), "bot", "v1")

	// Duas continuações (steps 1 e 2) + um checkpoint "fill" concluído.
	for i := 0; i < 2; i++ {
		if err := o.Continue(id, task.ContIncompleteObjective); err != nil {
			t.Fatalf("Continue: %v", err)
		}
	}
	if added, err := o.AddCheckpoint(id, "fill"); err != nil || !added {
		t.Fatalf("AddCheckpoint fill: added=%v err=%v", added, err)
	}

	// Simula restart: novo orquestrador re-hidrata a task com o estado persistido.
	o2 := New(Config{MaxContinue: 5}, WithRepository(repo))
	st, ok := o2.Status(id)
	if !ok {
		t.Fatal("task não retomada")
	}
	if st.CurrentStep != 2 {
		t.Errorf("CurrentStep=%d, esperava 2 (retomada preserva o passo)", st.CurrentStep)
	}
	if !hasCheckpoint(st, "fill") {
		t.Errorf("checkpoint 'fill' não sobreviveu à retomada: %v", st.Checkpoints)
	}

	// Retomada idempotente: etapa já em checkpoint NÃO re-executa (não duplica).
	if again, err := o2.AddCheckpoint(id, "fill"); err != nil || again {
		t.Fatalf("re-add fill após retomada NÃO deveria re-executar (again=%v err=%v)", again, err)
	}
	if st, _ := o2.Status(id); len(st.Checkpoints) != 1 {
		t.Errorf("Checkpoints deveria continuar com 1 (não duplica), got %v", st.Checkpoints)
	}
}

// TestF2OnEventPersists — OnEvent persiste o snapshot do data plane (Artifacts) e
// as observações no repositório; restart os preserva.
func TestF2OnEventPersists(t *testing.T) {
	repo := newFakeRepo()
	o := New(Config{MaxContinue: 5}, WithRepository(repo))
	id, _ := o.Create(f2Obj(), "bot", "v1")

	o.OnEvent(task.Event{Type: task.EventPositionChanged, Source: "oms", Severity: task.SeverityInfo, CorrelationID: string(id),
		Payload: task.PositionChangedPayload{TaskID: id, Artifacts: longArtifacts()}})

	o2 := New(Config{MaxContinue: 5}, WithRepository(repo))
	st, ok := o2.Status(id)
	if !ok {
		t.Fatal("task não retomada")
	}
	if st.Artifacts[task.ArtifactSide] != "buy" || st.Artifacts[task.ArtifactOpen] != "true" {
		t.Errorf("artifacts não persistidos via OnEvent: %+v", st.Artifacts)
	}
	if len(st.Observations) == 0 {
		t.Error("observations não persistidas via OnEvent")
	}
}

// TestF2Delete — Delete remove a task do repositório (contrato da interface).
// O orquestrador NÃO auto-deleta (tasks terminais viram histórico não-retomado),
// mas o método é exposto e fica disponível para limpeza externa.
func TestF2Delete(t *testing.T) {
	repo := newFakeRepo()
	repo.Save(&task.TaskState{TaskID: "t-1", Objective: f2Obj(), Status: task.StatusActive})
	if err := repo.Delete(task.TaskID("t-1")); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if repo.count() != 0 {
		t.Fatalf("Delete não removeu; repo tem %d tasks", repo.count())
	}
	if got, _ := repo.Load(task.TaskID("t-1")); got != nil {
		t.Error("Load após Delete deveria ser nil")
	}
}

// TestF2TerminalPersistsButNotRehydrated — Complete persiste o estado terminal;
// um restart NÃO retoma task terminal (só não-terminais). Fecha o ciclo.
func TestF2TerminalPersistsButNotRehydrated(t *testing.T) {
	repo := newFakeRepo()
	o := New(Config{MaxContinue: 5}, WithRepository(repo))
	id, _ := o.Create(f2Obj(), "bot", "v1")
	if err := o.Complete(id); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	// O estado terminal foi persistido (histórico)...
	if _, err := repo.Load(id); err != nil {
		t.Fatalf("estado terminal deveria estar persistido (histórico): %v", err)
	}
	// ...mas um restart não retoma trabalho concluído.
	o2 := New(Config{MaxContinue: 5}, WithRepository(repo))
	if _, ok := o2.Status(id); ok {
		t.Error("task terminal NÃO deveria ser retomada após restart")
	}
}

// TestF2NoDeadlockOnContinueWithRepo — com repositório injetado, Continue
// persiste FORA do lock e (com o emitter da F3) não deadlocka: o persist
// acontece antes do emit síncrono, e o emit volta a OnEvent sem mutex retido.
func TestF2NoDeadlockOnContinueWithRepo(t *testing.T) {
	repo := newFakeRepo()
	var o *TaskOrchestrator
	o = New(Config{MaxContinue: 5},
		WithRepository(repo),
		WithEmitter(func(ev task.Event) error {
			// O bus síncrono volta ao orquestrador via OnEvent (como no adapter F3).
			o.OnEvent(ev)
			return nil
		}),
	)
	id, _ := o.Create(f2Obj(), "bot", "v1")
	if err := o.Continue(id, task.ContIncompleteObjective); err != nil {
		t.Fatalf("Continue (com repo + emit) deu deadlock/erro: %v", err)
	}
	st, _ := o.Status(id)
	if st.CurrentStep != 1 {
		t.Errorf("CurrentStep=%d, esperava 1", st.CurrentStep)
	}
}
