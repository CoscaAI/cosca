// Package worlddebug is the Visual Understanding / Scene Inspector for the
// Cosca World Model. It is a SEPARATE observation layer (per the professor)
// that explains WHAT the Cosca knows, WHERE it came from, WHAT it inferred,
// WHAT decision it made, and HOW it became a representation.
//
// REGRA DE OURO (professor): the Inspector is NEVER the source of truth.
// It only observes the World Model + provenance. The truth stays in:
//   World Model + Provenance.
//
// System design (per professor):
//   World Model → queries / events / provenance / decisions → Inspector → Unreal
//
// This layer is pure observation — it never mutates the World.
package worlddebug

import (
	"fmt"
	"strings"

	"github.com/CoscaAI/cosca/internal/world"
)

// EpistemicClass is the provenance kind for a single attribute.
type EpistemicClass string

const (
	Observed  EpistemicClass = "OBSERVED"  // came directly from source data
	Inferred  EpistemicClass = "INFERRED"  // deduced from other evidence
	Generated EpistemicClass = "GENERATED" // created by Cosca to fill gaps
)

// Attribute is a single named property with its epistemic class.
type Attribute struct {
	Name  string         `json:"name"`
	Value string         `json:"value"`
	Class EpistemicClass `json:"class"`
}

// EntityInspect is the explanation of one entity.
type EntityInspect struct {
	ID      string `json:"id"`

	// — Entity —
	Class    string `json:"class"`
	Type     string `json:"type"`
	Source   string `json:"source"`
	Geometry string `json:"geometry"`
	Area     string `json:"area"`
	Height   string `json:"height"`
	Confidence float64 `json:"confidence"`

	// — Representation —
	Representation ReprInspect `json:"representation"`

	// — Reasoning —
	Reasoning ReasoningInspect `json:"reasoning"`

	// — Provenance —
	Provenance []Attribute `json:"provenance"`

	// — Relations —
	Relations []RelationLine `json:"relations"`
}

// ReprInspect is the visual representation decision for an entity.
type ReprInspect struct {
	Asset    string `json:"asset"`
	Scale    string `json:"scale"`
	Rotation string `json:"rotation"`
	Material string `json:"material"`
	LOD      string `json:"lod"`
}

// ReasoningInspect shows the decision logic.
type ReasoningInspect struct {
	Inputs      []Attribute `json:"inputs"`
	Reasons     []string    `json:"reasons"`
	Warnings    []string    `json:"warnings"`
	Uncertainty []string    `json:"uncertainty"`
	Confidence  float64     `json:"confidence"`
}

// RelationLine is a rendered relation edge.
type RelationLine struct {
	Relation string `json:"relation"`
	EntityID string `json:"entity_id"`
}

// Inspector observes a World Model and produces explanations.
type Inspector struct {
	world *world.World
}

// New creates an Inspector over a world. It observes, never mutates.
func New(w *world.World) *Inspector {
	return &Inspector{world: w}
}

// Explain produces the full entity explanation.
func (in *Inspector) Explain(id string) (*EntityInspect, bool) {
	e := in.world.GetEntity(id)
	if e == nil {
		return nil, false
	}
	return in.explainEntity(e), true
}

// explainEntity builds the structured explanation.
func (in *Inspector) explainEntity(e *world.Entity) *EntityInspect {
	// Entity section.
	geomKind := "unknown"
	area := "unknown"
	if e.Geometry != nil {
		geomKind = e.Geometry.Kind
		switch e.Geometry.Kind {
		case "polygon":
			area = fmt.Sprintf("%.1f m²", areaOf(e.Geometry.Points))
		case "linestring":
			area = fmt.Sprintf("%.1f m (length)", lengthOf(e.Geometry.Points))
		}
	}
	height := propertyString(e, "height")
	if height == "" {
		height = "unknown"
	}

	// Provenance per attribute (epistemic classes).
	prov := in.provenanceOf(e)

	// Representation decision.
	repr := ReprInspect{
		Asset:    assetFor(e), // semantic asset ID
		Scale:    fmt.Sprintf("%.2f", averageScale(e.Transform.Scale)),
		Rotation: fmt.Sprintf("%.0f", rotationDeg(e.Transform.Rotation)),
		Material: materialFor(e),
		LOD:      "LOD0", // default; would be resolved by renderer
	}

	// Reasoning: inputs + reasons + warnings + uncertainty.
	reasoning := in.reasoning(e, prov)

	// Relations.
	relations := in.relationsOf(e.ID)

	conf := e.Provenance.Accuracy
	if conf == 0 {
		conf = 0.5
	}

	return &EntityInspect{
		ID:             e.ID,
		Class:          string(e.Class),
		Type:           string(e.Type),
		Source:         e.Provenance.Source.Dataset,
		Geometry:       geomKind,
		Area:           area,
		Height:         height,
		Confidence:     conf,
		Representation: repr,
		Reasoning:      reasoning,
		Provenance:     prov,
		Relations:      relations,
	}
}

