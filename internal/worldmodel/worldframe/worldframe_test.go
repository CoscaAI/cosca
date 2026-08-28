package worldframe

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

func TestRegistry_ResolveSharedFrame(t *testing.T) {
	r := NewRegistry()
	r.AddAnchor(Anchor{ID: "mapa-1", Origin: worldmodel.Vec3{X: 100, Y: 0, Z: 200}})

	// Dois agentes no MESMO anchor, poses locais diferentes.
	_ = r.RegisterPose(Pose{Agent: "a", AnchorID: "mapa-1", Local: worldmodel.Vec3{X: 1, Y: 0, Z: 2}})
	_ = r.RegisterPose(Pose{Agent: "b", AnchorID: "mapa-1", Local: worldmodel.Vec3{X: 3, Y: 0, Z: 4}})

	pa, _ := r.Resolve("a")
	pb, _ := r.Resolve("b")
	// Resolvido para o frame absoluto (origem + local).
	if pa.X != 101 || pa.Z != 202 {
		t.Fatalf("resolve a: esperava (101,0,202), got %+v", pa)
	}
	if pb.X != 103 || pb.Z != 204 {
		t.Fatalf("resolve b: esperava (103,0,204), got %+v", pb)
	}
	// Mesmo anchor → concordam no espaço absoluto (saber sem se ver).
	if !r.AgreeAbsolute("a", "b") {
		t.Fatal("agentes no mesmo anchor devem concordar no espaço absoluto")
	}
}

func TestRegistry_FailClosed(t *testing.T) {
	r := NewRegistry()
	r.AddAnchor(Anchor{ID: "mapa-1"})
	// Ancorar a um anchor inexistente → erro (I2).
	if err := r.RegisterPose(Pose{Agent: "x", AnchorID: "mapa-inexistente"}); err == nil {
		t.Fatal("anchor inexistente deve dar erro (fail-closed)")
	}
	// Resolver agente não-anchorado → erro (I2).
	if _, err := r.Resolve("nao-existe"); err == nil {
		t.Fatal("agente não-anchorado deve dar erro (fail-closed)")
	}
	// Agentes em anchors DIFERENTES não concordam.
	r.AddAnchor(Anchor{ID: "mapa-2", Origin: worldmodel.Vec3{X: 999}})
	_ = r.RegisterPose(Pose{Agent: "c", AnchorID: "mapa-2"})
	if r.AgreeAbsolute("a", "c") {
		t.Fatal("agentes em anchors diferentes NÃO devem concordar no espaço")
	}
}

func TestRegistry_Deterministic(t *testing.T) {
	r := NewRegistry()
	r.AddAnchor(Anchor{ID: "m", Origin: worldmodel.Vec3{X: 5, Y: 5, Z: 5}})
	_ = r.RegisterPose(Pose{Agent: "a", AnchorID: "m", Local: worldmodel.Vec3{X: 1, Y: 2, Z: 3}})
	a, _ := r.Resolve("a")
	b, _ := r.Resolve("a")
	if a != b {
		t.Fatal("Resolve deve ser determinístico (I1)")
	}
}

// TestWorldEntity_KnownPosition checks the "saber vs ver" epistemic (I4).
func TestWorldEntity_KnownPosition(t *testing.T) {
	seen := worldmodel.WorldEntity{ID: "x", Position: worldmodel.Vec3{X: 1}, Visibility: worldmodel.VisibilityCurrent}
	if !seen.KnownPosition() {
		t.Fatal("entidade vista agora tem posição conhecida")
	}

	// Ocluída (conhecida, não vista): sabe onde está mesmo sem ver (see-through-walls).
	occluded := worldmodel.WorldEntity{
		ID:         "y",
		Position:   worldmodel.Vec3{X: 2},
		Persistent: true,
		Visibility: worldmodel.VisibilityStale,
		OccludedBy: []string{"wall-1"},
	}
	if !occluded.KnownPosition() {
		t.Fatal("entidade persistente/oclusa deve ter posição conhecida (saber ≠ ver)")
	}
	if occluded.KnownKind() != worldmodel.VisibilityStale {
		t.Fatal("Kind deveria ser stale")
	}
}
