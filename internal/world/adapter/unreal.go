// Package adapter implements the World → Renderer contract for Unreal.
//
// The UnrealAdapter translates abstract world.Entity/renderer concepts into
// bridge commands (WebSocket JSON) that the CoscaRuntime plugin understands.
//
// IMPORTANT: This is the ONLY place in the Go codebase that knows about
// Unreal-specific mapping. The world model stays pure; this adapter translates.
//
// Cosca sends abstract commands (spawn/move/destroy/weather/time); the
// Cosca-Unreal plugin translates them to AActor/UStaticMesh/UWorld.
package adapter

import (
	"context"

	"github.com/CoscaAI/cosca/internal/bridge"
	"github.com/CoscaAI/cosca/internal/world"
)

// UnrealAdapter implements world.RendererAdapter by translating to bridge
// commands over a WebSocket connection to the Cosca-Unreal plugin.
type UnrealAdapter struct {
	ctrl *bridge.Controller
	ctx  context.Context
}

// NewUnreal creates an UnrealAdapter around a bridge controller.
func NewUnreal(ctrl *bridge.Controller, ctx context.Context) *UnrealAdapter {
	return &UnrealAdapter{ctrl: ctrl, ctx: ctx}
}

// Name returns "unreal" (renderer identifier).
func (a *UnrealAdapter) Name() string { return "unreal" }

// MaterializeEntity spawns an entity in Unreal. Resolves the entity's
// AssetID (semantic) via the bridge's import_mesh command, which the plugin
// translates into a StaticMeshActor.
func (a *UnrealAdapter) MaterializeEntity(e world.Entity) (world.RendererEvent, error) {
	assetID := entityAssetID(e)

	// Determine the bridge entity-type string from the semantic class.
	bridgeType := classToBridgeType(e.Class)

	payload := bridge.ImportMeshPayload{
		EntityID: e.ID,
		MeshPath: assetID, // AssetID — NOT a /Game/ path (regra de ouro)
		Type:     bridgeType,
		Position: worldVecToArray(e.Transform.Position),
		Scale:    worldVecToArray(e.Transform.Scale),
	}

	if err := a.ctrl.ImportMesh(a.ctx, payload); err != nil {
		return world.RendererEvent{}, err
	}

	return world.RendererEvent{Type: "spawned", EntityID: e.ID}, nil
}

// UpdateEntity updates the transform of a spawned entity.
func (a *UnrealAdapter) UpdateEntity(entityID string, t world.Transform) (world.RendererEvent, error) {
	target := worldVecToArray(t.Position)
	if err := a.ctrl.Move(a.ctx, entityID, target); err != nil {
		return world.RendererEvent{}, err
	}
	return world.RendererEvent{Type: "state_changed", EntityID: entityID}, nil
}

// RemoveEntity destroys the entity in Unreal.
func (a *UnrealAdapter) RemoveEntity(entityID string) (world.RendererEvent, error) {
	if err := a.ctrl.Destroy(a.ctx, entityID); err != nil {
		return world.RendererEvent{}, err
	}
	return world.RendererEvent{Type: "destroyed", EntityID: entityID}, nil
}

// ApplyWorldState pushes weather + time-of-day to Unreal. This drives the
// day/night cycle and weather simulation implemented in the plugin.
func (a *UnrealAdapter) ApplyWorldState(weather world.WeatherState, t world.SimulationTime, season world.Season) error {
	// Weather → bridge weather command.
	if err := a.ctrl.SetWeather(a.ctx, weather.Type, weather.Intensity); err != nil {
		return err
	}
	// Time-of-day → bridge time command.
	if err := a.ctrl.SetTimeOfDay(a.ctx, t.Hour); err != nil {
		return err
	}
	return nil
}

// ──────────────────────────────────────────────────────────────
// Type translation helpers (semantic → bridge)
// ──────────────────────────────────────────────────────────────

// entityAssetID returns the semantic AssetID to materialize for an entity.
// The semantic class/type maps to an AssetID; the renderer resolves it to a
// real mesh via the Asset Registry. This is semantic (never a /Game/ path).
//
// Each entity type maps to a distinct abstract asset so the renderer can
// choose the best visual representation. This honors the professor's rule:
// "the real data provides WHAT exists; the visual system decides HOW to
// represent it."
func entityAssetID(e world.Entity) string {
	// Order by most specific type first, then class fallback.
	switch e.Class {
	case world.ClassVegetation:
		return "vegetation.tree"
	case world.ClassRoad:
		// Avenues/primary roads vs residential vs service.
		switch e.Type {
		case "road.primary", "road.trunk", "road.motorway", "road.secondary":
			return "road.avenue"
		case "road.service", "road.footway", "road.path":
			return "road.path"
		default:
			return "road.residential"
		}
	case world.ClassStructure:
		// Buildings: apartments vs house vs commercial.
		if e.Properties != nil {
			if bt, ok := e.Properties["building_type"].(string); ok {
				return "building." + bt
			}
		}
		return "building.house"
	case world.ClassWater:
		return "water.body"
	case world.ClassTerrain:
		if e.Type == "park" {
			return "terrain.park"
		}
		if e.Type == "square.public" {
			return "terrain.plaza"
		}
		if e.Type == "rock.boulder" {
			return "rock.boulder"
		}
		return "terrain.region"
	case world.ClassVehicle:
		return "vehicle.car"
	default:
		return "entity.object"
	}
}

// classToBridgeType maps a semantic entity class to the bridge type string.
func classToBridgeType(c world.EntityClass) string {
	switch c {
	case world.ClassVegetation:
		return "tree"
	case world.ClassRoad:
		return "road"
	case world.ClassStructure:
		return "building"
	case world.ClassWater:
		return "water"
	case world.ClassTerrain:
		return "terrain"
	case world.ClassVehicle:
		return "vehicle"
	default:
		return "object"
	}
}

// MaterializeAssetIDForTest exposes entityAssetID for testing (exported helper).
func MaterializeAssetIDForTest(e world.Entity) string {
	return entityAssetID(e)
}

// worldVecToArray converts a world Vec3 to a bridge [3]float64 position/scale.
func worldVecToArray(v world.Vec3) [3]float64 {
	return [3]float64{v.X, v.Y, v.Z}
}
