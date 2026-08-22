package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── LOOP V4 L334: caça ao irmão do bug (varredura de nil-deref) ─────────
//
// Heurística L333: "handlers HTTP com deps injetáveis sem garantia de
// inicialização têm alta bug_probability de nil-deref".
// Previsão: pelo menos 1 handler adicional vulnerável.
// Técnica: criar cada handler com ZERO-VALUE (deps nil) e chamar o endpoint
// com request vazio — se PANIC, é o mesmo padrão da família #17/#18.

func TestNilDerefScan_AuditHandler(t *testing.T) {
	h := &AuditHandler{} // store nil
	req := httptest.NewRequest("GET", "http://x", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req) // acessa h.store.List
	if rr.Code == http.StatusOK {
		t.Fatal("store nil não deve retornar 200")
	}
}

func TestNilDerefScan_ExecutionsHandler(t *testing.T) {
	h := &ExecutionsHandler{} // store nil
	req := httptest.NewRequest("GET", "http://x", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)
	if rr.Code == http.StatusOK {
		t.Fatal("store nil não deve retornar 200")
	}
	req2 := httptest.NewRequest("GET", "http://x", nil)
	rr2 := httptest.NewRecorder()
	h.Get(rr2, req2)
	if rr2.Code == http.StatusOK {
		t.Fatal("store nil não deve retornar 200")
	}
}

func TestNilDerefScan_APIKeysHandler(t *testing.T) {
	h := &APIKeysHandler{} // store nil
	req := httptest.NewRequest("GET", "http://x", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)
	if rr.Code == http.StatusOK {
		t.Fatal("store nil não deve retornar 200")
	}
}

func TestNilDerefScan_DepartmentsHandler(t *testing.T) {
	h := &DepartmentsHandler{} // store nil
	req := httptest.NewRequest("GET", "http://x", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)
	if rr.Code == http.StatusOK {
		t.Fatal("store nil não deve retornar 200")
	}
}

func TestNilDerefScan_PluginsHandler(t *testing.T) {
	h := &PluginsHandler{} // manager nil
	req := httptest.NewRequest("GET", "http://x", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)
	// Receiver nil-safe ou 503 — NUNCA panic.
	_ = rr.Code
}

func TestNilDerefScan_ProvidersHandler(t *testing.T) {
	h := &ProvidersHandler{} // mgr nil
	req := httptest.NewRequest("GET", "http://x", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)
	// Receiver nil-safe ou 503 — NUNCA panic.
	_ = rr.Code
}

func TestNilDerefScan_AgentsHandler(t *testing.T) {
	h := &AgentsHandler{} // mgr nil
	for _, path := range []string{"/", "/search"} {
		req := httptest.NewRequest("GET", "http://x"+path, nil)
		if path == "/search" {
			req = httptest.NewRequest("GET", "http://x?q=go", nil)
		}
		rr := httptest.NewRecorder()
		if path == "/search" {
			h.Search(rr, req)
		} else {
			h.List(rr, req)
		}
		// Receiver nil-safe ou 503 — NUNCA panic.
		_ = rr.Code
	}
	_ = strings.ToUpper
}
