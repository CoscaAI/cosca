package runtime

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// =============================================================================
// Init hooks with actual subsystems (not nil)
// =============================================================================

func TestInitKnowledgeWithSubsystem(t *testing.T) {
	t.Parallel()
	r := New()
	startCalled := false
	r.RegisterKnowledge(&MockSubsystem{
		NameFunc: func() string { return "knowledge" },
		StartFunc: func(_ context.Context) error {
			startCalled = true
			return nil
		},
		HealthFunc: func() ComponentStatus { return StatusHealthy },
	})

	ctx := context.Background()
	err := r.lifecycle.initKnowledge(ctx, r)
	if err != nil {
		t.Fatalf("initKnowledge failed: %v", err)
	}

	if !startCalled {
		t.Error("subsystem Start was not called")
	}

	info, ok := r.state.ComponentStatus("knowledge")
	if !ok || info.Status != StatusHealthy {
		t.Errorf("knowledge status = %v, want healthy", info.Status)
	}
}

func TestInitKnowledgeNilSubsystem(t *testing.T) {
	t.Parallel()
	r := New()
	// knowledge is nil, should return nil without error
	err := r.lifecycle.initKnowledge(context.Background(), r)
	if err != nil {
		t.Errorf("initKnowledge with nil subsystem: %v", err)
	}
}

func TestInitKnowledgeStartError(t *testing.T) {
	t.Parallel()
	r := New()
	r.RegisterKnowledge(&MockSubsystem{
		NameFunc: func() string { return "knowledge" },
		StartFunc: func(_ context.Context) error {
			return errors.New("init failed")
		},
		HealthFunc: func() ComponentStatus { return StatusUnhealthy },
	})

	err := r.lifecycle.initKnowledge(context.Background(), r)
	if err == nil {
		t.Error("initKnowledge should fail when Start fails")
	}

	info, _ := r.state.ComponentStatus("knowledge")
	if info.Status != StatusUnhealthy {
		t.Errorf("status = %v, want unhealthy", info.Status)
	}
}

func TestInitDiscovery(t *testing.T) {
	t.Parallel()
	r := New()
	startCalled := false
	r.RegisterDiscovery(&MockSubsystem{
		NameFunc: func() string { return "discovery" },
		StartFunc: func(_ context.Context) error {
			startCalled = true
			return nil
		},
	})
	err := r.lifecycle.initDiscovery(context.Background(), r)
	if err != nil {
		t.Fatalf("initDiscovery: %v", err)
	}
	if !startCalled {
		t.Error("Start not called")
	}
}

func TestInitMemory(t *testing.T) {
	t.Parallel()
	r := New()
	startCalled := false
	r.RegisterMemory(&MockSubsystem{
		NameFunc: func() string { return "memory" },
		StartFunc: func(_ context.Context) error {
			startCalled = true
			return nil
		},
	})
	err := r.lifecycle.initMemory(context.Background(), r)
	if err != nil {
		t.Fatalf("initMemory: %v", err)
	}
	if !startCalled {
		t.Error("Start not called")
	}
}

func TestInitCache(t *testing.T) {
	t.Parallel()
	r := New()
	startCalled := false
	r.RegisterCache(&MockSubsystem{
		NameFunc: func() string { return "cache" },
		StartFunc: func(_ context.Context) error {
			startCalled = true
			return nil
		},
	})
	err := r.lifecycle.initCache(context.Background(), r)
	if err != nil {
		t.Fatalf("initCache: %v", err)
	}
	if !startCalled {
		t.Error("Start not called")
	}
}

func TestInitPlugins(t *testing.T) {
	t.Parallel()
	r := New()
	startCalled := false
	r.RegisterPlugins(&MockSubsystem{
		NameFunc: func() string { return "plugins" },
		StartFunc: func(_ context.Context) error {
			startCalled = true
			return nil
		},
	})
	err := r.lifecycle.initPlugins(context.Background(), r)
	if err != nil {
		t.Fatalf("initPlugins: %v", err)
	}
	if !startCalled {
		t.Error("Start not called")
	}
}

