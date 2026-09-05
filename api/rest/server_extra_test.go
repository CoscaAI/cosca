package rest

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestServerLifecycle(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Host = "127.0.0.1"
	cfg.Port = 0 // efêmero

	// Todos os managers nil: o server monta a árvore com handlers que
	// respondem erro/desligado — não pode panicar.
	srv := New(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, cfg,
		nil, nil, nil, nil, nil, false, nil, nil, nil, nil)

	if srv.Mux() == nil {
		t.Fatal("Mux nil")
	}
	if srv.Handler() == nil {
		t.Fatal("Handler nil")
	}
	if srv.Addr() == "" {
		t.Fatal("Addr vazio")
	}

	// Ciclo de vida: Serve em goroutine + Shutdown.
	go func() { _ = srv.Serve() }()
	time.Sleep(100 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	// ServeTLS com cert inválido → erro (sem panic).
	_ = srv.ServeTLS("/nonexistent.crt", "/nonexistent.key")

	// buildHandler: HTTP handler funcional.
	h := srv.Handler()
	if h == nil {
		t.Fatal("buildHandler retornou nil")
	}
	_ = http.Handler(h)
}
