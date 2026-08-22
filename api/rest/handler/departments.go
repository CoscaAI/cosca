package handler

import (
	"encoding/json"
	"net/http"

	"github.com/CoscaAI/cosca/internal/department"
)

// DepartmentsHandler handles inter-department conversation endpoints (Dept→Dept)
// backed by the append-only conversation ledger (internal/department, SQLite
// .cosca/department.db). List and Thread are read-only projections; Ask appends
// a question to a thread. It is nil-safe like TracesHandler: a nil store makes
// every route respond 503 with a clear pt-BR message.
type DepartmentsHandler struct {
	store *department.ConversationStore
}

// NewDepartmentsHandler creates a new DepartmentsHandler backed by the given
// store. A nil store is allowed (nil-safe): every route returns 503 so the rest
// of the server keeps working even when the ledger cannot be opened.
func NewDepartmentsHandler(store *department.ConversationStore) *DepartmentsHandler {
	return &DepartmentsHandler{store: store}
}

// List handles GET /v1/departments — returns the known departments (the
// static list approved by the Don) plus the most recent messages across all
// threads (created_at DESC), the wire shape the Control Center UI consumes.
func (h *DepartmentsHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store de departamentos indisponível")
		return
	}

	departments := make([]string, 0, len(department.KnownDepartments))
	for _, d := range department.KnownDepartments {
		departments = append(departments, string(d))
	}

	recent, err := h.store.List(parseIntParam(r.URL.Query().Get("limit"), 20))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "falha ao consultar conversas: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"departments": departments,
		"recent":      recent,
	})
}

// Thread handles GET /v1/departments/threads/{thread} — returns the full
// audit trail of a conversation: every message ordered by created_at ASC.
// Unknown threads respond 404 (pt-BR) — the ledger is append-only, so an
// empty thread is indistinguishable from a missing one.
func (h *DepartmentsHandler) Thread(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store de departamentos indisponível")
		return
	}

	thread := r.PathValue("thread")
	msgs, err := h.store.Thread(thread)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "falha ao consultar thread: "+err.Error())
		return
	}
	if len(msgs) == 0 {
		writeError(w, http.StatusNotFound, "thread não encontrada")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"thread_id": thread,
		"messages":  msgs,
	})
}

// departmentAskRequest is the JSON body for Ask.
type departmentAskRequest struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Topic   string `json:"topic"`
	Message string `json:"message"`
}

// Ask handles POST /v1/departments/ask — starts a conversation: <from> asks
// <to> about <topic>. Validates the departments (pt-BR error) and appends the
// question to a new/existing thread (<topic>:<data>). Returns the message ID
// and the thread key. Gated editor+ — a write (consumes no LLM but appends to
// the audit ledger).
func (h *DepartmentsHandler) Ask(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store de departamentos indisponível")
		return
	}

	var req departmentAskRequest
	if r.ContentLength > 0 {
		limitBody(w, r, bodyLimitSmall)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeDecodeError(w, err)
			return
		}
	}

	director := department.NewDirector(h.store)
	msgID, thread, err := director.Ask(
		department.DepartmentID(req.From),
		department.DepartmentID(req.To),
		req.Topic,
		req.Message,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"message_id": msgID,
		"thread_id":  thread,
	})
}
