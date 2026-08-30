package di

import (
	"errors"
	"testing"
)

// stub is a sample service type used purely in tests.
type stub struct {
	name string
}

// engine is a fake "engine" service.
type engine struct {
	name   string
	dep    *stub
	logger string
}

func TestContainer_ResolveRootSingleton(t *testing.T) {
	c := New()
	// Use *stub so identity (pointer equality) is observable across resolves.
	RegisterRef(c, "stub", func(*Container) (*stub, error) {
		return &stub{name: "stub-v1"}, nil
	})

	s1, err := Resolve(c, Ref[*stub]("stub"))
	if err != nil {
		t.Fatalf("resolve 1: %v", err)
	}
	s2, err := Resolve(c, Ref[*stub]("stub"))
	if err != nil {
		t.Fatalf("resolve 2: %v", err)
	}
	if s1.name != "stub-v1" {
		t.Fatalf("name = %q, want stub-v1", s1.name)
	}
	// Root scope must memoize: second resolve returns the same pointer.
	if s1 != s2 {
		t.Fatal("root scope expected single shared instance")
	}
}

func TestContainer_ResolveDependencyGraph(t *testing.T) {
	c := New()
	// stub is a dependency of engine.
	RegisterRef(c, "stub", func(*Container) (stub, error) {
		return stub{name: "dep"}, nil
	})
	// engine depends on stub: it reads the resolved stub via c.Dependency.
	c.Register(Definition{
		ID:    "engine",
		Scope: ScopeRoot,
		Deps:  []string{"stub"},
		Build: func(c *Container) (any, error) {
			d, err := c.Dependency("stub")
			if err != nil {
				return nil, err
			}
			st, ok := d.(stub)
			if !ok {
				return nil, errors.New("dep is not stub")
			}
			return engine{name: "engine", dep: &st}, nil
		},
	})

	e, err := Resolve(c, Ref[engine]("engine"))
	if err != nil {
		t.Fatalf("resolve engine: %v", err)
	}
	if e.name != "engine" || e.dep.name != "dep" {
		t.Fatalf("engine = %+v, want dep=%q", e, "dep")
	}
}

func TestContainer_CycleDetection(t *testing.T) {
	c := New()
	c.Register(Definition{ID: "a", Deps: []string{"b"}, Build: func(*Container) (any, error) { return nil, nil }})
	c.Register(Definition{ID: "b", Deps: []string{"a"}, Build: func(*Container) (any, error) { return nil, nil }})

	_, err := Resolve(c, Ref[any]("a"))
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestContainer_UnregisteredService(t *testing.T) {
	c := New()
	if _, err := Resolve(c, Ref[any]("missing")); err == nil {
		t.Fatal("expected error for unregistered service")
	}
}

func TestContainer_TopologicalOrder(t *testing.T) {
	c := New()
	c.Register(Definition{ID: "dep"})
	c.Register(Definition{ID: "mid", Deps: []string{"dep"}})
	c.Register(Definition{ID: "top", Deps: []string{"mid"}})

	order, err := c.TopologicalOrder()
	if err != nil {
		t.Fatalf("topo: %v", err)
	}
	pos := map[string]int{}
	for i, id := range order {
		pos[id] = i
	}
	// dep must come before mid, mid before top.
	if !(pos["dep"] < pos["mid"] && pos["mid"] < pos["top"]) {
		t.Fatalf("bad topological order: %v", order)
	}
	if len(order) != 3 {
		t.Fatalf("len(order) = %d, want 3", len(order))
	}
}

func TestContainer_PluginScopeRebuilds(t *testing.T) {
	c := New()
	RegisterRef(c, "stub", func(*Container) (stub, error) {
		return stub{name: "plugin-instance"}, nil
	})
	s1, _ := Resolve(c, RefScoped[stub]("stub", ScopePlugin))
	s2, _ := Resolve(c, RefScoped[stub]("stub", ScopePlugin))
	if &s1 == &s2 {
		t.Fatal("plugin scope expected per-caller instances")
	}
}
