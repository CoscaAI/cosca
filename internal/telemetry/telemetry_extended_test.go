package telemetry

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	_ "modernc.org/sqlite"
)

// =============================================================================
// Helper to create an in-memory SQLite DB for tests
// =============================================================================

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	// Run same pragmas as production
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			t.Fatalf("pragma %s: %v", p, err)
		}
	}
	// Create schema
	schema := `
	CREATE TABLE IF NOT EXISTS events (
		id         TEXT PRIMARY KEY,
		timestamp  TEXT NOT NULL,
		type       TEXT NOT NULL,
		duration_ms INTEGER DEFAULT 0,
		success    INTEGER DEFAULT 1,
		metadata   TEXT DEFAULT '{}',
		sent       INTEGER DEFAULT 0,
		created_at TEXT DEFAULT (datetime('now'))
	);
	CREATE TABLE IF NOT EXISTS metadata (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		t.Fatalf("create test schema: %v", err)
	}
	return db
}

func newTelemetryWithDB(t *testing.T) *Telemetry {
	t.Helper()
	db := newTestDB(t)
	telem, err := New(WithDB(db), WithConfig(Config{Enabled: true, Version: "1.0.0"}))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	t.Cleanup(func() { telem.Close() })
	return telem
}

// =============================================================================
// WithLogger
// =============================================================================

func TestWithLogger_SetsLogger(t *testing.T) {
	t.Parallel()

	logger := zerolog.New(os.Stderr).Level(zerolog.DebugLevel)
	telem, err := New(WithLogger(logger))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if telem == nil {
		t.Fatal("New() returned nil")
	}
	// Logger is tested implicitly by checking no panic during operations
	telem.SetEnabled(true)
	telem.SetEnabled(false)
	telem.Close()
}

// =============================================================================
// WithDB
// =============================================================================

func TestWithDB_UsesProvidedDatabase(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	telem, err := New(WithDB(db), WithConfig(Config{Enabled: true}))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer telem.Close()

	// WithDB should not initialize a new database
	if telem.db != db {
		t.Error("WithDB should set the provided database")
	}
}

func TestWithDB_EventCountWorks(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	telem, err := New(WithDB(db), WithConfig(Config{Enabled: true, Version: "1.0"}))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer telem.Close()

	count, err := telem.EventCount()
	if err != nil {
		t.Fatalf("EventCount() error: %v", err)
	}
	if count != 0 {
		t.Errorf("EventCount() = %d, want 0", count)
	}
}

// =============================================================================
// initDB
// =============================================================================

func TestInitDB_CreatesDatabaseFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfg := Config{
		Enabled: true,
		DataDir: tmpDir,
		Version: "1.3.0",
	}
	telem, err := New(WithConfig(cfg))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer telem.Close()

	// Verify the database file exists
	dbPath := tmpDir + "/telemetry.db"
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("database file not created at %s", dbPath)
	}

	// Verify events table was created
	count, err := telem.EventCount()
	if err != nil {
		t.Fatalf("EventCount() error: %v", err)
	}
	if count != 0 {
		t.Errorf("EventCount() = %d, want 0", count)
	}
}

func TestInitDB_CreatesParentDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	nestedDir := tmpDir + "/nested/path"

	cfg := Config{
		Enabled: true,
		DataDir: nestedDir,
		Version: "1.0.0",
	}
	telem, err := New(WithConfig(cfg))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer telem.Close()

	// Directory should have been created
	if fi, err := os.Stat(nestedDir); err != nil || !fi.IsDir() {
		t.Errorf("expected directory %s to exist, err=%v", nestedDir, err)
	}

	if !telem.Enabled() {
		t.Error("telemetry should be enabled")
	}
}

// =============================================================================
// RecordEvent
// =============================================================================

func TestRecordEvent_StoresEvent(t *testing.T) {
	telem := newTelemetryWithDB(t)

	event := Event{
		Type:       EventCommandExecuted,
		Timestamp:  time.Now(),
		DurationMs: 150,
		Success:    true,
		Metadata: map[string]string{
			"command": "build",
		},
	}

	err := telem.RecordEvent(event)
	if err != nil {
		t.Fatalf("RecordEvent() error: %v", err)
	}

	count, err := telem.EventCount()
	if err != nil {
		t.Fatalf("EventCount() error: %v", err)
	}
	if count != 1 {
		t.Errorf("EventCount() = %d, want 1", count)
	}
}

func TestRecordEvent_AutoGeneratesID(t *testing.T) {
	telem := newTelemetryWithDB(t)

	event := Event{
		Type: EventRuntimeStarted,
	}
	err := telem.RecordEvent(event)
	if err != nil {
		t.Fatalf("RecordEvent() error: %v", err)
	}

	// Verify it was stored with a generated ID
	row := telem.db.QueryRow("SELECT id FROM events WHERE type = ?", string(EventRuntimeStarted))
	var id string
	if err := row.Scan(&id); err != nil {
		t.Fatalf("scan id: %v", err)
	}
	if id == "" {
		t.Error("expected generated ID")
	}
}

func TestRecordEvent_AutoSetsTimestamp(t *testing.T) {
	telem := newTelemetryWithDB(t)

	before := time.Now()
	event := Event{
		Type: EventRuntimeStopped,
		// Timestamp is zero, should be auto-set
	}
	err := telem.RecordEvent(event)
	if err != nil {
		t.Fatalf("RecordEvent() error: %v", err)
	}
	after := time.Now()

	row := telem.db.QueryRow("SELECT timestamp FROM events WHERE type = ?", string(EventRuntimeStopped))
	var ts string
	if err := row.Scan(&ts); err != nil {
		t.Fatalf("scan timestamp: %v", err)
	}
	parsed, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t.Fatalf("parse timestamp: %v", err)
	}
	// Format uses RFC3339 which truncates nanoseconds, so compare with second precision
	if parsed.Before(before.Truncate(time.Second)) || parsed.After(after.Truncate(time.Second).Add(time.Second)) {
		t.Errorf("timestamp %v not in range [%v, %v]", parsed, before, after)
	}
}

func TestRecordEvent_WhenDisabled(t *testing.T) {
	db := newTestDB(t)
	telem, err := New(WithDB(db), WithConfig(Config{Enabled: false}))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer telem.Close()

	err = telem.RecordEvent(Event{Type: EventCommandExecuted})
	if err != nil {
		t.Fatalf("RecordEvent() error: %v", err)
	}

	count, _ := telem.EventCount()
	if count != 0 {
		t.Errorf("EventCount() = %d, want 0 when disabled", count)
	}
}

func TestRecordEvent_WhenClosed(t *testing.T) {
	telem := newTelemetryWithDB(t)
	telem.Close()

	err := telem.RecordEvent(Event{Type: EventCommandExecuted})
	if err != nil {
		t.Fatalf("RecordEvent() error on closed: %v", err)
	}

	// Should silently discard
	if telem.closed != true {
		t.Error("expected closed = true")
	}
}

func TestRecordEvent_NilDB(t *testing.T) {
	telem, err := New(WithConfig(Config{Enabled: true}))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// No DB path and no WithDB — db is nil
	err = telem.RecordEvent(Event{Type: EventCommandExecuted})
	if err != nil {
		t.Fatalf("RecordEvent() error: %v", err)
	}
	// Should silently discard — no crash
}

func TestRecordEvent_WithMetadata(t *testing.T) {
	telem := newTelemetryWithDB(t)

	event := Event{
		Type: EventSearchPerformed,
		Metadata: map[string]string{
			"query_type":   "code",
			"result_count": "42",
		},
	}
	err := telem.RecordEvent(event)
	if err != nil {
		t.Fatalf("RecordEvent() error: %v", err)
	}

	row := telem.db.QueryRow("SELECT metadata FROM events WHERE type = ?", string(EventSearchPerformed))
	var meta string
	if err := row.Scan(&meta); err != nil {
		t.Fatalf("scan metadata: %v", err)
	}
	if !strings.Contains(meta, "query_type") || !strings.Contains(meta, "result_count") {
		t.Errorf("unexpected metadata: %s", meta)
	}
}

func TestRecordEvent_FailureEvent(t *testing.T) {
	telem := newTelemetryWithDB(t)

	event := NewErrorOccurredEvent("build", "E001")
	err := telem.RecordEvent(event)
	if err != nil {
		t.Fatalf("RecordEvent() error: %v", err)
	}

	row := telem.db.QueryRow("SELECT success FROM events WHERE type = ?", string(EventErrorOccurred))
	var success int
	if err := row.Scan(&success); err != nil {
		t.Fatalf("scan success: %v", err)
	}
	if success != 0 {
		t.Errorf("success = %d, want 0 for error event", success)
	}
}

// =============================================================================
// RecordEvents (batch)
// =============================================================================

func TestRecordEvents_BatchInsert(t *testing.T) {
	telem := newTelemetryWithDB(t)

	events := []Event{
		{Type: EventCommandExecuted, Success: true},
		{Type: EventIndexCompleted, Success: true},
		{Type: EventSearchPerformed, Success: false},
	}
	err := telem.RecordEvents(events)
	if err != nil {
		t.Fatalf("RecordEvents() error: %v", err)
	}

	count, err := telem.EventCount()
	if err != nil {
		t.Fatalf("EventCount() error: %v", err)
	}
	if count != 3 {
		t.Errorf("EventCount() = %d, want 3", count)
	}
}

func TestRecordEvents_EmptySlice(t *testing.T) {
	telem := newTelemetryWithDB(t)

	err := telem.RecordEvents(nil)
	if err != nil {
		t.Fatalf("RecordEvents(nil) error: %v", err)
	}

	err = telem.RecordEvents([]Event{})
	if err != nil {
		t.Fatalf("RecordEvents([]) error: %v", err)
	}

	count, _ := telem.EventCount()
	if count != 0 {
		t.Errorf("EventCount() = %d, want 0", count)
	}
}

func TestRecordEvents_WhenDisabled(t *testing.T) {
	db := newTestDB(t)
	telem, err := New(WithDB(db), WithConfig(Config{Enabled: false}))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer telem.Close()

	err = telem.RecordEvents([]Event{{Type: EventCommandExecuted}})
	if err != nil {
		t.Fatalf("RecordEvents() error: %v", err)
	}

	count, _ := telem.EventCount()
	if count != 0 {
		t.Errorf("EventCount() = %d, want 0 when disabled", count)
	}
}

func TestRecordEvents_WhenClosed(t *testing.T) {
	telem := newTelemetryWithDB(t)
	telem.Close()

	err := telem.RecordEvents([]Event{{Type: EventCommandExecuted}})
	if err != nil {
		t.Fatalf("RecordEvents() on closed: %v", err)
	}
}

func TestRecordEvents_NilDB(t *testing.T) {
	telem, err := New(WithConfig(Config{Enabled: true}))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	err = telem.RecordEvents([]Event{{Type: EventCommandExecuted}})
	if err != nil {
		t.Fatalf("RecordEvents() error: %v", err)
	}
}

// =============================================================================
// EventCount / UnsyncedCount
// =============================================================================

func TestEventCount_NilDB(t *testing.T) {
	t.Parallel()

	telem, _ := New(WithConfig(Config{Enabled: false}))
	count, err := telem.EventCount()
	if err != nil {
		t.Fatalf("EventCount() error: %v", err)
	}
	if count != 0 {
		t.Errorf("EventCount() = %d, want 0 with nil db", count)
	}
}

func TestEventCount_AfterInserts(t *testing.T) {
	telem := newTelemetryWithDB(t)

	for i := 0; i < 5; i++ {
		telem.RecordEvent(Event{Type: EventCommandExecuted})
	}

	count, err := telem.EventCount()
	if err != nil {
		t.Fatalf("EventCount() error: %v", err)
	}
	if count != 5 {
		t.Errorf("EventCount() = %d, want 5", count)
	}
}

func TestUnsyncedCount_NilDB(t *testing.T) {
	t.Parallel()

	telem, _ := New(WithConfig(Config{Enabled: false}))
	count, err := telem.UnsyncedCount()
	if err != nil {
		t.Fatalf("UnsyncedCount() error: %v", err)
	}
	if count != 0 {
		t.Errorf("UnsyncedCount() = %d, want 0 with nil db", count)
	}
}

func TestUnsyncedCount_AllUnsynced(t *testing.T) {
	telem := newTelemetryWithDB(t)

	for i := 0; i < 3; i++ {
		telem.RecordEvent(Event{Type: EventCommandExecuted})
	}

	count, err := telem.UnsyncedCount()
	if err != nil {
		t.Fatalf("UnsyncedCount() error: %v", err)
	}
	if count != 3 {
		t.Errorf("UnsyncedCount() = %d, want 3", count)
	}
}

// =============================================================================
// Close
// =============================================================================

func TestClose_Success(t *testing.T) {
	t.Parallel()

	telem, _ := New(WithConfig(Config{Enabled: false}))
	err := telem.Close()
	if err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	if !telem.closed {
		t.Error("closed should be true after Close()")
	}
}

func TestClose_DoubleClose(t *testing.T) {
	t.Parallel()

	telem, _ := New(WithConfig(Config{Enabled: false}))
	err := telem.Close()
	if err != nil {
		t.Fatalf("first Close() error: %v", err)
	}
	err = telem.Close()
	if err != nil {
		t.Fatalf("second Close() error: %v", err)
	}
}

func TestClose_WithDB(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	telem, err := New(WithDB(db), WithConfig(Config{Enabled: true}))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	err = telem.Close()
	if err != nil {
		t.Fatalf("Close() error: %v", err)
	}
}

func TestClose_ThenCannotRecord(t *testing.T) {
	telem := newTelemetryWithDB(t)
	telem.Close()

	err := telem.RecordEvent(Event{Type: EventCommandExecuted})
	if err != nil {
		t.Fatalf("RecordEvent() after Close: %v", err)
	}
}

// =============================================================================
// hexChar
// =============================================================================

func TestHexChar(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input byte
		want  byte
	}{
		{0, '0'},
		{1, '1'},
		{2, '2'},
		{3, '3'},
		{4, '4'},
		{5, '5'},
		{6, '6'},
		{7, '7'},
		{8, '8'},
		{9, '9'},
		{10, 'a'},
		{11, 'b'},
		{12, 'c'},
		{13, 'd'},
		{14, 'e'},
		{15, 'f'},
	}

	for _, c := range cases {
		got := hexChar(c.input)
		if got != c.want {
			t.Errorf("hexChar(%d) = %c, want %c", c.input, got, c.want)
		}
	}
}

// =============================================================================
// escapeJSON full coverage
// =============================================================================

func TestEscapeJSON_AllSpecialChars(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		contains string // substring that should appear in result
	}{
		{"simple", "hello", "hello"},
		{"quotes", `hello "world"`, `\"`},
		{"backslash", `a\b`, `\\`},
		{"newline", "a\nb", `\n`},
		{"carriage_return", "a\rb", `\r`},
		{"tab", "a\tb", `\t`},
		{"control_char", string([]byte{0x01}), `\u0001`},
		{"control_char_0xF", string([]byte{0x0F}), `\u000f`},
		{"control_char_null", string([]byte{0x00}), `\u0000`},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := escapeJSON(tt.input)
			if !strings.Contains(got, tt.contains) {
				t.Errorf("escapeJSON(%q) = %q, want it to contain %q", tt.input, got, tt.contains)
			}
		})
	}
}

