package compute

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// =============================================================================
// Worker Pool
// =============================================================================

// PoolConfig configures a worker pool.
type PoolConfig struct {
	Name        string        // Human-readable name ("agent", "tool", "index", "io", "sandbox")
	MinWorkers  int           // Floor — never scale below this
	MaxWorkers  int           // Ceiling — never scale above this
	QueueSize   int           // Buffer size for pending tasks
	IdleTimeout time.Duration // Worker exits after idle this long (only if > MinWorkers)
}

// Task is a unit of work submitted to a pool.
type Task struct {
	ID      string
	Weight  int           // CPU weight (1=light, 2=medium, 4=heavy)
	Timeout time.Duration // Per-task timeout
	Fn      func(ctx context.Context) (interface{}, error)
}

// TaskResult holds the outcome of a Task execution.
type TaskResult struct {
	TaskID string
	Value  interface{}
	Err    error
	Worker int
	Pool   string
	Start  time.Time
	End    time.Time
}

// WorkerPool manages a pool of goroutine workers for a specific task type.
// Supports dynamic scaling and work stealing.
type WorkerPool struct {
	cfg     PoolConfig
	mu      sync.RWMutex
	queue   chan poolTask
	workers int
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	// Metrics
	tasksSubmitted atomic.Int64
	tasksCompleted atomic.Int64
	tasksFailed    atomic.Int64
	totalLatencyNs atomic.Int64 // Sum of all task latencies
	stealsReceived atomic.Int64
	stealsGiven    atomic.Int64
}

type poolTask struct {
	Task   Task
	Result chan TaskResult
}

// NewWorkerPool creates a new worker pool.
func NewWorkerPool(cfg PoolConfig) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		cfg:    cfg,
		queue:  make(chan poolTask, cfg.QueueSize),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start launches the minimum number of workers.
func (wp *WorkerPool) Start() {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	for i := 0; i < wp.cfg.MinWorkers; i++ {
		wp.spawnWorker()
	}
}

// Stop gracefully shuts down all workers. Blocks until all finish OR the
// drain timeout elapses.
//
// CRITICAL FIX (audit 2026-08-01): the previous implementation did
// cancel() + wg.Wait() with no bound. A worker stuck inside a task that
// ignores its context (e.g. a task blocked on I/O that never observes
// cancellation) made Stop() block FOREVER — freezing the whole shutdown.
// Go cannot kill a goroutine from outside, so the only safe contract is:
// signal cancellation, wait for the drain window, then give up. Stuck
// workers are reaped when the process exits or the runtime context is
// cancelled. Workers that honor ctx exit promptly within the window.
func (wp *WorkerPool) Stop() {
	wp.cancel() // Signal all workers to exit.

	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All workers drained cleanly.
	case <-time.After(wp.stopDrainTimeout()):
		// Workers stuck in tasks that ignore ctx. Give up; the process
		// teardown will reclaim them. This is the anti-freeze guarantee.
	}
}

// stopDrainTimeout returns how long Stop() waits for workers to drain
// before giving up. Min workers that honor context exit in microseconds;
// the timeout only engages when a task ignores cancellation.
func (wp *WorkerPool) stopDrainTimeout() time.Duration {
	if wp.cfg.IdleTimeout > 0 && wp.cfg.IdleTimeout < 5*time.Second {
		return wp.cfg.IdleTimeout
	}
	return 5 * time.Second
}

// Submit enqueues a task. Blocks if the queue is full (backpressure).
func (wp *WorkerPool) Submit(ctx context.Context, task Task) (TaskResult, error) {
	wp.tasksSubmitted.Add(1)

	resultCh := make(chan TaskResult, 1)
	pt := poolTask{Task: task, Result: resultCh}

	select {
	case wp.queue <- pt:
	case <-ctx.Done():
		wp.tasksFailed.Add(1)
		return TaskResult{}, ctx.Err()
	}

	select {
	case result := <-resultCh:
		return result, nil
	case <-ctx.Done():
		wp.tasksFailed.Add(1)
		return TaskResult{}, ctx.Err()
	}
}

// TrySteal attempts to take a task from this pool's queue for another pool.
// Returns nil and false if the queue is empty.
func (wp *WorkerPool) TrySteal() (poolTask, bool) {
	select {
	case task := <-wp.queue:
		wp.stealsGiven.Add(1)
		return task, true
	default:
		return poolTask{}, false
	}
}

// ScaleTo adjusts the number of active workers. Respects min/max bounds.
func (wp *WorkerPool) ScaleTo(target int) int {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	target = clamp(target, wp.cfg.MinWorkers, wp.cfg.MaxWorkers)
	current := wp.workers

	if target > current {
		// Scale up — spawn new workers. Stops spawning once the pool
		// context is cancelled (Stop in progress) so no wg.Add races with
		// the Stop's wg.Wait.
		for i := current; i < target; i++ {
			if !wp.spawnWorker() {
				break
			}
		}
	} else if target < current {
		// Scale down — excess workers will exit via idle timeout or ctx cancellation.
		// We don't forcibly kill workers; they drain naturally.
	}

	return wp.workers
}

// ActiveWorkers returns the current worker count.
func (wp *WorkerPool) ActiveWorkers() int {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	return wp.workers
}

// QueueDepth returns the number of tasks waiting in the queue.
func (wp *WorkerPool) QueueDepth() int {
	return len(wp.queue)
}

// Name returns the pool name.
func (wp *WorkerPool) Name() string { return wp.cfg.Name }

