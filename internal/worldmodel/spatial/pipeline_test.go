package spatial

import (
	"context"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// ──────────────────────────────────────────────────────────────
// Pipeline tests
// ──────────────────────────────────────────────────────────────

func TestPipelineConfigDefaults(t *testing.T) {
	config := DefaultPipelineConfig()

	if config.MaxPointCloudSize != 100000 {
		t.Errorf("MaxPointCloudSize: got %v, want 100000", config.MaxPointCloudSize)
	}
	if config.MinConfidence != 0.3 {
		t.Errorf("MinConfidence: got %v, want 0.3", config.MinConfidence)
	}
	if config.UpdateRate != 10.0 {
		t.Errorf("UpdateRate: got %v, want 10.0", config.UpdateRate)
	}
}

func TestPipelineNew(t *testing.T) {
	config := DefaultPipelineConfig()
	p := NewPipeline(config)

	if p == nil {
		t.Fatal("NewPipeline returned nil")
	}
	if p.slam == nil {
		t.Error("slam adapter not initialized")
	}
	if p.recon == nil {
		t.Error("reconstruct adapter not initialized")
	}
	if p.reason == nil {
		t.Error("reasoning adapter not initialized")
	}
}

func TestPipelineNewMinimal(t *testing.T) {
	config := PipelineConfig{}
	p := NewPipeline(config)

	if p.slam != nil {
		t.Error("slam should be nil when not configured")
	}
}

// ──────────────────────────────────────────────────────────────
// Cluster tests
// ──────────────────────────────────────────────────────────────

func TestClusterPointsEmpty(t *testing.T) {
	clusters := clusterPoints(nil, 1.0)
	if len(clusters) != 0 {
		t.Errorf("empty points should produce 0 clusters, got %d", len(clusters))
	}
}

func TestClusterPointsSingle(t *testing.T) {
	points := []worldmodel.Vec3{{0, 0, 0}}
	clusters := clusterPoints(points, 1.0)
	if len(clusters) != 1 {
		t.Errorf("single point should produce 1 cluster, got %d", len(clusters))
	}
}

func TestClusterPointsGrid(t *testing.T) {
	points := []worldmodel.Vec3{
		{0.1, 0, 0},   // cell (0,0,0)
		{0.5, 0, 0},   // cell (0,0,0)
		{3.0, 0, 0},   // cell (3,0,0)
		{3.1, 0, 0},   // cell (3,0,0)
		{0.1, 3.5, 0}, // cell (0,3,0)
	}
	clusters := clusterPoints(points, 2.0)

	if len(clusters) != 3 {
		t.Errorf("expected 3 clusters, got %d", len(clusters))
	}
}

func TestClusterPointsDense(t *testing.T) {
	points := make([]worldmodel.Vec3, 100)
	for i := range points {
		points[i] = worldmodel.Vec3{
			X: float64(i%10) * 0.1,
			Y: float64(i/10) * 0.1,
			Z: 0,
		}
	}
	clusters := clusterPoints(points, 1.0)

	if len(clusters) != 1 {
		t.Errorf("dense points should produce 1 cluster, got %d", len(clusters))
	}
}

// ──────────────────────────────────────────────────────────────
// Spatial relation computation tests
// ──────────────────────────────────────────────────────────────

func TestComputeSpatialRelationsEmpty(t *testing.T) {
	pc := worldmodel.PointCloud{}
	pose := worldmodel.Pose6DoF{}
	relations := computeSpatialRelations(pc, pose)

	if len(relations) != 0 {
		t.Errorf("empty point cloud should produce 0 relations, got %d", len(relations))
	}
}

func TestComputeSpatialRelationsWithPoints(t *testing.T) {
	pc := worldmodel.PointCloud{
		Points: []worldmodel.Vec3{
			{5, 0, 0},   // right of agent
			{5.1, 0, 0},
			{5.2, 0, 0},
			{-3, 0, 0},  // left of agent
			{-3.1, 0, 0},
			{-3.2, 0, 0},
		},
	}
	pose := worldmodel.Pose6DoF{
		Position: worldmodel.Vec3{0, 0, 0},
	}

	relations := computeSpatialRelations(pc, pose)

	if len(relations) < 2 {
		t.Fatalf("expected >= 2 relations, got %d", len(relations))
	}

	// Check that we have right_of and left_of
	hasRight := false
	hasLeft := false
	for _, r := range relations {
		if r.Relation == "right_of_agent" {
			hasRight = true
		}
		if r.Relation == "left_of_agent" {
			hasLeft = true
		}
	}

	if !hasRight {
		t.Error("missing right_of_agent relation")
	}
	if !hasLeft {
		t.Error("missing left_of_agent relation")
	}
}

func TestComputeSpatialRelationsNearby(t *testing.T) {
	pc := worldmodel.PointCloud{
		Points: []worldmodel.Vec3{
			{0.3, 0, 0}, // very close
			{0.31, 0, 0},
			{0.29, 0, 0},
		},
	}
	pose := worldmodel.Pose6DoF{
		Position: worldmodel.Vec3{0, 0, 0},
	}

	relations := computeSpatialRelations(pc, pose)

	if len(relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(relations))
	}

	if relations[0].Relation != "next_to_agent" {
		t.Errorf("relation: got %v, want next_to_agent", relations[0].Relation)
	}
	if relations[0].Confidence < 0.7 {
		t.Errorf("confidence: got %v, want >= 0.7", relations[0].Confidence)
	}
}

