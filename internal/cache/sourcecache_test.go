package cache

import (
	"context"
	"sync"
	"testing"
	"time"
)

// ── Benchmark: hit-path sob paralelismo (single-flight evita N fetches) ────

func BenchmarkSourceCache_ParallelHit(b *testing.B) {
	clk := newFakeClock()
	f := &countingFetcher{counts: map[string]int{}}
	sc := NewSourceCache(f.fetch, SourceConfig{TTL: time.Hour})
	sc.now = clk.now
	ctx := context.Background()
	// Popula o cache uma vez; a partir daqui todo load é um hit (servido de
	// memória) — exatamente o cenário que o single-flight protege no miss,
	// aqui amortizado no hit path.
	_, _ = sc.Load(ctx, "k")
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = sc.Load(ctx, "k")
		}
	})
}

// fakeClock é um relógio controlável para testar TTL/stale sem dormir.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock { return &fakeClock{t: time.Now()} }

func (c *fakeClock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// countingFetcher conta chamadas por chave e opcionalmente bloqueia uma chave
// (in-flight). Todo acesso é sincronizado; o teste nunca lê o mapa interno do
// CachedSource diretamente.
type countingFetcher struct {
	mu     sync.Mutex
	counts map[string]int
	blocks map[string]chan struct{} // chave -> gate; se presente, bloqueia até fechar
}

func (f *countingFetcher) fetch(ctx context.Context, key string) (string, error) {
	f.mu.Lock()
	f.counts[key]++
	f.mu.Unlock()
	if gate, ok := f.blocks[key]; ok && gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return "value-" + key, nil
}

func (f *countingFetcher) calls(key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.counts[key]
}

// ── 1. cache hit normal ────────────────────────────────────────────────────

func TestSourceCache_Hit(t *testing.T) {
	t.Parallel()
	clk := newFakeClock()
	f := &countingFetcher{counts: map[string]int{}}
	sc := NewSourceCache(f.fetch, SourceConfig{TTL: time.Minute})
	sc.now = clk.now

	r1, err := sc.Load(context.Background(), "a")
	if err != nil {
		t.Fatalf("Load miss: %v", err)
	}
	if r1.Value != "value-a" || r1.Stale {
		t.Fatalf("r1 = %+v, want value-a, fresh", r1)
	}
	r2, err := sc.Load(context.Background(), "a")
	if err != nil {
		t.Fatalf("Load hit: %v", err)
	}
	if r2.Value != "value-a" || r2.Stale {
		t.Fatalf("r2 = %+v, want value-a, fresh", r2)
	}
	if got := f.calls("a"); got != 1 {
		t.Fatalf("fetcher called %d times, want 1", got)
	}
}

// ── 2. TTL respeitado ──────────────────────────────────────────────────────

func TestSourceCache_TTL(t *testing.T) {
	t.Parallel()
	clk := newFakeClock()
	f := &countingFetcher{counts: map[string]int{}}
	sc := NewSourceCache(f.fetch, SourceConfig{TTL: time.Minute})
	sc.now = clk.now

	_, _ = sc.Load(context.Background(), "a")
	if got := f.calls("a"); got != 1 {
		t.Fatalf("calls after first load = %d, want 1", got)
	}
	_, _ = sc.Load(context.Background(), "a") // hit dentro da janela
	if got := f.calls("a"); got != 1 {
		t.Fatalf("calls after hit = %d, want 1", got)
	}
	clk.advance(2 * time.Minute) // expira
	_, _ = sc.Load(context.Background(), "a")
	if got := f.calls("a"); got != 2 {
		t.Fatalf("calls after expiry = %d, want 2", got)
	}
}

// ── 3. single-flight: N callers concorrentes → 1 fetch ─────────────────────

func TestSourceCache_SingleFlight(t *testing.T) {
	t.Parallel()
	clk := newFakeClock()
	f := &countingFetcher{counts: map[string]int{}, blocks: map[string]chan struct{}{"shared": make(chan struct{})}}
	sc := NewSourceCache(f.fetch, SourceConfig{TTL: time.Minute})
	sc.now = clk.now

	const n = 8
	var wg sync.WaitGroup
	results := make([]SourceResult[string], n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			r, _ := sc.Load(context.Background(), "shared")
			results[idx] = r
		}(i)
	}
	close(start)
	time.Sleep(80 * time.Millisecond) // initiator entra no gate; demais se juntam
	if got := f.calls("shared"); got != 1 {
		t.Fatalf("fetcher called %d times during concurrency, want 1", got)
	}
	close(f.blocks["shared"])
	wg.Wait()

	if got := f.calls("shared"); got != 1 {
		t.Fatalf("fetcher called %d times after release, want 1", got)
	}
	for i := 0; i < n; i++ {
		if results[i].Value != "value-shared" || results[i].Stale {
			t.Fatalf("caller %d got %+v, want shared fresh", i, results[i])
		}
	}
}

