// Package orchestrator implementa o TaskOrchestrator (Task Manager) — o núcleo
// do Task Continuation Loop (ADR-015, F1). Ele mantém o TaskState de cada task
// em memória (mapa + mutex), CONSUME eventos de task (via OnEvent) atualizando
// o estado, e DECIDE CONTINUE | COMPLETE com base em motivo cognitivo:
//
//	INCOMPLETE_OBJECTIVE — objetivo ainda não satisfeito;
//	STEP_LIMIT           — limite de continuações atingido (escala p/ o Don);
//	INPUT_REQUIRED       — a task aguarda insumo externo;
//	WATCHDOG             — dead-man switch marcado (erro/risco) — marca e continua.
//
// O Kernel NÃO martela operação a operação: ele só volta quando há um destes
// motivos. Entre eles, a task roda de forma autônoma no data plane. A interface
// entre os planes é o TaskState + eventos — NUNCA uma chamada direta.
//
// F1 é SEM persistência (tudo em memória, eventos simulados) e ADITIVO: não
// altera o comportamento do Root existente. A persistência é um contrato
// (`task.TaskRepository`) injetado via `WithRepository` — a implementação
// concreta é um ADAPTER que pode reusar `internal/durable` ou
// `internal/pipeline` (reuso da borda, não da primitiva — ver ADR-015).
//
// A PRIMITIVA é NEUTRA DE DOMÍNIO e stdlib-only: NÃO importa pacotes de domínio
// (workers/editors/desktop/providers/...). Só importa o contrato `task`.
package orchestrator

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CoscaAI/cosca/internal/pending"
	"github.com/CoscaAI/cosca/internal/task"
)

// Decision é a saída do loop cognitivo: CONTINUE (retomar com novo prompt) |
// COMPLETE (objetivo satisfeito) | NOOP (task já terminal — nada a decidir).
type Decision string

const (
	DecisionContinue Decision = "CONTINUE"
	DecisionComplete Decision = "COMPLETE"
	DecisionNoop     Decision = "NOOP"
)

// DecisionResult é a decisão + o motivo cognitivo que a justificou.
//
// F3: passa a ser ESTRUTURADO e machine-readable (padrão gstack — campos
// legíveis, não prosa). O `Decision` (enum) continua para quem quer o atalho;
// os campos adicionais dão o resultado parseável sem depender de string:
//
//	Continue     — true se o loop cognitivo DEVE seguir (Decision==CONTINUE);
//	Reason       — o motivo cognitivo (ContinuationReason), vazio em COMPLETE;
//	CurrentStep  — o passo corrente da task (paridade com o teto de continuações);
//	Observations — as leituras/evidências do runtime no momento do veredito;
//	NextAction   — a ação sugerida (continue_loop|complete_task|await_input|…).
type DecisionResult struct {
	Decision     Decision                `json:"decision"`
	Continue     bool                    `json:"continue"`
	Reason       task.ContinuationReason `json:"reason,omitempty"`
	CurrentStep  int                     `json:"current_step"`
	Observations []string                `json:"observations"`
	NextAction   string                  `json:"next_action"`
}

// Erros do orquestrador.
var (
	// ErrUnknownTask é um task_id que não tem TaskState registrado.
	ErrUnknownTask = errors.New("task desconhecida")
	// ErrTaskTerminal é uma operação de ciclo de vida sobre uma task terminal.
	ErrTaskTerminal = errors.New("task terminal")
)

// Config parametriza o orquestrador.
type Config struct {
	// MaxContinue é o número máximo de continuações sem o Don
	// (COSCA_TASK_MAX_CONTINUE, default 5). Ao atingir, a decisão vira
	// STEP_LIMIT e escala para o Don (ABORT não automático — fail-closed).
	MaxContinue int
	// Now é a fonte de tempo (injetável p/ testes determinísticos do watchdog).
	Now func() time.Time
}

// Option configura o orquestrador no New.
type Option func(*TaskOrchestrator)

// WithPendingResolver liga a Pending Resolution (decisão do Don + professor,
// 2026-09-01): quando o limite de steps atinge, o orquestrador inspeciona o
// ESTADO e executa a continuação mínima implicada (pendência resolvível) em
// vez de escalar direto. recoverySteps é o teto próprio de recuperações
// (default 2 — nunca vira loop).
func WithPendingResolver(recoverySteps int) Option {
	return func(o *TaskOrchestrator) {
		o.pending = pending.New(recoverySteps)
	}
}

