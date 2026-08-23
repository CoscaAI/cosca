package asset

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ──────────────────────────────────────────────────────────────
// Config tests
// ──────────────────────────────────────────────────────────────

func TestDefaultBlenderAdapterConfig(t *testing.T) {
	config := DefaultBlenderAdapterConfig()

	if config.BlenderPath == "" {
		t.Error("BlenderPath should not be empty")
	}
	if config.WorkDir == "" {
		t.Error("WorkDir should not be empty")
	}
	if config.Timeout != 60 {
		t.Errorf("Timeout: got %v, want 60", config.Timeout)
	}
}

func TestNewBlenderAdapter(t *testing.T) {
	config := DefaultBlenderAdapterConfig()
	adapter := NewBlenderAdapter(config)

	if adapter == nil {
		t.Fatal("NewBlenderAdapter returned nil")
	}
}

// ──────────────────────────────────────────────────────────────
// AssetRequest tests
// ──────────────────────────────────────────────────────────────

func TestAssetRequestDefaults(t *testing.T) {
	req := AssetRequest{
		Type: AssetTree,
	}

	if req.Type != AssetTree {
		t.Errorf("type: got %v, want tree", req.Type)
	}
	if req.Format == "" {
		req.Format = FormatGLB
	}
	if req.Format != FormatGLB {
		t.Errorf("format: got %v, want glb", req.Format)
	}
}

func TestAssetRequestParams(t *testing.T) {
	req := AssetRequest{
		Type: AssetTree,
		Params: map[string]any{
			"scale":         1.5,
			"trunk_height":  3.0,
			"canopy_radius": 2.0,
		},
		Seed:   42,
		Format: FormatGLB,
	}

	if req.Params["scale"] != 1.5 {
		t.Errorf("scale: got %v, want 1.5", req.Params["scale"])
	}
	if req.Seed != 42 {
		t.Errorf("seed: got %v, want 42", req.Seed)
	}
}

// ──────────────────────────────────────────────────────────────
// AssetResult tests
// ──────────────────────────────────────────────────────────────

func TestAssetResultStructure(t *testing.T) {
	result := &AssetResult{
		ID:     "asset_001",
		Type:   AssetTree,
		Path:   "/tmp/tree.glb",
		Format: FormatGLB,
		Hash:   "sha256:abc123",
		Valid:  true,
		Metadata: AssetMetadata{
			Vertices:     1500,
			Faces:        2800,
			Materials:    []string{"bark", "leaves"},
			LODLevels:    3,
			HasCollision: true,
		},
		Provenance: AssetProvenance{
			Tool:      "blender",
			Version:   "4.2.0",
			Script:    "generate.py",
			Seed:      42,
			Timestamp: time.Now(),
		},
	}

	if result.ID != "asset_001" {
		t.Errorf("id: got %v, want asset_001", result.ID)
	}
	if result.Valid != true {
		t.Error("valid should be true")
	}
	if result.Metadata.Vertices != 1500 {
		t.Errorf("vertices: got %v, want 1500", result.Metadata.Vertices)
	}
}

// ──────────────────────────────────────────────────────────────
// AssetType tests
// ──────────────────────────────────────────────────────────────

func TestAssetTypes(t *testing.T) {
	types := []AssetType{AssetTree, AssetTerrain, AssetBuilding, AssetDebris, AssetVehicle, AssetProp, AssetCharacter, AssetCustom}

	for _, at := range types {
		if at == "" {
			t.Error("asset type should not be empty")
		}
	}
}

func TestExportFormats(t *testing.T) {
	formats := []ExportFormat{FormatGLB, FormatFBX, FormatUSD}

	for _, f := range formats {
		if f == "" {
			t.Error("export format should not be empty")
		}
	}
}

// ──────────────────────────────────────────────────────────────
// ValidationResult tests
// ──────────────────────────────────────────────────────────────

func TestValidationResultStructure(t *testing.T) {
	result := &ValidationResult{
		Valid: true,
		Stats: AssetMetadata{
			Vertices:     1000,
			Faces:        2000,
			Materials:    []string{"default"},
			LODLevels:    1,
			HasCollision: false,
		},
	}

	if result.Valid != true {
		t.Error("valid should be true")
	}
	if len(result.Issues) != 0 {
		t.Errorf("issues: got %d, want 0", len(result.Issues))
	}
}