// ── 4. resultado compartilhado entre callers ───────────────────────────────

func TestSourceCache_SharedResult(t *testing.T) {
	t.Parallel()
	clk := newFakeClock()
	f := &countingFetcher{counts: map[string]int{}, blocks: map[string]chan struct{}{"k": make(chan struct{})}}
	sc := NewSourceCache(f.fetch, SourceConfig{TTL: time.Minute})
	sc.now = clk.now

	ctx := context.Background()
	const n = 6
	var wg sync.WaitGroup
	results := make([]SourceResult[string], n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			r, _ := sc.Load(ctx, "k")
			results[idx] = r
		}(i)
	}
	time.Sleep(60 * time.Millisecond)
	if got := f.calls("k"); got != 1 {
		t.Fatalf("fetcher called %d times, want 1 (single-flight)", got)
	}
	close(f.blocks["k"])
	wg.Wait()

	for i := 0; i < n; i++ {
		if results[i].Value != "value-k" || results[i].Stale {
			t.Fatalf("caller %d result = %+v, want shared fresh value-k", i, results[i])
		}
	}
	if got := f.calls("k"); got != 1 {
		t.Fatalf("fetcher called %d times after release, want 1", got)
	}
}

// ── 5. erro sem valor anterior → erro permanece erro ───────────────────────

func TestSourceCache_ErrorNoPrior(t *testing.T) {
	t.Parallel()
	clk := newFakeClock()
	f := &countingFetcher{counts: map[string]int{}}
	sc := NewSourceCache(func(ctx context.Context, key string) (string, error) {
		return "", context.DeadlineExceeded
	}, SourceConfig{TTL: time.Minute})
	sc.now = clk.now

	r, err := sc.Load(context.Background(), "a")
	if err == nil {
		t.Fatal("expected error when no prior value")
	}
	if r.Stale {
		t.Errorf("should not be stale without a prior value")
	}
	// Sem valor anterior, o erro não é mascarado em chamadas seguintes.
	if _, err2 := sc.Load(context.Background(), "a"); err2 == nil {
		t.Fatal("expected error again (no stale to fall back on)")
	}
	_ = f
}

// ── 6. erro com stale disponível → stale retornado ─────────────────────────

func TestSourceCache_ErrorWithStale(t *testing.T) {
	t.Parallel()
	clk := newFakeClock()
	f := &countingFetcher{counts: map[string]int{}}
	sc := NewSourceCache(f.fetch, SourceConfig{TTL: time.Minute})
	sc.now = clk.now

	if _, err := sc.Load(context.Background(), "a"); err != nil {
		t.Fatalf("first load: %v", err)
	}
	clk.advance(2 * time.Minute) // expira
	// Troca o fetcher por um que falha, simulando refresh quebrado.
	sc.fetcher = func(ctx context.Context, key string) (string, error) {
		return "", context.DeadlineExceeded
	}

	r, err := sc.Load(context.Background(), "a")
	if err != nil {
		t.Fatalf("stale path should NOT surface the refresh error: %v", err)
	}
	if !r.Stale {
		t.Errorf("expected stale=true on failed refresh, got %+v", r)
	}
	if r.Value != "value-a" {
		t.Errorf("stale value = %q, want the last good copy", r.Value)
	}
}

// ── 7. comportamento após expiração ────────────────────────────────────────

func TestSourceCache_AfterExpirationGetsFresh(t *testing.T) {
	t.Parallel()
	clk := newFakeClock()
	f := &countingFetcher{counts: map[string]int{}}
	sc := NewSourceCache(f.fetch, SourceConfig{TTL: time.Minute})
	sc.now = clk.now

	_, _ = sc.Load(context.Background(), "k")
	clk.advance(90 * time.Second)
	r, err := sc.Load(context.Background(), "k")
	if err != nil {
		t.Fatalf("load after expiry: %v", err)
	}
	if r.Stale {
		t.Errorf("expected fresh after successful refetch, got stale")
	}
	if r.Value != "value-k" {
		t.Errorf("value = %q", r.Value)
	}
}

// ── 8. concorrência / race safety ──────────────────────────────────────────

