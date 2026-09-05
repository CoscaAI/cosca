package providers

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter.
// It supports context cancellation via Wait/WaitN and uses sync.Mutex for concurrency safety.
type RateLimiter struct {
	mu         sync.Mutex
	tokens     int
	capacity   int
	interval   time.Duration
	lastRefill time.Time
}

// NewRateLimiter creates a new RateLimiter with the given tokens-per-minute capacity.
// The bucket starts full and refills proportionally over time.
func NewRateLimiter(tokensPerMinute int) *RateLimiter {
	if tokensPerMinute <= 0 {
		tokensPerMinute = 1000000 // sensible default: 1M TPM
	}
	return &RateLimiter{
		tokens:     tokensPerMinute,
		capacity:   tokensPerMinute,
		interval:   time.Minute,
		lastRefill: time.Now(),
	}
}

// Wait blocks until n tokens are available, or ctx is cancelled.
// It returns nil on success, or ctx.Err() if the context is cancelled.
func (r *RateLimiter) Wait(ctx context.Context, n int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Refill tokens proportionally to elapsed time since last refill.
	r.refill()

	for r.tokens < n {
		// Calculate how long to wait for the remaining tokens.
		needed := n - r.tokens
		waitDuration := time.Duration(float64(needed)/float64(r.capacity)) * r.interval

		// Release the lock while waiting so other goroutines can refill.
		r.mu.Unlock()
		timer := time.NewTimer(waitDuration)
		select {
		case <-timer.C:
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			r.mu.Lock()
			return ctx.Err()
		}
		r.mu.Lock()
		r.refill()
	}

	r.tokens -= n
	return nil
}

// WaitN is an alias for Wait(ctx, n).
func (r *RateLimiter) WaitN(ctx context.Context, n int) error {
	return r.Wait(ctx, n)
}

// refill adds tokens proportional to elapsed time.
// Must be called while r.mu is held.
func (r *RateLimiter) refill() {
	elapsed := time.Since(r.lastRefill)
	r.lastRefill = time.Now()
	r.tokens += int(float64(elapsed) / float64(r.interval) * float64(r.capacity))
	if r.tokens > r.capacity {
		r.tokens = r.capacity
	}
}
