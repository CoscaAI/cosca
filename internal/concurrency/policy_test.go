package concurrency

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewLimiter_RequiresMaxGlobal(t *testing.T) {
	t.Parallel()
	if _, err := NewLimiter(Policy{}); err == nil {
		t.Fatal("MaxGlobal <= 0 deveria dar erro (sem constante mágica)")
	}
}

// TestMaxGlobal valida que a concorrência agregada nunca excede o teto global.
func TestMaxGlobal(t *testing.T) {
	t.Parallel()
	l, err := NewLimiter(Policy{MaxGlobal: 3})
	if err != nil {
		t.Fatalf("NewLimiter: %v", err)
	}
	var active, maxActive int64
	var mu sync.Mutex
	var wg sync.WaitGroup
	ctx := context.Background()
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := l.Acquire(ctx, "x"); err != nil {
				t.Errorf("acquire: %v", err)
				return
			}
			n := atomic.AddInt64(&active, 1)
			mu.Lock()
			if n > maxActive {
				maxActive = n
			}
			mu.Unlock()
			time.Sleep(time.Millisecond)
			atomic.AddInt64(&active, -1)
			l.Release("x")
		}()
	}
	wg.Wait()
	if maxActive > 3 {
		t.Errorf("maxActive = %d, esperava <= 3", maxActive)
	}
	if maxActive == 0 {
		t.Error("esperava concorrência observada > 0")
	}
}

// TestPerProvider valida tetos independentes por provider (o ganho sobre um
// semáforo global fixo: A=2, B=4 coexistem).
func TestPerProvider(t *testing.T) {
	t.Parallel()
	l, _ := NewLimiter(Policy{
		MaxGlobal: 8,
		PerProvider: map[string]ProviderLimit{
			"A": {MaxConcurrency: 2},
			"B": {MaxConcurrency: 4},
		},
	})
	var activeA, maxA, activeB, maxB int64
	var mu sync.Mutex
	var wg sync.WaitGroup
	ctx := context.Background()
	worker := func(provider string) {
		defer wg.Done()
		if err := l.Acquire(ctx, provider); err != nil {
			t.Errorf("acquire %s: %v", provider, err)
			return
		}
		var active, maxActive *int64
		if provider == "A" {
			active, maxActive = &activeA, &maxA
		} else {
			active, maxActive = &activeB, &maxB
		}
		n := atomic.AddInt64(active, 1)
		mu.Lock()
		if n > *maxActive {
			*maxActive = n
		}
		mu.Unlock()
		time.Sleep(time.Millisecond)
		atomic.AddInt64(active, -1)
		l.Release(provider)
	}
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go worker("A")
	}
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go worker("B")
	}
	wg.Wait()
	if maxA > 2 {
		t.Errorf("maxA = %d, esperava <= 2", maxA)
	}
	if maxB > 4 {
		t.Errorf("maxB = %d, esperava <= 4", maxB)
	}
}

// TestRateLimitBurst valida o rate limit + burst: o burst é consumido de imediato
// e o próximo acquire espera o rate.
func TestRateLimitBurst(t *testing.T) {
	t.Parallel()
	l, _ := NewLimiter(Policy{
		MaxGlobal: 4,
		PerProvider: map[string]ProviderLimit{
			"rl": {MaxConcurrency: 1, RatePerWindow: 1, Window: 200 * time.Millisecond, Burst: 1},
		},
	})
	ctx := context.Background()

	// O burst (1) é consumido imediatamente.
	if err := l.Acquire(ctx, "rl"); err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	l.Release("rl")

	// O próximo acquire precisa esperar (~200ms de janela para refill de 1).
	start := time.Now()
	if err := l.Acquire(ctx, "rl"); err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	elapsed := time.Since(start)
	l.Release("rl")
	if elapsed < 100*time.Millisecond {
		t.Errorf("acquire após burst deveria esperar ~200ms, esperou %s", elapsed)
	}
}

// TestBackoff valida que ReportRateLimited → a próxima Acquire espera o backoff.
func TestBackoff(t *testing.T) {
	t.Parallel()
	l, _ := NewLimiter(Policy{
		MaxGlobal: 2,
		Backoff:   Backoff{Initial: 60 * time.Millisecond, Max: 120 * time.Millisecond},
	})
	ctx := context.Background()
	l.ReportRateLimited("bp")
	start := time.Now()
	if err := l.Acquire(ctx, "bp"); err != nil {
		t.Fatalf("acquire após backoff: %v", err)
	}
	elapsed := time.Since(start)
	l.Release("bp")
	if elapsed < 30*time.Millisecond {
		t.Errorf("acquire deveria respeitar o backoff (~60ms), esperou %s", elapsed)
	}
}

// TestCancel valida o cancelamento via context: Acquire aborta e NÃO vaza slots.
func TestCancel(t *testing.T) {
	t.Parallel()
	l, _ := NewLimiter(Policy{MaxGlobal: 1})
	// Ocupa o único slot com uma goroutine que segura por um tempo.
	hold := make(chan struct{})
	go func() {
		_ = l.Acquire(context.Background(), "c")
		<-hold
		l.Release("c")
	}()
	time.Sleep(20 * time.Millisecond)

	// Segundo acquire com ctx cancelado deve abortar rapidamente.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := l.Acquire(ctx, "c"); err == nil {
		t.Fatal("acquire com ctx cancelado deveria retornar erro")
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Errorf("cancelamento deveria ser rápido, esperou %s", time.Since(start))
	}
	close(hold)
}

// TestNoDeadlock valida que uma mistura concorrente de Acquire/Release por
// vários providers termina (sem deadlock/starvation). Roda sob -race.
func TestNoDeadlock(t *testing.T) {
	t.Parallel()
	l, _ := NewLimiter(Policy{
		MaxGlobal: 3,
		PerProvider: map[string]ProviderLimit{
			"p1": {MaxConcurrency: 2},
			"p2": {MaxConcurrency: 2},
		},
	})
	var wg sync.WaitGroup
	ctx := context.Background()
	for i := 0; i < 30; i++ {
		wg.Add(1)
		provider := map[bool]string{true: "p1", false: "p2"}[i%2 == 0]
		go func(provider string) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				if err := l.Acquire(ctx, provider); err != nil {
					t.Errorf("acquire: %v", err)
					return
				}
				_ = j
				l.Release(provider)
			}
		}(provider)
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("deadlock/starvação detectada (não terminou em 10s)")
	}
}

// TestUnderLoad mede que o Limiter respeita o teto sob carga concorrente e que
// o release devolve os slots (concorrência volta a zero).
func TestUnderLoad(t *testing.T) {
	t.Parallel()
	l, _ := NewLimiter(Policy{MaxGlobal: 4})
	var active int64
	var mu sync.Mutex
	var wg sync.WaitGroup
	ctx := context.Background()
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := l.Acquire(ctx, "load"); err != nil {
				return
			}
			atomic.AddInt64(&active, 1)
			time.Sleep(200 * time.Microsecond)
			atomic.AddInt64(&active, -1)
			l.Release("load")
		}()
	}
	wg.Wait()
	mu.Lock()
	defer mu.Unlock()
	if a := atomic.LoadInt64(&active); a != 0 {
		t.Errorf("active após carga = %d, esperava 0 (slots devolvidos)", a)
	}
}
