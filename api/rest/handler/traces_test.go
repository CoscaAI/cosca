package handler_test

// Tests for TracesHandler (traces.go) — execution timeline + replay over the
// append-only trace flight recorder (internal/trace). Covers List/Get/Replay,
// the empty-store and unknown-trace (404 pt-BR) cases, divergence detection,
// and the server route registration (nil-safe wiring).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/api/rest"
	"github.com/CoscaAI/cosca/api/rest/handler"
	internalauth "github.com/CoscaAI/cosca/internal/auth"
	"github.com/CoscaAI/cosca/internal/trace"
)

// newTestTraceStore creates a temporary SQLite trace ledger for testing.
func newTestTraceStore(t *testing.T) *trace.Store {
	t.Helper()
	s, err := trace.NewStore(filepath.Join(t.TempDir(), "trace.db"))
	if err != nil {
		t.Fatalf("newTestTraceStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// seedTrace appends deterministic events (timestamp ASC) to the given store
// for a fixed trace ID and returns it.
func seedTrace(t *testing.T, s *trace.Store, id string, actions []string) {
	t.Helper()
	for i, action := range actions {
		ev := trace.Event{
			TraceID:   id,
			Actor:     "kernel",
			Action:    action,
			Timestamp: int64(1000 + i),
			Result:    "success",
			Details:   "passo",
		}
		if err := s.Append(ev); err != nil {
			t.Fatalf("seedTrace: append %s: %v", action, err)
		}
	}
}

// editorClaimsContext returns a context carrying editor JWT claims so the
// editorOnly-gated routes can be exercised through the real mux.
func editorClaimsContext() context.Context {
	return newClaimsContext(editorClaims())
}

// =============================================================================
// List
// =============================================================================

func TestTracesList_ReturnsSummaries(t *testing.T) {
	s := newTestTraceStore(t)
	h := handler.NewTracesHandler(s)

	idA := "TRACE-20260802-00010001"
	idB := "TRACE-20260802-00020002"
	seedTrace(t, s, idA, []string{"PLAN_CREATED", "TASK_STARTED"})
	seedTrace(t, s, idB, []string{"PLAN_CREATED", "TASK_STARTED", "TEST_FAILED"})

	req := httptest.NewRequest("GET", "/v1/traces", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Traces []struct {
			ID          string `json:"id"`
			Events      int    `json:"events"`
			FirstAction string `json:"first_action"`
			LastTS      int64  `json:"last_ts"`
		} `json:"traces"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Traces) != 2 {
		t.Fatalf("expected 2 trace summaries, got %d", len(resp.Traces))
	}

	// Most recent (idB) first — Latest orders by timestamp DESC.
	if resp.Traces[0].ID != idB || resp.Traces[0].Events != 3 || resp.Traces[0].FirstAction != "PLAN_CREATED" {
		t.Errorf("idB summary mismatch: %+v", resp.Traces[0])
	}
	if resp.Traces[1].ID != idA || resp.Traces[1].Events != 2 {
		t.Errorf("idA summary mismatch: %+v", resp.Traces[1])
	}
}

func TestTracesList_EmptyStore(t *testing.T) {
	s := newTestTraceStore(t)
	h := handler.NewTracesHandler(s)

	req := httptest.NewRequest("GET", "/v1/traces", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	// Empty store must yield {"traces":[]} — a real array, not null.
	if got := string(w.Body.Bytes()); got != "{\"traces\":[]}\n" {
		t.Errorf("empty store body = %q, want {\"traces\":[]}", got)
	}
}

func TestTracesList_NilStore(t *testing.T) {
	h := handler.NewTracesHandler(nil)

	req := httptest.NewRequest("GET", "/v1/traces", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
	if !jsonHasError(w.Body.Bytes()) {
		t.Errorf("expected pt-BR error message, got %s", w.Body.String())
	}
}

// =============================================================================
// Get
// =============================================================================

func TestTracesGet_FullTimeline(t *testing.T) {
	s := newTestTraceStore(t)
	h := handler.NewTracesHandler(s)

	id := "TRACE-20260802-0ABCABCD"
	seedTrace(t, s, id, []string{"PLAN_CREATED", "TASK_STARTED", "TEST_FAILED"})

	req := httptest.NewRequest("GET", "/v1/traces/"+id, nil)
	req.SetPathValue("id", id)
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		TraceID string `json:"trace_id"`
		Events  []struct {
			Action string `json:"action"`
			Actor  string `json:"actor"`
		} `json:"events"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.TraceID != id {
		t.Errorf("trace_id = %q, want %q", resp.TraceID, id)
	}
	if len(resp.Events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(resp.Events))
	}
	// Ordered by timestamp ASC — first action is PLAN_CREATED.
	if resp.Events[0].Action != "PLAN_CREATED" || resp.Events[2].Action != "TEST_FAILED" {
		t.Errorf("timeline order mismatch: %+v", resp.Events)
	}
	if resp.Events[0].Actor != "kernel" {
		t.Errorf("actor mismatch: %q", resp.Events[0].Actor)
	}
}

func TestTracesGet_UnknownID(t *testing.T) {
	s := newTestTraceStore(t)
	h := handler.NewTracesHandler(s)

	req := httptest.NewRequest("GET", "/v1/traces/TRACE-20260802-FFFFFFFF", nil)
	req.SetPathValue("id", "TRACE-20260802-FFFFFFFF")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	if !jsonHasError(w.Body.Bytes()) {
		t.Errorf("expected pt-BR error message, got %s", w.Body.String())
	}
}

func TestTracesGet_InvalidID(t *testing.T) {
	s := newTestTraceStore(t)
	h := handler.NewTracesHandler(s)

	req := httptest.NewRequest("GET", "/v1/traces/not-a-trace", nil)
	req.SetPathValue("id", "not-a-trace")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// =============================================================================
// Replay
// =============================================================================

func TestTracesReplay_OrderedSequence(t *testing.T) {
	s := newTestTraceStore(t)
	h := handler.NewTracesHandler(s)

	id := "TRACE-20260802-0ABCABCD"
	seedTrace(t, s, id, []string{"PLAN_CREATED", "TASK_STARTED", "TEST_FAILED"})

	req := httptest.NewRequest("GET", "/v1/traces/"+id+"/replay", nil)
	req.SetPathValue("id", id)
	w := httptest.NewRecorder()
	h.Replay(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		TraceID  string   `json:"trace_id"`
		Sequence []string `json:"sequence"`
		// Divergence intentionally absent in the struct — should be omitted
		// when there is no prior trace to compare.
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.TraceID != id {
		t.Errorf("trace_id = %q, want %q", resp.TraceID, id)
	}
	if len(resp.Sequence) != 3 {
		t.Fatalf("expected 3 sequence steps, got %d: %v", len(resp.Sequence), resp.Sequence)
	}
	want := []string{"PLAN_CREATED", "TASK_STARTED", "TEST_FAILED"}
	for i, a := range want {
		if resp.Sequence[i] != a {
			t.Errorf("sequence[%d] = %q, want %q", i, resp.Sequence[i], a)
		}
	}
}

func TestTracesReplay_NoPriorTrace_OmitsDivergence(t *testing.T) {
	s := newTestTraceStore(t)
	h := handler.NewTracesHandler(s)

	id := "TRACE-20260802-0ABCABCD"
	seedTrace(t, s, id, []string{"PLAN_CREATED"})

	req := httptest.NewRequest("GET", "/v1/traces/"+id+"/replay", nil)
	req.SetPathValue("id", id)
	w := httptest.NewRecorder()
	h.Replay(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if len(w.Body.Bytes()) > 0 && string(w.Body.Bytes())[:1] != "{" {
		t.Fatalf("expected JSON object, got %s", w.Body.String())
	}
	if string(w.Body.Bytes()) != "{\"trace_id\":\""+id+"\",\"sequence\":[\"PLAN_CREATED\"]}\n" {
		t.Errorf("divergence should be omitted without a prior trace, got %s", w.Body.String())
	}
}

func TestTracesReplay_DivergenceDetected(t *testing.T) {
	s := newTestTraceStore(t)
	h := handler.NewTracesHandler(s)

	// Prior trace: PLAN_CREATED, TASK_STARTED, TEST_FAILED.
	prior := "TRACE-20260802-0AAAAAAA"
	seedTrace(t, s, prior, []string{"PLAN_CREATED", "TASK_STARTED", "TEST_FAILED"})

	// Current trace diverges at position 2 (TASK_STARTED instead of TEST_PASSED... use TEST_RUNNING).
	current := "TRACE-20260802-0BBBBBBB"
	seedTrace(t, s, current, []string{"PLAN_CREATED", "TASK_STARTED", "TASK_RUNNING"})

	req := httptest.NewRequest("GET", "/v1/traces/"+current+"/replay", nil)
	req.SetPathValue("id", current)
	w := httptest.NewRecorder()
	h.Replay(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		TraceID    string   `json:"trace_id"`
		Sequence   []string `json:"sequence"`
		Divergence *struct {
			Positions []int  `json:"positions"`
			Note      string `json:"note"`
		} `json:"divergence"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Divergence == nil {
		t.Fatal("expected divergence info when a prior trace exists")
	}
	if len(resp.Divergence.Positions) == 0 {
		t.Fatalf("expected divergent positions, got %v", resp.Divergence.Positions)
	}
	if resp.Divergence.Positions[0] != 2 {
		t.Errorf("first divergent position = %d, want 2", resp.Divergence.Positions[0])
	}
	if resp.Divergence.Note == "" {
		t.Error("expected a pt-BR divergence note")
	}
}

func TestTracesReplay_UnknownID(t *testing.T) {
	s := newTestTraceStore(t)
	h := handler.NewTracesHandler(s)

	req := httptest.NewRequest("GET", "/v1/traces/TRACE-20260802-FFFFFFFF/replay", nil)
	req.SetPathValue("id", "TRACE-20260802-FFFFFFFF")
	w := httptest.NewRecorder()
	h.Replay(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	if !jsonHasError(w.Body.Bytes()) {
		t.Errorf("expected pt-BR error message, got %s", w.Body.String())
	}
}

// =============================================================================
// Route registration (server.go wiring)
// =============================================================================

// newTraceRouteServer builds a real REST server (rest.New) with a trace store
// and returns its mux, so the registered routes can be exercised end-to-end.
func newTraceRouteServer(t *testing.T, ts *trace.Store) *http.ServeMux {
	t.Helper()
	userStore := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	srv := rest.New(nil, nil, nil, nil, nil, nil, nil,
		userStore, nil, nil, rest.DefaultConfig(), nil, nil, nil, nil, nil,
		false, nil, ts, nil, nil)
	return srv.Mux()
}

func TestTraceRoutes_Registered(t *testing.T) {
	s := newTestTraceStore(t)
	mux := newTraceRouteServer(t, s)

	id := "TRACE-20260802-0ABCABCD"
	seedTrace(t, s, id, []string{"PLAN_CREATED", "TASK_STARTED"})

	cases := []struct {
		name   string
		path   string
		status int
	}{
		{"list", "/v1/traces", http.StatusOK},
		{"get", "/v1/traces/" + id, http.StatusOK},
		{"replay", "/v1/traces/" + id + "/replay", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			req = req.WithContext(editorClaimsContext())
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("GET %s = %d, want %d: %s", tc.path, w.Code, tc.status, w.Body.String())
			}
		})
	}
}

func TestTraceRoutes_NilStore_ServiceUnavailable(t *testing.T) {
	mux := newTraceRouteServer(t, nil)

	req := httptest.NewRequest("GET", "/v1/traces", nil)
	req = req.WithContext(editorClaimsContext())
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /v1/traces with nil store = %d, want 503: %s", w.Code, w.Body.String())
	}
	if !jsonHasError(w.Body.Bytes()) {
		t.Errorf("expected pt-BR error message, got %s", w.Body.String())
	}
}

// jsonHasError reports whether the body is a JSON object carrying a non-empty
// "error" field (the shape produced by writeError).
func jsonHasError(body []byte) bool {
	var resp map[string]string
	if err := json.Unmarshal(body, &resp); err != nil {
		return false
	}
	return resp["error"] != ""
}