// Stats returns current pool statistics.
func (wp *WorkerPool) Stats() PoolStats {
	completed := wp.tasksCompleted.Load()
	failed := wp.tasksFailed.Load()

	avgLatency := time.Duration(0)
	if completed > 0 {
		avgLatency = time.Duration(wp.totalLatencyNs.Load() / completed)
	}

	return PoolStats{
		Name:           wp.cfg.Name,
		ActiveWorkers:  wp.ActiveWorkers(),
		QueueDepth:     wp.QueueDepth(),
		Completed:      completed,
		Failed:         failed,
		AvgLatency:     avgLatency,
		StealsGiven:    wp.stealsGiven.Load(),
		StealsReceived: wp.stealsReceived.Load(),
	}
}

// PoolStats is a snapshot of pool statistics.
type PoolStats struct {
	Name           string
	ActiveWorkers  int
	QueueDepth     int
	Completed      int64
	Failed         int64
	AvgLatency     time.Duration
	StealsGiven    int64
	StealsReceived int64
}

// spawnWorker launches a new goroutine worker (caller must hold wp.mu).
//
// CRITICAL FIX (audit 2026-08-01): spawnWorker must not call wg.Add(1)
// while Stop()'s wg.Wait() is running — Go's WaitGroup forbids Add with a
// positive delta once Wait has started with a zero counter (data race,
// detected by -race). The ctx cancellation check makes Add race-free: after
// Stop() cancels the pool context, no new worker is added.
func (wp *WorkerPool) spawnWorker() bool {
	select {
	case <-wp.ctx.Done():
		return false // pool is stopping; do not add workers
	default:
	}
	id := wp.workers
	wp.workers++
	wp.wg.Add(1)
	go wp.workerLoop(id)
	return true
}

// workerLoop is the main loop for a single worker goroutine.
func (wp *WorkerPool) workerLoop(id int) {
	defer func() {
		wp.mu.Lock()
		wp.workers--
		wp.mu.Unlock()
		wp.wg.Done()
	}()

	// CRITICAL FIX (audit 2026-08-01): time.After(IdleTimeout) with a zero
	// IdleTimeout fired immediately, producing a busy-loop (100% CPU spin) for
	// workers at the minimum count. A reusable timer also avoids allocating a
	// new timer per idle iteration. IdleTimeout <= 0 disables idle exit
	// entirely (workers stay until ctx cancellation or pool Stop).
	var idle *time.Timer
	var idleC <-chan time.Time
	if wp.cfg.IdleTimeout > 0 {
		idle = time.NewTimer(wp.cfg.IdleTimeout)
		defer idle.Stop()
		idleC = idle.C
	}

	for {
		select {
		case <-wp.ctx.Done():
			return
		case pt := <-wp.queue:
			if idle != nil {
				if !idle.Stop() {
					select {
					case <-idle.C:
					default:
					}
				}
				idle.Reset(wp.cfg.IdleTimeout)
			}
			wp.executeTask(id, pt)
		case <-idleC:
			// Only exit if above min workers.
			wp.mu.RLock()
			aboveMin := wp.workers > wp.cfg.MinWorkers
			wp.mu.RUnlock()
			if aboveMin {
				return
			}
			if idle != nil {
				idle.Reset(wp.cfg.IdleTimeout)
			}
		}
	}
}

// executeTask runs a single task and sends the result.
func (wp *WorkerPool) executeTask(workerID int, pt poolTask) {
	start := time.Now()

	taskCtx := pt.Task.Timeout
	if taskCtx == 0 {
		taskCtx = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(wp.ctx, taskCtx)
	defer cancel()

	val, err := pt.Task.Fn(ctx)

	end := time.Now()
	latency := end.Sub(start)

	wp.totalLatencyNs.Add(int64(latency))
	if err != nil {
		wp.tasksFailed.Add(1)
	} else {
		wp.tasksCompleted.Add(1)
	}

	result := TaskResult{
		TaskID: pt.Task.ID,
		Value:  val,
		Err:    err,
		Worker: workerID,
		Pool:   wp.cfg.Name,
		Start:  start,
		End:    end,
	}

	select {
	case pt.Result <- result:
	default: // coverage: accept — Defensive; buffered channel always accepts first send.
		// Result channel full or closed — task was cancelled.
	}
}

// =============================================================================
// Helpers
// =============================================================================

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// randPool returns a random pool from the list (for work stealing).
func randPool(pools []*WorkerPool) *WorkerPool {
	if len(pools) == 0 {
		return nil
	}
	return pools[rand.Intn(len(pools))]
}

// stealFromAny attempts to steal a task from any pool in the list.
func stealFromAny(pools []*WorkerPool, done func(*WorkerPool) bool) (poolTask, bool) {
	// Randomize order for fairness.
	perm := rand.Perm(len(pools))
	for _, i := range perm {
		p := pools[i]
		if done != nil && done(p) {
			continue
		}
		if task, ok := p.TrySteal(); ok {
			return task, true
		}
	}
	return poolTask{}, false
}

// PoolSummary returns a human-readable summary of pool statistics.
func (wp *WorkerPool) PoolSummary() string {
	s := wp.Stats()
	return fmt.Sprintf("%s: %d/%d workers, queue=%d, completed=%d, failed=%d, avg_latency=%v",
		s.Name, s.ActiveWorkers, wp.cfg.MaxWorkers, s.QueueDepth,
		s.Completed, s.Failed, s.AvgLatency)
}
