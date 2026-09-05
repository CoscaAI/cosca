package worlddebug

import (
	"fmt"
	"strings"

	"github.com/CoscaAI/cosca/internal/world"
)

// Event is a single step in the world's construction timeline.
type Event struct {
	Step    int    `json:"step"`
	Type    string `json:"type"`   // "ingest", "entity_created", "classified", "relation", "asset", "validate"
	Entity  string `json:"entity_id,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

// Replay is the step-by-step reconstruction trace of a World.
// The professor's idea: watch the world come to life step by step.
type Replay struct {
	events []Event
}

// BuildReplay reconstructs the timeline of a World's construction from its
// entities and relations (in deterministic order). This is a pure observation
// of the World Model — it does NOT re-run generation.
func BuildReplay(w *world.World) *Replay {
	r := &Replay{events: []Event{}}
	step := 0

	// Ingest / source step.
	step++
	origin := w.CoordSystem.Origin
	r.events = append(r.events, Event{Step: step, Type: "ingest", Detail: fmt.Sprintf("source=osm origin=(%.4f,%.4f) entities=%d", origin.Latitude, origin.Longitude, len(w.Entities))})

	// Each entity, deterministic order by ID.
	ids := w.SortedIDs()
	for _, id := range ids {
		e := w.GetEntity(id)
		if e == nil {
			continue
		}

		step++
		r.events = append(r.events, Event{Step: step, Type: "entity_created", Entity: id, Detail: "class=" + string(e.Class)})

		step++
		r.events = append(r.events, Event{Step: step, Type: "classified", Entity: id, Detail: "type=" + string(e.Type)})

		step++
		geom := "unknown"
		if e.Geometry != nil {
			geom = e.Geometry.Kind
		}
		r.events = append(r.events, Event{Step: step, Type: "geometry", Entity: id, Detail: geom})

		step++
		r.events = append(r.events, Event{Step: step, Type: "asset", Entity: id, Detail: assetFor(e)})
	}

	// Relations.
	for _, rel := range w.Relations {
		step++
		r.events = append(r.events, Event{Step: step, Type: "relation", Entity: rel.Subject, Detail: fmt.Sprintf("%s->%s", rel.Relation, rel.Object)})
	}

	// Validation.
	vr := world.Validate(w)
	step++
	status := "OK"
	if !vr.Valid {
		status = fmt.Sprintf("%d issues", len(vr.Issues))
	}
	r.events = append(r.events, Event{Step: step, Type: "validate", Detail: status})

	return r
}

// Events returns the full timeline.
func (r *Replay) Events() []Event { return r.events }

// String renders the replay as a readable trace.
func (r *Replay) String() string {
	var b strings.Builder
	for _, e := range r.events {
		fmt.Fprintf(&b, "[%02d] %-16s %-24s %s\n", e.Step, e.Type, e.Entity, e.Detail)
	}
	return b.String()
}