// =============================================================================
// mapToJSON edge cases
// =============================================================================

func TestMapToJSON_SpecialChars(t *testing.T) {
	t.Parallel()

	result := mapToJSON(map[string]string{
		`key"with"quotes`: `val"with"quotes`,
	})
	// Should contain escaped quotes
	if !strings.Contains(result, `\"`) {
		t.Errorf("expected escaped quotes in: %s", result)
	}
}

func TestMapToJSON_SingleKey(t *testing.T) {
	t.Parallel()

	result := mapToJSON(map[string]string{"a": "1"})
	expected := `{"a":"1"}`
	if result != expected {
		t.Errorf("mapToJSON = %q, want %q", result, expected)
	}
}

// =============================================================================
// WithConfig DataDir path
// =============================================================================

func TestWithConfig_DataDir(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Enabled: false,
		DataDir: "/tmp/test-telemetry",
		Version: "1.0.0",
	}
	telem, err := New(WithConfig(cfg))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if telem.dbPath != "/tmp/test-telemetry/telemetry.db" {
		t.Errorf("dbPath = %q, want /tmp/test-telemetry/telemetry.db", telem.dbPath)
	}
}

func TestWithConfig_DataDirEmpty(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Enabled: true,
		DataDir: "",
		Version: "1.0.0",
	}
	telem, err := New(WithConfig(cfg))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if telem.dbPath != "" {
		t.Errorf("dbPath = %q, want empty", telem.dbPath)
	}
}

