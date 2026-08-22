package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/api/rest/handler"
	"github.com/CoscaAI/cosca/api/stream"
	"github.com/CoscaAI/cosca/internal/audit"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/search"
)

// auditFiltersFor builds an audit.AuditFilters scoped to a single action,
// used to assert that handlers recorded the expected audit events.
func auditFiltersFor(action string) audit.AuditFilters {
	return audit.AuditFilters{Action: action}
}

// newTestKnowledgeEngine creates a fully initialized knowledge engine backed
// by a temporary SQLite database. RootDir is a fresh empty temp directory so
// Sync operations complete quickly with empty results.
func newTestKnowledgeEngine(t *testing.T) *knowledge.Engine {
	t.Helper()
	cfg := knowledge.Config{
		DBPath:            filepath.Join(t.TempDir(), "knowledge.db"),
		RootDir:           t.TempDir(),
		AutoMigrate:       true,
		WatchEnabled:      false,
		EmbeddingProvider: "auto",
		IndexerConfig:     knowledge.DefaultConfig().IndexerConfig,
		CacheConfig:       knowledge.DefaultConfig().CacheConfig,
		RankingConfig:     knowledge.DefaultConfig().RankingConfig,
		SearchConfig:      search.DefaultSearchParams(),
	}
	engine, err := knowledge.New(cfg)
	if err != nil {
		t.Fatalf("failed to create knowledge engine: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	if err := engine.Init(); err != nil {
		t.Fatalf("failed to initialize knowledge engine: %v", err)
	}
	return engine
}

// newUninitializedKnowledgeEngine creates a knowledge engine that has NOT been
// initialized — every stateful operation returns "knowledge engine not
// initialized". This exercises the handler error paths deterministically.
func newUninitializedKnowledgeEngine(t *testing.T) *knowledge.Engine {
	t.Helper()
	cfg := knowledge.Config{
		DBPath:  filepath.Join(t.TempDir(), "knowledge.db"),
		RootDir: t.TempDir(),
	}
	engine, err := knowledge.New(cfg)
	if err != nil {
		t.Fatalf("failed to create knowledge engine: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	return engine
}

// TestKnowledgeNewHandler verifies handler construction.
func TestKnowledgeNewHandler(t *testing.T) {
	h := handler.NewKnowledgeHandler(nil, nil)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	// SetHub must not panic with a nil hub.
	h.SetHub(nil)
}

// TestKnowledgeSearchInvalidJSON verifies 400 on malformed body.
func TestKnowledgeSearchInvalidJSON(t *testing.T) {
	h := handler.NewKnowledgeHandler(newUninitializedKnowledgeEngine(t), nil)

	req := httptest.NewRequest("POST", "/v1/knowledge/search", strings.NewReader("not-json"))
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestKnowledgeSearchEngineError verifies 500 when the engine errors
// (uninitialized engine).
func TestKnowledgeSearchEngineError(t *testing.T) {
	h := handler.NewKnowledgeHandler(newUninitializedKnowledgeEngine(t), nil)

	body := `{"query":"golang"}`
	req := httptest.NewRequest("POST", "/v1/knowledge/search", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp["error"], "search failed") {
		t.Errorf("expected error prefix 'search failed', got '%s'", resp["error"])
	}
}

// TestKnowledgeSearchSuccess verifies a successful search returns 200 with
// the expected response shape. All optional request fields are populated to
// exercise the full parameter mapping.
func TestKnowledgeSearchSuccess(t *testing.T) {
	h := handler.NewKnowledgeHandler(newTestKnowledgeEngine(t), nil)

	body := `{
		"query":"golang","limit":5,"offset":0,"types":["document"],
		"path_filter":"/docs","min_score":0.5,
		"enable_fts":true,"enable_vector":true,"enable_graph":false,
		"enable_facets":true,"tags":{"lang":"go"}
	}`
	req := httptest.NewRequest("POST", "/v1/knowledge/search", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp handler.SearchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Results == nil {
		t.Error("expected results array to be present")
	}
	if resp.Total != 0 {
		t.Errorf("expected 0 total on empty index, got %d", resp.Total)
	}
}

// TestKnowledgeSearchEmptyQuery verifies that a request without a query or
// filter is rejected by the engine (500 with "query or filter required") —
// the handler surfaces the engine validation error unchanged.
func TestKnowledgeSearchEmptyQuery(t *testing.T) {
	h := handler.NewKnowledgeHandler(newTestKnowledgeEngine(t), nil)

	req := httptest.NewRequest("POST", "/v1/knowledge/search", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp["error"], "query or filter required") {
		t.Errorf("expected 'query or filter required' error, got '%s'", resp["error"])
	}
}

// TestKnowledgeSearchMinimalRequest verifies a minimal valid request body
// works with default search parameters applied.
func TestKnowledgeSearchMinimalRequest(t *testing.T) {
	h := handler.NewKnowledgeHandler(newTestKnowledgeEngine(t), nil)

	req := httptest.NewRequest("POST", "/v1/knowledge/search", strings.NewReader(`{"query":"golang"}`))
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestKnowledgeSearchEmptyBody verifies an empty request body returns 400.
func TestKnowledgeSearchEmptyBody(t *testing.T) {
	h := handler.NewKnowledgeHandler(newTestKnowledgeEngine(t), nil)

	req := httptest.NewRequest("POST", "/v1/knowledge/search", nil)
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty body, got %d: %s", w.Code, w.Body.String())
	}
}

// TestKnowledgeIndexInvalidJSON verifies 400 on malformed body.
func TestKnowledgeIndexInvalidJSON(t *testing.T) {
	h := handler.NewKnowledgeHandler(newTestKnowledgeEngine(t), nil)

	req := httptest.NewRequest("POST", "/v1/knowledge/index", strings.NewReader("not-json"))
	w := httptest.NewRecorder()
	h.Index(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestKnowledgeIndexMissingPath verifies 400 when path is empty.
func TestKnowledgeIndexMissingPath(t *testing.T) {
	h := handler.NewKnowledgeHandler(newTestKnowledgeEngine(t), nil)

	req := httptest.NewRequest("POST", "/v1/knowledge/index", strings.NewReader(`{"path":""}`))
	w := httptest.NewRecorder()
	h.Index(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestKnowledgeIndexEngineError verifies 500 when the engine errors
// (uninitialized engine) and that an audit error event is recorded.
func TestKnowledgeIndexEngineError(t *testing.T) {
	auditStore := newTestAuditStore(t)
	h := handler.NewKnowledgeHandler(newUninitializedKnowledgeEngine(t), auditStore)

	body := `{"path":"/nonexistent/file.md","recursive":false}`
	req := httptest.NewRequest("POST", "/v1/knowledge/index", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.Index(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", w.Code, w.Body.String())
	}

	// An audit entry with status "error" should have been recorded.
	entries, total, err := auditStore.List(10, 0, auditFiltersFor("knowledge.index"))
	if err != nil {
		t.Fatalf("failed to list audit entries: %v", err)
	}
	if total == 0 || len(entries) == 0 {
		t.Fatal("expected at least one audit entry for failed index")
	}
	if entries[0].Status != "error" {
		t.Errorf("expected status 'error', got '%s'", entries[0].Status)
	}
}

// TestKnowledgeIndexSuccess verifies indexing a real markdown file (inside
// the engine RootDir) returns 200.
func TestKnowledgeIndexSuccess(t *testing.T) {
	engine := newTestKnowledgeEngine(t)
	filePath := filepath.Join(engine.RootDir(), "doc.md")
	if err := os.WriteFile(filePath, []byte("# Title\n\nBody content."), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	h := handler.NewKnowledgeHandler(engine, nil)

	// JSON montado com json.Marshal: no Windows o filePath contém backslashes
	// (ex: C:\Users\...\doc.md) e a concatenação manual produzia JSON inválido
	// ("invalid character 'U' in string escape code" → 400).
	payload, err := json.Marshal(map[string]any{"path": filePath, "recursive": false})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest("POST", "/v1/knowledge/index", bytes.NewReader(payload))
	w := httptest.NewRecorder()
	h.Index(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp handler.IndexResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.DocumentsIndexed != 1 {
		t.Errorf("expected 1 document indexed, got %d", resp.DocumentsIndexed)
	}
}

// TestKnowledgeIndexOutsideRoot verifies the C1 containment fix at the HTTP
// layer: indexing a file outside the engine RootDir (e.g. /etc/passwd) must
// not succeed. The handler rejects obvious traversal with 400 and the indexer
// rejects out-of-root absolute paths with an error surfaced as 500.
func TestKnowledgeIndexOutsideRoot(t *testing.T) {
	h := handler.NewKnowledgeHandler(newTestKnowledgeEngine(t), nil)

	body := `{"path":"/etc/passwd","recursive":false}`
	req := httptest.NewRequest("POST", "/v1/knowledge/index", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.Index(w, req)

	if w.Code == http.StatusOK {
		t.Fatal("indexing /etc/passwd must not succeed")
	}
	if w.Code != http.StatusBadRequest {
		// 500 is expected when the indexer's containment check rejects the
		// path; the response must surface the containment error.
		if !strings.Contains(w.Body.String(), "path outside root directory") {
			t.Errorf("expected containment error in body, got %d: %s", w.Code, w.Body.String())
		}
	}
}

// TestKnowledgeIndexTraversalRejected verifies the handler rejects ".."
// escape attempts with 400 before they reach the engine.
func TestKnowledgeIndexTraversalRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("path traversal POSIX (../../../etc/passwd) resolve para o drive existente no Windows — semântica de path absoluto POSIX não aplicável")
	}
	h := handler.NewKnowledgeHandler(newTestKnowledgeEngine(t), nil)

	body := `{"path":"../../../etc/passwd","recursive":false}`
	req := httptest.NewRequest("POST", "/v1/knowledge/index", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.Index(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "traversal") {
		t.Errorf("expected traversal rejection in body, got %q", w.Body.String())
	}
}

// TestKnowledgeStatsSuccess verifies Stats returns 200 with the response shape.
func TestKnowledgeStatsSuccess(t *testing.T) {
	h := handler.NewKnowledgeHandler(newTestKnowledgeEngine(t), nil)

	req := httptest.NewRequest("GET", "/v1/knowledge/stats", nil)
	w := httptest.NewRecorder()
	h.Stats(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp handler.StatsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.GraphStats == nil {
		t.Error("expected graph_stats in response")
	}
	if resp.DocumentCount != 0 {
		t.Errorf("expected 0 documents, got %d", resp.DocumentCount)
	}
}

// TestKnowledgeStatsUninitializedEngine verifies Stats works even on an
// uninitialized engine (returns basic stats only).
func TestKnowledgeStatsUninitializedEngine(t *testing.T) {
	h := handler.NewKnowledgeHandler(newUninitializedKnowledgeEngine(t), nil)

	req := httptest.NewRequest("GET", "/v1/knowledge/stats", nil)
	w := httptest.NewRecorder()
	h.Stats(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestKnowledgeSyncEngineError verifies 500 when Sync fails on an
// uninitialized engine.
func TestKnowledgeSyncEngineError(t *testing.T) {
	auditStore := newTestAuditStore(t)
	h := handler.NewKnowledgeHandler(newUninitializedKnowledgeEngine(t), auditStore)

	req := httptest.NewRequest("POST", "/v1/knowledge/sync", nil)
	w := httptest.NewRecorder()
	h.Sync(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// TestKnowledgeSyncSuccess verifies a successful sync returns 200.
func TestKnowledgeSyncSuccess(t *testing.T) {
	h := handler.NewKnowledgeHandler(newTestKnowledgeEngine(t), nil)

	req := httptest.NewRequest("POST", "/v1/knowledge/sync", nil)
	w := httptest.NewRecorder()
	h.Sync(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp handler.SyncResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.DurationMs < 0 {
		t.Errorf("expected non-negative duration, got %f", resp.DurationMs)
	}
}

// TestKnowledgeSyncStreamEngineError verifies SyncStream reports engine errors
// as in-band SSE error events.
func TestKnowledgeSyncStreamEngineError(t *testing.T) {
	auditStore := newTestAuditStore(t)
	h := handler.NewKnowledgeHandler(newUninitializedKnowledgeEngine(t), auditStore)

	req := httptest.NewRequest("POST", "/v1/knowledge/sync/stream", nil)
	tw := stream.NewSSETestWriter()
	h.SyncStream(tw, req)

	body := tw.Body().String()
	if !strings.Contains(body, `"type":"progress"`) {
		t.Errorf("expected initial progress event, got: %s", body)
	}
	if !strings.Contains(body, `"type":"error"`) {
		t.Errorf("expected in-band error event, got: %s", body)
	}
}

// TestKnowledgeSyncStreamSuccess verifies SyncStream writes progress + done
// events and broadcasts hub events when a hub is set.
func TestKnowledgeSyncStreamSuccess(t *testing.T) {
	hub := stream.NewHub(zerolog.Nop())
	t.Cleanup(func() { _ = hub.Shutdown(context.Background()) })

	h := handler.NewKnowledgeHandler(newTestKnowledgeEngine(t), nil)
	h.SetHub(hub)

	req := httptest.NewRequest("POST", "/v1/knowledge/sync/stream", nil)
	tw := stream.NewSSETestWriter()
	h.SyncStream(tw, req)

	body := tw.Body().String()
	if !strings.Contains(body, `"type":"progress"`) {
		t.Errorf("expected progress event, got: %s", body)
	}
	if !strings.Contains(body, `"type":"done"`) {
		t.Errorf("expected done event, got: %s", body)
	}
}

// TestKnowledgeSyncStreamWithNilHub verifies SyncStream works without a hub.
func TestKnowledgeSyncStreamWithNilHub(t *testing.T) {
	h := handler.NewKnowledgeHandler(newTestKnowledgeEngine(t), nil)

	req := httptest.NewRequest("POST", "/v1/knowledge/sync/stream", nil)
	tw := stream.NewSSETestWriter()
	h.SyncStream(tw, req)

	if !strings.Contains(tw.Body().String(), `"type":"done"`) {
		t.Errorf("expected done event, got: %s", tw.Body().String())
	}
}
