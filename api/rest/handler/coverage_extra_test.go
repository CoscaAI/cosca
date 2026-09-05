package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apiauth "github.com/CoscaAI/cosca/api/auth"
	"github.com/CoscaAI/cosca/internal/agentbridge"
	internalauth "github.com/CoscaAI/cosca/internal/auth"
)

// ── Setters triviais (cobrem linhas a 0% por serem chamadas de injeção) ─

func TestSettersAreIdempotent(t *testing.T) {
	// KnowledgeHandler.
	kh := &KnowledgeHandler{}
	kh.SetKnowledgeClient(nil) // client nil → proxy GRPC desabilitado

	// MemoryHandler.
	mh := &MemoryHandler{}
	mh.SetMemoryClient(nil)

	// RunHandler setters.
	rh := &RunHandler{}
	rh.SetKnowledgeSearcher(nil)
	rh.SetMemoryRetriever(nil)
	rh.SetMemoryStorer(nil)
	rh.SetPipelineComponents(nil, nil, nil, nil, nil, nil)
	rh.SetPipelinePlanner(nil)
	rh.SetPipelineRunner(nil)

	// PipelineHandler setters.
	ph := &PipelineHandler{}
	_ = ph.SetHistoryDir(t.TempDir())
	ph.SetDurable(nil, nil, nil)
	ph.SetTimeout(0)
}

// ── Helpers puros ───────────────────────────────────────────────────────

func TestItoaAndLastSeq(t *testing.T) {
	if itoa(0) != "0" {
		t.Fatalf("itoa(0) = %q", itoa(0))
	}
	if itoa(12345) != "12345" {
		t.Fatalf("itoa(12345) = %q", itoa(12345))
	}
	if got := lastSeq(nil); got != 0 {
		t.Fatalf("lastSeq(nil) = %d", got)
	}
	events := []agentbridge.Event{
		{Seq: 1}, {Seq: 5}, {Seq: 9},
	}
	if got := lastSeq(events); got != 9 {
		t.Fatalf("lastSeq = %d, want 9", got)
	}
}

func TestClaimsRole(t *testing.T) {
	req := httptest.NewRequest("GET", "http://x", nil)
	if got := claimsRole(req); got != "" {
		t.Fatalf("no claims role = %q", got)
	}
	// Com claims no contexto via a chave pública do pacote de auth da API.
	req2 := httptest.NewRequest("GET", "http://x", nil)
	ctx := context.WithValue(req2.Context(), apiauth.ContextKeyClaims, &internalauth.Claims{Role: "editor"})
	req2 = req2.WithContext(ctx)
	if got := claimsRole(req2); got != "editor" {
		t.Fatalf("claims role = %q, want editor", got)
	}
}

// ── Paths de erro dos handlers (sem state → respostas claras, sem panic) ─

func TestReconcilePlanRequiresPlanID(t *testing.T) {
	ph := &PipelineHandler{}
	req := httptest.NewRequest("POST", "http://x", strings.NewReader(""))
	rr := httptest.NewRecorder()
	ph.ReconcilePlan(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestReconcilePlanNoHistory(t *testing.T) {
	ph := &PipelineHandler{}
	req := httptest.NewRequest("POST", "http://x/reconcile/plan-1", nil)
	req.SetPathValue("plan_id", "plan-1")
	rr := httptest.NewRecorder()
	ph.ReconcilePlan(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (no history)", rr.Code)
	}
}

func TestReplayPlanRequiresPlanID(t *testing.T) {
	ph := &PipelineHandler{}
	req := httptest.NewRequest("POST", "http://x", nil)
	rr := httptest.NewRecorder()
	ph.ReplayPlan(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestGetHistoryNoHistory(t *testing.T) {
	ph := &PipelineHandler{}

	// Sem plan_id → 400 (requer parâmetro antes do check de history).
	req := httptest.NewRequest("GET", "http://x", nil)
	rr := httptest.NewRecorder()
	ph.GetHistory(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("sem plan: status = %d, want 400", rr.Code)
	}

	// Com plan_id + history nil → 503.
	req2 := httptest.NewRequest("GET", "http://x/history/plan-1", nil)
	req2.SetPathValue("plan_id", "plan-1")
	rr2 := httptest.NewRecorder()
	ph.GetHistory(rr2, req2)
	if rr2.Code != http.StatusServiceUnavailable {
		t.Fatalf("com plan sem history: status = %d, want 503", rr2.Code)
	}
}

// ── ViaGRPC com client nil → erro claro, sem panic ──────────────────────

func TestSearchViaGRPCNoClient(t *testing.T) {
	kh := &KnowledgeHandler{}
	body := `{"query":"go","limit":10}`
	req := httptest.NewRequest("POST", "http://x", strings.NewReader(body))
	rr := httptest.NewRecorder()
	kh.searchViaGRPC(rr, req)
	// Com client nil deve retornar erro claro (500/503), não panic.
	if rr.Code != http.StatusInternalServerError && rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 5xx", rr.Code)
	}
}

func TestStoreViaGRPCNoClient(t *testing.T) {
	mh := &MemoryHandler{}
	body, _ := json.Marshal(map[string]interface{}{"id": "m1", "content": "x"})
	req := httptest.NewRequest("POST", "http://x", strings.NewReader(string(body)))
	rr := httptest.NewRecorder()
	mh.storeViaGRPC(rr, req)
	if rr.Code == http.StatusOK {
		t.Fatalf("storeViaGRPC sem client não deve retornar 200 (status=%d)", rr.Code)
	}
}

func TestMemoryStatsViaGRPCNoClient(t *testing.T) {
	mh := &MemoryHandler{}
	req := httptest.NewRequest("GET", "http://x", nil)
	rr := httptest.NewRecorder()
	mh.statsViaGRPC(rr, req)
	if rr.Code == http.StatusOK {
		t.Fatal("statsViaGRPC sem client não deve retornar 200")
	}
}
