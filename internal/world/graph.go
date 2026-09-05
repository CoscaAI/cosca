package world

// World Graph: navigate the semantic graph of entities and relations.
// An entity can be queried by relation, by spatial containment, by hierarchy.

// ChildrenOf returns the entity IDs that have `parent` as their Parent.
func (w *World) ChildrenOf(parentID string) []string {
	var out []string
	for i := range w.Entities {
		if w.Entities[i].Parent == parentID {
			out = append(out, w.Entities[i].ID)
		}
	}
	return out
}

// ParentOf returns the parent ID of an entity, if any.
func (w *World) ParentOf(entityID string) (string, bool) {
	e := w.GetEntity(entityID)
	if e == nil || e.Parent == "" {
		return "", false
	}
	return e.Parent, true
}

// RelationsFrom returns all relations where the entity is the subject.
func (w *World) RelationsFrom(entityID string) []Relation {
	var out []Relation
	for _, r := range w.Relations {
		if r.Subject == entityID {
			out = append(out, r)
		}
	}
	return out
}

// RelatedTo returns entities related to `entityID` via a given relation type,
// where the entity is either subject or object.
func (w *World) RelatedTo(entityID string, rel RelationType) []string {
	var out []string
	seen := map[string]bool{}
	for _, r := range w.Relations {
		if r.Relation != rel {
			continue
		}
		if r.Subject == entityID && !seen[r.Object] {
			out = append(out, r.Object)
			seen[r.Object] = true
		}
		if r.Object == entityID && !seen[r.Subject] {
			out = append(out, r.Subject)
			seen[r.Subject] = true
		}
	}
	return out
}

// EntitiesInRegion returns all entities whose position falls inside the region.
func (w *World) EntitiesInRegion(region TerrainRegion) ([]Entity, error) {
	var out []Entity
	for _, e := range w.Entities {
		if pointInPolygon(e.Transform.Position, region.Polygon.Points) {
			out = append(out, e)
		}
	}
	return out, nil
}

// pointInPolygon does a ray-casting point-in-polygon test (XY plane).
func pointInPolygon(pt Vec3, poly []Vec3) bool {
	if len(poly) < 3 {
		return false
	}
	inside := false
	j := len(poly) - 1
	for i := 0; i < len(poly); i++ {
		xi, yi := poly[i].X, poly[i].Y
		xj, yj := poly[j].X, poly[j].Y
		// Ray-cast: does a ray from pt cross edge (i,j)?
		intersect := ((yi > pt.Y) != (yj > pt.Y)) &&
			(pt.X < (xj-xi)*(pt.Y-yi)/(yj-yi)+xi)
		if intersect {
			inside = !inside
		}
		j = i
	}
	return inside
}