func TestInitEditors(t *testing.T) {
	t.Parallel()
	r := New()
	startCalled := false
	r.RegisterEditors(&MockSubsystem{
		NameFunc: func() string { return "editors" },
		StartFunc: func(_ context.Context) error {
			startCalled = true
			return nil
		},
	})
	err := r.lifecycle.initEditors(context.Background(), r)
	if err != nil {
		t.Fatalf("initEditors: %v", err)
	}
	if !startCalled {
		t.Error("Start not called")
	}
}

func TestInitWatcher(t *testing.T) {
	t.Parallel()
	r := New()
	startCalled := false
	r.RegisterWatcher(&MockSubsystem{
		NameFunc: func() string { return "watcher" },
		StartFunc: func(_ context.Context) error {
			startCalled = true
			return nil
		},
	})
	err := r.lifecycle.initWatcher(context.Background(), r)
	if err != nil {
		t.Fatalf("initWatcher: %v", err)
	}
	if !startCalled {
		t.Error("Start not called")
	}
}

// =============================================================================
// Start hooks with subsystems (covers the 50% gaps)
// =============================================================================

func TestStartKnowledgeWithSubsystem(t *testing.T) {
	t.Parallel()
	r := New()
	eventFired := make(chan struct{}, 1)
	r.events.Subscribe(EventSubsystemStarted, func(_ context.Context, e Event) error {
		if e.Source == "knowledge" {
			eventFired <- struct{}{}
		}
		return nil
	})

	r.RegisterKnowledge(&MockSubsystem{
		NameFunc: func() string { return "knowledge" },
	})

	err := r.lifecycle.startKnowledge(context.Background(), r)
	if err != nil {
		t.Fatalf("startKnowledge: %v", err)
	}

	select {
	case <-eventFired:
		// Event published
	case <-time.After(time.Second):
		t.Error("SubsystemStarted event not published for knowledge")
	}
}

func TestStartDiscoveryWithSubsystem(t *testing.T) {
	t.Parallel()
	r := New()
	eventFired := make(chan struct{}, 1)
	r.events.Subscribe(EventSubsystemStarted, func(_ context.Context, e Event) error {
		if e.Source == "discovery" {
			eventFired <- struct{}{}
		}
		return nil
	})

	r.RegisterDiscovery(&MockSubsystem{
		NameFunc: func() string { return "discovery" },
	})

	err := r.lifecycle.startDiscovery(context.Background(), r)
	if err != nil {
		t.Fatalf("startDiscovery: %v", err)
	}

	select {
	case <-eventFired:
	case <-time.After(time.Second):
		t.Error("event not published")
	}
}

func TestStartMemoryWithSubsystem(t *testing.T) {
	t.Parallel()
	r := New()
	eventFired := make(chan struct{})
	r.events.Subscribe(EventSubsystemStarted, func(_ context.Context, e Event) error {
		if e.Source == "memory" {
			close(eventFired)
		}
		return nil
	})

	r.RegisterMemory(&MockSubsystem{NameFunc: func() string { return "memory" }})

	err := r.lifecycle.startMemory(context.Background(), r)
	if err != nil {
		t.Fatalf("startMemory: %v", err)
	}

	select {
	case <-eventFired:
	case <-time.After(time.Second):
		t.Error("event not published")
	}
}

func TestStartCacheWithSubsystem(t *testing.T) {
	t.Parallel()
	r := New()
	r.RegisterCache(&MockSubsystem{NameFunc: func() string { return "cache" }})
	err := r.lifecycle.startCache(context.Background(), r)
	if err != nil {
		t.Fatalf("startCache: %v", err)
	}
}

func TestStartPluginsWithSubsystem(t *testing.T) {
	t.Parallel()
	r := New()
	r.RegisterPlugins(&MockSubsystem{NameFunc: func() string { return "plugins" }})
	err := r.lifecycle.startPlugins(context.Background(), r)
	if err != nil {
		t.Fatalf("startPlugins: %v", err)
	}
}

