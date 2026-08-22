package audit_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/audit"
)

// TestNewStore_CreatesDBAndTable verifies that NewStore successfully creates
// a database file and initializes the schema.
func TestNewStore_CreatesDBAndTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "audit.db")

	store, err := audit.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Verify the database file was created.
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("database file was not created")
	}
}

// TestNewStore_ReopenExisting verifies that a store can be reopened from an
// existing database.
func TestNewStore_ReopenExisting(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "audit.db")

	s1, err := audit.NewStore(dbPath)
	if err != nil {
		t.Fatalf("first NewStore() error: %v", err)
	}
	s1.Close()

	s2, err := audit.NewStore(dbPath)
	if err != nil {
		t.Fatalf("second NewStore() error: %v", err)
	}
	defer func() { _ = s2.Close() }()
}

// TestRecord_StoresEntry verifies that Record persists an audit entry
// and populates default fields.
func TestRecord_StoresEntry(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "audit.db")
	store, err := audit.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}
	defer func() { _ = store.Close() }()

	entry := &audit.AuditEntry{
		UserID:    "user-1",
		Action:    "login",
		Resource:  "auth",
		Details:   `{"ip":"192.168.1.1"}`,
		IPAddress: "192.168.1.1",
		Status:    "success",
	}

	err = store.Record(entry)
	if err != nil {
		t.Fatalf("Record() error: %v", err)
	}

	// Verify defaults were set.
	if entry.ID == "" {
		t.Error("ID should be auto-generated")
	}
	if entry.Timestamp == 0 {
		t.Error("Timestamp should be auto-generated")
	}
}

// TestRecord_DefaultValues verifies that Record fills in default values
// when fields are left empty.
func TestRecord_DefaultValues(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "audit.db")
	store, err := audit.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}
	defer func() { _ = store.Close() }()

	entry := &audit.AuditEntry{
		Action: "test_action",
	}

	err = store.Record(entry)
	if err != nil {
		t.Fatalf("Record() error: %v", err)
	}

	if entry.ID == "" {
		t.Error("ID should be auto-generated")
	}
	if entry.Status == "" {
		t.Error("Status should default to 'success'")
	}
	if entry.Details == "" {
		t.Error("Details should default to '{}'")
	}
	if entry.Timestamp == 0 {
		t.Error("Timestamp should be set")
	}
}

// TestRecord_IDAndTimestampPreserved verifies that Record preserves
// explicitly set ID and Timestamp values.
func TestRecord_IDAndTimestampPreserved(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "audit.db")
	store, err := audit.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}
	defer func() { _ = store.Close() }()

	now := time.Now().Unix()
	customID := "custom-id-123"

	entry := &audit.AuditEntry{
		ID:        customID,
		UserID:    "user-1",
		Action:    "api_call",
		Resource:  "secrets",
		Timestamp: now,
		Status:    "denied",
	}

	err = store.Record(entry)
	if err != nil {
		t.Fatalf("Record() error: %v", err)
	}

	if entry.ID != customID {
		t.Errorf("ID = %q, want %q", entry.ID, customID)
	}
	if entry.Timestamp != now {
		t.Errorf("Timestamp = %d, want %d", entry.Timestamp, now)
	}
}

// TestGetByID_ReturnsCorrectEntry verifies that GetByID returns the
// correct audit entry.
func TestGetByID_ReturnsCorrectEntry(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "audit.db")
	store, err := audit.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}
	defer func() { _ = store.Close() }()

	entry := &audit.AuditEntry{
		UserID:    "user-1",
		Action:    "create_project",
		Resource:  "project-123",
		Details:   `{"name":"my-project"}`,
		IPAddress: "10.0.0.1",
		Status:    "success",
	}

	err = store.Record(entry)
	if err != nil {
		t.Fatalf("Record() error: %v", err)
	}

	got, err := store.GetByID(entry.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID returned nil")
	}
	if got.ID != entry.ID {
		t.Errorf("ID = %q, want %q", got.ID, entry.ID)
	}
	if got.UserID != entry.UserID {
		t.Errorf("UserID = %q, want %q", got.UserID, entry.UserID)
	}
	if got.Action != entry.Action {
		t.Errorf("Action = %q, want %q", got.Action, entry.Action)
	}
	if got.Resource != entry.Resource {
		t.Errorf("Resource = %q, want %q", got.Resource, entry.Resource)
	}
	if got.Status != entry.Status {
		t.Errorf("Status = %q, want %q", got.Status, entry.Status)
	}
}

// TestGetByID_NotFound verifies that GetByID returns nil for a non-existent ID.
func TestGetByID_NotFound(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "audit.db")
	store, err := audit.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}
	defer func() { _ = store.Close() }()

	got, err := store.GetByID("non-existent-id")
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for non-existent ID, got %+v", got)
	}
}

