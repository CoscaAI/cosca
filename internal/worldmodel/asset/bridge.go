package asset

import (
	"context"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// BlenderAssetProvider bridges asset.BlenderAdapter to the worldmodel.AssetProvider
// interface. Since asset and worldmodel define their own AssetRequest types,
// this adapter performs the type conversion so the BlenderAdapter can be
// injected into the Orchestrator's asset pipeline without coupling worldmodel
// to the asset package's concrete types.
type BlenderAssetProvider struct {
	adapter *BlenderAdapter
}

// NewBlenderAssetProvider wraps a BlenderAdapter as a worldmodel.AssetProvider.
func NewBlenderAssetProvider(adapter *BlenderAdapter) *BlenderAssetProvider {
	return &BlenderAssetProvider{adapter: adapter}
}

// GenerateAsset implements worldmodel.AssetProvider.
func (b *BlenderAssetProvider) GenerateAsset(ctx context.Context, req worldmodel.AssetRequest) (*worldmodel.AssetResult, error) {
	// Convert worldmodel.AssetRequest to asset.AssetRequest
	assetReq := AssetRequest{
		Type:     AssetType(req.Type),
		Params:   req.Params,
		Seed:     req.Seed,
		Format:   ExportFormat(req.Format),
		Output:   req.Output,
		Validate: req.Validate,
	}

	result, err := b.adapter.GenerateAsset(ctx, assetReq)
	if err != nil {
		return nil, err
	}

	// Convert asset.AssetResult to worldmodel.AssetResult
	return &worldmodel.AssetResult{
		ID:     result.ID,
		Type:   string(result.Type),
		Path:   result.Path,
		Format: string(result.Format),
		Hash:   result.Hash,
		Valid:  result.Valid,
		Issues: result.Issues,
		Provenance: worldmodel.AssetProvenance{
			Tool:    result.Provenance.Tool,
			Version: result.Provenance.Version,
			Script:  result.Provenance.Script,
			Seed:    result.Provenance.Seed,
		},
	}, nil
}

// ValidateAsset implements worldmodel.AssetProvider.
func (b *BlenderAssetProvider) ValidateAsset(ctx context.Context, path string) (*worldmodel.AssetValidationResult, error) {
	result, err := b.adapter.ValidateAsset(ctx, path)
	if err != nil {
		return nil, err
	}

	var bound *worldmodel.AABB
	if result.Stats.BoundingBox != nil {
		bound = &worldmodel.AABB{
			Min: worldmodel.Vec3{
				X: result.Stats.BoundingBox.Min[0],
				Y: result.Stats.BoundingBox.Min[1],
				Z: result.Stats.BoundingBox.Min[2],
			},
			Max: worldmodel.Vec3{
				X: result.Stats.BoundingBox.Max[0],
				Y: result.Stats.BoundingBox.Max[1],
				Z: result.Stats.BoundingBox.Max[2],
			},
		}
	}

	return &worldmodel.AssetValidationResult{
		Valid: result.Valid,
		Issues: result.Issues,
		Stats: worldmodel.AssetMetadata{
			Vertices:     result.Stats.Vertices,
			Faces:        result.Stats.Faces,
			Materials:    result.Stats.Materials,
			Textures:     result.Stats.Textures,
			BoundingBox:  bound,
			LODLevels:    result.Stats.LODLevels,
			HasCollision: result.Stats.HasCollision,
		},
	}, nil
}

