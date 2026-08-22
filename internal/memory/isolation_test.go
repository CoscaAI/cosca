package memory

import (
	"context"
	"testing"
	"time"
)

// ── P0-6: Memory/context isolation — contaminação A → B → A ────────────
//
// Cenário do professor: sessão A grava informação, sessão B roda, sessão A
// consulta — B não pode contaminar A (A6/A7: OwnerFilter + AgentFilter).

func newTestEngine(t *testing.T) *MemoryEngine {
	t.Helper()
	e, err := NewEngine(WithConfig(EngineConfig{DataDir: t.TempDir()}))
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	return e
}

func TestIsolation_AgentMemoryDoesNotContaminate(t *testing.T) {
	ctx := context.Background()
	e := newTestEngine(t)

	// Sessão A grava.
	a := MemoryRecord{
		ID: "m-a-1", Type: MemoryTypeDecision, Layer: LayerSession,
		Agent: "cosca-backend", Owner: "user-a", Content: "A: PostgreSQL é obrigatório",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if _, err := e.Store(ctx, a); err != nil {
		t.Fatalf("Store A: %v", err)
	}

	// Sessão B grava uma informação DIFERENTE (potencialmente conflitante).
	b := MemoryRecord{
		ID: "m-b-1", Type: MemoryTypeDecision, Layer: LayerSession,
		Agent: "cosca-testing", Owner: "user-b", Content: "B: SQLite é obrigatório",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if _, err := e.Store(ctx, b); err != nil {
		t.Fatalf("Store B: %v", err)
	}

	// Sessão A consulta com AgentFilter=A: NÃO pode ver B.
	onlyA, err := e.Search(ctx, "obrigatorio", SearchOptions{AgentFilter: "cosca-backend", Limit: 10})
	if err != nil {
		t.Fatalf("Search A: %v", err)
	}
	for _, m := range onlyA {
		if m.Agent == "cosca-testing" {
			t.Fatalf("contamination: session A saw session B memory: %+v", m)
		}
	}
	if len(onlyA) == 0 {
		t.Fatal("session A should see its own memory")
	}

	// Sessão B consulta com AgentFilter=B: NÃO pode ver A.
	onlyB, err := e.Search(ctx, "obrigatorio", SearchOptions{AgentFilter: "cosca-testing", Limit: 10})
	if err != nil {
		t.Fatalf("Search B: %v", err)
	}
	for _, m := range onlyB {
		if m.Agent == "cosca-backend" {
			t.Fatalf("contamination: session B saw session A memory: %+v", m)
		}
	}

	// Sem filtro (admin/sistema): vê tudo.
	all, err := e.Search(ctx, "obrigatorio", SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Search all: %v", err)
	}
	if len(all) < 2 {
		t.Fatalf("unfiltered search should see both, got %d", len(all))
	}
}

func TestIsolation_OwnerScoping(t *testing.T) {
	ctx := context.Background()
	e := newTestEngine(t)

	rec := MemoryRecord{
		ID: "m-owner", Type: MemoryTypeDecision, Layer: LayerSession,
		Owner: "user-1", Content: "segredo do user-1",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if _, err := e.Store(ctx, rec); err != nil {
		t.Fatal(err)
	}

	// User-2 não pode ver a memória de user-1 (A6).
	res, err := e.Search(ctx, "segredo", SearchOptions{OwnerFilter: "user-2", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range res {
		if m.Owner == "user-1" {
			t.Fatalf("IDOR: user-2 saw user-1 memory: %+v", m)
		}
	}
}
