// Package ingest parses real-world geodata (start with OSM) into the
// World Model. This is the "FASE B — Real World Ingestion" per the professor.
//
// IMPORTANT: The world model stays pure. This package translates external
// formats (OSM JSON) into world.Entity semantics with provenance. It is a
// SEPARATE concern from rendering.
//
// The pipeline: OSM → Parser → Normalization → World Entities → World Model.
//
// Design: learned from examining OSM data, not copied. Supports the
// geometries and tag-classifications present in real OSM extracts.
package ingest

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/world"
)

// ──────────────────────────────────────────────────────────────
// OSM raw structures (subset of the Overpass JSON output)
// ──────────────────────────────────────────────────────────────

// osmElement is a single element from Overpass (node/way/relation).
type osmElement struct {
	Type   string            `json:"type"`
	ID     int64             `json:"id"`
	Lat    float64           `json:"lat,omitempty"`
	Lon    float64           `json:"lon,omitempty"`
	Nodes  []int64           `json:"nodes,omitempty"`
	Members []osmMember       `json:"members,omitempty"`
	Tags   map[string]string `json:"tags,omitempty"`
}

// osmMember is a relation member.
type osmMember struct {
	Type string `json:"type"`
	Ref  int64  `json:"ref"`
	Role string `json:"role"`
}

// osmDoc is the top-level Overpass JSON.
type osmDoc struct {
	Elements []osmElement `json:"elements"`
}

// ──────────────────────────────────────────────────────────────
// Ingestion config
// ──────────────────────────────────────────────────────────────

// IngestConfig configures OSM ingestion.
type IngestConfig struct {
	SourceDataset string // e.g. "osm"
	SourceVersion string
	Origin        world.GeoCoordinates // world anchor for coordinate conversion
}

// DefaultConfig returns a config for OSM ingestion.
func DefaultConfig(origin world.GeoCoordinates) IngestConfig {
	return IngestConfig{
		SourceDataset: "osm",
		SourceVersion: "1.0",
		Origin:        origin,
	}
}

// ──────────────────────────────────────────────────────────────
// Result
// ──────────────────────────────────────────────────────────────

// Result is the outcome of ingesting OSM into a World Model.
type Result struct {
	World   *world.World
	Classes map[world.EntityClass]int
}

// ──────────────────────────────────────────────────────────────
// Parser
// ──────────────────────────────────────────────────────────────

// ParseOSM parses an OSM Overpass JSON byte stream (read-only) into a World.
// It never mutates input; it only reads and normalizes.
func ParseOSM(data []byte, cfg IngestConfig) (*Result, error) {
	var doc osmDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("osm parse: %w", err)
	}

	cs := world.CoordinateSystem{
		Origin: cfg.Origin,
		Units:  "meters",
		Scale:  1.0,
	}
	w := world.NewWorld("osm-import", cs)

	// Index nodes by OSM id (unused id casing).
	nodes := map[int64]world.GeoCoordinates{}
	for _, el := range doc.Elements {
		if el.Type == "node" {
			nodes[el.ID] = world.GeoCoordinates{Latitude: el.Lat, Longitude: el.Lon}
		}
	}

	counts := map[world.EntityClass]int{}

	// Process ways (roads/buildings/water/park).
	idSeq := 0
	for _, el := range doc.Elements {
		if el.Type != "way" {
			continue
		}
		entity, ok := wayToEntity(el, nodes, cs, cfg, &idSeq)
		if ok {
			w.AddEntity(entity)
			counts[entity.Class]++
		}
	}

	return &Result{World: w, Classes: counts}, nil
}

// wayToEntity converts an OSM way into a World Entity, if it is recognized.
func wayToEntity(el osmElement, nodes map[int64]world.GeoCoordinates, cs world.CoordinateSystem, cfg IngestConfig, idSeq *int) (world.Entity, bool) {
	tags := el.Tags
	if tags == nil {
		return world.Entity{}, false
	}

	// Determine class from tags.
	class, etype, recognized := classifyWay(tags)
	if !recognized {
		return world.Entity{}, false
	}

	// Build geometry from node coordinates.
	pts := make([]world.Vec3, 0, len(el.Nodes))
	for _, nid := range el.Nodes {
		geo, ok := nodes[nid]
		if !ok {
			continue
		}
		pts = append(pts, cs.GeoToWorld(geo))
	}
	if len(pts) < 2 {
		return world.Entity{}, false
	}

	// Point/LineString/Polygon inference: closed → polygon, else linestring.
	kind := "linestring"
	closed := len(pts) > 2 && pts[0].DistanceTo(pts[len(pts)-1]) < 0.01
	if closed {
		kind = "polygon"
	}

	prov := world.Provenance{
		Class:   world.ClassFACT, // observed real-world data
		Source: world.Source{
			Dataset:   cfg.SourceDataset,
			Version:   cfg.SourceVersion,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Hash:      fmt.Sprintf("osm:way:%d", el.ID), // source identifier
		},
		Accuracy: 0.9, // OSM generally reliable
	}

	*idSeq++
	entity := world.Entity{
		ID:        fmt.Sprintf("osm:%s:%s:%d", class, slug(tags["name"]), el.ID),
		Class:     class,
		Type:      etype,
		Transform: world.Transform{Position: pts[0]},
		Geometry: &world.Geometry{
			Kind:       kind,
			Points:     pts,
			BoundingBox: lineBBox(pts),
		},
		BoundingBox: lineBBox(pts),
		Properties: map[string]any{
			"osm_id":          strconv.FormatInt(el.ID, 10),
			"name":            tags["name"],
			"source_dataset":  cfg.SourceDataset,
		},
		State:      world.EntityState{Alive: true, Health: 1, Condition: "intact"},
		Provenance: prov,
		Version:    world.SchemaVersion,
	}
	return entity, true
}

// classifyWay maps OSM tags to a world entity class + type.
func classifyWay(tags map[string]string) (world.EntityClass, world.EntityType, bool) {
	// Priority order: water > park > building > road.
	switch {
	case tags["waterway"] != "" || tags["natural"] == "water":
		return world.ClassWater, world.EntityType("water." + firstNonEmpty(tags["waterway"], tags["natural"])), true
	case tags["leisure"] == "park" || tags["landuse"] == "forest":
		return world.ClassTerrain, world.EntityType("park"), true
	case tags["building"] != "" && tags["building"] != "no":
		return world.ClassStructure, world.EntityType("building." + tags["building"]), true
	case tags["highway"] != "":
		return world.ClassRoad, world.EntityType("road." + tags["highway"]), true
	default:
		return "", "", false
	}
}

// helpers

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return "unknown"
}

func slug(s string) string {
	if s == "" {
		return "unnamed"
	}
	var b strings.Builder
	for _, r := range s {
		if isAlnum(r) {
			b.WriteRune(toLower(r))
		}
	}
	if b.Len() == 0 {
		return "unnamed"
	}
	return b.String()
}

func isAlnum(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func toLower(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + 32
	}
	return r
}

func lineBBox(pts []world.Vec3) *world.BoundingBox {
	if len(pts) == 0 {
		return nil
	}
	min, max := pts[0], pts[0]
	for _, p := range pts[1:] {
		if p.X < min.X { min.X = p.X }
		if p.Y < min.Y { min.Y = p.Y }
		if p.Z < min.Z { min.Z = p.Z }
		if p.X > max.X { max.X = p.X }
		if p.Y > max.Y { max.Y = p.Y }
		if p.Z > max.Z { max.Z = p.Z }
	}
	return &world.BoundingBox{Min: min, Max: max}
}
