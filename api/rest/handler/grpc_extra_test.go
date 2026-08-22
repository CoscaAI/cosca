package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── ViaGRPC restantes: o guard de nil (bug #17 corrigido) agora PROVA 503 ─

func TestAllViaGRPCNoClientReturn503(t *testing.T) {
	kh := &KnowledgeHandler{}
	mh := &MemoryHandler{}

	// Knowledge ViaGRPC.
	kTests := []struct {
		name string
		body string
		call func(w http.ResponseWriter, r *http.Request)
	}{
		{"index", `{"path":"x.go","doc_type":"code"}`, kh.indexViaGRPC},
		{"stats", ``, func(w http.ResponseWriter, r *http.Request) { kh.statsViaGRPC(w) }},
		{"sync", `{}`, kh.syncViaGRPC},
	}
	for _, tc := range kTests {
		req := httptest.NewRequest("POST", "http://x", strings.NewReader(tc.body))
		rr := httptest.NewRecorder()
		tc.call(rr, req)
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("knowledge.%s sem client: status=%d, want 503", tc.name, rr.Code)
		}
	}

	// Memory ViaGRPC.
	mTests := []struct {
		name string
		body string
		call func(w http.ResponseWriter, r *http.Request)
	}{
		{"delete", `{"id":"m1"}`, mh.deleteViaGRPC},
		{"get", `{"id":"m1"}`, mh.getViaGRPC},
		{"promote", `{"id":"m1"}`, mh.promoteViaGRPC},
		{"search", `{"query":"x"}`, mh.searchViaGRPC},
	}
	for _, tc := range mTests {
		req := httptest.NewRequest("POST", "http://x", strings.NewReader(tc.body))
		rr := httptest.NewRecorder()
		tc.call(rr, req)
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("memory.%s sem client: status=%d, want 503", tc.name, rr.Code)
		}
	}
}

func TestGRPCAuthCtx(t *testing.T) {
	req := httptest.NewRequest("GET", "http://x", nil)
	ctx := grpcAuthCtx(req)
	if ctx == nil || ctx == context.Background() {
		t.Fatal("grpcAuthCtx deve retornar um contexto com metadata")
	}
}

// ── Pipeline handlers: paths de erro (sem state) ────────────────────────

func TestPipelineHandlersErrorPaths(t *testing.T) {
	ph := &PipelineHandler{}

	// ReconcileStalePlans sem history → erro (não é HTTP handler; retorna erro).
	_, err := ph.ReconcileStalePlans(context.Background())
	if err == nil {
		t.Fatal("ReconcileStalePlans sem history deve dar erro")
	}

	// GetAnalytics: sem plan_id → 400; com plan_id + sem history → 503.
	req2 := httptest.NewRequest("GET", "http://x", nil)
	rr2 := httptest.NewRecorder()
	ph.GetAnalytics(rr2, req2)
	if rr2.Code != http.StatusBadRequest {
		t.Fatalf("GetAnalytics sem plan: status=%d, want 400", rr2.Code)
	}
	req2b := httptest.NewRequest("GET", "http://x/analytics/p1", nil)
	req2b.SetPathValue("plan_id", "p1")
	rr2b := httptest.NewRecorder()
	ph.GetAnalytics(rr2b, req2b)
	if rr2b.Code != http.StatusServiceUnavailable {
		t.Fatalf("GetAnalytics sem history: status=%d, want 503", rr2b.Code)
	}

	// GetAggregateAnalytics sem history → 503.
	req3 := httptest.NewRequest("GET", "http://x", nil)
	rr3 := httptest.NewRecorder()
	ph.GetAggregateAnalytics(rr3, req3)
	if rr3.Code != http.StatusServiceUnavailable {
		t.Fatalf("GetAggregateAnalytics: status=%d, want 503", rr3.Code)
	}
}
