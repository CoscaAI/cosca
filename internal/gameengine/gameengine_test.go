package gameengine

import (
	"strings"
	"testing"
)

func testPlayer() *Entity {
	return &Entity{
		ID: "player", Name: "player",
		Components: []Component{
			{Type: ComponentTransform, Params: map[string]any{"x": 0, "y": 0}},
			{Type: ComponentPhysics, Params: map[string]any{"gravity": true}},
			{Type: ComponentRender, Params: map[string]any{"sprite": "hero.png"}},
			{Type: ComponentInput},
		},
	}
}

func testEnemy() *Entity {
	return &Entity{
		ID: "enemy", Name: "enemy_1",
		Components: []Component{
			{Type: ComponentTransform},
			{Type: ComponentAI},
			{Type: ComponentHealth, Params: map[string]any{"hp": 100}},
		},
	}
}

func TestSceneBuild(t *testing.T) {
	scene := &Scene{Name: "level-1", Width: 800, Height: 600}
	if err := scene.AddEntity(testPlayer()); err != nil {
		t.Fatal(err)
	}
	if err := scene.AddEntity(testEnemy()); err != nil {
		t.Fatal(err)
	}
	if scene.EntityCount() != 2 {
		t.Fatalf("entities = %d, want 2", scene.EntityCount())
	}
}

func TestDuplicateEntity(t *testing.T) {
	scene := &Scene{Name: "s"}
	_ = scene.AddEntity(testPlayer())
	if err := scene.AddEntity(testPlayer()); err == nil {
		t.Fatal("expected duplicate entity error")
	}
}

func TestInvalidComponent(t *testing.T) {
	scene := &Scene{Name: "s"}
	e := &Entity{ID: "x", Components: []Component{{Type: ComponentType("bogus")}}}
	if err := scene.AddEntity(e); err == nil {
		t.Fatal("expected invalid component error")
	}
}

func TestEntitiesWith(t *testing.T) {
	scene := &Scene{Name: "s"}
	_ = scene.AddEntity(testPlayer())
	_ = scene.AddEntity(testEnemy())

	ai := scene.EntitiesWith(ComponentAI)
	if len(ai) != 1 || ai[0].ID != "enemy" {
		t.Fatalf("EntitiesWith(ai) = %+v", ai)
	}
	input := scene.EntitiesWith(ComponentInput)
	if len(input) != 1 || input[0].ID != "player" {
		t.Fatalf("EntitiesWith(input) = %+v", input)
	}
	if got := scene.EntitiesWith(ComponentScore); len(got) != 0 {
		t.Fatalf("EntitiesWith(score) should be empty")
	}
}

func TestCountSystem(t *testing.T) {
	scene := &Scene{Name: "s"}
	_ = scene.AddEntity(testPlayer())
	_ = scene.AddEntity(testEnemy())
	sys := CountSystem{Comp: ComponentPhysics}
	report, err := sys.Process(scene)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report, "1 entidades com physics") {
		t.Fatalf("report = %q", report)
	}
}

func TestRoundTripJSON(t *testing.T) {
	scene := &Scene{Name: "platformer", Width: 640, Height: 480}
	_ = scene.AddEntity(testPlayer())
	_ = scene.AddEntity(testEnemy())

	data, err := scene.MarshalIndent()
	if err != nil {
		t.Fatal(err)
	}
	got, err := UnmarshalScene(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "platformer" || got.EntityCount() != 2 {
		t.Fatalf("round trip: %+v", got)
	}
	p, ok := got.EntityByID("player")
	if !ok || !p.HasComponent(ComponentInput) {
		t.Fatalf("player after round trip: %+v", p)
	}
}

func TestUnmarshalInvalid(t *testing.T) {
	// Duplicate entity.
	bad := `{"name":"s","entities":[{"id":"a"},{"id":"a"}]}`
	if _, err := UnmarshalScene([]byte(bad)); err == nil {
		t.Fatal("expected duplicate entity error")
	}
	// Invalid component.
	bad2 := `{"name":"s","entities":[{"id":"a","components":[{"type":"bogus"}]}]}`
	if _, err := UnmarshalScene([]byte(bad2)); err == nil {
		t.Fatal("expected invalid component error")
	}
	// Sem nome.
	if _, err := UnmarshalScene([]byte(`{"entities":[]}`)); err == nil {
		t.Fatal("expected missing name error")
	}
}

func TestComponentsValid(t *testing.T) {
	for _, c := range []ComponentType{
		ComponentTransform, ComponentPhysics, ComponentRender, ComponentAnimation,
		ComponentAudio, ComponentInput, ComponentAI, ComponentCollider,
		ComponentHealth, ComponentScore,
	} {
		if !c.Valid() {
			t.Fatalf("component %q should be valid", c)
		}
	}
	if ComponentType("bogus").Valid() {
		t.Fatal("bogus should be invalid")
	}
	if !strings.Contains(ComponentsList(), "physics") {
		t.Fatal("ComponentsList missing physics")
	}
}