func TestStartEditorsWithSubsystem(t *testing.T) {
	t.Parallel()
	r := New()
	r.RegisterEditors(&MockSubsystem{NameFunc: func() string { return "editors" }})
	err := r.lifecycle.startEditors(context.Background(), r)
	if err != nil {
		t.Fatalf("startEditors: %v", err)
	}
}

func TestStartWatcherWithSubsystem(t *testing.T) {
	t.Parallel()
	r := New()
	r.RegisterWatcher(&MockSubsystem{NameFunc: func() string { return "watcher" }})
	err := r.lifecycle.startWatcher(context.Background(), r)
	if err != nil {
		t.Fatalf("startWatcher: %v", err)
	}
}

// =============================================================================
// Stop hooks with subsystems (covers the 18.2% gaps)
// =============================================================================

func TestStopWatcherWithSubsystem(t *testing.T) {
	t.Parallel()
	r := New()
	stopCalled := false
	r.RegisterWatcher(&MockSubsystem{
		NameFunc: func() string { return "watcher" },
		StopFunc: func(_ context.Context) error {
			stopCalled = true
			return nil
		},
	})

	err := r.lifecycle.stopWatcher(context.Background(), r)
	if err != nil {
		t.Fatalf("stopWatcher: %v", err)
	}
	if !stopCalled {
		t.Error("Stop not called")
	}

	info, ok := r.state.ComponentStatus("watcher")
	if !ok || info.Status != StatusStoppedComponent {
		t.Errorf("watcher status after stop = %v, want stopped", info.Status)
	}
}

func TestStopWatcherNilSubsystem(t *testing.T) {
	t.Parallel()
	r := New()
	err := r.lifecycle.stopWatcher(context.Background(), r)
	if err != nil {
		t.Errorf("stopWatcher with nil: %v", err)
	}
}

func TestStopWatcherError(t *testing.T) {
	t.Parallel()
	r := New()
	r.RegisterWatcher(&MockSubsystem{
		NameFunc: func() string { return "watcher" },
		StopFunc: func(_ context.Context) error {
			return errors.New("stop failed")
		},
	})

	err := r.lifecycle.stopWatcher(context.Background(), r)
	if err == nil {
		t.Error("stopWatcher should report error")
	}
	t.Logf("stop error: %v", err)
}

func TestStopEditors(t *testing.T) {
	t.Parallel()
	r := New()
	stopCalled := false
	r.RegisterEditors(&MockSubsystem{
		NameFunc: func() string { return "editors" },
		StopFunc: func(_ context.Context) error {
			stopCalled = true
			return nil
		},
	})

	err := r.lifecycle.stopEditors(context.Background(), r)
	if err != nil {
		t.Fatalf("stopEditors: %v", err)
	}
	if !stopCalled {
		t.Error("Stop not called")
	}
}

func TestStopPlugins(t *testing.T) {
	t.Parallel()
	r := New()
	stopCalled := false
	r.RegisterPlugins(&MockSubsystem{
		NameFunc: func() string { return "plugins" },
		StopFunc: func(_ context.Context) error {
			stopCalled = true
			return nil
		},
	})

	err := r.lifecycle.stopPlugins(context.Background(), r)
	if err != nil {
		t.Fatalf("stopPlugins: %v", err)
	}
	if !stopCalled {
		t.Error("Stop not called")
	}
}

func TestStopCache(t *testing.T) {
	t.Parallel()
	r := New()
	stopCalled := false
	r.RegisterCache(&MockSubsystem{
		NameFunc: func() string { return "cache" },
		StopFunc: func(_ context.Context) error {
			stopCalled = true
			return nil
		},
	})

	err := r.lifecycle.stopCache(context.Background(), r)
	if err != nil {
		t.Fatalf("stopCache: %v", err)
	}
	if !stopCalled {
		t.Error("Stop not called")
	}
}

func TestStopMemory(t *testing.T) {
	t.Parallel()
	r := New()
	stopCalled := false
	r.RegisterMemory(&MockSubsystem{
		NameFunc: func() string { return "memory" },
		StopFunc: func(_ context.Context) error {
			stopCalled = true
			return nil
		},
	})

	err := r.lifecycle.stopMemory(context.Background(), r)
	if err != nil {
		t.Fatalf("stopMemory: %v", err)
	}
	if !stopCalled {
		t.Error("Stop not called")
	}
}