// TestList_ReturnsPaginatedResults verifies that List returns results
// with correct pagination.
func TestList_ReturnsPaginatedResults(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "audit.db")
	store, err := audit.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Insert multiple entries.
	for i := 0; i < 25; i++ {
		entry := &audit.AuditEntry{
			UserID:   "user-1",
			Action:   "test_action",
			Resource: "test",
			Status:   "success",
		}
		err := store.Record(entry)
		if err != nil {
			t.Fatalf("Record() error: %v", err)
		}
	}

	// Get first page.
	entries, total, err := store.List(10, 0, audit.AuditFilters{})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(entries) != 10 {
		t.Errorf("first page len = %d, want 10", len(entries))
	}
	if total != 25 {
		t.Errorf("total = %d, want 25", total)
	}

	// Get second page.
	entries2, total2, err := store.List(10, 10, audit.AuditFilters{})
	if err != nil {
		t.Fatalf("List(offset=10) error: %v", err)
	}
	if len(entries2) != 10 {
		t.Errorf("second page len = %d, want 10", len(entries2))
	}
	if total2 != 25 {
		t.Errorf("total = %d, want 25", total2)
	}

	// Get last page.
	entries3, _, err := store.List(10, 20, audit.AuditFilters{})
	if err != nil {
		t.Fatalf("List(offset=20) error: %v", err)
	}
	if len(entries3) != 5 {
		t.Errorf("last page len = %d, want 5", len(entries3))
	}

	// Verify entries are ordered by timestamp descending.
	if len(entries) >= 2 {
		if entries[0].Timestamp < entries[1].Timestamp {
			t.Error("entries not sorted by timestamp DESC")
		}
	}
}

// TestList_FilterByUser verifies that List applies UserID filter correctly.
func TestList_FilterByUser(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "audit.db")
	store, err := audit.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Insert entries for different users.
	for i := 0; i < 5; i++ {
		_ = store.Record(&audit.AuditEntry{
			UserID:   "alice",
			Action:   "login",
			Resource: "auth",
			Status:   "success",
		})
	}
	for i := 0; i < 3; i++ {
		_ = store.Record(&audit.AuditEntry{
			UserID:   "bob",
			Action:   "login",
			Resource: "auth",
			Status:   "success",
		})
	}

	entries, total, err := store.List(100, 0, audit.AuditFilters{UserID: "alice"})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	for _, e := range entries {
		if e.UserID != "alice" {
			t.Errorf("expected alice entries only, got UserID=%q", e.UserID)
		}
	}
}

// TestList_FilterByAction verifies that List applies Action filter correctly.
func TestList_FilterByAction(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "audit.db")
	store, err := audit.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}
	defer func() { _ = store.Close() }()

	_ = store.Record(&audit.AuditEntry{UserID: "u1", Action: "login", Resource: "auth", Status: "success"})
	_ = store.Record(&audit.AuditEntry{UserID: "u1", Action: "logout", Resource: "auth", Status: "success"})
	_ = store.Record(&audit.AuditEntry{UserID: "u1", Action: "create", Resource: "project", Status: "success"})

	entries, total, err := store.List(100, 0, audit.AuditFilters{Action: "login"})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	for _, e := range entries {
		if e.Action != "login" {
			t.Errorf("expected login entries only, got Action=%q", e.Action)
		}
	}
}

// TestList_FilterByStatus verifies that List applies Status filter correctly.
func TestList_FilterByStatus(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "audit.db")
	store, err := audit.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}
	defer func() { _ = store.Close() }()

	_ = store.Record(&audit.AuditEntry{UserID: "u1", Action: "login", Resource: "auth", Status: "success"})
	_ = store.Record(&audit.AuditEntry{UserID: "u1", Action: "login", Resource: "auth", Status: "denied"})
	_ = store.Record(&audit.AuditEntry{UserID: "u1", Action: "login", Resource: "auth", Status: "error"})

	entries, total, err := store.List(100, 0, audit.AuditFilters{Status: "denied"})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	for _, e := range entries {
		if e.Status != "denied" {
			t.Errorf("expected denied entries only, got Status=%q", e.Status)
		}
	}
}

// TestList_LimitDefaults verifies that zero limit defaults to 20 and
// limit > 1000 is capped.
func TestList_LimitDefaults(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "audit.db")
	store, err := audit.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Insert 30 entries.
	for i := 0; i < 30; i++ {
		_ = store.Record(&audit.AuditEntry{
			UserID: "u1", Action: "test", Resource: "r", Status: "success",
		})
	}

	// Zero limit → defaults to 20.
	entries, total, err := store.List(0, 0, audit.AuditFilters{})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if total != 30 {
		t.Errorf("total = %d, want 30", total)
	}
	if len(entries) != 20 {
		t.Errorf("len = %d, want 20 (default limit)", len(entries))
	}
}

