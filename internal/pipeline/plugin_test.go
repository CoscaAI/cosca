package pipeline

import (
	"context"
	"strings"
	"testing"
)

// countingPlugin implements Plugin and records invocations.
type countingPlugin struct {
	name    string
	points  []ExtensionPoint
	prio    int
	calls   []ExtensionPoint
	failAt  ExtensionPoint
	failErr error
}

func (p *countingPlugin) Name() string                      { return p.name }
func (p *countingPlugin) ExtensionPoints() []ExtensionPoint { return p.points }
func (p *countingPlugin) Priority() int {
	if p.prio == 0 {
		return 100
	}
	return p.prio
}
func (p *countingPlugin) Execute(ctx context.Context, ext ExtensionPoint, pc *PluginContext) error {
	p.calls = append(p.calls, ext)
	if ext == p.failAt {
		return p.failErr
	}
	return nil
}

func TestPluginRegistryRegisterAndList(t *testing.T) {
	reg := NewPluginRegistry()
	p1 := &countingPlugin{name: "alpha", points: []ExtensionPoint{ExtStepPostExecute}, prio: 10}
	p2 := &countingPlugin{name: "beta", points: []ExtensionPoint{ExtPlanPreCreate}, prio: 20}
	reg.Register(p1)
	reg.Register(p2)

	names := reg.List()
	if len(names) != 2 || names[0] != "alpha" || names[1] != "beta" {
		t.Fatalf("list = %v", names)
	}
	// PluginsFor reflects registered extension points.
	if got := reg.PluginsFor(ExtStepPostExecute); len(got) != 1 || got[0] != "alpha" {
		t.Fatalf("plugins for post_execute = %v", got)
	}
}

func TestPluginRegistryPriorityOrder(t *testing.T) {
	reg := NewPluginRegistry()
	reg.Register(&countingPlugin{name: "slow", points: []ExtensionPoint{ExtStepPostExecute}, prio: 100})
	reg.Register(&countingPlugin{name: "fast", points: []ExtensionPoint{ExtStepPostExecute}, prio: 5})
	reg.Register(&countingPlugin{name: "mid", points: []ExtensionPoint{ExtStepPostExecute}, prio: 50})

	order := reg.PluginsFor(ExtStepPostExecute)
	want := []string{"fast", "mid", "slow"}
	if len(order) != 3 || order[0] != want[0] || order[1] != want[1] || order[2] != want[2] {
		t.Fatalf("order = %v, want %v", order, want)
	}
}

func TestPluginRegistryExecute(t *testing.T) {
	reg := NewPluginRegistry()
	a := &countingPlugin{name: "a", points: []ExtensionPoint{ExtStepPreExecute}, prio: 1}
	b := &countingPlugin{name: "b", points: []ExtensionPoint{ExtStepPreExecute}, prio: 2}
	reg.Register(a)
	reg.Register(b)

	pc := &PluginContext{}
	if err := reg.Execute(context.Background(), ExtStepPreExecute, pc); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(a.calls) != 1 || len(b.calls) != 1 {
		t.Fatalf("calls: a=%d b=%d", len(a.calls), len(b.calls))
	}

	// Unregistered extension point → no-op.
	if err := reg.Execute(context.Background(), ExtPipelineEnd, pc); err != nil {
		t.Fatalf("empty ext: %v", err)
	}
}

func TestPluginRegistryExecuteFailFast(t *testing.T) {
	reg := NewPluginRegistry()
	bad := &countingPlugin{name: "bad", points: []ExtensionPoint{ExtStepPostExecute}, prio: 1, failAt: ExtStepPostExecute, failErr: errSimulated}
	after := &countingPlugin{name: "after", points: []ExtensionPoint{ExtStepPostExecute}, prio: 2}
	reg.Register(bad)
	reg.Register(after)

	err := reg.Execute(context.Background(), ExtStepPostExecute, &PluginContext{})
	if err == nil {
		t.Fatal("fail-fast plugin error must propagate")
	}
	if !strings.Contains(err.Error(), `"bad"`) {
		t.Fatalf("error = %v", err)
	}
	if len(after.calls) != 0 {
		t.Fatal("plugins after a failure must not run")
	}
}

func TestPluginRegistryUnregister(t *testing.T) {
	reg := NewPluginRegistry()
	reg.Register(&countingPlugin{name: "gone", points: []ExtensionPoint{ExtStepPreExecute}, prio: 1})
	reg.Unregister("gone")
	if len(reg.List()) != 0 {
		t.Fatalf("list after unregister = %v", reg.List())
	}
	if got := reg.PluginsFor(ExtStepPreExecute); len(got) != 0 {
		t.Fatalf("order after unregister = %v", got)
	}
	// Execute on empty registry is a no-op.
	if err := reg.Execute(context.Background(), ExtStepPreExecute, &PluginContext{}); err != nil {
		t.Fatal(err)
	}
}

func TestBuiltinPluginsMetadata(t *testing.T) {
	build := &BuildPlugin{}
	if build.Name() != "build" || build.Priority() != 10 {
		t.Fatalf("build plugin: %+v", build)
	}
	test := &TestPlugin{}
	if test.Name() != "test" || test.Priority() != 20 {
		t.Fatalf("test plugin: %+v", test)
	}
	lint := &LintPlugin{}
	if lint.Name() != "lint" || lint.Priority() != 30 {
		t.Fatalf("lint plugin: %+v", lint)
	}
	// All builtins subscribe to step.post_execute.
	pts := lint.ExtensionPoints()
	if len(pts) != 1 || pts[0] != ExtStepPostExecute {
		t.Fatalf("lint points: %v", pts)
	}
}

func TestCheckpointPluginNilGuard(t *testing.T) {
	// Nil store or nil plan → no-op (no panic).
	cp := NewCheckpointPlugin(nil)
	if err := cp.Execute(context.Background(), ExtStepPostExecute, &PluginContext{}); err != nil {
		t.Fatalf("nil guard: %v", err)
	}
	cp2 := NewCheckpointPlugin(&CheckpointStore{})
	if err := cp2.Execute(context.Background(), ExtStepPostExecute, &PluginContext{Plan: nil}); err != nil {
		t.Fatalf("nil plan guard: %v", err)
	}
}

func TestRegisterBuiltinPlugins(t *testing.T) {
	reg := NewPluginRegistry()
	RegisterBuiltinPlugins(reg, nil)
	names := reg.List()
	// build, test, lint, checkpoint, trace.
	if len(names) < 4 {
		t.Fatalf("builtin plugins = %v", names)
	}
}
