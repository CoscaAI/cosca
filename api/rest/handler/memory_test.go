package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/api/rest/handler"
	"github.com/CoscaAI/cosca/internal/memory"
)

// newTestMemoryEngine creates a fully initialized memory engine backed by a
// temporary directory. AutoPrune is disabled to avoid goroutine leaks under
// -race, and SnapshotOnPromote is disabled to keep Promote fast.
func newTestMemoryEngine(t *testing.T) *memory.MemoryEngine {
	t.Helper()
	cfg := memory.EngineConfig{
		DataDir:           t.TempDir(),
		AutoPrune:         false,
		SnapshotOnPromote: false,
	}
	engine, err := memory.NewEngine(memory.WithConfig(cfg))
	if err != nil {
		t.Fatalf("failed to create memory engine: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	return engine
}

// newTestMemoryHandler creates a MemoryHandler with a real engine.
func newTestMemoryHandler(t *testing.T) *handler.MemoryHandler {
	t.Helper()
	return handler.NewMemoryHandler(newTestMemoryEngine(t), nil)
}

// storeTestMemory stores a memory record through the handler and returns
// the parsed StoreResponse.
func storeTestMemory(t *testing.T, h *handler.MemoryHandler, body string) handler.StoreResponse {
	t.Helper()
	req := httptest.NewRequest("POST", "/v1/memory/store", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Store(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp handler.StoreResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal store response: %v", err)
	}
	if resp.ID == "" {
		t.Fatal("expected non-empty id in store response")
	}
	return resp
}

// TestMemoryStoreSuccess verifies Store returns 201 with an id and
// created_at timestamp.
func TestMemoryStoreSuccess(t *testing.T) {
	h := newTestMemoryHandler(t)

	body := `{"type":"fact","layer":"session","content":"Cosca was started in 2026","priority":5}`
	req := httptest.NewRequest("POST", "/v1/memory/store", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Store(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp handler.StoreResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.ID == "" {
		t.Error("expected non-empty id")
	}
	if resp.CreatedAt == "" {
		t.Error("expected non-empty created_at")
	}
}

// TestMemoryStoreFullRequest verifies Store accepts TTL, metadata, and
// scope fields and still succeeds.
func TestMemoryStoreFullRequest(t *testing.T) {
	h := newTestMemoryHandler(t)

	body := `{"type":"bug","layer":"session","scope":"api/rest","content":"Handler bug found","priority":9,"ttl":"2h","metadata":{"source":"test"}}`
	req := httptest.NewRequest("POST", "/v1/memory/store", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Store(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

// TestMemoryStoreInvalidLayer verifies Store returns 500 (and does not panic)
// when the engine rejects the layer. The engine has no backing store for
// unknown layers, so Store returns (nil, err); the handler must not
// dereference the nil record when building the error-path audit event.
func TestMemoryStoreInvalidLayer(t *testing.T) {
	h := newTestMemoryHandler(t)

	body := `{"content":"x","layer":"bogus"}`
	req := httptest.NewRequest("POST", "/v1/memory/store", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Store(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "store failed") {
		t.Errorf("expected body mentioning 'store failed', got %q", w.Body.String())
	}
}

// TestMemorySearchSuccess verifies Search returns stored records and covers
// recordToAPI through the search response.
func TestMemorySearchSuccess(t *testing.T) {
	h := newTestMemoryHandler(t)

	storeTestMemory(t, h, `{"type":"fact","layer":"session","content":"Golang concurrency uses goroutines"}`)

	req := httptest.NewRequest("GET", "/v1/memory/search?query=Golang", nil)
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp handler.MemorySearchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("expected 1 result, got %d", resp.Total)
	}
	if len(resp.Records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(resp.Records))
	}
	if resp.Records[0].Content == "" {
		t.Error("expected record content to be populated (recordToAPI mapping)")
	}
	if resp.Records[0].Layer != "session" {
		t.Errorf("expected layer 'session', got %q", resp.Records[0].Layer)
	}
}

// TestMemorySearchNoResults verifies Search returns an empty records array
// when nothing matches.
func TestMemorySearchNoResults(t *testing.T) {
	h := newTestMemoryHandler(t)

	storeTestMemory(t, h, `{"type":"fact","layer":"session","content":"Golang concurrency uses goroutines"}`)

	req := httptest.NewRequest("GET", "/v1/memory/search?query=quantum-flux-compensator", nil)
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp handler.MemorySearchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Records == nil {
		t.Error("expected non-nil records array")
	}
	if resp.Total != 0 {
		t.Errorf("expected 0 total, got %d", resp.Total)
	}
}

// TestMemorySearchWithParams verifies Search honors types, layers, and
// limit query parameters.
func TestMemorySearchWithParams(t *testing.T) {
	h := newTestMemoryHandler(t)

	storeTestMemory(t, h, `{"type":"fact","layer":"session","content":"Alpha memory content"}`)
	storeTestMemory(t, h, `{"type":"bug","layer":"session","content":"Beta memory content"}`)

	req := httptest.NewRequest("GET", "/v1/memory/search?query=memory&types=fact&layers=session&limit=1", nil)
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp handler.MemorySearchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Total > 1 {
		t.Errorf("expected limit 1 to cap results, got %d", resp.Total)
	}
}

// TestMemorySearchEmptyQuery verifies Search works with an empty query
// (returns recent records).
func TestMemorySearchEmptyQuery(t *testing.T) {
	h := newTestMemoryHandler(t)

	storeTestMemory(t, h, `{"type":"fact","layer":"session","content":"Recent record content"}`)

	req := httptest.NewRequest("GET", "/v1/memory/search", nil)
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestMemoryGetSuccess verifies Get returns the stored record and covers
// recordToAPI through the get response.
func TestMemoryGetSuccess(t *testing.T) {
	h := newTestMemoryHandler(t)

	saved := storeTestMemory(t, h, `{"type":"decision","layer":"session","content":"Decision content"}`)

	req := httptest.NewRequest("GET", "/v1/memory/get?id="+saved.ID+"&layer=session", nil)
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp handler.GetResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Record.ID != saved.ID {
		t.Errorf("expected id %q, got %q", saved.ID, resp.Record.ID)
	}
	if resp.Record.Type != "decision" {
		t.Errorf("expected type 'decision', got %q", resp.Record.Type)
	}
	if resp.Record.CreatedAt == "" {
		t.Error("expected created_at to be formatted (recordToAPI mapping)")
	}
}

// TestMemoryGetNotFound verifies Get returns 404 for an unknown id.
func TestMemoryGetNotFound(t *testing.T) {
	h := newTestMemoryHandler(t)

	req := httptest.NewRequest("GET", "/v1/memory/get?id=missing-id&layer=session", nil)
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestMemoryDeleteSuccess verifies Delete removes a stored record.
func TestMemoryDeleteSuccess(t *testing.T) {
	h := newTestMemoryHandler(t)

	saved := storeTestMemory(t, h, `{"type":"fact","layer":"session","content":"To be deleted"}`)

	req := httptest.NewRequest("DELETE", "/v1/memory/delete?id="+saved.ID+"&layer=session", nil)
	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp["status"] != "deleted" {
		t.Errorf("expected status 'deleted', got %q", resp["status"])
	}

	// The record must actually be gone.
	getReq := httptest.NewRequest("GET", "/v1/memory/get?id="+saved.ID+"&layer=session", nil)
	w2 := httptest.NewRecorder()
	h.Get(w2, getReq)
	if w2.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", w2.Code)
	}
}

// TestMemoryDeleteNotFound verifies Delete returns 404 when the record does
// not exist.
func TestMemoryDeleteNotFound(t *testing.T) {
	h := newTestMemoryHandler(t)

	req := httptest.NewRequest("DELETE", "/v1/memory/delete?id=missing-id&layer=session", nil)
	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestMemoryPromoteSuccess verifies Promote moves a record from session to
// project and covers recordToAPI through the promote response.
func TestMemoryPromoteSuccess(t *testing.T) {
	h := newTestMemoryHandler(t)

	saved := storeTestMemory(t, h, `{"type":"pattern","layer":"session","content":"Promotable pattern"}`)

	body := `{"id":"` + saved.ID + `","from_layer":"session","to_layer":"project"}`
	req := httptest.NewRequest("POST", "/v1/memory/promote", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Promote(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp handler.PromoteResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Record.ID != saved.ID {
		t.Errorf("expected id %q, got %q", saved.ID, resp.Record.ID)
	}
	if resp.Record.Layer != "project" {
		t.Errorf("expected promoted layer 'project', got %q", resp.Record.Layer)
	}
}

// TestMemoryPromoteInvalidLayer verifies Promote returns 500 when the target
// layer has no backing store.
func TestMemoryPromoteInvalidLayer(t *testing.T) {
	h := newTestMemoryHandler(t)

	saved := storeTestMemory(t, h, `{"type":"fact","layer":"session","content":"Promote to nowhere"}`)

	body := `{"id":"` + saved.ID + `","from_layer":"session","to_layer":"bogus"}`
	req := httptest.NewRequest("POST", "/v1/memory/promote", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Promote(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// TestMemoryStats verifies Stats returns a layers map even with a nil
// request body.
func TestMemoryStats(t *testing.T) {
	h := newTestMemoryHandler(t)

	storeTestMemory(t, h, `{"type":"fact","layer":"session","content":"Stats record"}`)

	req := httptest.NewRequest("GET", "/v1/memory/stats", nil)
	w := httptest.NewRecorder()
	h.Stats(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp handler.MemoryStatsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Layers == nil {
		t.Fatal("expected non-nil layers map")
	}
	if len(resp.Layers) == 0 {
		t.Fatal("expected at least one layer in stats")
	}
	if stats, ok := resp.Layers["session"]; ok && stats.RecordCount < 1 {
		t.Errorf("expected at least 1 record in session layer, got %d", stats.RecordCount)
	}
}

// TestMemoryStoreRecordExpires ensures the engine TTL expiry path used by
// Retrieve (Get handler 404) does not leak goroutines — creates a record
// with a past CreatedAt so Retrieve reports it as expired.
func TestMemoryStoreRecordExpires(t *testing.T) {
	engine := newTestMemoryEngine(t)
	ctx := context.Background()

	expired := memory.MemoryRecord{
		ID:        "expired-id",
		Type:      memory.MemoryType("fact"),
		Layer:     memory.LayerSession,
		Content:   "Expired content",
		CreatedAt: time.Now().Add(-48 * time.Hour),
		TTL:       24 * time.Hour,
	}
	saved, err := engine.Store(ctx, expired)
	if err != nil {
		t.Fatalf("failed to store expired record: %v", err)
	}
	if saved == nil {
		t.Fatal("expected saved record")
	}

	_, err = engine.Retrieve(ctx, "expired-id", memory.LayerSession)
	if err == nil {
		t.Fatal("expected expired record retrieval to fail")
	}

	h := handler.NewMemoryHandler(engine, nil)
	req := httptest.NewRequest("GET", "/v1/memory/get?id=expired-id&layer=session", nil)
	w := httptest.NewRecorder()
	h.Get(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for expired record, got %d: %s", w.Code, w.Body.String())
	}
}
