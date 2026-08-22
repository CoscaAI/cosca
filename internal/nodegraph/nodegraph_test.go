package nodegraph

import (
	"testing"
)

// n é um helper de construção compacta: n(id, type, inputs...).
// Param pode ser usado: n(id, type, "key=value") ainda é input — use
// np(id, type, params, inputs...) para params explícitos.
func n(id string, typ string, inputs ...string) *Node {
	return &Node{ID: id, Type: NodeType(typ), Inputs: inputs}
}

// np cria um nó com params explícitos e inputs.
func np(id string, typ string, params map[string]any, inputs ...string) *Node {
	return &Node{ID: id, Type: NodeType(typ), Params: params, Inputs: inputs}
}

func TestBuildAndValidate(t *testing.T) {
	g, err := Build("pipeline", []*Node{
		np("load", "load_image", map[string]any{"path": "a.png"}),
		n("seg", "segment", "load"),
		n("bg", "remove_bg", "load", "seg"),
		n("out", "export", "bg"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if g.Len() != 4 {
		t.Fatalf("Len = %d, want 4", g.Len())
	}
}

func TestDuplicateID(t *testing.T) {
	g := New("x")
	_ = g.AddNode(n("a", "load"))
	if err := g.AddNode(n("a", "load")); err == nil {
		t.Fatal("expected duplicate id error")
	}
}

func TestMissingInput(t *testing.T) {
	g := New("x")
	_ = g.AddNode(n("a", "load", "ghost"))
	if err := g.Validate(); err == nil {
		t.Fatal("expected missing input error")
	}
}

func TestCycleDetected(t *testing.T) {
	g := New("x")
	_ = g.AddNode(n("a", "n1", "b"))
	_ = g.AddNode(n("b", "n2", "a"))
	if err := g.Validate(); err == nil {
		t.Fatal("expected cycle error")
	}
	if _, err := g.TopoOrder(); err == nil {
		t.Fatal("expected TopoOrder cycle error")
	}
}

func TestTopoOrderRespectsDependencies(t *testing.T) {
	g, err := Build("p", []*Node{
		n("out", "export", "bg"),
		n("load", "load"),
		n("bg", "remove_bg", "load", "seg"),
		n("seg", "segment", "load"),
	})
	if err != nil {
		t.Fatal(err)
	}
	order, err := g.TopoOrder()
	if err != nil {
		t.Fatal(err)
	}
	pos := make(map[string]int, len(order))
	for i, node := range order {
		pos[node.ID] = i
	}
	// load antes de seg/bg; seg antes de bg; bg antes de out.
	if pos["load"] > pos["seg"] || pos["load"] > pos["bg"] {
		t.Fatal("load must precede seg/bg")
	}
	if pos["seg"] > pos["bg"] {
		t.Fatal("seg must precede bg")
	}
	if pos["bg"] > pos["out"] {
		t.Fatal("bg must precede out")
	}
}

func TestSignatureStable(t *testing.T) {
	g1, err := Build("p", []*Node{
		np("load", "load_image", map[string]any{"path": "a.png"}),
		n("seg", "segment", "load"),
	})
	if err != nil {
		t.Fatal(err)
	}
	g2, err := Build("p", []*Node{
		np("load", "load_image", map[string]any{"path": "a.png"}),
		n("seg", "segment", "load"),
	})
	if err != nil {
		t.Fatal(err)
	}
	s1, _ := g1.Signature("seg")
	s2, _ := g2.Signature("seg")
	if s1 != s2 {
		t.Fatalf("signatures differ for identical graphs: %s vs %s", s1, s2)
	}
}

func TestSignatureChangesWithParams(t *testing.T) {
	g1, _ := Build("p", []*Node{np("load", "load_image", map[string]any{"path": "a.png"})})
	g2, _ := Build("p", []*Node{np("load", "load_image", map[string]any{"path": "b.png"})})
	s1, _ := g1.Signature("load")
	s2, _ := g2.Signature("load")
	if s1 == s2 {
		t.Fatal("signatures should differ when params change")
	}
}

func TestSignatureRecursiveThroughAncestors(t *testing.T) {
	// A assinatura do nó final muda quando um ANCESTRAL muda (cache por
	// assinatura recursiva — nunca resultado obsoleto, P1/§23).
	g1, _ := Build("p", []*Node{
		np("load", "load_image", map[string]any{"path": "a.png"}),
		n("seg", "segment", "load"),
		n("out", "export", "seg"),
	})
	g2, _ := Build("p", []*Node{
		np("load", "load_image", map[string]any{"path": "b.png"}),
		n("seg", "segment", "load"),
		n("out", "export", "seg"),
	})
	s1, _ := g1.Signature("out")
	s2, _ := g2.Signature("out")
	if s1 == s2 {
		t.Fatal("descendant signature must change when ancestor param changes")
	}
}

func TestSignatureUnchangedForIndependentNode(t *testing.T) {
	// Um nó que não depende do ancestral NÃO é afetado.
	g1, _ := Build("p", []*Node{
		np("load", "load_image", map[string]any{"path": "a.png"}),
		np("other", "load_image", map[string]any{"path": "x.png"}),
	})
	g2, _ := Build("p", []*Node{
		np("load", "load_image", map[string]any{"path": "b.png"}),
		np("other", "load_image", map[string]any{"path": "x.png"}),
	})
	s1, _ := g1.Signature("other")
	s2, _ := g2.Signature("other")
	if s1 != s2 {
		t.Fatal("independent node signature should be unchanged")
	}
}

func TestRoundTripJSON(t *testing.T) {
	g, err := Build("workflow", []*Node{
		np("v", "load_video", map[string]any{"path": "intro.mp4"}),
		n("a", "extract_audio", "v"),
		np("w", "whisper", map[string]any{"model": "large-v3"}, "a"),
		n("s", "subtitle", "w"),
		n("r", "render", "v", "s"),
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := g.MarshalIndent()
	if err != nil {
		t.Fatal(err)
	}
	g2, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if g2.Name != "workflow" || g2.Len() != 5 {
		t.Fatalf("round trip mismatch: name=%q len=%d", g2.Name, g2.Len())
	}
	// Assinaturas preservadas após round trip.
	s1, _ := g.Signature("r")
	s2, _ := g2.Signature("r")
	if s1 != s2 {
		t.Fatal("signature must survive JSON round trip")
	}
}

func TestUnmarshalInvalid(t *testing.T) {
	if _, err := Unmarshal([]byte(`{"name":"x","nodes":[{"id":"a","type":"t","inputs":["ghost"]}]}`)); err == nil {
		t.Fatal("expected error for missing input on unmarshal")
	}
	if _, err := Unmarshal([]byte(`not json`)); err == nil {
		t.Fatal("expected parse error")
	}
}
