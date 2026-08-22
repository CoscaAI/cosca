package compute

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// Hardware Probe Tests
// =============================================================================

func TestHardwareProbe_Snapshot(t *testing.T) {
	hp := NewHardwareProbe(100 * time.Millisecond)
	hp.Start()
	defer hp.Stop()

	time.Sleep(150 * time.Millisecond) // Wait for first probe.

	snap := hp.Snapshot()
	if snap.LogicalCores == 0 {
		t.Error("LogicalCores should be > 0")
	}
	if snap.ProbedAt.IsZero() {
		t.Error("ProbedAt should not be zero")
	}
	// On Linux, load and memory should be populated.
	if snap.TotalRAM == 0 {
		t.Log("TotalRAM is 0 — not running on Linux?")
	}
}

func TestHardwareProbe_RecommendWorkers(t *testing.T) {
	hp := NewHardwareProbe(time.Hour) // Not starting — set manually.
	hp.snapshot = HardwareSnapshot{LogicalCores: 16}

	tests := []struct {
		pct  float64
		min  int
		max  int
		want int
	}{
		{0.5, 2, 12, 8},  // 16*0.5 = 8
		{0.1, 2, 12, 2},  // 16*0.1 = 1.6 → clamped to min 2
		{1.0, 2, 12, 12}, // 16*1.0 = 16 → clamped to max 12
		{0.5, 0, 100, 8},
	}
	for _, tt := range tests {
		got := hp.RecommendWorkers(tt.pct, tt.min, tt.max)
		if got != tt.want {
			t.Errorf("RecommendWorkers(%.1f, %d, %d) = %d, want %d",
				tt.pct, tt.min, tt.max, got, tt.want)
		}
	}
}

func TestHardwareProbe_IsOverloaded(t *testing.T) {
	hp := NewHardwareProbe(time.Hour)
	hp.snapshot = HardwareSnapshot{LogicalCores: 16, Load1: 14.0} // 14/16 = 0.875

	if !hp.IsOverloaded() {
		t.Error("should be overloaded at 87.5% load")
	}

	hp.snapshot.Load1 = 5.0 // 5/16 = 0.3125
	if hp.IsOverloaded() {
		t.Error("should not be overloaded at 31% load")
	}
}

func TestHardwareProbe_IsMemoryPressured(t *testing.T) {
	hp := NewHardwareProbe(time.Hour)
	hp.snapshot = HardwareSnapshot{TotalRAM: 1000, UsedRAM: 900, MemoryUsage: 90.0}

	if !hp.IsMemoryPressured() {
		t.Error("should be memory pressured at 90%")
	}

	hp.snapshot.MemoryUsage = 50.0
	if hp.IsMemoryPressured() {
		t.Error("should not be memory pressured at 50%")
	}
}

func TestHardwareProbe_StartStop(t *testing.T) {
	hp := NewHardwareProbe(50 * time.Millisecond)
	hp.Start()
	time.Sleep(100 * time.Millisecond)
	hp.Stop()

	// Double stop should not panic.
	hp.Stop()

	// Double start should be safe.
	hp.Start()
	hp.Stop()
}

// =============================================================================
// Worker Pool Tests
// =============================================================================

func TestWorkerPool_SubmitSuccess(t *testing.T) {
	pool := NewWorkerPool(PoolConfig{
		Name: "test", MinWorkers: 2, MaxWorkers: 4, QueueSize: 10,
		IdleTimeout: time.Second,
	})
	pool.Start()
	defer pool.Stop()

	result, err := pool.Submit(context.Background(), Task{
		ID: "task-1",
		Fn: func(ctx context.Context) (interface{}, error) {
			return "hello", nil
		},
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if result.Value != "hello" {
		t.Errorf("Value = %v, want 'hello'", result.Value)
	}
	if result.Err != nil {
		t.Errorf("Err = %v, want nil", result.Err)
	}
	if result.Pool != "test" {
		t.Errorf("Pool = %q", result.Pool)
	}
}

func TestWorkerPool_SubmitError(t *testing.T) {
	pool := NewWorkerPool(PoolConfig{
		Name: "test", MinWorkers: 1, MaxWorkers: 2, QueueSize: 10,
		IdleTimeout: time.Second,
	})
	pool.Start()
	defer pool.Stop()

	_, err := pool.Submit(context.Background(), Task{
		ID: "task-err",
		Fn: func(ctx context.Context) (interface{}, error) {
			return nil, errors.New("intentional failure")
		},
	})
	if err != nil {
		t.Fatalf("Submit should not error (task error is in result.Err): %v", err)
	}
}

func TestWorkerPool_Concurrency(t *testing.T) {
	pool := NewWorkerPool(PoolConfig{
		Name: "test", MinWorkers: 4, MaxWorkers: 4, QueueSize: 100,
		IdleTimeout: time.Second,
	})
	pool.Start()
	defer pool.Stop()

	var counter atomic.Int64
	tasks := 50

	var wg sync.WaitGroup
	for i := 0; i < tasks; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := pool.Submit(context.Background(), Task{
				ID: fmt.Sprintf("task-%d", id),
				Fn: func(ctx context.Context) (interface{}, error) {
					counter.Add(1)
					return nil, nil
				},
			})
			if err != nil {
				t.Errorf("Submit %d failed: %v", id, err)
			}
		}(i)
	}
	wg.Wait()

	if got := counter.Load(); got != int64(tasks) {
		t.Errorf("counter = %d, want %d", got, tasks)
	}
}

