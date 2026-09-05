package pipeline

import (
	"context"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// ── Chaos V4: pipeline restart em momentos diferentes ───────────────────
//
// A nova régua do professor: o V4 testa CONTINUIDADE DA EXECUÇÃO, não só
// resistência à carga. Restart em 10%/25%/50%/75%/90% do progresso, com os
// invariantes duros:
//   - zero sessão/task perdida
//   - zero execução durable duplicada (tasks COMPLETAS nunca re-executam)
//   - zero estado inválido após restart
//   - pipeline retoma do ponto correto, nunca do zero
//   - retry continua finito (a task "no meio do crash" re-executa 1×, não N×)

// progressCrashRunner crasha quando o PROGRESSO GLOBAL (tasks reais
// completadas, compartilhado entre processos) atinge crashAtGlobal. Isso
// simula "o pipeline morre aos X% de progresso", independente de quantas
// tasks o processo atual re-executa do durable.
type progressCrashRunner struct {
	global  *atomic.Int64 // progresso global compartilhado
	crashAt int64         // crasha quando global >= crashAt
	calls   atomic.Int64
}

func (r *progressCrashRunner) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	r.calls.Add(1)
	if r.global.Load() >= r.crashAt {
		return nil, fmt.Errorf("runtime: process crash at progress %d (simulated, non-transient)", r.crashAt)
	}
	r.global.Add(1) // completou a task → progresso global avança
	return &RunResult{
		Response:    "ok: " + req.Prompt,
		Agent:       req.Agent,
		BuildResult: &BuildResult{Success: true},
		TestResult:  &TestResult{Success: true},
	}, nil
}

func (r *progressCrashRunner) RunStream(ctx context.Context, req RunRequest) (<-chan RunEvent, error) {
	ch := make(chan RunEvent, 1)
	ch <- RunEvent{Type: EventDone}
	close(ch)
	return ch, nil
}

// v4Plan cria um plano de n tasks independentes.
func v4Plan(id string, n int) *Plan {
	plan := &Plan{ID: id, Intent: "v4-restart", Tasks: make([]*TaskNode, n)}
	for i := 0; i < n; i++ {
		plan.Tasks[i] = &TaskNode{ID: fmt.Sprintf("t%d", i+1), Description: fmt.Sprintf("task %d", i+1), Agent: "a", Status: TaskPending}
	}
	return plan
}

// ── Helpers ─────────────────────────────────────────────────────────────

// loadDurableEvents carrega todos os eventos duráveis de um run.
func loadDurableEvents(dir, runID string) ([]DurableEvent, error) {
	l, err := NewDurableEventLog(dir)
	if err != nil {
		return nil, err
	}
	return l.LoadRun(runID)
}

// countEvent conta eventos de um tipo.
func countEvent(events []DurableEvent, typ string) int {
	n := 0
	for _, e := range events {
		if e.Type == typ {
			n++
		}
	}
	return n
}

