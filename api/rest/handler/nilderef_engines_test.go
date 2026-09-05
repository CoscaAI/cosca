package handler

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// ── LOOP V4 L334 (continuação): varredura de nil-deref nos engines ──────
//
// Os handlers de knowledge/memory usam h.engine (injetável) — o mesmo padrão
// da família. Testar com engine nil: NUNCA panic.

func TestNilDerefScan_KnowledgeEngine(t *testing.T) {
	h := &KnowledgeHandler{} // engine nil
	req := httptest.NewRequest("POST", "http://x", strings.NewReader(`{"query":"go"}`))
	rr := httptest.NewRecorder()
	h.Search(rr, req)
	_ = rr.Code

	req2 := httptest.NewRequest("GET", "http://x", nil)
	rr2 := httptest.NewRecorder()
	h.Sync(rr2, req2)
	_ = rr2.Code
}

func TestNilDerefScan_MemoryEngine(t *testing.T) {
	h := &MemoryHandler{} // engine nil
	req := httptest.NewRequest("POST", "http://x", strings.NewReader(`{"id":"m1","content":"x"}`))
	rr := httptest.NewRecorder()
	h.Store(rr, req)
	_ = rr.Code

	req2 := httptest.NewRequest("GET", "http://x?query=x", nil)
	rr2 := httptest.NewRecorder()
	h.Search(rr2, req2)
	_ = rr2.Code

	req3 := httptest.NewRequest("DELETE", "http://x", strings.NewReader(`{"id":"m1"}`))
	rr3 := httptest.NewRecorder()
	h.Delete(rr3, req3)
	_ = rr3.Code
}
