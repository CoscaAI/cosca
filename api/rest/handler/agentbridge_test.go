package handler

import (
	"github.com/CoscaAI/cosca/internal/agentbridge"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── LOOP V4 L333: error paths acessíveis do api/rest ───────────────────
//
// Hipótese: os error paths de agentbridge/run.go são acessíveis via chamada
// direta (request vazio → 400/404) — alta alavancagem por teste.
// Previsão: ≥0,1% coverage + ≥1 contrato observado.

func TestAgentBridgeErrorPaths(t *testing.T) {
	h := &AgentBridgeHandler{}

	// Sem id → 400.
	req := httptest.NewRequest("GET", "http://x", nil)
	rr := httptest.NewRecorder()
	h.SessionEventsStream(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("sem id: status=%d, want 400", rr.Code)
	}

	// Com id + sessions nil → deve dar erro (404/500), nunca panic.
	req2 := httptest.NewRequest("GET", "http://x/stream/abc", nil)
	req2.SetPathValue("id", "abc")
	rr2 := httptest.NewRecorder()
	h.SessionEventsStream(rr2, req2)
	if rr2.Code == http.StatusOK {
		t.Fatal("sem sessions não deve retornar 200")
	}
}

func TestWriteSSEFormat(t *testing.T) {
	rr := httptest.NewRecorder()
	writeSSE(rr, agentbridge.Event{Seq: 7, Type: "test"})
	out := rr.Body.String()
	if !strings.Contains(out, "id: 7") || !strings.Contains(out, "data: ") {
		t.Fatalf("formato SSE inválido: %q", out)
	}
	if !strings.Contains(out, "\n\n") {
		t.Fatal("SSE deve terminar com linha em branco")
	}
}

func TestRunHandlerExecuteWithPipelineNoProvider(t *testing.T) {
	rh := &RunHandler{}
	req := httptest.NewRequest("POST", "http://x", strings.NewReader(`{"prompt":"hi"}`))
	rr := httptest.NewRecorder()
	// Sem provider configurado → 503, nunca panic.
	rh.executeWithPipeline(rr, req, runRequest{Prompt: "hi"})
	if rr.Code != http.StatusServiceUnavailable && rr.Code != http.StatusInternalServerError {
		t.Fatalf("sem provider: status=%d, want 5xx", rr.Code)
	}
}
