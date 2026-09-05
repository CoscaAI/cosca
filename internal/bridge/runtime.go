// Package bridge — Runtime ties the Controller to the Cosca Orchestrator and
// the Blender asset pipeline. It is the integration seam for the Living World:
// frames from Unreal feed Vision, and asset generation (Blender) feeds spawns.
package bridge

import (
	"context"
	"fmt"

	"github.com/CoscaAI/cosca/internal/worldmodel"
)

// ──────────────────────────────────────────────────────────────
// Vision interface (implemented by the Vision pipeline; kept local to avoid
// a hard package dependency for the frame-handling seam).
// ──────────────────────────────────────────────────────────────

// FrameObserver processes a camera frame and returns detected entities.
// The concrete implementation is the Vision pipeline (CLIP/SAM/Depth).
type FrameObserver interface {
	// Observe returns detected world entities from a frame.
	Observe(ctx context.Context, frame []byte, width, height int) ([]worldmodel.WorldEntity, error)
}

// ──────────────────────────────────────────────────────────────
// AssetGenerator generates a 3D asset (via Blender) and returns its hash/path.
// The concrete implementation is the BlenderAdapter.
// ──────────────────────────────────────────────────────────────

// AssetGenerator generates a 3D asset and returns its result.
type AssetGenerator interface {
	// Generate creates an asset and returns path + sha256 hash.
	Generate(ctx context.Context, assetType string, seed int64) (path, hash string, err error)
}

// ──────────────────────────────────────────────────────────────
// Runtime
// ──────────────────────────────────────────────────────────────

// Runtime wires the Controller to the Orchestrator and asset pipeline.
type Runtime struct {
	controller *Controller
	observer   FrameObserver
	assets     AssetGenerator
	orchestrator *worldmodel.Orchestrator
}

// NewRuntime creates a Living World bridge runtime.
func NewRuntime(ctrl *Controller, osc *worldmodel.Orchestrator) *Runtime {
	return &Runtime{
		controller:   ctrl,
		orchestrator: osc,
	}
}

// SetFrameObserver injects the Vision pipeline as the frame observer.
func (r *Runtime) SetFrameObserver(obs FrameObserver) {
	r.observer = obs
}

// SetAssetGenerator injects the Blender adapter as the asset generator.
func (r *Runtime) SetAssetGenerator(gen AssetGenerator) {
	r.assets = gen
}

// ──────────────────────────────────────────────────────────────
// Frame handling (E6) — Unreal pushes frames, Cosca sees the world.
// ──────────────────────────────────────────────────────────────

// StartFrameLoop wires the controller's frame handler to the observer and
// merges detected entities into the Orchestrator's WorldState.
func (r *Runtime) StartFrameLoop() {
	r.controller.SetFrameHandler(func(frame FramePayload) {
		if r.observer == nil || r.orchestrator == nil {
			return
		}
		ctx := context.Background()
		entities, err := r.observer.Observe(ctx, frame.Data, frame.Width, frame.Height)
		if err != nil {
			return
		}
		// Merge into the Orchestrator world state.
		r.orchestrator.AddEntity(entities...)
	})
}

// ──────────────────────────────────────────────────────────────
// Asset feeding (E7) — Cosca generates a Blender asset, spawns in Unreal.
// ──────────────────────────────────────────────────────────────

// GenerateAndSpawn generates an asset via Blender and asks Unreal to spawn it.
// It wires asset generation into the world: Blender -> GLB/hash -> Unreal Actor.
func (r *Runtime) GenerateAndSpawn(ctx context.Context, assetType string, seed int64, spec EntitySpec) error {
	if r.assets == nil {
		return fmt.Errorf("no asset generator configured")
	}
	if r.controller == nil {
		return fmt.Errorf("no bridge controller configured")
	}

	path, hash, err := r.assets.Generate(ctx, assetType, seed)
	if err != nil {
		return fmt.Errorf("generate asset: %w", err)
	}

	// Attach the generated asset's hash and path to the spawn spec.
	spec.AssetHash = hash
	if spec.Type == "" {
		spec.Type = assetType
	}
	if spec.Config == nil {
		spec.Config = make(map[string]string)
	}
	spec.Config["source"] = "blender"
	spec.Config["glb_path"] = path

	// Spawn the actor in Unreal.
	if err := r.controller.Spawn(ctx, spec); err != nil {
		return fmt.Errorf("spawn: %w", err)
	}

	// Register in the Orchestrator so the world knows about it.
	if r.orchestrator != nil {
		r.orchestrator.AddEntity(worldmodel.WorldEntity{
			ID:     spec.ID,
			Type:   worldmodel.EntityObject,
			Label:  "asset_" + spec.Type,
			Position: worldmodel.Vec3{X: spec.Pos[0], Y: spec.Pos[1], Z: spec.Pos[2]},
			Metadata: map[string]string{
				"asset_hash": hash,
				"asset_path": path,
			},
			Persistent: true,
		})
	}
	return nil
}
