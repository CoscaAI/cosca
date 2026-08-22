package handler

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	apiauth "github.com/CoscaAI/cosca/api/auth"
	"github.com/CoscaAI/cosca/internal/audit"
)

// AuditHandler handles REST API requests for the audit log system (admin only).
type AuditHandler struct {
	store *audit.Store
}

// NewAuditHandler creates a new AuditHandler.
func NewAuditHandler(store *audit.Store) *AuditHandler {
	return &AuditHandler{store: store}
}

// List handles GET /v1/audit/logs — returns a paginated, filterable list of
// audit entries.
//
// Query parameters:
//
//	limit    — max records per page (default: 20, max: 1000)
//	offset   — records to skip (default: 0)
//	user_id  — filter by user ID
//	action   — filter by action
//	resource — filter by resource
//	status   — filter by status (success, denied, error)
//	from     — filter entries after this time (RFC3339 or Unix timestamp)
//	to       — filter entries before this time (RFC3339 or Unix timestamp)
//	format   — "csv" for CSV export instead of JSON
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "audit store not available")
		return
	}

	q := r.URL.Query()

	limit := parseIntParam(q.Get("limit"), 20)
	if limit > 1000 {
		limit = 1000
	}
	offset := parseIntParam(q.Get("offset"), 0)

	filters := audit.AuditFilters{
		UserID:   q.Get("user_id"),
		Action:   q.Get("action"),
		Resource: q.Get("resource"),
		Status:   q.Get("status"),
		From:     parseTimeParam(q.Get("from")),
		To:       parseTimeParam(q.Get("to")),
	}

	entries, total, err := h.store.List(limit, offset, filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list audit entries: "+err.Error())
		return
	}

	// CSV export.
	if strings.EqualFold(q.Get("format"), "csv") {
		h.writeCSV(w, entries)
		return
	}

	resp := map[string]interface{}{
		"entries": entries,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	}

	writeJSON(w, http.StatusOK, resp)
}

// Get handles GET /v1/audit/logs/{id} — returns a single audit entry by ID.
func (h *AuditHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "audit store not available")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "audit entry id is required")
		return
	}

	entry, err := h.store.GetByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get audit entry: "+err.Error())
		return
	}
	if entry == nil {
		writeError(w, http.StatusNotFound, "audit entry not found")
		return
	}

	writeJSON(w, http.StatusOK, entry)
}

// Prune handles POST /v1/audit/prune — removes audit entries older than a
// given age. Accepts a JSON body: {"retention_days": 90}. Default: 365 days.
func (h *AuditHandler) Prune(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "audit store not available")
		return
	}

	var req struct {
		RetentionDays int `json:"retention_days"`
	}
	if r.ContentLength > 0 {
		limitBody(w, r, bodyLimitSmall)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeDecodeError(w, err)
			return
		}
	}
	if req.RetentionDays <= 0 {
		req.RetentionDays = 365
	}

	cutoff := time.Now().AddDate(0, 0, -req.RetentionDays).Unix()
	deleted, err := h.store.Prune(cutoff)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to prune audit entries: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"deleted":        deleted,
		"retention_days": req.RetentionDays,
		"cutoff":         time.Unix(cutoff, 0).UTC().Format(time.RFC3339),
	})
}

// parseTimeParam parses a timestamp string as either a Unix timestamp or
// RFC3339 date. Returns 0 if the string is empty or invalid.
func parseTimeParam(s string) int64 {
	if s == "" {
		return 0
	}
	// Try Unix timestamp first.
	if ts, err := parseInt64(s); err == nil && ts > 0 {
		return ts
	}
	// Try RFC3339.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Unix()
	}
	// Try date-only (YYYY-MM-DD).
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.Unix()
	}
	return 0
}

func parseInt64(s string) (int64, error) {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number")
		}
		n = n*10 + int64(c-'0')
	}
	return n, nil
}

// writeCSV writes audit entries as a CSV download.
func (h *AuditHandler) writeCSV(w http.ResponseWriter, entries []audit.AuditEntry) {
	filename := fmt.Sprintf("audit_logs_%s.csv", time.Now().UTC().Format("20060102T150405Z"))
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	writer := csv.NewWriter(w)

	// Header row.
	_ = writer.Write([]string{"id", "user_id", "action", "resource", "details", "ip_address", "timestamp", "status"})

	for _, e := range entries {
		ts := time.Unix(e.Timestamp, 0).UTC().Format(time.RFC3339)
		_ = writer.Write([]string{e.ID, e.UserID, e.Action, e.Resource, e.Details, e.IPAddress, ts, e.Status})
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		// Can't change headers at this point — just log.
		_ = err
	}
}

// ── Audit logging helpers for use by other handlers ──────────────────────────

// LogEvent is a convenience helper for recording an audit event from HTTP handlers.
// It extracts the user identity from the request context.
func LogEvent(store *audit.Store, r *http.Request, action, resource, details, status string) {
	if store == nil {
		return
	}

	userID := "anonymous"
	if claims, ok := apiauth.ClaimsFromContext(r.Context()); ok && claims != nil {
		userID = claims.Sub
	}

	ip := r.RemoteAddr
	// Strip port from RemoteAddr if present.
	if idx := strings.LastIndex(ip, ":"); idx > 0 {
		ip = ip[:idx]
	}
	// Handle X-Forwarded-For header.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ip = strings.Split(xff, ",")[0]
		ip = strings.TrimSpace(ip)
	}

	entry := &audit.AuditEntry{
		UserID:    userID,
		Action:    action,
		Resource:  resource,
		Details:   details,
		IPAddress: ip,
		Status:    status,
	}

	if err := store.Record(entry); err != nil {
		log.Printf("ERROR: failed to record audit event: action=%s resource=%s status=%s err=%v", action, resource, status, err)
	}
}
