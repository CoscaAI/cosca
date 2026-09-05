package dflow

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// countingActivity conta execuções reais.
type countingActivity struct {
	ID  ActivityKey
	Cnt *int64
	Err error // erro nas primeiras N tentativas
}

func (c *countingActivity) Key() ActivityKey { return c.ID }
func (c *countingActivity) Run(_ context.Context) (any, error) {
	atomic.AddInt64(c.Cnt, 1)
	if c.Err != nil {
		return nil, c.Err
	}
	return string(c.ID), nil
}

// pipelineWF monta o workflow exemplo: extract_audio → whisper → subtitle.
// Usa as activities passadas (para os testes contarem execuções reais).
func makePipelineWF(extract, whisper, subtitle Activity) Workflow {
	return func(ctx context.Context, wc *WorkflowCtx) (any, error) {
		if _, err := wc.ExecActivity(ctx, extract); err != nil {
			return nil, err
		}
		text, err := wc.ExecActivity(ctx, whisper)
		if err != nil {
			return nil, err
		}
		if _, err := wc.ExecActivity(ctx, subtitle); err != nil {
			return nil, err
		}
		return text, nil
	}
}

func TestRunExecutesWorkflow(t *testing.T) {
	r := NewRunner()
	var cnt int64
	extract := &countingActivity{ID: "extract_audio", Cnt: &cnt}
	whisper := &countingActivity{ID: "whisper", Cnt: &cnt}
	subtitle := &countingActivity{ID: "subtitle", Cnt: &cnt}
	acts := []Activity{extract, whisper, subtitle}
	wf := makePipelineWF(extract, whisper, subtitle)

	res, err := r.Run(context.Background(), wf, acts, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "whisper" {
		t.Fatalf("output = %v", res.Output)
	}
	if res.Executed != 3 {
		t.Fatalf("Executed = %d, want 3", res.Executed)
	}
	if cnt != 3 {
		t.Fatalf("activity ran %d times, want 3", cnt)
	}
}

func TestReplayDoesNotReexecute(t *testing.T) {
	// Cenário P2/§40: workflow roda, histórico é salvo, processo morre.
	// Na retomada com o histórico, os effects NÃO re-executam.
	r := NewRunner()
	var cnt int64
	extract := &countingActivity{ID: "extract_audio", Cnt: &cnt}
	whisper := &countingActivity{ID: "whisper", Cnt: &cnt}
	subtitle := &countingActivity{ID: "subtitle", Cnt: &cnt}
	acts := []Activity{extract, whisper, subtitle}
	wf := makePipelineWF(extract, whisper, subtitle)

	// 1ª rodada — executa tudo.
	res1, err := r.Run(context.Background(), wf, acts, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res1.Executed != 3 || cnt != 3 {
		t.Fatalf("first run: executed=%d cnt=%d", res1.Executed, cnt)
	}

	// 2ª rodada com o histórico — replay: 0 execuções reais, tudo retomado.
	res2, err := r.Run(context.Background(), wf, acts, nil, &RunOptions{History: res1.History})
	if err != nil {
		t.Fatal(err)
	}
	if res2.Executed != 0 {
		t.Fatalf("replay executed = %d, want 0", res2.Executed)
	}
	if res2.Replayed != 3 {
		t.Fatalf("replay replayed = %d, want 3", res2.Replayed)
	}
	if cnt != 3 {
		t.Fatalf("activity re-ran %d times after replay, want 3 (no re-execution)", cnt)
	}
}

func TestRetrySucceeds(t *testing.T) {
	// Activity falha 2x, depois funciona. Retry deve recuperar.
	r := NewRunner()
	var cnt int64
	var attempts int
	failing := ActivityFunc{
		ID: "unstable",
		Fn: func(_ context.Context) (any, error) {
			attempts++
			atomic.AddInt64(&cnt, 1)
			if attempts < 3 {
				return nil, errors.New("transient failure")
			}
			return "ok", nil
		},
	}
	// sleep imediato para teste (sem espera real).
	r.sleep = func(_ context.Context, _ time.Duration) error { return nil }

	wf := func(ctx context.Context, wc *WorkflowCtx) (any, error) {
		return wc.ExecActivity(ctx, failing)
	}
	res, err := r.Run(context.Background(), wf, []Activity{failing}, map[ActivityKey]RetryPolicy{
		"unstable": {MaxAttempts: 5, InitialInterval: time.Millisecond},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "ok" {
		t.Fatalf("output = %v", res.Output)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestRetryExhausted(t *testing.T) {
	r := NewRunner()
	r.sleep = func(_ context.Context, _ time.Duration) error { return nil }
	var cnt int64
	always := &countingActivity{ID: "always-fails", Cnt: &cnt, Err: errors.New("hard failure")}

	wf := func(ctx context.Context, wc *WorkflowCtx) (any, error) {
		return wc.ExecActivity(ctx, always)
	}
	_, err := r.Run(context.Background(), wf, []Activity{always}, map[ActivityKey]RetryPolicy{
		"always-fails": {MaxAttempts: 3, InitialInterval: time.Millisecond},
	}, nil)
	if err == nil {
		t.Fatal("expected ErrActivityFailed")
	}
	if cnt != 3 {
		t.Fatalf("attempts = %d, want 3 (max)", cnt)
	}
}

func TestBackoffGrows(t *testing.T) {
	p := RetryPolicy{MaxAttempts: 4, InitialInterval: time.Second, MaxInterval: 30 * time.Second, BackoffMultiplier: 2}.withDefaults()
	if d := p.backoffFor(1); d != time.Second {
		t.Fatalf("backoff(1) = %v, want 1s", d)
	}
	if d := p.backoffFor(2); d != 2*time.Second {
		t.Fatalf("backoff(2) = %v, want 2s", d)
	}
	if d := p.backoffFor(3); d != 4*time.Second {
		t.Fatalf("backoff(3) = %v, want 4s", d)
	}
	// Teto: com initial alto, satura no MaxInterval.
	p2 := RetryPolicy{MaxAttempts: 10, InitialInterval: 30 * time.Second, MaxInterval: 30 * time.Second, BackoffMultiplier: 2}.withDefaults()
	if d := p2.backoffFor(5); d != 30*time.Second {
		t.Fatalf("capped backoff = %v, want 30s", d)
	}
}

func TestCrashRecovery(t *testing.T) {
	// Cenário §40 completo: workflow crasha no meio (2ª activity falha),
	// histórico parcial sobrevive, retomada completa sem refazer a 1ª.
	r := NewRunner()
	r.sleep = func(_ context.Context, _ time.Duration) error { return nil }

	var extractCnt, whisperCnt int64
	extract := &countingActivity{ID: "extract_audio", Cnt: &extractCnt}
	whisper := &countingActivity{ID: "whisper", Cnt: &whisperCnt}

	// Workflow que falha na 2ª activity (crash simulado).
	crashingWF := func(ctx context.Context, wc *WorkflowCtx) (any, error) {
		if _, err := wc.ExecActivity(ctx, extract); err != nil {
			return nil, err
		}
		return nil, errors.New("simulated crash before subtitle")
	}
	acts := []Activity{extract, whisper}
	if _, err := r.Run(context.Background(), crashingWF, acts, nil, nil); err == nil {
		t.Fatal("expected crash error")
	}
	if extractCnt != 1 {
		t.Fatalf("extract ran %d, want 1", extractCnt)
	}

	// Retomada: workflow completo com o histórico parcial.
	// O histórico do crash só tem extract (whisper nunca executou).
	partialHistory := NewHistory()
	partialHistory.ActivityOutcomes["extract_audio"] = ActivityOutcome{Key: "extract_audio", Value: "audio.wav"}
	partialHistory.ExecutedOrder = []ActivityKey{"extract_audio"}

	wf := makePipelineWF(extract, whisper, &countingActivity{ID: "subtitle", Cnt: new(int64)})
	res, err := r.Run(context.Background(), wf, acts, nil, &RunOptions{History: partialHistory})
	if err != nil {
		t.Fatal(err)
	}
	if extractCnt != 1 {
		t.Fatalf("extract re-ran after recovery, want 1 (idempotent via history)")
	}
	if whisperCnt != 1 {
		t.Fatalf("whisper ran %d, want 1 (only the missing one)", whisperCnt)
	}
	if res.Output != "whisper" {
		t.Fatalf("recovered output = %v", res.Output)
	}
}

func TestHistorySortKeys(t *testing.T) {
	h := NewHistory()
	h.ActivityOutcomes["b"] = ActivityOutcome{Key: "b", Value: 1}
	h.ActivityOutcomes["a"] = ActivityOutcome{Key: "a", Value: 2}
	keys := h.SortKeys()
	if keys[0] != "a" || keys[1] != "b" {
		t.Fatalf("sort keys = %v", keys)
	}
}

func TestActivityFromContext(t *testing.T) {
	var gotAttempt int
	act := ActivityFunc{
		ID: "ctx",
		Fn: func(ctx context.Context) (any, error) {
			ac := ActivityFromContext(ctx)
			if ac != nil {
				gotAttempt = ac.Attempt
			}
			return "ok", nil
		},
	}
	r := NewRunner()
	wf := func(ctx context.Context, wc *WorkflowCtx) (any, error) {
		return wc.ExecActivity(ctx, act)
	}
	if _, err := r.Run(context.Background(), wf, []Activity{act}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if gotAttempt != 1 {
		t.Fatalf("attempt = %d, want 1", gotAttempt)
	}
}

func TestSleepUntilReplay(t *testing.T) {
	// Sleep durável: 1ª rodada registra e dorme; replay com o histórico NÃO
	// chama o sleep de novo (deadline já passado → satisfeito imediatamente).
	r := NewRunner()
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return now }

	var sleeps int
	r.sleep = func(_ context.Context, d time.Duration) error {
		sleeps++
		return nil
	}

	deadline := now.Add(-time.Hour) // já passado
	wf := func(ctx context.Context, wc *WorkflowCtx) (any, error) {
		if err := wc.SleepUntil(ctx, deadline); err != nil {
			return nil, err
		}
		return "awake", nil
	}

	res1, err := r.Run(context.Background(), wf, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res1.Output != "awake" {
		t.Fatalf("run#1 output = %v", res1.Output)
	}
	if sleeps != 1 {
		t.Fatalf("run#1 sleep calls = %d, want 1", sleeps)
	}

	res2, err := r.Run(context.Background(), wf, nil, nil, &RunOptions{History: res1.History})
	if err != nil {
		t.Fatal(err)
	}
	if res2.Output != "awake" {
		t.Fatalf("run#2 output = %v", res2.Output)
	}
	if sleeps != 1 {
		t.Fatalf("sleep re-called on replay = %d, want 1 (no re-sleep)", sleeps)
	}
}

func TestSleepUntilCrashRecovery(t *testing.T) {
	// Crash no meio do sleep: o deadline foi registrado no histórico. A retomada
	// deve esperar apenas o tempo RESTANTE (deadline − now), não o total.
	r := NewRunner()
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return now }
	deadline := now.Add(10 * time.Minute)

	var got time.Duration
	failOnce := true
	r.sleep = func(_ context.Context, d time.Duration) error {
		got = d
		if failOnce {
			failOnce = false
			return errors.New("simulated crash mid-sleep")
		}
		return nil
	}

	wf := func(ctx context.Context, wc *WorkflowCtx) (any, error) {
		if err := wc.SleepUntil(ctx, deadline); err != nil {
			return nil, err
		}
		return "awake", nil
	}

	// Run #1: registra o sleep e o processo morre no meio (sleep falha).
	if _, err := r.Run(context.Background(), wf, nil, nil, nil); err == nil {
		t.Fatal("expected sleep failure on run#1")
	}

	// Histórico que sobreviveu (checkpoint com o sleep registrado).
	resumeHistory := NewHistory()
	resumeHistory.Sleeps["sleep:1"] = deadline

	// Retomada 2min depois — deve dormir só o restante (8min) e completar.
	now = now.Add(2 * time.Minute)
	res, err := r.Run(context.Background(), wf, nil, nil, &RunOptions{History: resumeHistory})
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "awake" {
		t.Fatalf("recovered output = %v", res.Output)
	}
	if want := 8 * time.Minute; got != want {
		t.Fatalf("remaining sleep = %v, want %v", got, want)
	}
}

func TestWaitForEventPendingResume(t *testing.T) {
	// HITL: workflow pausa (ErrPending) aguardando aprovação externa; o Don
	// resolve o evento e o re-run retoma do ponto exato.
	r := NewRunner()
	var cnt int64
	act := &countingActivity{ID: "post_approval", Cnt: &cnt}

	wf := func(ctx context.Context, wc *WorkflowCtx) (any, error) {
		v, ok := wc.WaitForEvent("approval")
		if !ok {
			return nil, ErrPending
		}
		res, err := wc.ExecActivity(ctx, act)
		if err != nil {
			return nil, err
		}
		return fmt.Sprintf("%v-%v", res, v), nil
	}

	// Run #1: nenhum evento → suspende (pending), sem erro.
	res1, err := r.Run(context.Background(), wf, []Activity{act}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res1.Pending {
		t.Fatal("run#1 should be pending")
	}
	if res1.Output != nil {
		t.Fatalf("run#1 output = %v, want nil", res1.Output)
	}
	if cnt != 0 {
		t.Fatalf("activity ran before approval = %d, want 0", cnt)
	}

	// Don aprova: resolve o evento SEM mutar o histórico original.
	h := ResolveEvent(res1.History, "approval", "approved")
	if _, ok := res1.History.Events["approval"]; ok {
		t.Fatal("ResolveEvent mutated the original history")
	}

	// Run #2 com o histórico resolvido: retoma e continua.
	res2, err := r.Run(context.Background(), wf, []Activity{act}, nil, &RunOptions{History: h})
	if err != nil {
		t.Fatal(err)
	}
	if res2.Pending {
		t.Fatal("run#2 should not be pending")
	}
	if res2.Output != "post_approval-approved" {
		t.Fatalf("run#2 output = %v, want post_approval-approved", res2.Output)
	}
	if cnt != 1 {
		t.Fatalf("activity count = %d, want 1 (executed once after resume)", cnt)
	}
	if res2.Executed != 1 || res2.Replayed != 0 {
		t.Fatalf("run#2 executed=%d replayed=%d, want 1/0", res2.Executed, res2.Replayed)
	}
}

func TestHistoryRetrocompat(t *testing.T) {
	// Histórico antigo (JSON sem Sleeps/Events → maps nil) deve funcionar:
	// leituras nil-safe, sleeps registrados sob demanda, activities retomadas.
	r := NewRunner()
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return now }
	var sleeps int
	r.sleep = func(_ context.Context, _ time.Duration) error { sleeps++; return nil }

	var cnt int64
	extract := &countingActivity{ID: "extract_audio", Cnt: &cnt}

	old := NewHistory()
	old.ActivityOutcomes["extract_audio"] = ActivityOutcome{Key: "extract_audio", Value: "audio.wav"}
	old.ExecutedOrder = []ActivityKey{"extract_audio"}
	old.Sleeps = nil
	old.Events = nil

	deadline := now.Add(time.Hour)
	wf := func(ctx context.Context, wc *WorkflowCtx) (any, error) {
		if v, ok := wc.WaitForEvent("approval"); ok {
			return v, nil
		}
		if err := wc.SleepUntil(ctx, deadline); err != nil {
			return nil, err
		}
		return wc.ExecActivity(ctx, extract)
	}

	res, err := r.Run(context.Background(), wf, []Activity{extract}, nil, &RunOptions{History: old})
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "audio.wav" {
		t.Fatalf("output = %v, want audio.wav (replayed from old history)", res.Output)
	}
	if cnt != 0 {
		t.Fatalf("activity re-ran = %d, want 0 (replayed)", cnt)
	}
	if sleeps != 1 {
		t.Fatalf("sleep calls = %d, want 1 (first registration)", sleeps)
	}
}
