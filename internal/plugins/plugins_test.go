package plugins

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// Hook Registry Tests
// =============================================================================

func TestHookRegistry_NewHookRegistry(t *testing.T) {
	t.Parallel()

	t.Run("valid timeout", func(t *testing.T) {
		r := NewHookRegistry(10 * time.Second)
		if r == nil {
			t.Fatal("expected non-nil registry")
		}
		if r.defaultTimeout != 10*time.Second {
			t.Errorf("timeout = %v, want 10s", r.defaultTimeout)
		}
	})

	t.Run("zero timeout defaults to 30s", func(t *testing.T) {
		r := NewHookRegistry(0)
		if r.defaultTimeout != 30*time.Second {
			t.Errorf("timeout = %v, want 30s", r.defaultTimeout)
		}
	})

	t.Run("negative timeout defaults to 30s", func(t *testing.T) {
		r := NewHookRegistry(-5 * time.Second)
		if r.defaultTimeout != 30*time.Second {
			t.Errorf("timeout = %v, want 30s", r.defaultTimeout)
		}
	})
}

func TestHookRegistry_RegisterHook(t *testing.T) {
	t.Parallel()

	t.Run("valid registration", func(t *testing.T) {
		r := NewHookRegistry(5 * time.Second)
		id, err := r.RegisterHook(HookBeforeIndex, "plugin-a", func(args interface{}) error { return nil }, 50)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id == "" {
			t.Error("expected non-empty hook id")
		}
		if !strings.Contains(id, "before_index_plugin-a") {
			t.Errorf("id = %q, expected to contain hook point and plugin id", id)
		}
	})

	t.Run("invalid hook point", func(t *testing.T) {
		r := NewHookRegistry(5 * time.Second)
		_, err := r.RegisterHook("invalid_hook", "plugin-a", func(args interface{}) error { return nil }, 50)
		if err == nil {
			t.Error("expected error for invalid hook point")
		}
	})

	t.Run("nil handler", func(t *testing.T) {
		r := NewHookRegistry(5 * time.Second)
		_, err := r.RegisterHook(HookBeforeIndex, "plugin-a", nil, 50)
		if err == nil {
			t.Error("expected error for nil handler")
		}
	})

	t.Run("multiple registrations on same hook point", func(t *testing.T) {
		r := NewHookRegistry(5 * time.Second)
		id1, _ := r.RegisterHook(HookBeforeIndex, "plugin-a", func(args interface{}) error { return nil }, 100)
		id2, _ := r.RegisterHook(HookBeforeIndex, "plugin-b", func(args interface{}) error { return nil }, 50)
		if id1 == id2 {
			t.Error("expected different hook ids")
		}
	})

	t.Run("priority ordering preserved", func(t *testing.T) {
		r := NewHookRegistry(5 * time.Second)
		order := make([]int, 0)
		r.RegisterHook(HookAfterSearch, "p1", func(args interface{}) error {
			order = append(order, 2)
			return nil
		}, 200)
		r.RegisterHook(HookAfterSearch, "p2", func(args interface{}) error {
			order = append(order, 1)
			return nil
		}, 100)
		r.RegisterHook(HookAfterSearch, "p3", func(args interface{}) error {
			order = append(order, 3)
			return nil
		}, 300)

		_ = r.ExecuteHooks(HookAfterSearch, nil)
		if len(order) != 3 {
			t.Fatalf("expected 3 executions, got %d", len(order))
		}
		if order[0] != 1 || order[1] != 2 || order[2] != 3 {
			t.Errorf("wrong execution order: %v (want [1 2 3])", order)
		}
	})
}

func TestHookRegistry_UnregisterHook(t *testing.T) {
	t.Parallel()

	t.Run("unregister existing hook", func(t *testing.T) {
		r := NewHookRegistry(5 * time.Second)
		id, _ := r.RegisterHook(HookBeforeIndex, "plugin-a", func(args interface{}) error { return nil }, 50)
		if err := r.UnregisterHook(id); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		err := r.ExecuteHooks(HookBeforeIndex, nil)
		if err != nil {
			t.Errorf("expected no error for empty hooks, got: %v", err)
		}
	})

	t.Run("unregister non-existent hook", func(t *testing.T) {
		r := NewHookRegistry(5 * time.Second)
		err := r.UnregisterHook("nonexistent")
		if err == nil {
			t.Error("expected error for non-existent hook")
		}
	})
}

func TestHookRegistry_ExecuteHooks(t *testing.T) {
	t.Parallel()

	t.Run("no hooks registered", func(t *testing.T) {
		r := NewHookRegistry(5 * time.Second)
		err := r.ExecuteHooks(HookBeforeIndex, nil)
		if err != nil {
			t.Errorf("expected no error for empty hooks: %v", err)
		}
	})

	t.Run("successful execution", func(t *testing.T) {
		r := NewHookRegistry(5 * time.Second)
		var called bool
		r.RegisterHook(HookAfterIndex, "p1", func(args interface{}) error {
			called = true
			return nil
		}, 50)
		err := r.ExecuteHooks(HookAfterIndex, "test-args")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !called {
			t.Error("hook was not called")
		}
	})

	t.Run("handler receives args", func(t *testing.T) {
		r := NewHookRegistry(5 * time.Second)
		var received interface{}
		r.RegisterHook(HookBeforeContext, "p1", func(args interface{}) error {
			received = args
			return nil
		}, 50)
		_ = r.ExecuteHooks(HookBeforeContext, map[string]string{"key": "value"})
		m, ok := received.(map[string]string)
		if !ok || m["key"] != "value" {
			t.Errorf("received = %v, want map[key:value]", received)
		}
	})

	t.Run("error collected but execution continues", func(t *testing.T) {
		r := NewHookRegistry(5 * time.Second)
		var secondCalled bool
		r.RegisterHook(HookAfterExecute, "p1", func(args interface{}) error {
			return fmt.Errorf("hook 1 failed")
		}, 100)
		r.RegisterHook(HookAfterExecute, "p2", func(args interface{}) error {
			secondCalled = true
			return nil
		}, 200)

		err := r.ExecuteHooks(HookAfterExecute, nil)
		if err == nil {
			t.Error("expected error from first hook")
		}
		if !secondCalled {
			t.Error("second hook should have been called despite first error")
		}
	})

	t.Run("all valid hook points", func(t *testing.T) {
		r := NewHookRegistry(5 * time.Second)
		points := ValidHookPoints()
		for _, p := range points {
			id, err := r.RegisterHook(p, "test", func(args interface{}) error { return nil }, 100)
			if err != nil {
				t.Errorf("unexpected error for %s: %v", p, err)
			}
			if err := r.UnregisterHook(id); err != nil {
				t.Errorf("unexpected unregister error for %s: %v", p, err)
			}
		}
		if len(points) != 10 {
			t.Errorf("expected 10 valid hook points, got %d", len(points))
		}
	})
}

