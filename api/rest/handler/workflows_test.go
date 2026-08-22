package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/CoscaAI/cosca/api/rest/handler"
	"github.com/CoscaAI/cosca/api/stream"
	"github.com/CoscaAI/cosca/internal/workflows"
)

// newTestWorkflowManager creates a workflow manager backed by a fresh temp
// directory (no user workflows) and registers a single deterministic
// 1-step workflow named "test-flow" so Run/RunStream stay fast (~10ms).
func newTestWorkflowManager(t *testing.T) *workflows.Manager {
	t.Helper()
	mgr := workflows.NewManager(t.TempDir())
	mgr.Add(workflows.Workflow{
		Name:        "test-flow",
		Description: "A deterministic workflow used by handler tests",
		Version:     "1.0.0",
		Status:      "active",
		Enabled:     true,
		Steps:       1,
		StepList: []workflows.Step{
			{Name: "step-1", Description: "first step", Agent: "general"},
		},
	})
	return mgr
}

// newTestWorkflowsHandler creates a WorkflowsHandler with a real manager.
func newTestWorkflowsHandler(t *testing.T) *handler.WorkflowsHandler {
	t.Helper()
	return handler.NewWorkflowsHandler(newTestWorkflowManager(t), nil)
}

// nonFlushWriter is an http.ResponseWriter that does NOT implement
// http.Flusher, used to exercise NewSSEWriter's failure path.
type nonFlushWriter struct {
	h      http.Header
	status int
	body   strings.Builder
}

func (w *nonFlushWriter) Header() http.Header         { return w.h }
func (w *nonFlushWriter) Write(b []byte) (int, error) { return w.body.Write(b) }
func (w *nonFlushWriter) WriteHeader(statusCode int)  { w.status = statusCode }

// TestWorkflowsNewHandler verifies handler construction with nil manager.
func TestWorkflowsNewHandler(t *testing.T) {
	h := handler.NewWorkflowsHandler(nil, nil)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}

// TestWorkflowsSetHub verifies SetHub accepts a nil hub without panicking.
func TestWorkflowsSetHub(t *testing.T) {
	h := handler.NewWorkflowsHandler(nil, nil)
	h.SetHub(nil)
}

