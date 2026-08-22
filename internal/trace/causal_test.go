// Tests for the causal graph (internal/trace/causal.go) — causa → consequência.
//
// Covers:
//   - BuildCausalGraph: events → nodes + consecutive "caused" edges; empty → empty
//   - DivergencePoints: same trace → none; differing action → flagged index
//   - ToGraph: imports nodes+edges into the knowledge graph; RelCaused valid

package trace

import (
	"strconv"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/graph"
)

// =============================================================================
// BuildCausalGraph
// =============================================================================

func TestBuildCausalGraph_Chain(t *testing.T) {
	id := NewID().String()
	events := []Event{
		{TraceID: id, Actor: "kernel", Action: "PLAN_CREATED", Result: "success", Timestamp: 100},
		{TraceID: id, Actor: "agent-x", Action: "AGENT_STARTED", Result: "running", Timestamp: 200},
		{TraceID: id, Actor: "agent-x", Action: "TEST_FAILED", Result: "failed", Timestamp: 300},
		{TraceID: id, Actor: "agent-x", Action: "RETRY", Result: "running", Timestamp: 400},
		{TraceID: id, Actor: "agent-x", Action: "TEST_PASSED", Result: "success", Timestamp: 500},
	}

	cg := BuildCausalGraph(events)
	if cg.TraceID != id {
		t.Errorf("TraceID = %q, esperava %q", cg.TraceID, id)
	}
	if len(cg.Nodes) != len(events) {
		t.Fatalf("Nodes len = %d, esperava %d", len(cg.Nodes), len(events))
	}
	if len(cg.Edges) != len(events)-1 {
		t.Fatalf("Edges len = %d, esperava %d", len(cg.Edges), len(events)-1)
	}

	// Cada evento vira um nó, na ordem.
	for i, e := range events {
		n := cg.Nodes[i]
		wantID := "trace:" + id + ":" + strconv.Itoa(i)
		if n.ID != wantID {
			t.Errorf("node[%d].ID = %q, esperava %q", i, n.ID, wantID)
		}
		if n.Action != e.Action || n.Actor != e.Actor || n.Result != e.Result {
			t.Errorf("node[%d] = %+v, esperava action=%q actor=%q result=%q", i, n, e.Action, e.Actor, e.Result)
		}
	}

	// Arestas consecutivas "caused": N → N+1.
	for i, e := range cg.Edges {
		if e.Type != CausalEdgeType {
			t.Errorf("edge[%d].Type = %q, esperava %q", i, e.Type, CausalEdgeType)
		}
		if e.From != cg.Nodes[i].ID || e.To != cg.Nodes[i+1].ID {
			t.Errorf("edge[%d] = %s → %s, esperava %s → %s", i, e.From, e.To, cg.Nodes[i].ID, cg.Nodes[i+1].ID)
		}
	}
}

func TestBuildCausalGraph_Empty(t *testing.T) {
	cg := BuildCausalGraph(nil)
	if len(cg.Nodes) != 0 || len(cg.Edges) != 0 {
		t.Errorf("grafo causal vazio deveria ter 0 nós e 0 arestas, got %+v", cg)
	}
	if cg.TraceID != "" {
		t.Errorf("TraceID de grafo vazio deveria ser \"\", got %q", cg.TraceID)
	}
}

func TestBuildCausalGraph_SingleEvent(t *testing.T) {
	id := NewID().String()
	cg := BuildCausalGraph([]Event{{TraceID: id, Actor: "kernel", Action: "PLAN_CREATED"}})
	if len(cg.Nodes) != 1 {
		t.Fatalf("Nodes len = %d, esperava 1", len(cg.Nodes))
	}
	if len(cg.Edges) != 0 {
		t.Errorf("Edges len = %d, esperava 0 (sem par para causar)", len(cg.Edges))
	}
}

// =============================================================================
// DivergencePoints
// =============================================================================

func TestDivergencePoints_None_SameTrace(t *testing.T) {
	id := NewID().String()
	events := []Event{
		{TraceID: id, Action: "PLAN_CREATED"},
		{TraceID: id, Action: "AGENT_STARTED"},
		{TraceID: id, Action: "TEST_PASSED"},
	}
	cg := BuildCausalGraph(events)
	if div := cg.DivergencePoints(events); len(div) != 0 {
		t.Errorf("mesma sequência não deveria divergir, got %v", div)
	}
}

func TestDivergencePoints_NoPrior(t *testing.T) {
	cg := BuildCausalGraph([]Event{{TraceID: NewID().String(), Action: "PLAN_CREATED"}})
	if div := cg.DivergencePoints(nil); len(div) != 0 {
		t.Errorf("sem histórico não deveria divergir, got %v", div)
	}
}

