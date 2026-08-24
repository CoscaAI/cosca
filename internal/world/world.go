// Package world defines the canonical abstract representation of a world.
//
// This is the COSCA WORLD MODEL. It is PURE Go — stdlib only — and has ZERO
// knowledge of Unreal, AActor, UStaticMesh, Google Maps, or any renderer.
//
// Design principles:
//   - DETERMINISTIC: same seed + same data + same version = same world.
//   - CANONICAL: exactly one representation of each concept.
//   - SELF-DESCRIBING: every entity carries provenance + version.
//   - INDEPENDENT: the world model is the source of truth; renderers adapt it.
//
// The world is built bottom-up: spatial primitives → entities → graph → world.
//
// FASE A — Cosca World Model (per professor's blueprint).
package world

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"time"
)

// ──────────────────────────────────────────────────────────────
// Schema versioning (regra do professor: arquivo de schema sempre versionado)
// ──────────────────────────────────────────────────────────────

// SchemaVersion is the version of the World Model schema.
// Bump on every breaking change to the serialized format.
const SchemaVersion = 1

// ──────────────────────────────────────────────────────────────
// Spatial primitives
// ──────────────────────────────────────────────────────────────

// Vec3 is a 3D point/vector in WORLD coordinate space (meters).
type Vec3 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// Add returns v + w.
func (v Vec3) Add(w Vec3) Vec3 { return Vec3{v.X + w.X, v.Y + w.Y, v.Z + w.Z} }

// Sub returns v - w.
func (v Vec3) Sub(w Vec3) Vec3 { return Vec3{v.X - w.X, v.Y - w.Y, v.Z - w.Z} }

// Scale returns v * s.
func (v Vec3) Scale(s float64) Vec3 { return Vec3{v.X * s, v.Y * s, v.Z * s} }

// Length returns the Euclidean norm |v|.
func (v Vec3) Length() float64 { return math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z) }

// DistanceTo returns the distance between v and w.
func (v Vec3) DistanceTo(w Vec3) float64 { return v.Sub(w).Length() }

