// Package api monta o roteador HTTP do gateway (ADR-004).
//
// A spec openapi/rizomai.yaml é a fonte única da verdade do contrato
// (ADR-005): as rotas registradas espelham a spec v0.1 + endpoints de OAuth.
package api

import (
	"net/http"

	"github.com/rizomai/rizomai/api/handlers"
	"github.com/rizomai/rizomai/api/middleware"
	"github.com/rizomai/rizomai/internal/platform"
	"github.com/rizomai/rizomai/internal/queue"
	"github.com/rizomai/rizomai/internal/store"
)

// Deps reúne as dependências do gateway (injetadas pelo main).
type Deps struct {
	Store           *store.Store
	Jobs            queue.Jobs
	Registry        *platform.Registry
	TokenKey        []byte // AES-256-GCM p/ tokens em repouso (RIZOMAI_TOKEN_KEY)
	BaseURL         string // base pública da API (PUBLIC_BASE_URL) p/ redirect_uri
	APIKeyPepper    string // pepper para hash das API keys (env API_KEY_PEPPER)
	RateLimitPerMin int    // token bucket por tenant (env RATE_LIMIT_PER_MIN)
}

// NewRouter monta o roteador do gateway.
//
// Públicas: /healthz e o callback OAuth (navegador — security: [] na spec).
// Rotas /v1/: protegidas por Auth (API key sk_... — ADR-006) + RateLimit.
//
// Ordem para o ServeMux (Go 1.22+): rotas estáticas mais específicas vencem as
// parametrizadas automaticamente (ex.: /v1/connect/{platform}/callback).
func NewRouter(d Deps) http.Handler {
	rateLimiter := middleware.NewRateLimiter(d.RateLimitPerMin)
	h := &handlers.Handlers{
		Store:    d.Store,
		Jobs:     d.Jobs,
		Registry: d.Registry,
		TokenKey: d.TokenKey,
		BaseURL:  d.BaseURL,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handlers.Healthz(d.Store))
	// Callback OAuth é público (redirect do navegador — ADR-006 §1.1).
	mux.HandleFunc("GET /v1/connect/{platform}/callback", h.ConnectCallback)

	// Sub-mux protegido (auth + rate-limit) para o contrato /v1.
	protected := http.NewServeMux()
	protected.HandleFunc("GET /v1/profiles", h.ListProfiles)
	protected.HandleFunc("POST /v1/profiles", h.CreateProfile)
	protected.HandleFunc("GET /v1/profiles/{id}", h.GetProfile)
	protected.HandleFunc("GET /v1/posts", h.ListPosts)
	protected.HandleFunc("POST /v1/posts", h.CreatePost)
	protected.HandleFunc("GET /v1/posts/{id}", h.GetPost)
	protected.HandleFunc("GET /v1/connect/{platform}", h.ConnectStart)
	protected.HandleFunc("POST /v1/connect/telegram/credentials", h.TelegramCredentials)

	chain := middleware.RateLimit(rateLimiter)(middleware.Auth(d.Store, d.APIKeyPepper)(protected))
	mux.Handle("/v1/", chain)

	return mux
}