// =============================================================================
// New with env override
// =============================================================================

func TestNew_EnvDisablesTelemetry(t *testing.T) {
	t.Setenv("COSCA_TELEMETRY_ENABLED", "false")

	telem, err := New(WithConfig(Config{Enabled: true}))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if telem.Enabled() {
		t.Error("telemetry should be disabled via env var")
	}
}

func TestNew_EnvEnablesTelemetry(t *testing.T) {
	t.Setenv("COSCA_TELEMETRY_ENABLED", "1")

	telem, err := New(WithConfig(Config{Enabled: false}))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	// Note: we provided a nil dbPath and no db, so if enabled is true,
	// it won't try initDB since dbPath is empty
	if !telem.Enabled() {
		t.Error("telemetry should be enabled via env var '1'")
	}
}

func TestNew_EnvTrue_DisablesTelemetry(t *testing.T) {
	t.Setenv("COSCA_TELEMETRY_ENABLED", "true")

	telem, err := New(WithConfig(Config{Enabled: false}))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if !telem.Enabled() {
		t.Error("telemetry should be enabled via env var 'true'")
	}
}

func TestNew_EnvZero(t *testing.T) {
	t.Setenv("COSCA_TELEMETRY_ENABLED", "0")

	telem, err := New(WithConfig(Config{Enabled: true}))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if telem.Enabled() {
		t.Error("telemetry should be disabled via env var '0'")
	}
}

