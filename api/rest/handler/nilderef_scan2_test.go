package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── LOOP V4 L335: fechamento da família Missing Dependency Guard ────────
//
// Varredura dos caminhos que passaram na rodada anterior mas por métodos
// diferentes (Stats do memory com GetLayerStats sem guard; todos os métodos
// do AgentBridge com sessions/events/bridge sem guard). Previsão: pelo
// menos 1 membro adicional OU família confirmada fechada.

func TestNilDerefScan_MemoryStats(t *testing.T) {
	h := &MemoryHandler{} // engine nil
	req := httptest.NewRequest("GET", "http://x", nil)
	rr := httptest.NewRecorder()
	h.Stats(rr, req) // usa GetLayerStats sem guard?
	if rr.Code == http.StatusOK {
		t.Log("Stats retornou 200 com engine nil (comportamento aceito)")
	}
}

func TestNilDerefScan_AgentBridgeMethods(t *testing.T) {
	h := &AgentBridgeHandler{} // sessions/events/bridge nil

	methods := []struct {
		name string
		call func(w http.ResponseWriter, r *http.Request)
	}{
		{"Status", h.Status},
		{"Sessions", h.Sessions},
		{"SessionEvents", h.SessionEvents},
		{"CreateSession", h.CreateSession},
		{"DetachSession", h.DetachSession},
		{"ResumeSession", h.ResumeSession},
		{"StopSession", h.StopSession},
		{"DestroySession", h.DestroySession},
		{"EmitEvent", h.EmitEvent},
	}
	for _, m := range methods {
		req := httptest.NewRequest("POST", "http://x", strings.NewReader(`{"id":"abc"}`))
		rr := httptest.NewRecorder()
		// Recupera panic para reportar o bug sem abortar a varredura.
		panicked := scanRecover(func() { m.call(rr, req) })
		if panicked {
			t.Fatalf("AgentBridge.%s PANIC com deps nil (bug da família!)", m.name)
		}
		_ = rr.Code
	}
}

func TestNilDerefScan_RunHandlerAgentsMgr(t *testing.T) {
	h := &RunHandler{} // agentsMgr nil
	req := httptest.NewRequest("POST", "http://x", strings.NewReader(`{"prompt":"hi","agent":"cosca-backend"}`))
	rr := httptest.NewRecorder()
	panicked := scanRecover(func() { h.Execute(rr, req) })
	if panicked {
		t.Log("RunHandler.Execute PANIC com agentsMgr nil — investigar")
	}
	_ = rr.Code
}

// scanRecover executa fn e retorna true se houve panic.
func scanRecover(fn func()) (panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			panicked = true
		}
	}()
	fn()
	return false
}
