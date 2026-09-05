package worldmodel

import (
	"testing"
	"time"
)

// ──────────────────────────────────────────────────────────────
// Vec3 tests
// ──────────────────────────────────────────────────────────────

func TestVec3Add(t *testing.T) {
	v := Vec3{1, 2, 3}
	w := Vec3{4, 5, 6}
	got := v.Add(w)
	want := Vec3{5, 7, 9}
	if got != want {
		t.Errorf("Add: got %v, want %v", got, want)
	}
}

func TestVec3Sub(t *testing.T) {
	v := Vec3{5, 7, 9}
	w := Vec3{1, 2, 3}
	got := v.Sub(w)
	want := Vec3{4, 5, 6}
	if got != want {
		t.Errorf("Sub: got %v, want %v", got, want)
	}
}

func TestVec3Scale(t *testing.T) {
	v := Vec3{1, 2, 3}
	got := v.Scale(2)
	want := Vec3{2, 4, 6}
	if got != want {
		t.Errorf("Scale: got %v, want %v", got, want)
	}
}

func TestVec3Dot(t *testing.T) {
	v := Vec3{1, 2, 3}
	w := Vec3{4, 5, 6}
	got := v.Dot(w)
	want := 32.0 // 1*4 + 2*5 + 3*6
	if got != want {
		t.Errorf("Dot: got %v, want %v", got, want)
	}
}

func TestVec3Cross(t *testing.T) {
	v := Vec3{1, 0, 0}
	w := Vec3{0, 1, 0}
	got := v.Cross(w)
	want := Vec3{0, 0, 1}
	if got != want {
		t.Errorf("Cross: got %v, want %v", got, want)
	}
}

func TestVec3Length(t *testing.T) {
	v := Vec3{3, 4, 0}
	got := v.Length()
	if got < 4.99 || got > 5.01 {
		t.Errorf("Length: got %v, want 5.0", got)
	}
}

func TestVec3Normalize(t *testing.T) {
	v := Vec3{3, 0, 0}
	got := v.Normalize()
	want := Vec3{1, 0, 0}
	if got != want {
		t.Errorf("Normalize: got %v, want %v", got, want)
	}
}

func TestVec3NormalizeZero(t *testing.T) {
	v := Vec3{}
	got := v.Normalize()
	zero := Vec3{}
	if got != zero {
		t.Errorf("Normalize zero: got %v, want zero", got)
	}
}

func TestVec3DistanceTo(t *testing.T) {
	v := Vec3{0, 0, 0}
	w := Vec3{3, 4, 0}
	got := v.DistanceTo(w)
	if got < 4.99 || got > 5.01 {
		t.Errorf("DistanceTo: got %v, want 5.0", got)
	}
}

// ──────────────────────────────────────────────────────────────
// AABB tests
// ──────────────────────────────────────────────────────────────

func TestAABBCenter(t *testing.T) {
	aabb := AABB{Min: Vec3{0, 0, 0}, Max: Vec3{2, 4, 6}}
	got := aabb.Center()
	want := Vec3{1, 2, 3}
	if got != want {
		t.Errorf("Center: got %v, want %v", got, want)
	}
}

func TestAABBSize(t *testing.T) {
	aabb := AABB{Min: Vec3{0, 0, 0}, Max: Vec3{2, 4, 6}}
	got := aabb.Size()
	want := Vec3{2, 4, 6}
	if got != want {
		t.Errorf("Size: got %v, want %v", got, want)
	}
}

func TestAABBContains(t *testing.T) {
	aabb := AABB{Min: Vec3{0, 0, 0}, Max: Vec3{10, 10, 10}}

	tests := []struct {
		point Vec3
		want  bool
	}{
		{Vec3{5, 5, 5}, true},
		{Vec3{0, 0, 0}, true},
		{Vec3{10, 10, 10}, true},
		{Vec3{11, 5, 5}, false},
		{Vec3{-1, 5, 5}, false},
	}

	for _, tt := range tests {
		got := aabb.Contains(tt.point)
		if got != tt.want {
			t.Errorf("Contains(%v): got %v, want %v", tt.point, got, tt.want)
		}
	}
}

// ──────────────────────────────────────────────────────────────
// WorldEntity tests
// ──────────────────────────────────────────────────────────────

func TestWorldEntityHash(t *testing.T) {
	e1 := WorldEntity{Type: EntityObject, Label: "car", Position: Vec3{1, 2, 3}}
	e2 := WorldEntity{Type: EntityObject, Label: "car", Position: Vec3{1, 2, 3}}
	e3 := WorldEntity{Type: EntityObject, Label: "tree", Position: Vec3{1, 2, 3}}

	if e1.Hash() != e2.Hash() {
		t.Error("same entities should have same hash")
	}
	if e1.Hash() == e3.Hash() {
		t.Error("different entities should have different hash")
	}
}

// ──────────────────────────────────────────────────────────────
// SpatialObservation tests
// ──────────────────────────────────────────────────────────────

func TestSpatialObservationEntityCount(t *testing.T) {
	obs := SpatialObservation{
		Entities: []WorldEntity{
			{ID: "1", Type: EntityObject},
			{ID: "2", Type: EntityNPC},
			{ID: "3", Type: EntityStructure},
		},
	}
	if obs.EntityCount() != 3 {
		t.Errorf("EntityCount: got %d, want 3", obs.EntityCount())
	}
}