// TestWorkflowsListNilMgr verifies List returns an empty array when the
// manager is nil.
func TestWorkflowsListNilMgr(t *testing.T) {
	h := handler.NewWorkflowsHandler(nil, nil)

	req := httptest.NewRequest("GET", "/v1/workflows", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if body := strings.TrimSpace(w.Body.String()); body != "[]" {
		t.Errorf("expected empty JSON array, got %s", body)
	}
}

// TestWorkflowsListWithWorkflows verifies List returns all registered
// workflows when a real manager is configured.
func TestWorkflowsListWithWorkflows(t *testing.T) {
	h := newTestWorkflowsHandler(t)

	req := httptest.NewRequest("GET", "/v1/workflows", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var items []workflows.Workflow
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected at least one workflow (embedded + test-flow)")
	}
	found := false
	for _, it := range items {
		if it.Name == "test-flow" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'test-flow' to be present in list response")
	}
}

// TestWorkflowsSearchMatch verifies Search returns matching workflows.
func TestWorkflowsSearchMatch(t *testing.T) {
	h := newTestWorkflowsHandler(t)

	req := httptest.NewRequest("GET", "/v1/workflows/search?q=test-flow", nil)
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var items []workflows.Workflow
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(items) != 1 || items[0].Name != "test-flow" {
		t.Errorf("expected exactly one match 'test-flow', got %+v", items)
	}
}

// TestWorkflowsSearchNoMatch verifies Search returns an empty array (not
// null) when no workflow matches.
func TestWorkflowsSearchNoMatch(t *testing.T) {
	h := newTestWorkflowsHandler(t)

	req := httptest.NewRequest("GET", "/v1/workflows/search?q=no-such-workflow-xyz", nil)
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if body := strings.TrimSpace(w.Body.String()); body != "[]" {
		t.Errorf("expected empty JSON array on no match, got %s", body)
	}
}

// TestWorkflowsSearchMissingQuery verifies Search returns 400 when the
// 'q' parameter is missing.
func TestWorkflowsSearchMissingQuery(t *testing.T) {
	h := newTestWorkflowsHandler(t)

	req := httptest.NewRequest("GET", "/v1/workflows/search", nil)
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestWorkflowsSearchNilMgr verifies Search returns an empty array when
// the manager is nil.
func TestWorkflowsSearchNilMgr(t *testing.T) {
	h := handler.NewWorkflowsHandler(nil, nil)

	req := httptest.NewRequest("GET", "/v1/workflows/search?q=anything", nil)
	w := httptest.NewRecorder()
	h.Search(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if body := strings.TrimSpace(w.Body.String()); body != "[]" {
		t.Errorf("expected empty JSON array, got %s", body)
	}
}

// TestWorkflowsGetFound verifies Get returns the requested workflow.
func TestWorkflowsGetFound(t *testing.T) {
	h := newTestWorkflowsHandler(t)

	req := httptest.NewRequest("GET", "/v1/workflows/test-flow", nil)
	req.SetPathValue("name", "test-flow")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var wf workflows.Workflow
	if err := json.Unmarshal(w.Body.Bytes(), &wf); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if wf.Name != "test-flow" {
		t.Errorf("expected name 'test-flow', got %q", wf.Name)
	}
}

// TestWorkflowsGetCaseInsensitive verifies Get matches case-insensitively.
func TestWorkflowsGetCaseInsensitive(t *testing.T) {
	h := newTestWorkflowsHandler(t)

	req := httptest.NewRequest("GET", "/v1/workflows/TEST-FLOW", nil)
	req.SetPathValue("name", "TEST-FLOW")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for case-insensitive lookup, got %d: %s", w.Code, w.Body.String())
	}
}

// TestWorkflowsGetNotFound verifies Get returns 404 for an unknown workflow.
func TestWorkflowsGetNotFound(t *testing.T) {
	h := newTestWorkflowsHandler(t)

	req := httptest.NewRequest("GET", "/v1/workflows/nope", nil)
	req.SetPathValue("name", "nope")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestWorkflowsGetNilMgr verifies Get returns 503 when the manager is nil.
func TestWorkflowsGetNilMgr(t *testing.T) {
	h := handler.NewWorkflowsHandler(nil, nil)

	req := httptest.NewRequest("GET", "/v1/workflows/test-flow", nil)
	req.SetPathValue("name", "test-flow")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
}

// TestWorkflowsRunSuccess verifies Run executes a workflow and returns a
// completed Result.
func TestWorkflowsRunSuccess(t *testing.T) {
	h := newTestWorkflowsHandler(t)

	req := httptest.NewRequest("POST", "/v1/workflows/test-flow/run", nil)
	req.SetPathValue("name", "test-flow")
	w := httptest.NewRecorder()
	h.Run(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var res workflows.Result
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if res.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", res.Status)
	}
	if res.TotalSteps != 1 || res.StepsCompleted != 1 {
		t.Errorf("expected 1/1 steps completed, got %d/%d", res.StepsCompleted, res.TotalSteps)
	}
}

// TestWorkflowsRunNotFound verifies Run returns 500 when the workflow does
// not exist.
func TestWorkflowsRunNotFound(t *testing.T) {
	h := newTestWorkflowsHandler(t)

	req := httptest.NewRequest("POST", "/v1/workflows/missing/run", nil)
	req.SetPathValue("name", "missing")
	w := httptest.NewRecorder()
	h.Run(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// TestWorkflowsRunNilMgr verifies Run returns 503 when the manager is nil.
func TestWorkflowsRunNilMgr(t *testing.T) {
	h := handler.NewWorkflowsHandler(nil, nil)

	req := httptest.NewRequest("POST", "/v1/workflows/test-flow/run", nil)
	req.SetPathValue("name", "test-flow")
	w := httptest.NewRecorder()
	h.Run(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
}

// TestWorkflowsRunStreamSuccess verifies RunStream emits step and done SSE
// events and broadcasts to the hub.
func TestWorkflowsRunStreamSuccess(t *testing.T) {
	hub := stream.NewHub(zerolog.Nop())
	t.Cleanup(func() { _ = hub.Shutdown(context.Background()) })

	h := newTestWorkflowsHandler(t)
	h.SetHub(hub)

	req := httptest.NewRequest("POST", "/v1/workflows/test-flow/run/stream", nil)
	req.SetPathValue("name", "test-flow")
	tw := stream.NewSSETestWriter()
	h.RunStream(tw, req)

	body := tw.Body().String()
	if !strings.Contains(body, `"type":"step"`) {
		t.Errorf("expected step event, got: %s", body)
	}
	if !strings.Contains(body, `"type":"done"`) {
		t.Errorf("expected done event, got: %s", body)
	}
}

// TestWorkflowsRunStreamError verifies RunStream writes an in-band error
// SSE event when the workflow does not exist.
func TestWorkflowsRunStreamError(t *testing.T) {
	h := newTestWorkflowsHandler(t)

	req := httptest.NewRequest("POST", "/v1/workflows/missing/run/stream", nil)
	req.SetPathValue("name", "missing")
	tw := stream.NewSSETestWriter()
	h.RunStream(tw, req)

	body := tw.Body().String()
	if !strings.Contains(body, `"type":"error"`) {
		t.Errorf("expected error event, got: %s", body)
	}
}

// TestWorkflowsRunStreamNilHub verifies RunStream works without a hub set.
func TestWorkflowsRunStreamNilHub(t *testing.T) {
	h := newTestWorkflowsHandler(t)

	req := httptest.NewRequest("POST", "/v1/workflows/test-flow/run/stream", nil)
	req.SetPathValue("name", "test-flow")
	tw := stream.NewSSETestWriter()
	h.RunStream(tw, req)

	body := tw.Body().String()
	if !strings.Contains(body, `"type":"step"`) {
		t.Errorf("expected step event, got: %s", body)
	}
	if !strings.Contains(body, `"type":"done"`) {
		t.Errorf("expected done event, got: %s", body)
	}
}

// TestWorkflowsRunStreamWithInput verifies RunStream accepts a JSON body
// with optional input.
func TestWorkflowsRunStreamWithInput(t *testing.T) {
	h := newTestWorkflowsHandler(t)

	body := `{"input":"hello","count":3}`
	req := httptest.NewRequest("POST", "/v1/workflows/test-flow/run/stream", strings.NewReader(body))
	req.SetPathValue("name", "test-flow")
	tw := stream.NewSSETestWriter()
	h.RunStream(tw, req)

	if !strings.Contains(tw.Body().String(), `"type":"done"`) {
		t.Errorf("expected done event, got: %s", tw.Body().String())
	}
}

// TestWorkflowsRunStreamEmptyName verifies RunStream returns 400 when the
// workflow name is empty.
func TestWorkflowsRunStreamEmptyName(t *testing.T) {
	h := newTestWorkflowsHandler(t)

	req := httptest.NewRequest("POST", "/v1/workflows//run/stream", nil)
	req.SetPathValue("name", "")
	w := httptest.NewRecorder()
	h.RunStream(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestWorkflowsRunStreamNilMgr verifies RunStream returns 503 when the
// manager is nil.
func TestWorkflowsRunStreamNilMgr(t *testing.T) {
	h := handler.NewWorkflowsHandler(nil, nil)

	req := httptest.NewRequest("POST", "/v1/workflows/test-flow/run/stream", nil)
	req.SetPathValue("name", "test-flow")
	w := httptest.NewRecorder()
	h.RunStream(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
}

// TestWorkflowsRunStreamWriterError verifies RunStream returns 500 when the
// ResponseWriter does not support flushing (no http.Flusher).
func TestWorkflowsRunStreamWriterError(t *testing.T) {
	h := newTestWorkflowsHandler(t)

	req := httptest.NewRequest("POST", "/v1/workflows/test-flow/run/stream", nil)
	req.SetPathValue("name", "test-flow")
	w := &nonFlushWriter{h: make(http.Header)}
	h.RunStream(w, req)

	if w.status != http.StatusInternalServerError {
		t.Fatalf("expected 500 for non-flushing writer, got %d: %s", w.status, w.body.String())
	}
}