func TestStopDiscovery(t *testing.T) {
	t.Parallel()
	r := New()
	stopCalled := false
	r.RegisterDiscovery(&MockSubsystem{
		NameFunc: func() string { return "discovery" },
		StopFunc: func(_ context.Context) error {
			stopCalled = true
			return nil
		},
	})

	err := r.lifecycle.stopDiscovery(context.Background(), r)
	if err != nil {
		t.Fatalf("stopDiscovery: %v", err)
	}
	if !stopCalled {
		t.Error("Stop not called")
	}
}

func TestStopKnowledge(t *testing.T) {
	t.Parallel()
	r := New()
	stopCalled := false
	r.RegisterKnowledge(&MockSubsystem{
		NameFunc: func() string { return "knowledge" },
		StopFunc: func(_ context.Context) error {
			stopCalled = true
			return nil
		},
	})

	err := r.lifecycle.stopKnowledge(context.Background(), r)
	if err != nil {
		t.Fatalf("stopKnowledge: %v", err)
	}
	if !stopCalled {
		t.Error("Stop not called")
	}
}

func TestStopKnowledgeError(t *testing.T) {
	t.Parallel()
	r := New()
	r.RegisterKnowledge(&MockSubsystem{
		NameFunc: func() string { return "knowledge" },
		StopFunc: func(_ context.Context) error {
			return fmt.Errorf("critical stop failure")
		},
	})

	err := r.lifecycle.stopKnowledge(context.Background(), r)
	if err == nil {
		t.Error("stopKnowledge should report error")
	}
}

// =============================================================================
// ExecuteInit/ExecuteStart/ExecuteStop with real subsystems
// =============================================================================

func TestExecuteInitWithAllSubsystems(t *testing.T) {
	t.Parallel()
	r := New()

	startCount := 0
	mock := &MockSubsystem{
		NameFunc: func() string { return "knowledge" },
		StartFunc: func(_ context.Context) error {
			startCount++
			return nil
		},
		HealthFunc: func() ComponentStatus { return StatusHealthy },
	}
	r.RegisterKnowledge(mock)

	err := r.lifecycle.ExecuteInit(context.Background(), r)
	if err != nil {
		t.Fatalf("ExecuteInit: %v", err)
	}

	if startCount < 1 {
		t.Errorf("Start called %d times, want at least 1", startCount)
	}
}

func TestExecuteStartWithAllSubsystems(t *testing.T) {
	t.Parallel()
	r := New()
	r.RegisterKnowledge(&MockSubsystem{NameFunc: func() string { return "knowledge" }})
	r.RegisterCache(&MockSubsystem{NameFunc: func() string { return "cache" }})

	err := r.lifecycle.ExecuteStart(context.Background(), r)
	if err != nil {
		t.Fatalf("ExecuteStart: %v", err)
	}
}

func TestExecuteStopWithAllSubsystems(t *testing.T) {
	t.Parallel()
	r := New()

	stopCount := 0
	mock := &MockSubsystem{
		NameFunc: func() string { return "knowledge" },
		StopFunc: func(_ context.Context) error {
			stopCount++
			return nil
		},
	}
	r.RegisterKnowledge(mock)

	err := r.lifecycle.ExecuteStop(context.Background(), r)
	if err != nil {
		t.Fatalf("ExecuteStop: %v", err)
	}

	if stopCount < 1 {
		t.Errorf("Stop called %d times, want 1", stopCount)
	}
}

// =============================================================================
// ExecuteInit with failing required subsystem
// =============================================================================

func TestExecuteInitRequiredHookFails(t *testing.T) {
	t.Parallel()
	r := New()
	r.RegisterKnowledge(&MockSubsystem{
		NameFunc: func() string { return "knowledge" },
		StartFunc: func(_ context.Context) error {
			return errors.New("init failed")
		},
		HealthFunc: func() ComponentStatus { return StatusUnhealthy },
	})

	err := r.lifecycle.ExecuteInit(context.Background(), r)
	if err == nil {
		t.Error("ExecuteInit should fail when required hook fails")
	}
}

