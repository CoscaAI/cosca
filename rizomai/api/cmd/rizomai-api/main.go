// Command rizomai-api é o gateway HTTP do RIZOMAI (binário 1 — ADR-004).
//
// Fase 3: conecta Postgres (migrations embedded), autentica por API key,
// serve o contrato /v1, executa o OAuth broker (X/LinkedIn/Telegram) e usa
// River como fila durável (fallback simulado via RIZOMAI_QUEUE=simulated).
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
	"github.com/rizomai/rizomai/internal/billing"
	"github.com/rizomai/rizomai/internal/oauth"
	"github.com/rizomai/rizomai/internal/platform"
	"github.com/rizomai/rizomai/internal/platform/linkedin"
	"github.com/rizomai/rizomai/internal/platform/telegram"
	"github.com/rizomai/rizomai/internal/platform/x"
	"github.com/rizomai/rizomai/internal/queue"
	"github.com/rizomai/rizomai/internal/store"
)

// devTokenKey é usado APENAS quando RIZOMAI_TOKEN_KEY não está definida.
// NUNCA use em produção (chave pública no código).
const devTokenKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func main() {
	log.SetPrefix("rizomai-api: ")
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL ausente — configure o Postgres (ex.: postgres://rizomai:rizomai@localhost:5433/rizomai?sslmode=disable). Suba com: docker compose -f deploy/docker-compose.yml up -d")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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

	// --- Configuração -----------------------------------------------------
	pepper := os.Getenv("API_KEY_PEPPER")
	if pepper == "" {
		pepper = "dev-pepper"
		log.Print("API_KEY_PEPPER não definida — usando 'dev-pepper' (apenas dev)")
	}

	tokenKeyHex := os.Getenv("RIZOMAI_TOKEN_KEY")
	if tokenKeyHex == "" {
		tokenKeyHex = devTokenKey
		log.Print("RIZOMAI_TOKEN_KEY não definida — usando chave DEV fixa (não use em produção)")
	}
	tokenKey, err := oauth.TokenKey(tokenKeyHex)
	if err != nil {
		log.Fatalf("RIZOMAI_TOKEN_KEY: %v", err)
	}

	baseURL := os.Getenv("PUBLIC_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	reg := platform.NewRegistry(platform.Config{
		X: x.Config{
			ClientID:     os.Getenv("X_CLIENT_ID"),
			ClientSecret: os.Getenv("X_CLIENT_SECRET"),
			RedirectURI:  baseURL + "/v1/connect/x/callback",
		},
		LinkedIn: linkedin.Config{
			ClientID:     os.Getenv("LINKEDIN_CLIENT_ID"),
			ClientSecret: os.Getenv("LINKEDIN_CLIENT_SECRET"),
			RedirectURI:  baseURL + "/v1/connect/linkedin/callback",
		},
		Telegram: telegram.Config{ParseMode: "HTML"},
	})

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

	// --- Fila (River durável | SimulatedQueue fallback) --------------------
	var jobs queue.Jobs
	var riverJobs *queue.RiverQueue

	switch os.Getenv("RIZOMAI_QUEUE") {
	case "", "river":
		rc, err := queue.StartRiver(ctx, st.Pool(), queue.RiverOptions{
			Store:   st,
			Registry: reg,
			TokenKey: tokenKey,
			Logger:  log.Default(),
		})
		if err != nil {
			log.Fatalf("river: %v", err)
		}
		riverJobs = queue.NewRiverQueue(rc, log.Default())
		jobs = riverJobs
		log.Print("fila: RIVER (durável)")
	default: // simulated
		jobs = &queue.SimulatedQueue{Store: st, TokenKey: tokenKey, Log: log.Default()}
		log.Print("fila: SIMULADA (RIZOMAI_QUEUE=simulated — demo sem conectores)")
	}

	// --- Billing (Stripe — ADR-010 §1.3) -----------------------------------
	stripeCfg := billing.FromEnv()
	if stripeCfg.SuccessURL == "" {
		stripeCfg.SuccessURL = baseURL + "/billing/success"
	}
	if stripeCfg.CancelURL == "" {
		stripeCfg.CancelURL = baseURL + "/billing/cancel"
	}
	stripeClient := billing.NewClient(stripeCfg)
	if !stripeClient.Configured() {
		log.Print("Stripe NÃO configurado (STRIPE_SECRET_KEY) — checkout retornará STRIPE_NOT_CONFIGURED")
	}

	mux := api.NewRouter(api.Deps{
		Store:           st,
		Jobs:            jobs,
		Registry:        reg,
		TokenKey:        tokenKey,
		BaseURL:         baseURL,
		APIKeyPepper:    pepper,
		RateLimitPerMin: rpm,
		Stripe:          stripeClient,
	})

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           middleware.Logging(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("ouvindo em %s (dashboard em /, healthz em /healthz)", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Print("sinal recebido — encerrando com grace")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		if riverJobs != nil {
			_ = riverJobs.Stop(shutdownCtx)
		}
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("servidor: %v", err)
		}
	}
	log.Print("encerrado")
}
