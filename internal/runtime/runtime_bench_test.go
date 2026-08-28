package runtime

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
)

func BenchmarkRuntimeStartup(b *testing.B) {
	for i := 0; b.Loop(); i++ {
		r := New()
		// Only test creation, not full Start/Stop cycle which is heavy
		// and requires subsystem setup
		_ = r
	}
}

func BenchmarkEventBusPublish(b *testing.B) {
	eb := NewEventBus(zerolog.Nop())
	handler := func(_ context.Context, _ Event) error { return nil }
	eb.Subscribe(EventStateChange, handler)
	eb.Subscribe(EventSubsystemStarted, handler)
	eb.Subscribe(EventSubsystemStopped, handler)

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		eb.Publish(ctx, EventStateChange, "benchmark", "test data")
	}
}

func BenchmarkEventBusSubscribe(b *testing.B) {
	eb := NewEventBus(zerolog.Nop())
	handler := func(_ context.Context, _ Event) error { return nil }

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		eb.Subscribe(EventStateChange, handler)
	}
}

func BenchmarkNewEventBus(b *testing.B) {
	for i := 0; b.Loop(); i++ {
		_ = NewEventBus(zerolog.Nop())
	}
}

func BenchmarkRuntimeStateTransition(b *testing.B) {
	r := New()

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_ = r.State().Current()
		_ = r.Health()
	}
}

func BenchmarkEventBusPublishMultipleHandlers(b *testing.B) {
	eb := NewEventBus(zerolog.Nop())
	handler := func(_ context.Context, _ Event) error { return nil }
	for i := 0; i < 10; i++ {
		eb.Subscribe(EventStateChange, handler)
	}

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		eb.Publish(ctx, EventStateChange, "benchmark", nil)
	}
}
