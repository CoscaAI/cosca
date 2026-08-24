package modlink

import (
	"errors"
	"reflect"
	"testing"
)

// testRoutes is the fixture route set for the urban-tree world-model domain
// plus a GIS route and a programming route (used to prove isolation). It
// mirrors ADR-013 §3.2: "árvore urbana no terreno" routes to world
// (entity), vegetation (generate_tree), unreal (actor) and materials
// (ground_integration) — never to programming/gis.
func testRoutes() []Route {
	return []Route{
		{Trigger: "árvore no terreno", Module: "world", Capability: "world.entity", Priority: 10, Conditions: []string{"spatial"}},
		{Trigger: "árvore no terreno", Module: "vegetation", Capability: "vegetation.generate_tree", Priority: 20, Conditions: []string{"terrain"}},
		{Trigger: "árvore urbana", Module: "unreal", Capability: "unreal.actor", Priority: 15, Conditions: []string{"asset"}},
		{Trigger: "árvore no terreno", Module: "materials", Capability: "materials.ground_integration", Priority: 5, Conditions: []string{"foliage"}},
		{Trigger: "território gis", Module: "gis", Capability: "gis.spatial_index", Priority: 5, Conditions: []string{"map"}},
		{Trigger: "implementar função", Module: "programming", Capability: "programming.code_gen", Priority: 10, Conditions: []string{"code"}},
	}
}

// testRoutesReversed returns the same routes in reverse registration order, to
// prove order-independence of the canonical result.
func testRoutesReversed() []Route {
	r := testRoutes()
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return r
}

func scopeModules(scope *SearchScope) map[string]bool {
	set := make(map[string]bool, len(scope.Modules))
	for _, m := range scope.Modules {
		set[m] = true
	}
	return set
}

// TestResolveRoutePositive — "árvore urbana no terreno" must route to
// {world, vegetation, unreal, materials} (and related capabilities), matching
// the ADR example exactly, in canonical (sorted) order.
func TestResolveRoutePositive(t *testing.T) {
	r := NewResolver(testRoutes())

	scope, err := r.ResolveRoute("árvore urbana no terreno")
	if err != nil {
		t.Fatalf("ResolveRoute: unexpected error %v", err)
	}
	if scope.NoRoute {
		t.Fatalf("ResolveRoute: unexpected NoRoute=true for a known query")
	}

	// Canonical (sorted) module order — proves determinism of the ordering.
	wantModules := []string{"materials", "unreal", "vegetation", "world"}
	if !reflect.DeepEqual(scope.Modules, wantModules) {
		t.Fatalf("Modules = %v, want %v (canonical sorted order)", scope.Modules, wantModules)
	}

	m := scopeModules(scope)
	for _, mod := range []string{"world", "vegetation", "unreal", "materials"} {
		if !m[mod] {
			t.Fatalf("Modules missing %q, got %v", mod, scope.Modules)
		}
	}

	// Related capabilities are present.
	wantCaps := []string{
		"materials.ground_integration",
		"unreal.actor",
		"vegetation.generate_tree",
		"world.entity",
	}
	if !reflect.DeepEqual(scope.Capabilities, wantCaps) {
		t.Fatalf("Capabilities = %v, want %v", scope.Capabilities, wantCaps)
	}

	if scope.Fingerprint == "" {
		t.Fatalf("Fingerprint must be populated")
	}
}

// TestResolveRouteDeterminism — the same query run N times yields the identical
// *SearchScope, including canonical order and fingerprint.
func TestResolveRouteDeterminism(t *testing.T) {
	r := NewResolver(testRoutes())

	first, err := r.ResolveRoute("árvore urbana no terreno")
	if err != nil {
		t.Fatalf("ResolveRoute: unexpected error %v", err)
	}

	for i := 1; i <= 3; i++ {
		next, err := r.ResolveRoute("árvore urbana no terreno")
		if err != nil {
			t.Fatalf("iteration %d: unexpected error %v", i, err)
		}
		if !reflect.DeepEqual(first, next) {
			t.Fatalf("iteration %d: scope differs from first\n first=%#v\n next =%#v", i, first, next)
		}
	}
}

// TestResolveRouteOrderIndependent — registering the routes in different orders
// must produce an identical canonical scope (modules, capabilities, conditions,
// priority, RouteID, fingerprint).
func TestResolveRouteOrderIndependent(t *testing.T) {
	normal := NewResolver(testRoutes())
	reversed := NewResolver(testRoutesReversed())

	q := "árvore urbana no terreno"
	a, err := normal.ResolveRoute(q)
	if err != nil {
		t.Fatalf("normal: unexpected error %v", err)
	}
	b, err := reversed.ResolveRoute(q)
	if err != nil {
		t.Fatalf("reversed: unexpected error %v", err)
	}

	if !reflect.DeepEqual(a, b) {
		t.Fatalf("order-independent scope mismatch\n normal=%#v\n rev  =%#v", a, b)
	}
	if a.RouteID != b.RouteID || a.Fingerprint != b.Fingerprint {
		t.Fatalf("RouteID/Fingerprint mismatch:\n %q / %q\n %q / %q", a.RouteID, b.RouteID, a.Fingerprint, b.Fingerprint)
	}
}

// TestResolveRouteIsolation — "como implementar uma função Go" must route to
// programming and MUST NOT open vegetation, gis, unreal or materials, even
// though trigger words from other domains are "semantically close" in intent.
func TestResolveRouteIsolation(t *testing.T) {
	r := NewResolver(testRoutes())

	scope, err := r.ResolveRoute("como implementar uma função Go")
	if err != nil {
		t.Fatalf("ResolveRoute: unexpected error %v", err)
	}

	m := scopeModules(scope)
	for _, forbidden := range []string{"vegetation", "gis", "unreal", "materials"} {
		if m[forbidden] {
			t.Fatalf("isolation violated: query opened forbidden module %q, got %v", forbidden, scope.Modules)
		}
	}
	if !m["programming"] {
		t.Fatalf("expected routing to programming, got %v", scope.Modules)
	}
	if len(scope.Modules) != 1 {
		t.Fatalf("expected exactly [programming], got %v", scope.Modules)
	}
}

// TestResolveRouteNoRoute — a query with no known route returns an explicit
// NO_ROUTE state, never a silent fallback to searching the whole universe.
func TestResolveRouteNoRoute(t *testing.T) {
	r := NewResolver(testRoutes())

	_, err := r.ResolveRoute("quero ouvir música eletrônica")
	if !errors.Is(err, ErrNoRoute) {
		t.Fatalf("ResolveRoute: err = %v, want ErrNoRoute", err)
	}

	// Non-error variant must also be explicit, not a fallback to all modules.
	scope := r.Resolve("quero ouvir música eletrônica")
	if !scope.NoRoute {
		t.Fatalf("Resolve: expected NoRoute=true, got %+v", scope)
	}
	if len(scope.Modules) != 0 || len(scope.Capabilities) != 0 {
		t.Fatalf("Resolve: NO_ROUTE scope must have empty Modules/Capabilities, got %+v", scope)
	}
	if scope.Fingerprint == "" {
		t.Fatalf("Resolve: NO_ROUTE scope must carry a Fingerprint")
	}
}
