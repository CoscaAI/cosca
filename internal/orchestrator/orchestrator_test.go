package orchestrator

import (
	"errors"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/task"
)

// longArtifacts é o snapshot genérico (Artifacts) de uma posição COMPRADA em
// curso — a tradução genérica que o ADAPTER faria. O teste do orchestrator é
// A-GNÓSTICO de domínio: trabalha só com o mapa de chaves.
func longArtifacts() map[string]string {
	return map[string]string{
		task.ArtifactSymbol: "BTCUSDT",
		task.ArtifactSide:   "buy",
		task.ArtifactOpen:   "true",
		task.ArtifactQty:    "0.5",
		task.ArtifactPrice:  "110",
	}
}

// buyObj é um objetivo de exposição longa (direction=buy) — satisfeito quando
// há posição buy em curso.
func buyObj() task.TaskObjective {
	return task.TaskObjective{ID: "t-1", Objective: "aumentar exposição em BTCUSDT", Symbol: "BTCUSDT", Direction: "buy"}
}

// TestCreateActiveStep0 — a task nasce ACTIVE, CurrentStep=0 (ADR-015).
func TestCreateActiveStep0(t *testing.T) {
	o := New(Config{MaxContinue: 5})
	id, err := o.Create(buyObj(), "bot", "v1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if o.Count() != 1 {
		t.Fatalf("Count=%d, esperava 1", o.Count())
	}
	st, ok := o.Status(id)
	if !ok {
		t.Fatal("task não encontrada")
	}
	if st.Status != task.StatusActive {
		t.Errorf("status=%s, esperava ACTIVE", st.Status)
	}
	if st.CurrentStep != 0 {
		t.Errorf("CurrentStep=%d, esperava 0", st.CurrentStep)
	}
	if st.Agent != "bot" || st.PolicyVersion != "v1" {
		t.Errorf("agent/policy=%q/%q, esperava bot/v1", st.Agent, st.PolicyVersion)
	}
	if task.IsTerminal(st.Status) {
		t.Errorf("ACTIVE não deveria ser terminal")
	}
}

// TestTaskIDUnique — task IDs são únicos (nunca colidem).
func TestTaskIDUnique(t *testing.T) {
	o := New(Config{MaxContinue: 5})
	seen := map[task.TaskID]bool{}
	for i := 0; i < 100; i++ {
		id, err := o.Create(buyObj(), "bot", "v1")
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if seen[id] {
			t.Fatalf("task ID duplicado: %q", id)
		}
		seen[id] = true
	}
}

// TestCreateInvalidObjective — um objetivo malformado é rejeitado sem criar.
func TestCreateInvalidObjective(t *testing.T) {
	o := New(Config{MaxContinue: 5})
	if _, err := o.Create(task.TaskObjective{Objective: "sem id/symbol/direction"}, "bot", "v1"); !errors.Is(err, task.ErrInvalidObjective) {
		t.Fatalf("objetivo inválido deveria ser ErrInvalidObjective, got %v", err)
	}
	if o.Count() != 0 {
		t.Fatalf("Count=%d, esperava 0 (sem task criada)", o.Count())
	}
}

// TestEventsUpdateTaskState — eventos de task (POSITION_CHANGED, INTENT_GENERATED,
// FILL) atualizam o TaskState: snapshot genérico (Artifacts), ações pendentes,
// observações e checkpoint idempotente.
func TestEventsUpdateTaskState(t *testing.T) {
	o := New(Config{MaxContinue: 5})
	id, _ := o.Create(buyObj(), "bot", "v1")
	// CorrelationID amarra os eventos da mesma task (ADR-015).
	corr := string(id)

	o.OnEvent(task.Event{Type: task.EventPositionChanged, Source: "oms", Severity: task.SeverityInfo, CorrelationID: corr,
		Payload: task.PositionChangedPayload{TaskID: id, Artifacts: longArtifacts()}})
	o.OnEvent(task.Event{Type: task.EventIntentGenerated, Source: "intent", CorrelationID: corr,
		Payload: task.IntentGeneratedPayload{TaskID: id, Intent: "place limit 100"}})
	o.OnEvent(task.Event{Type: task.EventFill, Source: "oms", CorrelationID: corr,
		Payload: task.FillPayload{TaskID: id, Artifacts: longArtifacts()}})

	st, _ := o.Status(id)
	// O orquestrador é a-gnóstico de domínio: lê o snapshot genérico (Artifacts).
	if st.Artifacts[task.ArtifactSide] != "buy" {
		t.Errorf("artifacts side deveria ser buy, got %q", st.Artifacts[task.ArtifactSide])
	}
	if st.Artifacts[task.ArtifactOpen] != "true" {
		t.Errorf("artifacts open deveria ser true, got %q", st.Artifacts[task.ArtifactOpen])
	}
	if st.Artifacts[task.ArtifactSymbol] != "BTCUSDT" {
		t.Errorf("artifacts symbol deveria ser BTCUSDT, got %q", st.Artifacts[task.ArtifactSymbol])
	}
	if len(st.PendingActions) != 1 || st.PendingActions[0] != "place limit 100" {
		t.Errorf("pending_actions=%v, esperava [place limit 100]", st.PendingActions)
	}
	if len(st.Observations) < 3 {
		t.Errorf("observations=%v, esperava >=3", st.Observations)
	}
	if !hasCheckpoint(st, "fill") {
		t.Errorf("checkpoint 'fill' deveria existir, got %v", st.Checkpoints)
	}
}

