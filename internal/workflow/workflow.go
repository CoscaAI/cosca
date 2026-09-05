// Package workflow provides a typed-routing workflow/pipeline engine for
// Cosca, adapted from Google's ADK-Go workflow model (Apache-2.0).
//
// The engine is value-based: every Node returns a Route value from Run, and
// the Workflow follows the first outgoing Edge whose Route matches that value.
// This is the inverse of the classic task-ID / DependsOn model in
// internal/pipeline — a node advertises its outcome ("ok", "retry", 1, true)
// and the graph decides the next node from that outcome rather than from a
// pre-wired task ID.
//
// The package is intentionally dependency-light: ToolNode and AgentNode use
// simple func contracts so the engine never needs to import internal/chat.
package workflow

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"
)

// Sentinel errors returned by the engine. Use errors.Is to test for them.
var (
	// ErrNoRouteMatch is returned when a node's returned Route matches no
	// outgoing edge and no default route is configured.
	ErrNoRouteMatch = errors.New("no route matches value")
	// ErrCycle is returned when a node executes twice on a single run.
	ErrCycle = errors.New("cycle detected")
	// ErrBranchFailed is returned when a JoinNode branch fails.
	ErrBranchFailed = errors.New("branch failed")
)

// Route is the routing condition carried by an Edge. A Node returns a Route
// value from Run and the engine follows the first Edge whose Route matches.
// Concrete implementations: StringRoute, IntRoute, BoolRoute, MultiRoute.
type Route interface{}

// StringRoute routes on an exact string value, e.g. "success", "retry", "ok".
type StringRoute string

// IntRoute routes on an exact integer value, e.g. 0, 1, 2.
type IntRoute int

// BoolRoute routes on a boolean value, e.g. true/false.
type BoolRoute bool

// MultiRoute routes when the returned value is any element of the set.
type MultiRoute[T comparable] []T

// Node is a unit of work in the workflow.
type Node interface {
	// Name returns the node's unique identifier within the workflow. The
	// engine rejects duplicate names.
	Name() string

	// Run executes the node and returns a routing value plus optional output.
	// The routing value is compared against the node's outgoing Edges to pick
	// the next node. A nil Route makes the node a sink unless an outgoing edge
	// carries a nil Route (an always-match fallback edge).
	Run(ctx context.Context, input map[string]any) (Route, map[string]any, error)
}

// Edge connects two nodes with a routing condition. When Route is nil the
// edge is an always-match fallback, followed only when no other outgoing edge
// matches the returned Route.
type Edge struct {
	From  Node
	To    Node
	Route Route
}

// Option configures a Workflow at construction time.
type Option func(*Workflow)

// WithMaxConcurrency caps the number of JoinNode branches that may execute in
// parallel. A value <= 0 means JoinNode runs its branches sequentially when
// Concurrent is false, or with no explicit cap when Concurrent is true.
func WithMaxConcurrency(n int) Option {
	return func(w *Workflow) { w.maxConcurrency = n }
}

// WithDefaultRoute sets the fallback node used when no outgoing Edge matches a
// node's returned Route. Without it, an unmatched route is an error.
func WithDefaultRoute(to Node) Option {
	return func(w *Workflow) { w.defaultRoute = to }
}

// Workflow is a directed graph of nodes connected by routed edges.
type Workflow struct {
	name           string
	edges          []Edge
	maxConcurrency int
	defaultRoute   Node

	byName    map[string]Node
	outEdges  map[string][]Edge
	nodeNames []string
	sources   []Node
}

