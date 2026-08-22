package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestNewRuntime(t *testing.T) {
	t.Parallel()
	r := New()
	if r == nil {
		t.Fatal("New() returned nil")
	}
	if r.state.CurrentState != StateUninitialized {
		t.Errorf("initial state = %v, want %v", r.state.CurrentState, StateUninitialized)
	}
	if r.config != DefaultRuntimeConfig() {
		t.Errorf("config not default")
	}
}

func TestDefaultRuntimeConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultRuntimeConfig()
	if cfg.Name != "cosca" {
		t.Errorf("Name = %q, want %q", cfg.Name, "cosca")
	}
	if cfg.Version != "0.0.0" {
		t.Errorf("Version = %q, want %q", cfg.Version, "0.0.0")
	}
	if cfg.ComponentTimeout != 30*time.Second {
		t.Errorf("ComponentTimeout = %v", cfg.ComponentTimeout)
	}
	if cfg.ShutdownTimeout != time.Minute {
		t.Errorf("ShutdownTimeout = %v", cfg.ShutdownTimeout)
	}
	if cfg.HealthCheckInterval != 30*time.Second {
		t.Errorf("HealthCheckInterval = %v", cfg.HealthCheckInterval)
	}
	if cfg.EnableMetrics != true {
		t.Error("EnableMetrics should be true")
	}
	if cfg.EnableDaemon != true {
		t.Error("EnableDaemon should be true by default")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
}

func TestWithLogger(t *testing.T) {
	t.Parallel()
	r := New(WithLogger(zerolog.Nop()))
	if r == nil {
		t.Fatal("New(WithLogger(...)) returned nil")
	}
	// zerolog.Logger is a value type, cannot be nil
}

func TestWithConfig(t *testing.T) {
	t.Parallel()
	cfg := RuntimeConfig{
		Name:    "test-app",
		Version: "2.0.0",
	}
	r := New(WithConfig(cfg))
	if r.config.Name != "test-app" {
		t.Errorf("Name = %q, want %q", r.config.Name, "test-app")
	}
	if r.config.Version != "2.0.0" {
		t.Errorf("Version = %q", r.config.Version)
	}
}

func TestWithSubsystem(t *testing.T) {
	t.Parallel()
	mock := &MockSubsystem{
		NameFunc: func() string { return "knowledge" },
	}
	r := New(WithSubsystem(mock))
	sub := r.Subsystem("knowledge")
	if sub == nil {
		t.Fatal("Subsystem should not be nil")
	}
	if sub.Name() != "knowledge" {
		t.Errorf("Name = %q, want %q", sub.Name(), "knowledge")
	}
}

func TestWithSubsystemUnknownIgnored(t *testing.T) {
	t.Parallel()
	mock := &MockSubsystem{
		NameFunc: func() string { return "unknown_subsystem" },
	}
	r := New(WithSubsystem(mock))
	sub := r.Subsystem("unknown_subsystem")
	if sub != nil {
		t.Error("Unknown subsystem should be nil")
	}
}

func TestRegisterSubsystems(t *testing.T) {
	t.Parallel()
	r := New()
	r.RegisterKnowledge(&MockSubsystem{NameFunc: func() string { return "knowledge" }})
	r.RegisterDiscovery(&MockSubsystem{NameFunc: func() string { return "discovery" }})
	r.RegisterMemory(&MockSubsystem{NameFunc: func() string { return "memory" }})
	r.RegisterCache(&MockSubsystem{NameFunc: func() string { return "cache" }})
	r.RegisterPlugins(&MockSubsystem{NameFunc: func() string { return "plugins" }})
	r.RegisterEditors(&MockSubsystem{NameFunc: func() string { return "editors" }})
	r.RegisterWatcher(&MockSubsystem{NameFunc: func() string { return "watcher" }})

	if r.Subsystem("knowledge") == nil {
		t.Error("knowledge should be registered")
	}
	if r.Subsystem("discovery") == nil {
		t.Error("discovery should be registered")
	}
	if r.Subsystem("memory") == nil {
		t.Error("memory should be registered")
	}
	if r.Subsystem("cache") == nil {
		t.Error("cache should be registered")
	}
	if r.Subsystem("plugins") == nil {
		t.Error("plugins should be registered")
	}
	if r.Subsystem("editors") == nil {
		t.Error("editors should be registered")
	}
	if r.Subsystem("watcher") == nil {
		t.Error("watcher should be registered")
	}
}

