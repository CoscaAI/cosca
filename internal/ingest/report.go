package ingest

import (
	"fmt"

	"github.com/CoscaAI/cosca/internal/world"
)

// Semántic Inventory — the "explain what you found" report the professor
// asked for. This turns raw polygon counts into a computable description
// of the city:
//
//	Edificações: 847   Ruas: 126   Avenidas: 8   Praças: 1
//	Geometrias inválidas: 0   Relações órfãs: 0   Duplicadas: 0
//
// It is derived from the World Model (not from geometry), so it reflects
// semantic interpretation, not just mesh counts.

// Inventory is a semantic summary of a World Model.
type Inventory struct {
	AnalyzedArea  string `json:"analyzed_area"`
	Buildings     int    `json:"buildings"`
	Roads         int    `json:"roads"`
	Avenues       int    `json:"avenues"`
	Parks         int    `json:"parks"`
	Places        int    `json:"places"`
	WaterBodies   int    `json:"water_bodies"`
	OtherFeatures int    `json:"other_features"`

	InvalidGeometry   int `json:"invalid_geometry"`
	OrphanRelations   int `json:"orphan_relations"`
	DuplicateEntities int `json:"duplicate_entities"`

	EntityCount int `json:"entity_count"`
}

// InventoryOf computes a semantic inventory of a world.
func InventoryOf(w *world.World) Inventory {
	inv := Inventory{}
	seenIDs := map[string]bool{}

	for i := range w.Entities {
		e := &w.Entities[i]

		// Duplicate detection.
		if seenIDs[e.ID] {
			inv.DuplicateEntities++
		}
		seenIDs[e.ID] = true

		switch e.Class {
		case world.ClassStructure:
			inv.Buildings++
		case world.ClassRoad:
			if isAvenue(e) {
				inv.Avenues++
			} else {
				inv.Roads++
			}
		case world.ClassTerrain:
			if e.Type == "park" || e.Type == "square.public" {
				inv.Parks++
			} else {
				inv.OtherFeatures++
			}
		case world.ClassWater:
			inv.WaterBodies++
		default:
			inv.OtherFeatures++
		}
	}

	// Validation-derived counts.
	vr := world.Validate(w)
	for _, iss := range vr.Issues {
		switch iss.Code {
		case "nan_coords", "negative_scale", "nan_bbox":
			inv.InvalidGeometry++
		case "orphan_subject", "orphan_object", "orphan_parent":
			inv.OrphanRelations++
		}
	}

	inv.EntityCount = len(w.Entities)
	return inv
}

// isAvenue reports whether a road entity is an avenue (semantic distinction
// from the OSM highway tag). Avenues are "primary", "trunk", "motorway", or
// name contains "Avenida".
func isAvenue(e *world.Entity) bool {
	if name, ok := e.Properties["name"].(string); ok {
		if contains(name, "Avenida") || contains(name, "Av.") {
			return true
		}
	}
	t := string(e.Type)
	return t == "road.primary" || t == "road.trunk" || t == "road.motorway"
}

// String renders the inventory as a human-readable explanation.
func (inv Inventory) String() string {
	return fmt.Sprintf(
		"Área analisada: %s\n"+
			"Edificações: %d\n"+
			"Ruas: %d\n"+
			"Avenidas: %d\n"+
			"Praças: %d\n"+
			"Áreas verdes: %d\n"+
			"Hidrografia: %d\n"+
			"Outras feições: %d\n"+
			"---\n"+
			"Geometrias inválidas: %d\n"+
			"Relações órfãs: %d\n"+
			"Entidades duplicadas: %d\n"+
			"Total de entidades: %d",
		inv.AnalyzedArea,
		inv.Buildings,
		inv.Roads,
		inv.Avenues,
		inv.Parks,
		inv.Parks,
		inv.WaterBodies,
		inv.OtherFeatures,
		inv.InvalidGeometry,
		inv.OrphanRelations,
		inv.DuplicateEntities,
		inv.EntityCount,
	)
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