// New validates the edges and builds a runnable Workflow. Nodes are derived
// from the edges; every From/To must be non-nil, carry a non-empty unique
// name, and reference a runnable Node implementation.
func New(name string, edges []Edge, opts ...Option) (*Workflow, error) {
	w := &Workflow{name: name, edges: edges}
	for _, opt := range opts {
		opt(w)
	}
	if err := w.build(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *Workflow) build() error {
	w.byName = make(map[string]Node)
	w.outEdges = make(map[string][]Edge)

	for i, e := range w.edges {
		if e.From == nil || e.To == nil {
			return fmt.Errorf("workflow %q: edge %d has nil From or To", w.name, i)
		}
		fromName, toName := e.From.Name(), e.To.Name()
		if fromName == "" || toName == "" {
			return fmt.Errorf("workflow %q: edge %d has a node with an empty name", w.name, i)
		}
		if err := w.addNode(e.From); err != nil {
			return err
		}
		if err := w.addNode(e.To); err != nil {
			return err
		}
		w.outEdges[fromName] = append(w.outEdges[fromName], e)
	}

	for _, n := range w.byName {
		if err := validateNode(n); err != nil {
			return fmt.Errorf("workflow %q: %w", w.name, err)
		}
	}

	if w.defaultRoute != nil {
		if err := validateNode(w.defaultRoute); err != nil {
			return fmt.Errorf("workflow %q: default route: %w", w.name, err)
		}
		// The default route is a fallback target, not a source: when it does
		// not already participate in an edge it is registered for lookup only.
		if _, ok := w.byName[w.defaultRoute.Name()]; !ok {
			if err := w.addLookupNode(w.defaultRoute); err != nil {
				return err
			}
		}
	}

	// Sources are edge-participating nodes that never appear as the To of a
	// non-self edge. A self-loop (From == To) is still a valid source so its
	// re-entry is caught at runtime by cycle detection rather than at build.
	hasIncoming := make(map[string]bool)
	for _, e := range w.edges {
		if sameNode(e.From, e.To) {
			continue
		}
		hasIncoming[e.To.Name()] = true
	}
	for _, name := range w.nodeNames {
		if !hasIncoming[name] {
			w.sources = append(w.sources, w.byName[name])
		}
	}
	sort.Slice(w.sources, func(i, j int) bool {
		return w.sources[i].Name() < w.sources[j].Name()
	})

	if len(w.nodeNames) > 0 && len(w.sources) == 0 {
		return fmt.Errorf("workflow %q: no entry point (every node has an incoming edge; possible cycle)", w.name)
	}
	return nil
}

// addNode registers a node that participates in the edge graph, rejecting a
// different node that reuses an existing name.
func (w *Workflow) addNode(n Node) error {
	if existing, ok := w.byName[n.Name()]; ok {
		if !sameNode(existing, n) {
			return fmt.Errorf("workflow %q: duplicate node name %q", w.name, n.Name())
		}
		return nil
	}
	w.byName[n.Name()] = n
	w.nodeNames = append(w.nodeNames, n.Name())
	return nil
}

// addLookupNode registers a node in the lookup map only (e.g. a default route
// that is not part of the edge graph). It never becomes a source.
func (w *Workflow) addLookupNode(n Node) error {
	if existing, ok := w.byName[n.Name()]; ok {
		if !sameNode(existing, n) {
			return fmt.Errorf("workflow %q: duplicate node name %q", w.name, n.Name())
		}
		return nil
	}
	w.byName[n.Name()] = n
	return nil
}

// Name returns the workflow name.
func (w *Workflow) Name() string { return w.name }

// Nodes returns the distinct nodes of the workflow in first-seen order.
func (w *Workflow) Nodes() []Node {
	out := make([]Node, 0, len(w.nodeNames))
	for _, name := range w.nodeNames {
		out = append(out, w.byName[name])
	}
	return out
}

// Edges returns the raw edge list the workflow was built from.
func (w *Workflow) Edges() []Edge {
	out := make([]Edge, len(w.edges))
	copy(out, w.edges)
	return out
}

// Run executes the workflow. Execution starts at every source node (nodes with
// no incoming edges) and follows routed edges until a sink node (no outgoing
// edges) or an error. initial seeds the starting state and is copied, so the
// caller's map is never mutated. The returned map is the merged output of
// every executed node, keyed by node name (or by the keys of map-valued
// outputs).
func (w *Workflow) Run(ctx context.Context, initial map[string]any) (map[string]any, error) {
	output := cloneMap(initial)
	if len(w.sources) == 0 {
		return output, nil
	}

	st := &runState{
		visited:        make(map[string]bool),
		maxConcurrency: w.maxConcurrency,
	}
	ctx = context.WithValue(ctx, runStateKey{}, st)

	for _, src := range w.sources {
		if err := ctx.Err(); err != nil {
			return output, err
		}
		if err := w.runPath(ctx, st, src.Name(), output); err != nil {
			return output, err
		}
		// Cycle detection is per-path: a node may legitimately appear on more
		// than one source path (DAG fan-in), but never twice on the same one.
		st.reset()
	}
	return output, nil
}

// runPath executes a single path from a node until a sink or an error. The
// visited set guarantees a node cannot execute twice on one path, which turns
// self-loops and longer cycles into a hard error instead of an infinite loop.
func (w *Workflow) runPath(ctx context.Context, st *runState, name string, output map[string]any) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := st.markExecuted(name); err != nil {
			return fmt.Errorf("workflow %q: %w", w.name, err)
		}

		node := w.byName[name]
		route, out, err := node.Run(ctx, cloneMap(output))
		if err != nil {
			return fmt.Errorf("workflow %q: node %q: %w", w.name, node.Name(), err)
		}
		mergeOutput(output, out)

		next, err := w.next(node, route)
		if err != nil {
			return err
		}
		if next == nil {
			return nil
		}
		name = next.Name()
	}
}

