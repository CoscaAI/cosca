package edit

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/world"
)

func TestResolve_Front(t *testing.T) {
	// Referência em (0,0,0), forward = norte (+Z). "colocar X à frente" → 10m ao norte.
	ref := world.Vec3{X: 0, Y: 0, Z: 0}
	fwd := world.Vec3{X: 0, Y: 0, Z: 1}
	p, err := Resolve(ref, fwd, PlacementIntent{Asset: "tree", Reference: "casa", Relation: Front, Offset: 10})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if p.Position.Z != 10 {
		t.Fatalf("front: posição deveria estar +10 em Z, got %+v", p.Position)
	}
	if p.Position.X != 0 {
		t.Fatalf("front: X não deve mudar, got %+v", p.Position)
	}
}

func TestResolve_Back(t *testing.T) {
	ref := world.Vec3{X: 0, Y: 0, Z: 0}
	fwd := world.Vec3{X: 0, Y: 0, Z: 1}
	p, _ := Resolve(ref, fwd, PlacementIntent{Asset: "x", Reference: "r", Relation: Back, Offset: 5})
	if p.Position.Z != -5 {
		t.Fatalf("back: posição deveria estar -5 em Z, got %+v", p.Position)
	}
}

func TestResolve_LeftRight(t *testing.T) {
	ref := world.Vec3{X: 0, Y: 0, Z: 0}
	fwd := world.Vec3{X: 0, Y: 0, Z: 1}
	l, _ := Resolve(ref, fwd, PlacementIntent{Asset: "x", Reference: "r", Relation: Left, Offset: 3})
	r, _ := Resolve(ref, fwd, PlacementIntent{Asset: "x", Reference: "r", Relation: Right, Offset: 3})
	// frente = +Z → esquerda = -X, direita = +X (plano XZ).
	if l.Position.X >= 0 || r.Position.X <= 0 {
		t.Fatalf("esquerda/direita: X deveria ser opostos, left=%+v right=%+v", l.Position, r.Position)
	}
}

func TestResolve_FailClosed(t *testing.T) {
	ref := world.Vec3{}
	// offset <= 0 → erro (I2).
	if _, err := Resolve(ref, world.Vec3{}, PlacementIntent{Relation: Front, Offset: 0}); err == nil {
		t.Fatal("offset <= 0 deve dar erro (fail-closed)")
	}
	// relação inválida → erro (I2).
	if _, err := Resolve(ref, world.Vec3{}, PlacementIntent{Relation: "up", Offset: 5}); err == nil {
		t.Fatal("relação inválida deve dar erro (fail-closed)")
	}
}

func TestResolve_Deterministic(t *testing.T) {
	ref := world.Vec3{X: 2, Y: 3, Z: 1}
	fwd := world.Vec3{X: 1, Y: 0, Z: 0}
	intent := PlacementIntent{Asset: "a", Reference: "r", Relation: Front, Offset: 4}
	a, _ := Resolve(ref, fwd, intent)
	b, _ := Resolve(ref, fwd, intent)
	if a.Position != b.Position || a.Heading != b.Heading {
		t.Fatal("Resolve deve ser determinístico (I1)")
	}
}

func TestResolve_DefaultForward(t *testing.T) {
	// Sem forward → DefaultForward (norte +Z). Determinístico.
	ref := world.Vec3{X: 0, Y: 0, Z: 0}
	p, err := Resolve(ref, world.Vec3{}, PlacementIntent{Asset: "x", Reference: "r", Relation: Front, Offset: 2})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if p.Position.Z != 2 {
		t.Fatalf("default forward deveria ser +Z, got %+v", p.Position)
	}
}