// Quat is a rotation quaternion (unit).
type Quat struct {
	W float64 `json:"w"`
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// IdentityQuat returns the identity quaternion (no rotation).
func IdentityQuat() Quat { return Quat{W: 1} }

// Transform is a position + rotation + scale in world space.
type Transform struct {
	Position Vec3 `json:"position"`
	Rotation Quat `json:"rotation"`
	Scale    Vec3 `json:"scale"`
}

// BoundingBox is an axis-aligned bounding box (min/max corners).
type BoundingBox struct {
	Min Vec3 `json:"min"`
	Max Vec3 `json:"max"`
}

// Center returns the box center.
func (b BoundingBox) Center() Vec3 {
	return Vec3{(b.Min.X + b.Max.X) / 2, (b.Min.Y + b.Max.Y) / 2, (b.Min.Z + b.Max.Z) / 2}
}

// Size returns the box dimensions.
func (b BoundingBox) Size() Vec3 { return b.Max.Sub(b.Min) }

// ──────────────────────────────────────────────────────────────
// Coordinate systems (regra do professor: camada explícita, nunca espalhar)
// ──────────────────────────────────────────────────────────────

// GeoCoordinates is a WGS84 geographic position (lat/lon/alt).
type GeoCoordinates struct {
	Latitude  float64 `json:"latitude"`  // degrees, -90..90
	Longitude float64 `json:"longitude"` // degrees, -180..180
	Altitude  float64 `json:"altitude"`  // meters above reference
}

// CoordinateSystem defines the world coordinate frame.
// world coordinates are LOCAL meters; geo is WGS84 anchored at Origin.
type CoordinateSystem struct {
	Origin GeoCoordinates `json:"origin"`   // geo anchor at world (0,0)
	Units  string         `json:"units"`    // "meters"
	Scale  float64        `json:"scale"`    // meters per world unit (1.0)
}

// GeoToWorld converts a geodetic position to local world meters
// relative to Origin. Uses a local tangent-plane (equirectangular) projection.
// This is a SINGLE conversion point — never reimplement elsewhere.
func (cs CoordinateSystem) GeoToWorld(geo GeoCoordinates) Vec3 {
	// Equirectangular approximation around the origin. Sufficient for local
	// regions (city scale). Document precision limits in the spec.
	const earthRadiusM = 6378137.0
	dx := (geo.Longitude - cs.Origin.Longitude) * math.Pi / 180.0 *
		earthRadiusM * math.Cos(cs.Origin.Latitude*math.Pi/180.0)
	dy := (geo.Latitude - cs.Origin.Latitude) * math.Pi / 180.0 * earthRadiusM
	return Vec3{X: dx * cs.Scale, Y: dy * cs.Scale, Z: (geo.Altitude - cs.Origin.Altitude) * cs.Scale}
}

// WorldToGeo is the inverse of GeoToWorld.
func (cs CoordinateSystem) WorldToGeo(v Vec3) GeoCoordinates {
	const earthRadiusM = 6378137.0
	vx := v.X / cs.Scale
	vy := v.Y / cs.Scale
	vz := v.Z / cs.Scale
	lon := cs.Origin.Longitude + vx/(earthRadiusM*math.Cos(cs.Origin.Latitude*math.Pi/180.0)) * 180.0 / math.Pi
	lat := cs.Origin.Latitude + vy/earthRadiusM * 180.0 / math.Pi
	return GeoCoordinates{Latitude: lat, Longitude: lon, Altitude: cs.Origin.Altitude + vz}
}

// ──────────────────────────────────────────────────────────────
// Provenance (regra do professor: toda entidade responde de onde veio)
// ──────────────────────────────────────────────────────────────

// KnowledgeClass is the epistemic status of an entity's source.
type KnowledgeClass string

const (
	ClassFACT       KnowledgeClass = "fact"        // directly observed/measured
	ClassMEASURED   KnowledgeClass = "measured"    // measured with uncertainty
	ClassINFERRED   KnowledgeClass = "inferred"    // derived from other data
	ClassGENERATED  KnowledgeClass = "generated"   // procedurally generated
	ClassHYPOTHESIS KnowledgeClass = "hypothesis"  // tentative
	ClassUNKNOWN    KnowledgeClass = "unknown"
)

// Source identifies where an entity's data came from.
type Source struct {
	Dataset   string `json:"dataset,omitempty"` // "osm", "dem", "procedural", "asset"
	Version   string `json:"version,omitempty"` // source dataset version
	Timestamp string `json:"timestamp,omitempty"`
	Hash      string `json:"hash,omitempty"` // content hash of raw source
}

// Generation identifies how an entity was procedurally created.
type Generation struct {
	Generator  string         `json:"generator"`             // "vegetation.generate_tree"
	Version    string         `json:"version"`               // generator version
	Seed       int64          `json:"seed"`                  // deterministic seed
	InputHash  string         `json:"input_hash,omitempty"`  // hash of generation inputs
	Parameters map[string]any `json:"parameters,omitempty"`
}

// Provenance is the complete origin trace of an entity.
type Provenance struct {
	Class      KnowledgeClass `json:"class"`      // epistemic status
	Source     Source         `json:"source"`     // data source
	Generation *Generation    `json:"generation,omitempty"` // if generated
	Accuracy   float64        `json:"accuracy"`   // 0..1 confidence
	CreatedAt  string         `json:"created_at"` // ISO-8601
}

// ──────────────────────────────────────────────────────────────
// Entity types (canonical namespace)
// ──────────────────────────────────────────────────────────────

// EntityClass is a broad category.
type EntityClass string

const (
	ClassTerrain   EntityClass = "terrain"
	ClassVegetation EntityClass = "vegetation"
	ClassStructure  EntityClass = "structure"
	ClassWater      EntityClass = "water"
	ClassRoad       EntityClass = "road"
	ClassVehicle    EntityClass = "vehicle"
	ClassEntity     EntityClass = "entity"
	ClassActor      EntityClass = "actor"
)

// EntityType is a specific type within a class.
type EntityType string

// ──────────────────────────────────────────────────────────────
// Entity (the canonical world entity)
// ──────────────────────────────────────────────────────────────

// EntityState describes the dynamic state of an entity.
type EntityState struct {
	Alive           bool   `json:"alive"`
	Health          float64 `json:"health"`          // 0..1
	Condition       string `json:"condition"`        // "intact", "damaged", "destroyed"
	Age             string `json:"age,omitempty"`    // "seedling", "mature", "ancient"
	Temporal        string `json:"temporal,omitempty"` // snapshot time
}

// Geometry is an abstract shape reference attached to an entity.
// It is SEPARATE from the entity's semantics (regra do professor, item 5):
// an entity is what it IS; geometry is its shape representation.
type Geometry struct {
	Kind     string   `json:"kind"`               // "point", "polygon", "linestring", "polyline", "volume", "bbox"
	Points   []Vec3   `json:"points,omitempty"`   // polyline/polygon vertices
	BoundingBox *BoundingBox `json:"bbox,omitempty"` // axis-aligned bounds
}

// Entity is a single canonical object in the world.
type Entity struct {
	ID          string         `json:"id"`          // stable entity ID (e.g. "tree_000184")
	Class       EntityClass    `json:"class"`       // broad category
	Type        EntityType     `json:"type"`        // specific type (e.g. "tree.oak")
	Transform   Transform      `json:"transform"`
	Geometry    *Geometry      `json:"geometry,omitempty"` // shape (separate from semantics)
	BoundingBox *BoundingBox   `json:"bbox,omitempty"`
	Parent      string         `json:"parent,omitempty"`  // parent entity ID
	Children    []string       `json:"children,omitempty"` // child entity IDs
	Properties  map[string]any `json:"properties"`        // semantic properties
	State       EntityState    `json:"state"`
	Provenance  Provenance     `json:"provenance"`
	Version     int            `json:"version"`   // entity schema version
}

// Hash returns a deterministic content hash of the entity's immutable parts.
// Used for deduplication and provenance verification.
func (e Entity) Hash() string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%.4f|%.4f|%.4f|%s|%v", e.ID, e.Type,
		e.Transform.Position.X, e.Transform.Position.Y, e.Transform.Position.Z,
		e.Provenance.Class, e.Provenance.Generation)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// ──────────────────────────────────────────────────────────────
