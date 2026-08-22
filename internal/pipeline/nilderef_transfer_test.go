package pipeline

import (
	"context"
	"testing"
)

// ── LOOP V4 L337: transferência para o StepRunner ──────────────────────
//
// O StepRunner acessa s.runner.Run sem guard. Com runner nil, RunStep deve
// PANIC — o mesmo padrão da família. Previsão: sim.

func scanPipelinePanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("TRANSFERÊNCIA CONFIRMADA: %s PANIC com dep nil (bug da família!)", name)
		}
	}()
	fn()
}

func TestTransfer_StepRunnerNilRunner(t *testing.T) {
	sr := &StepRunner{} // runner nil
	scanPipelinePanic(t, "StepRunner.RunStep", func() {
		_, _ = sr.RunStep(context.Background(), "p1", &TaskNode{ID: "t1", Description: "x"})
	})
}

func TestTransfer_StepRunnerNilRunnerRunPlan(t *testing.T) {
	sr := &StepRunner{} // runner nil
	plan := &Plan{ID: "p", Tasks: []*TaskNode{{ID: "t1", Description: "x", Status: TaskPending}}}
	scanPipelinePanic(t, "StepRunner.RunPlan", func() {
		_, _ = sr.RunPlan(context.Background(), plan)
	})
}
