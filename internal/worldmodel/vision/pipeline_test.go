package vision

import (
	"context"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// ──────────────────────────────────────────────────────────────
// Pipeline tests (using mock adapters)
// ──────────────────────────────────────────────────────────────

func TestPipelineConfigDefaults(t *testing.T) {
	config := DefaultPipelineConfig()

	// MinConfidence tracks the Grounding threshold so that zero-shot scores
	// (which run low for the small GroundingDINO-tiny model) are surfaced rather
	// than silently dropped by the pipeline filter.
	if config.MinConfidence != defaultGroundingThreshold {
		t.Errorf("MinConfidence: got %v, want %v", config.MinConfidence, defaultGroundingThreshold)
	}
	if config.Grounding == nil || len(config.Grounding.Prompt) == 0 {
		t.Error("default grounding config must carry a UI prompt")
	}
	if config.MaxEntities != 50 {
		t.Errorf("MaxEntities: got %v, want 50", config.MaxEntities)
	}
	if !config.Parallel {
		t.Error("Parallel should be true by default")
	}
}

func TestPipelineNew(t *testing.T) {
	config := DefaultPipelineConfig()
	p := NewPipeline(config)

	if p == nil {
		t.Fatal("NewPipeline returned nil")
	}
	if p.clip == nil {
		t.Error("clip adapter not initialized")
	}
	if p.sam == nil {
		t.Error("sam adapter not initialized")
	}
	if p.grounding == nil {
		t.Error("grounding adapter not initialized")
	}
	if p.depth == nil {
		t.Error("depth adapter not initialized")
	}
}

func TestPipelineNewMinimal(t *testing.T) {
	config := PipelineConfig{
		MinConfidence: 0.5,
	}
	p := NewPipeline(config)

	if p.clip != nil {
		t.Error("clip should be nil when not configured")
	}
	if p.sam != nil {
		t.Error("sam should be nil when not configured")
	}
}

// ──────────────────────────────────────────────────────────────
// Relation computation tests
// ──────────────────────────────────────────────────────────────

func TestComputeRelationsEmpty(t *testing.T) {
	relations := computeRelations(nil)
	if len(relations) != 0 {
		t.Errorf("empty entities should produce 0 relations, got %d", len(relations))
	}
}

func TestComputeRelationsSingle(t *testing.T) {
	entities := []worldmodel.WorldEntity{
		{ID: "a", Position: worldmodel.Vec3{X: 0, Y: 0, Z: 0}},
	}
	relations := computeRelations(entities)
	if len(relations) != 0 {
		t.Errorf("single entity should produce 0 relations, got %d", len(relations))
	}
}

func TestComputeRelationsNearby(t *testing.T) {
	entities := []worldmodel.WorldEntity{
		{ID: "a", Position: worldmodel.Vec3{X: 0, Y: 0, Z: 0}},
		{ID: "b", Position: worldmodel.Vec3{X: 1, Y: 0, Z: 0}}, // 1m to the right
	}
	relations := computeRelations(entities)

	if len(relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(relations))
	}

	rel := relations[0]
	if rel.Subject != "a" || rel.Object != "b" {
		t.Errorf("wrong subject/object: %s -> %s", rel.Subject, rel.Object)
	}
	if rel.Relation != "right_of" {
		t.Errorf("relation: got %v, want right_of", rel.Relation)
	}
}

func TestComputeRelationsBehind(t *testing.T) {
	entities := []worldmodel.WorldEntity{
		{ID: "a", Position: worldmodel.Vec3{X: 0, Y: 0, Z: 0}},
		{ID: "b", Position: worldmodel.Vec3{X: 0, Y: 0, Z: 3}}, // 3m behind
	}
	relations := computeRelations(entities)

	if len(relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(relations))
	}

	if relations[0].Relation != "behind" {
		t.Errorf("relation: got %v, want behind", relations[0].Relation)
	}
}

func TestComputeRelationsTooFar(t *testing.T) {
	entities := []worldmodel.WorldEntity{
		{ID: "a", Position: worldmodel.Vec3{X: 0, Y: 0, Z: 0}},
		{ID: "b", Position: worldmodel.Vec3{X: 25, Y: 0, Z: 0}}, // 25m away
	}
	relations := computeRelations(entities)

	if len(relations) != 0 {
		t.Errorf("entities >20m apart should not have relations, got %d", len(relations))
	}
}

