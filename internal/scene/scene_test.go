package scene

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/nodegraph"
	"github.com/CoscaAI/cosca/internal/procgen"
)

// =============================================================================
// Helpers
// =============================================================================

func approxFloat(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func approxVec3(a, b Vec3) bool {
	return approxFloat(a.X, b.X) && approxFloat(a.Y, b.Y) && approxFloat(a.Z, b.Z)
}

// =============================================================================
// TestSceneValidate
// =============================================================================

func TestSceneValidate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		s := NewScene("demo")
		terrain := NewTerrain("terrain", 256, 256, 1, "fbm", 42)
		veg := NewGroup("vegetation")
		if err := veg.AddChild(NewProceduralEntity("a", "perlin", 1, nil)); err != nil {
			t.Fatal(err)
		}
		if err := s.AddRoot(terrain); err != nil {
			t.Fatal(err)
		}
		if err := s.AddRoot(veg); err != nil {
			t.Fatal(err)
		}
		if err := s.Validate(); err != nil {
			t.Fatalf("expected valid scene, got %v", err)
		}
	})

	t.Run("duplicate id", func(t *testing.T) {
		s := NewScene("dup")
		a := &Entity{ID: "a", Type: Group, Children: []*Entity{
			{ID: "b", Type: Mesh},
			{ID: "b", Type: Mesh},
		}}
		s.Root = a
		err := s.Validate()
		if err == nil || !strings.Contains(err.Error(), "duplicate entity id") {
			t.Fatalf("expected duplicate id error, got %v", err)
		}
	})

	t.Run("cycle", func(t *testing.T) {
		s := NewScene("cycle")
		a := &Entity{ID: "a", Type: Group}
		b := &Entity{ID: "b", Type: Group}
		a.Children = append(a.Children, b)
		b.Children = append(b.Children, a)
		s.Root = a
		err := s.Validate()
		if err == nil || !strings.Contains(err.Error(), "cyclic") {
			t.Fatalf("expected cycle error, got %v", err)
		}
	})

	t.Run("add root duplicate id", func(t *testing.T) {
		s := NewScene("dup-root")
		_ = s.AddRoot(NewTerrain("t", 64, 64, 1, "fbm", 1))
		if err := s.AddRoot(NewTerrain("t", 64, 64, 1, "fbm", 2)); err == nil {
			t.Fatal("expected duplicate id error from AddRoot")
		}
	})
}

// =============================================================================
// TestTransform
// =============================================================================

