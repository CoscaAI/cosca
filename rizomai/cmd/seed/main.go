// Command seed bootstrap do ambiente de dev: aplica migrations, cria o team,
// uma API key (imprime a chave UMA única vez) e contas fictícias por
// plataforma para exercitar o pipeline de posts (a conexão OAuth real é Fase 3).
//
// Uso: DATABASE_URL=... go run ./cmd/seed
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/rizomai/rizomai/internal/domain"
	"github.com/rizomai/rizomai/internal/store"
)

func main() {
	log.SetPrefix("rizomai-seed: ")
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL ausente (ex.: postgres://rizomai:rizomai@localhost:5432/rizomai?sslmode=disable)")
	}
	pepper := os.Getenv("API_KEY_PEPPER")
	if pepper == "" {
		pepper = "dev-pepper"
	}

	ctx := context.Background()
	if err := store.Migrate(databaseURL); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	st, err := store.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("banco: %v", err)
	}

	// Team de dev (idempotente).
	const teamID = "team_dev"
	if err := st.EnsureTeam(ctx, teamID, "Dev Team"); err != nil {
		log.Fatalf("team: %v", err)
	}
	log.Printf("team %s garantido", teamID)

	// Profile padrão (recria se não existir — seed é idempotente por natureza
	// de dev; IDs são gerados, então repetir cria profiles novos. Aceitável
	// para bootstrap: o profile antigo fica órfão mas inofensivo).
	profileID, err := domain.NewProfileID()
	if err != nil {
		log.Fatal(err)
	}
	if err := st.CreateProfile(ctx, &domain.Profile{ID: profileID, TeamID: teamID, Name: "Default Profile"}); err != nil {
		log.Fatalf("profile: %v", err)
	}
	log.Printf("profile %s criado", profileID)

	// Contas fictícias por plataforma do MVP (ADR-006).
	accounts := []struct {
		platform domain.Platform
		handle   string
	}{
		{domain.PlatformX, "dev_x"},
		{domain.PlatformLinkedIn, "dev_linkedin"},
		{domain.PlatformTelegram, "dev_telegram"},
	}
	for _, a := range accounts {
		accountID, err := domain.NewAccountID()
		if err != nil {
			log.Fatal(err)
		}
		acct := &domain.SocialAccount{
			ID:             accountID,
			ProfileID:      profileID,
			Platform:       a.platform,
			DisplayName:    a.handle,
			PlatformUserID: a.handle,
			TokenStatus:    domain.TokenStatusOK,
			ConnectedAt:    time.Now().UTC(),
		}
		if err := st.CreateAccount(ctx, acct); err != nil {
			log.Fatalf("account %s: %v", a.platform, err)
		}
		log.Printf("conta %s → %s", a.platform, accountID)
	}

	// API key (a chave pura só aparece AQUI).
	plainKey, key, err := st.CreateAPIKey(ctx, teamID, "dev-key", pepper)
	if err != nil {
		log.Fatalf("api key: %v", err)
	}
	log.Printf("API key criada: %s (id %s)", key.KeyPrefix, key.ID)

	log.Print("──────────────────────────────────────────────")
	log.Printf("TEAM_ID:      %s", teamID)
	log.Printf("PROFILE_ID:   %s", profileID)
	log.Printf("API_KEY:      %s", plainKey)
	log.Print("Use nos requests:  Authorization: Bearer <API_KEY>")
	log.Print("──────────────────────────────────────────────")
}
