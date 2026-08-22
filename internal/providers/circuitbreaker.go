package providers

import (
	"time"

	"github.com/CoscaAI/cosca/internal/circuitbreaker"
)

// NewCircuitBreaker creates a circuit breaker for a provider with defaults.
func NewCircuitBreaker() *circuitbreaker.CircuitBreaker {
	cfg := circuitbreaker.DefaultConfig()
	cfg.FailureThreshold = 5
	cfg.SuccessThreshold = 2
	cfg.Timeout = 30 * time.Second
	return circuitbreaker.New(cfg)
}

// circuitBreakerStore manages circuit breakers per provider name.
type circuitBreakerStore struct {
	breakers map[string]*circuitbreaker.CircuitBreaker
}

func newCircuitBreakerStore() *circuitBreakerStore {
	return &circuitBreakerStore{
		breakers: make(map[string]*circuitbreaker.CircuitBreaker),
	}
}

func (s *circuitBreakerStore) Get(name string) *circuitbreaker.CircuitBreaker {
	if cb, ok := s.breakers[name]; ok {
		return cb
	}
	cb := NewCircuitBreaker()
	s.breakers[name] = cb
	return cb
}

func (s *circuitBreakerStore) Reset(name string) {
	if cb, ok := s.breakers[name]; ok {
		cb.Reset()
	}
}

func (s *circuitBreakerStore) ResetAll() {
	for _, cb := range s.breakers {
		cb.Reset()
	}
}
