package pipeline

import (
	"testing"
	"time"
)

func TestWorkqueueFIFO(t *testing.T) {
	q := NewWorkqueue(nil)
	defer q.ShutDown()

	q.Add("a")
	q.Add("b")
	q.Add("c")

	got := []string{}
	for i := 0; i < 3; i++ {
		item, shutdown := q.Get()
		if shutdown {
			t.Fatalf("unexpected shutdown")
		}
		got = append(got, item)
		q.Done(item)
	}

	want := []string{"a", "b", "c"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestWorkqueueDedup(t *testing.T) {
	q := NewWorkqueue(nil)
	defer q.ShutDown()

	q.Add("a")
	q.Add("a") // duplicate — should be a no-op
	q.Add("a")

	if q.Len() != 1 {
		t.Fatalf("expected 1 queued item after dedup, got %d", q.Len())
	}

	item, shutdown := q.Get()
	if shutdown || item != "a" {
		t.Fatalf("got item=%q shutdown=%v", item, shutdown)
	}
	q.Done(item)

	if q.Len() != 0 {
		t.Fatalf("expected empty queue, got %d", q.Len())
	}
}

func TestWorkqueueRequeueAfterReadd(t *testing.T) {
	q := NewWorkqueue(nil)
	defer q.ShutDown()

	q.Add("a")
	item, _ := q.Get() // a is now in-flight

	// Re-add while processing → marks dirty; Done requeues it.
	q.Add("a")
	q.Done(item)

	if q.Len() != 1 {
		t.Fatalf("expected item requeued after re-add, got len %d", q.Len())
	}
	item2, shutdown := q.Get()
	if shutdown || item2 != "a" {
		t.Fatalf("expected a to be requeued")
	}
	q.Done(item2)
}

func TestWorkqueueAddAfter(t *testing.T) {
	q := NewWorkqueue(nil)
	defer q.ShutDown()

	q.AddAfter("late", 60*time.Millisecond)

	if q.Len() != 0 {
		t.Fatalf("delayed item should not be immediately available, got len %d", q.Len())
	}

	// It becomes available after the delay.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if q.Len() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if q.Len() != 1 {
		t.Fatalf("delayed item should be available after delay, got len %d", q.Len())
	}

	item, shutdown := q.Get()
	if shutdown || item != "late" {
		t.Fatalf("got item=%q shutdown=%v", item, shutdown)
	}
	q.Done(item)
}

func TestItemExponentialFailureRateLimiter(t *testing.T) {
	r := NewItemExponentialFailureRateLimiter(5*time.Millisecond, time.Second)

	// 5ms, 10ms, 20ms, 40ms, ... capped at 1s.
	steps := []time.Duration{5 * time.Millisecond, 10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond}
	for i, want := range steps {
		got := r.When("x")
		if got != want {
			t.Fatalf("step %d: got %v, want %v", i, got, want)
		}
	}

	// Cap: keep doubling until max.
	var last time.Duration
	for i := 0; i < 20; i++ {
		last = r.When("x")
		if last > time.Second {
			t.Fatalf("delay exceeded cap: %v", last)
		}
	}
	if last != time.Second {
		t.Fatalf("expected cap %v, got %v", time.Second, last)
	}

	// Forget resets.
	r.Forget("x")
	if got := r.When("x"); got != 5*time.Millisecond {
		t.Fatalf("after Forget, expected 5ms, got %v", got)
	}
	if n := r.NumRequeues("x"); n != 1 {
		t.Fatalf("after reset+When, NumRequeues should be 1, got %d", n)
	}
}

func TestWorkqueueAddRateLimitedAndForget(t *testing.T) {
	r := NewItemExponentialFailureRateLimiter(5*time.Millisecond, time.Second)
	q := NewWorkqueue(r)
	defer q.ShutDown()

	// First failure: 5ms backoff.
	q.AddRateLimited("a")
	if q.NumRequeues("a") != 1 {
		t.Fatalf("expected 1 requeue, got %d", q.NumRequeues("a"))
	}

	// Forget clears; next failure starts over.
	q.Forget("a")
	if q.NumRequeues("a") != 0 {
		t.Fatalf("expected 0 requeues after Forget, got %d", q.NumRequeues("a"))
	}
}

func TestWorkqueueShutdown(t *testing.T) {
	q := NewWorkqueue(nil)
	q.ShutDown()

	_, shutdown := q.Get()
	if !shutdown {
		t.Fatalf("expected shutdown=true after ShutDown")
	}
	if q.ShuttingDown() != true {
		t.Fatalf("expected ShuttingDown to be true")
	}
}
