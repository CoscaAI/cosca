package ingest

import (
	"context"
	"testing"
)

// TestPipeline_HappyPath valida que steps sem drop completam e somam duração.
func TestPipeline_HappyPath(t *testing.T) {
	p := &Pipeline{Steps: []Step{
		{Name: "validate", Fn: func(ctx context.Context) (bool, DropReason) { return false, "" }},
		{Name: "execute", Fn: func(ctx context.Context) (bool, DropReason) { return false, "" }},
		{Name: "record", Fn: func(ctx context.Context) (bool, DropReason) { return false, "" }},
	}}
	res := p.Run(context.Background())
	if res.Dropped {
		t.Fatal("nao deveria estar dropped")
	}
	if len(res.Steps) != 3 {
		t.Fatalf("esperava 3 steps, got %d", len(res.Steps))
	}
	if res.DropReason != "" {
		t.Fatalf("drop_reason deveria ser vazio, got %q", res.DropReason)
	}
}

// TestPipeline_EarlyHaltOnDrop valida o padrão Plausible: o primeiro step que
// drop=true encerra (early-halt) e steps posteriores NÃO são executados.
func TestPipeline_EarlyHaltOnDrop(t *testing.T) {
	var afterExecuted bool
	p := &Pipeline{Steps: []Step{
		{Name: "budget_check", Fn: func(ctx context.Context) (bool, DropReason) { return true, DropCostoExcedido }},
		{Name: "execute", Fn: func(ctx context.Context) (bool, DropReason) { afterExecuted = true; return false, "" }},
	}}
	res := p.Run(context.Background())
	if !res.Dropped {
		t.Fatal("deveria estar dropped")
	}
	if res.DropReason != DropCostoExcedido {
		t.Fatalf("drop_reason deveria ser cost_exceeded, got %q", res.DropReason)
	}
	if afterExecuted {
		t.Fatal("step 'execute' nao deveria rodar apos early-halt")
	}
	if len(res.Steps) != 1 {
		t.Fatalf("esperava somente o step que dropou, got %d", len(res.Steps))
	}
}

// TestPipeline_StepDurations valida que cada step reporta duração (>=0).
func TestPipeline_StepDurations(t *testing.T) {
	p := &Pipeline{Steps: []Step{
		{Name: "a", Fn: func(ctx context.Context) (bool, DropReason) { return false, "" }},
		{Name: "b", Fn: func(ctx context.Context) (bool, DropReason) { return false, "" }},
	}}
	res := p.Run(context.Background())
	if res.Steps[0].DurationMs < 0 || res.Steps[1].DurationMs < 0 {
		t.Fatal("duracoes deveriam ser >= 0")
	}
	if res.TotalMs < 0 {
		t.Fatalf("total ms deveria ser >= 0, got %d", res.TotalMs)
	}
}