// next resolves the next node for a returned route following ADK-style typed
// matching. It tries explicit route matches first, then a nil-route edge, then
// the default route; an unmatched route is an error when none exist.
func (w *Workflow) next(node Node, route Route) (Node, error) {
	out := w.outEdges[node.Name()]
	if len(out) == 0 {
		return nil, nil // sink
	}
	for _, e := range out {
		if e.Route == nil {
			continue
		}
		if routeMatches(route, e.Route) {
			return e.To, nil
		}
	}
	for _, e := range out {
		if e.Route == nil {
			return e.To, nil
		}
	}
	if w.defaultRoute != nil {
		return w.defaultRoute, nil
	}
	return nil, fmt.Errorf("%w: %v for node %q", ErrNoRouteMatch, route, node.Name())
}

// runState carries per-run execution state threaded through the context so a
// JoinNode can coordinate branch concurrency and share the visited set.
type runState struct {
	mu             sync.Mutex
	visited        map[string]bool
	maxConcurrency int
}

type runStateKey struct{}

func stateFrom(ctx context.Context) *runState {
	st, _ := ctx.Value(runStateKey{}).(*runState)
	return st
}

// markExecuted records a node as executed on the current path, returning an
// error when it already ran (cycle). Safe for concurrent callers.
func (st *runState) markExecuted(name string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.visited[name] {
		return fmt.Errorf("%w: node %q already executed", ErrCycle, name)
	}
	st.visited[name] = true
	return nil
}

// reset clears the visited set between source paths.
func (st *runState) reset() {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.visited = make(map[string]bool)
}

// routeMatches reports whether a returned Route satisfies an Edge's Route.
// Values compare semantically, so a node may return either a typed Route
// (StringRoute("ok")) or the bare Go value ("ok").
func routeMatches(returned, edge any) bool {
	if edge == nil {
		return true
	}
	if returned == nil {
		return false
	}
	ev := reflect.ValueOf(edge)
	if ev.Kind() == reflect.Slice {
		// MultiRoute[T]: match when the returned value equals any element.
		for i := 0; i < ev.Len(); i++ {
			if valuesEqual(returned, ev.Index(i).Interface()) {
				return true
			}
		}
		return false
	}
	return valuesEqual(returned, edge)
}

func valuesEqual(a, b any) bool {
	av, bv := reflect.ValueOf(a), reflect.ValueOf(b)
	if !av.IsValid() || !bv.IsValid() {
		return false
	}
	if av.Type() == bv.Type() {
		return reflect.DeepEqual(a, b)
	}
	if av.Kind() != bv.Kind() {
		return false
	}
	switch av.Kind() {
	case reflect.String:
		return av.String() == bv.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return av.Int() == bv.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return av.Uint() == bv.Uint()
	case reflect.Bool:
		return av.Bool() == bv.Bool()
	}
	return false
}

// sameNode reports whether two Node values refer to the same node without
// panicking on non-comparable struct values.
func sameNode(a, b Node) bool {
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	if !va.IsValid() || !vb.IsValid() {
		return false
	}
	if va.Type() != vb.Type() {
		return false
	}
	switch va.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan, reflect.UnsafePointer:
		return va.Pointer() == vb.Pointer()
	}
	return reflect.DeepEqual(a, b)
}

func cloneMap(m map[string]any) map[string]any {
	cp := make(map[string]any, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

func mergeOutput(dst, src map[string]any) {
	for k, v := range src {
		dst[k] = v
	}
}
