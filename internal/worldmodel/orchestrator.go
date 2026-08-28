// Package worldmodel provides the Living World orchestrator for the Cosca agent.
//
// The orchestrator connects all 7 perception/action pipelines into a unified flow:
//
//	CAMERA FRAME
//	  → Vision (CLIP + SAM + GroundingDINO + Depth)
//	  → Spatial (SLAM + Reconstruct + Reasoning)
//	  → World State Update
//	  → Multi-Agent Coordination
//	  → Decision (which action to take?)
//	  → Action Execution:
//	      → VFX (visual effects)
//	      → Audio (speech + sound effects)
//	      → Destruction (fracture + debris)
//	      → Simulation (physics step)
//	  → Updated World State → Unreal Engine
//
// Usage:
//
//	orch := worldmodel.NewOrchestrator(config)
//	result, err := orchestrator.ProcessFrame(ctx, cameraFrame, imuData)
//	// result contains: vision, spatial, agents, actions, updated state
package worldmodel

import (
	"context"
	"fmt"
	"time"
)

// ──────────────────────────────────────────────────────────────
// Configuration
// ──────────────────────────────────────────────────────────────

// OrchestratorConfig configures the Living World orchestrator.
type OrchestratorConfig struct {
	Vision      *VisionPipelineConfig      `json:"vision,omitempty"`
	Spatial     *SpatialPipelineConfig     `json:"spatial,omitempty"`
	VFX         *VFXPipelineConfig         `json:"vfx,omitempty"`
	Audio       *AudioPipelineConfig       `json:"audio,omitempty"`
	Destruction *DestructionPipelineConfig `json:"destruction,omitempty"`
	Simulation  *SimulationPipelineConfig  `json:"simulation,omitempty"`
	Asset       *AssetPipelineConfig       `json:"asset,omitempty"`
	MaxEntities int                        `json:"max_entities"`
	UpdateRate  float64                    `json:"update_rate"` // Hz
}

// AssetPipelineConfig configures the asset pipeline.
type AssetPipelineConfig struct {
	Provider AssetProvider `json:"-"`
}

// Pipeline config types (aliases for sub-package configs)
type VisionPipelineConfig struct {
	Sequential bool `json:"sequential"`
}

type SpatialPipelineConfig struct {
	MaxPointCloudSize int `json:"max_point_cloud_size"`
}

type VFXPipelineConfig struct {
	MaxFrames int `json:"max_frames"`
}

type AudioPipelineConfig struct {
	WhisperModel string `json:"whisper_model"`
}

type DestructionPipelineConfig struct {
	MaxFragments int `json:"max_fragments"`
}

type SimulationPipelineConfig struct {
	StepsPerFrame int `json:"steps_per_frame"`
}

// DefaultOrchestratorConfig returns sensible defaults.
func DefaultOrchestratorConfig() OrchestratorConfig {
	return OrchestratorConfig{
		Vision:      &VisionPipelineConfig{Sequential: true},
		Spatial:     &SpatialPipelineConfig{MaxPointCloudSize: 100000},
		VFX:         &VFXPipelineConfig{MaxFrames: 300},
		Audio:       &AudioPipelineConfig{WhisperModel: "base"},
		Destruction: &DestructionPipelineConfig{MaxFragments: 20},
		Simulation:  &SimulationPipelineConfig{StepsPerFrame: 1},
		MaxEntities: 100,
		UpdateRate:  10.0,
	}
}

// ──────────────────────────────────────────────────────────────
// Frame input
// ──────────────────────────────────────────────────────────────

// FrameInput represents a single frame from the camera/sensors.
type FrameInput struct {
	Camera     []byte    `json:"camera"`        // PNG/JPEG bytes
	IMU        []float64 `json:"imu,omitempty"` // IMU data (accel + gyro)
	Timestamp  time.Time `json:"timestamp"`
	AudioData  []byte    `json:"audio_data,omitempty"` // audio chunk
	SampleRate int       `json:"sample_rate,omitempty"`
}

// ──────────────────────────────────────────────────────────────
// Processing result
// ──────────────────────────────────────────────────────────────