// Relations (world graph edges)
// ──────────────────────────────────────────────────────────────

// RelationType is a semantic relation between two entities.
type RelationType string

const (
	RelContains   RelationType = "contains"    // parent contains child
	RelLocatedAt  RelationType = "located_at"  // located at/on
	RelConnected  RelationType = "connected_to" // connected (roads)
	RelAdjacent   RelationType = "adjacent_to"
	RelBelongsTo  RelationType = "belongs_to"
	RelDerived    RelationType = "derived_from"
)

// Relation is an edge in the world graph: Subject →(Relation)→ Object.
type Relation struct {
	Subject     string       `json:"subject"`
	Object      string       `json:"object"`
	Relation    RelationType `json:"relation"`
	Confidence  float64      `json:"confidence"`
}

// ──────────────────────────────────────────────────────────────
// Temporal model
// ──────────────────────────────────────────────────────────────

// WeatherState describes current environmental conditions.
type WeatherState struct {
	Type      string  `json:"type"`      // "clear", "rain", "snow", "fog", "storm", "overcast"
	Intensity float64 `json:"intensity"` // 0..1
	Wind      float64 `json:"wind"`      // m/s
	Temp      float64 `json:"temperature"` // celsius
}

// Season is a world season.
type Season string

const (
	Spring Season = "spring"
	Summer Season = "summer"
	Autumn Season = "autumn"
	Winter Season = "winter"
)

// SimulationTime is deterministic world clock time.
type SimulationTime struct {
	Day       int     `json:"day"`
	Hour      float64 `json:"hour"` // 0..24
	TimeScale float64 `json:"time_scale"` // simulated hours per real hour
	Paused    bool    `json:"paused"`
}

// ──────────────────────────────────────────────────────────────
// World (top-level container)
// ──────────────────────────────────────────────────────────────

// World is the canonical in-memory representation of a world.
type World struct {
	WorldID       string         `json:"world_id"`
	SchemaVersion int            `json:"schema_version"`
	CoordSystem   CoordinateSystem `json:"coord_system"`
	Entities      []Entity       `json:"entities"`
	Relations     []Relation     `json:"relations"`
	Weather       WeatherState   `json:"weather"`
	Season        Season         `json:"season"`
	Time          SimulationTime `json:"time"`
	CreatedAt     string         `json:"created_at"`
	Version       int            `json:"version"`
}

// AddEntity appends an entity to the world.
func (w *World) AddEntity(e Entity) {
	w.Entities = append(w.Entities, e)
}

// GetEntity returns an entity by ID.
func (w *World) GetEntity(id string) *Entity {
	for i := range w.Entities {
		if w.Entities[i].ID == id {
			return &w.Entities[i]
		}
	}
	return nil
}

// AddRelation appends a relation edge.
func (w *World) AddRelation(r Relation) {
	w.Relations = append(w.Relations, r)
}

// NewWorld creates an empty world with the given coordinate system.
func NewWorld(worldID string, cs CoordinateSystem) *World {
	return &World{
		WorldID:       worldID,
		SchemaVersion: SchemaVersion,
		CoordSystem:   cs,
		Entities:      []Entity{},
		Relations:     []Relation{},
		Weather:       WeatherState{Type: "clear", Intensity: 0, Temp: 20},
		Season:        Summer,
		Time:          SimulationTime{Day: 1, Hour: 12, TimeScale: 1},
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		Version:       SchemaVersion,
	}
}
