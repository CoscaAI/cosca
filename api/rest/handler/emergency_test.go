package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/api/rest/handler"
	"github.com/CoscaAI/cosca/internal/kernel"
)

// newTestEmergencyHandler creates an EmergencyHandler backed by a fresh
// EmergencyManager and a real audit store.
func newTestEmergencyHandler(t *testing.T) (*handler.EmergencyHandler, *kernel.EmergencyManager) {
	t.Helper()
	mgr := kernel.NewEmergencyManager()
	h := handler.NewEmergencyHandler(mgr, newTestAuditStore(t))
	return h, mgr
}

// TestEmergencyHandler_StatusNone verifies GET returns state "none" with
// empty halted_at and reason before any trigger.
func TestEmergencyHandler_StatusNone(t *testing.T) {
	h, _ := newTestEmergencyHandler(t)

	req := httptest.NewRequest("GET", "/v1/kernel/emergency", nil)
	w := httptest.NewRecorder()
	h.Status(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		State    string `json:"state"`
		HaltedAt string `json:"halted_at"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.State != string(kernel.EmergencyNone) {
		t.Errorf("state = %q, want %q", resp.State, kernel.EmergencyNone)
	}
	if resp.HaltedAt != "" {
		t.Errorf("halted_at = %q, want empty", resp.HaltedAt)
	}
	if resp.Reason != "" {
		t.Errorf("reason = %q, want empty", resp.Reason)
	}
}

// TestEmergencyHandler_StatusAfterStop verifies GET reflects a triggered stop.
func TestEmergencyHandler_StatusAfterStop(t *testing.T) {
	h, mgr := newTestEmergencyHandler(t)
	if err := mgr.TriggerStop("test reason"); err != nil {
		t.Fatalf("TriggerStop: %v", err)
	}

	req := httptest.NewRequest("GET", "/v1/kernel/emergency", nil)
	w := httptest.NewRecorder()
	h.Status(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		State    string `json:"state"`
		HaltedAt string `json:"halted_at"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.State != string(kernel.EmergencyStop) {
		t.Errorf("state = %q, want %q", resp.State, kernel.EmergencyStop)
	}
	if resp.HaltedAt == "" {
		t.Error("halted_at must be set after trigger")
	}
	if resp.Reason != "test reason" {
		t.Errorf("reason = %q, want %q", resp.Reason, "test reason")
	}
}

// TestEmergencyHandler_Stop verifies POST /v1/kernel/emergency/stop returns
// 202 Accepted and leaves the manager in the EmergencyStop state.
func TestEmergencyHandler_Stop(t *testing.T) {
	h, mgr := newTestEmergencyHandler(t)

	body := `{"reason":"daemon compromised"}`
	req := httptest.NewRequest("POST", "/v1/kernel/emergency/stop", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Stop(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		State   string `json:"state"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.State != string(kernel.EmergencyStop) {
		t.Errorf("state = %q, want %q", resp.State, kernel.EmergencyStop)
	}
	if resp.Message == "" {
		t.Error("expected non-empty message")
	}
	if got := mgr.State(); got != kernel.EmergencyStop {
		t.Errorf("manager state = %q, want %q", got, kernel.EmergencyStop)
	}
	if got := mgr.Reason(); got != "daemon compromised" {
		t.Errorf("manager reason = %q, want %q", got, "daemon compromised")
	}
}

// TestEmergencyHandler_StopDefaultReason verifies POST without a body uses
// the default reason and still triggers the stop.
func TestEmergencyHandler_StopDefaultReason(t *testing.T) {
	h, mgr := newTestEmergencyHandler(t)

	req := httptest.NewRequest("POST", "/v1/kernel/emergency/stop", nil)
	w := httptest.NewRecorder()
	h.Stop(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	if mgr.State() != kernel.EmergencyStop {
		t.Errorf("manager state = %q, want %q", mgr.State(), kernel.EmergencyStop)
	}
	if mgr.Reason() != "emergency stop requested" {
		t.Errorf("manager reason = %q, want default", mgr.Reason())
	}
}

// TestEmergencyHandler_StopMalformedBody verifies a malformed body never
// blocks the emergency stop — the default reason is used and 202 returned.
func TestEmergencyHandler_StopMalformedBody(t *testing.T) {
	h, mgr := newTestEmergencyHandler(t)

	req := httptest.NewRequest("POST", "/v1/kernel/emergency/stop", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Stop(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	if mgr.State() != kernel.EmergencyStop {
		t.Errorf("manager state = %q, want %q", mgr.State(), kernel.EmergencyStop)
	}
}

// TestEmergencyHandler_StopDuplicate verifies a second stop returns 409 with
// the current state.
func TestEmergencyHandler_StopDuplicate(t *testing.T) {
	h, _ := newTestEmergencyHandler(t)

	first := httptest.NewRequest("POST", "/v1/kernel/emergency/stop", nil)
	w1 := httptest.NewRecorder()
	h.Stop(w1, first)
	if w1.Code != http.StatusAccepted {
		t.Fatalf("first stop: expected 202, got %d", w1.Code)
	}

	second := httptest.NewRequest("POST", "/v1/kernel/emergency/stop", nil)
	w2 := httptest.NewRecorder()
	h.Stop(w2, second)

	if w2.Code != http.StatusConflict {
		t.Fatalf("second stop: expected 409, got %d: %s", w2.Code, w2.Body.String())
	}

	var resp struct {
		State   string `json:"state"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.State != string(kernel.EmergencyStop) {
		t.Errorf("state = %q, want %q", resp.State, kernel.EmergencyStop)
	}
}

// TestEmergencyHandler_Halt verifies POST /v1/kernel/emergency/halt returns
// 202 Accepted and leaves the manager in the EmergencyHalted state.
func TestEmergencyHandler_Halt(t *testing.T) {
	h, mgr := newTestEmergencyHandler(t)

	req := httptest.NewRequest("POST", "/v1/kernel/emergency/halt", nil)
	w := httptest.NewRecorder()
	h.Halt(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		State   string `json:"state"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.State != string(kernel.EmergencyHalted) {
		t.Errorf("state = %q, want %q", resp.State, kernel.EmergencyHalted)
	}
	if got := mgr.State(); got != kernel.EmergencyHalted {
		t.Errorf("manager state = %q, want %q", got, kernel.EmergencyHalted)
	}
	if got := mgr.Reason(); got != "halted" {
		t.Errorf("manager reason = %q, want %q", got, "halted")
	}
}

// TestEmergencyHandler_HaltEscalatesStop verifies halt escalates a prior
// plain stop to EmergencyHalted with 202 (not 409).
func TestEmergencyHandler_HaltEscalatesStop(t *testing.T) {
	h, mgr := newTestEmergencyHandler(t)

	stopReq := httptest.NewRequest("POST", "/v1/kernel/emergency/stop", nil)
	wStop := httptest.NewRecorder()
	h.Stop(wStop, stopReq)
	if wStop.Code != http.StatusAccepted {
		t.Fatalf("stop: expected 202, got %d", wStop.Code)
	}

	haltReq := httptest.NewRequest("POST", "/v1/kernel/emergency/halt", nil)
	wHalt := httptest.NewRecorder()
	h.Halt(wHalt, haltReq)

	if wHalt.Code != http.StatusAccepted {
		t.Fatalf("halt after stop: expected 202, got %d: %s", wHalt.Code, wHalt.Body.String())
	}
	if mgr.State() != kernel.EmergencyHalted {
		t.Errorf("manager state = %q, want %q (escalated)", mgr.State(), kernel.EmergencyHalted)
	}
}

// TestEmergencyHandler_NilManager verifies the handler responds 503 when the
// manager is nil.
func TestEmergencyHandler_NilManager(t *testing.T) {
	h := handler.NewEmergencyHandler(nil, nil)

	req := httptest.NewRequest("GET", "/v1/kernel/emergency", nil)
	w := httptest.NewRecorder()
	h.Status(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}