// provenanceOf builds the per-attribute epistemic list.
func (in *Inspector) provenanceOf(e *world.Entity) []Attribute {
	// Source-derived attributes are OBSERVED at entity level (class, geometry, position).
	attrs := []Attribute{
		{Name: "class", Value: string(e.Class), Class: Observed},
		{Name: "geometry", Value: geometryKind(e), Class: Observed},
		{Name: "position", Value: fmt.Sprintf("(%.1f, %.1f, %.1f)", e.Transform.Position.X, e.Transform.Position.Y, e.Transform.Position.Z), Class: Observed},
	}
	// Type is OBSERVED if from source (OSM tag), else inferred.
	// Building type, height, species are typically INFERRED.
	if e.Provenance.Class == world.ClassFACT {
		attrs = append(attrs,
			Attribute{Name: "type", Value: string(e.Type), Class: Observed},
		)
	} else {
		attrs = append(attrs,
			Attribute{Name: "type", Value: string(e.Type), Class: Inferred},
		)
	}

	// Height / species are inferred unless present as a hard property.
	if propertyString(e, "height") == "" {
		attrs = append(attrs, Attribute{Name: "height", Value: "estimated", Class: Inferred})
	}
	// Visual mesh is always GENERATED (Cosca created it to fill the render gap).
	attrs = append(attrs, Attribute{Name: "visual_mesh", Value: assetFor(e), Class: Generated})

	return attrs
}

// reasoning builds the decision logic transcript.
func (in *Inspector) reasoning(e *world.Entity, prov []Attribute) ReasoningInspect {
	r := ReasoningInspect{Inputs: prov, Confidence: e.Provenance.Accuracy}

	switch e.Class {
	case world.ClassStructure:
		r.Reasons = append(r.Reasons,
			"classified as building (OSM tag building=*)",
			"footprint polygon from source",
			"located in urban context",
		)
		if propertyString(e, "height") == "" {
			r.Uncertainty = append(r.Uncertainty, "height: INFERRED (unavailable in source)")
		}
	case world.ClassRoad:
		r.Reasons = append(r.Reasons,
			"classified as road (OSM tag highway=*)",
			"centerline linestring from source",
			"connects to network",
		)
	case world.ClassTerrain:
		if e.Type == "park" {
			r.Reasons = append(r.Reasons, "classified as park (OSM leisure=park)", "boundary polygon from source")
		}
	case world.ClassWater:
		r.Reasons = append(r.Reasons, "classified as water (OSM natural/waterway)", "extent from source")
	default:
		r.Reasons = append(r.Reasons, "classified from source tags")
	}

	return r
}

// relationsOf lists the entity's relations.
func (in *Inspector) relationsOf(id string) []RelationLine {
	var out []RelationLine
	for _, rel := range in.world.Relations {
		if rel.Subject == id {
			out = append(out, RelationLine{Relation: string(rel.Relation), EntityID: rel.Object})
		}
		if rel.Object == id {
			out = append(out, RelationLine{Relation: string(rel.Relation) + " (inverse)", EntityID: rel.Subject})
		}
	}
	return out
}

// ──────────────────────────────────────────────────────────────
// Formatting helpers
// ──────────────────────────────────────────────────────────────

// String renders the EntityInspect as the professor's text layout.
func (in *EntityInspect) String() string {
	var b strings.Builder
	b.WriteString("ENTITY\n")
	b.WriteString("────────────────────\n")
	fmt.Fprintf(&b, "ID: %s\n", in.ID)
	fmt.Fprintf(&b, "Class: %s\n", in.Class)
	fmt.Fprintf(&b, "Type: %s\n", in.Type)
	fmt.Fprintf(&b, "Source: %s\n", in.Source)
	fmt.Fprintf(&b, "Geometry: %s\n", in.Geometry)
	fmt.Fprintf(&b, "Area: %s\n", in.Area)
	fmt.Fprintf(&b, "Height: %s\n", in.Height)
	fmt.Fprintf(&b, "Confidence: %.2f\n\n", in.Confidence)

	b.WriteString("REPRESENTATION\n")
	b.WriteString("────────────────────\n")
	fmt.Fprintf(&b, "Asset: %s\n", in.Representation.Asset)
	fmt.Fprintf(&b, "Scale: %s\n", in.Representation.Scale)
	fmt.Fprintf(&b, "Rotation: %s°\n", in.Representation.Rotation)
	fmt.Fprintf(&b, "Material: %s\n", in.Representation.Material)
	fmt.Fprintf(&b, "LOD: %s\n\n", in.Representation.LOD)

	b.WriteString("REASONING\n")
	b.WriteString("────────────────────\n")
	for _, r := range in.Reasoning.Reasons {
		fmt.Fprintf(&b, "✓ %s\n", r)
	}
	for _, w := range in.Reasoning.Warnings {
		fmt.Fprintf(&b, "⚠ %s\n", w)
	}
	for _, u := range in.Reasoning.Uncertainty {
		fmt.Fprintf(&b, "⚠ %s\n", u)
	}
	fmt.Fprintf(&b, "Confidence: %.2f\n\n", in.Reasoning.Confidence)

	b.WriteString("PROVENANCE\n")
	b.WriteString("────────────────────\n")
	grouped := map[EpistemicClass][]Attribute{}
	var order []EpistemicClass
	for _, p := range in.Provenance {
		if _, ok := grouped[p.Class]; !ok {
			order = append(order, p.Class)
		}
		grouped[p.Class] = append(grouped[p.Class], p)
	}
	for _, cls := range order {
		fmt.Fprintf(&b, "%s:\n", cls)
		for _, p := range grouped[cls] {
			fmt.Fprintf(&b, "  %s = %s\n", p.Name, p.Value)
		}
	}

	if len(in.Relations) > 0 {
		b.WriteString("\nRELATIONS\n")
		b.WriteString("────────────────────\n")
		for _, rel := range in.Relations {
			fmt.Fprintf(&b, "%s ──%s──> %s\n", in.ID, rel.Relation, rel.EntityID)
		}
	}
	return b.String()
}
