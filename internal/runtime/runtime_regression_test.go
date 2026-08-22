package runtime

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLifecycleDefaultsAreNotDuplicated(t *testing.T) {
	r := New()
	if err := r.lifecycle.ExecuteInit(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	first := len(r.lifecycle.initHooks)
	if err := r.lifecycle.ExecuteInit(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if got := len(r.lifecycle.initHooks); got != first {
		t.Fatalf("init hooks duplicated: first=%d second=%d", first, got)
	}
}

func TestRestartRecreatesShutdownChannel(t *testing.T) {
	r := New(WithConfig(func() RuntimeConfig {
		c := DefaultRuntimeConfig()
		c.HealthCheckInterval = time.Hour
		c.ShutdownWaitTimeout = time.Second
		return c
	}()))
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	old := r.ShutdownCh()
	if err := r.Restart(context.Background()); err != nil {
		t.Fatal(err)
	}
	if old == r.ShutdownCh() {
		t.Fatal("restart reused closed shutdown channel")
	}
	if err := r.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestStartStopAreSerialized(t *testing.T) {
	r := New()
	var starts atomic.Int32
	r.lifecycle.AddInitHook("gate", func(ctx context.Context, _ *Runtime) error {
		starts.Add(1)
		<-ctx.Done()
		return ctx.Err()
	}, 10*time.Millisecond, false)

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = r.Start(context.Background()) }()
	}
	wg.Wait()
	if starts.Load() != 1 {
		t.Fatalf("start operation ran %d times", starts.Load())
	}
}