// WithObjectiveSatisfied injeta o predicado que decide se o objetivo foi
// satisfeito. Default: snapshot do data plane indicando o estado "em curso"
// (ArtifactOpen == "true") na direção do objetivo (F1 simples); em F4 o
// Policy/Risk controller assume.
func WithObjectiveSatisfied(fn func(*task.TaskState) bool) Option {
	return func(o *TaskOrchestrator) { o.satisfied = fn }
}

// WithEmitter injeta um emissor de evento (F3: publica TASK_CONTINUE no bus ao
// continuar). Nil (default) = não publica — F1 é consumidor/decisor.
func WithEmitter(fn func(task.Event) error) Option {
	return func(o *TaskOrchestrator) { o.emit = fn }
}

// WithRepository injeta um TaskRepository (F2, ADR-015): quando não-nil, cada
// mutação relevante (Create/Continue/Complete/Abort/AddCheckpoint/OnEvent,
// além de MarkWatchdog/CheckWatchdog) é persistida FORA do lock — a lição da F3
// (nunca segurar o mutex em I/O, nem emitir evento com o lock retido). No New,
// re-hidrata as tasks NÃO-terminais do repositório (retomada idempotente após
// restart: a task continua de onde parou, etapa já em checkpoint não re-executa).
// Nil (default) = em memória, comportamento da F1 preservado.
func WithRepository(repo task.TaskRepository) Option {
	return func(o *TaskOrchestrator) { o.repo = repo }
}

// internalTask é o registro interno (estado publicado + bookkeeping de watchdog,
// que NÃO é publicado no TaskState porque o ADR não o modela explícito — o
// watchdog é o sinal cognitivo que Decide lê, limpo apenas no terminal).
type internalTask struct {
	state    task.TaskState
	watchdog bool // dead-man switch: marcado até o estado terminal (espelho do handoff)
}

// TaskOrchestrator é o Task Manager (thread-safe).
type TaskOrchestrator struct {
	mu          sync.RWMutex
	tasks       map[task.TaskID]*internalTask
	maxContinue int
	now         func() time.Time
	satisfied   func(*task.TaskState) bool
	emit        func(task.Event) error
	repo        task.TaskRepository // F2: persistência durável (nil = em memória)
	// pending é a Pending Resolution (decisão do Don + professor 2026-09-01):
	// quando o limite de steps atinge, inspeciona o ESTADO e executa a
	// continuação MÍNIMA implicada (se existir pendência resolvível) antes de
	// escalar. Nil = comportamento atual (STEP_LIMIT escala direto).
	pending *pending.Resolver
}

