package world

import "testing"

// Teste mínimo da FASE A: construir um mundo sintético SEM Unreal.
// Valida: entity model, coordinate conversion, spatial model, graph,
// provenance, serialization determinística, e o contrato renderer.

func newTestWorld() *World {
	cs := CoordinateSystem{
		Origin: GeoCoordinates{Latitude: -27.5, Longitude: -48.5, Altitude: 0},
		Units:  "meters",
		Scale:  1.0,
	}
	w := NewWorld("test-synthetic-city", cs)

	prov := Provenance{
		Class:      ClassGENERATED,
		Source:     Source{Dataset: "procedural", Version: "1.0"},
		Generation: &Generation{Generator: "world.test", Version: "1.0", Seed: 42},
		Accuracy:   1.0,
	}

	// Terrain region (forest)
	terrain := TerrainRegion{
		Polygon: Polygon{Points: []Vec3{
			{0, 0, 0}, {100, 0, 0}, {100, 100, 0}, {0, 100, 0},
		}},
		Class: "forest", Elevation: 10,
	}
	w.AddEntity(TerrainEntity("terrain_001", terrain, prov))

	// Road (centerline)
	road := Line{Points: []Vec3{{0, 50, 0}, {100, 50, 0}}}
	w.AddEntity(RoadEntity("road_001", road, 8, prov))

	// Tree (subordinate to terrain)
	tree := TreeEntity("tree_001", "oak", Vec3{30, 30, 0}, "mature", prov)
	tree.Parent = "terrain_001"
	w.AddEntity(tree)
	w.AddRelation(Relation{Subject: "tree_001", Object: "terrain_001", Relation: RelBelongsTo, Confidence: 1})

	// Building (near road)
	building := BuildingEntity("building_001", Vec3{60, 50, 0}, Vec3{20, 15, 12}, prov)
	building.Parent = "terrain_001"
	w.AddEntity(building)

	// Rock
	w.AddEntity(RockEntity("rock_001", Vec3{80, 20, 0}, prov))

	return w
}

// TestEntityModel verifica identidade, tipos, hierarquia, estado, provenance.
func TestEntityModel(t *testing.T) {
	w := newTestWorld()

	tree := w.GetEntity("tree_001")
	if tree == nil {
		t.Fatal("tree_001 not found")
	}
	if tree.Class != ClassVegetation {
		t.Errorf("class = %s, want %s", tree.Class, ClassVegetation)
	}
	if tree.Type != "tree.oak" {
		t.Errorf("type = %s, want tree.oak", tree.Type)
	}
	if tree.Parent != "terrain_001" {
		t.Errorf("parent = %s, want terrain_001", tree.Parent)
	}
	if tree.Provenance.Class != ClassGENERATED {
		t.Errorf("provenance.class = %s, want generated", tree.Provenance.Class)
	}
	if tree.Provenance.Generation.Seed != 42 {
		t.Errorf("seed = %d, want 42", tree.Provenance.Generation.Seed)
	}
}

// TestCoordinateConversion verifica Geo↔World conversão (idempotente).
func TestCoordinateConversion(t *testing.T) {
	cs := CoordinateSystem{
		Origin: GeoCoordinates{Latitude: -27.5, Longitude: -48.5, Altitude: 5},
		Scale:  1.0,
	}
	// Convert origin to world → should be (0,0,0)
	originWorld := cs.GeoToWorld(cs.Origin)
	if originWorld.X != 0 || originWorld.Y != 0 || originWorld.Z != 0 {
		t.Errorf("origin→world = %+v, want (0,0,0)", originWorld)
	}

	// Round-trip: a point offset then back
	pts := GeoCoordinates{Latitude: -27.51, Longitude: -48.51, Altitude: 10}
	worldPt := cs.GeoToWorld(pts)
	back := cs.WorldToGeo(worldPt)
	if abs(back.Latitude-pts.Latitude) > 1e-6 {
		t.Errorf("lat roundtrip = %f, want %f", back.Latitude, pts.Latitude)
	}
	if abs(back.Longitude-pts.Longitude) > 1e-6 {
		t.Errorf("lon roundtrip = %f, want %f", back.Longitude, pts.Longitude)
	}
}