func TestSpatialObservationFindEntity(t *testing.T) {
	obs := SpatialObservation{
		Entities: []WorldEntity{
			{ID: "a", Label: "car"},
			{ID: "b", Label: "tree"},
		},
	}

	got := obs.FindEntity("b")
	if got == nil || got.Label != "tree" {
		t.Errorf("FindEntity: got %v, want tree", got)
	}

	if obs.FindEntity("z") != nil {
		t.Error("FindEntity should return nil for missing ID")
	}
}

func TestSpatialObservationFilterByType(t *testing.T) {
	obs := SpatialObservation{
		Entities: []WorldEntity{
			{ID: "1", Type: EntityObject},
			{ID: "2", Type: EntityNPC},
			{ID: "3", Type: EntityObject},
		},
	}
	got := obs.FilterByType(EntityObject)
	if len(got) != 2 {
		t.Errorf("FilterByType: got %d entities, want 2", len(got))
	}
}

func TestSpatialObservationFilterByConfidence(t *testing.T) {
	obs := SpatialObservation{
		Entities: []WorldEntity{
			{ID: "1", Confidence: 0.9},
			{ID: "2", Confidence: 0.3},
			{ID: "3", Confidence: 0.8},
		},
	}
	got := obs.FilterByConfidence(0.5)
	if len(got) != 2 {
		t.Errorf("FilterByConfidence: got %d entities, want 2", len(got))
	}
}

// ──────────────────────────────────────────────────────────────
// WorldState tests
// ──────────────────────────────────────────────────────────────

func TestWorldStateEntityCount(t *testing.T) {
	state := WorldState{
		Entities: []WorldEntity{
			{ID: "1"},
			{ID: "2"},
		},
	}
	if state.EntityCount() != 2 {
		t.Errorf("EntityCount: got %d, want 2", state.EntityCount())
	}
}

func TestWorldStateFindEntity(t *testing.T) {
	state := WorldState{
		Entities: []WorldEntity{
			{ID: "x", Label: "mountain"},
		},
	}
	got := state.FindEntity("x")
	if got == nil || got.Label != "mountain" {
		t.Errorf("FindEntity: got %v, want mountain", got)
	}
}

// ──────────────────────────────────────────────────────────────
// ClimateState tests
// ──────────────────────────────────────────────────────────────

func TestClimateStateDefaults(t *testing.T) {
	c := ClimateState{}
	if c.Temperature != 0 {
		t.Errorf("default temperature: got %v, want 0", c.Temperature)
	}
	if c.Season != "" {
		t.Errorf("default season: got %v, want empty", c.Season)
	}
}

// ──────────────────────────────────────────────────────────────
// IdentityQuat tests
// ──────────────────────────────────────────────────────────────

func TestIdentityQuat(t *testing.T) {
	q := IdentityQuat()
	if q.W != 1 || q.X != 0 || q.Y != 0 || q.Z != 0 {
		t.Errorf("IdentityQuat: got %v, want {1,0,0,0}", q)
	}
}

// ──────────────────────────────────────────────────────────────
// Integration test — full observation cycle
// ──────────────────────────────────────────────────────────────

func TestFullObservationCycle(t *testing.T) {
	// Create an observation
	obs := SpatialObservation{
		Timestamp: time.Now(),
		AgentPose: Pose6DoF{
			Position: Vec3{0, 0, 0},
			Rotation: IdentityQuat(),
		},
		Entities: []WorldEntity{
			{
				ID:         "entity-1",
				Type:       EntityObject,
				Position:   Vec3{5, 0, 3},
				Label:      "red car",
				Confidence: 0.95,
				Depth:      5.8,
				LastSeen:   time.Now(),
			},
			{
				ID:         "entity-2",
				Type:       EntityNPC,
				Position:   Vec3{10, 0, 0},
				Label:      "villager",
				Confidence: 0.87,
				Depth:      10.0,
				LastSeen:   time.Now(),
			},
		},
		Relations: []SpatialRelation{
			{
				Subject:    "entity-1",
				Object:     "entity-2",
				Relation:   "left_of",
				Distance:   5.0,
				Confidence: 0.9,
			},
		},
	}

	// Verify observation
	if obs.EntityCount() != 2 {
		t.Fatalf("EntityCount: got %d, want 2", obs.EntityCount())
	}

	// Find entity
	car := obs.FindEntity("entity-1")
	if car == nil || car.Label != "red car" {
		t.Fatal("FindEntity failed for red car")
	}

	// Filter by type
	npcs := obs.FilterByType(EntityNPC)
	if len(npcs) != 1 || npcs[0].Label != "villager" {
		t.Fatal("FilterByType failed for NPC")
	}

	// Filter by confidence
	high := obs.FilterByConfidence(0.9)
	if len(high) != 1 {
		t.Fatalf("FilterByConfidence: got %d, want 1", len(high))
	}

	// Create world state
	state := WorldState{
		Entities:  obs.Entities,
		Relations: obs.Relations,
		Climate: ClimateState{
			Temperature: 22.5,
			Season:      Summer,
			TimeOfDay:   14.0,
		},
		Timestamp: time.Now(),
		Step:      1,
	}

	if state.EntityCount() != 2 {
		t.Fatal("WorldState EntityCount failed")
	}
}