// FrameResult contains everything the agent learned and did from one frame.
type FrameResult struct {
	// Perception
	Vision  *VisionResult    `json:"vision,omitempty"`
	Spatial *SpatialResult   `json:"spatial,omitempty"`
	Audio   *AudioPerception `json:"audio,omitempty"`

	// World state
	State WorldState `json:"state"`

	// Multi-agent
	Agents []*AgentInfo `json:"agents,omitempty"`
	Tasks  []*TaskInfo  `json:"tasks,omitempty"`

	// Actions taken
	Actions []ActionResult `json:"actions,omitempty"`

	// Performance
	Latency time.Duration `json:"latency"`
	Step    int64         `json:"step"`
}

// VisionResult wraps vision pipeline output.
type VisionResult struct {
	Entities   []WorldEntity     `json:"entities"`
	Relations  []SpatialRelation `json:"relations"`
	Confidence float64           `json:"confidence"`
}

// SpatialResult wraps spatial pipeline output.
type SpatialResult struct {
	Pose       Pose6DoF          `json:"pose"`
	PointCloud PointCloud        `json:"point_cloud"`
	Relations  []SpatialRelation `json:"relations"`
}

// AudioPerception wraps audio pipeline output.
type AudioPerception struct {
	Text   string       `json:"text"`
	Events []AudioEvent `json:"events"`
}

// AgentInfo is a snapshot of an agent's state.
type AgentInfo struct {
	ID       string `json:"id"`
	Role     string `json:"role"`
	Position Vec3   `json:"position"`
	Status   string `json:"status"`
}

// TaskInfo is a snapshot of a task.
type TaskInfo struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	AssignedTo string `json:"assigned_to"`
	Status     string `json:"status"`
	Priority   int    `json:"priority"`
}

