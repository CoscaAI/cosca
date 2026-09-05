// Package modlink implements the deterministic route resolver from ADR-013
// §3.2 — the ROUTER half of the modular knowledge architecture.
//
//	ROUTER = determines the search space; SEMANTIC SEARCH = searches inside it.
//
// This package ONLY does deterministic routing. It never embeds text, never
// queries a vector store, never calls an LLM, and never scores semantic
// similarity. Given a query, it resolves — by pure, closed-form rules — which
// modules (domains) and capabilities form the BOUNDED search space that the
// semantic search must then be confined to. The router decides the space; the
// search refines inside it, and never chooses the space.
//
// Determinism contract (the golden rules):
//
//   - DETERMINISTIC — the same query always yields the same *SearchScope:
//     identical module order, capabilities, RouteID and Fingerprint. Order of
//     modules is canonical (sorted), independent of the order the routes were
//     registered (ORDER-INDEPENDENT).
//
//   - PRECISE / ISOLATED — a route matches only when EVERY canonical word of
//     its trigger appears as a whole word in the query (whole-word token
//     containment). There is no substring, fuzzy, prefix or semantic matching.
//     This is what keeps domains isolated: a query about Go programming never
//     opens vegetation/gis/unreal/materials just because some of those triggers
//     mention a "semantically similar" word. The trigger match is exact.
//
//   - NO SILENT FALLBACK — a query with no known route never falls through to
//     "search the whole universe". ResolveRoute returns ErrNoRoute; Resolve
//     returns a *SearchScope with NoRoute=true (explicit NO_ROUTE state).
package modlink

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Route is a single declarative routing rule (ADR-013 §3.0 `route` row): a
// deterministic trigger maps a query to the module that OWNS a capability and
// to the capability itself. One Route carries exactly one module/capability;
// a resolver aggregates many matching routes into a single SearchScope.
//
// Trigger semantic: a route applies when every canonical whole-word token of
// Trigger is present in the query. It is the immutable hook the router uses —
// it "knows what exists, where, and for which capability" without ever loading
// module content (zero-content invariant, §3.0/F).
type Route struct {
	// Trigger is the deterministic trigger phrase. Whole-word token match
	// (case- and accent-insensitive). Never matched by substring.
	Trigger string
	// Module is the domain that owns the capability (level 2, e.g. "vegetation").
	Module string
	// Capability is the logical function within the module (level 3, e.g.
	// "vegetation.generate_tree"). It is logic, not a database.
	Capability string
	// Priority is the route precedence. When scopes aggregate, the highest
	// priority among the matched routes is kept (deterministic max).
	Priority int
	// Conditions are contextual conditions/context required for the route to
	// be applicable. Aggregated (deduped, sorted) across matched routes.
	Conditions []string
}

// SearchScope is the bounded search space produced by deterministic routing.
// It is the OUTPUT of the router and the INPUT of the semantic search: the
// search must be confined to Modules/Capabilities and never exceed them.
type SearchScope struct {
	// Modules is the sorted, deduped set of domain modules selected by the
	// router. Canonical (ascending) order.
	Modules []string
	// Capabilities is the sorted, deduped set of capability IDs selected.
	Capabilities []string
	// RouteID deterministically identifies the matched route set (sorted,
	// unique triggers joined by "|"). Empty when NoRoute is true.
	RouteID string
	// Priority is the highest Priority among the matched routes (0 when none).
	Priority int
	// Conditions is the sorted, deduped union of conditions of matched routes.
	Conditions []string
	// Fingerprint is a canonical SHA-256 of the scope's deterministic
	// serialization. Equal scopes always share a Fingerprint.
	Fingerprint string
	// NoRoute is the explicit NO_ROUTE state: true means the router found no
	// known route for the query. When true, Modules/Capabilities are empty and
	// there is NO silent fallback to "search everything".
	NoRoute bool
}

// ErrNoRoute is returned by ResolveRoute when the query matches no known route.
// It is never silently swallowed: callers must handle the explicit NO_ROUTE
// state rather than falling back to unbounded search.
var ErrNoRoute = errors.New("modlink: no route matches query")

// Resolver is the deterministic router. It holds an immutable snapshot of the
// declared routes (pre-normalized at construction) and resolves queries against
// them. It is safe for concurrent read-only use.
type Resolver struct {
	entries []routeEntry
}

// routeEntry is a pre-normalized route ready for fast, deterministic matching.
type routeEntry struct {
	route   Route
	trigger []string // canonical, deduped trigger tokens
}

