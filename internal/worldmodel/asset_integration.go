package worldmodel

import "context"

// AssetRequest requests generation of a 3D asset.
type AssetRequest struct {
	Type     string         `json:"type"`     // "tree", "terrain", "building", "cube"
	Params   map[string]any `json:"params,omitempty"`
	Seed     int64          `json:"seed,omitempty"`
	Format   string         `json:"format,omitempty"`   // "glb", "fbx", "usd"
	Output   string         `json:"output,omitempty"`   // output directory
	Validate bool           `json:"validate,omitempty"` // validate after generation
}

// AssetResult is the result of asset generation.
type AssetResult struct {
	ID         string       `json:"id"`
	Type       string       `json:"type"`
	Path       string       `json:"path"`   // output file path
	Format     string       `json:"format"`
	Hash       string       `json:"hash"`   // sha256 hash
	Valid      bool         `json:"valid"`
	Issues     []string     `json:"issues,omitempty"`
	Provenance AssetProvenance `json:"provenance"`
}

// AssetProvenance tracks the origin of an asset.
type AssetProvenance struct {
	Tool      string `json:"tool"`      // "blender"
	Version   string `json:"version"`
	Script    string `json:"script"`
	Seed      int64  `json:"seed"`
}

// AssetProvider is the interface for 3D asset generation.
// Implemented by asset.BlenderAdapter (via subprocess).
type AssetProvider interface {
	// GenerateAsset creates an asset from parameters.
	GenerateAsset(ctx context.Context, req AssetRequest) (*AssetResult, error)

	// ValidateAsset checks if an asset file is valid.
	ValidateAsset(ctx context.Context, path string) (*AssetValidationResult, error)
}

// AssetValidationResult holds validation results for an asset file.
type AssetValidationResult struct {
	Valid  bool          `json:"valid"`
	Issues []string      `json:"issues,omitempty"`
	Stats  AssetMetadata `json:"stats"`
}

// AssetMetadata holds asset statistics.
type AssetMetadata struct {
	Vertices     int      `json:"vertices"`
	Faces        int      `json:"faces"`
	Materials    []string `json:"materials,omitempty"`
	Textures     []string `json:"textures,omitempty"`
	BoundingBox  *AABB    `json:"bounding_box,omitempty"`
	LODLevels    int      `json:"lod_levels"`
	HasCollision bool     `json:"has_collision"`
}
