package handler_test

// Tests for TracesHandler.Causal (traces.go) — the causal graph
// (causa → consequência) endpoint. Covers nodes+edges from a seeded trace,
// the unknown-trace 404 pt-BR case, divergence inclusion only when a baseline
// exists, nil-store 503, and route registration through the real mux.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CoscaAI/cosca/api/rest/handler"
)

// =============================================================================
// Causal
// =============================================================================

func TestTracesCausal_NodesAndEdges(t *testing.T) {
	s := newTestTraceStore(t)
	h := handler.NewTracesHandler(s)

	id := "TRACE-20260802-0ABCABCD"
	seedTrace(t, s, id, []string{"PLAN_CREATED", "TASK_STARTED", "TEST_FAILED"})

	req := httptest.NewRequest("GET", "/v1/traces/"+id+"/causal", nil)
	req.SetPathValue("id", id)
	w := httptest.NewRecorder()
	h.Causal(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		TraceID string `json:"trace_id"`
		Nodes   []struct {
			ID     string `json:"id"`
			Action string `json:"action"`
		} `json:"nodes"`
		Edges []struct {
			From string `json:"from"`
			To   string `json:"to"`
			Type string `json:"type"`
		} `json:"edges"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.TraceID != id {
		t.Errorf("trace_id = %q, want %q", resp.TraceID, id)
	}
	if len(resp.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(resp.Nodes))
	}
	if resp.Nodes[0].Action != "PLAN_CREATED" || resp.Nodes[2].Action != "TEST_FAILED" {
		t.Errorf("node actions mismatch: %+v", resp.Nodes)
	}
	if resp.Nodes[0].ID != "trace:"+id+":0" {
		t.Errorf("node ID = %q, want %q", resp.Nodes[0].ID, "trace:"+id+":0")
	}
	if len(resp.Edges) != 2 {
		t.Fatalf("expected 2 caused edges, got %d", len(resp.Edges))
	}
	for i, e := range resp.Edges {
		if e.Type != "caused" {
			t.Errorf("edge[%d].type = %q, want \"caused\"", i, e.Type)
		}
		if e.From != resp.Nodes[i].ID || e.To != resp.Nodes[i+1].ID {
			t.Errorf("edge[%d] = %s → %s, want %s → %s", i, e.From, e.To, resp.Nodes[i].ID, resp.Nodes[i+1].ID)
		}
	}
}

func TestTracesCausal_NoPriorTrace_OmitsDivergence(t *testing.T) {
	s := newTestTraceStore(t)
	h := handler.NewTracesHandler(s)

	id := "TRACE-20260802-0ABCABCD"
	seedTrace(t, s, id, []string{"PLAN_CREATED"})

	req := httptest.NewRequest("GET", "/v1/traces/"+id+"/causal", nil)
	req.SetPathValue("id", id)
	w := httptest.NewRecorder()
	h.Causal(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if string(w.Body.Bytes()) != "{\"trace_id\":\""+id+"\",\"nodes\":[{\"id\":\"trace:"+id+":0\",\"action\":\"PLAN_CREATED\",\"actor\":\"kernel\",\"result\":\"success\",\"details\":\"passo\"}],\"edges\":[]}\n" {
		t.Errorf("divergence should be omitted without a prior trace, got %s", w.Body.String())
	}
}

func TestTracesCausal_DivergenceIncluded(t *testing.T) {
	s := newTestTraceStore(t)
	h := handler.NewTracesHandler(s)

	// Prior trace: PLAN_CREATED, TASK_STARTED, TEST_FAILED.
	prior := "TRACE-20260802-0AAAAAAA"
	seedTrace(t, s, prior, []string{"PLAN_CREATED", "TASK_STARTED", "TEST_FAILED"})

	// Current trace diverges at position 2 (TASK_RUNNING instead of TEST_FAILED).
	current := "TRACE-20260802-0BBBBBBB"
	seedTrace(t, s, current, []string{"PLAN_CREATED", "TASK_STARTED", "TASK_RUNNING"})

	req := httptest.NewRequest("GET", "/v1/traces/"+current+"/causal", nil)
	req.SetPathValue("id", current)
	w := httptest.NewRecorder()
	h.Causal(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		TraceID string `json:"trace_id"`
		Nodes   []struct {
			ID string `json:"id"`
		} `json:"nodes"`
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
	if len(resp.Divergence.Positions) == 0 || resp.Divergence.Positions[0] != 2 {
		t.Errorf("first divergent position = %v, want [2]", resp.Divergence.Positions)
	}
	if resp.Divergence.Note == "" {
		t.Error("expected a pt-BR divergence note")
	}
	if len(resp.Nodes) != 3 {
		t.Errorf("expected 3 causal nodes, got %d", len(resp.Nodes))
	}
}

func TestTracesCausal_UnknownID(t *testing.T) {
	s := newTestTraceStore(t)
	h := handler.NewTracesHandler(s)

	req := httptest.NewRequest("GET", "/v1/traces/TRACE-20260802-FFFFFFFF/causal", nil)
	req.SetPathValue("id", "TRACE-20260802-FFFFFFFF")
	w := httptest.NewRecorder()
	h.Causal(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	if !jsonHasError(w.Body.Bytes()) {
		t.Errorf("expected pt-BR error message, got %s", w.Body.String())
	}
}

func TestTracesCausal_NilStore(t *testing.T) {
	h := handler.NewTracesHandler(nil)

	req := httptest.NewRequest("GET", "/v1/traces/TRACE-20260802-0ABCABCD/causal", nil)
	req.SetPathValue("id", "TRACE-20260802-0ABCABCD")
	w := httptest.NewRecorder()
	h.Causal(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
	if !jsonHasError(w.Body.Bytes()) {
		t.Errorf("expected pt-BR error message, got %s", w.Body.String())
	}
}

// =============================================================================
// Route registration (server.go wiring)
// =============================================================================

func TestTracesCausal_RouteRegistered(t *testing.T) {
	s := newTestTraceStore(t)
	mux := newTraceRouteServer(t, s)

	id := "TRACE-20260802-0ABCABCD"
	seedTrace(t, s, id, []string{"PLAN_CREATED", "TASK_STARTED", "TEST_FAILED"})

	req := httptest.NewRequest("GET", "/v1/traces/"+id+"/causal", nil)
	req = req.WithContext(editorClaimsContext())
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /v1/traces/%s/causal = %d, want 200: %s", id, w.Code, w.Body.String())
	}

	var resp struct {
		TraceID string `json:"trace_id"`
		Nodes   []struct {
			ID string `json:"id"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.TraceID != id || len(resp.Nodes) != 3 {
		t.Errorf("causal route response mismatch: trace_id=%q nodes=%d", resp.TraceID, len(resp.Nodes))
	}
}
