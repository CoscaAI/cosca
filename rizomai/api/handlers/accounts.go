package handlers

import (
	"log"
	"net/http"

	"github.com/rizomai/rizomai/api/middleware"
	"github.com/rizomai/rizomai/api/respond"
	"github.com/rizomai/rizomai/internal/domain"
)

// ListAccounts — GET /v1/accounts (paginado, escopo do team).
// Filtra por profileId e/ou platform (spec). NUNCA expõe tokens de rede
// social — apenas tokenStatus e metadados (ADR-006 §1.2).
func (h *Handlers) ListAccounts(w http.ResponseWriter, r *http.Request) {
	teamID := middleware.TeamIDFromContext(r.Context())
	page, limit := pageLimit(r)
	offset := (page - 1) * limit

	accounts, total, err := h.Store.ListAccountsByTeam(r.Context(), teamID,
		r.URL.Query().Get("profileId"), r.URL.Query().Get("platform"), limit, offset)
	if err != nil {
		log.Printf("list accounts: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}
	if accounts == nil {
		accounts = []domain.SocialAccount{}
	}
	writeList(w, http.StatusOK, accounts, page, limit, total)
}