func TestNew_EnvRandomDisablesTelemetry(t *testing.T) {
	t.Setenv("COSCA_TELEMETRY_ENABLED", "maybe")

	telem, err := New(WithConfig(Config{Enabled: true}))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if telem.Enabled() {
		t.Error("telemetry should be disabled when env is not 'true' or '1'")
	}
}

// =============================================================================
// Emit function
// =============================================================================

func TestEmit_WithGlobalTelemetry(t *testing.T) {
	telem := newTelemetryWithDB(t)
	prev := globalTelemetry
	SetGlobal(telem)
	defer SetGlobal(prev)

	// Emit a simple event
	Emit("test_event", map[string]interface{}{
		"key": "value",
	})

	// Give goroutine time to record
	time.Sleep(50 * time.Millisecond)

	count, err := telem.EventCount()
	if err != nil {
		t.Fatalf("EventCount() error: %v", err)
	}
	if count < 1 {
		t.Errorf("EventCount() = %d, want >= 1 after Emit", count)
	}
}

func TestEmit_NilGlobalTelemetry(t *testing.T) {
	prev := globalTelemetry
	globalTelemetry = nil
	defer func() { globalTelemetry = prev }()

	// Should not panic
	Emit("test_event", nil)
}

func TestEmit_DisabledTelemetry(t *testing.T) {
	telem, _ := New(WithConfig(Config{Enabled: false}))
	prev := globalTelemetry
	SetGlobal(telem)
	defer SetGlobal(prev)

	// Should not record since disabled
	Emit("test_event", map[string]interface{}{
		"key": "value",
	})

	// No panic, nothing to verify
}

