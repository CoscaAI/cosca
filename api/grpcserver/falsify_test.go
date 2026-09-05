package grpcserver

import (
	"context"
	"testing"

	cospb "github.com/CoscaAI/cosca/api/grpc/pb"
)

// ── LOOP V5: FALSIFICAÇÃO — domínio gRPC ───────────────────────────────
//
// Previsão: gRPC NÃO vulnerável (os service servers podem delegar ou usar
// construtores que garantem deps). Testar com zero-value para FALSIFICAR.

func scanGRPCPanic(t *testing.T, name string, fn func() error) (panicked bool, err error) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			panicked = true
		}
	}()
	return false, fn()
}

func TestFalsify_KnowledgeServiceNilEngine(t *testing.T) {
	s := &KnowledgeServiceServer{} // engine nil
	ctx := context.Background()
	panicked, _ := scanGRPCPanic(t, "KnowledgeService.Search", func() error {
		_, err := s.Search(ctx, &cospb.SearchRequest{Query: "x"})
		return err
	})
	if panicked {
		t.Fatal("gRPC KnowledgeService PANIC com engine nil — hipótese sobrevive neste domínio")
	}
}

func TestFalsify_MemoryServiceNilEngine(t *testing.T) {
	s := &MemoryServiceServer{} // engine nil
	ctx := context.Background()
	panicked, _ := scanGRPCPanic(t, "MemoryService.Store", func() error {
		_, err := s.Store(ctx, &cospb.StoreRequest{})
		return err
	})
	if panicked {
		t.Fatal("gRPC MemoryService PANIC com engine nil — hipótese sobrevive neste domínio")
	}
}

// Compile-time import guard.
