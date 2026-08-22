package compute

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// Circuit Breaker
// =============================================================================

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // Normal operation.
	CircuitOpen                         // Failing — reject all requests.
	CircuitHalfOpen                     // Testing recovery.
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "CLOSED"
	case CircuitOpen:
		return "OPEN"
	case CircuitHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreaker protects pools from cascading failures.
type CircuitBreaker struct {
	mu               sync.RWMutex
	breakers         map[string]*poolBreaker
	failureThreshold int
	successThreshold int
	openTimeout      time.Duration
}

type poolBreaker struct {
	state       CircuitState
	failures    int
	successes   int
	lastFailure time.Time
	openedAt    time.Time
}

// NewCircuitBreaker creates a breaker with sensible defaults.
func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		breakers:         make(map[string]*poolBreaker),
		failureThreshold: 5,
		successThreshold: 2,
		openTimeout:      30 * time.Second,
	}
}

// IsOpen returns true if the circuit is open for the given pool.
func (cb *CircuitBreaker) IsOpen(pool string) bool {
	cb.mu.RLock()
	pb, ok := cb.breakers[pool]
	cb.mu.RUnlock()
	if !ok {
		return false
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch pb.state {
	case CircuitClosed:
		return false
	case CircuitOpen:
		if time.Since(pb.openedAt) > cb.openTimeout {
			pb.state = CircuitHalfOpen
			pb.successes = 0
			return false // Allow one probe request.
		}
		return true
	case CircuitHalfOpen:
		return false
	default:
		return false
	}
}

// RecordSuccess notifies the breaker of a successful operation.
func (cb *CircuitBreaker) RecordSuccess(pool string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	pb := cb.ensureBreaker(pool)

	switch pb.state {
	case CircuitClosed:
		pb.failures = 0
	case CircuitHalfOpen:
		pb.successes++
		if pb.successes >= cb.successThreshold {
			pb.state = CircuitClosed
			pb.failures = 0
		}
	}
}

// RecordFailure notifies the breaker of a failed operation.
func (cb *CircuitBreaker) RecordFailure(pool string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	pb := cb.ensureBreaker(pool)
	now := time.Now()

	switch pb.state {
	case CircuitClosed:
		pb.failures++
		pb.lastFailure = now
		if pb.failures >= cb.failureThreshold {
			pb.state = CircuitOpen
			pb.openedAt = now
		}
	case CircuitHalfOpen:
		pb.state = CircuitOpen
		pb.openedAt = now
		pb.failures = 0
	}
}

// Status returns a summary of all breakers.
func (cb *CircuitBreaker) Status() string {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if len(cb.breakers) == 0 {
		return "no breakers"
	}

	var parts []string
	for name, pb := range cb.breakers {
		parts = append(parts, fmt.Sprintf("%s=%s", name, pb.state))
	}
	return strings.Join(parts, ", ")
}

func (cb *CircuitBreaker) ensureBreaker(pool string) *poolBreaker {
	pb, ok := cb.breakers[pool]
	if !ok {
		pb = &poolBreaker{state: CircuitClosed}
		cb.breakers[pool] = pb
	}
	return pb
}

// =============================================================================
// Rate Limiter
// =============================================================================

// RateLimiter implements a token bucket algorithm per pool.
type RateLimiter struct {
	mu      sync.RWMutex
	buckets map[string]*tokenBucket
	rate    float64 // Tokens per second.
	burst   int     // Max burst size.
}

type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
}

// NewRateLimiter creates a rate limiter with defaults.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*tokenBucket),
		rate:    100, // 100 tokens/sec default.
		burst:   10,  // 10 burst.
	}
}

// Allow returns true if a request is allowed for the given pool.
func (rl *RateLimiter) Allow(pool string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, ok := rl.buckets[pool]
	if !ok {
		bucket = &tokenBucket{
			tokens:     float64(rl.burst),
			lastRefill: time.Now(),
		}
		rl.buckets[pool] = bucket
	}

	// Refill tokens based on elapsed time.
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.tokens += elapsed * rl.rate
	if bucket.tokens > float64(rl.burst) {
		bucket.tokens = float64(rl.burst)
	}
	bucket.lastRefill = now

	// Consume a token if available.
	if bucket.tokens >= 1.0 {
		bucket.tokens -= 1.0
		return true
	}
	return false
}

// Status returns a rate limiter status summary.
func (rl *RateLimiter) Status() string {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	if len(rl.buckets) == 0 {
		return "no buckets"
	}

	var parts []string
	for name, bucket := range rl.buckets {
		parts = append(parts, fmt.Sprintf("%s=%.0f tokens", name, bucket.tokens))
	}
	return strings.Join(parts, ", ")
}

// =============================================================================
// Memory Budget
// =============================================================================

// MemoryBudget tracks memory allocation across pools and prevents OOM.
type MemoryBudget struct {
	mu          sync.RWMutex
	totalBytes  uint64
	usedBytes   uint64
	allocations map[string]uint64 // Per-pool allocation.
}

// NewMemoryBudget creates a memory budget tracker.
func NewMemoryBudget() *MemoryBudget {
	return &MemoryBudget{
		allocations: make(map[string]uint64),
	}
}

// SetTotalBytes sets the total available memory budget.
func (mb *MemoryBudget) SetTotalBytes(total uint64) {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	// Reserve 20% for system+Go runtime overhead.
	mb.totalBytes = uint64(float64(total) * 0.8)
}

// TryAllocate attempts to reserve memory for a pool. Returns true if successful.
func (mb *MemoryBudget) TryAllocate(pool string, bytes uint64) bool {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if mb.totalBytes == 0 {
		return true // No budget set — allow everything.
	}

	if mb.usedBytes+bytes > mb.totalBytes {
		return false
	}

	mb.usedBytes += bytes
	mb.allocations[pool] += bytes
	return true
}

// Release frees allocated memory.
func (mb *MemoryBudget) Release(pool string, bytes uint64) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if mb.usedBytes >= bytes {
		mb.usedBytes -= bytes
	}
	if mb.allocations[pool] >= bytes {
		mb.allocations[pool] -= bytes
	}
}

// TotalBytes returns the configured budget.
func (mb *MemoryBudget) TotalBytes() uint64 {
	mb.mu.RLock()
	defer mb.mu.RUnlock()
	return mb.totalBytes
}

// UsedBytes returns currently allocated bytes.
func (mb *MemoryBudget) UsedBytes() uint64 {
	mb.mu.RLock()
	defer mb.mu.RUnlock()
	return mb.usedBytes
}

// UsagePercent returns memory usage as a percentage (0-100).
func (mb *MemoryBudget) UsagePercent() float64 {
	mb.mu.RLock()
	defer mb.mu.RUnlock()
	if mb.totalBytes == 0 {
		return 0
	}
	return float64(mb.usedBytes) / float64(mb.totalBytes) * 100.0
}
