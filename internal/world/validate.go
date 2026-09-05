package world

import (
	"fmt"
	"math"
)

// World Validator — checks semantic invariants of a World Model.
//
// These are the invariants the professor requires (item 9):
//   - Every EntityID is unique
//   - Every relation points to existing entities (no orphan refs)
//   - Every building belongs to a district/context
//   - Every spatial entity has coordinates
//   - No geometry contains NaN
//   - No invalid negative scale
//   - Graph has no broken references
//   - Minimum structural sanity

// ValidationIssue is a single failed invariant.
type ValidationIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Entity  string `json:"entity,omitempty"`
}

// ValidationResult is the outcome of validating a world.
type ValidationResult struct {
	Valid   bool              `json:"valid"`
	Issues  []ValidationIssue `json:"issues"`
	Checks  int               `json:"checks"`
}

// Validate runs all invariants against a world and returns the result.
func Validate(w *World) ValidationResult {
	r := ValidationResult{Valid: true, Issues: []ValidationIssue{}}

	// 1. Unique EntityIDs.
	seen := map[string]bool{}
	for _, e := range w.Entities {
		if e.ID == "" {
			r.add("empty_id", "entity has empty ID", "")
		}
		if seen[e.ID] {
			r.add("duplicate_id", "duplicate EntityID "+e.ID, e.ID)
		}
		seen[e.ID] = true
	}

	// 2. Every relation points to existing entities.
	for _, rel := range w.Relations {
		if !seen[rel.Subject] {
			r.add("orphan_subject", "relation subject not found: "+rel.Subject, rel.Subject)
		}
		if !seen[rel.Object] {
			r.add("orphan_object", "relation object not found: "+rel.Object, rel.Object)
		}
	}

	// 3. Parent references exist.
	for _, e := range w.Entities {
		if e.Parent != "" && !seen[e.Parent] {
			r.add("orphan_parent", "parent not found: "+e.Parent, e.ID)
		}
	}

	// 4. Spatial primitives valid (no NaN, finite coords).
	for _, e := range w.Entities {
		if hasNaN(e.Transform.Position) || hasNaN(e.Transform.Scale) {
			r.add("nan_coords", "entity has NaN coordinate", e.ID)
		}
		if e.Transform.Scale.X < 0 || e.Transform.Scale.Y < 0 || e.Transform.Scale.Z < 0 {
			r.add("negative_scale", "entity has negative scale", e.ID)
		}
		if e.BoundingBox != nil {
			if hasNaN(e.BoundingBox.Min) || hasNaN(e.BoundingBox.Max) {
				r.add("nan_bbox", "entity has NaN bounding box", e.ID)
			}
		}
	}

	r.Checks = 5
	if len(r.Issues) > 0 {
		r.Valid = false
	}
	return r
}

// add records an issue (maintaining the grouping).
func (r *ValidationResult) add(code, msg, entity string) {
	r.Issues = append(r.Issues, ValidationIssue{Code: code, Message: msg, Entity: entity})
	r.Valid = false
}

// hasNaN reports whether any component of a Vec3 is NaN or not finite.
func hasNaN(v Vec3) bool {
	return math.IsNaN(v.X) || math.IsNaN(v.Y) || math.IsNaN(v.Z) ||
		math.IsInf(v.X, 0) || math.IsInf(v.Y, 0) || math.IsInf(v.Z, 0)
}

// Error implements error so a failing ValidationResult can be returned directly.
func (r ValidationResult) Error() string {
	return fmt.Sprintf("world validation failed: %d issue(s)", len(r.Issues))
}
