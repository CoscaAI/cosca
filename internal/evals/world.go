package evals

// Harness world/body + trace de evidência (mineração ADR-017, openwork
// testkit `evals/packages/testkit/src/spec/{index,runtime,types}.ts`).
//
// O contrato minerado tem duas metades, implementadas aqui sem substituir o
// substrato (`internal/ledger`, `internal/trace`):
//
//  1. **world/body como INVARIANTE DE ORDEM** (não convenção): o setup (seed)
//     é separado do act (body). Semear DEPOIS do primeiro ato é um ERRO
//     (`SeedBeforeActError`); act antes de qualquer seed também é erro. Setup
//     em um stack limpo no dispose (isolamento por construção — I7): os
//     recursos criados no seed são liberados em `Dispose`, mesmo quando o
//     body falha.
//  2. **trace de evidência como REPLAYER DETERMINÍSTICO**: cada ação emite um
//     `evidence.TraceEntry{seq, at, verb, detail, ok, ms, error}` com redaction
//     embutida (email/Bearer/token/password), e cada passo carrega
//     `evidence.Step{name, ok, ms}` com estado `not-reached` (passos posteriores
//     nem rodam quando um falha) e `needs`/`unmetNeeds` (pré-requisito não
//     satisfeito → `skip` com reason, em vez de fail).
//
// A evidência é replayable: idêntica exceto pelos campos temporais (`at`/`ms`),
// como provam `evidence.ReplayEqual`/`ReplayHash`.

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/evidence"
	"github.com/CoscaAI/cosca/internal/trace"
)

// phaseState é a máquina de estados interna de ordem do harness.
type phaseState int

const (
	// phaseInit: nenhum seed/act ainda.
	phaseInit phaseState = iota
	// phaseSeeding: pelo menos um seed começou, nenhum act ainda.
	phaseSeeding
	// phaseActing: o primeiro act começou — seeds são agora proibidos.
	phaseActing
	// phaseDone: Dispose foi chamado — o harness é imutável daqui em diante.
	phaseDone
)

// SeedBeforeActError é devolvido quando um seed (setup/world) é tentado DEPOIS
// do primeiro act (body). A ordem world→body é um INVARIANTE DE ORDEM, não uma
// convenção: o harness recusa semear um mundo que já agiu.
type SeedBeforeActError struct {
	attempt string
}

// Error implementa error com a razão explícita de ordem.
func (e *SeedBeforeActError) Error() string {
	return fmt.Sprintf("evals: world seed %q attempted after first act (seed-before-act is an order invariant)", e.attempt)
}

// CleanupFunc libera um recurso adquirido durante o seed. Devolve error para o
// dispose reportar falhas de liberação (best-effort, não-fatal por contrato).
type CleanupFunc func() error

// cleanupItem empilha um recurso para liberação LIFO no Dispose.
type cleanupItem struct {
	name string
	fn   CleanupFunc
}

// Harness é o harness world/body + replayer de evidência do Cosca. Ele garante
// por construção que o seed precede o ato, mantém um stack LIFO de cleanup
// (I7: isolamento por construção) e emite uma trilha de evidência replayable,
// redacted, sem LLM.
type Harness struct {
	mu      sync.Mutex
	phase   phaseState
	seeded  bool
	acted   bool
	failed  bool // um passo já falhou → passos seguintes são not-reached
	seq     int
	name    string
	traceID trace.TraceID
	store   *trace.Store // substrato append-only opcional (flight recorder)

	// Evidência replayable.
	entries []evidence.TraceEntry
	steps   []evidence.Step
	byName  map[string]evidence.Step // último passo por nome (para checar needs)

	// Cleanup LIFO (I7).
	cleanup  []cleanupItem
	disposed bool
}

// NewHarness cria um Harness com um Trace ID universal novo
// (TRACE-YYYYMMDD-XXXX), pronto para seed(→world) e act(→body).
func NewHarness(name string) *Harness {
	return &Harness{
		phase:   phaseInit,
		name:    name,
		traceID: trace.NewID(),
		byName:  map[string]evidence.Step{},
	}
}

// Name devolve o identificador do harness.
func (h *Harness) Name() string { return h.name }

// TraceID devolve a identificação universal da execução (reusa internal/trace).
func (h *Harness) TraceID() trace.TraceID { return h.traceID }

// SetTraceStore encaixa o transporte append-only (internal/trace) como sink
// opcional: cada TraceEntry emitida é appendada à pauta best-effort. NUNCA
// reescreve o passado (append-only por contrato).
func (h *Harness) SetTraceStore(s *trace.Store) {
	h.mu.Lock()
	h.store = s
	h.mu.Unlock()
}

