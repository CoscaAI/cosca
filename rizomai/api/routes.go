// Package api monta o roteador HTTP do gateway (ADR-004).
//
// A spec openapi/rizomai.yaml é a fonte única da verdade do contrato
// (ADR-005): os handlers registrados aqui devem espelhar a spec e serão
// validados contra ela na Fase 2.
package api

import (
	"net/http"

	"github.com/rizomai/rizomai/api/handlers"
)

// NewRouter registra todas as rotas do gateway e devolve o http.Handler final.
//
// Ordem importante para o ServeMux (Go 1.22+): rotas estáticas mais específicas
// (ex.: /v1/accounts/health) são registradas antes das parametrizadas
// (/v1/accounts/{id}), evitando que "health" seja capturado como {id}.
func NewRouter() http.Handler {
	mux := http.NewServeMux()

	// Health check operacional do binário — fora do contrato /v1.
	mux.HandleFunc("GET /healthz", handlers.Healthz)

	// Contrato /v1 — Fase 2:
	//   handlers por recurso (profiles, accounts, posts, media, webhooks)
	//   espelhando openapi/rizomai.yaml + validação de request/response
	//   contra a spec (ADR-005).
	// Middlewares planejados (ADR-004): auth de API key sk_... (ADR-006),
	// rate-limit, idempotency, recovery.

	return mux
}