func TestTransform(t *testing.T) {
	t.Run("identity", func(t *testing.T) {
		id := Identity()
		if !id.IsIdentity() {
			t.Fatal("Identity() must be identity")
		}
		m := id.LocalMatrix()
		want := [16]float64{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
		if m != want {
			t.Fatalf("identity matrix = %v, want %v", m, want)
		}
		if got := id.Apply(Vec3{1, 2, 3}); got != (Vec3{1, 2, 3}) {
			t.Fatalf("identity apply = %v", got)
		}
		if got := (Transform{Position: Vec3{1, 0, 0}}).IsIdentity(); got {
			t.Fatal("translated transform must not be identity")
		}
		if got := (Transform{Scale: Vec3{2, 1, 1}}).IsIdentity(); got {
			t.Fatal("scaled transform must not be identity")
		}
	})

	t.Run("local matrix translation", func(t *testing.T) {
		tr := Identity()
		tr.Position = Vec3{5, 6, 7}
		m := tr.LocalMatrix()
		if !approxFloat(m[12], 5) || !approxFloat(m[13], 6) || !approxFloat(m[14], 7) {
			t.Fatalf("translation not in matrix: %v", m)
		}
		if !approxFloat(m[0], 1) || !approxFloat(m[5], 1) || !approxFloat(m[10], 1) || !approxFloat(m[15], 1) {
			t.Fatalf("identity block wrong: %v", m)
		}
	})

	t.Run("world matrix composes parent+child", func(t *testing.T) {
		parent := Identity()
		parent.Position = Vec3{1, 2, 3}
		child := Identity()
		child.Position = Vec3{10, 20, 30}
		world := child.WorldMatrix(parent)
		origin := transformPoint(world, Vec3{0, 0, 0})
		if !approxVec3(origin, Vec3{11, 22, 33}) {
			t.Fatalf("world origin = %v, want {11 22 33}", origin)
		}
	})

	t.Run("local matrix TRS order", func(t *testing.T) {
		// T(1,2,3) * Rz(90°) * S(2,2,2): ponto {1,0,0} → escala {2,0,0} →
		// rotação Z 90° → {0,2,0} → translação → {1,4,3}.
		tr := Identity()
		tr.Position = Vec3{1, 2, 3}
		tr.Rotation = Vec3{0, 0, 90}
		tr.Scale = Vec3{2, 2, 2}
		got := tr.Apply(Vec3{1, 0, 0})
		if !approxVec3(got, Vec3{1, 4, 3}) {
			t.Fatalf("apply = %v, want {1 4 3}", got)
		}
	})
}

// =============================================================================
// TestVec3
// =============================================================================

func TestVec3(t *testing.T) {
	if got := Vec3Add(Vec3{1, 2, 3}, Vec3{4, 5, 6}); got != (Vec3{5, 7, 9}) {
		t.Fatalf("add = %v", got)
	}
	if got := Vec3Sub(Vec3{5, 7, 9}, Vec3{1, 2, 3}); got != (Vec3{4, 5, 6}) {
		t.Fatalf("sub = %v", got)
	}
	if got := Vec3Scale(Vec3{1, 2, 3}, 2); got != (Vec3{2, 4, 6}) {
		t.Fatalf("scale = %v", got)
	}
	if got := Vec3Dot(Vec3{1, 2, 3}, Vec3{4, 5, 6}); got != 32 {
		t.Fatalf("dot = %v, want 32", got)
	}
	if got := Vec3Cross(Vec3{1, 0, 0}, Vec3{0, 1, 0}); !approxVec3(got, Vec3{0, 0, 1}) {
		t.Fatalf("cross = %v", got)
	}
	if got := Vec3Normalize(Vec3{3, 0, 0}); !approxVec3(got, Vec3{1, 0, 0}) {
		t.Fatalf("normalize = %v", got)
	}
	if got := Vec3Normalize(Vec3{0, 0, 0}); got != (Vec3{0, 0, 0}) {
		t.Fatalf("normalize zero = %v, want {0 0 0}", got)
	}
	if l := Vec3Length(Vec3Normalize(Vec3{1, 2, 2})); !approxFloat(l, 1) {
		t.Fatalf("unit length = %v", l)
	}
}

// =============================================================================
// TestSerialization — roundtrip JSON (contrato igual ao do node graph).
// NOTA: JSON tem UM tipo numérico (number → float64), então params numéricos
// da cena de teste usam float64 para o deep equal passar pós-roundtrip.
// =============================================================================

func TestSerialization(t *testing.T) {
	s := NewScene("serialization-demo")
	terrain := &Entity{
		ID: "terrain", Name: "Terrain", Type: Terrain, NodeRef: "fbm",
		Params: map[string]any{
			"seed": float64(42), "octaves": float64(5),
			"width": float64(256), "height": float64(256),
		},
		Transform: Transform{Position: Vec3{X: 0, Y: 0, Z: -5}},
	}
	veg := NewGroup("vegetation")
	perlin := &Entity{
		ID: "vegetation_perlin", Type: Procedural, NodeRef: "perlin",
		Params:    map[string]any{"seed": float64(7), "width": float64(128)},
		Transform: Transform{Position: Vec3{X: 3, Y: 0, Z: 1}, Scale: Vec3{X: 1, Y: 1, Z: 1}},
	}
	if err := veg.AddChild(perlin); err != nil {
		t.Fatal(err)
	}
	if err := s.AddRoot(terrain); err != nil {
		t.Fatal(err)
	}
	if err := s.AddRoot(veg); err != nil {
		t.Fatal(err)
	}

	data, err := s.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(s, got) {
		t.Fatalf("roundtrip changed scene:\nwant %+v\ngot  %+v", s, got)
	}

	indent, err := s.MarshalIndent()
	if err != nil {
		t.Fatalf("marshal indent: %v", err)
	}
	got2, err := Unmarshal(indent)
	if err != nil {
		t.Fatalf("unmarshal indent: %v", err)
	}
	if !reflect.DeepEqual(s, got2) {
		t.Fatalf("indent roundtrip changed scene:\nwant %+v\ngot  %+v", s, got2)
	}
}

// =============================================================================
// TestBuilder
// =============================================================================

func TestBuilder(t *testing.T) {
	cam := NewCamera("cam", Vec3{0, 5, 0}, Vec3{0, 0, 0})
	if cam.Type != Camera || cam.Transform.Position != (Vec3{0, 5, 0}) {
		t.Fatalf("camera wrong: %+v", cam)
	}
	if la, ok := cam.Params["look_at"].(Vec3); !ok || la != (Vec3{0, 0, 0}) {
		t.Fatalf("camera look_at wrong: %+v", cam.Params["look_at"])
	}

	light := NewLight("sun", "directional", 1.5, [3]float64{1, 1, 1})
	if light.Type != Light || light.NodeRef != "" {
		t.Fatalf("light wrong: %+v", light)
	}
	if light.Params["type"] != "directional" || light.Params["intensity"] != 1.5 {
		t.Fatalf("light params wrong: %+v", light.Params)
	}

	ter := NewTerrain("t", 256, 128, 1.5, "fbm", 42)
	if ter.Type != Terrain || ter.NodeRef != "fbm" {
		t.Fatalf("terrain wrong: %+v", ter)
	}
	if ter.Params["seed"] != int64(42) || ter.Params["width"] != 256 || ter.Params["height"] != 128 {
		t.Fatalf("terrain params wrong: %+v", ter.Params)
	}
	if ter.Params["octaves"] != int64(5) {
		t.Fatalf("terrain octaves wrong: %+v", ter.Params)
	}

	proc := NewProceduralEntity("p", "perlin", 7, map[string]any{"width": 64})
	if proc.Type != Procedural || proc.NodeRef != "perlin" || proc.Params["seed"] != int64(7) {
		t.Fatalf("procedural wrong: %+v", proc)
	}

	g := NewGroup("g")
	if g.Type != Group || len(g.Children) != 0 {
		t.Fatalf("group wrong: %+v", g)
	}
}

// =============================================================================
// TestCompile — a ponte ENTIDADES → NODE GRAPH
// =============================================================================

func TestCompile(t *testing.T) {
	reg := procgen.Registry()
	exec := nodegraph.ExecutorFunc(func(ctx context.Context, node *nodegraph.Node, input map[string]any) (any, error) {
		e, ok := reg[node.Type]
		if !ok {
			return nil, fmt.Errorf("no executor for %s", node.Type)
		}
		return e.Run(ctx, node, input)
	})

	s := NewScene("compile-demo")
	terrain := NewTerrain("terrain", 32, 32, 1, "fbm", 42)
	veg := NewGroup("vegetation")
	if err := veg.AddChild(NewProceduralEntity("a", "perlin", 1, map[string]any{"width": 32, "height": 32})); err != nil {
		t.Fatal(err)
	}
	if err := veg.AddChild(NewProceduralEntity("b", "worley", 2, map[string]any{"width": 32, "height": 32})); err != nil {
		t.Fatal(err)
	}
	_ = s.AddRoot(terrain)
	_ = s.AddRoot(veg)
	_ = s.AddRoot(NewCamera("cam", Vec3{0, 5, 0}, Vec3{0, 0, 0}))

	g, err := Compile(s, nil, reg)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if g.Len() != 3 {
		t.Fatalf("compiled %d nodes, want 3 (terrain, perlin, worley; camera is spatial-only)", g.Len())
	}
	for _, wantType := range []nodegraph.NodeType{procgen.NodeFbm, procgen.NodePerlin, procgen.NodeWorley} {
		found := false
		for _, n := range g.Nodes {
			if n.Type == wantType {
				found = true
			}
		}
		if !found {
			t.Fatalf("compiled graph missing node type %q", wantType)
		}
	}

	stats, err := g.Run(context.Background(), exec, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, id := range []string{"terrain", "a", "b"} {
		img, ok := stats.Results[id].(*procgen.RGBA)
		if !ok {
			t.Fatalf("result of %q is %T, want *procgen.RGBA", id, stats.Results[id])
		}
		if img.W != 32 || img.H != 32 {
			t.Fatalf("result of %q is %dx%d, want 32x32", id, img.W, img.H)
		}
	}

	t.Run("unknown node ref", func(t *testing.T) {
		s2 := NewScene("bad")
		_ = s2.AddRoot(NewProceduralEntity("x", "no_such_node", 1, nil))
		_, err := Compile(s2, nil, reg)
		if err == nil || !strings.Contains(err.Error(), "unknown node type") {
			t.Fatalf("expected unknown node type error, got %v", err)
		}
	})
}
