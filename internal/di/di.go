// Package di implements a declarative dependency-injection container
// modeled on the Backstage (Spotify) ServiceRef pattern. Services are
// referenced by a typed token (ServiceRef[T]) that carries no value — the
// container resolves the dependency graph topologically and injects
// instances. Factories declare their dependencies via a deps map, exactly
// like @backstage/backend-plugin-api.
//
// Intent: replace the hardcoded, hand-wired engine composition in
// internal/bootstrap.Compose with a declarative graph where each engine
// simply declares the services it needs, and the runtime resolves the
// order. Mirror of Backstage Pattern #2 (ServiceRef DI Container).
package di

import (
	"fmt"
	"sort"
)

// Scope controls the lifetime of a service instance.
type Scope int

const (
	// ScopeRoot yields one instance shared across the whole container.
	ScopeRoot Scope = iota
	// ScopePlugin yields one instance per requesting "plugin" (context).
	// Not yet widely used by Cosca engines; reserved for per-runtime scopes.
	ScopePlugin
)

// ServiceRef is a typed, value-less token that names a service. The type
// parameter T is a Type Carrier: it exists only so callers get a typed
// Resolve outcome, never a value. The id string is the actual key.
type ServiceRef[T any] struct {
	ID    string
	Scope Scope
}

// Ref constructs a ServiceRef token.
func Ref[T any](id string) ServiceRef[T] {
	return ServiceRef[T]{ID: id, Scope: ScopeRoot}
}

// RefScoped constructs a ServiceRef with an explicit scope.
func RefScoped[T any](id string, scope Scope) ServiceRef[T] {
	return ServiceRef[T]{ID: id, Scope: scope}
}

// Provider function signature accepted by Register. dep is the resolved
// dependency map keyed by dependency id; it mirrors the Backstage deps map.
type Provider func(*Container) (any, error)

// Definition is a registered service: how to build it and what it needs.
type Definition struct {
	// ID is the service name (e.g. "knowledge.Engine").
	ID string
	// Scope controls lifetime sharing.
	Scope Scope
	// Deps lists the IDs this service needs before it can be built.
	Deps []string
	// Build creates the instance given the resolved container.
	Build func(*Container) (any, error)
}

// Container is a graph of service definitions that resolves instances
// lazily, memoizes them per scope, and detects dependency cycles.
type Container struct {
	defs    map[string]Definition
	built   map[string]any
	building map[string]bool
	order   []string
}

// New returns an empty container.
func New() *Container {
	return &Container{
		defs:     make(map[string]Definition),
		built:    make(map[string]any),
		building: make(map[string]bool),
	}
}

// Register adds (or replaces) a definition.
func (c *Container) Register(def Definition) {
	c.defs[def.ID] = def
	c.built = make(map[string]any) // invalidate memoization on change
	c.order = nil
}

// RegisterRef adds a definition from a typed ref.
func RegisterRef[T any](c *Container, id string, build func(*Container) (T, error)) {
	c.Register(Definition{
		ID:    id,
		Scope: ScopeRoot,
		Build: func(c *Container) (any, error) { return build(c) },
	})
}

// Has reports whether a service is registered.
func (c *Container) Has(id string) bool {
	_, ok := c.defs[id]
	return ok
}

// Resolve returns a typed instance for the referenced service, building it
// (and its dependencies) on demand.
func Resolve[T any](c *Container, ref ServiceRef[T]) (T, error) {
	var zero T
	if ref.ID == "" {
		return zero, fmt.Errorf("di: service ref has empty id")
	}
	v, err := c.resolveID(ref.ID, ref.Scope)
	if err != nil {
		return zero, err
	}
	typed, ok := v.(T)
	if !ok {
		return zero, fmt.Errorf("di: service %q resolved to %T, want %T", ref.ID, v, zero)
	}
	return typed, nil
}

// resolveID builds the instance for a single id, memoizing per scope.
func (c *Container) resolveID(id string, scope Scope) (any, error) {
	def, ok := c.defs[id]
	if !ok {
		return nil, fmt.Errorf("di: unregistered service %q", id)
	}

	// Scope root → memoize once. Plugin scope → always rebuild (per-caller).
	memKey := id
	if scope == ScopePlugin {
		memKey = id + "#plugin"
	}
	if cached, ok := c.built[memKey]; ok {
		return cached, nil
	}

	if c.building[id] {
		return nil, fmt.Errorf("di: dependency cycle detected at %q", id)
	}
	c.building[id] = true
	defer delete(c.building, id)

	// Build dependencies in the declared order.
	if _, err := c.ensureDeps(def.Deps); err != nil {
		return nil, fmt.Errorf("di: resolving deps of %q: %w", id, err)
	}

	inst, err := def.Build(c)
	if err != nil {
		return nil, fmt.Errorf("di: building %q: %w", id, err)
	}
	c.built[memKey] = inst
	return inst, nil
}

// ensureDeps resolves every listed dependency (triggering builds).
func (c *Container) ensureDeps(deps []string) ([]any, error) {
	out := make([]any, 0, len(deps))
	for _, dep := range deps {
		v, err := c.resolveID(dep, ScopeRoot)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// Dependency returns one resolved dependency by id (for Build closures).
func (c *Container) Dependency(id string) (any, error) {
	return c.resolveID(id, ScopeRoot)
}

// TopologicalOrder returns the build order (dependencies before dependents).
// Deterministic: ties are broken by insertion order, not map iteration.
func (c *Container) TopologicalOrder() ([]string, error) {
	if c.order != nil {
		return c.order, nil
	}
	// Build the dependency graph and Kahn's algorithm.
	inDegree := make(map[string]int)
	adj := make(map[string][]string)
	for id, def := range c.defs {
		for _, dep := range def.Deps {
			adj[dep] = append(adj[dep], id)
			inDegree[id]++
		}
		if _, ok := inDegree[id]; !ok {
			inDegree[id] = 0
		}
	}
	// Deterministic start: sorted list of zero-indegree ids.
	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)
	var order []string
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		order = append(order, id)
		for _, next := range adj[id] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
				sort.Strings(queue)
			}
		}
	}
	if len(order) != len(c.defs) {
		return nil, fmt.Errorf("di: cycle detected (%d of %d services ordered)", len(order), len(c.defs))
	}
	c.order = order
	return order, nil
}

// String reports the registered services for logging/debugging.
func (c *Container) String() string {
	keys := make([]string, 0, len(c.defs))
	for id := range c.defs {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	return fmt.Sprintf("di.Container{%v}", keys)
}
