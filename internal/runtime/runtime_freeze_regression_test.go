package runtime

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// Regressão Bug 2: Publish não pode bloquear o chamador.
func TestEventBusPublishNonBlocking_Fixed(t *testing.T) {
	eb := NewEventBus(zerolog.New(io.Discard))
	eb.Subscribe(EventHealthChange, func(ctx context.Context, e Event) error {
		<-ctx.Done() // handler lento (aguarda timeout de 10s)
		return nil
	})

	start := time.Now()
	eb.Publish(context.Background(), EventHealthChange, "test", nil)
	elapsed := time.Since(start)
	// Contrato corrigido: Publish NUNCA bloqueia além do deadline global
	// (3s). Antes da correção, um handler que não observa ctx bloqueava o
	// chamador PARA SEMPRE.
	if elapsed > 4*time.Second {
		t.Fatalf("Publish bloqueou o chamador por %v — deve respeitar o deadline global", elapsed)
	}
	t.Logf("Publish retornou em %v (limitado pelo deadline global, não infinito)", elapsed)
}

// Regressão Bug 6: Daemon.Stop idempotente (não pode panicar em close duplo).
func TestDaemonStopIdempotent_Fixed(t *testing.T) {
	d := NewDaemon(New(), DefaultDaemonConfig())
	if err := d.Stop(); err != nil { // Stop sem Start
		t.Fatalf("primeiro Stop falhou: %v", err)
	}
	if err := d.Stop(); err != nil { // segundo Stop — antes panica
		t.Fatalf("segundo Stop falhou: %v", err)
	}
}

// Regressão Bug 4: hook que ignora contexto não trava o shutdown.
func TestLifecycleHookTimeout_Fixed(t *testing.T) {
	l := NewLifecycle()
	r := New()

	// Hook que NUNCA termina (ignora contexto).
	l.AddStartHook("stuck", func(ctx context.Context, r *Runtime) error {
		select {} // trava para sempre
	}, 200*time.Millisecond, true)

	start := time.Now()
	err := l.ExecuteStart(context.Background(), r)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("hook travado deve gerar erro de timeout")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("ExecuteStart levou %v — o timeout não está sendo respeitado", elapsed)
	}
	t.Logf("ExecuteStart retornou em %v com erro: %v", elapsed, err)
}