// New cria o orquestrador com o limite padrão de continuações.
func New(cfg Config, opts ...Option) *TaskOrchestrator {
	if cfg.MaxContinue <= 0 {
		cfg.MaxContinue = defaultMaxContinue()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	o := &TaskOrchestrator{
		tasks:       make(map[task.TaskID]*internalTask),
		maxContinue: cfg.MaxContinue,
		now:         cfg.Now,
		satisfied:   defaultObjectiveSatisfied,
	}
	for _, opt := range opts {
		opt(o)
	}
	if o.satisfied == nil {
		o.satisfied = defaultObjectiveSatisfied
	}
	// F2 — retomada idempotente: re-hidrata as tasks NÃO-terminais do repositório
	// para que a task continue de onde parou após um restart (status, checkpoints,
	// current_step e o snapshot do data plane restaurados — etapa já concluída não
	// re-executa). Falha do repositório aqui degrada para em memória (fail-safe).
	o.rehydrate()
	return o
}

// defaultMaxContinue lê COSCA_TASK_MAX_CONTINUE (default 5) — decisão do Don:
// máximo de 5 continuações sem o Don; ao atingir, escala (não auto-aborta).
func defaultMaxContinue() int {
	const def = 5
	if raw := os.Getenv("COSCA_TASK_MAX_CONTINUE"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// defaultObjectiveSatisfied: o objetivo é satisfeito quando o snapshot genérico
// do data plane indica que o estado está "em curso" (ArtifactOpen == "true") na
// direção pedida. É um FALLBACK neutro/agnóstico: o pacote `orchestrator` NÃO
// conhece domínio — ele lê o contrato de chaves dos Artifacts. Para o ciclo
// completo, o ADAPTER injeta o predicado específico via WithObjectiveSatisfied.
func defaultObjectiveSatisfied(st *task.TaskState) bool {
	if st == nil {
		return false
	}
	return st.Artifacts[task.ArtifactOpen] == "true" &&
		st.Artifacts[task.ArtifactSide] == st.Objective.Direction
}

// rehydrate restaura as tasks NÃO-terminais do repositório no mapa (F2, retomada
// idempotente). Só restaura estados que ainda têm trabalho a fazer
// (ACTIVE/WAITING/PAUSED): uma task terminal (COMPLETE/ABORTED) é um estado de
// saída e não retoma. Chama List() SEM lock (o repo tem seu próprio mutex) —
// nunca I/O sob o o.mu (lição F3).
func (o *TaskOrchestrator) rehydrate() {
	if o.repo == nil {
		return
	}
	states, err := o.repo.List()
	if err != nil {
		log.Printf("orchestrator: retomada do repositório falhou (%v) — rodando em memória (fail-safe)", err)
		return
	}
	restored := 0
	for _, st := range states {
		if st == nil || task.IsTerminal(st.Status) {
			continue // só restaura não-terminais
		}
		clone := st.Clone()
		o.tasks[clone.TaskID] = &internalTask{state: clone}
		restored++
	}
	if restored > 0 {
		log.Printf("orchestrator: %d task(s) não-terminal(is) retomada(s) do repositório", restored)
	}
}

// persist grava um TaskState no repositório FORA do lock (lição F3: nunca
// segurar o mutex em I/O). Falha de persistência NÃO derruba o sistema — o
// estado segue em memória (fail-safe) e o erro é logado.
func (o *TaskOrchestrator) persist(st *task.TaskState) {
	if o.repo == nil || st == nil {
		return
	}
	if err := o.repo.Save(st); err != nil {
		log.Printf("orchestrator: persistir task %q falhou (%v) — mantendo em memória (fail-safe)", st.TaskID, err)
	}
}

// Create nasce a task em ACTIVE, CurrentStep=0. Valida o objetivo; a task ID é
// única e vira o CorrelationID dos eventos da task.
func (o *TaskOrchestrator) Create(obj task.TaskObjective, agent, policyVersion string) (task.TaskID, error) {
	if err := task.ValidateObjective(obj); err != nil {
		return "", err
	}
	o.mu.Lock()
	id := task.TaskID(newTaskID())
	st := task.TaskState{
		TaskID:        id,
		Objective:     obj,
		Status:        task.StatusActive,
		CurrentStep:   0,
		Agent:         agent,
		PolicyVersion: policyVersion,
		Checkpoints:   []string{},
	}
	o.tasks[id] = &internalTask{state: st}
	o.mu.Unlock()
	// F2: persiste FORA do lock (lição F3: nunca I/O sob o mutex).
	o.persist(&st)
	return id, nil
}

// OnEvent consome um evento de task e atualiza o TaskState. O CorrelationID (ou
// um campo task_id no payload) amarra o evento à task — ADR-015: "reuso do
// CorrelationID/CausationID para amarrar os eventos de uma mesma task". Eventos
// de task: TASK_STARTED, POSITION_CHANGED, INTENT_GENERATED, RISK_CHECK,
// EXECUTION, FILL, STATE_CHANGED, TASK_CONTINUE.
func (o *TaskOrchestrator) OnEvent(ev task.Event) {
	id := taskIDFrom(ev)
	if id == "" {
		return
	}
	o.mu.Lock()
	it, ok := o.tasks[id]
	if !ok {
		o.mu.Unlock()
		return // task não criada por este orquestrador — Create é o ponto de entrada
	}
	st := &it.state
	switch ev.Type {
	case task.EventTaskStarted:
		if st.Status != task.StatusActive {
			st.Status = task.StatusActive
			st.Observations = append(st.Observations, "task.started")
		}
	case task.EventPositionChanged:
		updateArtifacts(st, ev.Payload)
	case task.EventFill:
		if updateArtifacts(st, ev.Payload) {
			// checkpoint idempotente: um fill já registrado como etapa não é re-executado.
			addCheckpoint(st, "fill")
		}
	case task.EventIntentGenerated:
		if p, ok := ev.Payload.(task.IntentGeneratedPayload); ok && p.Intent != "" {
			st.PendingActions = append(st.PendingActions, p.Intent)
			st.Observations = append(st.Observations, "intent: "+p.Intent)
		}
	case task.EventRiskCheck:
		handleRiskCheck(it, ev)
	case task.EventExecution:
		if p, ok := ev.Payload.(task.ExecutionPayload); ok && p.Detail != "" {
			st.Observations = append(st.Observations, "exec: "+p.Detail)
		}
	case task.EventStateChanged:
		if p, ok := ev.Payload.(task.StateChangedPayload); ok && p.Status != "" {
			if err := task.LegalTransition(st.Status, p.Status); err == nil {
				st.Status = p.Status
				st.Observations = append(st.Observations, "state: "+string(p.Status))
			}
		}
	case task.EventTaskContinue:
		if p, ok := ev.Payload.(task.ContinuePayload); ok && p.Reason != "" {
			st.ContinuationReason = p.Reason
		}
	}
	// F2: persiste FORA do lock (lição F3: nunca I/O sob o mutex). A snapshot é
	// copiada antes de liberar o lock — a persistência vê um estado consistente.
	snap := it.state.Clone()
	o.mu.Unlock()
	o.persist(&snap)
}

// Decide devolve CONTINUE | COMPLETE | NOOP com o motivo cognitivo, SEM mutar o
// estado (puro de leitura). Ordem de precedência (fail-closed):
//
//	terminal → NOOP; watchdog → CONTINUE WATCHDOG; objetivo satisfeito →
//	COMPLETE; limite de passos → CONTINUE STEP_LIMIT; insumo → CONTINUE
//	INPUT_REQUIRED; senão → CONTINUE INCOMPLETE_OBJECTIVE.
//
// O watchdog PRECEDE o "objetivo satisfeito": com o deadman marcado, o Control
// Plane não confia no sucesso/testemunha (fail-closed) — marca e continua.
func (o *TaskOrchestrator) Decide(id task.TaskID) (DecisionResult, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	it, ok := o.tasks[id]
	if !ok {
		return DecisionResult{}, ErrUnknownTask
	}
	st := &it.state
	switch {
	case task.IsTerminal(st.Status):
		return o.makeResult(st, DecisionNoop, ""), nil
	case it.watchdog:
		return o.makeResult(st, DecisionContinue, task.ContWatchdog), nil
	case o.satisfied(st):
		return o.makeResult(st, DecisionComplete, ""), nil
	case st.CurrentStep >= o.maxContinue:
		// Pending Resolution (Don + professor, 2026-09-01): antes de escalar
		// por limite de steps, inspeciona o ESTADO — se existe uma pendência
		// resolvível (ação já implicada: persistir observação, confirmar
		// checkpoint), devolve a continuação MÍNIMA com motivo próprio, em vez
		// de escalar. Nunca inventa: sem ação determinística derivável →
		// STEP_LIMIT (escala para o Don, fail-closed). O limite próprio de
		// recuperação impede o loop.
		if o.pending != nil {
			if pres := o.pending.Inspect(pending.FromTaskState(st, o.maxContinue)); pres.Verdict == pending.Resolve {
				return o.makeResult(st, DecisionContinue, task.ContPendingResolved), nil
			}
		}
		return o.makeResult(st, DecisionContinue, task.ContStepLimit), nil
	case st.Status == task.StatusWaiting:
		return o.makeResult(st, DecisionContinue, task.ContInputRequired), nil
	default:
		return o.makeResult(st, DecisionContinue, task.ContIncompleteObjective), nil
	}
}

// makeResult monta o DecisionResult estruturado a partir do estado (leitura).
// Puro: chamado sob o RLock do Decide; não muta nada.
func (o *TaskOrchestrator) makeResult(st *task.TaskState, dec Decision, reason task.ContinuationReason) DecisionResult {
	return DecisionResult{
		Decision:     dec,
		Continue:     dec == DecisionContinue,
		Reason:       reason,
		CurrentStep:  st.CurrentStep,
		Observations: append([]string(nil), st.Observations...),
		NextAction:   nextAction(dec, reason),
	}
}

// nextAction traduz a decisão + motivo numa AÇÃO legível/machine-readable
// (padrão gstack): quem consome o resultado sabe o que fazer sem interpretar
// prosa. Retornos possíveis: none | complete_task | continue_loop | await_input
// | escalate_to_don | handle_watchdog.
func nextAction(dec Decision, reason task.ContinuationReason) string {
	switch {
	case dec == DecisionNoop:
		return "none"
	case dec == DecisionComplete:
		return "complete_task"
	case reason == task.ContStepLimit:
		return "escalate_to_don" // teto de continuações → escala p/ o Don
	case reason == task.ContPendingResolved:
		return "resolve_pending" // continuação mínima para terminar pendência implicada
	case reason == task.ContInputRequired:
		return "await_input"
	case reason == task.ContWatchdog:
		return "handle_watchdog" // marca e continua (fail-closed)
	default:
		return "continue_loop"
	}
}

// Continue retoma a task (novo prompt de continuação): incrementa CurrentStep,
// registra a razão cognitiva e atualiza o status (WAITING p/ INPUT_REQUIRED,
// senão ACTIVE). NÃO limpa o watchdog — só o estado terminal limpa (espelho do
// handoff). Razão inválida → erro.
//
// Publica TASK_CONTINUE no bus quando um emitter foi injetado (F3).
func (o *TaskOrchestrator) Continue(id task.TaskID, reason task.ContinuationReason) error {
	if !reason.Valid() {
		return fmt.Errorf("razão de continuação inválida: %q", reason)
	}
	o.mu.Lock()
	it, ok := o.tasks[id]
	if !ok {
		o.mu.Unlock()
		return ErrUnknownTask
	}
	st := &it.state
	if task.IsTerminal(st.Status) {
		o.mu.Unlock()
		return errors.Join(task.ErrIllegalTransition, ErrTaskTerminal)
	}
	st.CurrentStep++
	st.ContinuationReason = reason
	if reason == task.ContInputRequired {
		st.Status = task.StatusWaiting
	} else {
		st.Status = task.StatusActive
	}
	st.Observations = append(st.Observations, fmt.Sprintf("continue: step=%d reason=%s", st.CurrentStep, reason))
	snap := it.state.Clone()
	o.mu.Unlock()

	// F2: persiste FORA do lock. Nunca I/O sob o mutex (lição F3).
	o.persist(&snap)

	// Publica TASK_CONTINUE FORA do lock. Emitir com o lock retido causa DEADLOCK
	// por reentrância no bus SÍNCRONO da F3: o emit volta a OnEvent, que tenta
	// re-adquirir o mesmo mutex. O padrão é o mesmo de qualquer runtime: mutar
	// sob lock, emitir após liberar.
	if o.emit != nil {
		_ = o.emit(task.Event{
			Type:          task.EventTaskContinue,
			Source:        "orchestrator",
			Severity:      task.SeverityInfo,
			CorrelationID: string(id),
			Payload:       task.ContinuePayload{TaskID: id, Reason: reason},
		})
	}
	return nil
}

// Complete leva a task ao estado terminal COMPLETE (objetivo satisfeito). Limpa o
// watchdog (estado de saída). Idempotente: já COMPLETE → no-op.
func (o *TaskOrchestrator) Complete(id task.TaskID) error {
	return o.terminate(id, task.StatusComplete)
}

// Abort leva a task ao estado terminal ABORTED. Limpa o watchdog. Idempotente.
func (o *TaskOrchestrator) Abort(id task.TaskID) error {
	return o.terminate(id, task.StatusAborted)
}

// terminate aplica um estado terminal com validação de transição (ilegal → erro).
func (o *TaskOrchestrator) terminate(id task.TaskID, to task.TaskStatus) error {
	o.mu.Lock()
	it, ok := o.tasks[id]
	if !ok {
		o.mu.Unlock()
		return ErrUnknownTask
	}
	st := &it.state
	if st.Status == to {
		o.mu.Unlock()
		return nil // idempotente
	}
	if err := task.LegalTransition(st.Status, to); err != nil {
		o.mu.Unlock()
		return err
	}
	st.Status = to
	it.watchdog = false // terminal limpa o watchdog (espelho do handoff)
	st.Observations = append(st.Observations, "terminal: "+string(to))
	snap := it.state.Clone()
	o.mu.Unlock()
	o.persist(&snap)
	return nil
}

// Status devolve uma cópia do TaskState de uma task.
func (o *TaskOrchestrator) Status(id task.TaskID) (task.TaskState, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	it, ok := o.tasks[id]
	if !ok {
		return task.TaskState{}, false
	}
	return it.state.Clone(), true
}

// List devolve todas as tasks (cópia) — visão do Kernel (GET /task).
func (o *TaskOrchestrator) List() []task.TaskState {
	o.mu.RLock()
	defer o.mu.RUnlock()
	out := make([]task.TaskState, 0, len(o.tasks))
	for _, it := range o.tasks {
		out = append(out, it.state.Clone())
	}
	return out
}

// Count devolve o número de tasks rastreadas.
func (o *TaskOrchestrator) Count() int {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return len(o.tasks)
}

// AddCheckpoint registra uma etapa concluída (idempotência da retomada): etapa já
// em checkpoint NÃO é re-executada. Devolve true se foi um NOVO checkpoint.
func (o *TaskOrchestrator) AddCheckpoint(id task.TaskID, cp string) (bool, error) {
	o.mu.Lock()
	it, ok := o.tasks[id]
	if !ok {
		o.mu.Unlock()
		return false, ErrUnknownTask
	}
	if task.IsTerminal(it.state.Status) {
		o.mu.Unlock()
		return false, errors.Join(ErrTaskTerminal, fmt.Errorf("task %q terminal (%s)", id, it.state.Status))
	}
	added := addCheckpoint(&it.state, cp)
	if !added {
		o.mu.Unlock()
		return false, nil // já registrado — não re-executa (idempotente)
	}
	snap := it.state.Clone()
	o.mu.Unlock()
	o.persist(&snap)
	return true, nil
}

// MarkWatchdog marca explicitamente o dead-man switch de uma task (fail-closed):
// o Control Plane passa a não confiar no sucesso até o terminal.
func (o *TaskOrchestrator) MarkWatchdog(id task.TaskID, reason string) error {
	o.mu.Lock()
	it, ok := o.tasks[id]
	if !ok {
		o.mu.Unlock()
		return ErrUnknownTask
	}
	it.watchdog = true
	it.state.Errors = append(it.state.Errors, "watchdog: "+reason)
	it.state.Observations = append(it.state.Observations, "watchdog: "+reason)
	snap := it.state.Clone()
	o.mu.Unlock()
	o.persist(&snap)
	return nil
}

// CheckWatchdog é o DEAD-MAN SWITCH (espelho do handoff): marca como watchdog as
// tasks não-terminais cujo Deadline (se parseável como RFC3339) já expirou sem
// confirmação terminal. Devolve os IDs marcados. Em F1, além do deadline o
// watchdog também é marcado por evento de risco/erro (OnEvent/MarkWatchdog).
func (o *TaskOrchestrator) CheckWatchdog() []task.TaskID {
	o.mu.Lock()
	now := o.now()
	var fired []task.TaskID
	for id, it := range o.tasks {
		if task.IsTerminal(it.state.Status) || it.watchdog {
			continue
		}
		if it.state.Objective.Deadline == "" {
			continue
		}
		dl, err := time.Parse(time.RFC3339, it.state.Objective.Deadline)
		if err != nil {
			continue // deadline não interpretável — F2 adota formato canônico
		}
		if now.After(dl) {
			it.watchdog = true
			it.state.Errors = append(it.state.Errors, "watchdog: deadline expirou sem confirmação terminal")
			fired = append(fired, id)
		}
	}
	// Copia as snapshots das tasks marcadas ANTES de liberar o lock; persiste
	// depois (lição F3: nunca I/O sob o mutex).
	snaps := make([]task.TaskState, 0, len(fired))
	for _, id := range fired {
		if it, ok := o.tasks[id]; ok {
			snaps = append(snaps, it.state.Clone())
		}
	}
	o.mu.Unlock()
	for i := range snaps {
		o.persist(&snaps[i])
	}
	return fired
}

// ── helpers ────────────────────────────────────────────────────────────────

// taskIDFrom extrai o TaskID de um evento: primeiro o CorrelationID (a amarração
// canônica da ADR-015), senão um campo task_id no payload tipado.
func taskIDFrom(ev task.Event) task.TaskID {
	if ev.CorrelationID != "" {
		return task.TaskID(ev.CorrelationID)
	}
	if ev.Payload == nil {
		return ""
	}
	switch p := ev.Payload.(type) {
	case task.PositionChangedPayload:
		return p.TaskID
	case task.FillPayload:
		return p.TaskID
	case task.IntentGeneratedPayload:
		return p.TaskID
	case task.RiskCheckPayload:
		return p.TaskID
	case task.ExecutionPayload:
		return p.TaskID
	case task.StateChangedPayload:
		return p.TaskID
	case task.ContinuePayload:
		return p.TaskID
	case task.StartedPayload:
		return p.Task.TaskID
	case map[string]any:
		if v, ok := p["task_id"].(string); ok {
			return task.TaskID(v)
		}
	}
	return ""
}

// updateArtifacts mescla o snapshot genérico do data plane (Artifacts) no
// TaskState. Devolve true se um snapshot foi aplicado. O ADAPTER que produz o
// payload já converteu o domínio nas chaves genéricas (Artifact*) — o
// orquestrador apenas guarda e observa, sem conhecer o domínio.
func updateArtifacts(st *task.TaskState, payload any) bool {
	var arts map[string]string
	switch p := payload.(type) {
	case task.PositionChangedPayload:
		arts = p.Artifacts
	case task.FillPayload:
		arts = p.Artifacts
	case map[string]string:
		arts = p
	default:
		return false
	}
	if len(arts) == 0 {
		return false
	}
	if st.Artifacts == nil {
		st.Artifacts = map[string]string{}
	}
	for k, v := range arts {
		st.Artifacts[k] = v
	}
	st.Observations = append(st.Observations,
		"artifacts: symbol="+arts[task.ArtifactSymbol]+
			" side="+arts[task.ArtifactSide]+
			" qty="+arts[task.ArtifactQty]+
			" open="+arts[task.ArtifactOpen])
	return true
}

// handleRiskCheck processa um RISK_CHECK: bloqueio/severidade crítica → marca o
// watchdog (fail-closed) e pausa a task. Observação leve → apenas registro.
func handleRiskCheck(it *internalTask, ev task.Event) {
	st := &it.state
	blocked := false
	reason := ""
	switch p := ev.Payload.(type) {
	case task.RiskCheckPayload:
		blocked = p.Blocked
		reason = p.Reason
	}
	if blocked || ev.Severity == task.SeverityError || ev.Severity == task.SeverityCritical {
		it.watchdog = true
		st.Errors = append(st.Errors, "risk check bloqueado: "+reason)
		st.Observations = append(st.Observations, "risk.check: "+reason)
		if blocked {
			st.Status = task.StatusPaused
		}
		return
	}
	st.Observations = append(st.Observations, "risk.check: "+reason)
}

// addCheckpoint registra uma etapa em Checkpoints de forma idempotente. Devolve
// true se foi realmente adicionada (false = já estava → não re-executa).
func addCheckpoint(st *task.TaskState, cp string) bool {
	if cp == "" {
		return false
	}
	for _, c := range st.Checkpoints {
		if c == cp {
			return false // já registrado — etapa concluída não re-executa
		}
	}
	st.Checkpoints = append(st.Checkpoints, cp)
	return true
}

// newTaskID gera um TaskID único (16 bytes aleatórios → hex). stdlib only
// (crypto/rand); se a fonte de aleatoriedade falhar (praticamente impossível),
// cai num ID monotônico por timestamp+counter para nunca colidir. Substitui o
// `github.com/google/uuid` do cosca-trader, mantendo a primitiva pura (stdlib).
func newTaskID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("%x-%d", time.Now().UnixNano(), atomic.AddUint64(&taskIDCounter, 1))
}

var taskIDCounter uint64
