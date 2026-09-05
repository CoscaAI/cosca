package pipeline

import (
	"context"
	"testing"
	"time"
)

// ── P1-1: Replay parcial pós-crash ──────────────────────────────────────
//
// Cenário do professor: crash no meio da execução. Eventos de t1-t2 foram
// gravados; t3 nunca rodou. O replay deve reconstruir um estado CONSISTENTE:
// t1/t2 com seus resultados, t3 ainda pending (nunca inventar estado).

func TestReplay_PartialCrashReconstructsConsistentState(t *testing.T) {
	plan := &Plan{ID: "rc-1", Tasks: []*TaskNode{
		{ID: "t1", Description: "a"},
		{ID: "t2", Description: "b", DependsOn: []string{"t1"}},
		{ID: "t3", Description: "c", DependsOn: []string{"t2"}},
	}}
	now := time.Now().UTC()
	// Crash depois de t1-t2: só os eventos deles existem.
	events := []StepEvent{
		{Type: StepEventCompleted, PlanID: "rc-1", TaskID: "t1", Output: "ok1", Agent: "a", Timestamp: now},
		{Type: StepEventCompleted, PlanID: "rc-1", TaskID: "t2", Output: "ok2", Agent: "a", Timestamp: now},
	}

	replayed := ReplayPlan(plan, events)
	if replayed.Tasks[0].Status != TaskCompleted || replayed.Tasks[0].Result.Output != "ok1" {
		t.Fatalf("t1: %+v", replayed.Tasks[0])
	}
	if replayed.Tasks[1].Status != TaskCompleted {
		t.Fatalf("t2: %+v", replayed.Tasks[1])
	}
	// INVARIANTE: t3 (nunca executado) deve estar pending — NUNCA inventado.
	if replayed.Tasks[2].Status != TaskPending {
		t.Fatalf("t3 = %q, want pending (must not be invented)", replayed.Tasks[2].Status)
	}
}

// ── P1-2: Idempotência — Resume ×3 ─────────────────────────────────────
//
// Cenário do professor: Resume(session) ×3. O durable log deve garantir que
// cada passo é executado UMA vez, não importa quantas vezes o run é repetido.

func TestDurable_ResumeThreeTimesIsIdempotent(t *testing.T) {
	dir := t.TempDir()

	runner := &countingRunner{}
	sr, err := NewStepRunnerWithOptions(runner, WithDurableLog(dir))
	if err != nil {
		t.Fatalf("NewStepRunnerWithOptions: %v", err)
	}

	plan := durableTestPlan("idem-3")
	for i := 0; i < 3; i++ {
		res, err := sr.RunPlan(context.Background(), plan)
		if err != nil {
			t.Fatalf("RunPlan #%d: %v", i+1, err)
		}
		if res.TasksDone != 3 || res.TasksFailed != 0 {
			t.Fatalf("RunPlan #%d: done/failed = %d/%d", i+1, res.TasksDone, res.TasksFailed)
		}
	}

	// INVARIANTE de idempotência: 3 runs → apenas 3 execuções de tarefa
	// (cada t1/t2/t3 rodou exatamente UMA vez).
	if got := runner.CallCount(); got != 3 {
		t.Fatalf("idempotency violated: 3 runs executed %d steps, want 3", got)
	}
}

// ── P1-3: Tool output extremo ───────────────────────────────────────────
// (testes de arquivo gigante/vazio ficam em internal/chat/tool — tool_extreme_test.go)

// ── P1-4: Adversarial prompts ───────────────────────────────────────────

func TestPlanner_AdversarialPrompts(t *testing.T) {
	p := &Planner{}

	// Vazio: não pode panickar; deve produzir um plano de fallback.
	empty := p.Plan("")
	if empty == nil {
		t.Fatal("empty prompt must still produce a plan (fallback), not nil")
	}
	if len(empty.Tasks) < 1 {
		t.Fatal("empty prompt must produce at least a fallback task")
	}

	// Só espaços.
	spaces := p.Plan("   \n\t  ")
	if spaces == nil || len(spaces.Tasks) < 1 {
		t.Fatal("whitespace-only prompt must fall back, not panic")
	}

	// Extremamente longo: 10k palavras.
	long := ""
	for i := 0; i < 10000; i++ {
		long += "palavra palavra palavra "
	}
	big := p.Plan(long)
	if big == nil || len(big.Tasks) < 1 {
		t.Fatal("long prompt must still produce a plan")
	}

	// Ambigüidade: keywords de intents CONFLITANTES (api + test + deploy).
	conflictPrompt := "build the api and write tests then deploy to production"
	confPlan := p.Plan(conflictPrompt)
	if confPlan == nil {
		t.Fatal("conflicting keywords must produce a plan (first intent wins deterministically)")
	}
	// Determinismo: o mesmo prompt → o mesmo plano.
	again := p.Plan(conflictPrompt)
	if confPlan.Intent != again.Intent {
		t.Fatalf("non-deterministic intent: %q vs %q", confPlan.Intent, again.Intent)
	}
}
