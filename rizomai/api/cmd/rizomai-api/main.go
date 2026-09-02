// Command rizomai-api é o gateway HTTP do RIZOMAI (binário 1 — ADR-004).
//
// Fase 1 (scaffold): servidor HTTP com /healthz e logging mínimo.
// NÃO conecta banco nem fila ainda — ver "próximos passos" abaixo.
//
// Próximos passos (Fase 2):
//   1. internal/store: pool pgx + migrations versionadas (schema public + river)
//      — ADR-002/003
//   2. Autenticação por API key `sk_...` (middleware) — ADR-006
//   3. Handlers por recurso validados contra openapi/rizomai.yaml — ADR-005
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/rizomai/rizomai/api"
	"github.com/rizomai/rizomai/api/middleware"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := api.NewRouter()
	handler := middleware.Logging(mux)

	addr := ":" + port
	log.Printf("rizomai-api: ouvindo em %s (healthz em /healthz)", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("rizomai-api: servidor encerrou com erro: %v", err)
	}
}
