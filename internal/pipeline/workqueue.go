package pipeline

import (
	"container/heap"
	"sync"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Rate limiting
// ─────────────────────────────────────────────────────────────────────────────

// RateLimiter maps an item to a delay and tracks its requeue count. It is the
// "When / Forget / NumRequeues" contract from the Kubernetes workqueue: success
// calls Forget (clears the failure count), failure calls AddRateLimited (which
// consults When for a growing delay).
type RateLimiter interface {
	// When returns how long item must wait before being eligible again. It is
	// expected to advance an internal failure count on each call.
	When(item string) time.Duration
	// Forget clears the failure count for item (a success).
	Forget(item string)
	// NumRequeues reports how many times item has been rate-limited.
	NumRequeues(item string) int
}

// ItemExponentialFailureRateLimiter backs off a per-item failure count as
// baseDelay * 2^(failures-1), capped at maxDelay. This is the same policy as
// Kubernetes' ItemExponentialFailureRateLimiter (5ms base / 1000s cap).
type ItemExponentialFailureRateLimiter struct {
	mu        sync.Mutex
	failures  map[string]int
	baseDelay time.Duration
	maxDelay  time.Duration
}

// NewItemExponentialFailureRateLimiter returns a limiter with baseDelay and
// maxDelay. Non-positive values fall back to sane defaults (1ms base, 1000x).
func NewItemExponentialFailureRateLimiter(baseDelay, maxDelay time.Duration) *ItemExponentialFailureRateLimiter {
	if baseDelay <= 0 {
		baseDelay = time.Millisecond
	}
	if maxDelay <= baseDelay {
		maxDelay = baseDelay * 1000
	}
	return &ItemExponentialFailureRateLimiter{
		failures:  map[string]int{},
		baseDelay: baseDelay,
		maxDelay:  maxDelay,
	}
}

func (r *ItemExponentialFailureRateLimiter) When(item string) time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	exp := r.failures[item]
	r.failures[item] = exp + 1

	d := r.baseDelay
	for i := 0; i < exp; i++ {
		d *= 2
		if d >= r.maxDelay {
			return r.maxDelay
		}
	}
	return d
}

func (r *ItemExponentialFailureRateLimiter) Forget(item string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.failures, item)
}

func (r *ItemExponentialFailureRateLimiter) NumRequeues(item string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.failures[item]
}

// MaxOfRateLimiter returns the worst-case (largest) delay of the given
// limiters. Kubernetes composes a per-item exponential limiter with a global
// bucket limiter this way.
type MaxOfRateLimiter struct {
	limiters []RateLimiter
}

func NewMaxOfRateLimiter(limiters ...RateLimiter) *MaxOfRateLimiter {
	return &MaxOfRateLimiter{limiters: limiters}
}

func (r *MaxOfRateLimiter) When(item string) time.Duration {
	var max time.Duration
	for _, l := range r.limiters {
		if d := l.When(item); d > max {
			max = d
		}
	}
	return max
}

func (r *MaxOfRateLimiter) Forget(item string) {
	for _, l := range r.limiters {
		l.Forget(item)
	}
}

func (r *MaxOfRateLimiter) NumRequeues(item string) int {
	max := 0
	for _, l := range r.limiters {
		if n := l.NumRequeues(item); n > max {
			max = n
		}
	}
	return max
}

// ─────────────────────────────────────────────────────────────────────────────
// Workqueue
// ─────────────────────────────────────────────────────────────────────────────

// waitEntry is a delayed item in the workqueue.
type waitEntry struct {
	data    string
	readyAt time.Time
	index   int // index in the heap
}

type waitHeap []*waitEntry

func (h waitHeap) Len() int            { return len(h) }
func (h waitHeap) Less(i, j int) bool  { return h[i].readyAt.Before(h[j].readyAt) }
func (h waitHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i]; h[i].index = i; h[j].index = j }
func (h *waitHeap) Push(x interface{}) { e := x.(*waitEntry); e.index = len(*h); *h = append(*h, e) }
func (h *waitHeap) Pop() interface{} {
	old := *h
	n := len(old)
	e := old[n-1]
	e.index = -1
	*h = old[:n-1]
	return e
}

// Workqueue is a keyed, deduplicating, rate-limited work queue. It stores string
// keys (not objects): callers keep the object store and look up items by key.
//
// Kubernetes Pattern (client-go util/workqueue): three layers — a base queue
// that deduplicates by key, a delaying layer (AddAfter) for scheduled requeues,
// and a rate-limiting layer (AddRateLimited) for per-item exponential backoff.
// All three are collapsed into this single concrete type for the pipeline.
type Workqueue struct {
	mu           sync.Mutex
	cond         *sync.Cond
	shuttingDown bool

	queue      []string
	dirty      map[string]struct{}
	processing map[string]struct{}

	limiter RateLimiter

	// Delaying queue state, serviced by delayingLoop.
	waiting   waitHeap
	waitingBy map[string]*waitEntry
	wakeup    chan struct{}
	done      chan struct{}
}

