// Package city generates deterministic synthetic city worlds.
//
// This implements the professor's "pulo do gato": model a full city
// HIERARCHY without any image, to prove the World Model can represent
// urban structure semantically before ingesting real data (OSM/GIS).
//
// Hierarchy:
//   City → District → Streets → Buildings/Trees/Vehicles → Square → River
//
// Deterministic: same seed ⇒ same city (verified by fingerprint).
package city

import (
	"fmt"
	"math/rand"

	"github.com/CoscaAI/cosca/internal/world"
)

// CityConfig configures the synthetic city generator.
type CityConfig struct {
	Seed         int64   `json:"seed"`
	Districts    int     `json:"districts"`
	StreetsPerD  int     `json:"streets_per_district"`
	BuildingsPer int     `json:"buildings_per_street"`
	TreesPerSt   int     `json:"trees_per_street"`
	VehiclesPer  int     `json:"vehicles_per_street"`
	HasRiver     bool    `json:"has_river"`
	Size         float64 `json:"size"` // city extent in meters
}

// DefaultConfig returns reasonable default sizes.
func DefaultConfig(seed int64) CityConfig {
	return CityConfig{
		Seed:         seed,
		Districts:    4,
		StreetsPerD:  3,
		BuildingsPer: 5,
		TreesPerSt:   4,
		VehiclesPer:  2,
		HasRiver:     true,
		Size:         1000,
	}
}

// CityResult is the generated city and its world.
type CityResult struct {
	World     *world.World
	CityID    string
	DistrictIDs []string
	StreetIDs   []string
}

// Generate creates a deterministic synthetic city wrapped in a World.
func Generate(cfg CityConfig) (*CityResult, error) {
	src := rand.NewSource(cfg.Seed)
	rng := rand.New(src)

	// Coordinate system anchored at a plausible city origin (WGS84).
	cs := world.CoordinateSystem{
		Origin: world.GeoCoordinates{Latitude: -27.5, Longitude: -48.5, Altitude: 0},
		Units:  "meters",
		Scale:  1.0,
	}

	cityID := fmt.Sprintf("city_%06d", cfg.Seed)
	w := world.NewWorld(cityID, cs)

	// Provenance for generated content (regra de ouro: sempre gen metadata).
	prov := world.Provenance{
		Class:  world.ClassGENERATED,
		Source: world.Source{Dataset: "procedural", Version: "1.0"},
		Generation: &world.Generation{
			Generator: "city.generate",
			Version:   "1.0",
			Seed:      cfg.Seed,
		},
		Accuracy: 1.0,
	}

	cityEntity := world.Entity{
		ID:     cityID,
		Class:  world.ClassStructure,
		Type:   "city",
		Transform: world.Transform{Position: world.Vec3{}},
		Properties: map[string]any{
			"name":     "Synthetic City",
			"districts": cfg.Districts,
		},
		State:      world.EntityState{Alive: true, Health: 1, Condition: "intact"},
		Provenance: prov,
		Version:    world.SchemaVersion,
	}
	w.AddEntity(cityEntity)

	// ── River (runs along one edge of the city) ──
	if cfg.HasRiver {
		river := buildRiver(rng, cfg, prov)
		w.AddEntity(river)
		w.AddRelation(world.Relation{Subject: river.ID, Object: cityID, Relation: world.RelLocatedAt, Confidence: 1})
	}

	// ── Districts ──
	districtIDs := make([]string, 0)
	streetIDs := make([]string, 0)
	districtSpacing := cfg.Size / float64(cfg.Districts)

	for d := 0; d < cfg.Districts; d++ {
		did := fmt.Sprintf("%s_district_%02d", cityID, d+1)
		dx := (float64(d) + 0.5) * districtSpacing

		district := world.Entity{
			ID:    did,
			Class: world.ClassTerrain,
			Type:  "district",
			Transform: world.Transform{Position: world.Vec3{X: dx, Y: cfg.Size / 2}},
			Parent: cityID,
			Properties: map[string]any{
				"index": d + 1,
			},
			State:      world.EntityState{Alive: true, Health: 1, Condition: "intact"},
			Provenance: prov,
			Version:    world.SchemaVersion,
		}
		w.AddEntity(district)
		districtIDs = append(districtIDs, did)
		w.AddRelation(world.Relation{Subject: did, Object: cityID, Relation: world.RelBelongsTo, Confidence: 1})

		// ── Streets within this district ──
		for s := 0; s < cfg.StreetsPerD; s++ {
			sid := buildStreet(d+1, s+1, dx, rng, cfg, prov)
			w.AddEntity(sid)
			streetIDs = append(streetIDs, sid.ID)
			w.AddRelation(world.Relation{Subject: sid.ID, Object: did, Relation: world.RelBelongsTo, Confidence: 1})
			w.AddRelation(world.Relation{Subject: sid.ID, Object: cityID, Relation: world.RelLocatedAt, Confidence: 1})

			// ── Buildings / Trees / Vehicles on this street ──
			populateStreet(w, sid, rng, cfg, prov)
		}
	}

	// ── Squares: one per district ──
	for d, did := range districtIDs {
		square := buildSquare(d+1, float64(d+1), rng, cfg, prov)
		w.AddEntity(square)
		w.AddRelation(world.Relation{Subject: square.ID, Object: did, Relation: world.RelLocatedAt, Confidence: 1})
	}

	return &CityResult{World: w, CityID: cityID, DistrictIDs: districtIDs, StreetIDs: streetIDs}, nil
}