// TestDecideContinueByStepLimit — ao atingir o limite de continuações configurado,
// a decisão vira CONTINUE com STEP_LIMIT (escala p/ o Don, fail-closed — não
// auto-aborta nem auto-completa).
func TestDecideContinueByStepLimit(t *testing.T) {
	o := New(Config{MaxContinue: 1})
	id, _ := o.Create(buyObj(), "bot", "v1") // sem posição → satisfeito = false

	// Antes do limite (step 0 < 1): motivo = objetivo incompleto.
	d1, err := o.Decide(id)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if d1.Decision != DecisionContinue || d1.Reason != task.ContIncompleteObjective {
		t.Fatalf("step0 esperava CONTINUE/INCOMPLETE_OBJECTIVE, got %s/%s", d1.Decision, d1.Reason)
	}
	if err := o.Continue(id, d1.Reason); err != nil {
		t.Fatalf("continue: %v", err)
	}

	// No limite (step 1 >= 1): motivo = STEP_LIMIT, mas CONTINUE (escala ao Don).
	d2, err := o.Decide(id)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if d2.Decision != DecisionContinue || d2.Reason != task.ContStepLimit {
		t.Fatalf("no limite esperava CONTINUE/STEP_LIMIT, got %s/%s", d2.Decision, d2.Reason)
	}
	// Continua com a razão STEP_LIMIT → o estado materializa a escalada.
	if err := o.Continue(id, d2.Reason); err != nil {
		t.Fatalf("continue (step_limit): %v", err)
	}
	st, _ := o.Status(id)
	if st.CurrentStep != 2 {
		t.Errorf("CurrentStep=%d, esperava 2", st.CurrentStep)
	}
	if st.ContinuationReason != task.ContStepLimit {
		t.Errorf("continuation_reason=%s, esperava STEP_LIMIT", st.ContinuationReason)
	}
}

// TestDecideCompleteWhenSatisfied — quando o objetivo está satisfeito (posição em
// curso na direção), a decisão é COMPLETE e o terminal é alcançado.
func TestDecideCompleteWhenSatisfied(t *testing.T) {
	o := New(Config{MaxContinue: 5})
	id, _ := o.Create(buyObj(), "bot", "v1")

	// Snapshot genérico que satisfaz o objetivo (buy em curso → open=true).
	o.OnEvent(task.Event{Type: task.EventPositionChanged, Source: "oms", CorrelationID: string(id),
		Payload: task.PositionChangedPayload{TaskID: id, Artifacts: longArtifacts()}})

	d, err := o.Decide(id)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if d.Decision != DecisionComplete {
		t.Fatalf("objetivo satisfeito deveria ser COMPLETE, got %s", d.Decision)
	}
	if d.Reason != "" {
		t.Errorf("COMPLETE não carrega razão de continuação, got %s", d.Reason)
	}

	if err := o.Complete(id); err != nil {
		t.Fatalf("complete: %v", err)
	}
	st, _ := o.Status(id)
	if st.Status != task.StatusComplete || !task.IsTerminal(st.Status) {
		t.Errorf("esperava COMPLETE terminal, got %s", st.Status)
	}
}