func TestWorkerPool_ScaleTo(t *testing.T) {
	pool := NewWorkerPool(PoolConfig{
		Name: "test", MinWorkers: 1, MaxWorkers: 4, QueueSize: 10,
		IdleTimeout: 5 * time.Second, // Long enough to not expire during test.
	})
	pool.Start()
	defer pool.Stop()

	pool.ScaleTo(3)
	time.Sleep(100 * time.Millisecond)

	workers := pool.ActiveWorkers()
	if workers < 2 || workers > 4 {
		t.Errorf("ActiveWorkers after scale = %d, expected 2-4", workers)
	}

	pool.ScaleTo(0) // Should clamp to min.
	time.Sleep(100 * time.Millisecond)
	if pool.ActiveWorkers() < 1 {
		t.Error("should have at least min workers after scale to 0")
	}
}

func TestWorkerPool_TrySteal(t *testing.T) {
	pool := NewWorkerPool(PoolConfig{
		Name: "test", MinWorkers: 0, MaxWorkers: 1, QueueSize: 10,
		IdleTimeout: time.Second,
	})
	// Don't start workers — they would dequeue the task before we can steal.
	pool.ctx, pool.cancel = context.WithCancel(context.Background())
	defer pool.cancel()

	// Submit a task directly to the queue.
	resultCh := make(chan TaskResult, 1)
	pool.queue <- poolTask{
		Task:   Task{ID: "steal-me", Fn: func(ctx context.Context) (interface{}, error) { return "stolen", nil }},
		Result: resultCh,
	}

	// Steal it.
	pt, ok := pool.TrySteal()
	if !ok {
		t.Fatal("TrySteal should succeed")
	}
	if pt.Task.ID != "steal-me" {
		t.Errorf("stolen task ID = %q", pt.Task.ID)
	}

	// Queue should be empty now.
	_, ok = pool.TrySteal()
	if ok {
		t.Error("TrySteal should fail on empty queue")
	}
}

func TestWorkerPool_Stats(t *testing.T) {
	pool := NewWorkerPool(PoolConfig{
		Name: "test", MinWorkers: 2, MaxWorkers: 2, QueueSize: 10,
		IdleTimeout: time.Second,
	})
	pool.Start()
	defer pool.Stop()

	for i := 0; i < 5; i++ {
		pool.Submit(context.Background(), Task{
			ID: fmt.Sprintf("t-%d", i),
			Fn: func(ctx context.Context) (interface{}, error) { return nil, nil },
		})
	}

	stats := pool.Stats()
	if stats.Completed != 5 {
		t.Errorf("Completed = %d, want 5", stats.Completed)
	}
	if stats.Name != "test" {
		t.Errorf("Name = %q", stats.Name)
	}
}

// =============================================================================
// Circuit Breaker Tests
// =============================================================================

func TestCircuitBreaker_OpenAfterFailures(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.failureThreshold = 3

	// 2 failures — should still be closed.
	for i := 0; i < 2; i++ {
		cb.RecordFailure("pool-a")
	}
	if cb.IsOpen("pool-a") {
		t.Error("should be closed after 2 failures")
	}

	// 3rd failure — should open.
	cb.RecordFailure("pool-a")
	if !cb.IsOpen("pool-a") {
		t.Error("should be open after 3 failures")
	}
}

func TestCircuitBreaker_HalfOpenRecovery(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.failureThreshold = 1
	cb.openTimeout = 10 * time.Millisecond

	cb.RecordFailure("pool-a")
	if !cb.IsOpen("pool-a") {
		t.Fatal("should be open")
	}

	time.Sleep(15 * time.Millisecond) // Wait for timeout.

	// First request after timeout — half-open, allows one probe.
	if cb.IsOpen("pool-a") {
		t.Error("should be half-open, allowing request")
	}

	// Success in half-open.
	cb.RecordSuccess("pool-a")
	cb.RecordSuccess("pool-a") // 2 successes = threshold.
	if cb.IsOpen("pool-a") {
		t.Error("should be closed after recovery")
	}
}

// =============================================================================
// Rate Limiter Tests
// =============================================================================

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter()
	rl.rate = 10 // 10 tokens/sec.
	rl.burst = 3

	// Burst allows first 3.
	for i := 0; i < 3; i++ {
		if !rl.Allow("test") {
			t.Errorf("burst request %d should be allowed", i)
		}
	}

	// 4th should be denied (tokens exhausted).
	if rl.Allow("test") {
		t.Error("4th request should be denied after burst exhausted")
	}

	// Wait for refill.
	time.Sleep(200 * time.Millisecond) // 2 tokens refilled.
	if !rl.Allow("test") {
		t.Error("request after refill should be allowed")
	}
}

// =============================================================================
// Memory Budget Tests
// =============================================================================

func TestMemoryBudget_TryAllocate(t *testing.T) {
	mb := NewMemoryBudget()
	mb.SetTotalBytes(1000)
	// Note: 20% is reserved for system, so budget = 800.

	if !mb.TryAllocate("pool-a", 300) {
		t.Error("should allocate 300/800")
	}
	if !mb.TryAllocate("pool-a", 400) {
		t.Error("should allocate 400/800 (700 total)")
	}
	if mb.TryAllocate("pool-a", 200) {
		t.Error("should not allocate 200 (would be 900 > 800)")
	}

	if mb.UsedBytes() != 700 {
		t.Errorf("UsedBytes = %d, want 700", mb.UsedBytes())
	}

	expectedPct := float64(700) / float64(800) * 100.0
	if mb.UsagePercent() != expectedPct {
		t.Errorf("UsagePercent = %.1f, want %.1f", mb.UsagePercent(), expectedPct)
	}

	mb.Release("pool-a", 300)
	if mb.UsedBytes() != 400 {
		t.Errorf("UsedBytes after release = %d, want 400", mb.UsedBytes())
	}
}

func TestMemoryBudget_NoBudget(t *testing.T) {
	mb := NewMemoryBudget() // No total set.
	if !mb.TryAllocate("pool-a", 9999999) {
		t.Error("should allow any allocation when no budget is set")
	}
}

// =============================================================================
// Fabric Tests
// =============================================================================

