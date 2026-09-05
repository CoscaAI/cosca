package modlink

import (
	"strings"
	"testing"
)

// TestDefaultRoutes_NotEmpty garante que o registry de produção tem rotas — o
// roteador modular nunca nasce vazio.
func TestDefaultRoutes_NotEmpty(t *testing.T) {
	routes := DefaultRoutes()
	if len(routes) == 0 {
		t.Fatal("DefaultRoutes() must not be empty")
	}
}

// TestDefaultRoutes_NoEmptyFields verifica que nenhuma rota tem Module,
// Capability ou Trigger vazio — o contrato de integridade do registry.
func TestDefaultRoutes_NoEmptyFields(t *testing.T) {
	for i, r := range DefaultRoutes() {
		if strings.TrimSpace(r.Module) == "" {
			t.Fatalf("route[%d]: Module is empty", i)
		}
		if strings.TrimSpace(r.Capability) == "" {
			t.Fatalf("route[%d] (%s): Capability is empty", i, r.Module)
		}
		if strings.TrimSpace(r.Trigger) == "" {
			t.Fatalf("route[%d] (%s): Trigger is empty", i, r.Module)
		}
	}
}

// TestDefaultRoutes_ValidatePasses: o registry de produção é válido e cada
// Trigger é único (ValidateRoutes não deve falhar).
func TestDefaultRoutes_ValidatePasses(t *testing.T) {
	if err := ValidateRoutes(DefaultRoutes()); err != nil {
		t.Fatalf("ValidateRoutes(DefaultRoutes()) = %v, want nil", err)
	}
}

// TestDefaultRoutes_UniqueTriggers garante a unicidade canônica dos Trigger
// (a acentuação/caixa é normalizada — "memória" e "memoria" colidiriam).
func TestDefaultRoutes_UniqueTriggers(t *testing.T) {
	seen := map[string]string{}
	for _, r := range DefaultRoutes() {
		key := strings.Join(canonicalTokens(r.Trigger), " ")
		if prev, ok := seen[key]; ok {
			t.Fatalf("duplicate canonical trigger %q on modules %q and %q", r.Trigger, prev, r.Module)
		}
		seen[key] = r.Module
	}
}

// TestRoutesByModule_CoversAll: o índice por módulo cobre todas as rotas (1:1 —
// cada módulo ocorre uma única vez no registry).
func TestRoutesByModule_CoversAll(t *testing.T) {
	routes := DefaultRoutes()
	byModule := RoutesByModule(routes)
	if len(byModule) != len(routes) {
		t.Fatalf("RoutesByModule covers %d modules but there are %d routes (duplicate modules?)", len(byModule), len(routes))
	}
	seen := map[string]bool{}
	for _, r := range routes {
		entry, ok := byModule[r.Module]
		if !ok {
			t.Fatalf("RoutesByModule missing module %q", r.Module)
		}
		if entry.Module != r.Module || entry.Trigger != r.Trigger {
			t.Fatalf("byModule[%q] = %+v, want %+v", r.Module, entry, r)
		}
		if seen[r.Module] {
			t.Fatalf("module %q registered more than once", r.Module)
		}
		seen[r.Module] = true
	}
}

// TestValidateRoutes_RejectsEmptyFields prova que um registry com Module vazio
// é rejeitado — nunca silenciosamente aceito.
func TestValidateRoutes_RejectsEmptyFields(t *testing.T) {
	cases := []struct {
		name  string
		routes []Route
	}{
		{"empty module", []Route{{Trigger: "x", Capability: "m.c"}}},
		{"empty trigger", []Route{{Module: "m", Capability: "m.c"}}},
		{"empty capability", []Route{{Trigger: "x", Module: "m"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateRoutes(tc.routes); err == nil {
				t.Fatalf("ValidateRoutes(%+v) = nil, want error", tc.routes)
			}
		})
	}
}

// TestValidateRoutes_RejectsDuplicateTrigger prova que dois Trigger canônicos
// iguais tornam o registry ambíguo e são rejeitados.
func TestValidateRoutes_RejectsDuplicateTrigger(t *testing.T) {
	routes := []Route{
		{Trigger: "decisão arquitetural", Module: "adr", Capability: "adr.record"},
		{Trigger: "decisão arquitetural", Module: "architecture", Capability: "architecture.overview"},
	}
	if err := ValidateRoutes(routes); err == nil {
		t.Fatalf("ValidateRoutes(duplicate trigger) = nil, want error")
	}
}
