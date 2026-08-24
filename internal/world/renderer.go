package world

// World → Renderer Contract.
//
// The World Model produces an abstract description. A RendererAdapter
// translates it to a specific renderer (Unreal, Blender, map, simulation).
// The world model NEVER talks to a renderer directly — it only defines
// the interface a renderer must implement to interpret the world.
//
// This is what separates "how the world is described" from
// "how the world is rendered" (the professor's key insight).

// RendererCommand is a high-level instruction from the world to a renderer.
// This is the abstract contract — renderer-specific commands are the
// adapter's responsibility.
type RendererCommand struct {
	Op       string         `json:"op"`       // "spawn", "move", "destroy", "set_material", "set_weather"
	EntityID string         `json:"entity_id,omitempty"`
	AssetID  string         `json:"asset_id,omitempty"` // renderer resolves to a real asset
	Transform *Transform    `json:"transform,omitempty"`
	Params   map[string]any `json:"params,omitempty"`
}

// RendererEvent is a state change reported BACK from the renderer.
type RendererEvent struct {
	Type     string         `json:"type"`     // "spawned", "destroyed", "collision", "state_changed"
	EntityID string         `json:"entity_id,omitempty"`
	Params   map[string]any `json:"params,omitempty"`
}

// RendererAdapter is the interface a renderer implements.
// It translates abstract world concepts into renderer-specific ones.
//
// IMPORTANT: the World Model knows ONLY this interface. It never references
// AActor, UStaticMesh, UWorld, or any renderer-specific type.
type RendererAdapter interface {
	// MaterializeEntity tells the renderer to create the visual/physical
	// representation of an entity (resolving its AssetID).
	MaterializeEntity(entity Entity) (RendererEvent, error)

	// UpdateEntity sets the transform/state of an already-materialized entity.
	UpdateEntity(entityID string, transform Transform) (RendererEvent, error)

	// RemoveEntity destroys the renderer representation.
	RemoveEntity(entityID string) (RendererEvent, error)

	// ApplyWorldState applies non-entity world state (weather, time) to the renderer.
	ApplyWorldState(weather WeatherState, time SimulationTime, season Season) error

	// Name returns the renderer identifier (e.g. "unreal").
	Name() string
}