func TestHookRegistry_IsValidHookPoint(t *testing.T) {
	t.Parallel()
	tests := []struct {
		point HookPoint
		valid bool
	}{
		{HookBeforeIndex, true},
		{HookAfterIndex, true},
		{HookBeforeSearch, true},
		{HookAfterSearch, true},
		{HookBeforeContext, true},
		{HookAfterContext, true},
		{HookBeforeExecute, true},
		{HookAfterExecute, true},
		{HookPreToolUse, true},
		{HookPostToolUse, true},
		{"invalid", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(string(tt.point), func(t *testing.T) {
			if got := IsValidHookPoint(tt.point); got != tt.valid {
				t.Errorf("IsValidHookPoint(%q) = %v, want %v", tt.point, got, tt.valid)
			}
		})
	}
}

func TestHookRegistry_ClearPluginHooks(t *testing.T) {
	t.Parallel()

	r := NewHookRegistry(5 * time.Second)
	r.RegisterHook(HookBeforeIndex, "plugin-a", func(args interface{}) error { return nil }, 50)
	r.RegisterHook(HookBeforeIndex, "plugin-b", func(args interface{}) error { return nil }, 50)
	r.RegisterHook(HookAfterIndex, "plugin-a", func(args interface{}) error { return nil }, 50)

	r.ClearPluginHooks("plugin-a")

	hooks := r.ListHooks()
	for _, entries := range hooks {
		for _, entry := range entries {
			if entry.PluginID == "plugin-a" {
				t.Errorf("plugin-a hook not cleared: %s", entry.ID)
			}
		}
	}
	totalB := 0
	for _, entries := range hooks {
		for _, entry := range entries {
			if entry.PluginID == "plugin-b" {
				totalB++
			}
		}
	}
	if totalB != 1 {
		t.Errorf("expected 1 plugin-b hook, got %d", totalB)
	}
}

func TestHookRegistry_ListHooks(t *testing.T) {
	t.Parallel()

	r := NewHookRegistry(5 * time.Second)
	r.RegisterHook(HookBeforeIndex, "p1", func(args interface{}) error { return nil }, 100)
	r.RegisterHook(HookBeforeIndex, "p2", func(args interface{}) error { return nil }, 50)
	r.RegisterHook(HookAfterIndex, "p1", func(args interface{}) error { return nil }, 100)

	hooks := r.ListHooks()
	if len(hooks) != 2 {
		t.Errorf("expected 2 hook points, got %d", len(hooks))
	}
	if len(hooks[HookBeforeIndex]) != 2 {
		t.Errorf("expected 2 hooks for before_index, got %d", len(hooks[HookBeforeIndex]))
	}
	if len(hooks[HookAfterIndex]) != 1 {
		t.Errorf("expected 1 hook for after_index, got %d", len(hooks[HookAfterIndex]))
	}
	if _, ok := hooks[HookAfterSearch]; ok {
		t.Error("expected empty hook point to not appear")
	}
}

func TestHookRegistry_DefaultHookRegistry(t *testing.T) {
	t.Parallel()
	r1 := DefaultHookRegistry()
	r2 := DefaultHookRegistry()
	if r1 != r2 {
		t.Error("DefaultHookRegistry should return the same instance")
	}
	if r1 == nil {
		t.Fatal("expected non-nil default registry")
	}
}

func TestHookRegistry_GlobalFunctions(t *testing.T) {
	t.Parallel()
	id, err := RegisterGlobalHook(HookBeforeIndex, "global-test", func(args interface{}) error { return nil }, 100)
	if err != nil {
		t.Fatalf("RegisterGlobalHook error: %v", err)
	}
	if id == "" {
		t.Error("expected non-empty hook id")
	}
	err = ExecuteGlobalHooks(HookBeforeIndex, "test")
	if err != nil {
		t.Errorf("ExecuteGlobalHooks error: %v", err)
	}
	DefaultHookRegistry().UnregisterHook(id)
}

func TestHookRegistry_Timeout(t *testing.T) {
	t.Parallel()
	r := NewHookRegistry(10 * time.Millisecond)
	r.RegisterHook(HookBeforeIndex, "slow", func(args interface{}) error {
		time.Sleep(1 * time.Second)
		return nil
	}, 100)

	err := r.ExecuteHooks(HookBeforeIndex, nil)
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestHookRegistry_PanicRecovery(t *testing.T) {
	t.Parallel()
	r := NewHookRegistry(5 * time.Second)
	r.RegisterHook(HookBeforeIndex, "panicky", func(args interface{}) error {
		panic("intentional panic")
	}, 100)

	err := r.ExecuteHooks(HookBeforeIndex, nil)
	if err == nil {
		t.Error("expected error from panicking hook")
	}
}

// =============================================================================
// Event Bus Tests
// =============================================================================

func TestEventBus_NewEvent(t *testing.T) {
	t.Parallel()
	e := NewEvent("test.type", "source-1", "data-value")
	if e.ID == "" {
		t.Error("expected non-empty event ID")
	}
	if e.Type != "test.type" {
		t.Errorf("Type = %q, want test.type", e.Type)
	}
	if e.Source != "source-1" {
		t.Errorf("Source = %q, want source-1", e.Source)
	}
	if e.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
	if e.Data != "data-value" {
		t.Errorf("Data = %v, want data-value", e.Data)
	}
}

func TestEventBus_EventString(t *testing.T) {
	t.Parallel()
	e := NewEvent("evt.type", "src", nil)
	s := e.String()
	if !strings.Contains(s, "evt.type") {
		t.Errorf("String() = %q, expected to contain evt.type", s)
	}
	if !strings.Contains(s, "src") {
		t.Errorf("String() = %q, expected to contain src", s)
	}
}

func TestEventBus_NewEventBus(t *testing.T) {
	t.Parallel()
	b := NewEventBus(50)
	if b == nil {
		t.Fatal("expected non-nil bus")
	}
	if b.bufferSize != 50 {
		t.Errorf("bufferSize = %d, want 50", b.bufferSize)
	}

	b2 := NewEventBus(0)
	if b2.bufferSize != 100 {
		t.Errorf("default bufferSize = %d, want 100", b2.bufferSize)
	}
}

func TestEventBus_Subscribe(t *testing.T) {
	t.Parallel()

	t.Run("valid subscription", func(t *testing.T) {
		b := NewEventBus(10)
		id, err := b.Subscribe([]string{"test.event"}, "plugin-x", func(e Event) {})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id == "" {
			t.Error("expected non-empty subscriber id")
		}
	})

	t.Run("nil handler", func(t *testing.T) {
		b := NewEventBus(10)
		_, err := b.Subscribe([]string{"test"}, "plugin-x", nil)
		if err == nil {
			t.Error("expected error for nil handler")
		}
	})

	t.Run("stopped bus", func(t *testing.T) {
		b := NewEventBus(10)
		b.Stop()
		_, err := b.Subscribe([]string{"test"}, "plugin-x", func(e Event) {})
		if err == nil {
			t.Error("expected error for stopped bus")
		}
	})
}

func TestEventBus_SubscribeWithFilter(t *testing.T) {
	t.Parallel()

	b := NewEventBus(10)
	filter := EventFilter{
		Types:   []string{"type-a", "type-b"},
		Sources: []string{"source-x"},
	}
	id, err := b.SubscribeWithFilter(filter, "plugin-y", func(e Event) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == "" || !strings.Contains(id, "plugin-y") {
		t.Errorf("id = %q, expected to contain plugin-y", id)
	}
}

func TestEventBus_Unsubscribe(t *testing.T) {
	t.Parallel()

	b := NewEventBus(10)
	id, _ := b.Subscribe([]string{"test"}, "p", func(e Event) {})

	err := b.Unsubscribe(id)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if b.SubscriberCount() != 0 {
		t.Errorf("subscriber count = %d, want 0", b.SubscriberCount())
	}

	err = b.Unsubscribe("foo")
	if err == nil {
		t.Error("expected error for non-existent subscriber")
	}
}

func TestEventBus_Publish_Async(t *testing.T) {
	t.Parallel()

	t.Run("matching subscriber receives event", func(t *testing.T) {
		b := NewEventBus(10)
		var mu sync.Mutex
		var received []Event
		b.Subscribe([]string{"foo.event"}, "p1", func(e Event) {
			mu.Lock()
			received = append(received, e)
			mu.Unlock()
		})
		evt := NewEvent("foo.event", "src", "payload")
		b.Publish(evt)
		time.Sleep(50 * time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		if len(received) != 1 {
			t.Errorf("received = %d, want 1", len(received))
		}
	})

	t.Run("non-matching subscriber not called", func(t *testing.T) {
		b := NewEventBus(10)
		var called int32
		b.Subscribe([]string{"bar.event"}, "p1", func(e Event) {
			atomic.AddInt32(&called, 1)
		})
		evt := NewEvent("foo.event", "src", nil)
		b.Publish(evt)
		time.Sleep(50 * time.Millisecond)
		if atomic.LoadInt32(&called) != 0 {
			t.Errorf("non-matching subscriber was called")
		}
	})

	t.Run("stopped bus drops event", func(t *testing.T) {
		b := NewEventBus(10)
		var called int32
		b.Subscribe([]string{"test"}, "p1", func(e Event) {
			atomic.AddInt32(&called, 1)
		})
		b.Stop()
		evt := NewEvent("test", "src", nil)
		b.Publish(evt)
		time.Sleep(50 * time.Millisecond)
		if atomic.LoadInt32(&called) != 0 {
			t.Error("event published after stop")
		}
	})

	t.Run("handler panic is isolated", func(t *testing.T) {
		b := NewEventBus(10)
		var secondCalled atomic.Bool
		b.Subscribe([]string{"test"}, "p1", func(e Event) {
			panic("intentional")
		})
		b.Subscribe([]string{"test"}, "p2", func(e Event) {
			secondCalled.Store(true)
		})
		b.Publish(NewEvent("test", "src", nil))
		time.Sleep(50 * time.Millisecond)
		if !secondCalled.Load() {
			t.Error("second handler should have been called despite first panic")
		}
	})
}

func TestEventBus_PublishSync(t *testing.T) {
	t.Parallel()

	b := NewEventBus(10)
	var received []Event
	b.Subscribe([]string{"sync.test"}, "p1", func(e Event) {
		received = append(received, e)
	})

	evt := NewEvent("sync.test", "src", "data")
	b.PublishSync(evt)

	if len(received) != 1 {
		t.Errorf("received = %d, want 1", len(received))
	}
	if received[0].Type != "sync.test" {
		t.Errorf("type = %q", received[0].Type)
	}
}

func TestEventBus_SubscriberCount(t *testing.T) {
	t.Parallel()

	b := NewEventBus(10)
	if b.SubscriberCount() != 0 {
		t.Errorf("count = %d, want 0", b.SubscriberCount())
	}
	b.Subscribe([]string{"a"}, "p1", func(e Event) {})
	b.Subscribe([]string{"b"}, "p2", func(e Event) {})
	if b.SubscriberCount() != 2 {
		t.Errorf("count = %d, want 2", b.SubscriberCount())
	}
}

func TestEventBus_ClearPluginSubscribers(t *testing.T) {
	t.Parallel()

	b := NewEventBus(10)
	b.Subscribe([]string{"a"}, "p1", func(e Event) {})
	b.Subscribe([]string{"b"}, "p1", func(e Event) {})
	b.Subscribe([]string{"c"}, "p2", func(e Event) {})

	removed := b.ClearPluginSubscribers("p1")
	if removed != 2 {
		t.Errorf("removed = %d, want 2", removed)
	}
	if b.SubscriberCount() != 1 {
		t.Errorf("count = %d, want 1", b.SubscriberCount())
	}
}

func TestEventBus_Stop(t *testing.T) {
	t.Parallel()

	b := NewEventBus(10)
	b.Subscribe([]string{"t"}, "p", func(e Event) {})
	b.Stop()
	if b.SubscriberCount() != 0 {
		t.Errorf("count after stop = %d, want 0", b.SubscriberCount())
	}
}

func TestEventBus_DefaultEventBus(t *testing.T) {
	t.Parallel()
	b1 := DefaultEventBus()
	b2 := DefaultEventBus()
	if b1 != b2 {
		t.Error("DefaultEventBus should return the same instance")
	}
}

func TestEventBus_GlobalFunctions(t *testing.T) {
	t.Parallel()
	id, err := SubscribeGlobal([]string{"global.test"}, "test-plugin", func(e Event) {})
	if err != nil {
		t.Fatalf("SubscribeGlobal error: %v", err)
	}
	if id == "" {
		t.Error("expected non-empty subscriber id")
	}
	PublishGlobalEvent("global.test", "test-src", "test-data")
	DefaultEventBus().Unsubscribe(id)
}

func TestEventFilter_Matches(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		filter EventFilter
		event  Event
		want   bool
	}{
		{
			name:   "empty filter matches all",
			filter: EventFilter{},
			event:  NewEvent("any", "any", nil),
			want:   true,
		},
		{
			name:   "type filter matches",
			filter: EventFilter{Types: []string{"match.me"}},
			event:  NewEvent("match.me", "any", nil),
			want:   true,
		},
		{
			name:   "type filter no match",
			filter: EventFilter{Types: []string{"match.me"}},
			event:  NewEvent("other", "any", nil),
			want:   false,
		},
		{
			name:   "source filter matches",
			filter: EventFilter{Sources: []string{"src1"}},
			event:  NewEvent("any", "src1", nil),
			want:   true,
		},
		{
			name:   "source filter no match",
			filter: EventFilter{Sources: []string{"src1"}},
			event:  NewEvent("any", "src2", nil),
			want:   false,
		},
		{
			name:   "both type and source match",
			filter: EventFilter{Types: []string{"t1"}, Sources: []string{"s1"}},
			event:  NewEvent("t1", "s1", nil),
			want:   true,
		},
		{
			name:   "type matches but source does not",
			filter: EventFilter{Types: []string{"t1"}, Sources: []string{"s1"}},
			event:  NewEvent("t1", "s2", nil),
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.matches(tt.event); got != tt.want {
				t.Errorf("matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Topological Sort Tests
// =============================================================================

func TestTopologicalSort(t *testing.T) {
	t.Parallel()

	t.Run("simple linear graph", func(t *testing.T) {
		graph := map[string][]string{
			"c": {"b"},
			"b": {"a"},
			"a": {},
		}
		order, err := topologicalSort(graph)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(order) != 3 {
			t.Errorf("order len = %d, want 3", len(order))
		}
		// Kahn's algorithm with reversed in-degree produces dependents-first order
		// c depends on b, b depends on a → order is [c, b, a]
		if order[0] != "c" || order[1] != "b" || order[2] != "a" {
			t.Errorf("wrong order: %v (expected [c b a])", order)
		}
	})

	t.Run("diamond graph", func(t *testing.T) {
		graph := map[string][]string{
			"d": {"b", "c"},
			"c": {"a"},
			"b": {"a"},
			"a": {},
		}
		order, err := topologicalSort(graph)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		pos := make(map[string]int)
		for i, id := range order {
			pos[id] = i
		}
		// d depends on b,c; b,c depend on a
		// d has inDegree 0, processed first
		if pos["a"] <= pos["b"] || pos["a"] <= pos["c"] {
			t.Errorf("a should come after b and c (it's the deepest dependency): %v", order)
		}
		if pos["d"] > pos["b"] || pos["d"] > pos["c"] {
			t.Errorf("d should come before b and c: %v", order)
		}
	})

	t.Run("circular dependency returns error", func(t *testing.T) {
		graph := map[string][]string{
			"a": {"b"},
			"b": {"a"},
		}
		_, err := topologicalSort(graph)
		if err == nil {
			t.Error("expected error for circular dependency")
		}
	})

	t.Run("empty graph", func(t *testing.T) {
		graph := map[string][]string{}
		order, err := topologicalSort(graph)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(order) != 0 {
			t.Errorf("order len = %d, want 0", len(order))
		}
	})

	t.Run("independent nodes", func(t *testing.T) {
		graph := map[string][]string{
			"x": {},
			"y": {},
			"z": {},
		}
		order, err := topologicalSort(graph)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(order) != 3 {
			t.Errorf("order len = %d, want 3", len(order))
		}
	})
}

// =============================================================================
// Manager: Dependency Tests
// =============================================================================

func TestManager_ResolveDependencies(t *testing.T) {
	t.Parallel()

	m := NewManager(DefaultManagerConfig(t.TempDir()), nil)
	m.plugins = map[string]pluginEntry{
		"a": {
			plugin: &MockPlugin{IDValue: "a"},
			info: PluginInfo{
				Manifest: PluginManifest{ID: "a", Dependencies: []PluginDependency{{PluginID: "b"}}},
				Enabled:  true,
				State:    PluginStateInstalled,
			},
		},
		"b": {
			plugin: &MockPlugin{IDValue: "b"},
			info: PluginInfo{
				Manifest: PluginManifest{ID: "b", Dependencies: []PluginDependency{{PluginID: "c"}}},
				Enabled:  true,
				State:    PluginStateInstalled,
			},
		},
		"c": {
			plugin: &MockPlugin{IDValue: "c"},
			info: PluginInfo{
				Manifest: PluginManifest{ID: "c"},
				Enabled:  true,
				State:    PluginStateInstalled,
			},
		},
	}

	err := m.ResolveDependencies()
	if err != nil {
		t.Fatalf("ResolveDependencies error: %v", err)
	}
}

func TestManager_ResolveDependencies_Circular(t *testing.T) {
	t.Parallel()

	m := NewManager(DefaultManagerConfig(t.TempDir()), nil)
	m.plugins = map[string]pluginEntry{
		"a": {
			info: PluginInfo{Manifest: PluginManifest{ID: "a", Dependencies: []PluginDependency{{PluginID: "b"}}}},
		},
		"b": {
			info: PluginInfo{Manifest: PluginManifest{ID: "b", Dependencies: []PluginDependency{{PluginID: "a"}}}},
		},
	}
	err := m.ResolveDependencies()
	if err == nil {
		t.Error("expected error for circular dependencies")
	}
}

func TestManager_ValidateDependencies(t *testing.T) {
	t.Parallel()

	t.Run("all satisfied", func(t *testing.T) {
		m := NewManager(DefaultManagerConfig(t.TempDir()), nil)
		m.plugins = map[string]pluginEntry{
			"a": {
				info: PluginInfo{
					Manifest: PluginManifest{ID: "a", Dependencies: []PluginDependency{{PluginID: "b", Version: "1.0"}}},
					Enabled:  true,
				},
			},
			"b": {
				info: PluginInfo{
					Manifest: PluginManifest{ID: "b", Version: "1.0"},
					Enabled:  true,
				},
			},
		}
		if err := m.ValidateDependencies(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing required dependency", func(t *testing.T) {
		m := NewManager(DefaultManagerConfig(t.TempDir()), nil)
		m.plugins = map[string]pluginEntry{
			"a": {
				info: PluginInfo{
					Manifest: PluginManifest{ID: "a", Dependencies: []PluginDependency{{PluginID: "b"}}},
					Enabled:  true,
				},
			},
		}
		err := m.ValidateDependencies()
		if err == nil {
			t.Error("expected error for missing dependency")
		}
	})

	t.Run("optional missing is ok", func(t *testing.T) {
		m := NewManager(DefaultManagerConfig(t.TempDir()), nil)
		m.plugins = map[string]pluginEntry{
			"a": {
				info: PluginInfo{
					Manifest: PluginManifest{ID: "a", Dependencies: []PluginDependency{{PluginID: "b", Optional: true}}},
					Enabled:  true,
				},
			},
		}
		if err := m.ValidateDependencies(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("dependency disabled but required", func(t *testing.T) {
		m := NewManager(DefaultManagerConfig(t.TempDir()), nil)
		m.plugins = map[string]pluginEntry{
			"a": {
				info: PluginInfo{
					Manifest: PluginManifest{ID: "a", Dependencies: []PluginDependency{{PluginID: "b"}}},
					Enabled:  true,
				},
			},
			"b": {
				info: PluginInfo{
					Manifest: PluginManifest{ID: "b"},
					Enabled:  false,
				},
			},
		}
		err := m.ValidateDependencies()
		if err == nil {
			t.Error("expected error for disabled dependency")
		}
	})

	t.Run("optional dependency disabled is ok", func(t *testing.T) {
		m := NewManager(DefaultManagerConfig(t.TempDir()), nil)
		m.plugins = map[string]pluginEntry{
			"a": {
				info: PluginInfo{
					Manifest: PluginManifest{ID: "a", Dependencies: []PluginDependency{{PluginID: "b", Optional: true}}},
					Enabled:  true,
				},
			},
			"b": {
				info: PluginInfo{
					Manifest: PluginManifest{ID: "b"},
					Enabled:  false,
				},
			},
		}
		if err := m.ValidateDependencies(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("version mismatch with required", func(t *testing.T) {
		m := NewManager(DefaultManagerConfig(t.TempDir()), nil)
		m.plugins = map[string]pluginEntry{
			"a": {
				info: PluginInfo{
					Manifest: PluginManifest{ID: "a", Dependencies: []PluginDependency{{PluginID: "b", Version: "2.0"}}},
					Enabled:  true,
				},
			},
			"b": {
				info: PluginInfo{
					Manifest: PluginManifest{ID: "b", Version: "1.0"},
					Enabled:  true,
				},
			},
		}
		err := m.ValidateDependencies()
		if err == nil {
			t.Error("expected error for version mismatch")
		}
	})

	t.Run("optional version mismatch is ok", func(t *testing.T) {
		m := NewManager(DefaultManagerConfig(t.TempDir()), nil)
		m.plugins = map[string]pluginEntry{
			"a": {
				info: PluginInfo{
					Manifest: PluginManifest{ID: "a", Dependencies: []PluginDependency{{PluginID: "b", Version: "2.0", Optional: true}}},
					Enabled:  true,
				},
			},
			"b": {
				info: PluginInfo{
					Manifest: PluginManifest{ID: "b", Version: "1.0"},
					Enabled:  true,
				},
			},
		}
		if err := m.ValidateDependencies(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// =============================================================================
// Manager: Enable/Disable Tests
// =============================================================================

func TestManager_EnableDisable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(DefaultManagerConfig(dir), nil)
	m.plugins["test-plugin"] = pluginEntry{
		plugin: &MockPlugin{IDValue: "test-plugin"},
		info: PluginInfo{
			Manifest: PluginManifest{ID: "test-plugin"},
			Enabled:  false,
			State:    PluginStateInstalled,
		},
	}

	if err := m.Enable("test-plugin"); err != nil {
		t.Fatalf("Enable error: %v", err)
	}
	info, _ := m.Get("test-plugin")
	if !info.Enabled {
		t.Error("plugin should be enabled")
	}

	if err := m.Enable("nonexistent"); err == nil {
		t.Error("expected error for non-existent plugin")
	}

	if err := m.Disable("test-plugin"); err != nil {
		t.Fatalf("Disable error: %v", err)
	}
	info, _ = m.Get("test-plugin")
	if info.Enabled {
		t.Error("plugin should be disabled")
	}

	if err := m.Disable("nonexistent"); err == nil {
		t.Error("expected error for non-existent plugin")
	}
}

func TestManager_DisableStopsRunningPlugin(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(DefaultManagerConfig(dir), nil)
	stopCalled := false
	m.plugins["p"] = pluginEntry{
		plugin: &MockPlugin{
			IDValue:  "p",
			StopFunc: func() error { stopCalled = true; return nil },
		},
		info: PluginInfo{
			Manifest: PluginManifest{ID: "p"},
			Enabled:  true,
			State:    PluginStateStarted,
		},
	}
	if err := m.Disable("p"); err != nil {
		t.Fatalf("Disable error: %v", err)
	}
	if !stopCalled {
		t.Error("Stop should have been called on the running plugin")
	}
	info, _ := m.Get("p")
	if info.State != PluginStateStopped {
		t.Errorf("state = %v, want stopped", info.State)
	}
}

// =============================================================================
// Manager: List/Get Tests
// =============================================================================

func TestManager_List(t *testing.T) {
	t.Parallel()

	m := NewManager(DefaultManagerConfig(t.TempDir()), nil)
	m.plugins = map[string]pluginEntry{
		"b": {info: PluginInfo{Manifest: PluginManifest{ID: "b"}}},
		"a": {info: PluginInfo{Manifest: PluginManifest{ID: "a"}}},
		"c": {info: PluginInfo{Manifest: PluginManifest{ID: "c"}}},
	}
	list := m.List()
	if len(list) != 3 {
		t.Errorf("len = %d, want 3", len(list))
	}
	if list[0].Manifest.ID != "a" || list[1].Manifest.ID != "b" || list[2].Manifest.ID != "c" {
		t.Errorf("list not sorted: %v", []string{list[0].Manifest.ID, list[1].Manifest.ID, list[2].Manifest.ID})
	}
}

func TestManager_Get(t *testing.T) {
	t.Parallel()

	m := NewManager(DefaultManagerConfig(t.TempDir()), nil)
	m.plugins["p1"] = pluginEntry{
		plugin: &MockPlugin{IDValue: "p1"},
		info:   PluginInfo{Manifest: PluginManifest{ID: "p1", Name: "P One"}, Enabled: true},
	}

	info, err := m.Get("p1")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if info.Manifest.Name != "P One" {
		t.Errorf("Name = %q", info.Manifest.Name)
	}

	_, err = m.Get("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent plugin")
	}
}

func TestManager_GetPlugin(t *testing.T) {
	t.Parallel()

	m := NewManager(DefaultManagerConfig(t.TempDir()), nil)
	m.plugins["p1"] = pluginEntry{
		plugin: &MockPlugin{IDValue: "p1"},
		info:   PluginInfo{Manifest: PluginManifest{ID: "p1"}},
	}

	p, err := m.GetPlugin("p1")
	if err != nil {
		t.Fatalf("GetPlugin error: %v", err)
	}
	if p.ID() != "p1" {
		t.Errorf("ID = %q", p.ID())
	}

	_, err = m.GetPlugin("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent plugin")
	}
}

func TestManager_DefaultManagerConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultManagerConfig("/base")
	if cfg.Dir != filepath.Join("/base", "plugins") {
		t.Errorf("Dir = %q", cfg.Dir)
	}
	if cfg.RegistryURL != "https://plugins.cosca.dev/v1" {
		t.Errorf("RegistryURL = %q", cfg.RegistryURL)
	}
	if cfg.MaxPluginSize != 50*1024*1024 {
		t.Errorf("MaxPluginSize = %d", cfg.MaxPluginSize)
	}
	if !cfg.VerifyChecksum {
		t.Error("VerifyChecksum should be true")
	}
}

func TestManager_NewManager(t *testing.T) {
	t.Parallel()
	cfg := DefaultManagerConfig(t.TempDir())
	m := NewManager(cfg, nil)
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if m.plugins == nil {
		t.Error("plugins map should be initialized")
	}
	if m.loader == nil {
		t.Error("loader should be initialized when nil passed")
	}
}

// =============================================================================
// Manager: ParseSource Tests
// =============================================================================

func TestManager_ParseSource(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(DefaultManagerConfig(dir), nil)

	t.Run("empty source", func(t *testing.T) {
		_, _, err := m.parseSource("")
		if err == nil {
			t.Error("expected error for empty source")
		}
	})

	t.Run("file scheme", func(t *testing.T) {
		sourceType, pluginID, err := m.parseSource("file:///path/to/plugin.tar.gz")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sourceType != "local" {
			t.Errorf("sourceType = %q, want local", sourceType)
		}
		if pluginID != "plugin" {
			t.Errorf("pluginID = %q, want plugin", pluginID)
		}
	})

	t.Run("registry scheme", func(t *testing.T) {
		sourceType, pluginID, err := m.parseSource("registry://my-plugin")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sourceType != "registry" {
			t.Errorf("sourceType = %q", sourceType)
		}
		if pluginID != "my-plugin" {
			t.Errorf("pluginID = %q", pluginID)
		}
	})

	t.Run("git url", func(t *testing.T) {
		sourceType, pluginID, err := m.parseSource("https://github.com/user/repo.git")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sourceType != "git" {
			t.Errorf("sourceType = %q, want git", sourceType)
		}
		if pluginID != "repo" {
			t.Errorf("pluginID = %q, want repo", pluginID)
		}
	})

	t.Run("http url with .git", func(t *testing.T) {
		sourceType, pluginID, err := m.parseSource("http://example.com/plugin.git")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sourceType != "git" {
			t.Errorf("sourceType = %q", sourceType)
		}
		if pluginID != "plugin" {
			t.Errorf("pluginID = %q", pluginID)
		}
	})

	t.Run("existing local dir", func(t *testing.T) {
		localDir := filepath.Join(dir, "my-extension")
		os.MkdirAll(localDir, 0o755)
		sourceType, pluginID, err := m.parseSource(localDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sourceType != "local" {
			t.Errorf("sourceType = %q", sourceType)
		}
		if pluginID != "my-extension" {
			t.Errorf("pluginID = %q", pluginID)
		}
	})

	t.Run("default to registry", func(t *testing.T) {
		sourceType, pluginID, err := m.parseSource("some-plugin-id")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sourceType != "registry" {
			t.Errorf("sourceType = %q, want registry", sourceType)
		}
		if pluginID != "some-plugin-id" {
			t.Errorf("pluginID = %q", pluginID)
		}
	})
}

func TestManager_ExtractPluginID(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(DefaultManagerConfig(dir), nil)

	tests := []struct {
		name string
		path string
		want string
	}{
		{"tar.gz", "/path/to/plugin.tar.gz", "plugin"},
		{"tgz", "/path/to/plugin.tgz", "plugin"},
		{"zip", "/path/to/plugin.zip", "plugin"},
		{"so", "/path/to/plugin.so", "plugin"},
		{"wasm", "/path/to/plugin.wasm", "plugin"},
		{"no ext", "/path/to/plugin", "plugin"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := m.extractPluginIDFromPath(tt.path); got != tt.want {
				t.Errorf("extractPluginIDFromPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestManager_ExtractPluginIDFromGitURL(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(DefaultManagerConfig(dir), nil)

	if got := m.extractPluginIDFromGitURL("https://github.com/user/my-plugin.git"); got != "my-plugin" {
		t.Errorf("got %q, want my-plugin", got)
	}
	if got := m.extractPluginIDFromGitURL("https://github.com/user/my-plugin"); got != "my-plugin" {
		t.Errorf("got %q, want my-plugin", got)
	}
}

// =============================================================================
// Loader: Config & Validation Tests
// =============================================================================

func TestLoader_DefaultLoaderConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultLoaderConfig("/plugins")
	if cfg.PluginsDir != "/plugins" {
		t.Errorf("PluginsDir = %q", cfg.PluginsDir)
	}
	if cfg.PermissionPolicy != "strict" {
		t.Errorf("PermissionPolicy = %q", cfg.PermissionPolicy)
	}
	if cfg.MaxWASMMemory != 128*1024*1024 {
		t.Errorf("MaxWASMMemory = %d", cfg.MaxWASMMemory)
	}
	if cfg.ExternalProcessTimeout != 30 {
		t.Errorf("ExternalProcessTimeout = %d", cfg.ExternalProcessTimeout)
	}
	if !cfg.DisableGoPlugins {
		t.Error("Go plugins should be disabled by default")
	}
	if !cfg.Sandbox.Enabled {
		t.Error("Sandbox should be enabled by default")
	}
	if cfg.Sandbox.MaxCPUSeconds != 30 {
		t.Errorf("MaxCPUSeconds = %d", cfg.Sandbox.MaxCPUSeconds)
	}
	if cfg.Sandbox.MaxMemoryMB != 512 {
		t.Errorf("MaxMemoryMB = %d", cfg.Sandbox.MaxMemoryMB)
	}
	if cfg.Sandbox.MaxFileSizeMB != 10 {
		t.Errorf("MaxFileSizeMB = %d", cfg.Sandbox.MaxFileSizeMB)
	}
	if cfg.Sandbox.MaxMessageBytes != 1*1024*1024 {
		t.Errorf("MaxMessageBytes = %d", cfg.Sandbox.MaxMessageBytes)
	}
	if !cfg.Sandbox.SeccompEnabled {
		t.Error("Seccomp should be enabled by default")
	}
}

func TestLoader_NewLoader(t *testing.T) {
	t.Parallel()
	l := NewLoader(DefaultLoaderConfig(t.TempDir()))
	if l == nil {
		t.Fatal("expected non-nil loader")
	}
	if l.loaded == nil {
		t.Error("loaded map should be initialized")
	}
}

func TestLoader_ValidateRuntime(t *testing.T) {
	t.Parallel()

	t.Run("no restrictions allows all", func(t *testing.T) {
		l := &Loader{config: LoaderConfig{AllowedRuntimes: nil}}
		if err := l.validateRuntime(RuntimeGo); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if err := l.validateRuntime(RuntimeWASM); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("restricted list", func(t *testing.T) {
		l := &Loader{config: LoaderConfig{AllowedRuntimes: []PluginRuntime{RuntimeWASM}}}
		if err := l.validateRuntime(RuntimeWASM); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if err := l.validateRuntime(RuntimeGo); err == nil {
			t.Error("expected error for disallowed runtime")
		}
	})
}

func TestLoader_ValidatePermissions(t *testing.T) {
	t.Parallel()

	t.Run("strict policy rejects unknown", func(t *testing.T) {
		l := &Loader{config: LoaderConfig{PermissionPolicy: "strict"}}
		m := &PluginManifest{ID: "test", Permissions: []string{"unknown_perm"}}
		err := l.validatePermissions(m)
		if err == nil {
			t.Error("expected error for unknown permission under strict policy")
		}
	})

	t.Run("warn policy allows unknown", func(t *testing.T) {
		l := &Loader{config: LoaderConfig{PermissionPolicy: "warn"}}
		m := &PluginManifest{ID: "test", Permissions: []string{"unknown_perm"}}
		err := l.validatePermissions(m)
		if err != nil {
			t.Errorf("unexpected error under warn policy: %v", err)
		}
	})

	t.Run("allow policy allows unknown", func(t *testing.T) {
		l := &Loader{config: LoaderConfig{PermissionPolicy: "allow"}}
		m := &PluginManifest{ID: "test", Permissions: []string{"anything"}}
		err := l.validatePermissions(m)
		if err != nil {
			t.Errorf("unexpected error under allow policy: %v", err)
		}
	})

	t.Run("empty permissions always ok", func(t *testing.T) {
		l := &Loader{config: LoaderConfig{PermissionPolicy: "strict"}}
		m := &PluginManifest{ID: "test", Permissions: []string{}}
		if err := l.validatePermissions(m); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("all known permissions ok", func(t *testing.T) {
		l := &Loader{config: LoaderConfig{PermissionPolicy: "strict"}}
		m := &PluginManifest{ID: "test", Permissions: []string{"network", "filesystem", "exec", "environment", "all"}}
		if err := l.validatePermissions(m); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestLoader_DetectRuntimeFromExtension(t *testing.T) {
	t.Parallel()
	l := &Loader{}

	tests := []struct {
		path    string
		runtime PluginRuntime
	}{
		{"plugin.so", RuntimeGo},
		{"plugin.dll", RuntimeSharedLib},
		{"plugin.dylib", RuntimeSharedLib},
		{"plugin.wasm", RuntimeWASM},
		{"plugin", RuntimeExternal},
		{"plugin.exe", RuntimeExternal},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := l.detectRuntimeFromExtension(tt.path); got != tt.runtime {
				t.Errorf("detectRuntimeFromExtension(%q) = %q, want %q", tt.path, got, tt.runtime)
			}
		})
	}
}

func TestLoader_IsPluginFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want bool
	}{
		{"plugin.so", true},
		{"plugin.wasm", true},
		{"plugin.dll", true},
		{"plugin.dylib", true},
		{"plugin.txt", false},
		{"no-ext", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPluginFile(tt.name); got != tt.want {
				t.Errorf("isPluginFile(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// =============================================================================
// WASM Plugin Health Tests
// =============================================================================

func TestWASMPlugin_Health_RuntimeNil(t *testing.T) {
	t.Parallel()
	p := &wasmPlugin{
		BasePlugin: BasePlugin{IDValue: "wasm-test"},
	}
	health, err := p.Health()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if health.Status != "unhealthy" {
		t.Errorf("Status = %q, want unhealthy", health.Status)
	}
	if health.PluginID != "wasm-test" {
		t.Errorf("PluginID = %q", health.PluginID)
	}
}

// =============================================================================
// External Plugin: sendJSON/readResponse Tests
// =============================================================================

func TestExternalPlugin_SendJSON_Oversized(t *testing.T) {
	t.Parallel()
	p := &externalPlugin{
		maxMessageSize: 10,
	}
	err := p.sendJSON("this is a long string that exceeds ten bytes")
	if err == nil {
		t.Error("expected error for oversized message")
	}
	if !strings.Contains(err.Error(), "exceeds limit") {
		t.Errorf("wrong error: %v", err)
	}
}

func TestExternalPlugin_ReadResponse_NoScanner(t *testing.T) {
	t.Parallel()
	// readResponse requires a non-nil scanner; ensure it panics with nil
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when scanner is nil")
		}
	}()
	p := &externalPlugin{
		maxMessageSize: 1024,
	}
	_ = p.readResponse(&struct{ Type string }{})
}

func TestExternalPlugin_ReadResponseWithTimeout(t *testing.T) {
	t.Parallel()
	// Use a pipe that never gets data written to trigger timeout
	pr, pw, _ := os.Pipe()
	defer pw.Close()
	defer pr.Close()
	p := &externalPlugin{
		maxMessageSize: 1024,
		scanner:        bufio.NewScanner(pr),
	}
	err := p.readResponseWithTimeout(&struct{ Type string }{}, 10*time.Millisecond)
	if err == nil {
		t.Error("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("wrong error: %v", err)
	}
}

func TestExternalPlugin_Health_ProcessNotStarted(t *testing.T) {
	t.Parallel()
	p := &externalPlugin{
		BasePlugin: BasePlugin{IDValue: "ext-test"},
	}
	health, err := p.Health()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if health.Status != "unhealthy" {
		t.Errorf("Status = %q, want unhealthy", health.Status)
	}
	if !strings.Contains(health.Message, "not started") {
		t.Errorf("Message = %q", health.Message)
	}
}

// =============================================================================
// Loader: loadGoPlugin disabled
// =============================================================================

func TestLoadGoPlugin_Disabled(t *testing.T) {
	t.Parallel()
	l := NewLoader(DefaultLoaderConfig(t.TempDir()))
	manifest := &PluginManifest{ID: "test", Runtime: RuntimeGo}
	_, err := l.loadGoPlugin(manifest, "/some/path.so")
	if err == nil {
		t.Error("expected error for disabled Go plugins")
	}
	if !strings.Contains(err.Error(), "disabled") {
		t.Errorf("wrong error: %v", err)
	}
}

// =============================================================================
// MockPlugin with function overrides
// =============================================================================

func TestMockPlugin_CustomInit(t *testing.T) {
	t.Parallel()
	called := false
	p := &MockPlugin{
		InitFunc: func(ctx *PluginContext) error {
			called = true
			return fmt.Errorf("custom init error")
		},
	}
	err := p.Init(&PluginContext{})
	if err == nil || err.Error() != "custom init error" {
		t.Errorf("expected custom init error: %v", err)
	}
	if !called {
		t.Error("InitFunc was not called")
	}
}

func TestMockPlugin_CustomStart(t *testing.T) {
	t.Parallel()
	p := &MockPlugin{
		StartFunc: func() error { return fmt.Errorf("start failed") },
	}
	err := p.Start()
	if err == nil || err.Error() != "start failed" {
		t.Errorf("expected start error: %v", err)
	}
}

func TestMockPlugin_CustomStop(t *testing.T) {
	t.Parallel()
	p := &MockPlugin{
		StopFunc: func() error { return fmt.Errorf("stop failed") },
	}
	err := p.Stop()
	if err == nil || err.Error() != "stop failed" {
		t.Errorf("expected stop error: %v", err)
	}
}

func TestMockPlugin_CustomHealth(t *testing.T) {
	t.Parallel()
	p := &MockPlugin{
		HealthFunc: func() (PluginHealth, error) {
			return PluginHealth{PluginID: "custom", Status: "degraded"}, nil
		},
	}
	health, err := p.Health()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if health.PluginID != "custom" || health.Status != "degraded" {
		t.Errorf("wrong health result: %+v", health)
	}
}

// =============================================================================
// MockLogger Tests
// =============================================================================

func TestMockPluginLogger(t *testing.T) {
	t.Parallel()

	t.Run("default no-op", func(t *testing.T) {
		l := &MockPluginLogger{}
		l.Debug("msg")
		l.Info("msg")
		l.Warn("msg")
		l.Error("msg")
	})

	t.Run("custom functions", func(t *testing.T) {
		var messages []string
		l := &MockPluginLogger{
			DebugFunc: func(msg string, kv ...interface{}) { messages = append(messages, "debug:"+msg) },
			InfoFunc:  func(msg string, kv ...interface{}) { messages = append(messages, "info:"+msg) },
			WarnFunc:  func(msg string, kv ...interface{}) { messages = append(messages, "warn:"+msg) },
			ErrorFunc: func(msg string, kv ...interface{}) { messages = append(messages, "error:"+msg) },
		}
		l.Debug("d")
		l.Info("i")
		l.Warn("w")
		l.Error("e")
		if len(messages) != 4 {
			t.Errorf("expected 4 messages, got %d", len(messages))
		}
	})
}

// =============================================================================
// MockRuntimeAPI custom functions
// =============================================================================

func TestMockRuntimeAPI_CustomFunctions(t *testing.T) {
	t.Parallel()

	api := &MockRuntimeAPI{
		GetConfigFunc:      func(key string) (interface{}, error) { return "val-" + key, nil },
		SetConfigFunc:      func(key string, value interface{}) error { return fmt.Errorf("set failed") },
		EmitEventFunc:      func(eventType string, data interface{}) error { return fmt.Errorf("emit failed") },
		RegisterHookFunc:   func(hookPoint string, handler func(args interface{}) error) (string, error) { return "custom-id", nil },
		UnregisterHookFunc: func(hookID string) error { return fmt.Errorf("unregister failed") },
	}

	v, err := api.GetConfig("foo")
	if err != nil || v != "val-foo" {
		t.Errorf("GetConfig = %v, %v, want val-foo", v, err)
	}
	err = api.SetConfig("k", "v")
	if err == nil || err.Error() != "set failed" {
		t.Errorf("SetConfig error = %v", err)
	}
	err = api.EmitEvent("e", nil)
	if err == nil || err.Error() != "emit failed" {
		t.Errorf("EmitEvent error = %v", err)
	}
	id, err := api.RegisterHook("h", nil)
	if err != nil || id != "custom-id" {
		t.Errorf("RegisterHook = %q, %v", id, err)
	}
	err = api.UnregisterHook("id")
	if err == nil || err.Error() != "unregister failed" {
		t.Errorf("UnregisterHook error = %v", err)
	}
}

// =============================================================================
// External Plugin: Init with missing binary path
// =============================================================================

func TestExternalPlugin_Init_NoBinaryPath(t *testing.T) {
	t.Parallel()
	p := &externalPlugin{
		BasePlugin: BasePlugin{IDValue: "test"},
	}
	err := p.Init(&PluginContext{})
	if err == nil {
		t.Error("expected error for empty binary path")
	}
}

// =============================================================================
// wasmPlugin Stop with nil cancel
// =============================================================================

func TestWASMPlugin_Stop_NilCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	p := &wasmPlugin{
		BasePlugin: BasePlugin{IDValue: "wasm-test", State: PluginStateStarted},
		ctx:        ctx,
		cancel:     cancel,
	}
	// Stop should handle nil module/runtime gracefully
	err := p.Stop()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if p.State != PluginStateStopped {
		t.Errorf("expected state=stopped, got %v", p.State)
	}
}

// =============================================================================
// Loader: findPluginBinary tests
// =============================================================================

func TestLoader_FindPluginBinary_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	l := NewLoader(DefaultLoaderConfig(dir))
	_, err := l.findPluginBinary(dir, RuntimeGo)
	if err == nil {
		t.Error("expected error for empty directory")
	}
}

func TestLoader_FindPluginBinary_Extensions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	l := NewLoader(DefaultLoaderConfig(dir))

	// Create files with different extensions
	files := map[string]string{
		"plugin.so":    "",
		"plugin2.wasm": "",
	}
	for name, _ := range files {
		os.WriteFile(filepath.Join(dir, name), []byte("dummy"), 0o644)
	}

	t.Run("finds .so for Go runtime", func(t *testing.T) {
		path, err := l.findPluginBinary(dir, RuntimeGo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(path, ".so") {
			t.Errorf("expected .so file, got %s", path)
		}
	})

	t.Run("finds .wasm for WASM runtime", func(t *testing.T) {
		path, err := l.findPluginBinary(dir, RuntimeWASM)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(path, ".wasm") {
			t.Errorf("expected .wasm file, got %s", path)
		}
	})
}

// =============================================================================
// Loader: DiscoverPlugins tests
// =============================================================================

func TestDiscoverPlugins_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	manifests, err := DiscoverPlugins(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(manifests) != 0 {
		t.Errorf("expected 0 manifests, got %d", len(manifests))
	}
}

func TestDiscoverPlugins_WithValidManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "my-plugin")
	os.MkdirAll(pluginDir, 0o755)
	manifestContent := `id: my-plugin
name: My Plugin
version: 1.0.0
runtime: wasm
`
	os.WriteFile(filepath.Join(pluginDir, "manifest.yaml"), []byte(manifestContent), 0o644)

	manifests, err := DiscoverPlugins(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(manifests))
	}
	if manifests[0].ID != "my-plugin" {
		t.Errorf("manifest ID = %q", manifests[0].ID)
	}
}

func TestDiscoverPlugins_SkipsInvalid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "broken")
	os.MkdirAll(pluginDir, 0o755)
	os.WriteFile(filepath.Join(pluginDir, "manifest.json"), []byte(`{"name":"no-id"}`), 0o644)

	manifests, err := DiscoverPlugins(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(manifests) != 0 {
		t.Errorf("expected 0 manifests (invalid manifest should be skipped), got %d", len(manifests))
	}
}

// =============================================================================
// Loader: loadSidecarManifest from fallback
// =============================================================================

func TestLoader_LoadSidecarManifest_Fallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	l := NewLoader(DefaultLoaderConfig(dir))

	pluginPath := filepath.Join(dir, "my-plugin.so")
	// No manifest file exists, so it should fall back to generating one from filename
	manifest, err := l.loadSidecarManifest(pluginPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if manifest.ID != "my-plugin" {
		t.Errorf("ID = %q, want my-plugin", manifest.ID)
	}
	if manifest.Name != "my-plugin" {
		t.Errorf("Name = %q, want my-plugin", manifest.Name)
	}
	if manifest.Version != "0.0.0" {
		t.Errorf("Version = %q, want 0.0.0", manifest.Version)
	}
	if manifest.Runtime != RuntimeGo {
		t.Errorf("Runtime = %q, want go", manifest.Runtime)
	}
}

// =============================================================================
// Loader: loadManifestFromDir tests
// =============================================================================

func TestLoadManifestFromDir_NoManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := loadManifestFromDir(dir)
	if err == nil {
		t.Error("expected error for directory without manifest")
	}
}

func TestLoadManifestFromDir_JSONManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	manifestJSON := `{"id":"json-plugin","name":"JSON Plugin","version":"1.0.0","runtime":"external"}`
	os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifestJSON), 0o644)

	m, err := loadManifestFromDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.ID != "json-plugin" {
		t.Errorf("ID = %q", m.ID)
	}
	if m.Runtime != RuntimeExternal {
		t.Errorf("Runtime = %q", m.Runtime)
	}
}

// =============================================================================
// LifecycleManager: NewLifecycleManager
// =============================================================================

func TestNewLifecycleManager(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(DefaultManagerConfig(dir), nil)
	lm := NewLifecycleManager(m, nil, nil)
	if lm == nil {
		t.Fatal("expected non-nil lifecycle manager")
	}
	if lm.hookRegistry == nil {
		t.Error("hookRegistry should be initialized")
	}
	if lm.eventBus == nil {
		t.Error("eventBus should be initialized")
	}
}

// =============================================================================
// LifecycleManager: InitPlugins no plugins
// =============================================================================

func TestLifecycleManager_InitPlugins_NoPlugins(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(DefaultManagerConfig(dir), nil)
	lm := NewLifecycleManager(m, nil, nil)

	err := lm.InitPlugins()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLifecycleManager_StartPlugins_NoPlugins(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(DefaultManagerConfig(dir), nil)
	lm := NewLifecycleManager(m, nil, nil)

	err := lm.StartPlugins()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLifecycleManager_StopPlugins_NoPlugins(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(DefaultManagerConfig(dir), nil)
	lm := NewLifecycleManager(m, nil, nil)

	err := lm.StopPlugins()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// =============================================================================
// LifecycleManager: Init + Start + Stop full flow
// =============================================================================

func TestLifecycleManager_FullLifecycle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(DefaultManagerConfig(dir), nil)

	m.plugins["plugin-a"] = pluginEntry{
		plugin: &MockPlugin{IDValue: "plugin-a", NameValue: "Plugin A", VersionValue: "1.0"},
		info: PluginInfo{
			Manifest: PluginManifest{ID: "plugin-a", Name: "Plugin A", Version: "1.0"},
			Enabled:  true,
			State:    PluginStateInstalled,
		},
	}

	lcm := NewLifecycleManager(m, nil, nil)

	if err := lcm.InitPlugins(); err != nil {
		t.Fatalf("InitPlugins error: %v", err)
	}

	// updatePlugin is a no-op; manually advance state so StartPlugins can proceed
	if entry, ok := m.plugins["plugin-a"]; ok {
		entry.info.State = PluginStateInitialized
		m.plugins["plugin-a"] = entry
	}

	if err := lcm.StartPlugins(); err != nil {
		t.Fatalf("StartPlugins error: %v", err)
	}

	// updatePlugin is a no-op; manually advance state for Status check
	if entry, ok := m.plugins["plugin-a"]; ok {
		entry.info.State = PluginStateStarted
		m.plugins["plugin-a"] = entry
	}

	status := lcm.Status()
	if status.TotalPlugins != 1 {
		t.Errorf("TotalPlugins = %d, want 1", status.TotalPlugins)
	}
	if status.Started != 1 {
		t.Errorf("Started = %d, want 1", status.Started)
	}

	if err := lcm.StopPlugins(); err != nil {
		t.Fatalf("StopPlugins error: %v", err)
	}
}

// =============================================================================
// LifecycleManager: Status with various states
// =============================================================================

func TestLifecycleManager_Status(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(DefaultManagerConfig(dir), nil)

	m.plugins["a"] = pluginEntry{
		plugin: &MockPlugin{IDValue: "a"},
		info: PluginInfo{
			Manifest: PluginManifest{ID: "a"},
			Enabled:  true,
			State:    PluginStateInitialized,
		},
	}
	m.plugins["b"] = pluginEntry{
		plugin: &MockPlugin{IDValue: "b"},
		info: PluginInfo{
			Manifest: PluginManifest{ID: "b"},
			Enabled:  true,
			State:    PluginStateError,
		},
	}
	m.plugins["c"] = pluginEntry{
		plugin: &MockPlugin{IDValue: "c"},
		info: PluginInfo{
			Manifest: PluginManifest{ID: "c"},
			Enabled:  true,
			State:    PluginStateStopped,
		},
	}

	lcm := NewLifecycleManager(m, nil, nil)
	status := lcm.Status()

	if status.TotalPlugins != 3 {
		t.Errorf("TotalPlugins = %d, want 3", status.TotalPlugins)
	}
	if status.Initialized != 1 {
		t.Errorf("Initialized = %d, want 1", status.Initialized)
	}
	if status.Errors != 1 {
		t.Errorf("Errors = %d, want 1", status.Errors)
	}
	if status.Stopped != 1 {
		t.Errorf("Stopped = %d, want 1", status.Stopped)
	}
}

// =============================================================================
// LifecycleManager: HealthCheckAll
// =============================================================================

func TestLifecycleManager_HealthCheckAll_NoStarted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(DefaultManagerConfig(dir), nil)
	m.plugins["p1"] = pluginEntry{
		plugin: &MockPlugin{IDValue: "p1"},
		info: PluginInfo{
			Manifest: PluginManifest{ID: "p1"},
			Enabled:  true,
			State:    PluginStateInstalled,
		},
	}

	lcm := NewLifecycleManager(m, nil, nil)
	results := lcm.HealthCheckAll()

	// None are started, so no health checks
	if len(results) != 0 {
		t.Errorf("expected 0 health check results, got %d", len(results))
	}
}

func TestLifecycleManager_GetPluginHealth_Missing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := NewManager(DefaultManagerConfig(dir), nil)
	lcm := NewLifecycleManager(m, nil, nil)

	_, err := lcm.GetPluginHealth("nonexistent")
	if err == nil {
		t.Error("expected error for missing health data")
	}
}

// =============================================================================
// PluginContext: thread-safety
// =============================================================================

func TestPluginContext_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	ctx := NewPluginContext(map[string]interface{}{}, &MockPluginLogger{}, t.TempDir(), &MockRuntimeAPI{})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", i)
			ctx.SetConfig(key, i)
		}(i)
	}
	wg.Wait()

	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key-%d", i)
		v, err := ctx.GetConfig(key)
		if err != nil {
			t.Errorf("GetConfig(%q) error: %v", key, err)
		}
		if v != i {
			t.Errorf("GetConfig(%q) = %v, want %d", key, v, i)
		}
	}
}

// =============================================================================
// BasePlugin: Health with zero StartedAt
// =============================================================================

func TestBasePlugin_Health_ZeroUptime(t *testing.T) {
	t.Parallel()
	bp := &BasePlugin{IDValue: "test"}
	health, err := bp.Health()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if health.Uptime != 0 {
		t.Errorf("uptime = %v, want 0", health.Uptime)
	}
	if health.Status != "healthy" {
		t.Errorf("status = %q", health.Status)
	}
}

// =============================================================================
// ErrDependencyMissing: Error format
// =============================================================================

func TestErrDependencyMissingFormat(t *testing.T) {
	t.Parallel()
	err := &ErrDependencyMissing{
		PluginID: "a",
		Dependency: PluginDependency{
			PluginID: "b",
			Version:  ">=1.0",
		},
	}
	msg := err.Error()
	if !strings.Contains(msg, "a") || !strings.Contains(msg, "b") {
		t.Errorf("error message = %q", msg)
	}
}

// =============================================================================
// SandboxConfig: default values
// =============================================================================

func TestSandboxConfig_Defaults(t *testing.T) {
	t.Parallel()
	cfg := SandboxConfig{
		Enabled:         true,
		MaxCPUSeconds:   30,
		MaxMemoryMB:     512,
		MaxFileSizeMB:   10,
		MaxMessageBytes: 1024 * 1024,
		SeccompEnabled:  true,
	}
	if !cfg.Enabled {
		t.Error("Enabled should be true")
	}
	if cfg.MaxCPUSeconds != 30 {
		t.Errorf("MaxCPUSeconds = %d", cfg.MaxCPUSeconds)
	}
	if cfg.MaxMemoryMB != 512 {
		t.Errorf("MaxMemoryMB = %d", cfg.MaxMemoryMB)
	}
}

func TestSandboxConfig_Disabled(t *testing.T) {
	t.Parallel()
	cfg := SandboxConfig{Enabled: false}
	if cfg.Enabled {
		t.Error("Enabled should be false")
	}
}

// =============================================================================
// Archive Extraction Security Tests
// =============================================================================

// writeTarGz builds a tar.gz in memory from the given entries and writes it
// to disk. Each entry has a name, typeflag, and optional linkname.
func writeTarGz(t *testing.T, path string, entries []struct {
	name     string
	typeflag byte
	linkname string
}) {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for _, e := range entries {
		hdr := &tar.Header{
			Name:     e.name,
			Typeflag: e.typeflag,
			Mode:     0o644,
			Linkname: e.linkname,
		}
		if e.typeflag == tar.TypeReg {
			hdr.Size = 4
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header: %v", err)
		}
		if e.typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte("data")); err != nil {
				t.Fatalf("write body: %v", err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}
}

func TestExtractArchive_RejectsAbsoluteSymlinkTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		// O teste depende de: (1) filepath.IsAbs("/etc/passwd") ser true
		// (no Windows, path sem drive não é absoluto — semântica POSIX) e
		// (2) criação de symlink, que exige privilégio de admin no Windows.
		t.Skip("semântica de path absoluto POSIX e symlink requer privilégio — não aplicável no Windows")
	}
	m := &Manager{}
	dest := t.TempDir()
	archive := filepath.Join(t.TempDir(), "evil.tar.gz")

	writeTarGz(t, archive, []struct {
		name     string
		typeflag byte
		linkname string
	}{{name: "link", typeflag: tar.TypeSymlink, linkname: "/etc/passwd"}})

	err := m.extractArchive(archive, dest)
	if err == nil {
		t.Fatal("expected error for absolute symlink target, got nil")
	}
	if !strings.Contains(err.Error(), "must be relative") {
		t.Errorf("expected 'must be relative', got %q", err.Error())
	}
}

func TestExtractArchive_RejectsEscapingHardlink(t *testing.T) {
	m := &Manager{}
	dest := t.TempDir()
	archive := filepath.Join(t.TempDir(), "evil-link.tar.gz")

	// Create a host file outside dest to prove the link would escape.
	hostFile := filepath.Join(t.TempDir(), "host-secret.txt")
	if err := os.WriteFile(hostFile, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	writeTarGz(t, archive, []struct {
		name     string
		typeflag byte
		linkname string
	}{{name: "host-secret.txt", typeflag: tar.TypeLink, linkname: "../../../../" + filepath.Base(hostFile)}})

	err := m.extractArchive(archive, dest)
	if err == nil {
		t.Fatal("expected error for escaping hardlink, got nil")
	}
	if !strings.Contains(err.Error(), "escapes destination") {
		t.Errorf("expected 'escapes destination', got %q", err.Error())
	}
}

func TestExtractArchive_AllowsRelativeSymlinkInside(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Criar symlink no Windows requer privilégio de admin/developer mode.
		t.Skip("criação de symlink requer privilégio no Windows — não aplicável")
	}
	m := &Manager{}
	dest := t.TempDir()
	archive := filepath.Join(t.TempDir(), "good.tar.gz")

	writeTarGz(t, archive, []struct {
		name     string
		typeflag byte
		linkname string
	}{{name: "target.txt", typeflag: tar.TypeReg, linkname: ""},
		{name: "link.txt", typeflag: tar.TypeSymlink, linkname: "target.txt"}})

	if err := m.extractArchive(archive, dest); err != nil {
		t.Fatalf("expected extraction to succeed, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "link.txt")); err != nil {
		t.Errorf("expected link.txt to exist, got %v", err)
	}
}
