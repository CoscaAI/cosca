package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/api/rest/handler"
	"github.com/CoscaAI/cosca/internal/audit"
)

// newIsolatedAuditStore creates a fresh temp-file-backed audit store. Unlike
// newTestAuditStore (which shares a single in-memory DB across the process),
// this store is isolated so tests can assert exact counts.
func newIsolatedAuditStore(t *testing.T) *audit.Store {
	t.Helper()
	store, err := audit.NewStore(t.TempDir() + "/audit.db")
	if err != nil {
		t.Fatalf("failed to create isolated audit store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// =============================================================================
// Audit List with from/to params — audit.go:40
// Exercises parseTimeParam (audit.go:148) and parseInt64 (audit.go:167).
// NOTE: the nil-store guard runs before parameter parsing, so a real store is
// required to reach parseTimeParam/parseInt64.
// =============================================================================

// TestAuditListEmptyStore verifies List against an empty store returns 200
// with zero total and an empty entries array.
func TestAuditListEmptyStore(t *testing.T) {
	h := handler.NewAuditHandler(newIsolatedAuditStore(t))

	req := httptest.NewRequest("GET", "/v1/audit/logs", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if total, ok := resp["total"].(float64); ok && total != 0 {
		t.Errorf("expected total 0 for empty store, got %v", total)
	}
	entries, ok := resp["entries"].([]interface{})
	if !ok {
		t.Fatal("expected 'entries' array in response")
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

// TestAuditListFromToUnix verifies Unix-timestamp from/to params are parsed by
// parseInt64 (multi-digit success path) and filter the results correctly.
func TestAuditListFromToUnix(t *testing.T) {
	store := newIsolatedAuditStore(t)
	now := time.Now().Unix()

	inside := &audit.AuditEntry{
		UserID: "fromto-user", Action: "inside.action", Resource: "r",
		IPAddress: "127.0.0.1", Timestamp: now - 3600, Status: "success",
	}
	outside := &audit.AuditEntry{
		UserID: "fromto-user", Action: "outside.action", Resource: "r",
		IPAddress: "127.0.0.1", Timestamp: now - 86400*10, Status: "success",
	}
	if err := store.Record(inside); err != nil {
		t.Fatalf("failed to record entry: %v", err)
	}
	if err := store.Record(outside); err != nil {
		t.Fatalf("failed to record entry: %v", err)
	}

	h := handler.NewAuditHandler(store)

	from := fmt.Sprintf("%d", now-86400)
	to := fmt.Sprintf("%d", now+86400)
	req := httptest.NewRequest("GET", "/v1/audit/logs?from="+from+"&to="+to, nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	entries, ok := resp["entries"].([]interface{})
	if !ok {
		t.Fatal("expected 'entries' array in response")
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry inside from/to window, got %d", len(entries))
	}
	first := entries[0].(map[string]interface{})
	if first["action"] != "inside.action" {
		t.Errorf("expected action 'inside.action', got %v", first["action"])
	}
	if total, ok := resp["total"].(float64); ok && total != 1 {
		t.Errorf("expected total 1, got %v", total)
	}
}

// TestAuditListFromToRFC3339 verifies RFC3339 from/to params are parsed.
func TestAuditListFromToRFC3339(t *testing.T) {
	store := newIsolatedAuditStore(t)
	entry := &audit.AuditEntry{
		UserID: "rfc-user", Action: "rfc.action", Resource: "r",
		IPAddress: "127.0.0.1", Timestamp: time.Now().Unix(), Status: "success",
	}
	if err := store.Record(entry); err != nil {
		t.Fatalf("failed to record entry: %v", err)
	}

	h := handler.NewAuditHandler(store)

	from := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	to := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/v1/audit/logs?from="+from+"&to="+to, nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	entries, ok := resp["entries"].([]interface{})
	if !ok {
		t.Fatal("expected 'entries' array in response")
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry within RFC3339 window, got %d", len(entries))
	}
}

// TestAuditListFromToDateOnly verifies date-only (YYYY-MM-DD) params are parsed.
func TestAuditListFromToDateOnly(t *testing.T) {
	store := newIsolatedAuditStore(t)
	entry := &audit.AuditEntry{
		UserID: "date-user", Action: "date.action", Resource: "r",
		IPAddress: "127.0.0.1", Timestamp: time.Now().Unix(), Status: "success",
	}
	if err := store.Record(entry); err != nil {
		t.Fatalf("failed to record entry: %v", err)
	}

	h := handler.NewAuditHandler(store)

	from := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	to := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
	req := httptest.NewRequest("GET", "/v1/audit/logs?from="+from+"&to="+to, nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	entries, ok := resp["entries"].([]interface{})
	if !ok {
		t.Fatal("expected 'entries' array in response")
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry within date-only window, got %d", len(entries))
	}
}

// TestAuditListZeroTimeParams verifies from=0/to=0 exercise the parseInt64
// success-with-zero branch and fall through the remaining parsers (returning 0,
// which means no time filtering).
func TestAuditListZeroTimeParams(t *testing.T) {
	h := handler.NewAuditHandler(newIsolatedAuditStore(t))

	req := httptest.NewRequest("GET", "/v1/audit/logs?from=0&to=0", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAuditListInvalidTimeParams verifies invalid timestamp strings are
// ignored (parse to 0) without breaking the request.
func TestAuditListInvalidTimeParams(t *testing.T) {
	h := handler.NewAuditHandler(newIsolatedAuditStore(t))

	req := httptest.NewRequest("GET", "/v1/audit/logs?from=not-a-time&to=garbage", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAuditListFiltersNoMatch verifies List returns an empty page when a
// filter matches nothing.
func TestAuditListFiltersNoMatch(t *testing.T) {
	store := newIsolatedAuditStore(t)
	entry := &audit.AuditEntry{
		UserID: "filter-user", Action: "filter.action", Resource: "r",
		IPAddress: "127.0.0.1", Timestamp: time.Now().Unix(), Status: "success",
	}
	if err := store.Record(entry); err != nil {
		t.Fatalf("failed to record entry: %v", err)
	}

	h := handler.NewAuditHandler(store)

	req := httptest.NewRequest("GET", "/v1/audit/logs?user_id=no-such-user-xyz", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if total, ok := resp["total"].(float64); ok && total != 0 {
		t.Errorf("expected total 0 for non-matching filter, got %v", total)
	}
	entries, ok := resp["entries"].([]interface{})
	if !ok {
		t.Fatal("expected 'entries' array in response")
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for non-matching filter, got %d", len(entries))
	}
}

// =============================================================================
// Audit Get — audit.go:86 (remaining branches with a real store)
// =============================================================================

// TestAuditGetMissingIDWithStore verifies Get returns 400 when the id is empty
// even with a real store available.
func TestAuditGetMissingIDWithStore(t *testing.T) {
	h := handler.NewAuditHandler(newIsolatedAuditStore(t))

	req := httptest.NewRequest("GET", "/v1/audit/logs/", nil)
	req.SetPathValue("id", "")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAuditGetNotFoundWithStore verifies Get returns 404 when the entry does
// not exist.
func TestAuditGetNotFoundWithStore(t *testing.T) {
	h := handler.NewAuditHandler(newIsolatedAuditStore(t))

	req := httptest.NewRequest("GET", "/v1/audit/logs/does-not-exist", nil)
	req.SetPathValue("id", "does-not-exist")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d: %s", w.Code, w.Body.String())
	}
}
