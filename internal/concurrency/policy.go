// Package concurrency define a política de concorrência do COSCA.
//
// PRINCÍPIO minerado (Osiris, MIT — src/lib/sherlock.ts, mapLimit + calibração)
// sob ADR-040. NENHUM código do repositório-fonte foi importado; apenas o
// princípio foi reimplementado como primitiva Go idiomática, stdlib-only.
//
// Ideia-axial (ADR-040): concorrência é PROPRIEDADE DO SISTEMA, não constante
// de worker. Um "12" fixo é ótimo para uma fonte e péssimo para outra; rate-limit
// é RESTRIÇÃO EXTERNA LEGÍTIMA — o sistema ADAPTA a estratégia (backoff/policy),
// nunca evade (rejeitado qualquer IP/UA-spoofing).
//
// A primitiva é ADITIVA e OPT-IN: implementa um Limiter que o caller usa
// (Acquire/Release/ReportRateLimited). Nenhum consumidor é migrado
// automaticamente; a integração é deliberada e medida.
package concurrency

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ProviderLimit define o teto por provider/fonte.
type ProviderLimit struct {
	// MaxConcurrency é o paralelismo simultâneo por provider (0 = sem teto
	// por provider; aplica-se apenas o global).
	MaxConcurrency int
	// RatePerWindow é quantas requisições o provider aceita por Window.
	// 0 = sem rate limit.
	RatePerWindow int
	// Window é a janela de rate. 0 = sem rate limit.
	Window time.Duration
	// Burst é a rajada extra acima do rate (0 = sem rajada).
	Burst int
}

// Backoff define o recuo a aplicar após um rate-limit reportado.
type Backoff struct {
	// Initial é o recuo inicial (ex. 100ms).
	Initial time.Duration
	// Max é o teto do recuo (ex. 5s).
	Max time.Duration
}

// Policy é a configuração de concorrência do sistema. Concorrência é propriedade
// do sistema, não um número mágico embutido no worker.
type Policy struct {
	// MaxGlobal é o teto global do sistema (borne do pool de conexões).
	MaxGlobal int
	// PerProvider mapeia cada provedor ao seu teto (0 = sem teto por provider).
	PerProvider map[string]ProviderLimit
	// Backoff é o recuo após rate-limit.
	Backoff Backoff
}

func (p Policy) validate() error {
	if p.MaxGlobal <= 0 {
		return fmt.Errorf("concurrency: MaxGlobal deve ser > 0 (concorrência é propriedade do sistema, não constante)")
	}
	if p.PerProvider == nil {
		p.PerProvider = map[string]ProviderLimit{}
	}
	return nil
}

// Limiter é o guard de concorrência que o caller usa por provider.
type Limiter struct {
	global    chan struct{}
	providers map[string]*limiterEntry
	mu        sync.Mutex
	backoff   Backoff
}

type limiterEntry struct {
	slots chan struct{} // nil se sem teto por provider

	// Rate bucket (não-nil apenas se RatePerWindow > 0).
	mu      sync.Mutex
	tokens  float64
	last    time.Time
	rate    float64 // tokens/segundo
	burst   float64
	enabled bool

	backoffUntil time.Time
}

// NewLimiter cria um Limiter a partir da Policy. Falha se MaxGlobal <= 0
// (sem constante mágica: quem usa a política declara o teto do sistema).
func NewLimiter(p Policy) (*Limiter, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	l := &Limiter{
		global:    make(chan struct{}, p.MaxGlobal),
		providers: make(map[string]*limiterEntry),
		backoff:   p.Backoff,
	}
	if l.backoff.Initial <= 0 {
		l.backoff.Initial = 100 * time.Millisecond
	}
	if l.backoff.Max <= 0 {
		l.backoff.Max = 5 * time.Second
	}
	for name, pl := range p.PerProvider {
		entry := &limiterEntry{}
		if pl.MaxConcurrency > 0 {
			entry.slots = make(chan struct{}, pl.MaxConcurrency)
		}
		if pl.RatePerWindow > 0 && pl.Window > 0 {
			entry.enabled = true
			entry.rate = float64(pl.RatePerWindow) / pl.Window.Seconds()
			entry.burst = float64(pl.Burst)
			if entry.burst < 1 {
				entry.burst = 1
			}
			entry.tokens = entry.burst
			entry.last = time.Now()
		}
		l.providers[name] = entry
	}
	return l, nil
}