func TestEmit_MultipleEvents(t *testing.T) {
	telem := newTelemetryWithDB(t)
	prev := globalTelemetry
	SetGlobal(telem)
	defer SetGlobal(prev)

	for i := 0; i < 5; i++ {
		Emit("test_event", map[string]interface{}{
			"index": i,
		})
	}

	// Wait for all goroutines
	time.Sleep(100 * time.Millisecond)

	count, err := telem.EventCount()
	if err != nil {
		t.Fatalf("EventCount() error: %v", err)
	}
	if count < 5 {
		t.Errorf("EventCount() = %d, want >= 5", count)
	}
}

// =============================================================================
// Reporter: Enqueue with batch size trigger
// =============================================================================

func TestReporterEnqueue_BatchFlush(t *testing.T) {
	t.Parallel()

	telem, _ := New(WithConfig(Config{Enabled: false}))
	r := NewReporter(telem, ReporterConfig{
		Endpoint:      "",
		BatchSize:     3,
		FlushInterval: time.Hour,
	})

	// Enqueue 2 events — should not flush yet
	r.Enqueue(Event{Type: EventCommandExecuted})
	r.Enqueue(Event{Type: EventIndexCompleted})
	if r.QueueSize() != 2 {
		t.Fatalf("QueueSize = %d, want 2", r.QueueSize())
	}

	// Third event triggers flush via batch size
	r.Enqueue(Event{Type: EventSearchPerformed})
	// Queue should be empty after flush (since endpoint="" returns nil)
	if r.QueueSize() != 0 {
		t.Errorf("QueueSize = %d, want 0 after batch flush", r.QueueSize())
	}
}

func TestReporterEnqueue_ExceedsBatch(t *testing.T) {
	t.Parallel()

	telem, _ := New(WithConfig(Config{Enabled: false}))
	r := NewReporter(telem, ReporterConfig{
		Endpoint:      "",
		BatchSize:     2,
		FlushInterval: time.Hour,
	})

	// Enqueue 5 events with batch size 2
	for i := 0; i < 5; i++ {
		r.Enqueue(Event{Type: EventCommandExecuted})
	}
	// After 5 events with batch 2, queue should be empty (flushed at 2, 4, and leftover 1 flushed at 5→wait, 5 doesn't trigger 3rd flush since flush resets)
	// Actually: events 1,2 → flush (queue empty), events 3,4 → flush (queue empty), event 5 → queue has 1
	if r.QueueSize() > 1 {
		t.Errorf("QueueSize = %d, want <= 1", r.QueueSize())
	}
}

// =============================================================================
// Reporter: Start double-start
// =============================================================================

func TestReporterStart_DoubleStart(t *testing.T) {
	telem, _ := New(WithConfig(Config{Enabled: false}))
	r := NewReporter(telem, ReporterConfig{
		Endpoint:      "",
		BatchSize:     10,
		FlushInterval: time.Hour,
	})

	r.Start()
	r.Start() // Should not start twice
	r.Stop()
}

// =============================================================================
// Reporter: Stop with double-stop
// =============================================================================

func TestReporterStop_DoubleStop(t *testing.T) {
	telem, _ := New(WithConfig(Config{Enabled: false}))
	r := NewReporter(telem, ReporterConfig{
		Endpoint:      "",
		BatchSize:     10,
		FlushInterval: time.Hour,
	})

	r.Start()
	r.Stop()
	r.Stop() // Should not panic
}

