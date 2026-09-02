// Command webhook-worker consome os jobs de entrega de webhooks
// (webhook.deliver) via River (ADR-003/009) — processo consumidor dedicado
// (ADR-004: binário 3).
//
// Entrega eventos assinados HMAC-SHA256 com retry e MESMO event id (dedup do
// consumidor — ADR-009 §1.4).
//
// Uso: DATABASE_URL=... RIZOMAI_TOKEN_KEY=... go run ./workers/cmd/webhook-worker
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rizomai/rizomai/internal/oauth"
	"github.com/rizomai/rizomai/internal/queue"
	"github.com/rizomai/rizomai/internal/store"
)

func main() {
	log.SetPrefix("webhook-worker: ")
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

	client, err := queue.StartRiver(ctx, st.Pool(), queue.RiverOptions{
		Store:         st,
		TokenKey:      tokenKey,
		Logger:        log.Default(),
		WebhookWorker: true, // este processo só consome a fila de webhooks
	})
	if err != nil {
		log.Fatalf("river: %v", err)
	}

	log.Print("consumindo fila webhook (webhook.deliver) — Ctrl+C para encerrar")
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = client.Stop(shutdownCtx)
	log.Print("encerrado")
}
