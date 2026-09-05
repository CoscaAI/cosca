package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/CoscaAI/cosca/internal/audit"
	"github.com/CoscaAI/cosca/internal/kernel"
)

// EmergencyHandler handles the Kernel kill-switch endpoints (admin only).
//
// The kill switch is the emergency mechanism that lets the Don stop the
// daemon remotely. Every trigger is recorded in the audit log with the
// acting user (from the request context) and the reason.
type EmergencyHandler struct {
	mgr        *kernel.EmergencyManager
	auditStore *audit.Store
}

// NewEmergencyHandler creates a new EmergencyHandler.
func NewEmergencyHandler(mgr *kernel.EmergencyManager, auditStore *audit.Store) *EmergencyHandler {
	return &EmergencyHandler{mgr: mgr, auditStore: auditStore}
}

// emergencyStopRequest is the expected JSON body for POST /v1/kernel/emergency/stop.
// The reason is optional; when omitted a default reason is used.
type emergencyStopRequest struct {
	Reason string `json:"reason"`
}

// Status handles GET /v1/kernel/emergency — returns the current kill-switch
// state, the halted timestamp (RFC3339, empty when never triggered), and the
// recorded reason.
func (h *EmergencyHandler) Status(w http.ResponseWriter, r *http.Request) {
	if h.mgr == nil {
		writeError(w, http.StatusServiceUnavailable, "emergency manager not available")
		return
	}

	haltedAt := ""
	if at := h.mgr.HaltedAt(); !at.IsZero() {
		haltedAt = at.Format(time.RFC3339)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"state":     string(h.mgr.State()),
		"halted_at": haltedAt,
		"reason":    h.mgr.Reason(),
	})
}

// Stop handles POST /v1/kernel/emergency/stop — pulls the kill switch and
// triggers the daemon shutdown. The reason is read from the request body,
// defaulting to "emergency stop requested".
//
// The request body is parsed leniently on purpose: a malformed body must
// never prevent an emergency stop from going through.
func (h *EmergencyHandler) Stop(w http.ResponseWriter, r *http.Request) {
	if h.mgr == nil {
		writeError(w, http.StatusServiceUnavailable, "emergency manager not available")
		return
	}

	reason := "emergency stop requested"
	var req emergencyStopRequest
	if r.Body != nil {
		limitBody(w, r, bodyLimitSmall)
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.Reason != "" {
			reason = req.Reason
		}
	}

	if err := h.mgr.TriggerStop(reason); err != nil {
		LogEvent(h.auditStore, r, "kernel.emergency_stop", "kernel",
			audit.DetailsJSON(map[string]string{"reason": reason, "error": err.Error()}), "error")
		writeJSON(w, http.StatusConflict, map[string]interface{}{
			"state":   string(h.mgr.State()),
			"message": "emergency already triggered",
		})
		return
	}

	LogEvent(h.auditStore, r, "kernel.emergency_stop", "kernel",
		audit.DetailsJSON(map[string]string{"reason": reason}), "success")

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"state":   string(h.mgr.State()),
		"message": "emergency stop requested",
	})
}

// Halt handles POST /v1/kernel/emergency/halt — the stronger kill switch.
// It triggers the same shutdown path but records the reserved reason
// "halted", which escalates the manager into the EmergencyHalted state
// (stopping everything). Even if a plain stop was already triggered, a
// halt escalates it.
func (h *EmergencyHandler) Halt(w http.ResponseWriter, r *http.Request) {
	if h.mgr == nil {
		writeError(w, http.StatusServiceUnavailable, "emergency manager not available")
		return
	}

	const reason = "halted"

	if err := h.mgr.TriggerStop(reason); err != nil {
		LogEvent(h.auditStore, r, "kernel.emergency_halt", "kernel",
			audit.DetailsJSON(map[string]string{"reason": reason, "error": err.Error()}), "error")
		writeJSON(w, http.StatusConflict, map[string]interface{}{
			"state":   string(h.mgr.State()),
			"message": "emergency already triggered",
		})
		return
	}

	LogEvent(h.auditStore, r, "kernel.emergency_halt", "kernel",
		audit.DetailsJSON(map[string]string{"reason": reason}), "success")

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"state":   string(h.mgr.State()),
		"message": "emergency halt requested",
	})
}
