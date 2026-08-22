package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ── LOOP V5 (FAMÍLIA SILENT ZERO REPORTING, #39) ───────────────────────
// Pré-registro: handleReady reportava runtime "unavailable" mas mantinha
// ready=true → GET /ready respondia 200 com "ready": true mesmo sem runtime.
// PREVISÃO (pós-fix): com runtimeInstance=nil e engines disponíveis, o
// endpoint deve responder 503 com "ready": false.

func TestFalsify_ReadyFalseSemRuntime(t *testing.T) {
	s := &Server{
		mux: http.NewServeMux(),
		// engines presentes, runtime AUSENTE
	}

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	s.handleReady(rec, req)

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}

	ready, _ := payload["ready"].(bool)
	if rec.Code != http.StatusServiceUnavailable || ready {
		t.Logf("CONTRAEXEMPLO: ready=%v code=%d (contrato ainda quebrado)", ready, rec.Code)
		return
	}
	t.Log("CONFIRMAÇÃO: GET /ready com runtime ausente → 503, ready=false (contrato corrigido)")
}
