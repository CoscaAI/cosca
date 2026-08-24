package world

import (
	"sort"
)

// World Queries — spatial + semantic reasoning over a World Model.
//
// These answer the questions the professor listed (item 11):
//   - How many districts exist?
//   - Which road connects district 2 to district 4?
//   - Which buildings are near the river?
//   - Which trees are inside a plaza?
//   - Which is the tallest building?
//   - Which entities are inside a region?

// CountByClass returns entity counts grouped by class.
func (w *World) CountByClass() map[EntityClass]int {
	m := map[EntityClass]int{}
	for _, e := range w.Entities {
		m[e.Class]++
	}
	return m
}

// EntitiesOfClass returns entities of a given class.
func (w *World) EntitiesOfClass(c EntityClass) []Entity {
	var out []Entity
	for _, e := range w.Entities {
		if e.Class == c {
			out = append(out, e)
		}
	}
	return out
}

// ByType returns entities of a specific semantic type (e.g. "building.house").
func (w *World) ByType(t EntityType) []Entity {
	var out []Entity
	for _, e := range w.Entities {
		if e.Type == t {
			out = append(out, e)
		}
	}
	return out
}

// TallestBuilding returns the building with the largest height (from the
// "height" property, or from the bounding box Z-size as a fallback).
func (w *World) TallestBuilding() (*Entity, bool) {
	var best *Entity
	var bestH float64
	for i := range w.Entities {
		e := &w.Entities[i]
		if e.Class != ClassStructure {
			continue
		}
		h := propertyHeight(e)
		if h > bestH {
			bestH = h
			best = e
		}
	}
	return best, best != nil
}

// EntitiesNear returns entities within `radius` meters of a position.
// Used for "buildings near river".
func (w *World) EntitiesNear(pos Vec3, radius float64) []Entity {
	var out []Entity
	for _, e := range w.Entities {
		if e.Transform.Position.DistanceTo(pos) <= radius {
			out = append(out, e)
		}
	}
	return out
}

// EntitiesInside returns entities whose position falls within a polygon region.
func (w *World) EntitiesInside(poly Polygon) []Entity {
	var out []Entity
	for _, e := range w.Entities {
		if pointInPolygon(e.Transform.Position, poly.Points) {
			out = append(out, e)
		}
	}
	return out
}

// ConnectBetween returns entities of `viaClass` that connect the two given
// entities (direct or via relations). Answers "which road connects district X to Y".
func (w *World) ConnectBetween(a, b string) []Entity {
	// Roads that are related (locate/connect) to both a and b.
	var out []Entity
	for _, e := range w.Entities {
		if e.Class != ClassRoad {
			continue
		}
		if w.relatedToAny(e.ID, a) && w.relatedToAny(e.ID, b) {
			out = append(out, e)
		}
	}
	return out
}

// relatedToAny reports whether entity `id` is related to `target` directly
// or transitively through one hop.
func (w *World) relatedToAny(id, target string) bool {
	for _, r := range w.Relations {
		if (r.Subject == id && r.Object == target) || (r.Subject == target && r.Object == id) {
			return true
		}
	}
	// one hop: relation via a common entity
	neighbors := w.adjacency(id)
	if neighbors[target] {
		return true
	}
	return false
}

// adjacency returns a set of direct neighbors of an entity.
func (w *World) adjacency(id string) map[string]bool {
	out := map[string]bool{}
	for _, r := range w.Relations {
		if r.Subject == id {
			out[r.Object] = true
		}
		if r.Object == id {
			out[r.Subject] = true
		}
	}
	return out
}

// SortedIDs returns entity IDs sorted ascending (nice for stable enumeration).
func (w *World) SortedIDs() []string {
	ids := make([]string, len(w.Entities))
	for i, e := range w.Entities {
		ids[i] = e.ID
	}
	sort.Strings(ids)
	return ids
}

// propertyHeight extracts a building's height (property "height" or "total_height")
// falling back to bounding box Z extent.
func propertyHeight(e *Entity) float64 {
	if e.Properties != nil {
		if h, ok := e.Properties["height"].(float64); ok {
			return h
		}
		if h, ok := e.Properties["total_height"].(float64); ok {
			return h
		}
	}
	return e.BoundingBox.Size().Z
}
