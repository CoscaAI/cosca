package mcpserver

import (
	"context"
	"encoding/json"
	"testing"
)

// noopHandler é um ToolHandler vazio (nil-safe para os testes).
func noopHandler(ctx context.Context, _ json.RawMessage) (*CallResult, error) {
	return &CallResult{Content: []ContentItem{{Type: "text", Text: "ok"}}}, nil
}

// TestRegistry_DuplicatePanics valida o comportamento do professor (ponto 2):
// registrar uma tool em duplicidade é ERRO ARQUITETURAL → panic, NÃO
// "última vitória". Duplicidade faria Tools() (order) divergir de Call()
// (byName). Panic em dev captura o defeito cedo.
func TestRegistry_DuplicatePanics(t *testing.T) {
	r := newRegistry()
	r.register(ToolDef{Name: "cosca.test", Domain: "test", Handler: noopHandler})

	defer func() {
		if recover() == nil {
			t.Fatal("registro duplicado deveria dar panic (duplicidade = erro arquitetural)")
		}
	}()
	r.register(ToolDef{Name: "cosca.test", Domain: "test", Handler: noopHandler})
}

// TestRegistry_DefaultPermission valida que sem Permission explícita o default
// é public (read-only) e que o registro é coerente entre list() e get().
func TestRegistry_DefaultPermission(t *testing.T) {
	r := newRegistry()
	r.register(ToolDef{Name: "cosca.x", Domain: "cognition", Risk: RiskRead, Handler: noopHandler})

	def := r.get("cosca.x")
	if def == nil {
		t.Fatal("tool registrada nao encontrada")
	}
	if def.Permission != PermissionPublic {
		t.Fatalf("sem Permission explicita, default deveria ser public, got %q", def.Permission)
	}
	// list() e get() devem apontar para o NÃO MESMO registro (sem divergência).
	listed := r.list()
	if len(listed) != 1 || listed[0].Name != "cosca.x" {
		t.Fatalf("list() divergente do get(): %+v", listed)
	}
}

// TestRegistry_OwnerPermission valida que ToolDef escrito pode declarar owner.
func TestRegistry_OwnerPermission(t *testing.T) {
	r := newRegistry()
	r.register(ToolDef{Name: "cosca.w", Domain: "memory", Risk: RiskWrite, Permission: PermissionOwner, Handler: noopHandler})
	if got := r.get("cosca.w").Permission; got != PermissionOwner {
		t.Fatalf("esperava owner, got %q", got)
	}
}
