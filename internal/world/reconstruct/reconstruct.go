// Package reconstruct materializes a world.World into a renderer via the
// RendererAdapter interface. This is the "World Model → reconstruction" step.
//
// The renderer decides how to represent each entity; the reconstructor only
// maps entities to renderer calls. It is renderer-agnostic (uses the
// RendererAdapter contract), so the SAME world can go to Unreal, Blender,
// a map, or a simulation.
//
// This enables the professor's vision:
//   World Model → Reconstructor → RendererAdapter → Unreal/Blender/map
package reconstruct

import (
	"fmt"

	"github.com/CoscaAI/cosca/internal/world"
)

// Reconstructor materializes a whole world through a RendererAdapter.
type Reconstructor struct {
	renderer world.RendererAdapter
}

// New creates a Reconstructor bound to a RendererAdapter.
func New(renderer world.RendererAdapter) *Reconstructor {
	return &Reconstructor{renderer: renderer}
}

// Options configures materialization.
type Options struct {
	// MaxEntities limits how many entities get materialized (0 = all).
	MaxEntities int
	// OnlyClasses filters by entity class (empty = all).
	OnlyClasses []world.EntityClass
}

// Result reports how many entities were materialized.
type Result struct {
	Materialized int
	Skipped      int
	Errors       []error
}

// Reconstruct iterates a world's entities and materializes each via the
// adapter. It also applies world state (weather/time) per the professor's
// pipeline (World → renderer includes non-entity world state).
func (r *Reconstructor) Reconstruct(w *world.World, opts Options) (*Result, error) {
	res := &Result{Errors: []error{}}

	allowed := map[world.EntityClass]bool{}
	for _, c := range opts.OnlyClasses {
		allowed[c] = true
	}

	count := 0
	for i := range w.Entities {
		e := w.Entities[i]

		// Class filter.
		if len(allowed) > 0 && !allowed[e.Class] {
			res.Skipped++
			continue
		}

		// Entity count cap.
		if opts.MaxEntities > 0 && count >= opts.MaxEntities {
			res.Skipped++
			continue
		}

		if _, err := r.renderer.MaterializeEntity(e); err != nil {
			res.Errors = append(res.Errors, fmt.Errorf("materialize %s: %w", e.ID, err))
			continue
		}
		res.Materialized++
		count++
	}

	// Apply non-entity world state (weather/time/season).
	if err := r.renderer.ApplyWorldState(w.Weather, w.Time, w.Season); err != nil {
		res.Errors = append(res.Errors, fmt.Errorf("apply world state: %w", err))
	}

	return res, nil
}
