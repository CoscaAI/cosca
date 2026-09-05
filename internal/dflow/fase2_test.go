package dflow

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestReplayDivergence_FailClosed: retomar com um workflow que muda a ORDEM das
// activities → ErrReplayDiverged (fail-closed I2; nunca "conserta").
func TestReplayDivergence_FailClosed(t *testing.T) {
	r := NewRunner()
	r.sleep = func(_ context.Context, _ time.Duration) error { return nil }

	a := &countingActivity{ID: "a", Cnt: new(int64)}
	b := &countingActivity{ID: "b", Cnt: new(int64)}
	c := &countingActivity{ID: "c", Cnt: new(int64)}

	// 1º run: a→b→c.
	wf := func(ctx context.Context, wc *WorkflowCtx) (any, error) {
		_, _ = wc.ExecActivity(ctx, a)
		_, _ = wc.ExecActivity(ctx, b)
		_, _ = wc.ExecActivity(ctx, c)
		return nil, nil
	}
	res, _ := r.Run(context.Background(), wf, []Activity{a, b, c}, nil, nil)
	if len(res.History.ExecutedOrder) != 3 {
		t.Fatalf("historico do 1o run deveria ter 3 itens, got %d", len(res.History.ExecutedOrder))
	}

	// 2º run: replay com ordem DIFERENTE (a→c→b) e o histórico completo.
	// O workflow PROPAGA o erro do ExecActivity — a divergência precisa chegar
	// ao Run (se o workflow engolir o erro, o Run vê sucesso).
	wf2 := func(ctx context.Context, wc *WorkflowCtx) (any, error) {
		if _, err := wc.ExecActivity(ctx, a); err != nil {
			return nil, err
		}
		if _, err := wc.ExecActivity(ctx, c); err != nil { // diverge: esperava b
			return nil, err
		}
		if _, err := wc.ExecActivity(ctx, b); err != nil {
			return nil, err
		}
		return nil, nil
	}
	_, err := r.Run(context.Background(), wf2, []Activity{a, b, c}, nil, &RunOptions{History: res.History})
	if !errors.Is(err, ErrReplayDiverged) {
		t.Fatalf("replay de ordem divergente deveria dar ErrReplayDiverged, got %v", err)
	}
}

// TestNow_Rand_Deterministic: wc.Now()/wc.Rand() são determinísticos pela seed
// (mesma seed → mesmas decisões no replay; I1 mecânico).
func TestNow_Rand_Deterministic(t *testing.T) {
	r := NewRunner()
	r.sleep = func(_ context.Context, _ time.Duration) error { return nil }
	r.SetRandomSeed(7)
	now := time.Unix(1700000000, 0)
	r.now = func() time.Time { return now }

	pick := func(ctx context.Context, wc *WorkflowCtx) (any, error) {
		_ = wc.Now()          // relógio determinístico
		n := wc.Rand().Intn(1000) // RNG determinístico (seed 7)
		return n, nil
	}

	// Duas execuções: mesma seed → mesmo número.
	r1, _ := r.Run(context.Background(), pick, nil, nil, nil)
	r2, _ := r.Run(context.Background(), pick, nil, nil, nil)
	if r1.Output != r2.Output {
		t.Fatalf("Rand() deveria ser determinístico com mesma seed: %v vs %v", r1.Output, r2.Output)
	}
	v, _ := r1.Output.(int)
	if v < 0 || v >= 1000 {
		t.Fatalf("rand fora de faixa: %v", r1.Output)
	}
}

// TestNow_Replay_RelogioCongelado: no replay com histórico, Now() deve refletir
// o relógio do runner (determinístico), não time real.
func TestNow_Replay_RelogioCongelado(t *testing.T) {
	r := NewRunner()
	r.sleep = func(_ context.Context, _ time.Duration) error { return nil }
	now := time.Unix(1800000000, 0)
	r.now = func() time.Time { return now }

	seenNow := time.Time{}
	wf := func(_ context.Context, wc *WorkflowCtx) (any, error) {
		seenNow = wc.Now()
		return nil, nil
	}
	_, _ = r.Run(context.Background(), wf, nil, nil, nil)
	if !seenNow.Equal(now) {
		t.Fatalf("wc.Now() deveria refletir o relógio injetado (determinístico), got %v want %v", seenNow, now)
	}
}