// TestChaosV4_RestartAtProgressPoints simula 6 "processos": 5 morrem em
// pontos de progresso GLOBAL (10/25/50/75/90% de 20 tasks), o 6º completa.
func TestChaosV4_RestartAtProgressPoints(t *testing.T) {
	dir := t.TempDir()
	const total = 20
	// Pontos de crash GLOBAIS: 2, 5, 10, 15, 18 ≈ 10%, 25%, 50%, 75%, 90%.
	crashPoints := []int64{2, 5, 10, 15, 18}
	global := &atomic.Int64{}

	for pi, crashAt := range crashPoints {
		runner := &progressCrashRunner{global: global, crashAt: crashAt}
		sr, err := NewStepRunnerWithOptions(runner, WithDurableLog(filepath.Join(dir, "events")))
		if err != nil {
			t.Fatalf("process %d: %v", pi, err)
		}
		plan := v4Plan("run-v4", total)
		res, err := sr.RunPlan(context.Background(), plan)
		// O crash deixa a task "no meio" falhada: run termina com falhas.
		if err == nil && res.TasksFailed == 0 {
			t.Fatalf("process %d (crash@%d) must leave failed tasks (progress=%d)", pi, crashAt, global.Load())
		}
		_ = res
	}

	// Processo final: sem crash, completa tudo retomando do log.
	finRunner := &progressCrashRunner{global: global, crashAt: 100}
	srF, err := NewStepRunnerWithOptions(finRunner, WithDurableLog(filepath.Join(dir, "events")))
	if err != nil {
		t.Fatal(err)
	}
	res, err := srF.RunPlan(context.Background(), v4Plan("run-v4", total))
	if err != nil {
		t.Fatalf("final process: %v", err)
	}
	if res.TasksDone != total || res.TasksFailed != 0 {
		t.Fatalf("final: done/failed = %d/%d, want %d/0", res.TasksDone, res.TasksFailed, total)
	}

	// INVARIANTES do professor:
	// - zero perda: TODAS as 20 tasks têm evento de completed no log, e cada
	//   task completou EXATAMENTE 1× (zero duplicação de completed).
	// - retry continua finito: as falhas registradas são tentativas de
	//   recuperação do recovery loop DENTRO do run (>= 1 por ponto de crash),
	//   mas nunca infinitas.
	events, err := loadDurableEvents(filepath.Join(dir, "events"), "run-v4")
	if err != nil {
		t.Fatalf("load events: %v", err)
	}
	completed := countEvent(events, DurableStepCompleted)
	failed := countEvent(events, DurableStepFailed)
	if completed != total {
		t.Fatalf("completed events = %d, want %d (zero loss / zero duplication of completed)", completed, total)
	}
	if failed < 5 {
		t.Fatalf("failed events = %d, want >= 5 (pelo menos 1 por ponto de crash)", failed)
	}
	// O log é íntegro após 5 restarts.
	l, err := NewDurableEventLog(filepath.Join(dir, "events"))
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := l.VerifyChain("run-v4"); err != nil || !ok {
		t.Fatalf("chain inválida após 5 restarts: %v, %v", ok, err)
	}
	t.Logf("V4: %d tasks, 5 restarts (10/25/50/75/90%%), zero perda (completed=%d), retries finitos (failed=%d), chain íntegra", total, completed, failed)
}

// TestChaosV4_StateEquivalenceBeforeAfter compara o estado lógico antes e
// depois de um restart: tasks completadas ANTES permanecem completadas DEPOIS
// e NÃO re-executam (BEFORE == AFTER no nível de estado).
func TestChaosV4_StateEquivalenceBeforeAfter(t *testing.T) {
	dir := t.TempDir()

	// Processo 1: completa 3 tasks e morre.
	global := &atomic.Int64{}
	runner1 := &progressCrashRunner{global: global, crashAt: 3}
	sr1, _ := NewStepRunnerWithOptions(runner1, WithDurableLog(filepath.Join(dir, "events")))
	_, _ = sr1.RunPlan(context.Background(), v4Plan("equiv", 10))

	// BEFORE RESTART: tasks 1-3 completadas (estado gravado no log).
	eventsBefore, _ := loadDurableEvents(filepath.Join(dir, "events"), "equiv")
	beforeCompleted := countEvent(eventsBefore, DurableStepCompleted)
	if beforeCompleted != 3 {
		t.Fatalf("before restart: completed = %d, want 3", beforeCompleted)
	}

	// RESTART + RECOVERY: processo novo completa o resto.
	runner2 := &countingRunner{}
	sr2, _ := NewStepRunnerWithOptions(runner2, WithDurableLog(filepath.Join(dir, "events")))
	res, err := sr2.RunPlan(context.Background(), v4Plan("equiv", 10))
	if err != nil {
		t.Fatalf("after restart: %v", err)
	}
	if res.TasksDone != 10 {
		t.Fatalf("after restart: done = %d, want 10", res.TasksDone)
	}

	// AFTER: estado equivalente — 10 completadas no log, e o processo 2 só
	// executou 7 tasks (t4-t10), nunca re-executou t1-t3.
	eventsAfter, _ := loadDurableEvents(filepath.Join(dir, "events"), "equiv")
	afterCompleted := countEvent(eventsAfter, DurableStepCompleted)
	if afterCompleted != 10 {
		t.Fatalf("after restart: completed = %d, want 10", afterCompleted)
	}
	if runner2.CallCount() != 7 {
		t.Fatalf("post-restart executed %d steps, want 7 (t4-t10; t1-t3 replayed)", runner2.CallCount())
	}
	t.Logf("V4 equivalência: BEFORE={t1-t3 completed} → RESTART → AFTER={10 completed}, pós-restart executou 7 (sem re-execução)")
}