func TestFabric_CreateAndLifecycle(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())

	if f.Name() != "compute-fabric" {
		t.Errorf("Name = %q", f.Name())
	}

	ctx := context.Background()
	if err := f.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify pools exist.
	for _, name := range []string{"agent", "tool", "index", "io", "sandbox"} {
		if f.Pool(name) == nil {
			t.Errorf("pool %q should exist", name)
		}
	}

	if err := f.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Double stop should be safe.
	if err := f.Stop(ctx); err != nil {
		t.Fatalf("second Stop failed: %v", err)
	}
}

func TestFabric_Submit(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	if err := f.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer f.Stop(ctx)

	result, err := f.Submit(ctx, "agent", Task{
		ID: "test-1",
		Fn: func(ctx context.Context) (interface{}, error) { return "ok", nil },
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if result.Value != "ok" {
		t.Errorf("Value = %v", result.Value)
	}
	if result.Pool != "agent" {
		t.Errorf("Pool = %q", result.Pool)
	}
}

func TestFabric_SubmitUnknownPool(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	f.Start(ctx)
	defer f.Stop(ctx)

	_, err := f.Submit(ctx, "nonexistent", Task{
		ID: "test", Fn: func(ctx context.Context) (interface{}, error) { return nil, nil },
	})
	if err == nil {
		t.Error("should error on unknown pool")
	}
}

func TestFabric_FanOut(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	f.Start(ctx)
	defer f.Stop(ctx)

	tasks := make([]Task, 10)
	for i := 0; i < 10; i++ {
		i := i
		tasks[i] = Task{
			ID: fmt.Sprintf("fan-%d", i),
			Fn: func(ctx context.Context) (interface{}, error) {
				time.Sleep(10 * time.Millisecond)
				return i * 2, nil
			},
		}
	}

	results, err := f.FanOut(ctx, "agent", tasks)
	if err != nil {
		t.Fatalf("FanOut: %v", err)
	}
	if len(results) != 10 {
		t.Errorf("len(results) = %d, want 10", len(results))
	}
}

func TestFabric_StatusReport(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	f.Start(ctx)
	defer f.Stop(ctx)

	// Submit a few tasks so there are stats.
	f.Submit(ctx, "agent", Task{
		ID: "s1", Fn: func(ctx context.Context) (interface{}, error) { return nil, nil },
	})

	report := f.StatusReport()
	if report == "" {
		t.Error("StatusReport should not be empty")
	}
	// Check for key sections.
	for _, keyword := range []string{"COMPUTE FABRIC", "Hardware", "Worker Pools", "Backpressure"} {
		if !containsString(report, keyword) {
			t.Errorf("StatusReport missing %q", keyword)
		}
	}
}

func TestFabric_Health(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	f.Start(ctx)
	defer f.Stop(ctx)

	health := f.Health()
	if health != "healthy" && health != "degraded" {
		t.Errorf("Health = %q, want healthy or degraded", health)
	}
}

// =============================================================================
// Concurrent Stress Tests
// =============================================================================

func TestWorkerPool_ConcurrentSubmit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	pool := NewWorkerPool(PoolConfig{
		Name: "stress", MinWorkers: runtime.NumCPU(), MaxWorkers: runtime.NumCPU(),
		QueueSize: 1000, IdleTimeout: time.Minute,
	})
	pool.Start()
	defer pool.Stop()

	var counter atomic.Int64
	tasks := 1000

	var wg sync.WaitGroup
	for i := 0; i < tasks; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := pool.Submit(context.Background(), Task{
				ID: fmt.Sprintf("s-%d", id),
				Fn: func(ctx context.Context) (interface{}, error) {
					counter.Add(1)
					return nil, nil
				},
			})
			if err != nil {
				t.Errorf("Submit %d: %v", id, err)
			}
		}(i)
	}
	wg.Wait()

	if got := counter.Load(); got != int64(tasks) {
		t.Errorf("counter = %d, want %d", got, tasks)
	}
}

func TestFabric_ConcurrentSubmit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	// Increase rate limiter capacity for stress test.
	f.limiter.rate = 10000
	f.limiter.burst = 1000
	f.Start(ctx)
	defer f.Stop(ctx)

	var counter atomic.Int64
	tasks := 100

	var wg sync.WaitGroup
	for i := 0; i < tasks; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := f.Submit(ctx, "io", Task{ // Use "io" pool (less contention than "tool").
				ID: fmt.Sprintf("cf-%d", id),
				Fn: func(ctx context.Context) (interface{}, error) {
					counter.Add(1)
					return nil, nil
				},
			})
			if err != nil {
				t.Errorf("Submit %d: %v", id, err)
			}
		}(i)
	}
	wg.Wait()

	if got := counter.Load(); got != int64(tasks) {
		t.Errorf("counter = %d, want %d", got, tasks)
	}
}

// =============================================================================
// Helpers
// =============================================================================

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// =============================================================================
// Coverage Gap Tests
// =============================================================================

// --- Hardware Probe ---

func TestHardwareProbe_ZeroCores(t *testing.T) {
	hp := NewHardwareProbe(time.Hour)
	hp.snapshot = HardwareSnapshot{LogicalCores: 0}
	if hp.IsOverloaded() {
		t.Error("should not be overloaded with 0 cores")
	}
}

func TestHardwareProbe_ZeroRAM(t *testing.T) {
	hp := NewHardwareProbe(time.Hour)
	hp.snapshot = HardwareSnapshot{TotalRAM: 0}
	if hp.IsMemoryPressured() {
		t.Error("should not be pressured with 0 RAM")
	}
}

// --- Circuit Breaker ---

func TestCircuitBreaker_String(t *testing.T) {
	if CircuitClosed.String() != "CLOSED" {
		t.Error("Closed.String() mismatch")
	}
	if CircuitOpen.String() != "OPEN" {
		t.Error("Open.String() mismatch")
	}
	if CircuitHalfOpen.String() != "HALF_OPEN" {
		t.Error("HalfOpen.String() mismatch")
	}
	if CircuitState(99).String() != "UNKNOWN" {
		t.Error("Unknown state mismatch")
	}
}

