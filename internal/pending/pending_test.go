package pending

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/task"
)

// TestInspect_NoPending_Finalizes: sem pendências, a task finaliza limpa
// (NothingPending — não inventa trabalho).
func TestInspect_NoPending_Finalizes(t *testing.T) {
	r := New(2)
	res := r.Inspect(State{CurrentStep: 20, MaxSteps: 20})
	if res.Verdict != NothingPending {
		t.Fatalf("verdict = %s, esperava NOTHING_PENDING", res.Verdict)
	}
}

// TestInspect_ObservationPersist_Resolves: a pendência "registrar observação"
// é resolvível quando a observação JÁ existe no estado (ação implicada).
func TestInspect_ObservationPersist_Resolves(t *testing.T) {
	r := New(2)
	res := r.Inspect(State{
		CurrentStep:    20,
		MaxSteps:       20,
		PendingActions: []string{"persist execution result"},
		Observations:   []string{"teste passou com 12/12"},
	})
	if res.Verdict != Resolve {
		t.Fatalf("verdict = %s, esperava RESOLVE", res.Verdict)
	}
	if res.Action != "persist execution result" {
		t.Fatalf("action = %q, esperava a pendência implicada", res.Action)
	}
	if r.RecoveryRemaining() != 1 {
		t.Fatalf("recovery remaining = %d, esperava 1 (gastou 1)", r.RecoveryRemaining())
	}
}

// TestInspect_ObservationPersist_SemObservacao_NaoInventa: a pendência
// "persistir" SEM observação no estado NÃO é resolvível — não inventa o que
// persistir.
func TestInspect_ObservationPersist_SemObservacao_NaoInventa(t *testing.T) {
	r := New(2)
	res := r.Inspect(State{
		CurrentStep:    20,
		MaxSteps:       20,
		PendingActions: []string{"persist execution result"},
		// sem Observations — o estado não prova o que persistir
	})
	if res.Verdict != CannotResolve {
		t.Fatalf("verdict = %s, esperava CANNOT_RESOLVE (não inventa)", res.Verdict)
	}
}

// TestInspect_CheckpointConfirm_Resolves: pendência "confirmar etapa" é
// resolvível quando a etapa já consta nos checkpoints (idempotente).
func TestInspect_CheckpointConfirm_Resolves(t *testing.T) {
	r := New(2)
	res := r.Inspect(State{
		CurrentStep:    20,
		MaxSteps:       20,
		PendingActions: []string{"confirm checkpoint step-9"},
		Checkpoints:    []string{"step-9"},
	})
	if res.Verdict != Resolve {
		t.Fatalf("verdict = %s, esperava RESOLVE", res.Verdict)
	}
}

// TestInspect_PendenciaDesconhecida_Escalates: pendência que o estado não
// explica → CannotResolve (finalizar/escalar — NUNCA inventa próximo passo).
func TestInspect_PendenciaDesconhecida_Escalates(t *testing.T) {
	r := New(2)
	res := r.Inspect(State{
		CurrentStep:    20,
		MaxSteps:       20,
		PendingActions: []string{"do something creative"},
		Observations:   []string{"obs"},
		Checkpoints:    []string{"step-1"},
	})
	if res.Verdict != CannotResolve {
		t.Fatalf("verdict = %s, esperava CANNOT_RESOLVE (pendência desconhecida)", res.Verdict)
	}
}

// TestInspect_RecoveryLimit_StopLoop: o limite próprio de recuperação impede
// o loop (o problema do Trader — eventos gerando Continue indefinidamente).
func TestInspect_RecoveryLimit_StopLoop(t *testing.T) {
	r := New(2) // recovery_steps = 2
	st := State{
		CurrentStep:    20,
		MaxSteps:       20,
		PendingActions: []string{"persist execution result"},
		Observations:   []string{"obs"},
	}

	// 2 recuperações resolvidas...
	if r.Inspect(st).Verdict != Resolve {
		t.Fatal("recuperação 1 deveria resolver")
	}
	if r.Inspect(st).Verdict != Resolve {
		t.Fatal("recuperação 2 deveria resolver")
	}
	// ...a 3ª é bloqueada (loop-safe).
	if res := r.Inspect(st); res.Verdict != CannotResolve {
		t.Fatalf("verdict = %s, esperava CANNOT_RESOLVE (limite de recuperação esgotado)", res.Verdict)
	}
}

// TestFromTaskState_Projects: a projeção do TaskState preserva os campos.
func TestFromTaskState_Projects(t *testing.T) {
	st := &task.TaskState{
		CurrentStep:    20,
		PendingActions: []string{"persist result"},
		Observations:   []string{"obs-1"},
		Checkpoints:    []string{"step-1"},
	}
	proj := FromTaskState(st, 20)
	if len(proj.PendingActions) != 1 || proj.Observations[0] != "obs-1" {
		t.Fatalf("projeção errada: %+v", proj)
	}
	if proj.MaxSteps != 20 || proj.CurrentStep != 20 {
		t.Fatalf("projeção de steps errada: %+v", proj)
	}
}

// TestInspect_NilState_Safe: estado vazio → NothingPending (sem panic).
func TestInspect_NilState_Safe(t *testing.T) {
	r := New(2)
	if res := r.Inspect(State{}); res.Verdict != NothingPending {
		t.Fatalf("verdict = %s, esperava NOTHING_PENDING para estado vazio", res.Verdict)
	}
}
