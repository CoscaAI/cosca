// Command rizomai-api é o gateway HTTP do RIZOMAI (binário 1 — ADR-004).
//
// Fase 2: conecta Postgres (migrations embedded + pgxpool), autentica por API
// key (sk_...) e serve os endpoints essenciais do contrato /v1.
//
// Fase 3: OAuth broker, conectores, webhooks, River de verdade.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/rizomai/rizomai/api"
	"github.com/rizomai/rizomai/api/middleware"
	"github.com/rizomai/rizomai/internal/queue"
	"github.com/rizomai/rizomai/internal/store"
)

func main() {
	log.SetPrefix("rizomai-api: ")
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL ausente — configure o Postgres (ex.: postgres://rizomai:rizomai@localhost:5432/rizomai?sslmode=disable). Suba com: docker compose -f deploy/docker-compose.yml up -d")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Migrations versionadas embedded (single-binary — ADR-001). Idempotente.
	if os.Getenv("SKIP_MIGRATIONS") != "1" {
		if err := store.Migrate(databaseURL); err != nil {
			log.Fatalf("migrate: %v", err)
		}
		log.Print("migrations aplicadas")
	}

	st, err := store.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("banco: %v", err)
	}

	pepper := os.Getenv("API_KEY_PEPPER")
	if pepper == "" {
		pepper = "dev-pepper"
		log.Print("API_KEY_PEPPER não definida — usando 'dev-pepper' (apenas dev)")
	}
	rpm := 60
	if v := os.Getenv("RATE_LIMIT_PER_MIN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			rpm = n
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := api.NewRouter(api.Deps{
		Store:           st,
		Jobs:            &queue.SimulatedQueue{Store: st, Log: log.Default()},
		APIKeyPepper:    pepper,
		RateLimitPerMin: rpm,
	})

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           middleware.Logging(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("ouvindo em %s (healthz em /healthz)", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Print("sinal recebido — encerrando com grace")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("servidor: %v", err)
		}
	}
	log.Print("encerrado")
}