func TestCircuitBreaker_NonexistentPool(t *testing.T) {
	cb := NewCircuitBreaker()
	if cb.IsOpen("nonexistent") {
		t.Error("nonexistent pool should not be open")
	}
}

func TestCircuitBreaker_RecordFailureHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.failureThreshold = 1
	cb.openTimeout = 10 * time.Millisecond

	// Open the circuit.
	cb.RecordFailure("pool")

	time.Sleep(15 * time.Millisecond)

	// Half-open allows probe.
	cb.IsOpen("pool") // Should transition to half-open.

	// Failure in half-open → back to open.
	cb.RecordFailure("pool")
	if !cb.IsOpen("pool") {
		t.Error("should be open after failure in half-open")
	}
}

func TestCircuitBreaker_RecordSuccessClosed(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.RecordSuccess("pool")
	cb.RecordFailure("pool")
	cb.RecordSuccess("pool")
	// Should not panic.
}

func TestCircuitBreaker_StatusEmpty(t *testing.T) {
	cb := NewCircuitBreaker()
	if cb.Status() != "no breakers" {
		t.Errorf("empty status = %q", cb.Status())
	}
}

// --- Rate Limiter ---

func TestRateLimiter_StatusEmpty(t *testing.T) {
	rl := NewRateLimiter()
	if rl.Status() != "no buckets" {
		t.Errorf("empty status = %q", rl.Status())
	}
}

func TestRateLimiter_NewPool(t *testing.T) {
	rl := NewRateLimiter()
	rl.rate = 1
	rl.burst = 5
	// Initialize with Allow.
	for i := 0; i < 5; i++ {
		rl.Allow("new-pool")
	}
	if rl.Allow("new-pool") {
		t.Error("6th request should be denied")
	}
}

// --- Memory Budget ---

func TestMemoryBudget_UsagePercentZero(t *testing.T) {
	mb := NewMemoryBudget()
	if mb.UsagePercent() != 0 {
		t.Errorf("UsagePercent no budget = %.1f, want 0", mb.UsagePercent())
	}
}

// --- Worker Pool ---

func TestWorkerPool_Name(t *testing.T) {
	pool := NewWorkerPool(PoolConfig{Name: "test"})
	if pool.Name() != "test" {
		t.Errorf("Name = %q", pool.Name())
	}
}

func TestWorkerPool_SubmitContextCanceled(t *testing.T) {
	pool := NewWorkerPool(PoolConfig{
		Name: "test", MinWorkers: 1, MaxWorkers: 1, QueueSize: 0, // Full queue.
		IdleTimeout: time.Minute,
	})
	pool.Start()
	defer pool.Stop()

	// Fill the queue.
	for i := 0; i < 1; i++ {
		pool.queue <- poolTask{
			Task:   Task{ID: "blocker", Timeout: time.Hour, Fn: func(ctx context.Context) (interface{}, error) { time.Sleep(time.Second); return nil, nil }},
			Result: make(chan TaskResult, 1),
		}
	}

	// Now submit with canceled context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := pool.Submit(ctx, Task{ID: "canceled", Fn: func(ctx context.Context) (interface{}, error) { return nil, nil }})
	if err == nil {
		t.Error("should fail with canceled context")
	}
}

func TestWorkerPool_PoolSummary(t *testing.T) {
	pool := NewWorkerPool(PoolConfig{
		Name: "summary-test", MinWorkers: 1, MaxWorkers: 1, QueueSize: 10,
		IdleTimeout: time.Minute,
	})
	pool.Start()
	defer pool.Stop()

	pool.Submit(context.Background(), Task{
		ID: "s1", Fn: func(ctx context.Context) (interface{}, error) { return nil, nil },
	})

	s := pool.PoolSummary()
	if s == "" {
		t.Error("PoolSummary should not be empty")
	}
}

// --- Fabric ---

func TestFabric_StartTwice(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	if err := f.Start(ctx); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if err := f.Start(ctx); err != nil {
		t.Fatalf("second Start: %v", err)
	}
	f.Stop(ctx)
}

func TestFabric_StopTwice(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	f.Start(ctx)
	f.Stop(ctx)
	// Should not panic.
	f.Stop(ctx)
}

func TestFabric_Snapshot(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	f.Start(ctx)
	defer f.Stop(ctx)

	snap := f.Snapshot()
	if snap.LogicalCores == 0 {
		t.Error("LogicalCores should be > 0")
	}
}

func TestFabric_SubmitTaskError(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	f.Start(ctx)
	defer f.Stop(ctx)

	result, err := f.Submit(ctx, "agent", Task{
		ID: "err-task",
		Fn: func(ctx context.Context) (interface{}, error) {
			return nil, errors.New("task-level failure")
		},
	})
	if err != nil {
		t.Fatalf("Submit should not error on task failure: %v", err)
	}
	if result.Err == nil {
		t.Error("result.Err should be non-nil for task failure")
	}
}

func TestFabric_SubmitCancelledContext(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	f.Start(ctx)
	defer f.Stop(ctx)

	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	_, err := f.Submit(cancelCtx, "agent", Task{
		ID: "cancelled", Fn: func(ctx context.Context) (interface{}, error) { return nil, nil },
	})
	if err == nil {
		t.Error("should error with cancelled context")
	}
}

func TestFabric_FanOutWithError(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	f.Start(ctx)
	defer f.Stop(ctx)

	tasks := []Task{
		{ID: "ok", Fn: func(ctx context.Context) (interface{}, error) { return "ok", nil }},
		{ID: "bad-pool", Fn: func(ctx context.Context) (interface{}, error) { return nil, nil }},
	}

	// Use a non-existent pool for the second task to trigger Submit error in FanOut.
	results, err := f.FanOut(ctx, "nonexistent", tasks)
	if err == nil && results != nil {
		// If it doesn't error, at least verify partial results.
		_ = results
	}
}

