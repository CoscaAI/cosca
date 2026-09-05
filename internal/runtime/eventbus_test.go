package runtime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestNewEventBus(t *testing.T) {
	t.Parallel()
	eb := NewEventBus(zerolog.Nop())
	if eb == nil {
		t.Fatal("NewEventBus returned nil")
	}
	eb.mu.RLock()
	if len(eb.handlers) != 0 {
		t.Errorf("handlers should be empty, got %d", len(eb.handlers))
	}
	eb.mu.RUnlock()
}

func TestSubscribe(t *testing.T) {
	t.Parallel()
	eb := NewEventBus(zerolog.Nop())
	handler := func(_ context.Context, _ Event) error { return nil }
	eb.Subscribe(EventStateChange, handler)

	eb.mu.RLock()
	if len(eb.handlers[EventStateChange]) != 1 {
		t.Errorf("expected 1 handler, got %d", len(eb.handlers[EventStateChange]))
	}
	eb.mu.RUnlock()
}

func TestSubscribeMultipleHandlers(t *testing.T) {
	t.Parallel()
	eb := NewEventBus(zerolog.Nop())
	eb.Subscribe(EventSubsystemStarted, func(_ context.Context, _ Event) error { return nil })
	eb.Subscribe(EventSubsystemStarted, func(_ context.Context, _ Event) error { return nil })
	eb.Subscribe(EventSubsystemStarted, func(_ context.Context, _ Event) error { return nil })

	eb.mu.RLock()
	if len(eb.handlers[EventSubsystemStarted]) != 3 {
		t.Errorf("expected 3 handlers, got %d", len(eb.handlers[EventSubsystemStarted]))
	}
	eb.mu.RUnlock()
}

func TestUnsubscribe(t *testing.T) {
	t.Parallel()
	eb := NewEventBus(zerolog.Nop())
	eb.Subscribe(EventHealthChange, func(_ context.Context, _ Event) error { return nil })
	eb.Unsubscribe(EventHealthChange)

	eb.mu.RLock()
	if _, ok := eb.handlers[EventHealthChange]; ok {
		t.Error("handlers should be removed after Unsubscribe")
	}
	eb.mu.RUnlock()
}

func TestPublishCallsHandler(t *testing.T) {
	t.Parallel()
	eb := NewEventBus(zerolog.Nop())
	called := false
	eb.Subscribe(EventStateChange, func(_ context.Context, event Event) error {
		called = true
		if event.Type != EventStateChange {
			t.Errorf("event.Type = %v, want %v", event.Type, EventStateChange)
		}
		if event.Source != "test" {
			t.Errorf("event.Source = %q, want %q", event.Source, "test")
		}
		return nil
	})

	eb.Publish(context.Background(), EventStateChange, "test", "data")
	if !called {
		t.Error("handler was not called")
	}
}

func TestPublishNoHandlers(t *testing.T) {
	t.Parallel()
	eb := NewEventBus(zerolog.Nop())
	if eb == nil {
		t.Fatal("NewEventBus returned nil")
	}
	// Should not panic when publishing with no handlers registered
	eb.Publish(context.Background(), EventStateChange, "test", nil)
}

func TestPublishHandlerError(t *testing.T) {
	t.Parallel()
	eb := NewEventBus(zerolog.Nop())
	if eb == nil {
		t.Fatal("NewEventBus returned nil")
	}
	eb.Subscribe(EventSubsystemError, func(_ context.Context, _ Event) error {
		return errors.New("handler error")
	})
	// Should not panic despite handler returning error
	eb.Publish(context.Background(), EventSubsystemError, "test", "error data")
	// Check that handler count is non-zero (event was subscribed)
	eb.mu.RLock()
	handlerCount := len(eb.handlers[EventSubsystemError])
	eb.mu.RUnlock()
	if handlerCount != 1 {
		t.Errorf("handler count = %d, want 1", handlerCount)
	}
}

func TestPublishAllEventTypes(t *testing.T) {
	t.Parallel()
	eventTypes := []EventType{
		EventStateChange,
		EventSubsystemStarted,
		EventSubsystemStopped,
		EventSubsystemError,
		EventHealthChange,
		EventStartupComplete,
		EventShutdownInitiated,
		EventShutdownComplete,
		EventConfigReload,
	}
	for _, et := range eventTypes {
		t.Run(string(et), func(t *testing.T) {
			eb := NewEventBus(zerolog.Nop())
			called := false
			eb.Subscribe(et, func(_ context.Context, _ Event) error {
				called = true
				return nil
			})
			eb.Publish(context.Background(), et, "source", nil)
			if !called {
				t.Errorf("handler for %s was not called", et)
			}
		})
	}
}

func TestEventHasID(t *testing.T) {
	t.Parallel()
	eb := NewEventBus(zerolog.Nop())
	var capturedEvent Event
	eb.Subscribe(EventStateChange, func(_ context.Context, event Event) error {
		capturedEvent = event
		return nil
	})
	eb.Publish(context.Background(), EventStateChange, "src", "data")
	if capturedEvent.ID == "" {
		t.Error("Event ID should not be empty")
	}
	if capturedEvent.Timestamp.IsZero() {
		t.Error("Event Timestamp should be set")
	}
}

func TestPublishMultipleHandlers(t *testing.T) {
	t.Parallel()
	eb := NewEventBus(zerolog.Nop())
	var mu sync.Mutex
	count := 0
	for i := 0; i < 5; i++ {
		eb.Subscribe(EventStateChange, func(_ context.Context, _ Event) error {
			mu.Lock()
			count++
			mu.Unlock()
			return nil
		})
	}
	eb.Publish(context.Background(), EventStateChange, "test", nil)
	mu.Lock()
	if count != 5 {
		t.Errorf("expected 5 handler calls, got %d", count)
	}
	mu.Unlock()
}

func TestPublishTimeout(t *testing.T) {
	t.Parallel()
	eb := NewEventBus(zerolog.Nop())
	eb.Subscribe(EventStateChange, func(_ context.Context, _ Event) error {
		// This should be cancelled by the 10s timeout, but we don't block
		return nil
	})
	// Should complete quickly
	done := make(chan struct{})
	go func() {
		eb.Publish(context.Background(), EventStateChange, "test", nil)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("Publish should not block for more than 1 second")
	}
}

func TestDifferentEventTypes(t *testing.T) {
	t.Parallel()
	eb := NewEventBus(zerolog.Nop())
	stateCalled := false
	healthCalled := false
	eb.Subscribe(EventStateChange, func(_ context.Context, _ Event) error {
		stateCalled = true
		return nil
	})
	eb.Subscribe(EventHealthChange, func(_ context.Context, _ Event) error {
		healthCalled = true
		return nil
	})
	eb.Publish(context.Background(), EventStateChange, "t", nil)
	if !stateCalled {
		t.Error("state handler should be called")
	}
	if healthCalled {
		t.Error("health handler should NOT be called")
	}
}

func TestEventDataPayload(t *testing.T) {
	t.Parallel()
	eb := NewEventBus(zerolog.Nop())
	var data interface{}
	eb.Subscribe(EventConfigReload, func(_ context.Context, event Event) error {
		data = event.Data
		return nil
	})
	payload := map[string]string{"key": "value"}
	eb.Publish(context.Background(), EventConfigReload, "cfg", payload)
	got, ok := data.(map[string]string)
	if !ok {
		t.Fatalf("data type = %T, want map[string]string", data)
	}
	if got["key"] != "value" {
		t.Errorf("data[key] = %q, want %q", got["key"], "value")
	}
}