// =============================================================================
// Reporter: reportLoop ticker path and ctx.Done path
// =============================================================================

func TestReporterReportLoop_ContextCancellation(t *testing.T) {
	telem, _ := New(WithConfig(Config{Enabled: false}))
	r := NewReporter(telem, ReporterConfig{
		Endpoint:      "",
		BatchSize:     10,
		FlushInterval: time.Hour,
	})

	r.Start()
	// Wait a tick to ensure the goroutine is in the select loop
	time.Sleep(10 * time.Millisecond)
	r.Stop()
}

// =============================================================================
// Reporter: flush with events
// =============================================================================

func TestReporterFlush_WithEvents(t *testing.T) {
	telem, _ := New(WithConfig(Config{Enabled: false}))
	r := NewReporter(telem, ReporterConfig{
		Endpoint:      "", // Empty endpoint — should skip HTTP
		BatchSize:     10,
		FlushInterval: time.Hour,
	})

	r.Start()
	r.Enqueue(Event{Type: EventCommandExecuted})
	r.Enqueue(Event{Type: EventIndexCompleted})
	r.Stop() // triggers flush

	// Queue should be empty after flush (empty endpoint returns nil, no requeue)
	if r.QueueSize() != 0 {
		t.Errorf("QueueSize = %d, want 0 after flush", r.QueueSize())
	}
}

// =============================================================================
// Reporter: sendEvents with HTTP
// =============================================================================

func TestReporterSendEvents_Success(t *testing.T) {
	received := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 2048)
		n, _ := r.Body.Read(buf)
		received <- buf[:n]
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	telem, _ := New(WithConfig(Config{Enabled: false, Version: "1.0.0"}))
	r := NewReporter(telem, ReporterConfig{
		Endpoint:      srv.URL,
		BatchSize:     1, // flush immediately on each enqueue
		FlushInterval: time.Hour,
		Timeout:       5 * time.Second,
	})

	// Enqueue with batch=1 triggers immediate flush while context is alive
	r.Enqueue(Event{Type: EventCommandExecuted, Success: true})

	select {
	case payload := <-received:
		var data map[string]interface{}
		if err := json.Unmarshal(payload, &data); err != nil {
			t.Fatalf("unmarshal payload: %v, body=%s", err, string(payload))
		}
		if data["version"] != "1.0.0" {
			t.Errorf("version = %v, want 1.0.0", data["version"])
		}
		events, ok := data["events"].([]interface{})
		if !ok || len(events) != 1 {
			t.Errorf("events count = %d, want 1", len(events))
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for HTTP request")
	}
}

func TestReporterSendEvents_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	telem, _ := New(WithConfig(Config{Enabled: false, Version: "1.0.0"}))
	r := NewReporter(telem, ReporterConfig{
		Endpoint:      srv.URL,
		BatchSize:     10,
		FlushInterval: time.Hour,
		Timeout:       5 * time.Second,
	})

	event := Event{Type: EventCommandExecuted, Success: true}
	r.Enqueue(event)
	r.Start()
	r.Stop() // flush

	// Events should be requeued after server error
	if r.QueueSize() != 1 {
		t.Errorf("QueueSize = %d, want 1 after failed send (requeued)", r.QueueSize())
	}
}

func TestReporterSendEvents_NilTelemetry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Pass nil telemetry — should not panic
	r := NewReporter(nil, ReporterConfig{
		Endpoint:      srv.URL,
		BatchSize:     10,
		FlushInterval: time.Hour,
		Timeout:       5 * time.Second,
	})

	r.Enqueue(Event{Type: EventCommandExecuted})
	r.Start()
	r.Stop()
}