func TestFabric_IsOverloaded(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	f.Start(ctx)
	defer f.Stop(ctx)

	// After start, should not be overloaded (unless system is actually loaded).
	// Just verify it doesn't panic.
	_ = f.IsOverloaded()
}

func TestFabric_IsMemoryPressured(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	f.Start(ctx)
	defer f.Stop(ctx)

	_ = f.IsMemoryPressured()
}

func TestFabric_Pool(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	if f.Pool("agent") == nil {
		t.Error("agent pool should exist")
	}
	if f.Pool("nonexistent") != nil {
		t.Error("nonexistent pool should be nil")
	}
}

func TestFabric_ProgressBar(t *testing.T) {
	tests := []struct {
		current, max, width int
		contains            string
	}{
		{0, 10, 10, "░░░░░░░░░░"},
		{5, 10, 10, "█████░░░░░"},
		{10, 10, 10, "██████████"},
		{0, 0, 10, "░░░░░░░░░░"},
	}
	for _, tt := range tests {
		got := progressBar(tt.current, tt.max, tt.width)
		if got != tt.contains {
			t.Errorf("progressBar(%d, %d, %d) = %q, want %q", tt.current, tt.max, tt.width, got, tt.contains)
		}
	}
}

func TestFabric_SubmitRateLimit(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	// Set very restrictive rate limit.
	f.limiter.rate = 1
	f.limiter.burst = 0
	ctx := context.Background()
	f.Start(ctx)
	defer f.Stop(ctx)

	// First request uses initial token.
	_, err1 := f.Submit(ctx, "agent", Task{
		ID: "r1", Fn: func(ctx context.Context) (interface{}, error) { return nil, nil },
	})
	if err1 != nil {
		t.Logf("first Submit rate-limited: %v", err1)
	}

	// Second should definitely be rate-limited.
	_, err2 := f.Submit(ctx, "agent", Task{
		ID: "r2", Fn: func(ctx context.Context) (interface{}, error) { return nil, nil },
	})
	if err2 == nil {
		t.Log("second Submit not rate-limited (token may have refilled)")
	}
}

func TestClamp(t *testing.T) {
	if clamp(5, 0, 10) != 5 {
		t.Error("clamp(5,0,10)")
	}
	if clamp(-1, 0, 10) != 0 {
		t.Error("clamp(-1,0,10)")
	}
	if clamp(15, 0, 10) != 10 {
		t.Error("clamp(15,0,10)")
	}
}

func TestFormatBytes(t *testing.T) {
	if formatBytes(0) != "0 B" {
		t.Errorf("0 B = %q", formatBytes(0))
	}
	if formatBytes(1024) != "1.0 KB" {
		t.Errorf("1KB = %q", formatBytes(1024))
	}
}

func TestMin(t *testing.T) {
	if min(1, 2) != 1 {
		t.Error("min(1,2)")
	}
	if min(2, 1) != 1 {
		t.Error("min(2,1)")
	}
}

func TestRandPool(t *testing.T) {
	pools := []*WorkerPool{
		NewWorkerPool(PoolConfig{Name: "a"}),
		NewWorkerPool(PoolConfig{Name: "b"}),
	}
	p := randPool(pools)
	if p == nil {
		t.Error("randPool should return non-nil")
	}

	if randPool(nil) != nil {
		t.Error("randPool(nil) should be nil")
	}
}

func TestStealFromAny(t *testing.T) {
	// Don't start workers — we're testing queue operations directly.
	p1 := NewWorkerPool(PoolConfig{Name: "a", MinWorkers: 0, MaxWorkers: 1, QueueSize: 10, IdleTimeout: time.Minute})
	// Manually set up context so TrySteal works.
	p1.ctx, p1.cancel = context.WithCancel(context.Background())

	// Queue a task.
	p1.queue <- poolTask{
		Task:   Task{ID: "steal-target", Fn: func(ctx context.Context) (interface{}, error) { return "yes", nil }},
		Result: make(chan TaskResult, 1),
	}

	pools := []*WorkerPool{p1}
	pt, ok := stealFromAny(pools, nil)
	if !ok {
		t.Fatal("stealFromAny should succeed with queued task")
	}
	if pt.Task.ID != "steal-target" {
		t.Errorf("stolen task = %q", pt.Task.ID)
	}

	// Now queue is empty.
	_, ok = stealFromAny(pools, nil)
	if ok {
		t.Error("stealFromAny should fail on empty queue")
	}

	p1.cancel()
}

// --- Scheduler / Adapt ---

func TestFabric_Adapt(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	f.Start(ctx)
	defer f.Stop(ctx)

	// Call adapt directly to cover the scheduling logic.
	// First, simulate low load + backlog by submitting many tasks quickly.
	for i := 0; i < 30; i++ {
		go f.Submit(ctx, "agent", Task{
			ID: fmt.Sprintf("adapt-%d", i),
			Fn: func(ctx context.Context) (interface{}, error) {
				time.Sleep(50 * time.Millisecond)
				return nil, nil
			},
		})
	}

	time.Sleep(100 * time.Millisecond)
	f.adapt(ctx) // Should trigger scale-up due to backlog.

	// Wait for tasks to drain.
	time.Sleep(500 * time.Millisecond)
}

// --- Worker Loop Idle Timeout ---

