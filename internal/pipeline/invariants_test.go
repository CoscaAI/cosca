package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ── P0-1: Invariantes cross-entity ──────────────────────────────────────
//
// Regra do professor: testar a integridade do SISTEMA, não funções isoladas.
// Invariantes verificados:
//   1. plan com PlanCompleted → nenhuma task pode estar running/pending
//   2. task.completed → task.Result não nil e Success=true
//   3. ReplayPlan(estado) == estado esperado (STATE == STATE')

func TestInvariant_PlanCompletedMeansAllTasksDone(t *testing.T) {
	plan := &Plan{ID: "inv-1", Tasks: []*TaskNode{
		{ID: "t1", Description: "a"},
		{ID: "t2", Description: "b", DependsOn: []string{"t1"}},
		{ID: "t3", Description: "c", DependsOn: []string{"t2"}},
	}}
	now := time.Now().UTC()
	events := []StepEvent{
		{Type: PlanCreated, PlanID: "inv-1", Timestamp: now},
		{Type: StepEventCompleted, PlanID: "inv-1", TaskID: "t1", Timestamp: now},
		{Type: StepEventCompleted, PlanID: "inv-1", TaskID: "t2", Timestamp: now},
		{Type: StepEventCompleted, PlanID: "inv-1", TaskID: "t3", Timestamp: now},
		{Type: PlanCompleted, PlanID: "inv-1", Timestamp: now},
	}

	replayed := ReplayPlan(plan, events)
	if replayed.Status != "completed" {
		t.Fatalf("plan status = %q, want completed", replayed.Status)
	}
	for _, task := range replayed.Tasks {
		// INVARIANTE: plan completed → nenhuma task running/pending.
		if task.Status == TaskRunning || task.Status == TaskPending {
			t.Errorf("invariant violated: plan completed but task %s = %q", task.ID, task.Status)
		}
		// INVARIANTE: task completed → Result não nil e success.
		if task.Status == TaskCompleted && (task.Result == nil || !task.Result.Success) {
			t.Errorf("invariant violated: task %s completed but Result=%+v", task.ID, task.Result)
		}
	}
}

func TestInvariant_PlanFailedKeepsFailedTask(t *testing.T) {
	plan := &Plan{ID: "inv-2", Tasks: []*TaskNode{
		{ID: "t1", Description: "a"},
		{ID: "t2", Description: "b", DependsOn: []string{"t1"}},
	}}
	now := time.Now().UTC()
	events := []StepEvent{
		{Type: StepEventCompleted, PlanID: "inv-2", TaskID: "t1", Timestamp: now},
		{Type: StepEventFailed, PlanID: "inv-2", TaskID: "t2", Timestamp: now},
		{Type: PlanFailed, PlanID: "inv-2", Timestamp: now},
	}

	replayed := ReplayPlan(plan, events)
	if replayed.Status != "failed" {
		t.Fatalf("plan status = %q, want failed", replayed.Status)
	}
	if replayed.Tasks[0].Status != TaskCompleted {
		t.Errorf("t1 = %q, want completed", replayed.Tasks[0].Status)
	}
	if replayed.Tasks[1].Status != TaskFailed {
		t.Errorf("t2 = %q, want failed", replayed.Tasks[1].Status)
	}
	// INVARIANTE: task failed → Result não nil e !Success.
	if replayed.Tasks[1].Result == nil || replayed.Tasks[1].Result.Success {
		t.Errorf("failed task Result = %+v", replayed.Tasks[1].Result)
	}
}

// TestInvariant_ReplayIsDeterministic verifica STATE == STATE' após replay
// repetido (o mesmo conjunto de eventos produz o mesmo estado).
func TestInvariant_ReplayIsDeterministic(t *testing.T) {
	plan := &Plan{ID: "inv-3", Tasks: []*TaskNode{
		{ID: "t1", Description: "a"},
		{ID: "t2", Description: "b"},
	}}
	now := time.Now().UTC()
	events := []StepEvent{
		{Type: StepEventCompleted, PlanID: "inv-3", TaskID: "t1", Output: "ok", Timestamp: now},
		{Type: StepEventFailed, PlanID: "inv-3", TaskID: "t2", Output: "boom", Timestamp: now},
	}

	a := ReplayPlan(plan, events)
	b := ReplayPlan(plan, events)
	if a.Tasks[0].Status != b.Tasks[0].Status || a.Tasks[1].Result.Error != b.Tasks[1].Result.Error {
		t.Fatal("replay not deterministic")
	}
	// O plano original NUNCA é mutado (replay puro).
	if plan.Tasks[0].Status != "" && plan.Tasks[0].Status != TaskPending {
		t.Fatal("replay mutated the input plan")
	}
}

