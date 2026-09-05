package handler_test

// Smoke tests para os contratos de leitura do painel "Casa Visível": os
// endpoints Shadow (metrologia ADR-033) e Decision Trace (causalidade
// operacional ADR-032/033), usando o DefaultStore injetado (temp) e o trace
// flight recorder (temp). Cobre Summary/Histogram/Decisions/Get, List/Stats/Get
// (por request_id e por TraceID) e o registro das rotas no server.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/api/rest"
	"github.com/CoscaAI/cosca/api/rest/handler"
	internalauth "github.com/CoscaAI/cosca/internal/auth"
	"github.com/CoscaAI/cosca/internal/shadow"
	"github.com/CoscaAI/cosca/internal/trace"
)

// newTestShadowStore cria um ShadowStore temporário e o seed com observações
// determinísticas (RequestID estáveis + At ascendente por eleição).
func newTestShadowStore(t *testing.T) *shadow.Store {
	t.Helper()
	s := shadow.NewStore(t.TempDir())

	base := time.Unix(1754188800, 0).UTC()
	seed := []shadow.ShadowTrace{
		{
			RequestID:     "req-1",
			Agent:         "cosca-backend",
			Decision:      shadow.EMIT_OK,
			WouldEscalate: false, WouldRespond: "resposta determinística",
			Confidence: 0.82, Convergence: 0.79, EvidenceIDs: []string{"ev-a", "ev-b"},
			Positions: 2, Reason: "converged; kernel would answer without LLM",
			Breakdown: "base=0.7900 final=0.8200", DurationMs: 120, At: base,
		},
		{
			RequestID:     "req-2",
			Agent:         "cosca-security",
			Decision:      shadow.ESCALATE,
			WouldEscalate: true,
			Confidence:    0.44, Convergence: 0.38, EvidenceIDs: []string{"ev-conflict", "ev-risk"},
			Positions: 1, Reason: "conflicting evidence",
			Breakdown: "base=0.3800 -0.10=0.2800 ... final=0.4400", DurationMs: 95, At: base.Add(time.Minute),
		},
		{
			RequestID:     "req-3",
			Agent:         "cosca-kernel",
			Decision:      shadow.RETRIEVAL_INSUFFICIENT,
			WouldEscalate: true,
			Confidence:    0.20, Convergence: 0.00,
			Positions: 0, Reason: "no substantiated evidence (retrieval insufficient)",
			DurationMs: 60, At: base.Add(2 * time.Minute),
		},
	}

	for _, st := range seed {
		if err := s.Append(st); err != nil {
			t.Fatalf("seed shadow: %v", err)
		}
	}
	return s
}