func TestSubsystemNotFound(t *testing.T) {
	t.Parallel()
	r := New()
	sub := r.Subsystem("nonexistent")
	if sub != nil {
		t.Error("Nonexistent subsystem should return nil")
	}
}

func TestStateAccessor(t *testing.T) {
	t.Parallel()
	r := New()
	rs := r.State()
	if rs == nil {
		t.Fatal("State() returned nil")
	}
}

func TestMetricsAccessor(t *testing.T) {
	t.Parallel()
	r := New()
	m := r.Metrics()
	if m == nil {
		t.Fatal("Metrics() returned nil")
	}
}

func TestEventsAccessor(t *testing.T) {
	t.Parallel()
	r := New()
	eb := r.Events()
	if eb == nil {
		t.Fatal("Events() returned nil")
	}
}

func TestLifecycleAccessor(t *testing.T) {
	t.Parallel()
	r := New()
	lc := r.Lifecycle()
	if lc == nil {
		t.Fatal("Lifecycle() returned nil")
	}
}

func TestConfigAccessor(t *testing.T) {
	t.Parallel()
	r := New()
	cfg := r.Config()
	if cfg.Name != "cosca" {
		t.Errorf("Config().Name = %q", cfg.Name)
	}
}

func TestShutdownCh(t *testing.T) {
	t.Parallel()
	r := New()
	ch := r.ShutdownCh()
	if ch == nil {
		t.Fatal("ShutdownCh returned nil")
	}
}

func TestContext(t *testing.T) {
	t.Parallel()
	r := New()
	ctx := r.Context()
	if ctx == nil {
		t.Fatal("Context returned nil")
	}
}

func TestHealth(t *testing.T) {
	t.Parallel()
	r := New()
	h := r.Health()
	if h != StatusUnknown {
		t.Errorf("Health = %v, want %v", h, StatusUnknown)
	}
}

func TestHealthReport(t *testing.T) {
	t.Parallel()
	r := New()
	report := r.HealthReport()
	if report == nil {
		t.Fatal("HealthReport returned nil")
	}
	if report["version"] != "0.0.0" {
		t.Errorf("version = %v", report["version"])
	}
	if report["health"] != StatusUnknown {
		t.Errorf("health = %v", report["health"])
	}
}

func TestStartWithoutInit(t *testing.T) {
	t.Parallel()
	r := New()
	ctx := context.Background()
	err := r.Start(ctx)
	if err != nil {
		t.Errorf("Start should succeed with nil subsystems: %v", err)
	}
	// After Start, state should be running
	if r.state.Current() != StateRunning {
		t.Errorf("state after Start = %v, want %v", r.state.Current(), StateRunning)
	}
}

func TestStopWithoutStart(t *testing.T) {
	t.Parallel()
	r := New()
	ctx := context.Background()
	err := r.Stop(ctx)
	if err != nil {
		// Should handle already stopped gracefully
		t.Logf("Stop error: %v", err)
	}
}

func TestRuntimeDaemonAccess(t *testing.T) {
	t.Parallel()
	r := New()
	d := r.Daemon()
	if d != nil {
		t.Error("Daemon should be nil before registration")
	}
}

func TestRegisterDaemon(t *testing.T) {
	t.Parallel()
	r := New()
	cfg := DefaultDaemonConfig()
	d := NewDaemon(r, cfg)
	r.RegisterDaemon(d)
	if r.Daemon() == nil {
		t.Error("Daemon should not be nil after registration")
	}
}