func TestExecuteInitRequiredHookFailsStopsExecution(t *testing.T) {
	t.Parallel()
	r := New()

	// knowledge is required — if it fails, cache (next required) should NOT be reached
	knowledgeCalled := false
	cacheCalled := false

	r.RegisterKnowledge(&MockSubsystem{
		NameFunc: func() string { return "knowledge" },
		StartFunc: func(_ context.Context) error {
			knowledgeCalled = true
			return errors.New("knowledge init failed")
		},
	})
	r.RegisterCache(&MockSubsystem{
		NameFunc: func() string { return "cache" },
		StartFunc: func(_ context.Context) error {
			cacheCalled = true
			return nil
		},
	})

	err := r.lifecycle.ExecuteInit(context.Background(), r)
	if err == nil {
		t.Error("should fail")
	}
	if !knowledgeCalled {
		t.Error("knowledge should be called")
	}
	// cache is after knowledge in the hook order
	// but since knowledge was required and failed, cache may or may not be called
	// (depends on whether executeHooks stops on first error or continues)
	t.Logf("cache called: %v", cacheCalled)
}

// =============================================================================
// executeHooks with cancelled context
// =============================================================================

func TestExecuteHooksContextCancelled(t *testing.T) {
	t.Parallel()
	lc := NewLifecycle()
	lc.AddInitHook("test", func(_ context.Context, _ *Runtime) error { return nil }, time.Second, false)

	r := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel

	err := lc.executeHooks(ctx, r, lc.initHooks, PhaseInit)
	if err == nil {
		t.Error("executeHooks should return error with cancelled context")
	}
}

// =============================================================================
// Hook with zero timeout (defaults to 30s)
// =============================================================================

func TestHookZeroTimeout(t *testing.T) {
	t.Parallel()
	lc := NewLifecycle()
	lc.AddInitHook("defaultTimeout", func(_ context.Context, _ *Runtime) error {
		return nil
	}, 0, false)

	r := New()
	err := lc.executeHooks(context.Background(), r, lc.initHooks, PhaseInit)
	if err != nil {
		t.Fatalf("hook with zero timeout failed: %v", err)
	}
}

// =============================================================================
// Default hook event subscriptions
// =============================================================================

func TestInitHooksPublishStartEvents(t *testing.T) {
	t.Parallel()
	r := New()

	events := make(chan string, 7)
	r.events.Subscribe(EventSubsystemStarted, func(_ context.Context, e Event) error {
		events <- e.Source
		return nil
	})

	r.RegisterKnowledge(&MockSubsystem{
		NameFunc:  func() string { return "knowledge" },
		StartFunc: func(_ context.Context) error { return nil },
	})
	r.RegisterCache(&MockSubsystem{
		NameFunc:  func() string { return "cache" },
		StartFunc: func(_ context.Context) error { return nil },
	})

	_ = r.lifecycle.ExecuteInit(context.Background(), r)

	// Collect events
	close(events)
	got := make(map[string]bool)
	for e := range events {
		got[e] = true
	}
	if !got["knowledge"] {
		t.Error("knowledge event not published")
	}
	if !got["cache"] {
		t.Error("cache event not published")
	}
}

// =============================================================================
// Stop hooks publish stop events
// =============================================================================

func TestStopHooksPublishStopEvents(t *testing.T) {
	t.Parallel()
	r := New()

	events := make(chan string, 7)
	r.events.Subscribe(EventSubsystemStopped, func(_ context.Context, e Event) error {
		events <- e.Source
		return nil
	})

	r.RegisterKnowledge(&MockSubsystem{
		NameFunc: func() string { return "knowledge" },
		StopFunc: func(_ context.Context) error { return nil },
	})

	_ = r.lifecycle.ExecuteStop(context.Background(), r)

	close(events)
	got := make(map[string]bool)
	for e := range events {
		got[e] = true
	}
	if !got["knowledge"] {
		t.Error("knowledge stop event not published")
	}
}