// newTestDecisionFullTrace cria um trace.Store temporário e o seed com uma
// cadeia de eventos para um TraceID único (para o GET /v1/decisions/{traceID}).
func newTestDecisionFullTrace(t *testing.T) (*trace.Store, trace.TraceID) {
	t.Helper()
	s, err := trace.NewStore(filepath.Join(t.TempDir(), "trace.db"))
	if err != nil {
		t.Fatalf("newTestTraceStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	tid := trace.NewID().String()
	evs := []trace.Event{
		{TraceID: tid, Timestamp: 1754188800, Actor: "kernel", Action: "CONTEXT_ASSEMBLED", Result: "success"},
		{TraceID: tid, Timestamp: 1754188801, Actor: "kernel", Action: "DELIBERATED", Result: "success", Details: "convergence=0.79"},
		{TraceID: tid, Timestamp: 1754188802, Actor: "agent", Action: "AGENT_EXECUTED", Result: "success"},
		{TraceID: tid, Timestamp: 1754188803, Actor: "agent", Action: "TOOL_RESULT", Result: "failed", Details: "test failed"},
		{TraceID: tid, Timestamp: 1754188804, Actor: "agent", Action: "EXECUTION_DONE", Result: "success"},
	}
	for i := range evs {
		if err := s.Append(evs[i]); err != nil {
			t.Fatalf("seed trace: %v", err)
		}
	}
	return s, trace.TraceID(tid)
}

// =============================================================================
// ShadowHandler — Summary / Histogram / Decisions / Get
// =============================================================================

func TestShadowSummary_ReturnsAggregate(t *testing.T) {
	h := handler.NewShadowHandler(newTestShadowStore(t))

	req := httptest.NewRequest("GET", "/v1/shadow/summary", nil)
	w := httptest.NewRecorder()
	h.Summary(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Total           int            `json:"total"`
		Decisions       map[string]int `json:"decisions"`
		EscalationRate  float64        `json:"escalation_rate"`
		SelfResolveRate float64        `json:"self_resolve_rate"`
		AvgConfidence   float64        `json:"avg_confidence"`
		AvgConvergence  float64        `json:"avg_convergence"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Total != 3 {
		t.Errorf("total = %d, want 3", resp.Total)
	}
	if resp.Decisions[string(shadow.EMIT_OK)] != 1 {
		t.Errorf("EMIT_OK count = %d, want 1", resp.Decisions[string(shadow.EMIT_OK)])
	}
	if resp.SelfResolveRate <= 0 || resp.SelfResolveRate >= 1 {
		t.Errorf("self_resolve_rate = %f, want in (0,1)", resp.SelfResolveRate)
	}
	if resp.AvgConfidence == 0 {
		t.Error("avg_confidence must be non-zero")
	}
}

func TestShadowHistogram_ReturnsBuckets(t *testing.T) {
	h := handler.NewShadowHandler(newTestShadowStore(t))

	req := httptest.NewRequest("GET", "/v1/shadow/histogram", nil)
	w := httptest.NewRecorder()
	h.Histogram(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Histogram []struct {
			Low   float64 `json:"low"`
			High  float64 `json:"high"`
			Count int     `json:"count"`
		} `json:"histogram"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Histogram) != 6 {
		t.Errorf("histogram buckets = %d, want 6", len(resp.Histogram))
	}
	if resp.Total != 3 {
		t.Errorf("total = %d, want 3", resp.Total)
	}
}

func TestShadowDecisions_ListsMostRecentFirst(t *testing.T) {
	h := handler.NewShadowHandler(newTestShadowStore(t))

	req := httptest.NewRequest("GET", "/v1/shadow/decisions?limit=2", nil)
	w := httptest.NewRecorder()
	h.Decisions(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Observations []shadow.ShadowTrace `json:"observations"`
		Total        int                  `json:"total"`
		Limit        int                  `json:"limit"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Total != 3 {
		t.Errorf("total = %d, want 3", resp.Total)
	}
	if len(resp.Observations) != 2 {
		t.Fatalf("limit=2 -> %d observations, want 2", len(resp.Observations))
	}
	// Most recent first: req-3 (At = base+2m), then req-2.
	if resp.Observations[0].RequestID != "req-3" {
		t.Errorf("most recent = %q, want req-3", resp.Observations[0].RequestID)
	}
	if resp.Observations[1].RequestID != "req-2" {
		t.Errorf("second = %q, want req-2", resp.Observations[1].RequestID)
	}
}

func TestShadowGet_ReturnsSingleObservation(t *testing.T) {
	h := handler.NewShadowHandler(newTestShadowStore(t))

	req := httptest.NewRequest("GET", "/v1/shadow/decisions/req-1", nil)
	req.SetPathValue("id", "req-1")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		RequestID   string             `json:"request_id"`
		Observation shadow.ShadowTrace `json:"observation"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Observation.RequestID != "req-1" {
		t.Errorf("observation.request_id = %q, want req-1", resp.Observation.RequestID)
	}
	if resp.Observation.Decision != shadow.EMIT_OK {
		t.Errorf("decision = %q, want EMIT_OK", resp.Observation.Decision)
	}
}

func TestShadowGet_Unknown(t *testing.T) {
	h := handler.NewShadowHandler(newTestShadowStore(t))

	req := httptest.NewRequest("GET", "/v1/shadow/decisions/nope", nil)
	req.SetPathValue("id", "nope")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	if !jsonHasError(w.Body.Bytes()) {
		t.Errorf("expected pt-BR error message, got %s", w.Body.String())
	}
}

// =============================================================================
// DecisionHandler — List / Get (por request_id e por TraceID) / Stats
// =============================================================================

func TestDecisionList_ReturnsDecisions(t *testing.T) {
	h := handler.NewDecisionHandler(newTestShadowStore(t), nil)

	req := httptest.NewRequest("GET", "/v1/decisions", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Decisions []map[string]interface{} `json:"decisions"`
		Total     int                      `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Total != 3 {
		t.Errorf("total = %d, want 3", resp.Total)
	}
	if len(resp.Decisions) != 3 {
		t.Fatalf("decisions = %d, want 3", len(resp.Decisions))
	}
	first := resp.Decisions[0]
	if first["request_id"] != "req-3" {
		t.Errorf("first decision.request_id = %v, want req-3", first["request_id"])
	}
}

func TestDecisionGet_ByRequestID(t *testing.T) {
	h := handler.NewDecisionHandler(newTestShadowStore(t), nil)

	req := httptest.NewRequest("GET", "/v1/decisions/req-2", nil)
	req.SetPathValue("id", "req-2")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		ResolvedAs   string `json:"resolved_as"`
		Decision     string `json:"decision"`
		Confidence   float64
		Deliberation struct {
			Verdict  string `json:"verdict"`
			Conflicts bool  `json:"conflicts"`
		} `json:"deliberation"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.ResolvedAs != "shadow" {
		t.Errorf("resolved_as = %q, want shadow", resp.ResolvedAs)
	}
	if resp.Decision != string(shadow.ESCALATE) {
		t.Errorf("decision = %q, want ESCALATE", resp.Decision)
	}
	if resp.Deliberation.Verdict != "escalate" {
		t.Errorf("deliberation.verdict = %q, want escalate", resp.Deliberation.Verdict)
	}
	if !resp.Deliberation.Conflicts {
		t.Error("deliberation.conflicts should be true for conflicting evidence")
	}
}

func TestDecisionGet_ByTraceID(t *testing.T) {
	ts, tid := newTestDecisionFullTrace(t)
	h := handler.NewDecisionHandler(newTestShadowStore(t), ts)

	req := httptest.NewRequest("GET", "/v1/decisions/"+tid.String(), nil)
	req.SetPathValue("id", tid.String())
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		ResolvedAs string `json:"resolved_as"`
		TraceID    string `json:"trace_id"`
		Outcome    string `json:"outcome"`
		Sequence   []string
		Causal     struct {
			Nodes []trace.CausalNode `json:"nodes"`
			Edges []trace.CausalEdge `json:"edges"`
		} `json:"causal"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.ResolvedAs != "trace" {
		t.Errorf("resolved_as = %q, want trace", resp.ResolvedAs)
	}
	if resp.TraceID != tid.String() {
		t.Errorf("trace_id = %q, want %q", resp.TraceID, tid.String())
	}
	if resp.Outcome != "success" {
		t.Errorf("outcome = %q, want success (last event result)", resp.Outcome)
	}
	if len(resp.Sequence) != 5 {
		t.Errorf("sequence = %d actions, want 5", len(resp.Sequence))
	}
	if len(resp.Causal.Nodes) != 5 || len(resp.Causal.Edges) != 4 {
		t.Errorf("causal graph = %d nodes / %d edges, want 5/4", len(resp.Causal.Nodes), len(resp.Causal.Edges))
	}
}

func TestDecisionGet_Unknown(t *testing.T) {
	h := handler.NewDecisionHandler(newTestShadowStore(t), nil)

	req := httptest.NewRequest("GET", "/v1/decisions/ghost", nil)
	req.SetPathValue("id", "ghost")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeliberationStats_ReturnsAggregate(t *testing.T) {
	h := handler.NewDecisionHandler(newTestShadowStore(t), nil)

	req := httptest.NewRequest("GET", "/v1/deliberation/stats", nil)
	w := httptest.NewRecorder()
	h.Stats(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Total             int              `json:"total"`
		AvgConfidence     float64          `json:"avg_confidence"`
		AvgConvergence    float64          `json:"avg_convergence"`
		EscalationRate    float64          `json:"escalation_rate"`
		SelfResolveRate   float64          `json:"self_resolve_rate"`
		Decisions         map[string]int   `json:"decisions"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Total != 3 {
		t.Errorf("total = %d, want 3", resp.Total)
	}
	if resp.AvgConfidence == 0 || resp.AvgConvergence == 0 {
		t.Error("avg fields must be non-zero")
	}
}

// =============================================================================
// Route registration (server.go wiring)
// =============================================================================

func TestShadowRoutes_Registered(t *testing.T) {
	// A new REST server (nil in-memory engines, valid auth store) registers the
	// Shadow/Decision routes. The shadow routes are read-only (HandleFunc, no
	// role gate); /v1/decisions/{id} is editor+.
	userStore := internalauth.NewUserStore(internalauth.UserStoreConfig{})
	srv := rest.New(nil, nil, nil, nil, nil, nil, nil,
		userStore, nil, nil, rest.DefaultConfig(), nil, nil, nil, nil, nil,
		false, nil, nil, nil, nil)
	mux := srv.Mux()

	t.Run("shadow summary registered", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/shadow/summary", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET /v1/shadow/summary = %d, want 200: %s", w.Code, w.Body.String())
		}
	})

	t.Run("shadow histogram registered", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/shadow/histogram", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET /v1/shadow/histogram = %d, want 200: %s", w.Code, w.Body.String())
		}
	})

	t.Run("decisions list registered", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/decisions", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET /v1/decisions = %d, want 200: %s", w.Code, w.Body.String())
		}
	})

	t.Run("deliberation stats registered", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/deliberation/stats", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET /v1/deliberation/stats = %d, want 200: %s", w.Code, w.Body.String())
		}
	})
}
