package telemetry

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	if cfg.Enabled != true {
		t.Error("Enabled should be true")
	}
	if cfg.QueueSize != 100 {
		t.Errorf("QueueSize = %d, want 100", cfg.QueueSize)
	}
	if cfg.Version != "0.0.0" {
		t.Errorf("Version = %q", cfg.Version)
	}
}

func TestNewTelemetryEnabled(t *testing.T) {
	t.Parallel()

	telem, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if telem == nil {
		t.Fatal("New() returned nil")
	}
	if !telem.Enabled() {
		t.Error("telemetry should be enabled by default")
	}
	if telem.InstanceID() == "" {
		t.Error("instance ID should not be empty")
	}
}

func TestWithLogger(t *testing.T) {
	t.Parallel()

	// Just ensure no panic
	telem, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	_ = telem
}

func TestWithConfig(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Enabled:   false,
		QueueSize: 50,
		Version:   "1.0.0",
	}
	telem, err := New(WithConfig(cfg))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if telem.Enabled() {
		t.Error("telemetry should be disabled")
	}
	if telem.queueSize != 50 {
		t.Errorf("queueSize = %d, want 50", telem.queueSize)
	}
	if telem.version != "1.0.0" {
		t.Errorf("version = %q", telem.version)
	}
}

func TestSetEnabled(t *testing.T) {
	t.Parallel()

	cfg := Config{Enabled: false}
	telem, err := New(WithConfig(cfg))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	telem.SetEnabled(true)
	if !telem.Enabled() {
		t.Error("should be enabled after SetEnabled(true)")
	}

	telem.SetEnabled(false)
	if telem.Enabled() {
		t.Error("should be disabled after SetEnabled(false)")
	}
}

func TestSetGlobal(t *testing.T) {
	t.Parallel()

	// Reset global
	globalTelemetry = nil
	SetGlobal(nil)
	if globalTelemetry != nil {
		t.Error("globalTelemetry should be nil")
	}

	// Just ensure no panic on emit with nil
	Emit("test", nil)
}

func TestInstanceID(t *testing.T) {
	t.Parallel()

	telem, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	id1 := telem.InstanceID()
	id2 := telem.InstanceID()
	if id1 != id2 {
		t.Error("InstanceID should be stable")
	}
}

func TestVersion(t *testing.T) {
	t.Parallel()

	telem, err := New(WithConfig(Config{Version: "2.0.0"}))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if telem.Version() != "2.0.0" {
		t.Errorf("Version() = %q", telem.Version())
	}
}

func TestEventConstructors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		event    Event
		wantType EventType
	}{
		{"command executed", NewCommandExecutedEvent("build", 100, true), EventCommandExecuted},
		{"index completed", NewIndexCompletedEvent("local", 10, 500, true), EventIndexCompleted},
		{"search performed", NewSearchPerformedEvent("code", 5, 200, true), EventSearchPerformed},
		{"context built", NewContextBuiltEvent("full", 1000, 300, true), EventContextBuilt},
		{"memory stored", NewMemoryStoredEvent("episodic", "fact", true), EventMemoryStored},
		{"plugin installed", NewPluginInstalledEvent("my-plugin", "1.0", true), EventPluginInstalled},
		{"error occurred", NewErrorOccurredEvent("build", "E001"), EventErrorOccurred},
		{"runtime started", NewRuntimeStartedEvent("1.0", "server"), EventRuntimeStarted},
		{"runtime stopped", NewRuntimeStoppedEvent(3600, "shutdown"), EventRuntimeStopped},
		{"provider called", NewProviderCalledEvent("openai", "gpt4", 1000, 500, true), EventProviderCalled},
		{"config changed", NewConfigChangedEvent("theme", "user"), EventConfigChanged},
		{"update checked", NewUpdateCheckedEvent("1.0", "2.0", true), EventUpdateChecked},
		{"sync completed", NewSyncCompletedEvent("pull", 10, 300, true), EventSyncCompleted},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.event.Type != tc.wantType {
				t.Errorf("Type = %q, want %q", tc.event.Type, tc.wantType)
			}
			if tc.event.Timestamp.IsZero() {
				t.Error("Timestamp should not be zero")
			}
		})
	}
}

func TestEventConstructorValues(t *testing.T) {
	t.Parallel()

	e := NewCommandExecutedEvent("build", 1500, true)
	if e.DurationMs != 1500 {
		t.Errorf("DurationMs = %d, want 1500", e.DurationMs)
	}
	if e.Metadata["command"] != "build" {
		t.Errorf("command = %q", e.Metadata["command"])
	}

	e2 := NewErrorOccurredEvent("run", "ERR_42")
	if e2.Success != false {
		t.Error("error events should have Success = false")
	}
}

