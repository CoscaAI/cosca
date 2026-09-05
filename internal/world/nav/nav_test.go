package nav

import (
	"testing"
)

// buildTestGraph monta um grafo de navegação simples (A→B→C e A→D→C).
func buildTestGraph() *Graph {
	g := NewGraph()
	g.AddNode(Node{ID: "entrada", X: 0, Y: 0})
	g.AddNode(Node{ID: "corredorA", X: 5, Y: 0})
	g.AddNode(Node{ID: "corredorD", X: 0, Y: 5})
	g.AddNode(Node{ID: "sala", X: 10, Y: 5})
	g.AddEdge(Edge{From: "entrada", To: "corredorA", Cost: 5})
	g.AddEdge(Edge{From: "corredorA", To: "sala", Cost: 5})
	g.AddEdge(Edge{From: "entrada", To: "corredorD", Cost: 5})
	g.AddEdge(Edge{From: "corredorD", To: "sala", Cost: 5})
	return g
}

func TestPlan_ValidDestination_FindsPath(t *testing.T) {
	g := buildTestGraph()
	route, err := g.Plan("entrada", "sala", Node{})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(route.Nodes) == 0 {
		t.Fatal("esperava waypoints")
	}
	if route.Nodes[0].ID != "entrada" || route.Nodes[len(route.Nodes)-1].ID != "sala" {
		t.Fatalf("rota deve começar em entrada e terminar em sala: %+v", route.Nodes)
	}
	// Caminho mais curto (A): entrada→corredorA→sala (cost 10) OU entrada→corredorD→sala (cost 10) — ambos 10.
	if route.Cost != 10 {
		t.Fatalf("custo deveria ser 10 (dois caminhos igualmente curtos), got %v", route.Cost)
	}
}

func TestPlan_InvalidDestination_FailClosed(t *testing.T) {
	g := buildTestGraph()
	// Destino que NÃO existe no mundo → fail-closed (I2), nunca plano vazio.
	if _, err := g.Plan("entrada", "lugar-inexistente", Node{}); err == nil {
		t.Fatal("destino inexistente deve dar erro (fail-closed I2)")
	}
	if err := g.ValidateDestination("lugar-inexistente"); err == nil {
		t.Fatal("ValidateDestination deve rejeitar destino inválido")
	}
}

func TestPlan_Unreachable_FailClosed(t *testing.T) {
	g := buildTestGraph()
	// nó órfão: não ligado a nada.
	g.AddNode(Node{ID: "isolado", X: 100, Y: 100})
	if _, err := g.Plan("entrada", "isolado", Node{}); err == nil {
		t.Fatal("destino inalcançável deve dar erro (fail-closed I2)")
	}
}

func TestPlan_Deterministic(t *testing.T) {
	g := buildTestGraph()
	a, _ := g.Plan("entrada", "sala", Node{})
	b, _ := g.Plan("entrada", "sala", Node{})
	// Mesmo grafo+start+goal → mesma rota (I1).
	if len(a.Nodes) != len(b.Nodes) || a.Cost != b.Cost {
		t.Fatal("plano deve ser determinístico")
	}
}

func TestValidateDestination_StartMissing(t *testing.T) {
	g := buildTestGraph()
	if _, err := g.Plan("nao-existe", "sala", Node{}); err == nil {
		t.Fatal("start inexistente deve dar erro")
	}
}