// ──────────────────────────────────────────────────────────────
// SpatialResult tests
// ──────────────────────────────────────────────────────────────

func TestSpatialResultStructure(t *testing.T) {
	result := &SpatialResult{
		Pose: worldmodel.Pose6DoF{
			Position: worldmodel.Vec3{1, 2, 3},
			Rotation: worldmodel.IdentityQuat(),
		},
		PointCloud: worldmodel.PointCloud{
			Points: []worldmodel.Vec3{
				{5, 0, 0},
				{-3, 0, 0},
			},
			Timestamp: time.Now(),
		},
		Relations: []worldmodel.SpatialRelation{
			{Subject: "region_0", Object: "agent", Relation: "right_of_agent", Distance: 5.0},
		},
		Latency: 50 * time.Millisecond,
	}

	if result.Pose.Position.X != 1 {
		t.Errorf("pose X: got %v, want 1", result.Pose.Position.X)
	}
	if len(result.PointCloud.Points) != 2 {
		t.Errorf("point cloud: got %d points, want 2", len(result.PointCloud.Points))
	}
	if len(result.Relations) != 1 {
		t.Errorf("relations: got %d, want 1", len(result.Relations))
	}
}

// ──────────────────────────────────────────────────────────────
// Integration test (no adapters)
// ──────────────────────────────────────────────────────────────

func TestPipelineIntegration(t *testing.T) {
	config := PipelineConfig{
		MaxPointCloudSize: 1000,
		MinConfidence:     0.3,
		// No adapters — pipeline will use fallback
	}

	p := NewPipeline(config)
	ctx := context.Background()
	frame := []byte{0x89, 0x50, 0x4E, 0x47}

	result, err := p.Process(ctx, frame, nil)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Latency < 0 {
		t.Error("latency should be non-negative")
	}
}

func TestPipelineBatchIntegration(t *testing.T) {
	config := PipelineConfig{
		MaxPointCloudSize: 1000,
	}

	p := NewPipeline(config)
	ctx := context.Background()

	frames := [][]byte{
		{0x89, 0x50, 0x4E, 0x47},
		{0x89, 0x50, 0x4E, 0x47},
		{0x89, 0x50, 0x4E, 0x47},
	}

	result, err := p.ProcessBatch(ctx, frames, nil)
	if err != nil {
		t.Fatalf("ProcessBatch: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
}

// ──────────────────────────────────────────────────────────────
// Adapter config tests
// ──────────────────────────────────────────────────────────────

func TestSLAMConfig(t *testing.T) {
	config := SLAMConfig{
		Backend: "orb_slam3",
		Device:  "cpu",
	}
	adapter := NewSLAMAdapter(config)

	if adapter.config.Backend != "orb_slam3" {
		t.Errorf("backend: got %v, want orb_slam3", adapter.config.Backend)
	}
	if adapter.config.Script == "" {
		t.Error("script should have default value")
	}
}

func TestReconstructConfig(t *testing.T) {
	config := ReconstructConfig{
		Backend: "instant_ngp",
		Device:  "cuda",
	}
	adapter := NewReconstructAdapter(config)

	if adapter.config.Backend != "instant_ngp" {
		t.Errorf("backend: got %v, want instant_ngp", adapter.config.Backend)
	}
}

func TestReasoningConfig(t *testing.T) {
	config := ReasoningConfig{
		Backend: "spatial_vlm",
		Device:  "cpu",
	}
	adapter := NewReasoningAdapter(config)

	if adapter.config.Backend != "spatial_vlm" {
		t.Errorf("backend: got %v, want spatial_vlm", adapter.config.Backend)
	}
}