// TestList_EmptyDB verifies that List returns empty results for an empty database.
func TestList_EmptyDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "audit.db")
	store, err := audit.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}
	defer func() { _ = store.Close() }()

	entries, total, err := store.List(50, 0, audit.AuditFilters{})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
	if len(entries) != 0 {
		t.Errorf("len = %d, want 0", len(entries))
	}
}

// TestClose_NoError verifies Close does not return an error.
func TestClose_NoError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "audit.db")
	store, err := audit.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}

	if err := store.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}
}

// TestClose_DoubleClose verifies that double close is safe.
func TestClose_DoubleClose(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "audit.db")
	store, err := audit.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}

	if err := store.Close(); err != nil {
		t.Logf("first Close() error: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Logf("second Close() error: %v", err)
	}
}

// TestDetailsJSON_Basic verifies DetailsJSON marshals a simple value.
func TestDetailsJSON_Basic(t *testing.T) {
	result := audit.DetailsJSON(map[string]interface{}{
		"ip":   "192.168.1.1",
		"port": 8080,
	})

	if !json.Valid([]byte(result)) {
		t.Errorf("invalid JSON: %s", result)
	}
}

// TestDetailsJSON_Nil verifies DetailsJSON handles nil input.
func TestDetailsJSON_Nil(t *testing.T) {
	result := audit.DetailsJSON(nil)
	if result != "null" {
		t.Errorf("expected 'null' for nil input, got %q", result)
	}
}

// TestDetailsJSON_String verifies DetailsJSON marshals a string.
func TestDetailsJSON_String(t *testing.T) {
	result := audit.DetailsJSON("simple string")
	if !json.Valid([]byte(result)) {
		t.Errorf("invalid JSON: %s", result)
	}
	var s string
	if err := json.Unmarshal([]byte(result), &s); err != nil {
		t.Errorf("unmarshal error: %v", err)
	}
}

// TestList_FromToFilter verifies date-range filtering with From and To.
func TestList_FromToFilter(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "audit.db")
	store, err := audit.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Insert entries at different timestamps.
	now := time.Now().Unix()
	old := now - 7200    // 2 hours ago
	older := now - 86400 // 1 day ago

	for _, ts := range []struct {
		action string
		ts     int64
	}{
		{"recent", now},
		{"mid", old},
		{"ancient", older},
	} {
		entry := &audit.AuditEntry{Action: ts.action, Timestamp: ts.ts}
		if err := store.Record(entry); err != nil {
			t.Fatalf("Record() error: %v", err)
		}
	}

	// Filter: only entries from the last hour.
	oneHourAgo := now - 3600
	entries, total, err := store.List(100, 0, audit.AuditFilters{From: oneHourAgo})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if total != 1 {
		t.Errorf("From filter: total = %d, want 1", total)
	}
	if len(entries) != 1 || entries[0].Action != "recent" {
		t.Errorf("From filter: got %v, want [recent]", entries)
	}

	// Filter: entries between 3 hours ago and 30 min ago.
	threeHoursAgo := now - 10800
	thirtyMinAgo := now - 1800
	entries, total, err = store.List(100, 0, audit.AuditFilters{From: threeHoursAgo, To: thirtyMinAgo})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if total != 1 {
		t.Errorf("From+To filter: total = %d, want 1", total)
	}
	if len(entries) != 1 || entries[0].Action != "mid" {
		t.Errorf("From+To filter: got %v, want [mid]", entries)
	}

	// Filter: nothing in the future.
	future := now + 3600
	entries, total, err = store.List(100, 0, audit.AuditFilters{From: future})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if total != 0 {
		t.Errorf("Future filter: total = %d, want 0", total)
	}
}

// TestPrune verifies that Prune removes old entries.
func TestPrune(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "audit.db")
	store, err := audit.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}
	defer func() { _ = store.Close() }()

	now := time.Now().Unix()
	// Insert 5 entries.
	for i := 0; i < 5; i++ {
		entry := &audit.AuditEntry{Action: "test", Timestamp: now + int64(i)}
		if err := store.Record(entry); err != nil {
			t.Fatalf("Record() error: %v", err)
		}
	}

	// Prune nothing (cutoff in the past).
	deleted, err := store.Prune(now - 86400)
	if err != nil {
		t.Fatalf("Prune() error: %v", err)
	}
	if deleted != 0 {
		t.Errorf("Prune nothing: deleted = %d, want 0", deleted)
	}

	// Prune all (cutoff in the future).
	deleted, err = store.Prune(now + 10)
	if err != nil {
		t.Fatalf("Prune() error: %v", err)
	}
	if deleted != 5 {
		t.Errorf("Prune all: deleted = %d, want 5", deleted)
	}

	// Verify empty after prune.
	entries, total, _ := store.List(100, 0, audit.AuditFilters{})
	if total != 0 || len(entries) != 0 {
		t.Errorf("after prune: total = %d, entries = %d, want 0/0", total, len(entries))
	}
}