func TestComputeRelationsNextTo(t *testing.T) {
	entities := []worldmodel.WorldEntity{
		{ID: "a", Position: worldmodel.Vec3{X: 0, Y: 0, Z: 0}},
		{ID: "b", Position: worldmodel.Vec3{X: 0.3, Y: 0, Z: 0}}, // 30cm away
	}
	relations := computeRelations(entities)

	if len(relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(relations))
	}

	if relations[0].Relation != "next_to" {
		t.Errorf("relation: got %v, want next_to", relations[0].Relation)
	}
	if relations[0].Confidence < 0.8 {
		t.Errorf("confidence: got %v, want >= 0.8", relations[0].Confidence)
	}
}

func TestComputeRelationsMultiple(t *testing.T) {
	entities := []worldmodel.WorldEntity{
		{ID: "car", Position: worldmodel.Vec3{X: 0, Y: 0, Z: 0}},
		{ID: "tree", Position: worldmodel.Vec3{X: 5, Y: 0, Z: 0}},
		{ID: "house", Position: worldmodel.Vec3{X: 0, Y: 0, Z: 25}}, // 25m away — too far
	}
	relations := computeRelations(entities)

	// car-tree (5m), car-house (25m — too far), tree-house (~25m — too far)
	if len(relations) != 1 {
		t.Errorf("expected 1 relation, got %d", len(relations))
	}
}

// ──────────────────────────────────────────────────────────────
// Integration test (mock pipeline)
// ──────────────────────────────────────────────────────────────

func TestPipelineIntegration(t *testing.T) {
	// This test verifies the pipeline structure without actual Python subprocesses
	config := PipelineConfig{
		MinConfidence: 0.5,
		MaxEntities:   10,
		Parallel:      false,
		// No adapters configured — pipeline will skip all steps
	}

	p := NewPipeline(config)
	ctx := context.Background()
	frame := []byte{0x89, 0x50, 0x4E, 0x47} // fake PNG

	pose := worldmodel.Pose6DoF{
		Position: worldmodel.Vec3{X: 0, Y: 0, Z: 0},
		Rotation: worldmodel.IdentityQuat(),
	}

	obs, err := p.Process(ctx, frame, pose)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	if obs == nil {
		t.Fatal("observation is nil")
	}

	if len(obs.Entities) != 0 {
		t.Errorf("expected 0 entities (no adapters), got %d", len(obs.Entities))
	}

	if obs.Latency < 0 {
		t.Error("latency should be non-negative")
	}
}

func TestPipelineMaxEntities(t *testing.T) {
	config := PipelineConfig{
		MinConfidence: 0.0,
		MaxEntities:   2, // limit to 2
		Parallel:      false,
	}

	p := NewPipeline(config)
	_ = p // Would test with mock adapters

	// Verify config is set
	if p.config.MaxEntities != 2 {
		t.Errorf("MaxEntities: got %v, want 2", p.config.MaxEntities)
	}
}

// ──────────────────────────────────────────────────────────────
// Helper: validate observation structure
// ──────────────────────────────────────────────────────────────

func TestObservationStructure(t *testing.T) {
	obs := &Observation{
		Entities: []worldmodel.WorldEntity{
			{
				ID:         "test-1",
				Type:       worldmodel.EntityObject,
				Position:   worldmodel.Vec3{X: 5, Y: 0, Z: 3},
				Label:      "car",
				Confidence: 0.95,
				Depth:      5.8,
				LastSeen:   time.Now(),
			},
		},
		Relations: []worldmodel.SpatialRelation{
			{
				Subject:    "test-1",
				Object:     "test-2",
				Relation:   "left_of",
				Distance:   3.0,
				Confidence: 0.8,
			},
		},
		Latency: 100 * time.Millisecond,
	}

	if len(obs.Entities) != 1 {
		t.Errorf("entities: got %d, want 1", len(obs.Entities))
	}
	if obs.Entities[0].Label != "car" {
		t.Errorf("label: got %v, want car", obs.Entities[0].Label)
	}
	if len(obs.Relations) != 1 {
		t.Errorf("relations: got %d, want 1", len(obs.Relations))
	}
	if obs.Latency != 100*time.Millisecond {
		t.Errorf("latency: got %v, want 100ms", obs.Latency)
	}
}