// TestWatchdogMarksAndContinues — um RISK_CHECK crítico marca o watchdog; a
// decisão volta CONTINUE/WATCHDOG (fail-closed) e a task "marca e continua".
func TestWatchdogMarksAndContinues(t *testing.T) {
	o := New(Config{MaxContinue: 3})
	id, _ := o.Create(buyObj(), "bot", "v1")

	o.OnEvent(task.Event{Type: task.EventRiskCheck, Source: "risk", Severity: task.SeverityError, CorrelationID: string(id),
		Payload: task.RiskCheckPayload{TaskID: id, Blocked: true, Reason: "drawdown cruzou o limite"}})

	d, err := o.Decide(id)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if d.Decision != DecisionContinue || d.Reason != task.ContWatchdog {
		t.Fatalf("watchdog esperava CONTINUE/WATCHDOG, got %s/%s", d.Decision, d.Reason)
	}

	if err := o.Continue(id, d.Reason); err != nil {
		t.Fatalf("continue: %v", err)
	}
	st, _ := o.Status(id)
	if st.ContinuationReason != task.ContWatchdog {
		t.Errorf("continuation_reason=%s, esperava WATCHDOG", st.ContinuationReason)
	}
	if len(st.Errors) == 0 {
		t.Errorf("watchdog deveria registrar erro (marca), got errors=%v", st.Errors)
	}
	if st.CurrentStep != 1 {
		t.Errorf("CurrentStep=%d, esperava 1 (continou)", st.CurrentStep)
	}
	if task.IsTerminal(st.Status) {
		t.Errorf("watchdog NÃO é terminal — task continua ativa, got %s", st.Status)
	}
}

// TestMarkWatchdogThenTerminalClears — o watchdog persiste até o terminal (espelho
// do handoff): depois de COMPLETE, Decide volta NOOP (não mais WATCHDOG).
func TestMarkWatchdogThenTerminalClears(t *testing.T) {
	o := New(Config{MaxContinue: 3})
	id, _ := o.Create(buyObj(), "bot", "v1")
	if err := o.MarkWatchdog(id, "ack perdido"); err != nil {
		t.Fatalf("mark watchdog: %v", err)
	}
	if err := o.Complete(id); err != nil {
		t.Fatalf("complete: %v", err)
	}
	d, err := o.Decide(id)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if d.Decision != DecisionNoop {
		t.Fatalf("task terminal deveria ser NOOP, got %s", d.Decision)
	}
}

// TestRetakeIdempotentCheckpoint — retomada idempotente: etapa já em checkpoint
// NÃO é re-executada; Complete/Continue sobre terminal são no-op/erro.
func TestRetakeIdempotentCheckpoint(t *testing.T) {
	o := New(Config{MaxContinue: 5})
	id, _ := o.Create(buyObj(), "bot", "v1")

	added, err := o.AddCheckpoint(id, "step1")
	if err != nil || !added {
		t.Fatalf("AddCheckpoint step1: added=%v err=%v", added, err)
	}
	// Re-add do MESMO checkpoint → não re-executa (idempotente).
	again, err := o.AddCheckpoint(id, "step1")
	if err != nil {
		t.Fatalf("re-add step1: %v", err)
	}
	if again {
		t.Error("re-add do mesmo checkpoint NÃO deveria re-executar (again=true)")
	}
	st, _ := o.Status(id)
	if len(st.Checkpoints) != 1 {
		t.Errorf("Checkpoints=%v, esperava apenas [step1] (não duplica)", st.Checkpoints)
	}

	// Complete é idempotente: chamar duas vezes → sem erro.
	if err := o.Complete(id); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if err := o.Complete(id); err != nil {
		t.Fatalf("complete idempotente: %v", err)
	}
	// Continue sobre terminal → ilegal (não pode continuar task terminada).
	if err := o.Continue(id, task.ContIncompleteObjective); !errors.Is(err, task.ErrIllegalTransition) {
		t.Fatalf("Continue sobre terminal deveria ser ErrIllegalTransition, got %v", err)
	}
}

// TestContinueInvalidReason — razão de continuação desconhecida é rejeitada.
func TestContinueInvalidReason(t *testing.T) {
	o := New(Config{MaxContinue: 5})
	id, _ := o.Create(buyObj(), "bot", "v1")
	if err := o.Continue(id, task.ContinuationReason("INVENTED")); err == nil {
		t.Fatal("razão inválida deveria ser rejeitada")
	}
}

// TestUnknownTask — operações sobre um task_id desconhecido dão ErrUnknownTask.
func TestUnknownTask(t *testing.T) {
	o := New(Config{MaxContinue: 5})
	unknown := task.TaskID("nao-existe")
	if _, err := o.Decide(unknown); !errors.Is(err, ErrUnknownTask) {
		t.Fatalf("Decide desconhecido deveria ser ErrUnknownTask, got %v", err)
	}
	if err := o.Continue(unknown, task.ContIncompleteObjective); !errors.Is(err, ErrUnknownTask) {
		t.Fatalf("Continue desconhecido deveria ser ErrUnknownTask, got %v", err)
	}
	if _, ok := o.Status(unknown); ok {
		t.Fatal("Status desconhecido deveria reportar !ok")
	}
}

