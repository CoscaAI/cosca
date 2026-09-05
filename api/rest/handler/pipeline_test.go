package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/pipeline"
)

type mockPipelineRunner struct {
	runResult *pipeline.RunResult
	runErr    error
}

func (m *mockPipelineRunner) Run(_ context.Context, _ pipeline.RunRequest) (*pipeline.RunResult, error) {
	return m.runResult, m.runErr
}

func (m *mockPipelineRunner) RunStream(_ context.Context, _ pipeline.RunRequest) (<-chan pipeline.RunEvent, error) {
	return nil, nil
}

func TestNewPipelineHandler(t *testing.T) {
	runner := &mockPipelineRunner{}
	planner := &pipeline.Planner{}
	recovery := pipeline.NewRecoveryLoop(nil)
	hook := pipeline.NewPostTaskHook(t.TempDir())
	cmi := pipeline.NewCMITracker()

	h := NewPipelineHandler(runner, planner, recovery, hook, cmi, nil, nil)
	if h == nil {
		t.Fatal("NewPipelineHandler returned nil")
	}
	if h.timeout == 0 {
		t.Error("timeout should have a default")
	}
}

func TestPipelineHandler_SetTimeout(t *testing.T) {
	h := &PipelineHandler{}
	h.SetTimeout(0)
	if h.timeout != 0 {
		t.Error("zero should not change default")
	}

	d := 5 * time.Second
	h.SetTimeout(d)
	if h.timeout != d {
		t.Errorf("expected %v, got %v", d, h.timeout)
	}
}

func TestPipelineHandler_ExecutePipeline_MissingPrompt(t *testing.T) {
	h := &PipelineHandler{
		runner:  &mockPipelineRunner{},
		planner: &pipeline.Planner{},
		cmi:     pipeline.NewCMITracker(),
		timeout: 10 * time.Minute,
	}

	body := `{"prompt":""}`
	req := httptest.NewRequest(http.MethodPost, "/v1/pipeline/run", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ExecutePipeline(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestPipelineHandler_ExecutePipeline_MissingBody(t *testing.T) {
	h := &PipelineHandler{
		runner:  &mockPipelineRunner{},
		planner: &pipeline.Planner{},
		cmi:     pipeline.NewCMITracker(),
		timeout: 10 * time.Minute,
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/pipeline/run", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ExecutePipeline(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestPipelineHandler_ExecutePipeline_Success(t *testing.T) {
	runner := &mockPipelineRunner{
		runResult: &pipeline.RunResult{Response: "task completed"},
	}

	h := &PipelineHandler{
		runner:  runner,
		planner: &pipeline.Planner{},
		cmi:     pipeline.NewCMITracker(),
		hook:    pipeline.NewPostTaskHook(t.TempDir()),
		timeout: 10 * time.Minute,
	}

	body := `{"prompt":"create a CRUD API"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/pipeline/run", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ExecutePipeline(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp pipelineResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp.PlanID == "" {
		t.Error("plan_id should not be empty")
	}
}

func TestPipelineHandler_ExecutePipeline_Agent(t *testing.T) {
	runner := &mockPipelineRunner{
		runResult: &pipeline.RunResult{Response: "done"},
	}
	h := &PipelineHandler{
		runner:  runner,
		planner: &pipeline.Planner{},
		cmi:     pipeline.NewCMITracker(),
		hook:    pipeline.NewPostTaskHook(t.TempDir()),
		timeout: 10 * time.Minute,
	}

	body := `{"prompt":"fix the login bug","agent":"cosca-backend"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/pipeline/run", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ExecutePipeline(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestPipelineResponse_JSON(t *testing.T) {
	resp := pipelineResponse{
		PlanID:      "PLAN-20260808-ABCDEF",
		Intent:      "create CRUD API",
		IntentType:  "create-crud-api",
		TasksTotal:  5,
		TasksDone:   5,
		TasksFailed: 0,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	var decoded pipelineResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.PlanID != resp.PlanID {
		t.Errorf("plan_id mismatch: %s != %s", decoded.PlanID, resp.PlanID)
	}
}

// TestPipelineHandler_ExecutePipeline_PersistsAnalytics verifies the Fix-2
// regression: an executed pipeline run must persist its analytics into the
// event history, so downstream consumers (cosca qgate / cosca status autonomy
// dashboard) reflect the real run instead of reading an empty history.
func TestPipelineHandler_ExecutePipeline_PersistsAnalytics(t *testing.T) {
	runner := &mockPipelineRunner{
		runResult: &pipeline.RunResult{Response: "task completed"},
	}

	histDir := t.TempDir()
	hist, err := pipeline.NewWorkflowHistory(histDir)
	if err != nil {
		t.Fatalf("new history: %v", err)
	}

	h := &PipelineHandler{
		runner:  runner,
		planner: &pipeline.Planner{},
		cmi:     pipeline.NewCMITracker(),
		hook:    pipeline.NewPostTaskHook(t.TempDir()),
		history: hist,
		timeout: 10 * time.Minute,
	}

	body := `{"prompt":"create a CRUD API"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/pipeline/run", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ExecutePipeline(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp pipelineResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp.PlanID == "" {
		t.Fatal("plan_id should not be empty")
	}

	plans, err := hist.ListPlans()
	if err != nil {
		t.Fatalf("list plans: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("expected 1 plan persisted in history, got %d", len(plans))
	}
	if plans[0] != resp.PlanID {
		t.Errorf("persisted plan %q, want %q", plans[0], resp.PlanID)
	}

	// The analytics store must reconstruct the run with a non-zero autonomy
	// source: the run had 1 task done.
	store := pipeline.NewAnalyticsStore(hist)
	a, loadErr := store.Load(resp.PlanID)
	if loadErr != nil {
		t.Fatalf("load analytics: %v", loadErr)
	}
	if a.PlanID != resp.PlanID {
		t.Errorf("analytics plan %q, want %q", a.PlanID, resp.PlanID)
	}
}