func TestEventTimeDefaults(t *testing.T) {
	t.Parallel()

	e := NewIndexCompletedEvent("local", 5, 100, true)
	if e.Timestamp.IsZero() {
		t.Error("should set timestamp")
	}
}

func TestReporterConfigDefaults(t *testing.T) {
	t.Parallel()

	cfg := DefaultReporterConfig()
	if cfg.Endpoint != "https://telemetry.cosca.enterprise/v1/events" {
		t.Errorf("Endpoint = %q", cfg.Endpoint)
	}
	if cfg.BatchSize != 100 {
		t.Errorf("BatchSize = %d", cfg.BatchSize)
	}
	if cfg.FlushInterval != 5*time.Minute {
		t.Errorf("FlushInterval = %v", cfg.FlushInterval)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v", cfg.Timeout)
	}
}

func TestNewReporter(t *testing.T) {
	t.Parallel()

	telem, err := New(WithConfig(Config{Enabled: false}))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	r := NewReporter(telem, DefaultReporterConfig())
	if r == nil {
		t.Fatal("NewReporter returned nil")
	}
	if r.endpoint != "https://telemetry.cosca.enterprise/v1/events" {
		t.Errorf("endpoint = %q", r.endpoint)
	}
}

func TestReporterStartStop(t *testing.T) {
	t.Parallel()

	telem, err := New(WithConfig(Config{Enabled: false}))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	r := NewReporter(telem, ReporterConfig{
		Endpoint:      "",
		BatchSize:     10,
		FlushInterval: time.Hour,
	})

	r.Start()
	r.Stop()
}

func TestReporterEnqueue(t *testing.T) {
	t.Parallel()

	telem, _ := New(WithConfig(Config{Enabled: false}))
	r := NewReporter(telem, ReporterConfig{BatchSize: 100, FlushInterval: time.Hour})

	r.Enqueue(Event{Type: EventCommandExecuted})
	if r.QueueSize() != 1 {
		t.Errorf("queue size = %d, want 1", r.QueueSize())
	}
}

func TestReporterFlushEmpty(t *testing.T) {
	t.Parallel()

	telem, _ := New(WithConfig(Config{Enabled: false}))
	if telem == nil {
		t.Fatal("New telemetry returned nil")
	}
	r := NewReporter(telem, ReporterConfig{Endpoint: "", BatchSize: 10})
	if r == nil {
		t.Fatal("NewReporter returned nil")
	}

	// flush when empty - should not panic
	r.Enqueue(Event{Type: EventCommandExecuted})
	if r.QueueSize() != 1 {
		t.Errorf("QueueSize = %d, want 1 after enqueue", r.QueueSize())
	}
	r.Stop() // triggers flush
}

func TestBoolToInt(t *testing.T) {
	t.Parallel()

	if boolToInt(true) != 1 {
		t.Error("boolToInt(true) should be 1")
	}
	if boolToInt(false) != 0 {
		t.Error("boolToInt(false) should be 0")
	}
}

func TestEscapeJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
	}{
		{"simple"},
		{"with \"quotes\""},
		{"with\nnewline"},
		{"with\ttab"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			result := escapeJSON(tc.input)
			if result == "" {
				t.Error("result should not be empty")
			}
		})
	}
}

func TestMapToJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		m    map[string]string
	}{
		{"empty", map[string]string{}},
		{"single", map[string]string{"key": "value"}},
		{"multiple", map[string]string{"a": "1", "b": "2"}},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := mapToJSON(tc.m)
			if result == "" {
				t.Error("result should not be empty")
			}
		})
	}
}

func TestEventTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		eventType EventType
		want      string
	}{
		{EventCommandExecuted, "command_executed"},
		{EventIndexCompleted, "index_completed"},
		{EventSearchPerformed, "search_performed"},
		{EventContextBuilt, "context_built"},
		{EventMemoryStored, "memory_stored"},
		{EventPluginInstalled, "plugin_installed"},
		{EventErrorOccurred, "error_occurred"},
		{EventRuntimeStarted, "runtime_started"},
		{EventRuntimeStopped, "runtime_stopped"},
		{EventProviderCalled, "provider_called"},
		{EventConfigChanged, "config_changed"},
		{EventUpdateChecked, "update_checked"},
		{EventSyncCompleted, "sync_completed"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			if string(tc.eventType) != tc.want {
				t.Errorf("EventType = %q, want %q", tc.eventType, tc.want)
			}
		})
	}
}