// buildRiver creates a river entity along the city's edge.
func buildRiver(rng *rand.Rand, cfg CityConfig, prov world.Provenance) world.Entity {
	width := cfg.Size * 0.03
	// River centerline runs north-south along the west edge.
	line := world.Line{Points: []world.Vec3{
		{X: 0, Y: 0, Z: 0},
		{X: 0, Y: cfg.Size, Z: 0},
	}}
	return world.Entity{
		ID:    "river_001",
		Class: world.ClassWater,
		Type:  "water.river",
		Transform: world.Transform{Position: line.Points[0]},
		BoundingBox: lineBBox(line),
		Properties: map[string]any{
			"width":    width,
			"length":   line.Length(),
			"flow":     "south-to-north",
			"name":     "Synthetic River",
		},
		State:      world.EntityState{Alive: true, Health: 1, Condition: "intact"},
		Provenance: prov,
		Version:    world.SchemaVersion,
	}
}

// buildStreet creates a road entity with a centerline.
func buildStreet(district, idx int, dx float64, rng *rand.Rand, cfg CityConfig, prov world.Provenance) world.Entity {
	yStart := float64(idx) * (cfg.Size / float64(cfg.StreetsPerD+1))
	// Roads run east-west (perpendicular to river).
	line := world.Line{Points: []world.Vec3{
		{X: dx - cfg.Size/(2*float64(cfg.Districts)) + 10, Y: yStart},
		{X: dx + cfg.Size/(2*float64(cfg.Districts)) - 10, Y: yStart},
	}}
	width := 8.0 // meters
	sid := fmt.Sprintf("city_%06d_d%02d_s%02d", cfg.Seed, district, idx)
	e := world.Entity{
		ID:    sid,
		Class: world.ClassRoad,
		Type:  "road.way",
		Transform: world.Transform{Position: line.Points[0]},
		BoundingBox: lineBBox(line),
		Properties: map[string]any{
			"length":  line.Length(),
			"width":   width,
			"surface": "asphalt",
			"lanes":   2,
		},
		State:      world.EntityState{Alive: true, Health: 1, Condition: "intact"},
		Provenance: prov,
		Version:    world.SchemaVersion,
	}
	return e
}

