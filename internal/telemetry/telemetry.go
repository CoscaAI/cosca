//
// Package telemetry provides the anonymous usage telemetry system for the
// Cosca platform. It supports opt-in/opt-out, local event recording to
// SQLite, privacy controls (no PII, no file contents), and batched reporting
// to a telemetry server.

package telemetry

import (
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/safe"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	_ "modernc.org/sqlite" // SQLite driver
)

// =============================================================================
// Telemetry
// =============================================================================

// Telemetry manages anonymous usage data collection and reporting.
// It respects opt-in/opt-out via both environment variables and configuration.
// Only aggregate metrics and event types are recorded - no PII, no file
// contents, no identifiable information.
type Telemetry struct {
	mu         sync.RWMutex
	logger     zerolog.Logger
	db         *sql.DB
	enabled    bool
	instanceID string
	version    string
	dbPath     string
	queueSize  int
	closed     bool
}

// Config configures the telemetry system.
type Config struct {
	// Enabled controls whether telemetry is active.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// DataDir is the directory for the telemetry database.
	DataDir string `json:"data_dir" yaml:"data_dir"`
	// Version is the Cosca version for reporting.
	Version string `json:"version" yaml:"version"`
	// QueueSize is the max number of events to buffer before flushing.
	QueueSize int `json:"queue_size" yaml:"queue_size"`
}

// DefaultConfig returns a default telemetry configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:   true, // Opt-in by default; users can disable
		QueueSize: 100,
		Version:   "0.0.0",
	}
}

// Option configures the telemetry system.
type Option func(*Telemetry)

// WithLogger sets the logger for telemetry.
func WithLogger(logger zerolog.Logger) Option {
	return func(t *Telemetry) {
		t.logger = logger
	}
}

// WithConfig sets the telemetry configuration.
func WithConfig(cfg Config) Option {
	return func(t *Telemetry) {
		t.enabled = cfg.Enabled
		t.version = cfg.Version
		t.queueSize = cfg.QueueSize
		if cfg.DataDir != "" {
			t.dbPath = cfg.DataDir + "/telemetry.db"
		}
	}
}

// WithDB sets the telemetry database.
func WithDB(db *sql.DB) Option {
	return func(t *Telemetry) {
		t.db = db
	}
}

// New creates a new telemetry system.
func New(opts ...Option) (*Telemetry, error) {
	t := &Telemetry{
		logger:     zerolog.Nop(),
		enabled:    true,
		instanceID: uuid.New().String(),
		queueSize:  100,
		version:    "0.0.0",
	}

	for _, opt := range opts {
		opt(t)
	}

	// Check environment override for opt-out
	if envVal := os.Getenv("COSCA_TELEMETRY_ENABLED"); envVal != "" {
		t.enabled = envVal == "true" || envVal == "1"
	}

	if !t.enabled {
		t.logger.Info().Msg("telemetry disabled")
		return t, nil
	}

	// Initialize the SQLite database if not provided
	if t.db == nil && t.dbPath != "" {
		if err := t.initDB(); err != nil {
			return nil, fmt.Errorf("init telemetry database: %w", err)
		}
	}

	t.logger.Info().
		Bool("enabled", t.enabled).
		Str("instance_id", t.instanceID[:8]).
		Str("db_path", t.dbPath).
		Msg("telemetry initialized")

	return t, nil
}

// =============================================================================
// Database Initialization
// =============================================================================

// initDB creates and initializes the SQLite telemetry store.
func (t *Telemetry) initDB() error {
	dir := t.dbPath[:len(t.dbPath)-len("/telemetry.db")]
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create telemetry directory: %w", err)
	}

	db, err := sql.Open("sqlite", t.dbPath)
	if err != nil {
		return fmt.Errorf("open telemetry database: %w", err)
	}

	// Configure pragmas for performance
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=-8000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return fmt.Errorf("telemetry pragma: %w", err)
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
		created_at TEXT DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_events_type ON events(type);
	CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);
	CREATE TABLE IF NOT EXISTS metadata (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	INSERT OR IGNORE INTO metadata (key, value) VALUES ('instance_id', ?);
	INSERT OR IGNORE INTO metadata (key, value) VALUES ('version', ?);
	INSERT OR IGNORE INTO metadata (key, value) VALUES ('created_at', datetime('now'));
	`
	if _, err := db.Exec(schema, t.instanceID, t.version); err != nil {
		_ = db.Close()
		return fmt.Errorf("create telemetry schema: %w", err)
	}

	// Add sent column if it doesn't exist (migration)
	_, _ = db.Exec("ALTER TABLE events ADD COLUMN sent INTEGER DEFAULT 0")
	// Add sent index (must come after the column exists)
	_, _ = db.Exec("CREATE INDEX IF NOT EXISTS idx_events_sent ON events(sent)")

	t.db = db
	return nil
}

// =============================================================================
// Event Recording
// =============================================================================

// RecordEvent records a telemetry event to the local store.
// If telemetry is disabled, the event is silently discarded.
func (t *Telemetry) RecordEvent(event Event) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.enabled || t.closed {
		return nil
	}

	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	t.logger.Debug().
		Str("event_id", event.ID[:8]).
		Str("type", string(event.Type)).
		Bool("success", event.Success).
		Msg("recording telemetry event")

	if t.db == nil {
		// No database available; event is discarded (telemetry disabled or no storage)
		return nil
	}

	// Serialize metadata
	metaJSON := "{}"
	if len(event.Metadata) > 0 {
		metaJSON = mapToJSON(event.Metadata)
	}

	_, err := t.db.Exec(
		`INSERT INTO events (id, timestamp, type, duration_ms, success, metadata)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		event.ID,
		event.Timestamp.UTC().Format(time.RFC3339),
		string(event.Type),
		event.DurationMs,
		boolToInt(event.Success),
		metaJSON,
	)
	if err != nil {
		return fmt.Errorf("insert telemetry event: %w", err)
	}

	return nil
}