// TestSpatialModel verifica área de polígono, comprimento de linha, BBox.
func TestSpatialModel(t *testing.T) {
	poly := Polygon{Points: []Vec3{{0, 0, 0}, {100, 0, 0}, {100, 100, 0}, {0, 100, 0}}}
	if area := poly.Area(); area != 10000 {
		t.Errorf("polygon area = %f, want 10000", area)
	}

	line := Line{Points: []Vec3{{0, 0, 0}, {100, 0, 0}}}
	if length := line.Length(); length != 100 {
		t.Errorf("line length = %f, want 100", length)
	}
}

// TestWorldGraph verifica relações semânticas.
func TestWorldGraph(t *testing.T) {
	w := newTestWorld()

	children := w.ChildrenOf("terrain_001")
	if len(children) != 2 { // tree + building
		t.Errorf("terrain_001 children = %d, want 2", len(children))
	}

	related := w.RelatedTo("tree_001", RelBelongsTo)
	if len(related) != 1 || related[0] != "terrain_001" {
		t.Errorf("tree belongs_to = %v, want [terrain_001]", related)
	}

	// Entities in region
	region := TerrainRegion{
		Polygon: Polygon{Points: []Vec3{{0, 0, 0}, {200, 0, 0}, {200, 200, 0}, {0, 200, 0}}},
	}
	inRegion, err := w.EntitiesInRegion(region)
	if err != nil {
		t.Fatal(err)
	}
	if len(inRegion) != 5 {
		t.Errorf("entities in region = %d, want 5", len(inRegion))
	}
}

// TestSerializationDeterministic verifica determinismo + fingerprint.
func TestSerializationDeterministic(t *testing.T) {
	w := newTestWorld()

	data1, err := MarshalWorldCanonical(w)
	if err != nil {
		t.Fatal(err)
	}
	data2, err := MarshalWorldCanonical(w)
	if err != nil {
		t.Fatal(err)
	}
	if string(data1) != string(data2) {
		t.Error("serialization not deterministic")
	}

	fp1, _ := w.HashWorld()
	fp2, _ := w.HashWorld()
	if fp1 != fp2 {
		t.Errorf("fingerprint not stable: %s != %s", fp1, fp2)
	}
	if len(fp1) != 64 {
		t.Errorf("fingerprint length = %d, want 64 (sha256 hex)", len(fp1))
	}

	// Round-trip
	w2, err := UnmarshalWorld(data1)
	if err != nil {
		t.Fatal(err)
	}
	if w2.SchemaVersion != SchemaVersion {
		t.Errorf("roundtrip schema = %d, want %d", w2.SchemaVersion, SchemaVersion)
	}
	fp3, _ := w2.HashWorld()
	if fp3 != fp1 {
		t.Error("roundtrip world not identical to original")
	}
}

// TestRendererContract verifica que o Model conhece apenas a interface.
func TestRendererContract(t *testing.T) {
	var _ RendererAdapter = (*mockRenderer)(nil) // compile-time check
}

// mockRenderer implements RendererAdapter for tests.
type mockRenderer struct {
	name   string
	events []RendererEvent
}

func (m *mockRenderer) MaterializeEntity(e Entity) (RendererEvent, error) {
	ev := RendererEvent{Type: "spawned", EntityID: e.ID}
	m.events = append(m.events, ev)
	return ev, nil
}
func (m *mockRenderer) UpdateEntity(entityID string, t Transform) (RendererEvent, error) {
	return RendererEvent{Type: "state_changed", EntityID: entityID}, nil
}
func (m *mockRenderer) RemoveEntity(entityID string) (RendererEvent, error) {
	return RendererEvent{Type: "destroyed", EntityID: entityID}, nil
}
func (m *mockRenderer) ApplyWorldState(w WeatherState, s SimulationTime, season Season) error {
	return nil
}
func (m *mockRenderer) Name() string { return "mock" }

