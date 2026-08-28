// Package nav — Local Action Planner (World Building, ADR-021; mineração SimWorld).
//
// O padrão que o SimWorld provou (e o Cosca já desenhou como I1): o LLM/agente
// PROPÕE um destino (intenção estruturada); o SISTEMA decide — valida o destino
// contra o mundo e executa o caminho. Aqui é a camada "executor determinístico":
//
//	Plan(proposta de destino) → gate (valida) → A* → waypoints
//
// Regras (invariantes):
//   - I1: determinação, zero LLM. O destino chega como dado estruturado (saída do
//     proposal gate); o planner só decide rota.
//   - I2 (fail-closed): destino inválido/inalcançável → ERRO (nunca "plano vazio
//     silencioso").
//   - Determinístico: mesmo grafo+start+goal → mesma rota.
package nav

import (
	"container/heap"
	"fmt"
	"math"
)

// Node é um waypoint do grafo de navegação (espaço-mundo).
type Node struct {
	ID string  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
}

// Edge é uma aresta ponderada entre nós (direcionada).
type Edge struct {
	From string
	To   string
	Cost float64
}

// Graph é o grafo de navegação (waypoints + arestas).
type Graph struct {
	nodes map[string]Node
	edges map[string][]Edge // from -> edges
}

// NewGraph cria um grafo de navegação vazio.
func NewGraph() *Graph {
	return &Graph{nodes: map[string]Node{}, edges: map[string][]Edge{}}
}

// AddNode adiciona um waypoint.
func (g *Graph) AddNode(n Node) {
	g.nodes[n.ID] = n
	if _, ok := g.edges[n.ID]; !ok {
		g.edges[n.ID] = []Edge{}
	}
}

// AddEdge adiciona aresta direcionada (with cost).
func (g *Graph) AddEdge(e Edge) {
	if _, ok := g.nodes[e.From]; !ok {
		g.AddNode(Node{ID: e.From})
	}
	if _, ok := g.nodes[e.To]; !ok {
		g.AddNode(Node{ID: e.To})
	}
	g.edges[e.From] = append(g.edges[e.From], e)
}

// Node devolve um nó por ID.
func (g *Graph) Node(id string) (Node, bool) { n, ok := g.nodes[id]; return n, ok }

// HasNode indica se um nó existe no grafo.
func (g *Graph) HasNode(id string) bool { _, ok := g.nodes[id]; return ok }

// Route é o plano resultante (waypoints + custo).
type Route struct {
	Nodes []Node `json:"nodes"`
	Cost  float64 `json:"cost"`
}

// Distance é a distância euclidiana 2D.
func Distance(a, b Node) float64 {
	dx, dy := a.X-b.X, a.Y-b.Y
	return math.Sqrt(dx*dx + dy*dy)
}

// ValidateDestination é o GATE I1/I2: confirma que o destino existe no mundo.
// Destino inválido → erro (fail-closed, nunca "plano vazio silencioso").
func (g *Graph) ValidateDestination(dest string) error {
	if !g.HasNode(dest) {
		return fmt.Errorf("destination %q not in world (gate I2: reject)", dest)
	}
	return nil
}

// Plan resolve a rota de `start` a `goal`. É determinístico (I1) e fail-closed
// (I2): destino inválido ou inalcançável → erro. O LLM propôs `goal`; o sistema
// decide o caminho.
func (g *Graph) Plan(start, goal string, agentPos Node) (*Route, error) {
	// Gate: o destino deve existir no mundo (autoridade do sistema, I1).
	if err := g.ValidateDestination(goal); err != nil {
		return nil, err
	}
	if !g.HasNode(start) {
		return nil, fmt.Errorf("start %q not in world (gate I2)", start)
	}
	path, cost, ok := g.aStar(start, goal)
	if !ok {
		return nil, fmt.Errorf("no path from %q to %q (fail-closed)", start, goal)
	}
	nodes := make([]Node, 0, len(path))
	for _, id := range path {
		nodes = append(nodes, g.nodes[id])
	}
	_ = agentPos
	return &Route{Nodes: nodes, Cost: cost}, nil
}

// aStar — busca A* no grafo (determinístico, heurística = distância euclidiana).
func (g *Graph) aStar(start, goal string) ([]string, float64, bool) {
	open := &pqueue{}
	heap.Init(open)
	gScore := map[string]float64{start: 0}
	cameFrom := map[string]string{}
	heap.Push(open, pqItem{id: start, f: 0})

	for open.Len() > 0 {
		cur := heap.Pop(open).(pqItem).id
		if cur == goal {
			return reconstruct(cameFrom, goal), gScore[goal], true
		}
		for _, e := range g.edges[cur] {
			tent := gScore[cur] + e.Cost
			if old, ok := gScore[e.To]; !ok || tent < old {
				gScore[e.To] = tent
				cameFrom[e.To] = cur
				h := Distance(g.nodes[e.To], g.nodes[goal])
				heap.Push(open, pqItem{id: e.To, f: tent + h})
			}
		}
	}
	return nil, 0, false
}

func reconstruct(cameFrom map[string]string, goal string) []string {
	path := []string{goal}
	for cur := goal; ; {
		prev, ok := cameFrom[cur]
		if !ok {
			break
		}
		path = append([]string{prev}, path...)
		cur = prev
	}
	return path
}

// pqItem para a fila de prioridade do A*.
type pqItem struct {
	id string
	f  float64
}

type pqueue []pqItem

func (q pqueue) Len() int            { return len(q) }
func (q pqueue) Less(i, j int) bool  { return q[i].f < q[j].f }
func (q pqueue) Swap(i, j int)       { q[i], q[j] = q[j], q[i] }
func (q *pqueue) Push(x interface{}) { *q = append(*q, x.(pqItem)) }
func (q *pqueue) Pop() interface{} {
	old := *q
	n := len(old)
	item := old[n-1]
	*q = old[:n-1]
	return item
}