// DeferCleanup registra um recurso para liberação no Dispose. A ordem é LIFO
// (o mais recentemente registrado é liberado primeiro). É o mecanismo de I7:
// o seed empilha os recursos que criar; o dispose os derruba mesmo que o body
// falhe.
func (h *Harness) DeferCleanup(name string, fn CleanupFunc) {
	if fn == nil {
		return
	}
	h.mu.Lock()
	h.cleanup = append(h.cleanup, cleanupItem{name: name, fn: fn})
	h.mu.Unlock()
}

// Seed executa um passo de setup (world). Pode ser chamado várias vezes, MAS
// apenas ANTES do primeiro act. Um seed após o act devolve SeedBeforeActError.
// O corpo é o "world setup"; os recursos adquiridos devem ser empilhados via
// DeferCleanup para o Dispose os liberar.
func (h *Harness) Seed(ctx context.Context, name string, fn func(ctx context.Context) error) error {
	h.mu.Lock()
	switch h.phase {
	case phaseActing, phaseDone:
		h.mu.Unlock()
		return &SeedBeforeActError{attempt: name}
	case phaseInit:
		h.phase = phaseSeeding
		h.seeded = true
	default: // phaseSeeding
		h.seeded = true
	}
	h.mu.Unlock()

	start := time.Now()
	var err error
	if fn != nil {
		err = fn(ctx)
	}
	h.emit("seed", name, err == nil, time.Since(start), errString(err))
	return err
}

// Act executa um passo de act (body). Exige que tenha havido pelo menos um
// seed (setup antes do ato senão erro); a partir do primeiro act, todos os
// seeds passam a devolver SeedBeforeActError. Chamar Act torna a execução
// "world→body" e fecha a fase de seed.
func (h *Harness) Act(ctx context.Context, name string, fn func(ctx context.Context) error) error {
	h.mu.Lock()
	if h.phase == phaseDone {
		h.mu.Unlock()
		return fmt.Errorf("evals: harness %q already disposed", h.name)
	}
	if !h.seeded {
		h.mu.Unlock()
		return fmt.Errorf("evals: act %q before any world seed (setup before act is an order invariant)", name)
	}
	if h.phase < phaseActing {
		h.phase = phaseActing
	}
	h.acted = true
	h.mu.Unlock()

	start := time.Now()
	var err error
	if fn != nil {
		err = fn(ctx)
	}
	h.emit("act", name, err == nil, time.Since(start), errString(err))
	return err
}

// Step roda um passo nomeado e registra `evidence.Step{name, ok, ms}`. Se um
// passo anterior FOI marcado como falho, o passo é registrado como not-reached
// e a função NÃO é invocada (short-circuit, replay determinístico). Devolve o
// Step gravado.
func (h *Harness) Step(ctx context.Context, name string, fn func(ctx context.Context) error) evidence.Step {
	return h.StepWithNeeds(ctx, name, nil, fn)
}

// StepWithNeeds roda um passo nomeado que declara pré-requisitos (`needs` —
// nomes de passos que devem ter passado). Se um need não está satisfeito, o
// passo é SKIPPED com reason (não é fail). Se um passo anterior falhou, o passo
// é not-reached. Devolve o Step gravado.
func (h *Harness) StepWithNeeds(ctx context.Context, name string, needs []string, fn func(ctx context.Context) error) evidence.Step {
	h.mu.Lock()
	if h.phase < phaseActing {
		h.phase = phaseActing
		h.acted = true
	}
	needCopy := append([]string(nil), needs...)

	// (a) not-reached: um passo anterior falhou → este NÃO roda (short-circuit).
	if h.failed {
		st := evidence.Step{Name: name, Status: evidence.StepNotReached, OK: false, Needs: needCopy}
		h.appendStepLocked(st)
		h.mu.Unlock()
		h.emit("step", name, false, 0, "not-reached (a prior step failed)")
		return st
	}

	// (b) needs/unmetNeeds → skip com reason, em vez de fail.
	if reason := h.unmetNeeds(needs); reason != "" {
		st := evidence.Step{Name: name, Status: evidence.StepSkipped, OK: false, Reason: reason, Needs: needCopy}
		h.appendStepLocked(st)
		h.mu.Unlock()
		h.emit("step", name, false, 0, reason)
		return st
	}
	h.mu.Unlock()

	// (c) roda o passo.
	start := time.Now()
	var err error
	if fn != nil {
		err = fn(ctx)
	}
	elapsed := time.Since(start)
	st := evidence.Step{Name: name, OK: err == nil, Ms: elapsed.Milliseconds(), Needs: needCopy}
	if err != nil {
		st.Status = evidence.StepFailed
		st.Error = evidence.Redact(err.Error())
	} else {
		st.Status = evidence.StepOK
	}

	h.mu.Lock()
	h.appendStepLocked(st)
	if st.Status == evidence.StepFailed {
		h.failed = true
	}
	h.mu.Unlock()
	h.emit("step", name, err == nil, elapsed, errString(err))
	return st
}

