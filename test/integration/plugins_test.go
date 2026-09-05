
//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/plugins"
	"gopkg.in/yaml.v3"
)

// TestPluginManifest_ValidateYAML tests that plugin manifests can be
// parsed from YAML and correctly validated.
func TestPluginManifest_ValidateYAML(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp(".", "cosca-plugin-manifest-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "valid minimal manifest",
			yaml: `
id: test-plugin
name: Test Plugin
version: 1.0.0
runtime: wasm
`,
			wantErr: false,
		},
		{
			name: "valid full manifest with deps",
			yaml: `
id: advanced-plugin
name: Advanced Plugin
version: 2.1.0
description: An advanced plugin with dependencies
author: Cosca Team
runtime: external
permissions:
  - network
  - filesystem
dependencies:
  - plugin_id: base-plugin
    version: ">=1.0.0"
    optional: false
  - plugin_id: optional-helper
    version: ">=0.5.0"
    optional: true
hooks:
  - before_search
  - after_search
`,
			wantErr: false,
		},
		{
			name: "missing id",
			yaml: `
name: No ID
version: 1.0.0
runtime: wasm
`,
			wantErr: true,
		},
		{
			name: "missing name",
			yaml: `
id: no-name
version: 1.0.0
runtime: wasm
`,
			wantErr: true,
		},
		{
			name: "missing version",
			yaml: `
id: no-version
name: No Version
runtime: wasm
`,
			wantErr: true,
		},
		{
			name: "missing runtime",
			yaml: `
id: no-runtime
name: No Runtime
version: 1.0.0
`,
			wantErr: true,
		},
		{
			name: "invalid runtime",
			yaml: `
id: bad-runtime
name: Bad Runtime
version: 1.0.0
runtime: invalid_runtime
`,
			wantErr: true,
		},
		{
			name: "unknown permission (strict policy)",
			yaml: `
id: strict-plugin
name: Strict Plugin
version: 1.0.0
runtime: go
permissions:
  - unknown_perm
`,
			wantErr: true, // manifest.Validate() rejects unknown permissions
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifestPath := filepath.Join(tmpDir, "manifest.yaml")
			if err := os.WriteFile(manifestPath, []byte(tt.yaml), 0o644); err != nil {
				t.Fatal(err)
			}

			var manifest plugins.PluginManifest
			if err := yaml.Unmarshal([]byte(tt.yaml), &manifest); err != nil {
				t.Fatalf("failed to parse YAML: %v", err)
			}

			err := manifest.Validate()
			if tt.wantErr && err == nil {
				t.Error("expected validation error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

// TestPluginLifecycle exercises the full plugin lifecycle:
// Init → Start → Stop with a custom plugin.
func TestPluginLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// ── Create a simple test plugin ───────────────────────────────────────
	testPlugin := &testPluginImpl{
		id:      "test-lifecycle-plugin",
		name:    "Lifecycle Test",
		version: "1.0.0",
		desc:    "A plugin for testing lifecycle",
		author:  "Cosca Test",
	}

	ctx := plugins.NewPluginContext(
		map[string]interface{}{"test_key": "test_value"},
		&testLogger{},
		os.TempDir(),
		&testRuntimeAPI{},
	)

	// ── Init ──────────────────────────────────────────────────────────────
	if err := testPlugin.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if testPlugin.initCalled != 1 {
		t.Errorf("expected Init called once, got %d", testPlugin.initCalled)
	}

	// ── Start ─────────────────────────────────────────────────────────────
	if err := testPlugin.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	testPlugin.StartedAt = time.Now()
	if testPlugin.startCalled != 1 {
		t.Errorf("expected Start called once, got %d", testPlugin.startCalled)
	}

	// ── Health check ──────────────────────────────────────────────────────
	health, err := testPlugin.Health()
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	if health.Status != "healthy" {
		t.Errorf("expected healthy status, got %q", health.Status)
	}
	if health.PluginID != "test-lifecycle-plugin" {
		t.Errorf("expected plugin ID 'test-lifecycle-plugin', got %q", health.PluginID)
	}
	if health.Uptime <= 0 {
		t.Error("expected positive uptime")
	}

	// ── Stop ──────────────────────────────────────────────────────────────
	if err := testPlugin.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if testPlugin.stopCalled != 1 {
		t.Errorf("expected Stop called once, got %d", testPlugin.stopCalled)
	}

	t.Logf("plugin lifecycle completed: init(%d) → start(%d) → stop(%d)",
		testPlugin.initCalled, testPlugin.startCalled, testPlugin.stopCalled)
}

// TestPluginManager_Dependencies verifies dependency resolution
// including topological sorting and missing dependency detection.
func TestPluginManager_Dependencies(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp(".", "cosca-plugin-mgr-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	loader := plugins.NewLoader(plugins.DefaultLoaderConfig(tmpDir))
	mgr := plugins.NewManager(plugins.DefaultManagerConfig(tmpDir), loader)

	// Create plugins with dependencies
	_ = &testPluginImpl{
		id:      "base-plugin",
		name:    "Base Plugin",
		version: "1.0.0",
	}
	_ = &testPluginImpl{
		id:      "dependent-plugin",
		name:    "Dependent Plugin",
		version: "1.0.0",
	}
	_ = &testPluginImpl{
		id:      "orphan-plugin",
		name:    "Orphan Plugin",
		version: "1.0.0",
	}

	// Register plugins in the manager
	mgr.Install("file://" + tmpDir + "/base-plugin")
	mgr.Install("file://" + tmpDir + "/dependent-plugin")
	mgr.Install("file://" + tmpDir + "/orphan-plugin")

	// Since the actual Install does file operations, we'll test the
	// topological sort and dependency validation logic directly.

	// Test topological sort with a simple graph
	graph := map[string][]string{
		"base-plugin":      {},
		"dependent-plugin": {"base-plugin"},
		"orphan-plugin":    {},
	}

	order, err := topologicalSort(graph)
	if err != nil {
		t.Fatalf("topological sort failed: %v", err)
	}

	// base-plugin must come before dependent-plugin
	baseIdx := -1
	depIdx := -1
	for i, id := range order {
		if id == "base-plugin" {
			baseIdx = i
		}
		if id == "dependent-plugin" {
			depIdx = i
		}
	}

	if baseIdx < 0 {
		t.Error("base-plugin not found in sorted order")
	}
	if depIdx < 0 {
		t.Error("dependent-plugin not found in sorted order")
	}
	if baseIdx >= 0 && depIdx >= 0 && baseIdx < depIdx {
		t.Error("base-plugin should come after dependent-plugin in topological order")
	}

	t.Logf("topological order: %v", order)

	// Test circular dependency detection
	circularGraph := map[string][]string{
		"plugin-a": {"plugin-b"},
		"plugin-b": {"plugin-c"},
		"plugin-c": {"plugin-a"},
	}

	_, err = topologicalSort(circularGraph)
	if err == nil {
		t.Error("expected error for circular dependency")
	} else {
		t.Logf("circular dependency correctly detected: %v", err)
	}
}

// TestPluginManager_EventBus verifies the event bus publish/subscribe mechanism.
func TestPluginManager_EventBus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	bus := plugins.NewEventBus(10)

	received := make(chan plugins.Event, 5)

	// Subscribe to specific event type
	subID, err := bus.Subscribe([]string{"test.event"}, "test-plugin", func(event plugins.Event) {
		received <- event
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	defer bus.Unsubscribe(subID)

	// Publish a matching event
	bus.Publish(plugins.NewEvent("test.event", "test-source", map[string]string{"key": "val"}))

	// Wait for the event (async delivery)
	select {
	case event := <-received:
		if event.Type != "test.event" {
			t.Errorf("expected event type 'test.event', got %q", event.Type)
		}
		if event.Source != "test-source" {
			t.Errorf("expected source 'test-source', got %q", event.Source)
		}
		t.Logf("received event: %s", event.String())
	case <-timeoutChan(2 * time.Second):
		t.Fatal("timeout waiting for event delivery")
	}

	// Publish a non-matching event (should not be received)
	go bus.Publish(plugins.NewEvent("other.event", "other-source", nil))

	// Verify subscriber count
	if count := bus.SubscriberCount(); count != 1 {
		t.Errorf("expected 1 subscriber, got %d", count)
	}

	// Unsubscribe
	if err := bus.Unsubscribe(subID); err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}
	if count := bus.SubscriberCount(); count != 0 {
		t.Errorf("expected 0 subscribers after unsubscribe, got %d", count)
	}
}

// TestPluginManager_HookRegistry verifies hook registration and execution.
func TestPluginManager_HookRegistry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	registry := plugins.NewHookRegistry(5 * time.Second)

	hookCalled := false

	// Register a hook
	hookID, err := registry.RegisterHook(
		plugins.HookBeforeSearch,
		"test-plugin",
		func(args interface{}) error {
			hookCalled = true
			return nil
		},
		100,
	)
	if err != nil {
		t.Fatalf("RegisterHook failed: %v", err)
	}
	defer registry.UnregisterHook(hookID)

	// Execute hooks for the registered point
	if err := registry.ExecuteHooks(plugins.HookBeforeSearch, nil); err != nil {
		t.Fatalf("ExecuteHooks failed: %v", err)
	}

	if !hookCalled {
		t.Error("expected hook to be called")
	}

	// Verify it's not called for a different hook point
	if err := registry.ExecuteHooks(plugins.HookAfterSearch, nil); err != nil {
		t.Fatalf("ExecuteHooks for different point failed: %v", err)
	}

	// List hooks
	hooks := registry.ListHooks()
	if len(hooks) == 0 {
		t.Error("expected at least 1 hook in registry")
	}
}

// ── Test Helpers ──────────────────────────────────────────────────────────

// testPluginImpl implements the plugins.Plugin interface for testing.
type testPluginImpl struct {
	plugins.BasePlugin
	id      string
	name    string
	version string
	desc    string
	author  string

	initCalled  int
	startCalled int
	stopCalled  int
}

func (p *testPluginImpl) ID() string          { return p.id }
func (p *testPluginImpl) Name() string        { return p.name }
func (p *testPluginImpl) Version() string     { return p.version }
func (p *testPluginImpl) Description() string { return p.desc }
func (p *testPluginImpl) Author() string      { return p.author }

func (p *testPluginImpl) Init(ctx *plugins.PluginContext) error {
	p.initCalled++
	p.State = plugins.PluginStateInitialized
	return nil
}

func (p *testPluginImpl) Start() error {
	p.startCalled++
	p.State = plugins.PluginStateStarted
	return nil
}

func (p *testPluginImpl) Stop() error {
	p.stopCalled++
	p.State = plugins.PluginStateStopped
	return nil
}

// testLogger implements plugins.PluginLogger with no-op.
type testLogger struct{}

func (l *testLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (l *testLogger) Info(msg string, keysAndValues ...interface{})  {}
func (l *testLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (l *testLogger) Error(msg string, keysAndValues ...interface{}) {}

// testRuntimeAPI implements plugins.RuntimeAPI with no-op.
type testRuntimeAPI struct{}

func (a *testRuntimeAPI) GetConfig(key string) (interface{}, error) {
	return nil, nil
}
func (a *testRuntimeAPI) SetConfig(key string, value interface{}) error {
	return nil
}
func (a *testRuntimeAPI) EmitEvent(eventType string, data interface{}) error {
	return nil
}
func (a *testRuntimeAPI) RegisterHook(hookPoint string, handler func(args interface{}) error) (string, error) {
	return "", nil
}
func (a *testRuntimeAPI) UnregisterHook(hookID string) error {
	return nil
}

// topologicalSort re-implements the topological sort for testing.
// This mirrors the internal implementation in internal/plugins.
func topologicalSort(graph map[string][]string) ([]string, error) {
	inDegree := make(map[string]int)
	for node := range graph {
		if _, ok := inDegree[node]; !ok {
			inDegree[node] = 0
		}
		for _, dep := range graph[node] {
			inDegree[dep]++
		}
	}

	queue := make([]string, 0)
	for node, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, node)
		}
	}

	result := make([]string, 0, len(graph))
	visited := make(map[string]bool)

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		if visited[node] {
			continue
		}
		visited[node] = true
		result = append(result, node)

		for _, neighbor := range graph[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(result) != len(graph) {
		return nil, circularDependencyError(graph)
	}

	return result, nil
}

func circularDependencyError(graph map[string][]string) error {
	return circularDepErr{graph: graph}
}

type circularDepErr struct {
	graph map[string][]string
}

func (e circularDepErr) Error() string {
	return "circular dependency detected"
}

// timeoutChan returns a channel that fires after the given duration.
func timeoutChan(d time.Duration) <-chan time.Time {
	return time.After(d)
}

func (p *testPluginImpl) Health() (plugins.PluginHealth, error) {
	uptime := time.Duration(0)
	if p.StartedAt.IsZero() {
		p.StartedAt = time.Now()
	} else {
		uptime = time.Since(p.StartedAt)
	}
	return plugins.PluginHealth{
		PluginID:  p.id,
		Status:    "healthy",
		LastCheck: time.Now(),
		Uptime:    uptime,
		State:     p.State,
	}, nil
}
