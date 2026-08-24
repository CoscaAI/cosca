package world

// Entity factories — ergonomic builders for common entity types.
// These are pure semantic constructors; the renderer decides the visual form.

// TerrainEntity creates a terrain region entity.
func TerrainEntity(id string, region TerrainRegion, prov Provenance) Entity {
	return Entity{
		ID:         id,
		Class:      ClassTerrain,
		Type:       "terrain.region",
		Transform:  Transform{Position: region.Polygon.Points[0]},
		BoundingBox: polygonBBox(region.Polygon),
		Properties: map[string]any{
			"class":     region.Class,
			"elevation": region.Elevation,
		},
		State:     EntityState{Alive: true, Health: 1, Condition: "intact"},
		Provenance: prov,
		Version:   SchemaVersion,
	}
}

// RoadEntity creates a road entity from a centerline.
func RoadEntity(id string, line Line, width float64, prov Provenance) Entity {
	return Entity{
		ID:        id,
		Class:     ClassRoad,
		Type:      "road.way",
		Transform: Transform{Position: midpoint(line)},
		BoundingBox: lineBBox(line),
		Properties: map[string]any{
			"length": line.Length(),
			"width":  width,
			"surface": "asphalt",
		},
		State:     EntityState{Alive: true, Health: 1, Condition: "intact"},
		Provenance: prov,
		Version:   SchemaVersion,
	}
}

// TreeEntity creates a vegetation entity.
func TreeEntity(id string, species string, pos Vec3, age string, prov Provenance) Entity {
	return Entity{
		ID:        id,
		Class:     ClassVegetation,
		Type:      EntityType("tree." + species),
		Transform: Transform{Position: pos, Scale: Vec3{1, 1, 1}},
		Properties: map[string]any{
			"species": species,
			"age":     age,
		},
		State:     EntityState{Alive: true, Health: 1, Condition: "intact", Age: age},
		Provenance: prov,
		Version:   SchemaVersion,
	}
}

// BuildingEntity creates a structure entity.
func BuildingEntity(id string, pos Vec3, size Vec3, prov Provenance) Entity {
	return Entity{
		ID:        id,
		Class:     ClassStructure,
		Type:      "building.house",
		Transform: Transform{Position: pos},
		BoundingBox: &BoundingBox{
			Min: pos,
			Max: pos.Add(size),
		},
		Properties: map[string]any{
			"footprint_w": size.X,
			"footprint_d": size.Y,
			"height":      size.Z,
		},
		State:     EntityState{Alive: true, Health: 1, Condition: "intact"},
		Provenance: prov,
		Version:   SchemaVersion,
	}
}

// RockEntity creates a rock/geological entity.
func RockEntity(id string, pos Vec3, prov Provenance) Entity {
	return Entity{
		ID:        id,
		Class:     ClassTerrain,
		Type:      "rock.boulder",
		Transform: Transform{Position: pos, Scale: Vec3{1, 1, 1}},
		Properties: map[string]any{
			"kind": "boulder",
		},
		State:     EntityState{Alive: true, Health: 1, Condition: "intact"},
		Provenance: prov,
		Version:   SchemaVersion,
	}
}

// midpoint returns the midpoint of a line's points.
func midpoint(l Line) Vec3 {
	if len(l.Points) == 0 {
		return Vec3{}
	}
	if len(l.Points) == 1 {
		return l.Points[0]
	}
	first := l.Points[0]
	last := l.Points[len(l.Points)-1]
	return Vec3{(first.X + last.X) / 2, (first.Y + last.Y) / 2, (first.Z + last.Z) / 2}
}

// lineBBox computes the bounding box of a line.
func lineBBox(l Line) *BoundingBox {
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
	return &BoundingBox{Min: min, Max: max}
}

// polygonBBox computes the bounding box of a polygon.
func polygonBBox(p Polygon) *BoundingBox {
	return lineBBox(Line{Points: p.Points})
}
