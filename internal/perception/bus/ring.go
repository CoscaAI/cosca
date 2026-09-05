package bus

import (
	"sort"
	"sync"
	"time"
)

// ──────────────────────────────────────────────────────────────
// Defaults
// ──────────────────────────────────────────────────────────────

const (
	// DefaultWindow is the default temporal window the ring keeps (5s). This is
	// the "what was being seen when I heard X" horizon: enough to cover a spoken
	// sentence at a 2s vision cadence.
	DefaultWindow = 5 * time.Second
	// DefaultMaxObs is the hard capacity of the ring (O(1) eviction at this cap,
	// regardless of window length). At the default window this comfortably holds
	// realtime + normal-cadence frames without unbounded growth.
	DefaultMaxObs = 256
)

// ──────────────────────────────────────────────────────────────
// Ring
// ──────────────────────────────────────────────────────────────

// Ring is an O(1) circular temporal buffer of recent Observations. It evicts by
// two rules:
//
//  1. Time window: an observation older than `window` (relative to the newest
//     observation pushed) is dropped.
//  2. Capacity: when `maxObs` is reached, the oldest observation is overwritten
//     (a hard cap that guarantees bounded memory no matter the window).
//
// Not concurrent-safe on its own: the owning Bus serialises access via the
// ring's internal mutex (the ring is safe for concurrent Push/Snapshots).
type Ring struct {
	mu     sync.Mutex
	buf    []Observation
	start  int // index of the oldest entry
	size   int // number of valid entries
	max    int // capacity (hard cap)
	window time.Duration
	newest MonotonicTime
}

// NewRing creates a Ring with the given temporal window and hard capacity. A
// window <= 0 disables time-based eviction (capacity-only). A max <= 0 uses
// DefaultMaxObs.
func NewRing(window time.Duration, max int) *Ring {
	if window < 0 {
		window = 0
	}
	if max <= 0 {
		max = DefaultMaxObs
	}
	return &Ring{
		buf:    make([]Observation, max),
		max:    max,
		window: window,
	}
}

// Push adds a single observation. It updates the newest timestamp and evicts
// stale entries (time window + capacity).
func (r *Ring) Push(o Observation) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pushLocked(o)
	r.evictLocked()
}

// PushN adds several observations and evicts once at the end.
func (r *Ring) PushN(obs ...Observation) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, o := range obs {
		r.pushLocked(o)
	}
	r.evictLocked()
}

// pushLocked inserts an observation. Caller must hold r.mu.
func (r *Ring) pushLocked(o Observation) {
	if o.Timestamp > r.newest {
		r.newest = o.Timestamp
	}
	if r.size < r.max {
		r.buf[(r.start+r.size)%r.max] = o
		r.size++
		return
	}
	// Full: overwrite the oldest entry.
	r.buf[r.start] = o
	r.start = (r.start + 1) % r.max
}

// evictLocked drops entries older than the window (relative to newest). Caller
// must hold r.mu.
func (r *Ring) evictLocked() {
	if r.window <= 0 {
		return
	}
	limit := r.newest - ToMonotonic(r.window)
	for r.size > 0 {
		oldest := r.atLocked(0)
		if oldest.Timestamp < limit {
			r.popOldestLocked()
			continue
		}
		break
	}
}

// popOldestLocked removes and discards the oldest entry. Caller must hold r.mu.
func (r *Ring) popOldestLocked() {
	r.start = (r.start + 1) % r.max
	r.size--
}

// atLocked returns the i-th oldest entry (cycle-safe). Caller must hold r.mu
// and guarantee i < r.size.
func (r *Ring) atLocked(i int) Observation {
	return r.buf[(r.start+i)%r.max]
}

// Snapshots returns a defensive, timestamp-sorted copy of the live observations.
// The Observation structs are copied; Payload pointers are shared (read-only by
// convention — the bus never mutates a pushed payload).
func (r *Ring) Snapshots() []Observation {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Observation, 0, r.size)
	for i := 0; i < r.size; i++ {
		out = append(out, r.atLocked(i))
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Timestamp < out[j].Timestamp
	})
	return out
}

// Since returns a defensive copy of the observations with Timestamp >= t,
// sorted by timestamp.
func (r *Ring) Since(t MonotonicTime) []Observation {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Observation, 0, r.size)
	for i := 0; i < r.size; i++ {
		o := r.atLocked(i)
		if o.Timestamp >= t {
			out = append(out, o)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Timestamp < out[j].Timestamp
	})
	return out
}

// WindowAround returns a defensive copy of the observations whose timestamp
// falls in [t-before, t+after], sorted by timestamp. This is the primitive that
// answers "what was being seen/heard around instant t" — e.g. the vision frames
// around a spoken word.
//
// before/after are durations; a negative value clamps to zero.
func (r *Ring) WindowAround(t MonotonicTime, before, after time.Duration) []Observation {
	if before < 0 {
		before = 0
	}
	if after < 0 {
		after = 0
	}
	lo := t - ToMonotonic(before)
	hi := t + ToMonotonic(after)
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Observation, 0, r.size)
	for i := 0; i < r.size; i++ {
		o := r.atLocked(i)
		if o.Timestamp >= lo && o.Timestamp <= hi {
			out = append(out, o)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Timestamp < out[j].Timestamp
	})
	return out
}

// Len returns the current number of live observations.
func (r *Ring) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.size
}

// Cap returns the hard capacity.
func (r *Ring) Cap() int { return r.max }
