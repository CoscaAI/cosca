package providers

import (
	"testing"
)

func TestNewCircuitBreakerDefaults(t *testing.T) {
	cb := NewCircuitBreaker()
	if cb == nil {
		t.Fatal("nil circuit breaker")
	}
}

func TestCircuitBreakerStore(t *testing.T) {
	s := newCircuitBreakerStore()

	// First Get creates a breaker per name.
	cb1 := s.Get("openai")
	if cb1 == nil {
		t.Fatal("nil breaker")
	}
	// Second Get returns the same instance.
	if s.Get("openai") != cb1 {
		t.Fatal("breaker must be cached")
	}
	// Different name → different breaker.
	if s.Get("anthropic") == cb1 {
		t.Fatal("distinct breakers per name")
	}

	// Reset on a known name works; unknown name is a no-op.
	s.Reset("openai")
	s.Reset("ghost")

	// ResetAll resets every breaker.
	s.ResetAll()
}