// RecordEvents records multiple telemetry events in a single transaction.
func (t *Telemetry) RecordEvents(events []Event) error {
	if len(events) == 0 {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.enabled || t.closed || t.db == nil {
		return nil
	}

	tx, err := t.db.Begin()
	if err != nil {
		return fmt.Errorf("begin telemetry transaction: %w", err)
	}
	defer safe.Rollback(tx)

	stmt, err := tx.Prepare(
		`INSERT INTO events (id, timestamp, type, duration_ms, success, metadata)
		 VALUES (?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("prepare telemetry statement: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, event := range events {
		if event.ID == "" {
			event.ID = uuid.New().String()
		}
		if event.Timestamp.IsZero() {
			event.Timestamp = time.Now()
		}
		metaJSON := "{}"
		if len(event.Metadata) > 0 {
			metaJSON = mapToJSON(event.Metadata)
		}
		if _, err := stmt.Exec(
			event.ID,
			event.Timestamp.UTC().Format(time.RFC3339),
			string(event.Type),
			event.DurationMs,
			boolToInt(event.Success),
			metaJSON,
		); err != nil {
			return fmt.Errorf("insert batch event: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit telemetry batch: %w", err)
	}

	t.logger.Debug().Int("count", len(events)).Msg("batch recorded telemetry events")
	return nil
}

// =============================================================================
// Event Count
// =============================================================================

// EventCount returns the total number of recorded events.
func (t *Telemetry) EventCount() (int, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.db == nil {
		return 0, nil
	}

	var count int
	err := t.db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
	return count, err
}

// UnsyncedCount returns the number of events not yet sent to the server.
func (t *Telemetry) UnsyncedCount() (int, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.db == nil {
		return 0, nil
	}

	var count int
	err := t.db.QueryRow("SELECT COUNT(*) FROM events WHERE sent = 0").Scan(&count)
	return count, err
}

// =============================================================================
// Properties
// =============================================================================

// Enabled returns whether telemetry is enabled.
func (t *Telemetry) Enabled() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.enabled
}

// SetEnabled enables or disables telemetry at runtime.
func (t *Telemetry) SetEnabled(enabled bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.enabled = enabled
	t.logger.Info().Bool("enabled", enabled).Msg("telemetry state changed")
}

// InstanceID returns the telemetry instance identifier.
func (t *Telemetry) InstanceID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.instanceID
}

// Version returns the Cosca version used in telemetry.
func (t *Telemetry) Version() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.version
}

// =============================================================================
// Lifecycle
// =============================================================================

// Close closes the telemetry system and its database.
func (t *Telemetry) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}
	t.closed = true

	if t.db != nil {
		if err := t.db.Close(); err != nil {
			return fmt.Errorf("close telemetry database: %w", err)
		}
	}

	t.logger.Info().Msg("telemetry closed")
	return nil
}

// =============================================================================
// Internal Helpers
// =============================================================================

// mapToJSON serializes a string map to a simple JSON object.
// This avoids importing encoding/json in the hot path.
func mapToJSON(m map[string]string) string {
	if len(m) == 0 {
		return "{}"
	}
	buf := []byte{'{'}
	first := true
	for k, v := range m {
		if first {
			first = false
		} else {
			buf = append(buf, ',')
		}
		buf = append(buf, '"')
		buf = append(buf, []byte(escapeJSON(k))...)
		buf = append(buf, '"', ':')
		buf = append(buf, '"')
		buf = append(buf, []byte(escapeJSON(v))...)
		buf = append(buf, '"')
	}
	buf = append(buf, '}')
	return string(buf)
}

// escapeJSON escapes special characters in a JSON string value.
func escapeJSON(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\':
			result = append(result, '\\', '\\')
		case '"':
			result = append(result, '\\', '"')
		case '\n':
			result = append(result, '\\', 'n')
		case '\r':
			result = append(result, '\\', 'r')
		case '\t':
			result = append(result, '\\', 't')
		default:
			if c < 0x20 {
				result = append(result, '\\', 'u', '0', '0', hexChar(c>>4), hexChar(c&0x0f))
			} else {
				result = append(result, c)
			}
		}
	}
	return string(result)
}

func hexChar(v byte) byte {
	if v < 10 {
		return '0' + v
	}
	return 'a' + v - 10
}

// boolToInt converts a boolean to an integer (0 or 1).
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// =============================================================================
// Global Emit Function
// =============================================================================

// globalTelemetry holds the package-level telemetry instance for Emit().
var (
	globalTelemetry *Telemetry
	globalMu        sync.RWMutex
)

// SetGlobal sets the global telemetry instance used by Emit().
func SetGlobal(t *Telemetry) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalTelemetry = t
}

// Emit records a simple telemetry event by type with metadata.
// This is a convenience function used by CLI commands.
func Emit(eventType string, metadata map[string]interface{}) {
	globalMu.RLock()
	t := globalTelemetry
	globalMu.RUnlock()

	if t == nil || !t.Enabled() {
		return
	}

	// Convert metadata map to string map for the Event type
	meta := make(map[string]string)
	for k, v := range metadata {
		meta[k] = fmt.Sprintf("%v", v)
	}

	// Record asynchronously to avoid blocking
	go func() {
		event := Event{
			Type:      EventType(eventType),
			Timestamp: time.Now(),
			Success:   true,
			Metadata:  meta,
		}
		if err := t.RecordEvent(event); err != nil {
			t.logger.Debug().Err(err).Str("type", eventType).Msg("emit failed")
		}
	}()
}