// ActionResult describes what action was taken.
type ActionResult struct {
	Type     string            `json:"type"`   // "vfx", "audio", "destruction", "simulation"
	Action   string            `json:"action"` // specific action name
	Target   string            `json:"target"` // target entity/position
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ──────────────────────────────────────────────────────────────
// Orchestrator
// ──────────────────────────────────────────────────────────────

// Orchestrator connects all Living World pipelines.
type Orchestrator struct {
	config OrchestratorConfig
	state  WorldState
	// lister é o CACHE DE CRENÇA com selo epistêmico (I3/I4, ADR-023 item 5):
	// espelha as entidades do mundo com Source/TrustState/Revision por entidade.
	// É a visão enriquecida para o reconciler de mundo — o estado de visão
	// (state.Entities) é o snapshot; o lister é a crença (saber ≠ ver, I4).
	lister *Lister
}

// NewOrchestrator creates a Living World orchestrator.
func NewOrchestrator(config OrchestratorConfig) *Orchestrator {
	return &Orchestrator{
		config: config,
		lister: NewLister(),
		state: WorldState{
			Step: 0,
			Climate: ClimateState{
				Temperature: 22.0,
				TimeOfDay:   12.0,
				Season:      Summer,
			},
			Entities: make([]WorldEntity, 0),
		},
	}
}

// ProcessFrame processes a single camera frame through all pipelines.
func (o *Orchestrator) ProcessFrame(ctx context.Context, input FrameInput) (*FrameResult, error) {
	start := time.Now()
	result := &FrameResult{
		Step: o.state.Step,
	}

	// ─── Step 1: Vision ───
	if len(input.Camera) > 0 {
		visionResult, err := o.processVision(ctx, input.Camera)
		if err == nil {
			result.Vision = visionResult
			// Merge detected entities into world state
			o.mergeEntities(visionResult.Entities)
		}
	}

	// ─── Step 2: Spatial ───
	if len(input.Camera) > 0 {
		spatialResult, err := o.processSpatial(ctx, input.Camera, input.IMU)
		if err == nil {
			result.Spatial = spatialResult
		}
	}

	// ─── Step 3: Audio ───
	if len(input.AudioData) > 0 {
		audioResult, err := o.processAudio(ctx, input.AudioData, input.SampleRate)
		if err == nil {
			result.Audio = audioResult
		}
	}

	// ─── Step 4: World state snapshot ───
	result.State = o.copyState()

	// ─── Step 5: Multi-agent coordination ───
	agents, tasks := o.coordinateAgents()
	result.Agents = agents
	result.Tasks = tasks

	// ─── Step 6: Action execution (placeholder — actions come from decision layer) ───
	result.Actions = make([]ActionResult, 0)

	// ─── Step 7: Advance simulation ───
	o.state.Step++

	result.Latency = time.Since(start)
	return result, nil
}

// ──────────────────────────────────────────────────────────────
// Pipeline stubs (implementations delegate to sub-packages)
// ──────────────────────────────────────────────────────────────

func (o *Orchestrator) processVision(ctx context.Context, camera []byte) (*VisionResult, error) {
	// Vision pipeline: camera → entities + relations
	// Full implementation delegates to internal/worldmodel/vision
	return &VisionResult{
		Entities:   make([]WorldEntity, 0),
		Relations:  make([]SpatialRelation, 0),
		Confidence: 0.0,
	}, nil
}

func (o *Orchestrator) processSpatial(ctx context.Context, camera []byte, imu []float64) (*SpatialResult, error) {
	// Spatial pipeline: camera + IMU → pose + point cloud + relations
	// Full implementation delegates to internal/worldmodel/spatial
	return &SpatialResult{
		Pose:       Pose6DoF{},
		PointCloud: PointCloud{},
		Relations:  make([]SpatialRelation, 0),
	}, nil
}

func (o *Orchestrator) processAudio(ctx context.Context, audioData []byte, sampleRate int) (*AudioPerception, error) {
	// Audio pipeline: audio → text + events
	// Full implementation delegates to internal/worldmodel/audio
	return &AudioPerception{
		Text:   "",
		Events: make([]AudioEvent, 0),
	}, nil
}

// ──────────────────────────────────────────────────────────────
// Entity management
// ──────────────────────────────────────────────────────────────

// mergeEntities merges newly detected entities into the world state.
func (o *Orchestrator) mergeEntities(newEntities []WorldEntity) {
	for _, new := range newEntities {
		// Informa o cache de crença (selo epistêmico I3/I4). Manter a crença e a
		// visão separadas é o coração do "saber ≠ ver" (ADR-021): o lister retém
		// entidades STALE (última posição conhecida) mesmo quando o snapshot de
		// visão as poda.
		o.listerUpsert(new)

		found := false
		for i, existing := range o.state.Entities {
			if existing.ID == new.ID {
				// Update existing entity
				o.state.Entities[i].Position = new.Position
				o.state.Entities[i].Rotation = new.Rotation
				o.state.Entities[i].Label = new.Label
				o.state.Entities[i].Confidence = new.Confidence
				o.state.Entities[i].LastSeen = time.Now()
				found = true
				break
			}
		}
		if !found && len(o.state.Entities) < o.config.MaxEntities {
			o.state.Entities = append(o.state.Entities, new)
		}
	}

	// Prune old entities (not seen in 5 seconds)
	now := time.Now()
	pruned := o.state.Entities[:0]
	for _, e := range o.state.Entities {
		if now.Sub(e.LastSeen) < 5*time.Second {
			pruned = append(pruned, e)
		}
	}
	o.state.Entities = pruned
}

// listerUpsert espelha uma entidade no cache de crença com selo epistêmico.
// AsOf = LastSeen (se zero, agora) — fencing do Lister evita regressão temporal.
func (o *Orchestrator) listerUpsert(e WorldEntity) {
	asOf := e.LastSeen
	if asOf.IsZero() {
		asOf = time.Now()
	}
	o.lister.Upsert(e, ObservationSource{SensorID: "orchestrator", Authenticated: true}, TrustKnown, asOf)
}

// Lister devolve o cache de crença (stamped) do mundo. O reconciler lê daqui
// (I3/I4 por construção) — nunca de uma única request.
func (o *Orchestrator) Lister() *Lister {
	return o.lister
}

// ResyncWorld reconcilia o mundo contra um snapshot autoritativo — a entrada de
// AUTO-CURA da Fase 3 (ADR-023 item 5): entidades perdidas no snapshot (drop)
// são removidas, novas entram, atualizadas sobem revision. Reenche tanto a
// crença (lister) quanto o snapshot de visão (state.Entities). Fail-closed I2:
// a divergência é exposta em ResyncDiff, nunca "consertada" em silêncio.
func (o *Orchestrator) ResyncWorld(snapshot []WorldEntity, src ObservationSource, trust TrustState, asOf time.Time) ResyncDiff {
	diff := o.lister.Resync(snapshot, src, trust, asOf)
	o.state.Entities = make([]WorldEntity, len(snapshot))
	copy(o.state.Entities, snapshot)
	return diff
}

// copyState returns a deep copy of the current world state.
func (o *Orchestrator) copyState() WorldState {
	duplicated := WorldState{
		Step:     o.state.Step,
		Climate:  o.state.Climate,
		Entities: make([]WorldEntity, len(o.state.Entities)),
	}
	copy(duplicated.Entities, o.state.Entities)
	return duplicated
}

// ──────────────────────────────────────────────────────────────
// Multi-agent coordination
// ──────────────────────────────────────────────────────────────

// Agent represents a simplified agent for the orchestrator.
type Agent struct {
	ID           string
	Role         string
	Capabilities []string
	Position     Vec3
	Status       string
}

// Task represents a simplified task.
type Task struct {
	ID         string
	Type       string
	Target     Vec3
	Priority   int
	AssignedTo string
	Status     string
}

// coordinateAgents returns current agent and task status.
func (o *Orchestrator) coordinateAgents() ([]*AgentInfo, []*TaskInfo) {
	// Placeholder — full implementation uses internal/worldmodel/multiagent
	return nil, nil
}

// ──────────────────────────────────────────────────────────────
// State access
// ──────────────────────────────────────────────────────────────

// GetState returns the current world state.
func (o *Orchestrator) GetState() WorldState {
	return o.copyState()
}

// GetEntity returns an entity by ID.
func (o *Orchestrator) GetEntity(id string) (*WorldEntity, bool) {
	for i := range o.state.Entities {
		if o.state.Entities[i].ID == id {
			return &o.state.Entities[i], true
		}
	}
	return nil, false
}

// GetStep returns the current simulation step.
func (o *Orchestrator) GetStep() int64 {
	return o.state.Step
}

// InjectEvent injects a world event.
func (o *Orchestrator) InjectEvent(event WorldEvent) {
	// Handle event types
	switch event.Type {
	case "spawn":
		// Entity will be added on next vision frame
	case "destroy":
		// Remove entity da visão e da crença (a entidade deixou de existir).
		o.lister.Delete(event.EntityID)
		pruned := o.state.Entities[:0]
		for _, e := range o.state.Entities {
			if e.ID != event.EntityID {
				pruned = append(pruned, e)
			}
		}
		o.state.Entities = pruned
	case "weather_change":
		// Climate update handled externally
	}
}

// UpdateClimate updates the climate state.
func (o *Orchestrator) UpdateClimate(climate ClimateState) {
	o.state.Climate = climate
}

// AddEntity merges one or more world entities into the world state.
// It is the public entry point for external perception sources (e.g. the
// Unreal bridge) to register newly observed/spawned entities.
func (o *Orchestrator) AddEntity(entities ...WorldEntity) {
	if len(entities) == 0 {
		return
	}
	o.mergeEntities(entities)
}

// GenerateAsset delegates asset generation to the configured AssetProvider.
// If no provider is configured, it returns an error.
func (o *Orchestrator) GenerateAsset(ctx context.Context, req AssetRequest) (*AssetResult, error) {
	if o.config.Asset == nil || o.config.Asset.Provider == nil {
		return nil, fmt.Errorf("no asset provider configured")
	}
	result, err := o.config.Asset.Provider.GenerateAsset(ctx, req)
	if err != nil {
		return nil, err
	}

	// Register the spawned asset as a world entity so it appears in the world.
	if result.Valid && result.Path != "" {
		o.registerAssetEntity(result)
	}

	return result, nil
}

// registerAssetEntity adds a generated asset to the world state as a persistent entity.
// This is the bridge between asset creation and the living world: the asset
// becomes a real object the agent can perceive and interact with.
func (o *Orchestrator) registerAssetEntity(result *AssetResult) {
	entity := WorldEntity{
		ID:    result.ID,
		Type:  EntityObject,
		Label: fmt.Sprintf("asset_%s", result.Type),
		Metadata: map[string]string{
			"asset_path":   result.Path,
			"asset_hash":   result.Hash,
			"asset_format": result.Format,
			"tool":         result.Provenance.Tool,
		},
		Confidence: 1.0,
		Persistent: true,
		LastSeen:   time.Now(),
	}
	o.mergeEntities([]WorldEntity{entity})
}