func TestSourceCache_RaceSafety(t *testing.T) {
	t.Parallel()
	clk := newFakeClock()
	f := &countingFetcher{counts: map[string]int{}}
	sc := NewSourceCache(f.fetch, SourceConfig{TTL: 50 * time.Millisecond})
	sc.now = clk.now

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 30; j++ {
				_, _ = sc.Load(context.Background(), "k")
			}
		}()
		if i%5 == 0 {
			clk.advance(60 * time.Millisecond) // força expiração sob concorrência
		}
	}
	wg.Wait()
}

// ── 9. mapa com teto: evicta a mais antiga, nunca a in-flight ──────────────

func TestSourceCache_EvictionOldest(t *testing.T) {
	t.Parallel()
	clk := newFakeClock()
	f := &countingFetcher{counts: map[string]int{}}
	sc := NewSourceCache(f.fetch, SourceConfig{TTL: time.Minute, MaxEntries: 3})
	sc.now = clk.now

	ctx := context.Background()
	for _, k := range []string{"a", "b", "c"} {
		if _, err := sc.Load(ctx, k); err != nil {
			t.Fatalf("load %s: %v", k, err)
		}
	}
	// Inserir "d" estoura o teto → a mais antiga ("a") é evictada.
	if _, err := sc.Load(ctx, "d"); err != nil {
		t.Fatalf("load d: %v", err)
	}
	// "a" evictada → próximo Load refaz o fetch (contador de "a" sobe para 2).
	_, _ = sc.Load(ctx, "a")
	if got := f.calls("a"); got != 2 {
		t.Fatalf("a should have been evicted and refetched, calls = %d", got)
	}
	// b, c, d são hits (contador continua 1).
	for _, k := range []string{"b", "c", "d"} {
		if got := f.calls(k); got != 1 {
			t.Errorf("key %q should be a hit, calls = %d", k, got)
		}
	}
}

func TestSourceCache_EvictionSkipsInflight(t *testing.T) {
	t.Parallel()
	clk := newFakeClock()
	f := &countingFetcher{counts: map[string]int{}, blocks: map[string]chan struct{}{"b": make(chan struct{})}}
	sc := NewSourceCache(f.fetch, SourceConfig{TTL: time.Minute, MaxEntries: 2})
	sc.now = clk.now

	ctx := context.Background()
	// "a" mais antiga, válida, NÃO in-flight.
	if _, err := sc.Load(ctx, "a"); err != nil {
		t.Fatalf("load a: %v", err)
	}
	// "b" fica in-flight (bloqueia no gate de "b").
	doneB := make(chan struct{})
	go func() { _, _ = sc.Load(ctx, "b"); close(doneB) }()
	time.Sleep(40 * time.Millisecond)
	if got := f.calls("b"); got != 1 {
		t.Fatalf("b fetch should have started, calls = %d", got)
	}

	// Inserir "c" estoura o teto (max=2). Só há uma entrada evictável não
	// in-flight e mais antiga: "a". "b" (in-flight) deve ser preservada.
	if _, err := sc.Load(ctx, "c"); err != nil {
		t.Fatalf("load c: %v", err)
	}
	select {
	case <-doneB:
		t.Fatal("b should still be in-flight right after inserting c")
	default:
	}

	// "a" foi evictada → próximo Load refaz o fetch.
	_, _ = sc.Load(ctx, "a")
	if got := f.calls("a"); got != 2 {
		t.Errorf("a should be evicted and refetched, calls = %d", got)
	}
	if got := f.calls("c"); got != 1 {
		t.Errorf("c should be a hit, calls = %d", got)
	}

	// Libera "b" → o fetch completa sem corrupção.
	close(f.blocks["b"])
	<-doneB
}

// ── Clear ──────────────────────────────────────────────────────────────────

func TestSourceCache_Clear(t *testing.T) {
	t.Parallel()
	clk := newFakeClock()
	f := &countingFetcher{counts: map[string]int{}}
	sc := NewSourceCache(f.fetch, SourceConfig{TTL: time.Minute})
	sc.now = clk.now

	if _, err := sc.Load(context.Background(), "a"); err != nil {
		t.Fatalf("load a: %v", err)
	}
	if got := f.calls("a"); got != 1 {
		t.Fatalf("calls before clear = %d, want 1", got)
	}
	sc.Clear()
	// Após Clear, um Load volta a buscar (miss).
	if _, err := sc.Load(context.Background(), "a"); err != nil {
		t.Fatalf("load after clear: %v", err)
	}
	if got := f.calls("a"); got != 2 {
		t.Fatalf("calls after clear+load = %d, want 2", got)
	}
}
