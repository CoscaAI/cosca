package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// helperNode is a configurable node used across tests.
type helperNode struct {
	name   string
	fn     func(ctx context.Context, input map[string]any) (Route, any, error)
	path   *[]string
	pathMu *sync.Mutex
}

func (h *helperNode) Name() string { return h.name }

func (h *helperNode) Run(ctx context.Context, input map[string]any) (Route, map[string]any, error) {
	if h.path != nil && h.pathMu != nil {
		h.pathMu.Lock()
		*h.path = append(*h.path, h.name)
		h.pathMu.Unlock()
	}
	return runCore(ctx, h.name, h.fn, input)
}

func record(name string, path *[]string, mu *sync.Mutex) *helperNode {
	return &helperNode{name: name, path: path, pathMu: mu}
}

func mustNew(t *testing.T, name string, edges []Edge, opts ...Option) *Workflow {
	t.Helper()
	w, err := New(name, edges, opts...)
	if err != nil {
		t.Fatalf("New(%q): %v", name, err)
	}
	return w
}

func TestWorkflow_StringRoute(t *testing.T) {
	var mu sync.Mutex
	var path []string

	a := record("a", &path, &mu)
	a.fn = func(_ context.Context, _ map[string]any) (Route, any, error) {
		return StringRoute("ok"), "from-a", nil
	}
	b := record("b", &path, &mu)
	b.fn = func(_ context.Context, _ map[string]any) (Route, any, error) {
		return nil, "from-b", nil
	}

	w := mustNew(t, "string-route", []Edge{{From: a, To: b, Route: StringRoute("ok")}})
	out, err := w.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if fmt.Sprint(path) != "[a b]" {
		t.Errorf("expected path [a b], got %v", path)
	}
	if out["a"] != "from-a" || out["b"] != "from-b" {
		t.Errorf("unexpected outputs: %v", out)
	}
}

func TestWorkflow_StringRoute_NoMatch(t *testing.T) {
	var mu sync.Mutex
	var path []string

	a := record("a", &path, &mu)
	a.fn = func(_ context.Context, _ map[string]any) (Route, any, error) {
		return StringRoute("nope"), nil, nil
	}
	b := record("b", &path, &mu)
	b.fn = func(_ context.Context, _ map[string]any) (Route, any, error) { return nil, nil, nil }

	w := mustNew(t, "string-route-nomatch", []Edge{{From: a, To: b, Route: StringRoute("ok")}})
	if _, err := w.Run(context.Background(), nil); err == nil {
		t.Fatal("expected routing error")
	} else if !errors.Is(err, ErrNoRouteMatch) {
		t.Fatalf("expected no-route error, got %v", err)
	}
}

