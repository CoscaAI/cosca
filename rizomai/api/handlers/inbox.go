// Handlers do inbox unificado (Fase 6): DMs/comentários/menções.
package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/rizomai/rizomai/api/middleware"
	"github.com/rizomai/rizomai/api/respond"
	"github.com/rizomai/rizomai/internal/domain"
)

// ListInbox — GET /v1/inbox?platform=&isRead=&limit=: mensagens do team.
func (h *Handlers) ListInbox(w http.ResponseWriter, r *http.Request) {
	teamID := middleware.TeamIDFromContext(r.Context())

	platform := r.URL.Query().Get("platform")
	var isRead *bool
	if v := r.URL.Query().Get("isRead"); v != "" {
		b := v == "true"
		isRead = &b
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	msgs, err := h.Store.ListMessages(r.Context(), teamID, platform, isRead, limit)
	if err != nil {
		log.Printf("list inbox: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}
	if msgs == nil {
		msgs = []domain.InboxMessage{}
	}
	writeData(w, http.StatusOK, msgs)
}

// MarkInboxRead — POST /v1/inbox/{id}/read: marca a mensagem como lida.
func (h *Handlers) MarkInboxRead(w http.ResponseWriter, r *http.Request) {
	teamID := middleware.TeamIDFromContext(r.Context())
	id := r.PathValue("id")

	err := h.Store.MarkRead(r.Context(), teamID, id)
	switch {
	case isNotFound(err):
		respond.Error(w, http.StatusNotFound, "NOT_FOUND", "Mensagem não encontrada",
			map[string]any{"resource": "message", "id": id})
	case err != nil:
		log.Printf("mark read: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
	default:
		writeData(w, http.StatusOK, map[string]any{"id": id, "isRead": true})
	}
}