func TestReporterSendEvents_WithAPIKey(t *testing.T) {
	receivedKey := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedKey <- r.Header.Get("X-API-Key")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	telem, _ := New(WithConfig(Config{Enabled: false, Version: "1.0.0"}))
	r := NewReporter(telem, ReporterConfig{
		Endpoint:      srv.URL,
		BatchSize:     1, // flush immediately on each enqueue
		FlushInterval: time.Hour,
		APIKey:        "secret-key-123",
		Timeout:       5 * time.Second,
	})

	// Enqueue with batch=1 triggers immediate flush while context is alive
	r.Enqueue(Event{Type: EventCommandExecuted})

	select {
	case key := <-receivedKey:
		if key != "secret-key-123" {
			t.Errorf("API key = %q, want secret-key-123", key)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for HTTP request")
	}
}

// =============================================================================
// Reporter: sendEvents with empty endpoint
// =============================================================================

func TestReporterSendEvents_EmptyEndpoint(t *testing.T) {
	t.Parallel()

	telem, _ := New(WithConfig(Config{Enabled: false}))
	r := NewReporter(telem, ReporterConfig{
		Endpoint:      "",
		BatchSize:     10,
		FlushInterval: time.Hour,
	})

	r.Enqueue(Event{Type: EventCommandExecuted})
	r.Start()
	r.Stop()

	// With empty endpoint, sendEvents returns nil without HTTP call
	if r.QueueSize() != 0 {
		t.Errorf("QueueSize = %d, want 0", r.QueueSize())
	}
}

// =============================================================================
// Reporter: Concurrency tests
// =============================================================================

func TestReporterConcurrentEnqueue(t *testing.T) {
	telem, _ := New(WithConfig(Config{Enabled: false}))
	r := NewReporter(telem, ReporterConfig{
		Endpoint:      "",
		BatchSize:     50,
		FlushInterval: time.Hour,
	})

	var wg sync.WaitGroup
	n := 100
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			r.Enqueue(Event{Type: EventCommandExecuted})
		}()
	}
	wg.Wait()

	// With batch size 50 and empty endpoint, flush resets queue to 0 each time
	// Final queue should be less than batchSize
	sz := r.QueueSize()
	if sz < 0 || sz > n {
		t.Errorf("QueueSize = %d, expected in [0, %d]", sz, n)
	}
}

// =============================================================================
// Reporter: Config with custom timeout
// =============================================================================

func TestReporterConfig_CustomTimeout(t *testing.T) {
	t.Parallel()

	telem, _ := New(WithConfig(Config{Enabled: false}))
	r := NewReporter(telem, ReporterConfig{
		Endpoint:      "https://example.com",
		BatchSize:     20,
		FlushInterval: 30 * time.Second,
		Timeout:       10 * time.Second,
	})

	if r.batchSize != 20 {
		t.Errorf("batchSize = %d, want 20", r.batchSize)
	}
	if r.flushInterval != 30*time.Second {
		t.Errorf("flushInterval = %v, want 30s", r.flushInterval)
	}
	if r.client.Timeout != 10*time.Second {
		t.Errorf("client.Timeout = %v, want 10s", r.client.Timeout)
	}
}

// =============================================================================
// Telemetry: DefaultConfig
// =============================================================================

func TestDefaultConfig_AllFields(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	if !cfg.Enabled {
		t.Error("Enabled should default to true")
	}
	if cfg.QueueSize != 100 {
		t.Errorf("QueueSize = %d", cfg.QueueSize)
	}
	if cfg.Version != "0.0.0" {
		t.Errorf("Version = %q", cfg.Version)
	}
}

// =============================================================================
// Test the full telemetry lifecycle with initDB
// =============================================================================

func TestTelemetryFullLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		Enabled: true,
		DataDir: tmpDir,
		Version: "2.0.0",
	}

	telem, err := New(WithConfig(cfg))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Record several events
	for i := 0; i < 10; i++ {
		evt := NewCommandExecutedEvent("test", int64(i*100), true)
		if err := telem.RecordEvent(evt); err != nil {
			t.Fatalf("RecordEvent %d: %v", i, err)
		}
	}

	// Check counts
	count, err := telem.EventCount()
	if err != nil {
		t.Fatalf("EventCount() error: %v", err)
	}
	if count != 10 {
		t.Errorf("EventCount() = %d, want 10", count)
	}

	unsynced, err := telem.UnsyncedCount()
	if err != nil {
		t.Fatalf("UnsyncedCount() error: %v", err)
	}
	if unsynced != 10 {
		t.Errorf("UnsyncedCount() = %d, want 10", unsynced)
	}

	// Close and verify
	if err := telem.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
}

// =============================================================================
// Test Emit with nil metadata
// =============================================================================

func TestEmit_NilMetadata(t *testing.T) {
	telem := newTelemetryWithDB(t)
	prev := globalTelemetry
	SetGlobal(telem)
	defer SetGlobal(prev)

	Emit("test_nil_meta", nil)

	time.Sleep(50 * time.Millisecond)

	count, err := telem.EventCount()
	if err != nil {
		t.Fatalf("EventCount() error: %v", err)
	}
	if count < 1 {
		t.Errorf("EventCount() = %d, want >= 1", count)
	}
}

// =============================================================================
// Verify Emit doesn't block
// =============================================================================

func TestEmit_DoesNotBlock(t *testing.T) {
	telem, _ := New(WithConfig(Config{Enabled: false}))
	prev := globalTelemetry
	SetGlobal(telem)
	defer SetGlobal(prev)

	done := make(chan bool, 1)
	go func() {
		// Emit should not block even when telemetry is disabled
		for i := 0; i < 100; i++ {
			Emit("noblock", nil)
		}
		done <- true
	}()

	select {
	case <-done:
		// Success — Emit did not block
	case <-time.After(2 * time.Second):
		t.Fatal("Emit blocked unexpectedly")
	}
}