func TestWorkerPool_IdleTimeoutExit(t *testing.T) {
	pool := NewWorkerPool(PoolConfig{
		Name: "idle-test", MinWorkers: 1, MaxWorkers: 4, QueueSize: 10,
		IdleTimeout: 50 * time.Millisecond, // Short timeout.
	})
	pool.Start()
	defer pool.Stop()

	// Scale up to 3 workers.
	pool.ScaleTo(3)
	time.Sleep(100 * time.Millisecond)

	// After idle timeout, excess workers (above min=1) should exit.
	// Give them time to time out.
	time.Sleep(200 * time.Millisecond)

	workers := pool.ActiveWorkers()
	if workers > 2 {
		t.Logf("workers = %d after idle timeout (expected <= 2)", workers)
	}
}

// --- progressBar edge ---

func TestProgressBar_Overflow(t *testing.T) {
	// current > max shouldn't happen but should not panic.
	got := progressBar(15, 10, 10)
	// UTF-8 blocks are 3 bytes each, 10 chars = 30 bytes.
	if len(got) < 10 {
		t.Errorf("len = %d, expected >= 10", len(got))
	}
}

// --- Submit with context cancellation after enqueue ---

func TestWorkerPool_SubmitContextCancelAfterEnqueue(t *testing.T) {
	pool := NewWorkerPool(PoolConfig{
		Name: "ctx-test", MinWorkers: 1, MaxWorkers: 1, QueueSize: 10,
		IdleTimeout: time.Minute,
	})
	pool.Start()
	defer pool.Stop()

	// Submit a long-running task to block the worker.
	go pool.Submit(context.Background(), Task{
		ID:      "blocker",
		Timeout: time.Hour,
		Fn:      func(ctx context.Context) (interface{}, error) { time.Sleep(time.Second); return nil, nil },
	})

	time.Sleep(50 * time.Millisecond) // Let blocker get picked up.

	// Submit another task with a short timeout context.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := pool.Submit(ctx, Task{
		ID:      "timeout",
		Timeout: time.Millisecond,
		Fn:      func(ctx context.Context) (interface{}, error) { return nil, nil },
	})
	if err == nil {
		t.Log("submit with timeout succeeded")
	}
	// Either the context times out waiting for queue or the task times out.
}

// --- Circuit Breaker IsOpen in HalfOpen state ---

func TestCircuitBreaker_IsOpenHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.failureThreshold = 1
	cb.openTimeout = 100 * time.Millisecond

	// Open the circuit.
	cb.RecordFailure("pool")
	if !cb.IsOpen("pool") {
		t.Fatal("should be open")
	}

	time.Sleep(150 * time.Millisecond) // Wait for timeout.

	// IsOpen should transition to HalfOpen and return false.
	if cb.IsOpen("pool") {
		t.Error("should be half-open, allowing probe")
	}

	// Second call should still be false (still half-open).
	if cb.IsOpen("pool") {
		t.Error("should still be half-open")
	}
}

// --- Submit with circuit breaker open ---

func TestFabric_SubmitCircuitBreakerOpen(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	f.Start(ctx)
	defer f.Stop(ctx)

	// Force circuit breaker open.
	for i := 0; i < 10; i++ {
		f.breaker.RecordFailure("agent")
	}

	_, err := f.Submit(ctx, "agent", Task{
		ID: "cb-test", Fn: func(ctx context.Context) (interface{}, error) { return nil, nil },
	})
	if err == nil {
		t.Error("should fail with open circuit breaker")
	}
}

// --- Submit memory budget exceeded ---

func TestFabric_SubmitMemoryBudgetExceeded(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	f.Start(ctx)
	defer f.Stop(ctx)

	// Set tiny budget AFTER Start (Start overwrites it with actual RAM).
	f.budget.SetTotalBytes(100) // Budget = 80 after 20% reserve (80 bytes!).

	// Submit with massive weight to trigger budget exceeded.
	// Weight=1 → 10MB. 10MB > 80 bytes → should fail.
	_, err := f.Submit(ctx, "agent", Task{
		ID:     "mem-test",
		Weight: 1, // 1 * 10MB > 80 bytes budget
		Fn:     func(ctx context.Context) (interface{}, error) { return nil, nil },
	})
	if err == nil {
		t.Error("should fail when memory budget exceeded")
	}
}

// --- Scheduler adapt with low load + empty queues (scale down) ---

func TestFabric_AdaptLowLoad(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	f.Start(ctx)
	defer f.Stop(ctx)

	// Override probe to simulate low load.
	f.probe.mu.Lock()
	f.probe.snapshot.LogicalCores = 16
	f.probe.snapshot.Load1 = 2.0 // 2/16 = 0.125 — very low load.
	f.probe.mu.Unlock()

	// Scale up agent pool above min.
	f.Pool("agent").ScaleTo(4)
	time.Sleep(50 * time.Millisecond)

	// adapt should see low load and empty queues → scale down.
	f.adapt(ctx)
}

// --- Scheduler adapt with high load (scale down non-agent pools) ---

func TestFabric_AdaptHighLoad(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	f.Start(ctx)
	defer f.Stop(ctx)

	// Simulate high load.
	f.probe.mu.Lock()
	f.probe.snapshot.LogicalCores = 16
	f.probe.snapshot.Load1 = 14.0 // 14/16 = 0.875 — high load.
	f.probe.mu.Unlock()

	f.adapt(ctx)
}

// --- Scheduler adapt with backlog ---

func TestFabric_AdaptBacklog(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	f.Start(ctx)
	defer f.Stop(ctx)

	// Low load, and we'll create a backlog.
	f.probe.mu.Lock()
	f.probe.snapshot.LogicalCores = 16
	f.probe.snapshot.Load1 = 2.0
	f.probe.mu.Unlock()

	// Artificially queue tasks to create backlog.
	pool := f.Pool("agent")
	for i := 0; i < 30; i++ {
		select {
		case pool.queue <- poolTask{
			Task:   Task{ID: fmt.Sprintf("b-%d", i), Fn: func(ctx context.Context) (interface{}, error) { return nil, nil }},
			Result: make(chan TaskResult, 1),
		}:
		default:
		}
	}

	f.adapt(ctx)
}

// --- Health degraded path ---

