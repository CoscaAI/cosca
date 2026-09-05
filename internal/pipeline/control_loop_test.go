package pipeline

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// TestControlLoop_ReconcilesKeys: verifica que o loop processa as keys enfileiradas.
func TestControlLoop_ReconcilesKeys(t *testing.T) {
	var processed atomic.Int32
	loop := NewControlLoop(func(_ context.Context, key string) error {
		processed.Add(1)
		return nil
	}, 2, 3)

	ctx, cancel := context.WithCancel(context.Background())
	go loop.Run(ctx)
	loop.Enqueue("a")
	loop.Enqueue("b")
	loop.Enqueue("c")

	// Aguarda processar (poll com timeout).
	deadline := time.Now().Add(2 * time.Second)
	for processed.Load() < 3 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	loop.ShutDown()
	cancel()
	if processed.Load() != 3 {
		t.Fatalf("esperava processar 3 keys, got %d", processed.Load())
	}
}

// TestControlLoop_FailureRequeues: falha → AddRateLimited; sucesso → Forget.
func TestControlLoop_FailureThenSuccess(t *testing.T) {
	var first atomic.Bool
	var attempts atomic.Int32
	loop := NewControlLoop(func(_ context.Context, key string) error {
		n := attempts.Add(1)
		if n == 1 {
			return errors.New("boom") // primeira falha → requeue
		}
		if !first.CompareAndSwap(false, true) {
			t.Fatal("key não deveria rodar concorrentemente (exclusão por key)")
		}
		return nil
	}, 1, 3)

	ctx, cancel := context.WithCancel(context.Background())
	go loop.Run(ctx)
	loop.Enqueue("k")

	deadline := time.Now().Add(2 * time.Second)
	for attempts.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	loop.ShutDown()
	cancel()
	if attempts.Load() < 2 {
		t.Fatalf("esperava >=2 tentativas (1 falha + retry), got %d", attempts.Load())
	}
}

// TestControlLoop_DropAfterMaxRetries: falha persistente → drop observável.
func TestControlLoop_DropAfterMaxRetries(t *testing.T) {
	var dropped atomic.Int32
	loop := NewControlLoop(func(_ context.Context, key string) error {
		return errors.New("sempre falha")
	}, 1, 2)
	loop.OnDrop(func(key string, err error) { dropped.Add(1) })

	ctx, cancel := context.WithCancel(context.Background())
	go loop.Run(ctx)
	loop.Enqueue("k")

	deadline := time.Now().Add(2 * time.Second)
	for dropped.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	loop.ShutDown()
	cancel()
	if dropped.Load() == 0 {
		t.Fatal("falha persistente deveria dropar após maxRetries (fail-closed)")
	}
}

// TestControlLoop_DedupByKey: Add de key repetida não duplica o processamento
// em paralelo (a workqueue exclui por key).
func TestControlLoop_DedupByKey(t *testing.T) {
	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32
	loop := NewControlLoop(func(_ context.Context, _ string) error {
		c := concurrent.Add(1)
		for {
			cur := maxConcurrent.Load()
			if c <= cur || maxConcurrent.CompareAndSwap(cur, c) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		concurrent.Add(-1)
		return nil
	}, 2, 3)

	ctx, cancel := context.WithCancel(context.Background())
	go loop.Run(ctx)
	for i := 0; i < 5; i++ {
		loop.Enqueue("mesma-key") // dedup → 1 processamento
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if loop.Len() == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	loop.ShutDown()
	cancel()
	if maxConcurrent.Load() > 1 {
		t.Fatalf("mesma key não deve rodar concorrentemente, got max=%d", maxConcurrent.Load())
	}
}
