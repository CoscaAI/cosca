package worldloop

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/worldspec"
)

func TestRun_ConnectsIsolated(t *testing.T) {
	spec := worldspec.New(42, "cidade", "open_world")
	spec.Provenance = worldspec.Provenance{Source: "llm", Class: "generated", Seed: 42}
	// nós: porto-centro conectados; norte isolado.
	spec.Navigation.Nodes = []worldspec.NavNode{
		{ID: "n_porto"}, {ID: "n_centro"}, {ID: "n_norte"},
	}
	spec.Navigation.Edges = []worldspec.NavEdge{{From: "n_porto", To: "n_centro", Type: "road"}}

	rep, err := Run(*spec, AccessibilitySimulator{}, ConnectIsolated{}, DefaultGate(), 3)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// Deve ter conectado o nó isolado (norte) ao hub e passado no gate.
	if !rep.Verdict.Promoted {
		t.Fatalf("loop deveria ter promovido após conectar: %s", rep.Verdict.Summary)
	}
	if rep.Iterations < 2 {
		t.Fatalf("esperava >=2 iterações (1 conecta, 2 valida), got %d", rep.Iterations)
	}
	// O norte deve ter ganhado uma aresta.
	degrees := map[string]int{}
	for _, e := range rep.Final.Navigation.Edges {
		degrees[e.From]++
		degrees[e.To]++
	}
	if degrees["n_norte"] == 0 {
		t.Fatalf("norte deveria ter sido conectado; edges=%+v", rep.Final.Navigation.Edges)
	}
}

func TestRun_AlreadyConnected(t *testing.T) {
	spec := worldspec.New(1, "ok", "open_world")
	spec.Provenance = worldspec.Provenance{Source: "llm", Class: "generated", Seed: 1}
	spec.Navigation.Nodes = []worldspec.NavNode{{ID: "n_a"}, {ID: "n_b"}}
	spec.Navigation.Edges = []worldspec.NavEdge{{From: "n_a", To: "n_b", Type: "road"}}

	rep, err := Run(*spec, AccessibilitySimulator{}, ConnectIsolated{}, DefaultGate(), 3)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !rep.Verdict.Promoted {
		t.Fatalf("mundo já conectado deveria promover: %s", rep.Verdict.Summary)
	}
	if rep.Iterations != 1 {
		t.Fatalf("esperava 1 iteração (já ok), got %d", rep.Iterations)
	}
}