// entryProvider devolve o estado do provider, criando um default (só global) se
// desconhecido — um provider sem política declarada usa apenas o teto global.
func (l *Limiter) entryProvider(provider string) *limiterEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.providers[provider]
	if !ok {
		entry = &limiterEntry{}
		l.providers[provider] = entry
	}
	return entry
}

// Acquire espera até que uma "permissão" esteja disponível para o provider,
// respeitando: teto global, teto por provider, rate limit (com burst) e backoff.
// Cancela via ctx (nunca fica preso; slots adquiridos são liberados em erro).
func (l *Limiter) Acquire(ctx context.Context, provider string) error {
	// Ordem fixa de aquisição (global → provider) para evitar deadlock;
	// a liberação é na ordem inversa.
	if err := acquireChan(ctx, l.global); err != nil {
		return err
	}
	entry := l.entryProvider(provider)
	if entry.slots != nil {
		if err := acquireChan(ctx, entry.slots); err != nil {
			releaseChan(l.global)
			return err
		}
	}
	if err := l.waitPolicy(ctx, entry); err != nil {
		// Libera o que foi adquirido (rollback em erro) — nunca vaza slot.
		if entry.slots != nil {
			releaseChan(entry.slots)
		}
		releaseChan(l.global)
		return err
	}
	return nil
}

// Release devolve a permissão ao Limiter. Deve ser chamada exatamente uma vez
// por Acquire bem-sucedido (defer em cada caller).
func (l *Limiter) Release(provider string) {
	entry := l.entryProvider(provider)
	if entry.slots != nil {
		releaseChan(entry.slots)
	}
	releaseChan(l.global)
}

// ReportRateLimited sinaliza que o provider respondeu um rate-limit (429/503).
// O Limiter, então, aplica backoff nas próximas Acquire para esse provider —
// ADAPTAR a estratégia, nunca EVADIR (ADR-040).
func (l *Limiter) ReportRateLimited(provider string) {
	entry := l.entryProvider(provider)
	entry.mu.Lock()
	now := time.Now()
	d := l.backoff.Initial
	if until := entry.backoffUntil; now.Before(until) {
		d = until.Sub(now) * 2 // recuo exponencial
		if d > l.backoff.Max {
			d = l.backoff.Max
		}
	}
	entry.backoffUntil = now.Add(d)
	entry.mu.Unlock()
}

// waitPolicy aplica backoff e rate limit do provider, bloqueando cancelável.
func (l *Limiter) waitPolicy(ctx context.Context, entry *limiterEntry) error {
	for {
		// 1. Backoff (se o provider foi recém rate-limited).
		entry.mu.Lock()
		wait := time.Until(entry.backoffUntil)
		if wait > 0 {
			entry.mu.Unlock()
			if err := sleepCtx(ctx, wait); err != nil {
				return err
			}
			continue
		}
		// 2. Rate limit (token bucket).
		if entry.enabled {
			w, ok := entry.reserve(time.Now())
			if !ok {
				entry.mu.Unlock()
				if err := sleepCtx(ctx, w); err != nil {
					return err
				}
				continue
			}
		}
		entry.mu.Unlock()
		return nil
	}
}

// reserve tenta consumir um token do bucket; se não houver, devolve quanto
// esperar até haver um. Não bloqueia; devolve ok=false + wait.
func (e *limiterEntry) reserve(now time.Time) (wait time.Duration, ok bool) {
	// e.mu já está obtida pelo chamador (waitPolicy).
	if e.last.IsZero() {
		e.last = now
	}
	elapsed := now.Sub(e.last).Seconds()
	// Refill contínuo, limitado ao burst.
	e.tokens += elapsed * e.rate
	if e.tokens > e.burst {
		e.tokens = e.burst
	}
	e.last = now
	if e.tokens >= 1 {
		e.tokens--
		return 0, true
	}
	// 1 token falta; quanto tempo até o refill = (1 - tokens) / rate.
	need := (1 - e.tokens) / e.rate
	return time.Duration(need * float64(time.Second)), false
}

// acquireChan envia um token no canal, respeitando cancelamento de ctx.
func acquireChan(ctx context.Context, ch chan struct{}) error {
	select {
	case ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func releaseChan(ch chan struct{}) { <-ch }

// sleepCtx dorme d ou cancela via ctx.
func sleepCtx(ctx context.Context, d time.Duration) error {
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
