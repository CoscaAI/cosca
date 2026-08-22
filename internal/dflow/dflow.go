// Package dflow implementa o Durable Workflow (§40 do manifesto
// Creative/Scientific/Media) — Fase 1, etapa 1.9.
//
// Padrão Temporal (pesquisa da Fase 0 — princípio P2):
//   - Workflow = função PURA e determinística: só ORQUESTRA, nunca faz
//     efeitos colaterais diretos (sem clock, RNG, I/O no workflow — isso
//     quebraria o replay).
//   - Activities = efeitos colaterais REAIS (I/O, GPU, rede) com RETRY.
//   - Replay determinístico: o workflow re-executa do início com o HISTÓRICO
//     de decisões; activities já completadas DEVOLVEM o resultado gravado —
//     efeitos reais acontecem uma única vez.
//
// Se o processo morre (GPU crash, network drop, disk full — §40), o run
// retoma do histórico e continua de onde parou, sem repetir effects.
package dflow

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// ActivityKey identifica uma activity dentro do workflow (idempotência).
type ActivityKey string

// Activity é um efeito colateral executável com retry.
type Activity interface {
	// Key identifica a activity (para replay/idempotência).
	Key() ActivityKey
	// Run executa o efeito. ctx carrega o ActivityCtx (tentativa).
	Run(ctx context.Context) (any, error)
}

// ActivityFunc adapta uma função a Activity.
type ActivityFunc struct {
	ID ActivityKey
	Fn func(ctx context.Context) (any, error)
}

// Key implementa Activity.
func (f ActivityFunc) Key() ActivityKey { return f.ID }

// Run implementa Activity.
func (f ActivityFunc) Run(ctx context.Context) (any, error) { return f.Fn(ctx) }

// RetryPolicy controla as tentativas de uma activity.
type RetryPolicy struct {
	// MaxAttempts é o número máximo de tentativas (default 3).
	MaxAttempts int
	// InitialInterval é o backoff inicial (default 1s).
	InitialInterval time.Duration
	// MaxInterval limita o backoff (default 30s).
	MaxInterval time.Duration
	// BackoffMultiplier (default 2.0).
	BackoffMultiplier float64
}

// withDefaults preenche os defaults.
func (r RetryPolicy) withDefaults() RetryPolicy {
	p := r
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = 3
	}
	if p.InitialInterval <= 0 {
		p.InitialInterval = time.Second
	}
	if p.MaxInterval <= 0 {
		p.MaxInterval = 30 * time.Second
	}
	if p.BackoffMultiplier <= 0 {
		p.BackoffMultiplier = 2.0
	}
	return p
}

// StepKey é a chave de um passo do workflow (para histórico).
type StepKey string

// ActivityOutcome é o resultado registrado de uma activity no histórico.
type ActivityOutcome struct {
	Key   ActivityKey `json:"key" yaml:"key"`
	Value any         `json:"value,omitempty" yaml:"value,omitempty"`
	Error string      `json:"error,omitempty" yaml:"error,omitempty"`
}

// History é o registro append-only de decisões do workflow (P2: state =
// history, nunca variável em memória).
type History struct {
	// ActivityOutcomes das activities já completadas, por key.
	ActivityOutcomes map[ActivityKey]ActivityOutcome `json:"activity_outcomes" yaml:"activity_outcomes"`
	// ExecutedOrder preserva a ordem (para logs).
	ExecutedOrder []ActivityKey `json:"executed_order,omitempty" yaml:"executed_order,omitempty"`
	// Sleeps registra sleeps duráveis: chave "sleep:<seq>" → deadline (UTC).
	// Nil-safe: históricos antigos (sem este campo) são tratados como vazios.
	Sleeps map[string]time.Time `json:"sleeps,omitempty" yaml:"sleeps,omitempty"`
	// Events registra eventos externos resolvidos: nome → valor.
	// Nil-safe: históricos antigos (sem este campo) são tratados como vazios.
	Events map[string]any `json:"events,omitempty" yaml:"events,omitempty"`
}

// NewHistory cria um histórico vazio.
func NewHistory() *History {
	return &History{
		ActivityOutcomes: make(map[ActivityKey]ActivityOutcome),
		Sleeps:           make(map[string]time.Time),
		Events:           make(map[string]any),
	}
}

// Clone devolve uma cópia independente.
func (h *History) Clone() *History {
	c := NewHistory()
	for k, v := range h.ActivityOutcomes {
		c.ActivityOutcomes[k] = v
	}
	c.ExecutedOrder = append([]ActivityKey(nil), h.ExecutedOrder...)
	for k, v := range h.Sleeps {
		c.Sleeps[k] = v
	}
	for k, v := range h.Events {
		c.Events[k] = v
	}
	return c
}