// unmetNeeds devolve a razão do primeiro pré-requisito não satisfeito, ou ""
// se todos estiverem OK. É consultado sob lock.
func (h *Harness) unmetNeeds(needs []string) string {
	for _, n := range needs {
		st, ok := h.byName[n]
		if !ok || !st.OK {
			return fmt.Sprintf("unmet need %q (missing or failed)", n)
		}
	}
	return ""
}

// appendStepLocked registra um passo e indexa por nome (sob lock).
func (h *Harness) appendStepLocked(st evidence.Step) {
	h.steps = append(h.steps, st)
	h.byName[st.Name] = st
}

// Entries devolve uma cópia da trilha de evidência (replayable).
func (h *Harness) Entries() []evidence.TraceEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]evidence.TraceEntry, len(h.entries))
	copy(out, h.entries)
	return out
}

// Steps devolve uma cópia dos passos gravados (replayable).
func (h *Harness) Steps() []evidence.Step {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]evidence.Step, len(h.steps))
	copy(out, h.steps)
	return out
}

// ReplayHash devolve a assinatura determinística da evidência (ignora campos
// temporais). Usada para comparar duas execuções do mesmo script.
func (h *Harness) ReplayHash() string {
	h.mu.Lock()
	entries := make([]evidence.TraceEntry, len(h.entries))
	copy(entries, h.entries)
	steps := make([]evidence.Step, len(h.steps))
	copy(steps, h.steps)
	h.mu.Unlock()
	return evidence.ReplayHash(entries, steps)
}

// Dispose libera o stack de recursos do seed em ordem LIFO (I7: isolamento por
// construção) e sela o harness (immutável daqui em diante). Chamadas
// subsequentes são no-op (idempotente). Se qualquer cleanup falhar, devolve o
// primeiro erro (não-fatal por contrato: os demais seguem).
func (h *Harness) Dispose() error {
	h.mu.Lock()
	if h.disposed {
		h.mu.Unlock()
		return nil
	}
	h.disposed = true
	h.phase = phaseDone
	items := make([]cleanupItem, len(h.cleanup))
	copy(items, h.cleanup)
	h.mu.Unlock()

	// LIFO: o recurso mais recentemente empilhado é liberado primeiro.
	var firstErr error
	for i := len(items) - 1; i >= 0; i-- {
		if err := items[i].fn(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("evals: cleanup %q: %w", items[i].name, err)
		}
	}
	h.emit("dispose", "cleanup", firstErr == nil, 0, errString(firstErr))
	return firstErr
}

// emit anexa uma TraceEntry redacted à trilha e, opcionalmente, append-a ao
// flight recorder append-only (best-effort). O lock NÃO é mantido durante o
// append para não serializar a pauta atrás da evidência.
func (h *Harness) emit(verb, detail string, ok bool, elapsed time.Duration, errText string) evidence.TraceEntry {
	h.mu.Lock()
	h.seq++
	entry := evidence.TraceEntry{
		Seq:    h.seq,
		At:     time.Now().UTC(),
		Verb:   verb,
		Detail: evidence.Redact(detail),
		OK:     ok,
		Ms:     elapsed.Milliseconds(),
		Error:  evidence.Redact(errText),
	}
	h.entries = append(h.entries, entry)
	store := h.store
	h.mu.Unlock()

	if store != nil {
		// Best-effort: a pauta append-only é um substrato opcional; se ela
		// recusar, a evidência em memória é suficiente e o harness não falha.
		_ = store.Append(trace.Event{
			TraceID: h.traceID.String(),
			Actor:   "harness:" + h.name,
			Action:  verb,
			Result:  boolString(ok),
			Details: entry.Detail,
		})
	}
	return entry
}

func boolString(b bool) string {
	if b {
		return "success"
	}
	return "failed"
}

// errString converte um error em texto ou "" — preserva nil (sem erro) como vazio.
func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
