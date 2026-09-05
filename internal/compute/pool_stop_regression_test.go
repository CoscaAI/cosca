package compute

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// Regressão Bug 1 (raiz): pool.Stop() não pode travar para sempre com um
// worker preso em task que ignora contexto. Deve retornar após o drain
// timeout (5s) em vez de bloquear infinitamente.
func TestPoolStopDirect_Fixed(t *testing.T) {
	cfg := PoolConfig{Name: "test", MinWorkers: 1, MaxWorkers: 2, QueueSize: 10, IdleTimeout: time.Minute}
	wp := NewWorkerPool(cfg)
	wp.Start()

	blocked := make(chan struct{})
	release := make(chan struct{})
	taskDone := make(chan struct{})
	submitDone := make(chan struct{})
	go func() {
		defer close(submitDone)
		_, _ = wp.Submit(context.Background(), Task{
			Fn: func(ctx context.Context) (interface{}, error) {
				close(blocked)
				<-release
				close(taskDone)
				return nil, nil
			},
		})
	}()
	<-blocked

	start := time.Now()
	wp.Stop()
	elapsed := time.Since(start)
	if elapsed > 10*time.Second {
		t.Fatalf("pool.Stop levou %v — ainda trava (anti-freeze falhou)", elapsed)
	}
	t.Logf("pool.Stop retornou em %v (anti-freeze OK)", elapsed)

	close(release)
	<-taskDone
	<-submitDone
}

var _ = fmt.Sprintf