// populateStreet adds buildings, trees, vehicles along a street.
func populateStreet(w *world.World, street world.Entity, rng *rand.Rand, cfg CityConfig, prov world.Provenance) {
	// Buildings along the street (offset from centerline).
	for b := 0; b < cfg.BuildingsPer; b++ {
		t := (float64(b) + 1) / float64(cfg.BuildingsPer+1)
		pos := world.Vec3{
			X: street.Transform.Position.X + t*street.Properties["length"].(float64),
			Y: street.Transform.Position.Y + 15, // offset from road center
		}
		building := world.BuildingEntity(
			fmt.Sprintf("%s_b%02d", street.ID, b+1), pos,
			world.Vec3{X: 12 + rng.Float64()*8, Y: 10 + rng.Float64()*5, Z: 6 + rng.Float64()*20},
			prov,
		)
		building.Parent = street.ID
		w.AddEntity(building)
		w.AddRelation(world.Relation{Subject: building.ID, Object: street.ID, Relation: world.RelLocatedAt, Confidence: 1})
	}

	// Trees along the sidewalk.
	for t := 0; t < cfg.TreesPerSt; t++ {
		pos := world.Vec3{
			X: street.Transform.Position.X + (float64(t)+1)/(float64(cfg.TreesPerSt+1))*street.Properties["length"].(float64),
			Y: street.Transform.Position.Y - 5, // other side
		}
		tree := world.TreeEntity(
			fmt.Sprintf("%s_tr%02d", street.ID, t+1),
			[]string{"oak", "pine", "birch"}[t%3],
			pos,
			[]string{"seedling", "mature", "ancient"}[t%3],
			prov,
		)
		tree.Parent = street.ID
		w.AddEntity(tree)
		w.AddRelation(world.Relation{Subject: tree.ID, Object: street.ID, Relation: world.RelLocatedAt, Confidence: 1})
	}

	// Vehicles parked on the street.
	for v := 0; v < cfg.VehiclesPer; v++ {
		pos := world.Vec3{
			X: street.Transform.Position.X + (float64(v)+1)/(float64(cfg.VehiclesPer+1))*street.Properties["length"].(float64),
			Y: street.Transform.Position.Y,
		}
		vehicle := world.Entity{
			ID:    fmt.Sprintf("%s_ve%02d", street.ID, v+1),
			Class: world.ClassVehicle,
			Type:  "vehicle.car",
			Transform: world.Transform{Position: pos, Scale: world.Vec3{X: 1, Y: 1, Z: 1}},
			Properties: map[string]any{
				"kind": []string{"sedan", "suv", "hatchback"}[v%3],
			},
			State:      world.EntityState{Alive: true, Health: 1, Condition: "intact"},
			Provenance: prov,
			Version:    world.SchemaVersion,
		}
		vehicle.Parent = street.ID
		w.AddEntity(vehicle)
		w.AddRelation(world.Relation{Subject: vehicle.ID, Object: street.ID, Relation: world.RelLocatedAt, Confidence: 1})
	}
}

// buildSquare creates a public square within a district.
func buildSquare(idx int, pos float64, rng *rand.Rand, cfg CityConfig, prov world.Provenance) world.Entity {
	center := world.Vec3{X: pos * 100, Y: cfg.Size / 2}
	return world.Entity{
		ID:    fmt.Sprintf("square_%02d", idx),
		Class: world.ClassTerrain,
		Type:  "square.public",
		Transform: world.Transform{Position: center},
		BoundingBox: &world.BoundingBox{
			Min: center.Add(world.Vec3{X: -25, Y: -25}),
			Max: center.Add(world.Vec3{X: 25, Y: 25}),
		},
		Properties: map[string]any{
			"area_m2": 2500,
		},
		State:      world.EntityState{Alive: true, Health: 1, Condition: "intact"},
		Provenance: prov,
		Version:    world.SchemaVersion,
	}
}

// lineBBox computes a bounding box from a line (local copy).
func lineBBox(l world.Line) *world.BoundingBox {
	if len(l.Points) == 0 {
		return nil
	}
	min, max := l.Points[0], l.Points[0]
	for _, p := range l.Points[1:] {
		if p.X < min.X { min.X = p.X }
		if p.Y < min.Y { min.Y = p.Y }
		if p.Z < min.Z { min.Z = p.Z }
		if p.X > max.X { max.X = p.X }
		if p.Y > max.Y { max.Y = p.Y }
		if p.Z > max.Z { max.Z = p.Z }
	}
	return &world.BoundingBox{Min: min, Max: max}
}