// TestCheckWatchdogByDeadline — o dead-man switch marca a task cujo Deadline
// (RFC3339) expirou sem confirmação terminal (relógio injetável).
func TestCheckWatchdogByDeadline(t *testing.T) {
	o := New(Config{MaxContinue: 5})
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	o.now = func() time.Time { return now }

	expired := task.TaskObjective{ID: "t-exp", Objective: "x", Symbol: "BTCUSDT", Direction: "buy",
		Deadline: now.Add(-time.Hour).Format(time.RFC3339)}
	future := task.TaskObjective{ID: "t-fut", Objective: "x", Symbol: "BTCUSDT", Direction: "buy",
		Deadline: now.Add(time.Hour).Format(time.RFC3339)}
	idExp, _ := o.Create(expired, "bot", "v1")
	idFut, _ := o.Create(future, "bot", "v1")

	fired := o.CheckWatchdog()
	if len(fired) != 1 || fired[0] != idExp {
		t.Fatalf("watchdog deveria marcar %s, got %v", idExp, fired)
	}
	if st, _ := o.Status(idFut); st.ContinuationReason == task.ContWatchdog {
		t.Errorf("task com deadline futuro não deveria ter sido marcada")
	}
	// A marcada aceita a decisão WATCHDOG (marca e continua).
	if d, _ := o.Decide(idExp); d.Reason != task.ContWatchdog {
		t.Errorf("task marcada deveria decidir WATCHDOG, got %s", d.Reason)
	}
}

// TestDecideStructured — o DecisionResult é ESTRUTURADO (machine-readable), não
// um enum solto: carrega Continue, CurrentStep, Observations e NextAction
// consistentes com a decisão/motivo (padrão gstack).
func TestDecideStructured(t *testing.T) {
	o := New(Config{MaxContinue: 5})
	id, _ := o.Create(buyObj(), "bot", "v1")

	// Até satisfazer, a decisão é CONTINUE / INCOMPLETE_OBJECTIVE com NextAction
	// "continue_loop", refletindo o CurrentStep corrente e as observações do runtime.
	o.OnEvent(task.Event{Type: task.EventPositionChanged, Source: "oms", CorrelationID: string(id),
		Payload: task.PositionChangedPayload{TaskID: id, Artifacts: longArtifacts()}})
	d, err := o.Decide(id)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if d.Decision != DecisionComplete {
		t.Fatalf("objetivo satisfeito deveria ser COMPLETE, got %s", d.Decision)
	}
	if d.Continue {
		t.Errorf("COMPLETE não deve marcar Continue=true (got %v)", d.Continue)
	}
	if d.Reason != "" {
		t.Errorf("COMPLETE não carrega razão de continuação, got %q", d.Reason)
	}
	if d.NextAction != "complete_task" {
		t.Errorf("NextAction deveria ser complete_task, got %q", d.NextAction)
	}
	if d.CurrentStep != 0 {
		t.Errorf("CurrentStep deveria ser 0, got %d", d.CurrentStep)
	}
	if len(d.Observations) == 0 {
		t.Errorf("DecisionResult deveria carregar as observações do runtime, got %v", d.Observations)
	}

	// Antes de satisfazer, o enum + NextAction precisam ser consistentes:
	// INCOMPLETE_OBJECTIVE → continue_loop (o Kernel retoma, não martela).
	o2 := New(Config{MaxContinue: 5})
	id2, _ := o2.Create(buyObj(), "bot", "v1")
	d2, err := o2.Decide(id2)
	if err != nil {
		t.Fatalf("decide2: %v", err)
	}
	if d2.Decision != DecisionContinue || !d2.Continue {
		t.Fatalf("objetivo incompleto deveria ser CONTINUE/Continue=true, got %s/%v", d2.Decision, d2.Continue)
	}
	if d2.Reason != task.ContIncompleteObjective {
		t.Errorf("motivo deveria ser INCOMPLETE_OBJECTIVE, got %q", d2.Reason)
	}
	if d2.NextAction != "continue_loop" {
		t.Errorf("NextAction deveria ser continue_loop, got %q", d2.NextAction)
	}
}

func hasCheckpoint(st task.TaskState, cp string) bool {
	for _, c := range st.Checkpoints {
		if c == cp {
			return true
		}
	}
	return false
}