func TestWorkflow_IntRoute(t *testing.T) {
	var mu sync.Mutex
	var path []string

	a := record("a", &path, &mu)
	a.fn = func(_ context.Context, _ map[string]any) (Route, any, error) {
		return IntRoute(1), nil, nil
	}
	b := record("b", &path, &mu)
	b.fn = func(_ context.Context, _ map[string]any) (Route, any, error) { return nil, nil, nil }

	w := mustNew(t, "int-route", []Edge{{From: a, To: b, Route: IntRoute(1)}})
	if _, err := w.Run(context.Background(), nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if fmt.Sprint(path) != "[a b]" {
		t.Errorf("expected path [a b], got %v", path)
	}
}

func TestWorkflow_IntRoute_BareValue(t *testing.T) {
	// A node may return a bare int instead of an IntRoute.
	var mu sync.Mutex
	var path []string

	a := record("a", &path, &mu)
	a.fn = func(_ context.Context, _ map[string]any) (Route, any, error) {
		return 2, nil, nil
	}
	b := record("b", &path, &mu)
	b.fn = func(_ context.Context, _ map[string]any) (Route, any, error) { return nil, nil, nil }

	w := mustNew(t, "int-route-bare", []Edge{{From: a, To: b, Route: IntRoute(2)}})
	if _, err := w.Run(context.Background(), nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if fmt.Sprint(path) != "[a b]" {
		t.Errorf("expected path [a b], got %v", path)
	}
}

func TestWorkflow_BoolRoute(t *testing.T) {
	var mu sync.Mutex
	var path []string

	trueNode := record("true-node", &path, &mu)
	trueNode.fn = func(_ context.Context, _ map[string]any) (Route, any, error) {
		return BoolRoute(true), nil, nil
	}
	reached := record("reached", &path, &mu)
	reached.fn = func(_ context.Context, _ map[string]any) (Route, any, error) { return nil, nil, nil }

	w := mustNew(t, "bool-route", []Edge{{From: trueNode, To: reached, Route: BoolRoute(true)}})
	if _, err := w.Run(context.Background(), nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if fmt.Sprint(path) != "[true-node reached]" {
		t.Errorf("expected path [true-node reached], got %v", path)
	}
}

func TestWorkflow_MultiRoute(t *testing.T) {
	var mu sync.Mutex
	var path []string

	a := record("a", &path, &mu)
	a.fn = func(_ context.Context, _ map[string]any) (Route, any, error) {
		return "yellow", nil, nil
	}
	b := record("b", &path, &mu)
	b.fn = func(_ context.Context, _ map[string]any) (Route, any, error) { return nil, nil, nil }

	w := mustNew(t, "multi-route", []Edge{{
		From: a, To: b, Route: MultiRoute[string]{"red", "yellow", "blue"},
	}})
	if _, err := w.Run(context.Background(), nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if fmt.Sprint(path) != "[a b]" {
		t.Errorf("expected path [a b], got %v", path)
	}
}

func TestWorkflow_MultiRoute_NoMatch(t *testing.T) {
	a := &helperNode{name: "a"}
	a.fn = func(_ context.Context, _ map[string]any) (Route, any, error) {
		return "green", nil, nil
	}
	b := &helperNode{name: "b"}
	b.fn = func(_ context.Context, _ map[string]any) (Route, any, error) { return nil, nil, nil }

	w := mustNew(t, "multi-route-nomatch", []Edge{{
		From: a, To: b, Route: MultiRoute[string]{"red", "yellow", "blue"},
	}})
	if _, err := w.Run(context.Background(), nil); err == nil {
		t.Fatal("expected routing error")
	}
}

func TestWorkflow_NoRouteMatch_Error(t *testing.T) {
	a := &helperNode{name: "a"}
	a.fn = func(_ context.Context, _ map[string]any) (Route, any, error) {
		return StringRoute("left"), nil, nil
	}
	b := &helperNode{name: "b"}
	b.fn = func(_ context.Context, _ map[string]any) (Route, any, error) { return nil, nil, nil }

	w := mustNew(t, "no-route", []Edge{{From: a, To: b, Route: StringRoute("right")}})
	if _, err := w.Run(context.Background(), nil); err == nil {
		t.Fatal("expected no-route error")
	}
}

func TestWorkflow_DefaultRoute_Fallback(t *testing.T) {
	var mu sync.Mutex
	var path []string

	a := record("a", &path, &mu)
	a.fn = func(_ context.Context, _ map[string]any) (Route, any, error) {
		return StringRoute("unknown"), nil, nil
	}
	b := record("b", &path, &mu)
	b.fn = func(_ context.Context, _ map[string]any) (Route, any, error) { return nil, nil, nil }
	fallback := record("fallback", &path, &mu)
	fallback.fn = func(_ context.Context, _ map[string]any) (Route, any, error) { return nil, nil, nil }

	w := mustNew(t, "default-route", []Edge{{From: a, To: b, Route: StringRoute("known")}},
		WithDefaultRoute(fallback))
	if _, err := w.Run(context.Background(), nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if fmt.Sprint(path) != "[a fallback]" {
		t.Errorf("expected path [a fallback], got %v", path)
	}
}

func TestWorkflow_JoinNode_Sequential_Merges(t *testing.T) {
	workerA := NewFunctionNode("worker-a", func(_ context.Context, _ map[string]any) (Route, any, error) {
		return nil, "a", nil
	})
	workerB := NewFunctionNode("worker-b", func(_ context.Context, _ map[string]any) (Route, any, error) {
		return nil, "b", nil
	})
	join := NewJoinNode("join", []Node{workerA, workerB})

	sink := NewFunctionNode("sink", func(_ context.Context, in map[string]any) (Route, any, error) {
		return nil, map[string]any{
			"a": in["worker-a"],
			"b": in["worker-b"],
		}, nil
	})

	w := mustNew(t, "join-seq", []Edge{{From: join, To: sink}})
	out, err := w.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out["worker-a"] != "a" || out["worker-b"] != "b" {
		t.Errorf("expected merged branch outputs, got %v", out)
	}
	if out["a"] != "a" || out["b"] != "b" {
		t.Errorf("expected join-visible branch outputs, got %v", out)
	}
}

func TestWorkflow_JoinNode_Concurrent_Merges(t *testing.T) {
	mk := func(name, out string) Node {
		return NewFunctionNode(name, func(_ context.Context, _ map[string]any) (Route, any, error) {
			time.Sleep(10 * time.Millisecond)
			return nil, out, nil
		})
	}
	join := NewJoinNode("join", []Node{mk("worker-a", "a"), mk("worker-b", "b"), mk("worker-c", "c")})
	join.Concurrent = true

	sink := NewFunctionNode("sink", func(_ context.Context, _ map[string]any) (Route, any, error) {
		return nil, nil, nil
	})
	w := mustNew(t, "join-conc", []Edge{{From: join, To: sink}})
	out, err := w.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out["worker-a"] != "a" || out["worker-b"] != "b" || out["worker-c"] != "c" {
		t.Errorf("expected all branch outputs merged, got %v", out)
	}
}

func TestWorkflow_JoinNode_BranchError(t *testing.T) {
	bad := NewFunctionNode("bad", func(_ context.Context, _ map[string]any) (Route, any, error) {
		return nil, nil, errors.New("boom")
	})
	join := NewJoinNode("join", []Node{bad})
	join.Concurrent = true
	sink := NewFunctionNode("sink", func(_ context.Context, _ map[string]any) (Route, any, error) {
		return nil, nil, nil
	})
	w := mustNew(t, "join-err", []Edge{{From: join, To: sink}})
	if _, err := w.Run(context.Background(), nil); err == nil {
		t.Fatal("expected branch error to fail the run")
	} else if !errors.Is(err, ErrBranchFailed) && !contains(err, "boom") {
		t.Fatalf("expected boom error, got %v", err)
	}
}

func TestWorkflow_CycleDetection(t *testing.T) {
	// A self-loop: node returns "again", edge routes back to itself.
	a := &helperNode{name: "a"}
	a.fn = func(_ context.Context, _ map[string]any) (Route, any, error) {
		return StringRoute("again"), nil, nil
	}

	w, err := New("cycle", []Edge{{From: a, To: a, Route: StringRoute("again")}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := w.Run(context.Background(), nil); err == nil {
		t.Fatal("expected cycle error")
	} else if !errors.Is(err, ErrCycle) {
		t.Fatalf("expected cycle error, got %v", err)
	}
}

func TestWorkflow_CycleDetection_TwoNodeCycle(t *testing.T) {
	a := &helperNode{name: "a"}
	a.fn = func(_ context.Context, _ map[string]any) (Route, any, error) { return StringRoute("b"), nil, nil }
	b := &helperNode{name: "b"}
	b.fn = func(_ context.Context, _ map[string]any) (Route, any, error) { return StringRoute("a"), nil, nil }

	// a -> b -> a is a closed cycle with no entry point: New must reject it.
	if _, err := New("two-cycle", []Edge{
		{From: a, To: b, Route: StringRoute("b")},
		{From: b, To: a, Route: StringRoute("a")},
	}); err == nil {
		t.Fatal("expected build error for closed cycle without entry point")
	}
}

func TestWorkflow_ContextCancellation(t *testing.T) {
	a := &helperNode{name: "a"}
	a.fn = func(_ context.Context, _ map[string]any) (Route, any, error) {
		return StringRoute("ok"), nil, nil
	}
	b := &helperNode{name: "b"}
	b.fn = func(ctx context.Context, _ map[string]any) (Route, any, error) {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(5 * time.Second):
			return nil, nil, nil
		}
	}

	w := mustNew(t, "cancel", []Edge{{From: a, To: b, Route: StringRoute("ok")}})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := w.Run(ctx, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context cancellation to stop execution, got %v", err)
	}
}

func TestWorkflow_ContextCancellation_BeforeRun(t *testing.T) {
	called := false
	a := NewFunctionNode("a", func(_ context.Context, _ map[string]any) (Route, any, error) {
		called = true
		return nil, nil, nil
	})
	sink := NewFunctionNode("sink", func(_ context.Context, _ map[string]any) (Route, any, error) {
		return nil, nil, nil
	})
	w := mustNew(t, "cancel-pre", []Edge{{From: a, To: sink}})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := w.Run(ctx, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if called {
		t.Error("node should not run when the context is already cancelled")
	}
}

func TestWorkflow_WithMaxConcurrency_LimitsBranches(t *testing.T) {
	const branches = 4
	const limit = 2

	var active atomic.Int32
	var maxActive atomic.Int32

	mk := func(name string) Node {
		return NewFunctionNode(name, func(_ context.Context, _ map[string]any) (Route, any, error) {
			cur := active.Add(1)
			for {
				m := maxActive.Load()
				if cur <= m || maxActive.CompareAndSwap(m, cur) {
					break
				}
			}
			time.Sleep(30 * time.Millisecond)
			active.Add(-1)
			return nil, name, nil
		})
	}

	var branchesList []Node
	for i := 0; i < branches; i++ {
		branchesList = append(branchesList, mk(fmt.Sprintf("w%d", i)))
	}
	join := NewJoinNode("join", branchesList)
	join.Concurrent = true
	sink := NewFunctionNode("sink", func(_ context.Context, _ map[string]any) (Route, any, error) {
		return nil, nil, nil
	})

	w := mustNew(t, "maxconc", []Edge{{From: join, To: sink}}, WithMaxConcurrency(limit))
	if _, err := w.Run(context.Background(), nil); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := maxActive.Load(); got > limit {
		t.Errorf("max concurrent branches = %d, want <= %d", got, limit)
	}
	if got := maxActive.Load(); got != limit {
		t.Errorf("expected %d branches to overlap, observed max = %d", limit, got)
	}
	if got := active.Load(); got != 0 {
		t.Errorf("active counter not drained: %d", got)
	}
}

func TestWorkflow_New_ValidatesEdges(t *testing.T) {
	n := &helperNode{name: "n"}
	n.fn = func(_ context.Context, _ map[string]any) (Route, any, error) { return nil, nil, nil }

	t.Run("nil From", func(t *testing.T) {
		if _, err := New("t", []Edge{{From: nil, To: n}}); err == nil {
			t.Fatal("expected error for nil From")
		}
	})
	t.Run("nil To", func(t *testing.T) {
		if _, err := New("t", []Edge{{From: n, To: nil}}); err == nil {
			t.Fatal("expected error for nil To")
		}
	})
	t.Run("empty name", func(t *testing.T) {
		empty := &helperNode{name: ""}
		empty.fn = func(_ context.Context, _ map[string]any) (Route, any, error) { return nil, nil, nil }
		if _, err := New("t", []Edge{{From: empty, To: n}}); err == nil {
			t.Fatal("expected error for empty node name")
		}
	})
	t.Run("duplicate names", func(t *testing.T) {
		a1 := &helperNode{name: "dup"}
		a1.fn = func(_ context.Context, _ map[string]any) (Route, any, error) { return nil, nil, nil }
		a2 := &helperNode{name: "dup"}
		a2.fn = func(_ context.Context, _ map[string]any) (Route, any, error) { return nil, nil, nil }
		b := &helperNode{name: "b"}
		b.fn = func(_ context.Context, _ map[string]any) (Route, any, error) { return nil, nil, nil }
		if _, err := New("t", []Edge{{From: a1, To: b}, {From: a2, To: b}}); err == nil {
			t.Fatal("expected error for duplicate node names")
		}
	})
	t.Run("nil Fn", func(t *testing.T) {
		bad := NewFunctionNode("bad", nil)
		if _, err := New("t", []Edge{{From: bad, To: n}}); err == nil {
			t.Fatal("expected error for nil Fn")
		}
	})
	t.Run("empty JoinNode", func(t *testing.T) {
		join := NewJoinNode("join", nil)
		if _, err := New("t", []Edge{{From: n, To: join}}); err == nil {
			t.Fatal("expected error for empty JoinNode")
		}
	})
}

func TestWorkflow_MultipleSources(t *testing.T) {
	var mu sync.Mutex
	var path []string

	one := record("one", &path, &mu)
	one.fn = func(_ context.Context, _ map[string]any) (Route, any, error) {
		return StringRoute("go"), "1", nil
	}
	two := record("two", &path, &mu)
	two.fn = func(_ context.Context, _ map[string]any) (Route, any, error) {
		return StringRoute("go"), "2", nil
	}
	merged := record("merged", &path, &mu)
	merged.fn = func(_ context.Context, _ map[string]any) (Route, any, error) { return nil, nil, nil }

	w := mustNew(t, "multi-source", []Edge{
		{From: one, To: merged, Route: StringRoute("go")},
		{From: two, To: merged, Route: StringRoute("go")},
	})
	out, err := w.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if out["one"] != "1" || out["two"] != "2" {
		t.Errorf("unexpected outputs: %v", out)
	}
}

func TestWorkflow_InitialState_NotMutated(t *testing.T) {
	initial := map[string]any{"seed": "keep"}
	a := NewFunctionNode("a", func(_ context.Context, _ map[string]any) (Route, any, error) {
		return nil, "out", nil
	})
	sink := NewFunctionNode("sink", func(_ context.Context, _ map[string]any) (Route, any, error) {
		return nil, nil, nil
	})
	w := mustNew(t, "initial", []Edge{{From: a, To: sink}})

	if _, err := w.Run(context.Background(), initial); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(initial) != 1 || initial["seed"] != "keep" {
		t.Errorf("caller's initial map was mutated: %v", initial)
	}
}

func TestWorkflow_EmptyWorkflow(t *testing.T) {
	w := mustNew(t, "empty", nil)
	out, err := w.Run(context.Background(), map[string]any{"x": 1})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out["x"] != 1 {
		t.Errorf("expected initial state preserved, got %v", out)
	}
}

// contains reports whether err's message contains substr.
func contains(err error, substr string) bool {
	return err != nil && strings.Contains(err.Error(), substr)
}