// NewResolver builds a Resolver from a slice of declared routes. Routes with a
// trigger that canonicalizes to zero tokens (empty/whitespace-only) are skipped:
// they can never be matched deterministically and must not match everything.
func NewResolver(routes []Route) *Resolver {
	r := &Resolver{entries: make([]routeEntry, 0, len(routes))}
	for _, rt := range routes {
		trig := canonicalTokens(rt.Trigger)
		if len(trig) == 0 {
			continue
		}
		r.entries = append(r.entries, routeEntry{route: rt, trigger: dedup(trig)})
	}
	return r
}

// ResolveRoute deterministically resolves a query to a bounded *SearchScope.
//
// The matching is whole-word token containment: a route applies iff every
// canonical token of its trigger appears as a whole word in the query. Matching
// routes are aggregated (modules, capabilities, conditions), canonicalized
// (sorted, deduped), fingerprinted, and returned.
//
// Returns ErrNoRoute — never a fallback — when no known route matches.
func (r *Resolver) ResolveRoute(query string) (*SearchScope, error) {
	qTokens := canonicalTokens(query)
	if len(qTokens) == 0 {
		return nil, ErrNoRoute
	}

	matched := r.match(qTokens)
	if len(matched) == 0 {
		return nil, ErrNoRoute
	}

	return r.buildScope(matched), nil
}

// Resolve is the non-error convenience form of ResolveRoute. It returns a
// *SearchScope always. For a query with no known route it returns a scope with
// NoRoute=true (and empty Modules/Capabilities) — never a silent fallback to
// "search everything".
func (r *Resolver) Resolve(query string) *SearchScope {
	scope, err := r.ResolveRoute(query)
	if err != nil {
		if errors.Is(err, ErrNoRoute) {
			s := &SearchScope{NoRoute: true}
			s.Fingerprint = s.ComputeFingerprint()
			return s
		}
		s := &SearchScope{NoRoute: true}
		s.Fingerprint = s.ComputeFingerprint()
		return s
	}
	return scope
}

// match returns the route entries whose trigger tokens are all contained in
// the query tokens. Deterministic: it only depends on the query and the route
// data, never on registration order of the result.
func (r *Resolver) match(qTokens []string) []routeEntry {
	var matched []routeEntry
	for _, e := range r.entries {
		if containsAll(qTokens, e.trigger) {
			matched = append(matched, e)
		}
	}
	return matched
}

// buildScope aggregates a set of matched routes into a single canonical scope:
// modules/capabilities/conditions are deduped and sorted; Priority is the max;
// RouteID is the sorted unique triggers joined by "|". The Fingerprint is
// computed last from the canonical fields.
func (r *Resolver) buildScope(matched []routeEntry) *SearchScope {
	moduleSet := map[string]struct{}{}
	capSet := map[string]struct{}{}
	condSet := map[string]struct{}{}
	triggerSet := map[string]struct{}{}
	priority := 0

	for _, e := range matched {
		if e.route.Module != "" {
			moduleSet[e.route.Module] = struct{}{}
		}
		if e.route.Capability != "" {
			capSet[e.route.Capability] = struct{}{}
		}
		for _, c := range e.route.Conditions {
			if c != "" {
				condSet[c] = struct{}{}
			}
		}
		triggerSet[e.route.Trigger] = struct{}{}
		if e.route.Priority > priority {
			priority = e.route.Priority
		}
	}

	scope := &SearchScope{
		Modules:      sortedKeys(moduleSet),
		Capabilities: sortedKeys(capSet),
		RouteID:      strings.Join(sortedKeys(triggerSet), "|"),
		Priority:     priority,
		Conditions:   sortedKeys(condSet),
	}
	scope.Fingerprint = scope.ComputeFingerprint()
	return scope
}

// ComputeFingerprint returns the canonical SHA-256 (hex) of the scope's
// deterministic serialization. It is a pure function of the scope fields, so
// equal scopes always produce an equal Fingerprint. The serialization includes
// the NoRoute flag to keep a NO_ROUTE state distinguishable from an (impossible)
// empty routed scope.
func (s *SearchScope) ComputeFingerprint() string {
	var b strings.Builder
	b.WriteString("modules:")
	b.WriteString(strings.Join(s.Modules, ","))
	b.WriteString(";caps:")
	b.WriteString(strings.Join(s.Capabilities, ","))
	b.WriteString(";route:")
	b.WriteString(s.RouteID)
	fmt.Fprintf(&b, ";priority:%d", s.Priority)
	b.WriteString(";conditions:")
	b.WriteString(strings.Join(s.Conditions, ","))
	if s.NoRoute {
		b.WriteString(";noroute:true")
	}
	sum := sha256.Sum256([]byte(b.String()))
	return fmt.Sprintf("%x", sum)
}

// sortedKeys returns the keys of a string-set, sorted ascending. This is the
// canonicalization that makes the scope order independent of registration.
func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
