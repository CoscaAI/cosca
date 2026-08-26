package worlddebug

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/world"
)

// buildTestWorld creates a small deterministic world for inspection.
func buildTestWorld() *world.World {
	cs := world.CoordinateSystem{Origin: world.GeoCoordinates{Latitude: -27.6375, Longitude: -48.6765}}
	w := world.NewWorld("inspect-test", cs)
	prov := world.Provenance{
		Class:   world.ClassFACT,
		Source:  world.Source{Dataset: "osm", Hash: "osm:way:123"},
	}
	// Building (observed footprint).
	b := world.BuildingEntity("building_001", world.Vec3{X: 10, Y: 10, Z: 0}, world.Vec3{X: 12, Y: 9, Z: 18}, prov)
	w.AddEntity(b)
	// Road (observed linestring).
	road := world.RoadEntity("road_001", world.Line{Points: []world.Vec3{{X: 0, Y: 50, Z: 0}, {X: 100, Y: 50, Z: 0}}}, 8, prov)
	w.AddEntity(road)
	// Tree (inferred type).
	t := world.TreeEntity("tree_001", "oak", world.Vec3{X: 25, Y: 25, Z: 0}, "mature", prov)
	w.AddEntity(t)
	// Relations.
	w.AddRelation(world.Relation{Subject: "building_001", Object: "road_001", Relation: world.RelLocatedAt, Confidence: 1})
	return w
}

// TestInspectorExplain verifies the EntityInspect structure per the professor.
func TestInspectorExplain(t *testing.T) {
	w := buildTestWorld()
	insp := New(w)

	// Explain a building.
	ex, ok := insp.Explain("building_001")
	if !ok {
		t.Fatal("building_001 not found")
	}
	if ex.ID != "building_001" {
		t.Errorf("id = %s", ex.ID)
	}
	if ex.Class != "structure" {
		t.Errorf("class = %s, want structure", ex.Class)
	}
	if ex.Source != "osm" {
		t.Errorf("source = %s, want osm", ex.Source)
	}
	if ex.Geometry != "polygon" {
		t.Errorf("geometry = %s, want polygon", ex.Geometry)
	}
	if ex.Representation.Asset == "" {
		t.Error("missing representation asset")
	}
	if len(ex.Reasoning.Reasons) == 0 {
		t.Error("missing reasoning reasons")
	}
	if len(ex.Provenance) == 0 {
		t.Error("missing provenance")
	}
}

// TestInspectorEpistemology verifies OBSERVED/INFERRED/GENERATED separation.
func TestInspectorEpistemology(t *testing.T) {
	w := buildTestWorld()
	insp := New(w)
	ex, _ := insp.Explain("building_001")

	var hasObserved, hasInferred, hasGenerated bool
	for _, p := range ex.Provenance {
		switch p.Class {
		case Observed:
			hasObserved = true
		case Inferred:
			hasInferred = true
		case Generated:
			hasGenerated = true
		}
	}
	if !hasObserved {
		t.Error("expected OBSERVED provenance (class/geometry from source)")
	}
	if !hasGenerated {
		t.Error("expected GENERATED provenance (visual_mesh)")
	}
	if !hasObserved && !hasInferred {
		t.Error("expected at least one epistemic class")
	}
}

// TestReplay verifies the timeline step-by-step trace.
func TestReplay(t *testing.T) {
	w := buildTestWorld()
	replay := BuildReplay(w)
	events := replay.Events()
	if len(events) == 0 {
		t.Fatal("empty replay")
	}
	// First event should be ingest, last should be validate.
	if events[0].Type != "ingest" {
		t.Errorf("first event = %s, want ingest", events[0].Type)
	}
	last := events[len(events)-1]
	if last.Type != "validate" {
		t.Errorf("last event = %s, want validate", last.Type)
	}
	// Events must be sequential.
	for i := 1; i < len(events); i++ {
		if events[i].Step != events[i-1].Step+1 {
			t.Errorf("non-sequential step at %d", i)
		}
	}
}

// TestAssetBrowserSemantic verifies query by semantics, not filename.
func TestAssetBrowserSemantic(t *testing.T) {
	b := NewBrowser(DefaultTreeCatalog())

	// Query: mature oak in urban context.
	matches := b.Query(AssetQuery{
		Category: "tree",
		Species:  []string{"oak"},
		Contexts: []string{"urban"},
		Ages:     []string{"mature"},
		MaxScale: 12,
	})
	if len(matches) == 0 {
		t.Fatal("no matches")
	}
	// Best match should be urban oak.
	if matches[0].AssetID != "tree_urban_oak_mature" {
		t.Errorf("best match = %s, want tree_urban_oak_mature", matches[0].AssetID)
	}
}

// TestInspectorReadOnly verifies the Inspector never mutates the world.
func TestInspectorReadOnly(t *testing.T) {
	w := buildTestWorld()
	before, _ := world.WorldFingerprint(w)

	insp := New(w)
	insp.Explain("building_001")
	BuildReplay(w)

	after, _ := world.WorldFingerprint(w)
	if before != after {
		t.Error("Inspector mutated the World Model — violates rule of gold")
	}
}