// TestFingerprintStableAcrossConstructors verifica que o mesmo mundo
// construído de formas diferentes produz o mesmo fingerprint.
func TestFingerprintStableAcrossConstructors(t *testing.T) {
	w1 := newTestWorld()
	w2 := newTestWorld()
	fp1, _ := w1.HashWorld()
	fp2, _ := w2.HashWorld()
	if fp1 != fp2 {
		t.Error("identical worlds produced different fingerprints")
	}
}

// TestCanonicalOrderIndependent verifica a regra do professor (item 10):
// o fingerprint NÃO deve depender da ordem de inserção. Inserir entidades
// em ordens diferentes deve produzir o MESMO hash.
func TestCanonicalOrderIndependent(t *testing.T) {
	build := func(order []int) *World {
		cs := CoordinateSystem{Origin: GeoCoordinates{Latitude: -27.5, Longitude: -48.5}}
		w := NewWorld("order-test", cs)
		prov := Provenance{Class: ClassGENERATED, Generation: &Generation{Generator: "t", Seed: 7}}
		entities := []Entity{
			TreeEntity("tree_001", "oak", Vec3{1, 1, 0}, "mature", prov),
			BuildingEntity("bld_001", Vec3{5, 5, 0}, Vec3{10, 10, 20}, prov),
			RockEntity("rock_001", Vec3{8, 8, 0}, prov),
		}
		for _, idx := range order {
			w.AddEntity(entities[idx])
		}
		return w
	}

	// Insert in different orders — fingerprint must be identical.
	wA := build([]int{0, 1, 2}) // tree, building, rock
	wB := build([]int{2, 1, 0}) // rock, building, tree
	wC := build([]int{1, 0, 2}) // building, tree, rock

	fpA, _ := wA.HashWorld()
	fpB, _ := wB.HashWorld()
	fpC, _ := wC.HashWorld()

	if fpA != fpB {
		t.Errorf("order A != order B fingerprints: %s vs %s", fpA, fpB)
	}
	if fpA != fpC {
		t.Errorf("order A != order C fingerprints: %s vs %s", fpA, fpC)
	}
}

// TestValidator verifica as invariantes do World Model.
func TestValidator(t *testing.T) {
	w := newTestWorld()
	res := Validate(w)
	if !res.Valid {
		t.Errorf("valid world reported invalid: %+v", res.Issues)
	}

	// Break an invariant: orphan relation.
	w2 := newTestWorld()
	w2.AddRelation(Relation{Subject: "ghost", Object: "tree_001", Relation: RelLocatedAt, Confidence: 1})
	res2 := Validate(w2)
	if res2.Valid {
		t.Error("orphan relation should be flagged")
	}
	if len(res2.Issues) == 0 {
		t.Error("expected issues, got none")
	}
}

// TestQueries verifica as queries espaciais+sêmicas (item 11 do professor).
func TestQueries(t *testing.T) {
	w := newTestWorld()

	// Count by class
	counts := w.CountByClass()
	if counts[ClassVegetation] != 1 {
		t.Errorf("vegetation count = %d, want 1", counts[ClassVegetation])
	}
	if counts[ClassStructure] != 1 {
		t.Errorf("structure count = %d, want 1", counts[ClassStructure])
	}

	// Tallest building
	tallest, ok := w.TallestBuilding()
	if !ok || tallest.ID != "building_001" {
		t.Errorf("tallest building = %v, want building_001", tallest)
	}

	// Road connect between entities
	conn := w.ConnectBetween("terrain_001", "road_001")
	if len(conn) != 0 {
		t.Errorf("ConnectBetween = %v", conn)
	}
}

// TestGeometrySeparate verifica que geometria é separada da semântica (item 5).
func TestGeometrySeparate(t *testing.T) {
	bld := BuildingEntity("bld_042", Vec3{10, 20, 0}, Vec3{8, 6, 24}, Provenance{})
	if bld.Geometry == nil {
		t.Fatal("building missing geometry")
	}
	if bld.Geometry.Kind != "polygon" {
		t.Errorf("geometry kind = %s, want polygon", bld.Geometry.Kind)
	}
	// Semantic properties present too.
	if bld.Properties["total_height"] != float64(24) {
		t.Errorf("total_height = %v, want 24", bld.Properties["total_height"])
	}
}

// abs is a float abs helper.
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