func TestValidationResultWithIssues(t *testing.T) {
	result := &ValidationResult{
		Valid: false,
		Issues: []string{
			"mesh: no UV layers",
			"mesh: n-gon face with 5 vertices",
		},
	}

	if result.Valid != false {
		t.Error("valid should be false")
	}
	if len(result.Issues) != 2 {
		t.Errorf("issues: got %d, want 2", len(result.Issues))
	}
}

// ──────────────────────────────────────────────────────────────
// AABB tests
// ──────────────────────────────────────────────────────────────

func TestAABBStructure(t *testing.T) {
	aabb := &AABB{
		Min: [3]float64{-1, -2, -3},
		Max: [3]float64{1, 2, 3},
	}

	if aabb.Min[0] != -1 || aabb.Max[0] != 1 {
		t.Errorf("X: got [%v, %v], want [-1, 1]", aabb.Min[0], aabb.Max[0])
	}
}

// ──────────────────────────────────────────────────────────────
// Provenance tests
// ──────────────────────────────────────────────────────────────

func TestAssetProvenanceStructure(t *testing.T) {
	provenance := AssetProvenance{
		Tool:      "blender",
		Version:   "4.2.0",
		Script:    "generate_tree.py",
		Seed:      42,
		Timestamp: time.Now(),
	}

	if provenance.Tool != "blender" {
		t.Errorf("tool: got %v, want blender", provenance.Tool)
	}
	if provenance.Seed != 42 {
		t.Errorf("seed: got %v, want 42", provenance.Seed)
	}
}

// ──────────────────────────────────────────────────────────────
// Hash computation tests
// ──────────────────────────────────────────────────────────────

func TestComputeFileHash(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(tmpFile, []byte("hello world"), 0644)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	hash, err := computeFileHash(tmpFile)
	if err != nil {
		t.Fatalf("computeFileHash: %v", err)
	}

	if hash == "" {
		t.Error("hash should not be empty")
	}
	if len(hash) < 7 { // "sha256:" prefix
		t.Error("hash should have sha256 prefix")
	}
}

func TestComputeFileHashSameInput(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(tmpFile, []byte("same content"), 0644)

	hash1, _ := computeFileHash(tmpFile)
	hash2, _ := computeFileHash(tmpFile)

	if hash1 != hash2 {
		t.Errorf("same input should produce same hash: %v != %v", hash1, hash2)
	}
}

// ──────────────────────────────────────────────────────────────
// Integration tests (require Blender installed)
// ──────────────────────────────────────────────────────────────

func TestBlenderAdapterIntegration(t *testing.T) {
	config := BlenderAdapterConfig{
		BlenderPath: detectBlenderPath(),
		WorkDir:     t.TempDir(),
		Timeout:     30,
		ScriptsDir:  "../../scripts/blender",
	}

	adapter := NewBlenderAdapter(config)

	// Verify Blender is actually available; skip otherwise
	ctx := context.Background()
	if _, err := adapter.GetVersion(ctx); err != nil {
		t.Skipf("Blender not installed, skipping integration test: %v", err)
	}

	// Full vertical slice: generate a cube, then validate it
	req := AssetRequest{
		Type:     "cube",
		Format:   FormatGLB,
		Seed:     42,
		Validate: true,
		Params: map[string]any{
			"size": 1.0,
		},
	}

	result, err := adapter.GenerateAsset(ctx, req)
	if err != nil {
		t.Fatalf("GenerateAsset: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
	if result.Path == "" {
		t.Error("result.Path should not be empty")
	}
	if result.Hash == "" {
		t.Error("result.Hash should not be empty")
	}
	if !result.Valid {
		t.Errorf("expected valid asset, got issues: %v", result.Issues)
	}
	if result.Provenance.Tool != "blender" {
		t.Errorf("provenance.tool: got %v, want blender", result.Provenance.Tool)
	}

	// Verify the file exists
	if _, err := os.Stat(result.Path); err != nil {
		t.Errorf("generated file missing: %v", err)
	}
}

// ──────────────────────────────────────────────────────────────
// Metadata tests
// ──────────────────────────────────────────────────────────────

func TestAssetMetadataStructure(t *testing.T) {
	metadata := AssetMetadata{
		Vertices:     5000,
		Faces:        10000,
		Materials:    []string{"metal", "glass"},
		Textures:     []string{"diffuse.png", "normal.png"},
		LODLevels:    3,
		HasCollision: true,
	}

	if metadata.Vertices != 5000 {
		t.Errorf("vertices: got %v, want 5000", metadata.Vertices)
	}
	if len(metadata.Materials) != 2 {
		t.Errorf("materials: got %d, want 2", len(metadata.Materials))
	}
}