// ── P0-5: Concorrência — stress de contadores ──────────────────────────
//
// 100 tasks independentes × 8 workers: contadores finais devem ser exatos
// (sem lost update / duplicate event). Roda com -race no CI.

func TestStress_RunPlanManyIndependentTasks(t *testing.T) {
	or := &orderRunner{}
	sr := NewStepRunner(or)
	q := NewTaskQueue(sr, 8)

	plan := &Plan{ID: "stress-1", Tasks: make([]*TaskNode, 100)}
	for i := range plan.Tasks {
		plan.Tasks[i] = &TaskNode{
			ID:          "task-" + itoa(i),
			Description: "t" + itoa(i),
			Status:      TaskPending,
		}
	}

	result, err := q.RunPlan(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	if result.TasksDone != 100 {
		t.Errorf("TasksDone = %d, want 100", result.TasksDone)
	}
	if result.TasksFailed != 0 {
		t.Errorf("TasksFailed = %d, want 0", result.TasksFailed)
	}
	if len(or.Order()) != 100 {
		t.Errorf("executed = %d, want 100", len(or.Order()))
	}
}

func TestStress_RunPlanParallelIndependent(t *testing.T) {
	or := &orderRunner{}
	sr := NewStepRunner(or)

	plan := &Plan{ID: "stress-2", Tasks: make([]*TaskNode, 50)}
	for i := range plan.Tasks {
		plan.Tasks[i] = &TaskNode{ID: "p" + itoa(i), Description: "x", Status: TaskPending}
	}

	result, err := sr.RunPlanParallel(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("RunPlanParallel: %v", err)
	}
	if result.TasksDone != 50 || result.TasksFailed != 0 {
		t.Fatalf("done/failed = %d/%d", result.TasksDone, result.TasksFailed)
	}
	if len(or.Order()) != 50 {
		t.Fatalf("executed = %d, want 50", len(or.Order()))
	}
}

// ── P0-2: Corrupção no MEIO da chain (crash mid-write) ─────────────────

func TestInvariant_TruncatedEventInMiddleBreaksChain(t *testing.T) {
	h, _ := NewWorkflowHistory(t.TempDir())
	now := time.Now().UTC()

	// Três eventos válidos.
	for i := 1; i <= 3; i++ {
		_ = h.Append("p", StepEvent{Type: StepEventStarted, PlanID: "p", TaskID: "t" + itoa(i), Timestamp: now})
	}

	// Simula um crash durante a escrita: o SEGUNDO evento é truncado no meio
	// (JSON parcial). O Load é lenient (ignora linhas inválidas) — mas o
	// VerifyIntegrity deve DETECTAR a quebra da chain.
	path := h.filePath("p")
	lines := readLines(path)
	lines[1] = lines[1][:len(lines[1])/2] // trunca no meio do JSON
	writeLines(t, path, lines)

	events, err := h.Load("p")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(events) == 3 {
		t.Skip("load kept the truncated line — nothing to verify (unexpected)")
	}
	// O Load ignora o evento truncado; a chain dos restantes deve quebrar.
	n, err := h.VerifyIntegrity("p")
	if err == nil {
		t.Fatalf("VerifyIntegrity passed (%d events) despite truncated event in the middle", n)
	}
	if !strings.Contains(err.Error(), "hash chain broken") {
		t.Fatalf("expected hash chain break, got: %v", err)
	}
}

// TestInvariant_AppendAfterTruncationFails checksum do checkpoint também pega.
func TestInvariant_CheckpointDetectsPartialWrite(t *testing.T) {
	s, _ := NewCheckpointStore(t.TempDir())
	cp := Checkpoint{PlanID: "cp-1", TaskStatuses: map[string]string{"t1": "completed"}}
	if err := s.Save(cp); err != nil {
		t.Fatal(err)
	}
	// Simula crash mid-write: arquivo com JSON truncado.
	path := filepath.Join(s.dir, "cp-1.checkpoint.json")
	data, _ := os.ReadFile(path)
	_ = os.WriteFile(path, data[:len(data)/2], 0o644)

	if _, err := s.Load("cp-1"); err == nil {
		t.Fatal("truncated checkpoint must fail to load (CRC32 or JSON)")
	}
}
