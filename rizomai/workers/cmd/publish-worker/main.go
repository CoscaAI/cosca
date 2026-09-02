// Command publish-worker consome os jobs de publicação (publish.target) via
// River (ADR-003/007) — processo consumidor dedicado (ADR-004: binário 2).
//
// Compartilha a MESMA base Postgres (schema river) com a API: jobs enfileirados
// pelo gateway são processados aqui (ou por qualquer réplica do worker).
//
// Uso: DATABASE_URL=... RIZOMAI_TOKEN_KEY=... go run ./workers/cmd/publish-worker
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rizomai/rizomai/internal/oauth"
	"github.com/rizomai/rizomai/internal/platform"
	"github.com/rizomai/rizomai/internal/platform/linkedin"
	"github.com/rizomai/rizomai/internal/platform/telegram"
	"github.com/rizomai/rizomai/internal/platform/x"
	"github.com/rizomai/rizomai/internal/queue"
	"github.com/rizomai/rizomai/internal/store"
)

func main() {
	log.SetPrefix("publish-worker: ")
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL ausente (ex.: postgres://rizomai:rizomai@localhost:5433/rizomai?sslmode=disable)")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("banco: %v", err)
	}

	tokenKeyHex := os.Getenv("RIZOMAI_TOKEN_KEY")
	if tokenKeyHex == "" {
		tokenKeyHex = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"
		log.Print("RIZOMAI_TOKEN_KEY ausente — chave DEV fixa (não use em produção)")
	}
	tokenKey, err := oauth.TokenKey(tokenKeyHex)
	if err != nil {
		log.Fatalf("RIZOMAI_TOKEN_KEY: %v", err)
	}

	reg := platform.NewRegistry(platform.Config{
		X:        x.Config{ClientID: os.Getenv("X_CLIENT_ID"), ClientSecret: os.Getenv("X_CLIENT_SECRET")},
		LinkedIn: linkedin.Config{ClientID: os.Getenv("LINKEDIN_CLIENT_ID"), ClientSecret: os.Getenv("LINKEDIN_CLIENT_SECRET")},
		Telegram: telegram.Config{ParseMode: "HTML"},
	})

	client, err := queue.StartRiver(ctx, st.Pool(), queue.RiverOptions{
		Store:         st,
		Registry:      reg,
		TokenKey:      tokenKey,
		Logger:        log.Default(),
		PublishWorker: true, // este processo só consome a fila de publicação
	})
	if err != nil {
		log.Fatalf("river: %v", err)
	}

	log.Print("consumindo fila publish (publish.target) — Ctrl+C para encerrar")
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = client.Stop(shutdownCtx)
	log.Print("encerrado")
}