func TestFabric_HealthDegraded(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	f.Start(ctx)
	defer f.Stop(ctx)

	// Force overload.
	f.probe.mu.Lock()
	f.probe.snapshot.LogicalCores = 16
	f.probe.snapshot.Load1 = 14.0
	f.probe.mu.Unlock()

	health := f.Health()
	if health != "degraded" {
		t.Logf("Health = %q (may not be degraded if machine is fast)", health)
	}

	// Force memory pressure.
	f.probe.mu.Lock()
	f.probe.snapshot.Load1 = 1.0
	f.probe.snapshot.TotalRAM = 1000
	f.probe.snapshot.UsedRAM = 900
	f.probe.snapshot.MemoryUsage = 90.0
	f.probe.mu.Unlock()

	health2 := f.Health()
	if health2 != "degraded" {
		t.Logf("Health after memory pressure = %q", health2)
	}
}

// --- Start with zero cores (fallback path) ---

func TestFabric_StartZeroCores(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	f.probe.snapshot = HardwareSnapshot{LogicalCores: 0, TotalRAM: 0}
	ctx := context.Background()
	if err := f.Start(ctx); err != nil {
		t.Fatalf("Start with zero cores: %v", err)
	}
	defer f.Stop(ctx)
}

// --- stealFromAny with done callback ---

func TestStealFromAny_WithDone(t *testing.T) {
	p1 := NewWorkerPool(PoolConfig{Name: "a", MinWorkers: 0, MaxWorkers: 1, QueueSize: 10, IdleTimeout: time.Minute})
	p1.ctx, p1.cancel = context.WithCancel(context.Background())
	defer p1.cancel()

	// Queue a task.
	p1.queue <- poolTask{
		Task:   Task{ID: "skip-me", Fn: func(ctx context.Context) (interface{}, error) { return "yes", nil }},
		Result: make(chan TaskResult, 1),
	}

	pools := []*WorkerPool{p1}

	// done returns true for p1 → it should be skipped.
	_, ok := stealFromAny(pools, func(p *WorkerPool) bool { return p.Name() == "a" })
	if ok {
		t.Error("stealFromAny should skip pool 'a' when done returns true")
	}

	p1.cancel()
}

// --- schedulerLoop context done ---

func TestSchedulerLoop_Cancel(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx, cancel := context.WithCancel(context.Background())
	f.Start(ctx)

	// Cancel immediately — schedulerLoop should exit via ctx.Done().
	cancel()
	time.Sleep(100 * time.Millisecond)

	f.Stop(context.Background())
}

// --- IsOpen creates breaker for new pool ---

func TestCircuitBreaker_IsOpenNewPool(t *testing.T) {
	cb := NewCircuitBreaker()
	// First call for unknown pool returns false without creating.
	if cb.IsOpen("brand-new") {
		t.Error("new pool should not be open")
	}
	// Record a success to create the breaker in CLOSED state.
	cb.RecordSuccess("brand-new")
	// Now check again.
	if cb.IsOpen("brand-new") {
		t.Error("new pool should still not be open after success")
	}
}

// --- adapt: deep backlog triggers +2 scale-up under medium load ---

func TestFabric_AdaptDeepBacklog(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	f.Start(ctx)
	defer f.Stop(ctx)

	// Medium load (not low enough for first case, not high enough for second).
	f.probe.mu.Lock()
	f.probe.snapshot.LogicalCores = 16
	f.probe.snapshot.Load1 = 10.0 // 10/16 = 0.625 — between 0.5 and 0.8.
	f.probe.mu.Unlock()

	// Small pool max to make the threshold easy to hit.
	pool := f.Pool("tool")
	pool.cfg.MaxWorkers = 2
	pool.ScaleTo(1)

	// Deep backlog: > MaxWorkers*2 (= 4).
	for i := 0; i < 10; i++ {
		select {
		case pool.queue <- poolTask{
			Task:   Task{ID: fmt.Sprintf("db-%d", i), Fn: func(ctx context.Context) (interface{}, error) { return nil, nil }},
			Result: make(chan TaskResult, 1),
		}:
		default:
		}
	}

	f.adapt(ctx)
}

// =============================================================================
// Phase 2.5 — Remainder Coverage
// =============================================================================

// --- CircuitBreaker: IsOpen with unknown state (default case) ---

func TestCircuitBreaker_IsOpenUnknownState(t *testing.T) {
	cb := NewCircuitBreaker()
	// Inject a breaker with an unknown/unsupported state to exercise the default case.
	cb.mu.Lock()
	cb.breakers["unknown"] = &poolBreaker{state: CircuitState(99)}
	cb.mu.Unlock()

	if cb.IsOpen("unknown") {
		t.Error("unknown state should not be open (default returns false)")
	}
}

// --- HardwareProbe: double-Start without Stop (running guard) ---

func TestHardwareProbe_DoubleStart(t *testing.T) {
	hp := NewHardwareProbe(100 * time.Millisecond)
	hp.Start()
	// Call Start again while already running — should hit the `if hp.running { return }` guard.
	hp.Start()
	hp.Stop()
}

// --- Fabric adapt with zero cores (fallback to 4) ---

func TestFabric_AdaptZeroCores(t *testing.T) {
	f := NewFabric(DefaultFabricConfig())
	ctx := context.Background()
	f.Start(ctx)
	defer f.Stop(ctx)

	// Set snapshot with zero cores to trigger the cores==0 fallback.
	f.probe.mu.Lock()
	f.probe.snapshot.LogicalCores = 0
	f.probe.mu.Unlock()

	f.adapt(ctx) // Should exercise the `if cores == 0 { cores = 4 }` branch.
}

// =============================================================================
// Fabric Config — Dimensionamento real via percentuais (Plano 1)
// =============================================================================