// NewWorkqueue returns a started Workqueue. limiter may be nil (no rate limit).
func NewWorkqueue(limiter RateLimiter) *Workqueue {
	q := &Workqueue{
		dirty:      map[string]struct{}{},
		processing: map[string]struct{}{},
		limiter:    limiter,
		waitingBy:  map[string]*waitEntry{},
		wakeup:     make(chan struct{}, 1),
		done:       make(chan struct{}),
	}
	q.cond = sync.NewCond(&q.mu)
	heap.Init(&q.waiting)
	go q.delayingLoop()
	return q
}

// Add enqueues item for immediate processing. It is idempotent: adding the same
// item twice before it is processed enqueues it once (dedup by key). Adding an
// item that is currently processing marks it dirty so Done() requeues it.
func (q *Workqueue) Add(item string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.shuttingDown {
		return
	}
	if _, ok := q.dirty[item]; ok {
		return
	}
	q.dirty[item] = struct{}{}
	if _, ok := q.processing[item]; ok {
		// Will be requeued by Done.
		return
	}
	q.queue = append(q.queue, item)
	q.cond.Signal()
}

// AddAfter schedules item to be enqueued after at least d. Scheduling the same
// item again keeps the earliest readyAt.
func (q *Workqueue) AddAfter(item string, d time.Duration) {
	if d <= 0 {
		q.Add(item)
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.shuttingDown {
		return
	}
	readyAt := time.Now().Add(d)
	if e, ok := q.waitingBy[item]; ok {
		if readyAt.Before(e.readyAt) {
			e.readyAt = readyAt
			heap.Fix(&q.waiting, e.index)
		}
		return
	}
	e := &waitEntry{data: item, readyAt: readyAt}
	q.waitingBy[item] = e
	heap.Push(&q.waiting, e)
	select {
	case q.wakeup <- struct{}{}:
	default:
	}
}

// AddRateLimited schedules item after limiter.When(item), and is the canonical
// retry path: call it on failure, Forget on success.
func (q *Workqueue) AddRateLimited(item string) {
	if q.limiter == nil {
		q.Add(item)
		return
	}
	q.AddAfter(item, q.limiter.When(item))
}

// Get blocks until an item is available or the queue is shutting down. The
// returned item is marked in-flight and must be released with Done (or Forget
// to also clear rate-limit state).
func (q *Workqueue) Get() (item string, shutdown bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.queue) == 0 && !q.shuttingDown {
		q.cond.Wait()
	}
	if len(q.queue) == 0 {
		return "", true
	}
	item = q.queue[0]
	q.queue[0] = ""
	q.queue = q.queue[1:]
	q.processing[item] = struct{}{}
	delete(q.dirty, item)
	return item, false
}

// Done releases item from processing. If the item was re-added while it was
// processing, it is requeued.
func (q *Workqueue) Done(item string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.processing, item)
	if _, ok := q.dirty[item]; ok {
		q.queue = append(q.queue, item)
		q.cond.Signal()
	}
}

// Forget clears the rate-limit state for item (call after a successful
// completion so the next failure starts backoff from scratch).
func (q *Workqueue) Forget(item string) {
	if q.limiter != nil {
		q.limiter.Forget(item)
	}
}

// NumRequeues reports how many times item has been rate-limited.
func (q *Workqueue) NumRequeues(item string) int {
	if q.limiter == nil {
		return 0
	}
	return q.limiter.NumRequeues(item)
}

// ShutDown stops the queue. Get returns shutdown=true once the queue is empty.
func (q *Workqueue) ShutDown() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.shuttingDown = true
	q.cond.Broadcast()
	select {
	case q.wakeup <- struct{}{}:
	default:
	}
}

// ShuttingDown reports whether ShutDown has been called.
func (q *Workqueue) ShuttingDown() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.shuttingDown
}

// Len reports the number of immediately-available items (excludes in-flight and
// delayed items). Intended for tests and observability.
func (q *Workqueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.queue)
}

// delayingLoop moves delayed items that have become ready into the queue.
func (q *Workqueue) delayingLoop() {
	defer close(q.done)
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	resetTimer := func(d time.Duration) {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(d)
	}

	for {
		q.mu.Lock()
		if q.shuttingDown {
			q.mu.Unlock()
			return
		}
		var delay time.Duration
		if len(q.waiting) > 0 {
			delay = time.Until(q.waiting[0].readyAt)
			if delay < 0 {
				delay = 0
			}
		} else {
			delay = time.Hour
		}
		q.mu.Unlock()

		if delay > 0 {
			resetTimer(delay)
			select {
			case <-q.wakeup:
				continue
			case <-timer.C:
			}
		}

		q.moveReady()
	}
}

// moveReady drains all delayed items whose readyAt has passed into the queue.
func (q *Workqueue) moveReady() {
	for {
		q.mu.Lock()
		if q.shuttingDown || len(q.waiting) == 0 || q.waiting[0].readyAt.After(time.Now()) {
			q.mu.Unlock()
			return
		}
		e := heap.Pop(&q.waiting).(*waitEntry)
		delete(q.waitingBy, e.data)
		item := e.data
		q.mu.Unlock()
		q.Add(item)
	}
}
