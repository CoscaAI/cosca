package worlddebug

import (
	"math"

	"github.com/CoscaAI/cosca/internal/world"
)

// propertyString returns a property as a string, or "" if absent.
func propertyString(e *world.Entity, name string) string {
	if e == nil || e.Properties == nil {
		return ""
	}
	if v, ok := e.Properties[name]; ok {
		return toString(v)
	}
	return ""
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		if t == math.Trunc(t) {
			return formatInt(int64(t))
		}
		return formatFloat(t)
	case int:
		return formatInt(int64(t))
	case int64:
		return formatInt(t)
	default:
		return ""
	}
}

func formatInt(v int64) string {
	buf := make([]byte, 0, 20)
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	for v > 0 {
		buf = append([]byte{byte('0' + v%10)}, buf...)
		v /= 10
	}
	if neg {
		return "-" + string(buf)
	}
	return string(buf)
}

func formatFloat(f float64) string {
	// Truncate to 2 decimals.
	scaled := math.Round(f*100) / 100
	return formatInt(int64(scaled)) // approximate; sufficient for display
}

// geometryKind returns the entity's geometry kind.
func geometryKind(e *world.Entity) string {
	if e.Geometry == nil {
		return "unknown"
	}
	return e.Geometry.Kind
}

// areaOf computes the polygon area (shoelace) if closed.
func areaOf(pts []world.Vec3) float64 {
	if len(pts) < 3 {
		return 0
	}
	var sum float64
	n := len(pts)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		sum += pts[i].X*pts[j].Y - pts[j].X*pts[i].Y
	}
	return math.Abs(sum) / 2
}

// lengthOf computes polyline length.
func lengthOf(pts []world.Vec3) float64 {
	var total float64
	for i := 1; i < len(pts); i++ {
		total += pts[i].DistanceTo(pts[i-1])
	}
	return total
}

// assetFor resolves a semantic asset ID for an entity (used by Inspector).
// This mirrors the adapter's semantic mapping but stays observation-only.
func assetFor(e *world.Entity) string {
	switch e.Class {
	case world.ClassVegetation:
		return "vegetation.tree"
	case world.ClassRoad:
		switch e.Type {
		case "road.primary", "road.secondary", "road.trunk":
			return "road.avenue"
		case "road.service", "road.footway":
			return "road.path"
		default:
			return "road.residential"
		}
	case world.ClassStructure:
		return "building.house"
	case world.ClassWater:
		return "water.body"
	case world.ClassTerrain:
		if e.Type == "park" {
			return "terrain.park"
		}
		return "terrain.region"
	case world.ClassVehicle:
		return "vehicle.car"
	default:
		return "entity.object"
	}
}

// materialFor returns the default material name.
func materialFor(e *world.Entity) string {
	switch e.Class {
	case world.ClassStructure:
		return "plaster_white"
	case world.ClassRoad:
		return "asphalt"
	case world.ClassVegetation:
		return "foliage"
	case world.ClassWater:
		return "water"
	default:
		return "default"
	}
}

// averageScale returns the mean of the scale components.
func averageScale(s world.Vec3) float64 {
	return (s.X + s.Y + s.Z) / 3
}

// rotationDeg approximates rotation from a quaternion's Z-bias (display only).
func rotationDeg(q world.Quat) float64 {
	// Approximate yaw from quaternion: yaw = atan2(2(wz+xy), 1-2(y²+z²))
	yaw := math.Atan2(2*(q.W*q.Z+q.X*q.Y), 1-2*(q.Y*q.Y+q.Z*q.Z))
	deg := yaw * 180 / math.Pi
	if deg < 0 {
		deg += 360
	}
	return deg
}
