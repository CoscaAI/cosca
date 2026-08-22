// Causal graph — a causa → consequência a partir do Trace.
//
// Regra do Don (Control Center): "não mostrar apenas uma timeline. Mostrar
// causa → consequência: Knowledge K-81 → Agent-72 decision → daemon.go changed
// → test failed → Agent-72 correction → test passed → Reviewer approved. Isso
// seria extremamente útil para investigar bugs."
//
// Os eventos do trace JÁ SÃO a cadeia causal: action → result → next action.
// Um trace como [PLAN_CREATED → AGENT_STARTED → TEST_FAILED → RETRY →
// TEST_PASSED → APPROVED] é um caminho causa→consequência. Este pacote expõe a
// construção determinística (sem LLM) desse grafo a partir do ledger.
package trace

import (
	"fmt"
	"strings"

	"github.com/CoscaAI/cosca/internal/graph"
)

// CausalEdgeType é o tipo de aresta da cadeia causal: evento N causou o evento
// N+1 (espelha graph.RelCaused = "caused").
const CausalEdgeType = "caused"

// CausalNode é um passo da cadeia causa→consequência. ID único no formato
// "trace:<traceID>:<idx>".
type CausalNode struct {
	ID      string `json:"id"` // "trace:<traceID>:<idx>"
	Action  string `json:"action"`
	Actor   string `json:"actor"`
	Result  string `json:"result,omitempty"`
	Details string `json:"details,omitempty"`
}

// CausalEdge é uma aresta dirigida da cadeia causal.
type CausalEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"` // "caused" | "led_to" — mantém simples: "caused"
}

// CausalGraph é o grafo causal de um trace: cada evento vira um nó e eventos
// consecutivos recebem uma aresta "caused" (a cadeia causa→consequência).
type CausalGraph struct {
	TraceID string       `json:"trace_id"`
	Nodes   []CausalNode `json:"nodes"`
	Edges   []CausalEdge `json:"edges"`
}

// BuildCausalGraph constrói o grafo causal a partir dos eventos de um trace
// (já ordenados por timestamp — o que o Store.Get devolve). Cada evento vira
// um CausalNode (id "trace:<traceID>:<idx>"); eventos consecutivos recebem a
// aresta "caused" (evento N causou evento N+1). Determinístico, sem LLM.
// Eventos sem action continuam no grafo como nós (fazem parte da linha do
// tempo), mas não contam como passo na sequência de divergência.
func BuildCausalGraph(events []Event) CausalGraph {
	cg := CausalGraph{
		Nodes: make([]CausalNode, 0, len(events)),
		Edges: make([]CausalEdge, 0, len(events)),
	}
	if len(events) == 0 {
		return cg
	}
	cg.TraceID = events[0].TraceID
	for i, e := range events {
		id := causalNodeID(e.TraceID, i)
		cg.Nodes = append(cg.Nodes, CausalNode{
			ID:      id,
			Action:  e.Action,
			Actor:   e.Actor,
			Result:  e.Result,
			Details: e.Details,
		})
		if i > 0 {
			cg.Edges = append(cg.Edges, CausalEdge{
				From: causalNodeID(events[i-1].TraceID, i-1),
				To:   id,
				Type: CausalEdgeType,
			})
		}
	}
	return cg
}

// causalNodeID gera o ID canônico de um nó causal: "trace:<traceID>:<idx>".
func causalNodeID(traceID string, idx int) string {
	return fmt.Sprintf("trace:%s:%d", traceID, idx)
}

// DivergencePoints reutiliza a heurística de divergência (DetectDivergence,
// determinística, sem LLM): devolve os índices (0-based) da SEQUÊNCIA DE AÇÕES
// deste grafo causal onde a ação difere de um trace anterior. Extensão ao
// final (prefix-extension) é normal e NÃO é marcada. Sem trace anterior (nil
// ou vazio) ⇒ nenhuma divergência.
func (cg CausalGraph) DivergencePoints(prior []Event) []int {
	if len(prior) == 0 {
		return nil
	}
	current := Sequence{Actions: make([]string, 0, len(cg.Nodes))}
	for _, n := range cg.Nodes {
		if strings.TrimSpace(n.Action) != "" {
			current.Actions = append(current.Actions, n.Action)
		}
	}
	return DetectDivergence([]Sequence{SequenceFromEvents(prior)}, current)
}

// ToGraph importa a cadeia causal para o grafo de conhecimento: cada
// CausalNode vira um graph.Node (Type "causal_event", Name = ação) e cada
// aresta "caused" vira uma graph.Edge com Type graph.RelCaused. Não altera o
// comportamento do grafo — apenas adiciona nós e arestas.
func (cg CausalGraph) ToGraph(g *graph.Graph) error {
	if g == nil {
		return fmt.Errorf("causal: graph é nil")
	}
	for _, n := range cg.Nodes {
		node := &graph.Node{
			ID:   n.ID,
			Type: "causal_event",
			Name: n.Action,
			Metadata: map[string]interface{}{
				"actor":   n.Actor,
				"result":  n.Result,
				"details": n.Details,
			},
		}
		if err := g.AddNode(node); err != nil {
			return fmt.Errorf("causal: add node %s: %w", n.ID, err)
		}
	}
	for _, e := range cg.Edges {
		edge := &graph.Edge{Source: e.From, Target: e.To, Type: graph.RelCaused}
		if err := g.AddEdge(edge); err != nil {
			return fmt.Errorf("causal: add edge %s → %s: %w", e.From, e.To, err)
		}
	}
	return nil
}
