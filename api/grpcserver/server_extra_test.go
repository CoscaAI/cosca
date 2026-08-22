package grpcserver

import (
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/rs/zerolog"
)

func TestSyncResultToPb(t *testing.T) {
	// Nil → resposta vazia, sem panic.
	if got := syncResultToPb(nil); got == nil {
		t.Fatal("syncResultToPb(nil) deve retornar resposta vazia")
	}
	// Válido → contadores.
	res := &knowledge.SyncResult{
		Added:   []string{"a", "b"},
		Updated: []string{"c"},
		Removed: nil,
	}
	pb := syncResultToPb(res)
	if pb.Added != 2 || pb.Updated != 1 || pb.Removed != 0 {
		t.Fatalf("syncResultToPb = %+v", pb)
	}
}

func TestNewServerWithNilEngines(t *testing.T) {
	// New com engines nil: não deve panicar (o server monta a árvore com
	// handlers que respondem erro/desligado).
	srv := New(nil, nil, nil, Config{Host: "127.0.0.1", Port: 0}, zerolog.Nop())
	if srv == nil {
		t.Fatal("New retornou nil")
	}
	if srv.Addr() == "" {
		t.Fatal("Addr vazio")
	}

	// Serve em porta efêmera e Stop: ciclo de vida completo.
	srv2 := New(nil, nil, nil, Config{Host: "127.0.0.1", Port: 0, JWTSecret: nil}, zerolog.Nop())
	go func() {
		_ = srv2.Serve()
	}()
	time.Sleep(50 * time.Millisecond) // deixa o servidor subir

	srv2.GracefulStop()
	srv2.Stop() // Stop é idempotente
}
