package bootstrap

import (
	"fmt"

	"github.com/CoscaAI/cosca/internal/di"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/memory"
)

// ── Declarative Service Registry (Backstage Pattern #2) ─────────────
//
// The engines built by Compose are exposed as DI services so that agents,
// departments and sub-systems can declare the dependencies they need and
// have the runtime inject them — instead of reaching into the hardcoded
// Result struct by name. Each registry entry mirrors @backstage's
// "service" + "deps": the id names the service, Deps lists what it needs
// before it can be built, and Build resolves the dependencies from the
// container.
//
// This is the seam that lets a future "agent as module" contribute a
// service that depends on knowledge+memory without touching Compose.

// Service IDs registered in the DI container.
const (
	ServiceKnowledge = "service.knowledge"
	ServiceMemory    = "service.memory"
	// ServiceKnowledgeSummary is an example facet: it depends on the
	// knowledge engine and is built on demand by the container.
	ServiceKnowledgeSummary = "service.knowledge.summary"
)

// RegisterEngines wires the already-built engines into the DI container as
// root-scoped singleton services. It is additive and does not replace the
// explicit wiring in Compose (which stays the source of truth for the
// lifecycle); it only makes the engines reachable through the DI graph.
//
// Returns the container so callers can Resolve by typed ref.
func RegisterEngines(res *Result) *di.Container {
	c := di.New()

	if res.Knowledge != nil {
		// knowledge engine as a singleton service.
		c.Register(di.Definition{
			ID:    ServiceKnowledge,
			Scope: di.ScopeRoot,
			Build: func(*di.Container) (any, error) { return res.Knowledge, nil },
		})
	}

	if res.Memory != nil {
		c.Register(di.Definition{
			ID:    ServiceMemory,
			Scope: di.ScopeRoot,
			Build: func(*di.Container) (any, error) { return res.Memory, nil },
		})
	}

	if res.Knowledge != nil {
		// A composable service that DEPENDS on knowledge — the Backstage
		// idiom: declare the dep, the container injects it.
		c.Register(di.Definition{
			ID:    ServiceKnowledgeSummary,
			Scope: di.ScopeRoot,
			Deps:  []string{ServiceKnowledge},
			Build: func(c *di.Container) (any, error) {
				v, err := c.Dependency(ServiceKnowledge)
				if err != nil {
					return nil, fmt.Errorf("knowledge summary: %w", err)
				}
				ke, ok := v.(*knowledge.Engine)
				if !ok {
					return nil, fmt.Errorf("knowledge summary: dep %q is %T, want *knowledge.Engine", ServiceKnowledge, v)
				}
				return &KnowledgeSummary{Source: ServiceKnowledge, Engine: ke}, nil
			},
		})
	}

	return c
}

// KnowledgeSummary is a thin DI-consumed facet: it depends on the knowledge
// engine (injected by the container) and exposes its count. Kept small on
// purpose — it proves the injection seam, not a full feature.
type KnowledgeSummary struct {
	Source string
	Engine *knowledge.Engine
}

// Count delegates to the injected engine. The runtime never reaches into a
// global or a Result struct — the dependency was injected by the container.
func (s *KnowledgeSummary) Count() (int, error) {
	if s.Engine == nil {
		return 0, fmt.Errorf("knowledge summary: no engine injected")
	}
	st, err := s.Engine.GetStats()
	if err != nil {
		return 0, err
	}
	if st == nil {
		return 0, nil
	}
	return st.DocumentCount, nil
}

// ResolveKnowledgeSummary resolves the composable facet from the container.
func ResolveKnowledgeSummary(c *di.Container) (*KnowledgeSummary, error) {
	return di.Resolve(c, di.Ref[*KnowledgeSummary](ServiceKnowledgeSummary))
}

// ResolveKnowledge returns the knowledge engine from the DI container.
func ResolveKnowledge(c *di.Container) (*knowledge.Engine, error) {
	v, err := di.Resolve(c, di.Ref[*knowledge.Engine](ServiceKnowledge))
	if err != nil {
		return nil, err
	}
	return v, nil
}

// ResolveMemory returns the memory engine from the DI container.
func ResolveMemory(c *di.Container) (*memory.MemoryEngine, error) {
	v, err := di.Resolve(c, di.Ref[*memory.MemoryEngine](ServiceMemory))
	if err != nil {
		return nil, err
	}
	return v, nil
}