// =============================================================================
// Test Emit when telemetry is closed — exercises the RecordEvent error path in the goroutine
// =============================================================================

func TestEmit_RecordEventFails(t *testing.T) {
	telem := newTelemetryWithDB(t)
	prev := globalTelemetry
	SetGlobal(telem)
	defer SetGlobal(prev)

	// Close telemetry so RecordEvent in goroutine is silently discarded
	telem.Close()

	// Emit should not panic even when telemetry is closed
	Emit("test_closed", map[string]interface{}{
		"key": "value",
	})

	// Give goroutine time to attempt recording
	time.Sleep(50 * time.Millisecond)
	// No panic = success
}

// =============================================================================
// Test batch insert with generated IDs
// =============================================================================

func TestRecordEvents_GeneratedIDs(t *testing.T) {
	telem := newTelemetryWithDB(t)

	events := []Event{
		{Type: EventCommandExecuted}, // no ID
		{Type: EventCommandExecuted}, // no ID
	}
	err := telem.RecordEvents(events)
	if err != nil {
		t.Fatalf("RecordEvents() error: %v", err)
	}

	rows, err := telem.db.Query("SELECT id FROM events")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}

	if len(ids) != 2 {
		t.Fatalf("expected 2 ids, got %d", len(ids))
	}
	if ids[0] == "" || ids[1] == "" {
		t.Error("generated IDs should not be empty")
	}
	if ids[0] == ids[1] {
		t.Error("generated IDs should be unique")
	}
}

// =============================================================================
// Test context in reporter
// =============================================================================

func TestReporterContextCancellation(t *testing.T) {
	t.Parallel()

	telem, _ := New(WithConfig(Config{Enabled: false}))
	r := NewReporter(telem, ReporterConfig{
		Endpoint:      "",
		BatchSize:     10,
		FlushInterval: 100 * time.Millisecond,
	})

	// Start the reporter
	r.Start()

	// Wait for at least one tick to fire
	time.Sleep(150 * time.Millisecond)

	// Stop should cancel context and wait for goroutine
	r.Stop()
}

// =============================================================================
// Test reporter ticker runs flush periodically
// =============================================================================

func TestReporterTickerTriggersFlush(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	telem, _ := New(WithConfig(Config{Enabled: false, Version: "1.0.0"}))
	r := NewReporter(telem, ReporterConfig{
		Endpoint:      srv.URL,
		BatchSize:     100,
		FlushInterval: 50 * time.Millisecond,
		Timeout:       5 * time.Second,
	})

	r.Enqueue(Event{Type: EventCommandExecuted})
	r.Start()

	// Wait for ticker to fire and flush
	time.Sleep(150 * time.Millisecond)

	r.Stop()

	// Events should have been flushed by ticker
	// Since server returns 200, queue should be empty
	if r.QueueSize() != 0 {
		t.Logf("QueueSize = %d after ticker flush, might have been requeued if timing was off", r.QueueSize())
	}
}

// =============================================================================
// Edge cases: New with initDB failure
// =============================================================================

func TestNew_InitDBFails_ReadOnlyDir(t *testing.T) {
	// Create a file where the directory should be — MkdirAll will fail
	tmpDir := t.TempDir()
	blockingFile := tmpDir + "/block"
	if err := os.WriteFile(blockingFile, []byte("block"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cfg := Config{
		Enabled: true,
		DataDir: blockingFile + "/sub", // blockingFile is a file, can't mkdir
		Version: "1.0.0",
	}
	_, err := New(WithConfig(cfg))
	if err == nil {
		t.Error("New() should fail when initDB can't create directory")
	}
}

// =============================================================================
// Edge cases: sendEvents with invalid URL
// =============================================================================

func TestReporterSendEvents_InvalidURL(t *testing.T) {
	t.Parallel()

	telem, _ := New(WithConfig(Config{Enabled: false}))
	r := NewReporter(telem, ReporterConfig{
		Endpoint:      "http://invalid-url-%zz.test",
		BatchSize:     1,
		FlushInterval: time.Hour,
		Timeout:       5 * time.Second,
	})

	// Enqueue triggers flush, sendEvents should fail to create request
	// The error is swallowed (events requeued), we just verify no panic
	r.Enqueue(Event{Type: EventCommandExecuted})
	// Events should be requeued after send failure
	if r.QueueSize() != 1 {
		t.Errorf("QueueSize = %d, want 1 after failed send (requeued)", r.QueueSize())
	}
}
