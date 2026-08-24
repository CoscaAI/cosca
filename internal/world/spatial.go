package world

// Spatial model: geometric primitives for describing shapes in the world.
// Pure Go, stdlib, no Unreal dependency.

// Point is a single location.
type Point struct {
	Position Vec3 `json:"position"`
}

// Line is a polyline in world space (e.g. a road centerline).
type Line struct {
	Points []Vec3 `json:"points"`
}

// Length returns the total polyline length in meters.
func (l Line) Length() float64 {
	var total float64
	for i := 1; i < len(l.Points); i++ {
		total += l.Points[i].DistanceTo(l.Points[i-1])
	}
	return total
}

// Polygon is a closed ring of points (in XY plane; Z is elevation).
type Polygon struct {
	Points []Vec3 `json:"points"`
}

// Area returns the polygon area (shoelace formula, XY-plane).
func (p Polygon) Area() float64 {
	if len(p.Points) < 3 {
		return 0
	}
	var sum float64
	n := len(p.Points)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		sum += p.Points[i].X*p.Points[j].Y - p.Points[j].X*p.Points[i].Y
	}
	return sum / 2
}

// Volume is a 3D region (e.g. a building volume).
type Volume struct {
	Position Vec3        `json:"position"`
	Size     Vec3        `json:"size"`
}

// TerrainRegion describes a contiguous area of terrain with a classification.
type TerrainRegion struct {
	Polygon  Polygon      `json:"polygon"`
	Class    string       `json:"class"`  // "forest", "water", "urban", "agriculture"
	Elevation float64     `json:"elevation"` // base elevation (meters)
}

// CoordinateTransform maps between world, geo, and engine spaces.
type CoordinateTransform struct {
	World  CoordinateSystem `json:"world"`
	Origin GeoCoordinates   `json:"origin"`
	Scale  float64          `json:"scale"`
}

// LineToPolygon creates a closed polygon from a line ring.
func LineToPolygon(points []Vec3) Polygon {
	return Polygon{Points: points}
}