func TestDivergencePoints_DifferingActionFlagged(t *testing.T) {
	id := NewID().String()
	prior := []Event{
		{TraceID: id, Action: "PLAN_CREATED"},
		{TraceID: id, Action: "TASK_STARTED"},
		{TraceID: id, Action: "TEST_PASSED"},
	}
	current := []Event{
		{TraceID: id, Action: "PLAN_CREATED"},
		{TraceID: id, Action: "TASK_STARTED"},
		{TraceID: id, Action: "TEST_FAILED"},
	}
	cg := BuildCausalGraph(current)
	div := cg.DivergencePoints(prior)
	if len(div) != 1 || div[0] != 2 {
		t.Errorf("divergência na posição 2 (índice 0-based) esperada, got %v", div)
	}
}

func TestDivergencePoints_ExtensionNotFlagged(t *testing.T) {
	id := NewID().String()
	prior := []Event{
		{TraceID: id, Action: "PLAN_CREATED"},
		{TraceID: id, Action: "TASK_STARTED"},
	}
	current := []Event{
		{TraceID: id, Action: "PLAN_CREATED"},
		{TraceID: id, Action: "TASK_STARTED"},
		{TraceID: id, Action: "TEST_PASSED"},
		{TraceID: id, Action: "APPROVED"},
	}
	cg := BuildCausalGraph(current)
	if div := cg.DivergencePoints(prior); len(div) != 0 {
		t.Errorf("extensão ao final é normal (prefix-extension), got %v", div)
	}
}

// =============================================================================
// ToGraph — importação no grafo de conhecimento
// =============================================================================

func TestToGraph_ImportsNodesAndEdges(t *testing.T) {
	id := NewID().String()
	events := []Event{
		{TraceID: id, Actor: "kernel", Action: "PLAN_CREATED", Result: "success"},
		{TraceID: id, Actor: "agent-x", Action: "TEST_FAILED", Result: "failed"},
		{TraceID: id, Actor: "agent-x", Action: "TEST_PASSED", Result: "success"},
	}
	cg := BuildCausalGraph(events)

	g := graph.New()
	if err := cg.ToGraph(g); err != nil {
		t.Fatalf("ToGraph: %v", err)
	}
	if g.GetNodeCount() != 3 {
		t.Errorf("node count = %d, esperava 3", g.GetNodeCount())
	}
	if g.GetEdgeCount() != 2 {
		t.Errorf("edge count = %d, esperava 2", g.GetEdgeCount())
	}

	// Nós importados com tipo causal_event.
	for _, n := range cg.Nodes {
		got, ok := g.GetNode(n.ID)
		if !ok {
			t.Fatalf("nó %s não importado", n.ID)
		}
		if got.Type != "causal_event" {
			t.Errorf("node %s Type = %q, esperava \"causal_event\"", n.ID, got.Type)
		}
		if got.Name != n.Action {
			t.Errorf("node %s Name = %q, esperava %q", n.ID, got.Name, n.Action)
		}
	}

	// Arestas com RelCaused.
	edges := g.GetAllEdges()
	for _, e := range edges {
		if e.Type != graph.RelCaused {
			t.Errorf("edge %s → %s Type = %q, esperava %q", e.Source, e.Target, e.Type, graph.RelCaused)
		}
	}
}

func TestToGraph_NilGraph(t *testing.T) {
	cg := BuildCausalGraph([]Event{{TraceID: NewID().String(), Action: "PLAN_CREATED"}})
	if err := cg.ToGraph(nil); err == nil {
		t.Error("ToGraph com grafo nil deveria falhar")
	}
}

// RelCaused é um tipo de relação válido no grafo de conhecimento (ADITIVO).
func TestRelCaused_ValidRelationship(t *testing.T) {
	found := false
	for _, rel := range graph.ValidRelationshipTypes() {
		if rel == graph.RelCaused {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("ValidRelationshipTypes deveria incluir %q", graph.RelCaused)
	}
}

func TestRelCaused_Serializes(t *testing.T) {
	id := NewID().String()
	cg := BuildCausalGraph([]Event{
		{TraceID: id, Action: "PLAN_CREATED"},
		{TraceID: id, Action: "TEST_FAILED"},
	})
	g := graph.New()
	if err := cg.ToGraph(g); err != nil {
		t.Fatalf("ToGraph: %v", err)
	}
	data, err := g.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	if !strings.Contains(string(data), graph.RelCaused) {
		t.Errorf("serialização deveria conter %q", graph.RelCaused)
	}
	loaded, err := graph.Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize: %v", err)
	}
	if loaded.GetEdgeCount() != 1 {
		t.Errorf("round-trip edge count = %d, esperava 1", loaded.GetEdgeCount())
	}
}