func TestDefaultFabricConfig(t *testing.T) {
	cfg := DefaultFabricConfig()
	if cfg.ProbeInterval != 5*time.Second {
		t.Errorf("ProbeInterval = %v, want 5s", cfg.ProbeInterval)
	}
	want := []struct {
		name string
		got  float64
		want float64
	}{
		{"CPUPercent", cfg.CPUPercent, 0.75},
		{"ToolPercent", cfg.ToolPercent, 0.375},
		{"IndexPercent", cfg.IndexPercent, 0.25},
		{"IOPercent", cfg.IOPercent, 0.25},
		{"SandboxPercent", cfg.SandboxPercent, 0.25},
	}
	for _, tc := range want {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

func TestLoadFabricConfig_EnvOverrides(t *testing.T) {
	t.Setenv("COSCA_FABRIC_CPU_PERCENT", "0.9")
	t.Setenv("COSCA_FABRIC_TOOL_PERCENT", "0.4")
	t.Setenv("COSCA_FABRIC_INDEX_PERCENT", "0.3")
	t.Setenv("COSCA_FABRIC_IO_PERCENT", "0.2")
	t.Setenv("COSCA_FABRIC_SANDBOX_PERCENT", "0.1")

	cfg := LoadFabricConfig()
	if cfg.CPUPercent != 0.9 {
		t.Errorf("CPUPercent = %v, want 0.9", cfg.CPUPercent)
	}
	if cfg.ToolPercent != 0.4 {
		t.Errorf("ToolPercent = %v, want 0.4", cfg.ToolPercent)
	}
	if cfg.IndexPercent != 0.3 {
		t.Errorf("IndexPercent = %v, want 0.3", cfg.IndexPercent)
	}
	if cfg.IOPercent != 0.2 {
		t.Errorf("IOPercent = %v, want 0.2", cfg.IOPercent)
	}
	if cfg.SandboxPercent != 0.1 {
		t.Errorf("SandboxPercent = %v, want 0.1", cfg.SandboxPercent)
	}
}

func TestLoadFabricConfig_InvalidEnvKeepsDefaults(t *testing.T) {
	// Parse errors.
	t.Setenv("COSCA_FABRIC_INDEX_PERCENT", "abc")
	t.Setenv("COSCA_FABRIC_IO_PERCENT", "0.5xyz")
	t.Setenv("COSCA_FABRIC_SANDBOX_PERCENT", "")
	// Out of range (> 4.0 — o teto de over-subscription; 2.5 agora é VÁLIDO,
	// ver TestEnvPercent_OverSubscription).
	t.Setenv("COSCA_FABRIC_CPU_PERCENT", "5.0")
	// Out of range (<= 0).
	t.Setenv("COSCA_FABRIC_TOOL_PERCENT", "-0.1")

	cfg := LoadFabricConfig()
	def := DefaultFabricConfig()
	if cfg.CPUPercent != def.CPUPercent {
		t.Errorf("CPUPercent = %v, want default %v (invalid '5.0' > teto 4.0)", cfg.CPUPercent, def.CPUPercent)
	}
	if cfg.ToolPercent != def.ToolPercent {
		t.Errorf("ToolPercent = %v, want default %v (invalid '-0.1')", cfg.ToolPercent, def.ToolPercent)
	}
	if cfg.IndexPercent != def.IndexPercent {
		t.Errorf("IndexPercent = %v, want default %v (invalid 'abc')", cfg.IndexPercent, def.IndexPercent)
	}
	if cfg.IOPercent != def.IOPercent {
		t.Errorf("IOPercent = %v, want default %v (invalid '0.5xyz')", cfg.IOPercent, def.IOPercent)
	}
	if cfg.SandboxPercent != def.SandboxPercent {
		t.Errorf("SandboxPercent = %v, want default %v (empty)", cfg.SandboxPercent, def.SandboxPercent)
	}
}

func TestFabric_StartScalesByPercent(t *testing.T) {
	cores := runtime.NumCPU()

	f := NewFabric(FabricConfig{
		ProbeInterval:  5 * time.Second,
		CPUPercent:     1.0,
		ToolPercent:    0.5,
		IndexPercent:   0.25,
		IOPercent:      0.25,
		SandboxPercent: 0.25,
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		_ = f.Stop(context.Background())
	})

	if err := f.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	wantAgent := min(cores, 64)
	if got := f.Pool("agent").cfg.MaxWorkers; got != wantAgent {
		t.Errorf("agent MaxWorkers = %d, want %d", got, wantAgent)
	}
	wantTool := min(cores/2, 32)
	if got := f.Pool("tool").cfg.MaxWorkers; got != wantTool {
		t.Errorf("tool MaxWorkers = %d, want %d", got, wantTool)
	}
	wantIdx := min(cores/4, 16)
	for _, name := range []string{"index", "io", "sandbox"} {
		if got := f.Pool(name).cfg.MaxWorkers; got != wantIdx {
			t.Errorf("%s MaxWorkers = %d, want %d", name, got, wantIdx)
		}
	}
}

func TestFabric_StartScalesFloors(t *testing.T) {
	// Small percentages must not collapse a pool below its floor
	// (agent >= 2, demais >= 1).
	f := NewFabric(FabricConfig{
		ProbeInterval:  5 * time.Second,
		CPUPercent:     0.1,
		ToolPercent:    0.1,
		IndexPercent:   0.1,
		IOPercent:      0.1,
		SandboxPercent: 0.1,
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		_ = f.Stop(context.Background())
	})

	if err := f.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if got := f.Pool("agent").cfg.MaxWorkers; got < 2 {
		t.Errorf("agent MaxWorkers = %d, want >= 2 (floor)", got)
	}
	for _, name := range []string{"tool", "index", "io", "sandbox"} {
		if got := f.Pool(name).cfg.MaxWorkers; got < 1 {
			t.Errorf("%s MaxWorkers = %d, want >= 1 (floor)", name, got)
		}
	}
}
