package orchestrator

// pending_test.go — Pending Resolution no TaskOrchestrator (decisão do Don +
// professor, 2026-09-01): quando o limite de steps atinge, o orquestrador
// inspeciona o ESTADO e continua SÓ para terminar a pendência implicada, em
// vez de escalar direto. Nunca inventa próximo passo.

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/task"
)

// seedPendingResolvable monta uma task no limite de steps com uma pendência
// resolvível: observação existe (via EventTaskStarted) + ação de persistir
// (via EventIntentGenerated).
func seedPendingResolvable(t *testing.T, o *TaskOrchestrator) task.TaskID {
	t.Helper()
	id, _ := o.Create(buyObj(), "bot", "v1")

	// Avança até o limite (1 step).
	d1, err := o.Decide(id)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if err := o.Continue(id, d1.Reason); err != nil {
		t.Fatalf("continue: %v", err)
	}

	// Pendência resolvível: observação (task.started) + ação de persistir.
	o.OnEvent(task.Event{
		Type:          task.EventIntentGenerated,
		Payload:       task.IntentGeneratedPayload{Intent: "persist execution result"},
		CorrelationID: string(id),
	})
	return id
}

// TestDecidePendingResolved_ObservationPersist: com a Pending Resolution
// ligada, um STEP_LIMIT com pendência "persistir observação" (observação já
// existe no estado) vira PENDING_RESOLVED — continuação mínima, não escala.
func TestDecidePendingResolved_ObservationPersist(t *testing.T) {
	o := New(Config{MaxContinue: 1}, WithPendingResolver(2))
	id := seedPendingResolvable(t, o)

	d2, err := o.Decide(id)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if d2.Decision != DecisionContinue || d2.Reason != task.ContPendingResolved {
		t.Fatalf("no limite com pendência resolvível esperava CONTINUE/PENDING_RESOLVED, got %s/%s", d2.Decision, d2.Reason)
	}
	if d2.NextAction != "resolve_pending" {
		t.Fatalf("next_action = %q, esperava resolve_pending", d2.NextAction)
	}
}

// TestDecidePendingResolved_SemResolver_Escalates: SEM a Pending Resolution
// ligada, o STEP_LIMIT escala como antes (sem regressão).
func TestDecidePendingResolved_SemResolver_Escalates(t *testing.T) {
	o := New(Config{MaxContinue: 1}) // sem WithPendingResolver
	id := seedPendingResolvable(t, o)

	d2, err := o.Decide(id)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if d2.Reason != task.ContStepLimit {
		t.Fatalf("sem resolver, o limite deveria escalar STEP_LIMIT, got %s", d2.Reason)
	}
}

// TestDecidePendingResolved_RecoveryLimit: o teto próprio de recuperação
// impede o loop — depois de N resoluções, volta a escalar (fail-closed).
func TestDecidePendingResolved_RecoveryLimit(t *testing.T) {
	o := New(Config{MaxContinue: 1}, WithPendingResolver(1)) // recovery_steps = 1
	id := seedPendingResolvable(t, o)

	// 1ª vez no limite: pendência resolvível → PENDING_RESOLVED.
	d2, _ := o.Decide(id)
	if d2.Reason != task.ContPendingResolved {
		t.Fatalf("1ª recuperação esperava PENDING_RESOLVED, got %s", d2.Reason)
	}

	// 2ª vez no limite: recuperação esgotada → volta a escalar (STEP_LIMIT).
	if err := o.Continue(id, d2.Reason); err != nil {
		t.Fatalf("continue: %v", err)
	}
	d3, _ := o.Decide(id)
	if d3.Reason != task.ContStepLimit {
		t.Fatalf("recuperação esgotada deveria escalar STEP_LIMIT, got %s", d3.Reason)
	}
}
