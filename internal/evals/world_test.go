package evals

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/evidence"
	"github.com/CoscaAI/cosca/internal/trace"
)

func ctx() context.Context { return context.Background() }

// TestSeedBeforeActError_SeedAfterAct — (a) semear DEPOIS do primeiro ato é
// um erro explícito `SeedBeforeActError` (invariante de ordem, não convenção).
func TestSeedBeforeActError_SeedAfterAct(t *testing.T) {
	h := NewHarness("wbs")
	if err := h.Seed(ctx(), "setup", func(context.Context) error { return nil }); err != nil {
		t.Fatalf("primeiro seed deveria passar: %v", err)
	}
	if err := h.Act(ctx(), "act-1", func(context.Context) error { return nil }); err != nil {
		t.Fatalf("act deveria passar: %v", err)
	}
	err := h.Seed(ctx(), "setup", func(context.Context) error { return nil })
	if err == nil {
		t.Fatal("seed após act deveria dar erro")
	}
	var sbe *SeedBeforeActError
	if !errors.As(err, &sbe) {
		t.Fatalf("erro deveria ser *SeedBeforeActError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "seed-before-act") {
		t.Errorf("mensagem deveria citar o invariante: %q", err.Error())
	}
}

// TestActBeforeSeed_Error valida o outro lado do invariante: atuar sem o
// setup (world) → erro. "Setup antes do ato senão erro".
func TestActBeforeSeed_Error(t *testing.T) {
	h := NewHarness("wbs")
	err := h.Act(ctx(), "act", func(context.Context) error { return nil })
	if err == nil {
		t.Fatal("act antes de qualquer seed deveria dar erro")
	}
	if !strings.Contains(err.Error(), "before any world seed") {
		t.Errorf("mensagem deveria citar o setup antes do ato: %q", err.Error())
	}
}

// TestStep_NotReached — (b) falha no passo 1 → o passo 2 NÃO roda e é marcado
// `not-reached` (short-circuit determinístico do replay).
func TestStep_NotReached(t *testing.T) {
	h := NewHarness("wbs")
	if err := h.Seed(ctx(), "w", func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	ran := false
	s1 := h.Step(ctx(), "s1", func(context.Context) error { return errors.New("boom") })
	if s1.Status != evidence.StepFailed || s1.OK {
		t.Fatalf("s1 deveria falhar: %+v", s1)
	}

	s2 := h.Step(ctx(), "s2", func(context.Context) error { ran = true; return nil })
	if ran {
		t.Error("s2 não deveria ter rodado (not-reached)")
	}
	if s2.Status != evidence.StepNotReached {
		t.Errorf("s2 status = %q, esperava not-reached", s2.Status)
	}
	if s2.OK || s2.Ms != 0 {
		t.Errorf("s2 deveria ser not-ok com Ms 0 (não rodou): %+v", s2)
	}
}

// TestStep_Needs_SkipWithReason — (c) pré-requisito não satisfeito → skip com
// reason, em vez de fail (e a função NÃO roda).
func TestStep_Needs_SkipWithReason(t *testing.T) {
	h := NewHarness("wbs")
	if err := h.Seed(ctx(), "w", func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	ran := false
	// `upload` depende de `auth` que nunca rodou → skipped.
	s := h.StepWithNeeds(ctx(), "upload", []string{"auth"}, func(context.Context) error {
		ran = true
		return nil
	})
	if ran {
		t.Error("upload não deveria rodar (need 'auth' não satisfeito)")
	}
	if s.Status != evidence.StepSkipped {
		t.Errorf("status = %q, esperava skipped", s.Status)
	}
	if !strings.Contains(s.Reason, "auth") {
		t.Errorf("reason deveria citar o need: %q", s.Reason)
	}
	if s.OK {
		t.Error("skipped não é ok")
	}

	// Com o need satisfeito, roda e passa.
	authOK := h.Step(ctx(), "auth", func(context.Context) error { return nil })
	if authOK.Status != evidence.StepOK {
		t.Fatalf("auth deveria passar: %+v", authOK)
	}
	upload2 := h.StepWithNeeds(ctx(), "upload", []string{"auth"}, func(context.Context) error { return nil })
	if upload2.Status != evidence.StepOK || !upload2.OK {
		t.Errorf("upload com need satisfeito deveria passar: %+v", upload2)
	}
}

// TestStep_Needs_FailedNeed_HardAborts — se um need RODOU e FALHOU
// (StepFailed), a dependência é `not-reached` (aborto duro do replayer), não
// skip: uma falha de execução é o que encerra a sequência.
func TestStep_Needs_FailedNeed_HardAborts(t *testing.T) {
	h := NewHarness("wbs")
	if err := h.Seed(ctx(), "w", func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	h.Step(ctx(), "auth", func(context.Context) error { return errors.New("nope") })
	s := h.StepWithNeeds(ctx(), "upload", []string{"auth"}, func(context.Context) error { return nil })
	if s.Status != evidence.StepNotReached {
		t.Errorf("status = %q, esperava not-reached (need auth FALHOU = aborto duro)", s.Status)
	}
}

// TestStep_Needs_SoftSkipDoesNotAbort — um skip por need NÃO satisfeito é
// SUAVE: um passo independente subsequente ainda roda (não vira not-reached).
func TestStep_Needs_SoftSkipDoesNotAbort(t *testing.T) {
	h := NewHarness("wbs")
	if err := h.Seed(ctx(), "w", func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	// `upload` dependente de `auth` (que nunca rodou) → skipped.
	if s := h.StepWithNeeds(ctx(), "upload", []string{"auth"}, func(context.Context) error { return nil }); s.Status != evidence.StepSkipped {
		t.Fatalf("upload deveria ser skipped: %+v", s)
	}
	// `deploy` é independente e NÃO está após falha → roda e passa.
	deploy := h.Step(ctx(), "deploy", func(context.Context) error { return nil })
	if deploy.Status != evidence.StepOK {
		t.Errorf("deploy independente deveria rodar após skip suave: %+v", deploy)
	}
}

// TestTrace_Redaction — (d) email/Bearer/senha/token nunca vazam na trilha de
// evidência (nem em Detail nem em Error).
func TestTrace_Redaction(t *testing.T) {
	h := NewHarness("wbs")
	h.Seed(ctx(), "w", func(context.Context) error { return nil })
	h.Step(ctx(), "s1", func(context.Context) error {
		return errors.New("auth failed for dev@cosca.ai Bearer eyJ0eXAiOiJKV1Qi password=secret123")
	})

	for _, e := range h.Entries() {
		for _, leak := range []string{"dev@cosca.ai", "eyJ0eXAiOiJKV1Qi", "secret123"} {
			if strings.Contains(e.Detail, leak) || strings.Contains(e.Error, leak) {
				t.Errorf("segredo %q vazou em entry %d (detail=%q error=%q)", leak, e.Seq, e.Detail, e.Error)
			}
		}
	}
}

// TestTrace_Reproducible — (e) duas execuções com o mesmo script produzem a
// mesma assinatura de replay (ignoram campos temporais) e trilhas ReplayEqual.
func TestTrace_Reproducible(t *testing.T) {
	run := func() *Harness {
		h := NewHarness("wbs")
		_ = h.Seed(ctx(), "w", func(context.Context) error { return nil })
		h.Step(ctx(), "s1", func(context.Context) error {
			if true {
				return errors.New("fail")
			}
			return nil
		})
		h.Step(ctx(), "s2", func(context.Context) error { return nil }) // not-reached
		_ = h.Dispose()
		return h
	}
	a, b := run(), run()
	if a.ReplayHash() != b.ReplayHash() {
		t.Errorf("replay hash deveria ser igual: %s != %s", a.ReplayHash(), b.ReplayHash())
	}
	if !evidence.ReplayEqualTrace(a.Entries(), a.Steps(), b.Entries(), b.Steps()) {
		t.Error("trilhas não são ReplayEqual")
	}
}

// TestDispose_IsolationI7 — (g) o stack de cleanup do seed é liberado no
// dispose em ordem LIFO, MESMO quando o body falha (isolamento por construção).
func TestDispose_IsolationI7(t *testing.T) {
	h := NewHarness("wbs")
	var order []string
	h.DeferCleanup("a", func() error { order = append(order, "a"); return nil })
	_ = h.Seed(ctx(), "w", func(context.Context) error {
		h.DeferCleanup("b", func() error { order = append(order, "b"); return nil })
		return nil
	})
	h.Step(ctx(), "s1", func(context.Context) error { return errors.New("boom") }) // body falha
	if err := h.Dispose(); err != nil {
		t.Fatalf("dispose: %v", err)
	}
	// LIFO: b foi empilhado por último → liberado primeiro.
	if len(order) != 2 || order[0] != "b" || order[1] != "a" {
		t.Errorf("cleanup deveria ser LIFO (b, a), got %v", order)
	}
	// Idempotente: segundo dispose é no-op.
	if err := h.Dispose(); err != nil {
		t.Errorf("dispose duplo não deveria falhar: %v", err)
	}
}

// TestDispose_AfterSeedFailure — mesmo quando o próprio seed falha, o dispose
// libera os recursos já empilhados.
func TestDispose_AfterSeedFailure(t *testing.T) {
	h := NewHarness("wbs")
	cleaned := false
	h.DeferCleanup("tmpdir", func() error { cleaned = true; return nil })
	_ = h.Seed(ctx(), "w", func(context.Context) error { return errors.New("seed boom") })
	_ = h.Dispose()
	if !cleaned {
		t.Error("cleanup deveria rodar mesmo após seed falhar")
	}
}

// TestHarness_TraceID_AndStoreSink — o harness reusa o Trace ID universal do
// substrato internal/trace e o sink append-only é opcional (nil-safe).
func TestHarness_TraceID_AndStoreSink(t *testing.T) {
	h := NewHarness("wbs")
	if _, ok := trace.Parse(h.TraceID().String()); !ok {
		t.Errorf("TraceID %q não é um Trace ID universal", h.TraceID().String())
	}
	if err := h.Seed(ctx(), "w", func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	_ = h.Act(ctx(), "a", func(context.Context) error { return nil })
	_ = h.Dispose()
	if len(h.Entries()) == 0 {
		t.Error("esperava entradas de evidência")
	}
}
