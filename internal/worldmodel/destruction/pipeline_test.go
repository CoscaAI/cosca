package destruction

import (
	"context"
	"testing"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// ──────────────────────────────────────────────────────────────
// Pipeline tests
// ──────────────────────────────────────────────────────────────

func TestPipelineConfigDefaults(t *testing.T) {
	config := DefaultPipelineConfig()

	if config.MaxFrags != 20 {
		t.Errorf("MaxFrags: got %v, want 20", config.MaxFrags)
	}
	if config.Fracture == nil {
		t.Error("Fracture config should not be nil")
	}
	if config.Fracture.Method != "voronoi" {
		t.Errorf("Method: got %v, want voronoi", config.Fracture.Method)
	}
}

func TestPipelineNew(t *testing.T) {
	config := DefaultPipelineConfig()
	p := NewPipeline(config)

	if p == nil {
		t.Fatal("NewPipeline returned nil")
	}
	if p.fracture == nil {
		t.Error("fracture adapter not initialized")
	}
}

func TestPipelineNewMinimal(t *testing.T) {
	config := PipelineConfig{}
	p := NewPipeline(config)

	if p.fracture != nil {
		t.Error("fracture should be nil when not configured")
	}
}

func TestPipelineNoAdapter(t *testing.T) {
	config := PipelineConfig{}
	p := NewPipeline(config)
	ctx := context.Background()

	action := worldmodel.PhysicalAction{
		Type:     "apply_impulse",
		TargetID: "wall_01",
		Impulse:  worldmodel.Vec3{X: 0, Y: 0, Z: -500},
	}
	mesh := worldmodel.Mesh{
		Vertices: []worldmodel.Vec3{
			{X: 0, Y: 0, Z: 0}, {X: 1, Y: 0, Z: 0}, {X: 1, Y: 1, Z: 0}, {X: 0, Y: 1, Z: 0},
		},
		Faces: [][3]int{{0, 1, 2}, {0, 2, 3}},
	}
	material := worldmodel.Material{
		Name:        "concrete",
		Density:     2400,
		Hardness:    0.7,
		FractureType: "voronoi",
		Toughness:   0.5,
	}

	_, err := p.Destroy(ctx, action, mesh, material)
	if err == nil {
		t.Error("Destroy without adapter should fail")
	}
}

// ──────────────────────────────────────────────────────────────
// Debris generation tests
// ──────────────────────────────────────────────────────────────

func TestGenerateDebris(t *testing.T) {
	p := NewPipeline(DefaultPipelineConfig())

	fragments := []worldmodel.Fragment{
		{
			Position:              worldmodel.Vec3{X: 1, Y: 0, Z: 0},
			Velocity:              worldmodel.Vec3{X: 5, Y: 0, Z: 0},
			BoundingSphereRadius:  0.5,
		},
		{
			Position:              worldmodel.Vec3{X: 2, Y: 0, Z: 0},
			Velocity:              worldmodel.Vec3{X: 3, Y: 0, Z: 0},
			BoundingSphereRadius:  0.3,
		},
	}

	action := worldmodel.PhysicalAction{
		Impulse: worldmodel.Vec3{X: 0, Y: 0, Z: -10},
	}

	debris := p.generateDebris(fragments, action)

	// 2 fragments: dust_0, sound (from first fragment), dust_1 = 3 debris
	if len(debris) != 3 {
		t.Errorf("debris count: got %d, want 3", len(debris))
	}

	// First debris is dust from fragment 0
	if debris[0].Type != "dust" {
		t.Errorf("first debris type: got %v, want dust", debris[0].Type)
	}

	// Second debris is sound (from first fragment)
	if debris[1].Type != "sound" {
		t.Errorf("second debris type: got %v, want sound", debris[1].Type)
	}
	if debris[1].Velocity.X != 0 || debris[1].Velocity.Y != 0 || debris[1].Velocity.Z != 0 {
		t.Error("sound debris should have zero velocity")
	}

	// Third debris is dust from fragment 1
	if debris[2].Type != "dust" {
		t.Errorf("third debris type: got %v, want dust", debris[2].Type)
	}
}

// ──────────────────────────────────────────────────────────────
// Structural impact tests
// ──────────────────────────────────────────────────────────────

func TestComputeStructuralImpactEmpty(t *testing.T) {
	p := NewPipeline(DefaultPipelineConfig())

	fragments := []worldmodel.Fragment{}
	mesh := worldmodel.Mesh{}

	impact := p.computeStructuralImpact(fragments, mesh)
	if impact != 0.0 {
		t.Errorf("empty impact: got %v, want 0.0", impact)
	}
}

func TestComputeStructuralImpactWithFragments(t *testing.T) {
	p := NewPipeline(DefaultPipelineConfig())

	fragments := []worldmodel.Fragment{
		{Volume: 0.5},
		{Volume: 0.3},
	}
	mesh := worldmodel.Mesh{
		Vertices: []worldmodel.Vec3{
			{X: 0, Y: 0, Z: 0}, {X: 2, Y: 0, Z: 0}, {X: 2, Y: 2, Z: 0}, {X: 0, Y: 2, Z: 0},
			{X: 0, Y: 0, Z: 2}, {X: 2, Y: 0, Z: 2}, {X: 2, Y: 2, Z: 2}, {X: 0, Y: 2, Z: 2},
		},
	}

	impact := p.computeStructuralImpact(fragments, mesh)

	if impact <= 0 {
		t.Errorf("impact should be > 0, got %v", impact)
	}
	if impact > 1.0 {
		t.Errorf("impact should be <= 1.0, got %v", impact)
	}
}

// ──────────────────────────────────────────────────────────────
// Mesh volume estimation tests
// ──────────────────────────────────────────────────────────────

func TestEstimateMeshVolumeEmpty(t *testing.T) {
	mesh := worldmodel.Mesh{}
	vol := estimateMeshVolume(mesh)
	if vol != 0.0 {
		t.Errorf("empty mesh volume: got %v, want 0.0", vol)
	}
}

func TestEstimateMeshVolumeCube(t *testing.T) {
	mesh := worldmodel.Mesh{
		Vertices: []worldmodel.Vec3{
			{X: 0, Y: 0, Z: 0}, {X: 2, Y: 0, Z: 0}, {X: 2, Y: 2, Z: 0}, {X: 0, Y: 2, Z: 0},
			{X: 0, Y: 0, Z: 2}, {X: 2, Y: 0, Z: 2}, {X: 2, Y: 2, Z: 2}, {X: 0, Y: 2, Z: 2},
		},
	}
	vol := estimateMeshVolume(mesh)

	// Bounding box: 2×2×2 = 8
	if vol != 8.0 {
		t.Errorf("cube volume: got %v, want 8.0", vol)
	}
}

// ──────────────────────────────────────────────────────────────
// Material tests
// ──────────────────────────────────────────────────────────────

func TestMaterialTypes(t *testing.T) {
	materials := []worldmodel.Material{
		{Name: "concrete", Density: 2400, Hardness: 0.7, FractureType: "voronoi", Toughness: 0.5},
		{Name: "glass", Density: 2500, Hardness: 0.9, FractureType: "cutoff", Toughness: 0.1},
		{Name: "wood", Density: 600, Hardness: 0.4, FractureType: "markov", Toughness: 0.8},
		{Name: "metal", Density: 7800, Hardness: 0.8, FractureType: "voronoi", Toughness: 0.9},
	}

	for _, m := range materials {
		if m.Density <= 0 {
			t.Errorf("%s: density should be > 0", m.Name)
		}
		if m.Hardness < 0 || m.Hardness > 1 {
			t.Errorf("%s: hardness should be 0-1, got %v", m.Name, m.Hardness)
		}
	}
}

// ──────────────────────────────────────────────────────────────
// DestructionResult structure test
// ──────────────────────────────────────────────────────────────

func TestDestructionResultStructure(t *testing.T) {
	result := &DestructionResult{
		Fragments: []worldmodel.Fragment{
			{ID: "f1", Volume: 0.5, Mass: 1.2},
			{ID: "f2", Volume: 0.3, Mass: 0.8},
		},
		Debris: []worldmodel.Debris{
			{Type: "dust", Lifetime: 2.0},
			{Type: "sound", Lifetime: 0.5},
		},
		StructuralImpact: 0.65,
	}

	if len(result.Fragments) != 2 {
		t.Errorf("fragments: got %d, want 2", len(result.Fragments))
	}
	if len(result.Debris) != 2 {
		t.Errorf("debris: got %d, want 2", len(result.Debris))
	}
	if result.StructuralImpact != 0.65 {
		t.Errorf("structural impact: got %v, want 0.65", result.StructuralImpact)
	}
}

// ──────────────────────────────────────────────────────────────
// Adapter config tests
// ──────────────────────────────────────────────────────────────

func TestFractureConfig(t *testing.T) {
	config := FractureConfig{
		Method:   "markov",
		MaxFrags: 30,
		Seed:     42,
		Script:   "custom/fracture.py",
	}
	adapter := NewFractureAdapter(config)

	if adapter.config.Method != "markov" {
		t.Errorf("method: got %v, want markov", adapter.config.Method)
	}
	if adapter.config.MaxFrags != 30 {
		t.Errorf("max_frags: got %v, want 30", adapter.config.MaxFrags)
	}
}

func TestFractureConfigDefaultScript(t *testing.T) {
	config := FractureConfig{Method: "voronoi"}
	adapter := NewFractureAdapter(config)

	if adapter.config.Script == "" {
		t.Error("script should have default value")
	}
}
