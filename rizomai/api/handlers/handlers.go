// Package handlers contém os handlers HTTP do gateway, 1 arquivo por recurso,
// espelhando openapi/rizomai.yaml (ADR-004).
package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/rizomai/rizomai/api/respond"
	"github.com/rizomai/rizomai/internal/platform"
	"github.com/rizomai/rizomai/internal/queue"
	"github.com/rizomai/rizomai/internal/store"
)

// maxBodyBytes limita o corpo das mutações (1 MB — posts de texto + lista de
// targets; mídia vai por upload/presign, ADR-008).
const maxBodyBytes = 1 << 20

// Handlers agrupa as dependências dos handlers HTTP.
type Handlers struct {
	Store    *store.Store
	Jobs     queue.Jobs
	Registry *platform.Registry
	TokenKey []byte // AES-256-GCM (RIZOMAI_TOKEN_KEY) p/ tokens em repouso
	BaseURL  string // base pública da API p/ redirect_uri (PUBLIC_BASE_URL)
}

// decodeJSON lê o corpo (limitado) e decodifica; em erro de formato responde
// 400 BAD_REQUEST e devolve false.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Falha ao ler o corpo da requisição", nil)
		return false
	}
	if len(body) == 0 {
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Corpo da requisição vazio", nil)
		return false
	}
	if err := json.Unmarshal(body, v); err != nil {
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Corpo da requisição inválido",
			map[string]any{"error": err.Error()})
		return false
	}
	return true
}

// writeData escreve o envelope {data} e devolve o body serializado
// (necessário para armazenar a resposta na idempotency — ADR-005 §1.3).
func writeData(w http.ResponseWriter, status int, data any) []byte {
	env := map[string]any{"data": data}
	b, _ := json.Marshal(env)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(b)
	return b
}

// writeList escreve o envelope paginado e devolve o body serializado.
func writeList(w http.ResponseWriter, status int, data any, page, limit, total int) []byte {
	env := map[string]any{"data": data, "page": page, "limit": limit, "total": total}
	b, _ := json.Marshal(env)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(b)
	return b
}

// replayResponse devolve uma resposta previamente armazenada (idempotency).
func replayResponse(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// pageLimit extrai page/limit dos query params com defaults da spec
// (page=1, limit=20, máx 100).
func pageLimit(r *http.Request) (page, limit int) {
	page, limit = 1, 20
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			page = n
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 100 {
			limit = n
		}
	}
	return page, limit
}

// isNotFound auxilia o mapeamento de erros do store.
func isNotFound(err error) bool { return errors.Is(err, store.ErrNotFound) }
