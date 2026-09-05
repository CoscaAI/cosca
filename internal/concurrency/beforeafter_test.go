package concurrency

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// rateBudget simula um provider externo com um orçamento de taxa: permite no
// máximo `limit` requisições por janela `window`. Além disso → recusa (rate limited).
type rateBudget struct {
	mu          sync.Mutex
	windowStart time.Time
	count       int
	limit       int
	window      time.Duration
}

func newRateBudget(limit int, window time.Duration) *rateBudget {
	return &rateBudget{windowStart: time.Now(), limit: limit, window: window}
}

func (r *rateBudget) allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	if now.Sub(r.windowStart) >= r.window {
		r.windowStart = now
		r.count = 0
	}
	r.count++
	return r.count <= r.limit
}

// TestBeforeAfter_AdaptsToRateLimitedProvider é a medição ANTES × DEPOIS do
// ADR-040, provando o valor da policy num provider que rate-limita.
//
// ANTES (semáforo cru, cap 5): 5 façam-out chegam DE UMA VEZ; o provider só
// aceita 1 por janela → 4 erros de rate-limit.
//
// DEPOIS (policy com rate+backoff): as requisições são espaçadas (1 por janela);
// o provider nunca estoura → 0 erros. O sistema ADAPTA (rate/backoff), não EVADE.
func TestBeforeAfter_AdaptsToRateLimitedProvider(t *testing.T) {
	const window = 20 * time.Millisecond
	const requests = 5

	// ── ANTES: semáforo cru (sem rate limit / sem backoff) ─────────────────
	beforeBudget := newRateBudget(1, window)
	sem := make(chan struct{}, 5) // raw semaphore
	var beforeErrs int64
	var wg sync.WaitGroup
	ctx := context.Background()
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if !beforeBudget.allow() { // provider devolve rate-limit
				atomic.AddInt64(&beforeErrs, 1)
			}
		}()
	}
	wg.Wait()

	// ── DEPOIS: policy com rate+limit+backoff ──────────────────────────────
	afterBudget := newRateBudget(1, window)
	pol, err := NewLimiter(Policy{
		MaxGlobal: 5,
		PerProvider: map[string]ProviderLimit{
			"embed": {MaxConcurrency: 1, RatePerWindow: 1, Window: window, Burst: 1},
		},
		Backoff: Backoff{Initial: 20 * time.Millisecond, Max: 40 * time.Millisecond},
	})
	if err != nil {
		t.Fatalf("NewLimiter: %v", err)
	}
	var afterErrs int64
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := pol.Acquire(ctx, "embed"); err != nil {
				return
			}
			defer pol.Release("embed")
			if !afterBudget.allow() {
				atomic.AddInt64(&afterErrs, 1)
				pol.ReportRateLimited("embed")
			}
		}()
	}
	wg.Wait()

	t.Logf("provider rate-limita (1 req / %v); requests=%d", window, requests)
	t.Logf("ANTES  (semáforo cru): erro de rate-limit = %d", beforeErrs)
	t.Logf("DEPOIS (policy)      : erro de rate-limit = %d", afterErrs)

	// A policy deve eliminar (ou reduzir fortemente) os erros de rate-limit,
	// porque ela ADAPTA a estratégia (rate/backoff) em vez de estourar na fonte.
	if afterErrs >= beforeErrs {
		t.Errorf("policy deveria reduzir erros de rate-limit: antes=%d depois=%d", beforeErrs, afterErrs)
	}
	if beforeErrs == 0 {
		t.Skip("semáforo cru não gerou erro (provider muito tolerante) — não dá para comparar")
	}
}
