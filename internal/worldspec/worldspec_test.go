package worldspec

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/gameengine"
	"github.com/CoscaAI/cosca/internal/world"
)

func TestWorldSpec_NewAndValidate(t *testing.T) {
	s := New(42, "cidade-costeira", "open_world")
	s.Provenance = Provenance{Source: "llm_proposal", Class: "generated", Tool: "cosca-planner@1", Seed: 42}
	if err := s.Validate(); err != nil {
		t.Fatalf("validate deveria passar: %v", err)
	}
	if s.Hash() == "" {
		t.Fatal("hash vazio")
	}
	js, err := s.JSON()
	if err != nil || len(js) == 0 {
		t.Fatalf("json: %v", err)
	}
}

func TestWorldSpec_ValidateFailClosed(t *testing.T) {
	// sem seed (I1)
	if err := New(0, "x", "y").Validate(); err == nil {
		t.Fatal("sem seed deveria falhar (I1 determinismo)")
	}
	// sem proveniência (I4)
	if err := New(42, "x", "y").Validate(); err == nil {
		t.Fatal("sem provenance.class deveria falhar (I4)")
	}
	// classe inválida
	s := New(42, "x", "y")
	s.Provenance.Class = "inventado"
	if err := s.Validate(); err == nil {
		t.Fatal("classe inválida deveria falhar")
	}
}

func TestFromWorld(t *testing.T) {
	w := world.NewWorld("w1", world.CoordinateSystem{})
	w.AddEntity(world.Entity{ID: "tree_1", Type: world.EntityType("tree.oak"), Properties: map[string]any{"height": 10}})

	s := FromWorld(w, 42)
	if s == nil || s.WorldID != "w1" || s.Seed != 42 {
		t.Fatalf("worldspec inesperado: %+v", s)
	}
	if len(s.Entities) != 1 || s.Entities[0].ID != "tree_1" {
		t.Fatalf("entidades inesperadas: %+v", s.Entities)
	}
	if s.Entities[0].Components[0].Type != "world:tree.oak" {
		t.Fatalf("componente world inesperado: %+v", s.Entities[0].Components)
	}
}

func TestFromGameEngine(t *testing.T) {
	g := &gameengine.Scene{Name: "level-1"}
	_ = g.AddEntity(&gameengine.Entity{ID: "p1", Name: "player", Components: []gameengine.Component{{Type: "transform"}}})

	s := FromGameEngine(g, 7)
	if s == nil || s.WorldID != "level-1" {
		t.Fatalf("worldspec inesperado: %+v", s)
	}
	if len(s.Entities) != 1 || s.Entities[0].Name != "player" {
		t.Fatalf("entidades inesperadas: %+v", s.Entities)
	}
}