// SortKeys devolve as keys ordenadas (exibição determinística).
func (h *History) SortKeys() []ActivityKey {
	keys := make([]ActivityKey, 0, len(h.ActivityOutcomes))
	for k := range h.ActivityOutcomes {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

// ErrActivityFailed é retornado quando uma activity esgota as tentativas.
var ErrActivityFailed = errors.New("dflow: activity failed after retries")

// ActivityCtx carrega o estado de execução de uma activity.
type ActivityCtx struct {
	Attempt   int
	LastError error
}

// Workflow é uma função pura de orquestração. Recebe um Context de execução
// que oferece ExecActivity (idempotente via histórico). NUNCA fazer I/O
// direto — isso quebraria o replay.
type Workflow func(ctx context.Context, wc *WorkflowCtx) (any, error)

// WorkflowCtx é o contexto de orquestração do workflow.
type WorkflowCtx struct {
	// history é o estado append-only do run (P2).
	history *History
	// activities disponíveis.
	activities map[ActivityKey]Activity
	// retries por activity.
	retries map[ActivityKey]RetryPolicy
	// now injetável para testes de retry (backoff determinístico).
	now func() time.Time
	// sleep injetável (real: time.Sleep; teste: imediato).
	sleep func(context.Context, time.Duration) error
	// sleepSeq sequencia os sleeps duráveis ("sleep:<seq>"). Determinístico no
	// replay: o workflow é puro e chama SleepUntil na mesma ordem.
	sleepSeq int
	// result map para memória da última execução (não persistido).
	mu      sync.Mutex
	results map[ActivityKey]any
}

// ExecActivity executa uma activity de forma idempotente: se o histórico já
// tem o resultado, devolve-o (replay) sem re-executar o efeito.
func (wc *WorkflowCtx) ExecActivity(ctx context.Context, a Activity) (any, error) {
	key := a.Key()

	wc.mu.Lock()
	if prev, ok := wc.history.ActivityOutcomes[key]; ok {
		wc.mu.Unlock()
		if prev.Error != "" {
			return nil, errors.New(prev.Error)
		}
		return prev.Value, nil
	}
	wc.mu.Unlock()

	// Não está no histórico → executa com retry.
	policy := wc.retries[key].withDefaults()
	var lastErr error
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		actCtx := context.WithValue(ctx, activityCtxKeyValue, &ActivityCtx{
			Attempt:   attempt,
			LastError: lastErr,
		})
		val, err := a.Run(actCtx)
		if err == nil {
			// Registra no histórico (append-only).
			wc.mu.Lock()
			wc.history.ActivityOutcomes[key] = ActivityOutcome{Key: key, Value: val}
			wc.history.ExecutedOrder = append(wc.history.ExecutedOrder, key)
			wc.results[key] = val
			wc.mu.Unlock()
			return val, nil
		}
		lastErr = err
		if attempt < policy.MaxAttempts {
			backoff := policy.backoffFor(attempt)
			if err := wc.sleep(ctx, backoff); err != nil {
				return nil, err
			}
		}
	}
	wc.mu.Lock()
	wc.history.ActivityOutcomes[key] = ActivityOutcome{Key: key, Error: lastErr.Error()}
	wc.history.ExecutedOrder = append(wc.history.ExecutedOrder, key)
	wc.mu.Unlock()
	return nil, fmt.Errorf("%w: %s: %v", ErrActivityFailed, key, lastErr)
}

// ErrPending é retornado pelo workflow quando ele precisa de um evento externo
// para continuar (HITL). O Runner trata como SUSPENSÃO, não como falha.
var ErrPending = errors.New("dflow: workflow pending external event")

// SleepUntil dorme até o deadline de forma DURÁVEL: a intenção é registrada no
// History (append-only) ANTES de dormir, então (a) o replay não dorme de novo
// se o deadline já passou, e (b) a retomada pós-crash espera apenas o tempo
// RESTANTE até o deadline. NUNCA chame time.Sleep direto no workflow.
func (wc *WorkflowCtx) SleepUntil(ctx context.Context, deadline time.Time) error {
	wc.mu.Lock()
	wc.sleepSeq++
	key := fmt.Sprintf("sleep:%d", wc.sleepSeq)
	wc.mu.Unlock()

	wc.mu.Lock()
	registered, exists := wc.history.Sleeps[key]
	if !exists {
		if wc.history.Sleeps == nil {
			wc.history.Sleeps = make(map[string]time.Time)
		}
		wc.history.Sleeps[key] = deadline
		registered = deadline
	}
	wc.mu.Unlock()

	remaining := registered.Sub(wc.now())
	if exists && remaining <= 0 {
		// Replay/retomada: o sleep já foi satisfeito, não espera de novo.
		return nil
	}
	return wc.sleep(ctx, remaining)
}

// WaitForEvent consulta um evento externo resolvido no History. Se o evento
// ainda não foi resolvido, devolve (nil, false) — o workflow tipicamente
// devolve ErrPending para suspender e aguardar aprovação externa (HITL).
func (wc *WorkflowCtx) WaitForEvent(name string) (any, bool) {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	if len(wc.history.Events) == 0 {
		return nil, false
	}
	v, ok := wc.history.Events[name]
	return v, ok
}

// ResolveEventValue é um alias conveniente de WaitForEvent (mesma leitura).
func (wc *WorkflowCtx) ResolveEventValue(name string) (any, bool) {
	return wc.WaitForEvent(name)
}

// History devolve o histórico atual (para persistir).
func (wc *WorkflowCtx) History() *History { return wc.history }

// Result devolve o valor de uma activity já executada.
func (wc *WorkflowCtx) Result(key ActivityKey) (any, bool) {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	v, ok := wc.results[key]
	return v, ok
}

// backoffFor calcula o backoff da tentativa (exponencial com teto).
func (p RetryPolicy) backoffFor(attempt int) time.Duration {
	// attempt = 1 → initial; attempt = 2 → initial*mult; ...
	exp := time.Duration(math.Pow(p.BackoffMultiplier, float64(attempt-1)))
	d := p.InitialInterval * exp
	if d > p.MaxInterval {
		d = p.MaxInterval
	}
	return d
}

type activityCtxKeyType struct{}

var activityCtxKeyValue = activityCtxKeyType{}

// ActivityFromContext recupera o ActivityCtx (tentativa/último erro).
func ActivityFromContext(ctx context.Context) *ActivityCtx {
	c, _ := ctx.Value(activityCtxKeyValue).(*ActivityCtx)
	return c
}

// =============================================================================
// Executor
// =============================================================================

// Result é o resultado de um run durável.
type Result struct {
	// Output do workflow.
	Output any `json:"output" yaml:"output"`
	// History persistível (state append-only).
	History *History `json:"history" yaml:"history"`
	// Executed é o número de effects executados nesta rodada.
	Executed int `json:"executed" yaml:"executed"`
	// Replayed é o número de effects retomados do histórico.
	Replayed int `json:"replayed" yaml:"replayed"`
	// Pending indica que o workflow suspendeu aguardando um evento externo
	// (HITL): não é erro, é um estado — o chamador resolve o evento e re-roda.
	Pending bool `json:"pending,omitempty" yaml:"pending,omitempty"`
}

// Runner executa workflows com replay determinístico.
type Runner struct {
	now   func() time.Time
	sleep func(context.Context, time.Duration) error
}

// NewRunner cria um runner (sleep real).
func NewRunner() *Runner {
	return &Runner{now: time.Now, sleep: defaultSleep}
}

func defaultSleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// RunOptions configura um run.
type RunOptions struct {
	// History pré-existente (retomada §40). Quando presente, activities já
	// completadas não re-executam (replay).
	History *History
}

// Run executa o workflow com replay determinístico (P2). O workflow recebe
// um WorkflowCtx cujo histórico é semelhante ao informado (clone) — ao final,
// o histórico efetivo é devolvido no Result para persistência.
func (r *Runner) Run(ctx context.Context, wf Workflow, activities []Activity, retries map[ActivityKey]RetryPolicy, opts *RunOptions) (*Result, error) {
	if wf == nil {
		return nil, errors.New("dflow: workflow is required")
	}
	acts := make(map[ActivityKey]Activity, len(activities))
	for _, a := range activities {
		acts[a.Key()] = a
	}
	baseHistory := NewHistory()
	if opts != nil && opts.History != nil {
		baseHistory = opts.History.Clone()
	}

	wc := &WorkflowCtx{
		history:    baseHistory,
		activities: acts,
		retries:    retries,
		now:        r.now,
		sleep:      r.sleep,
		results:    make(map[ActivityKey]any),
	}

	out, err := wf(ctx, wc)
	if err != nil {
		if errors.Is(err, ErrPending) {
			// Suspensão HITL: não é erro fatal. Devolve o histórico (com sleeps
			// e registros feitos até aqui) marcado como pendente.
			executed, replayed := countWork(wc.History(), opts)
			return &Result{
				History:  wc.History(),
				Pending:  true,
				Executed: executed,
				Replayed: replayed,
			}, nil
		}
		return nil, err
	}

	// Conta efeitos desta rodada vs retomados.
	executed, replayed := countWork(wc.History(), opts)
	return &Result{
		Output:   out,
		History:  wc.History(),
		Executed: executed,
		Replayed: replayed,
	}, nil
}

// ResolveEvent é uma função PURA: clona o History (sem mutar o original), seta
// o evento externo resolvido e devolve o clone. O chamador re-roda o workflow
// com o novo histórico — o WaitForEvent correspondente então devolve o valor.
// Padrão HITL: Run → Pending → Don aprova → ResolveEvent → Run de novo.
func ResolveEvent(h *History, name string, value any) *History {
	c := h.Clone()
	if c.Events == nil {
		c.Events = make(map[string]any)
	}
	c.Events[name] = value
	return c
}

func countWork(h *History, opts *RunOptions) (executed, replayed int) {
	total := len(h.ExecutedOrder)
	if opts == nil || opts.History == nil || len(opts.History.ExecutedOrder) == 0 {
		return total, 0
	}
	prior := make(map[ActivityKey]bool, len(opts.History.ExecutedOrder))
	for _, k := range opts.History.ExecutedOrder {
		prior[k] = true
	}
	for _, k := range h.ExecutedOrder {
		if prior[k] {
			replayed++
		} else {
			executed++
		}
	}
	return executed, replayed
}
