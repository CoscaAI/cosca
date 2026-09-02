package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"time"

	"github.com/rizomai/rizomai/api/middleware"
	"github.com/rizomai/rizomai/api/respond"
	"github.com/rizomai/rizomai/internal/domain"
	"github.com/rizomai/rizomai/internal/oauth"
	"github.com/rizomai/rizomai/internal/platform"
	"github.com/rizomai/rizomai/internal/platform/x"
)

// oauthStateTTL é a validade do state (uso único — ADR-006 §1.1).
const oauthStateTTL = 10 * time.Minute

// ConnectStart — GET /v1/connect/{platform}?profileId=... (ADR-006 §1.1).
// Monta a URL de autorização (PKCE no X), persiste o state e devolve {authUrl, state}.
func (h *Handlers) ConnectStart(w http.ResponseWriter, r *http.Request) {
	teamID := middleware.TeamIDFromContext(r.Context())
	p := domain.Platform(r.PathValue("platform"))
	profileID := r.URL.Query().Get("profileId")

	if p == domain.PlatformTelegram {
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST",
			"Telegram não usa OAuth — POST /v1/connect/telegram/credentials com {botToken, chatId}", nil)
		return
	}

	oa, err := h.Registry.OAuth(p)
	if err != nil {
		respond.Error(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Plataforma não suportada",
			map[string]any{"fields": map[string]string{"platform": "use x ou linkedin"}})
		return
	}

	state, err := randomToken(16)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}

	var verifier string
	if p == domain.PlatformX {
		verifier, err = x.NewCodeVerifier() // PKCE S256 (insumo §6.1)
		if err != nil {
			respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
			return
		}
	}

	authURL, err := oa.AuthURL(state, verifier)
	if err != nil {
		respond.Error(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
		return
	}

	os := &domain.OAuthState{
		ID:           mustID(domain.NewOAuthStateID),
		TeamID:       teamID,
		Platform:     p,
		State:        state,
		CodeVerifier: verifier,
		RedirectURI:  oauthRedirectURI(h, p),
		ProfileID:    profileID,
		ExpiresAt:    time.Now().UTC().Add(oauthStateTTL),
	}
	if err := h.Store.CreateOAuthState(r.Context(), os); err != nil {
		log.Printf("oauth state: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}

	writeData(w, http.StatusOK, map[string]any{"authUrl": authURL, "state": state})
}

// ConnectCallback — GET /v1/connect/{platform}/callback (PÚBLICO — navegador).
// Valida o state, troca code por token, criptografa e salva a conta (ADR-006).
func (h *Handlers) ConnectCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", "query params code e state são obrigatórios", nil)
		return
	}

	ctx := r.Context()
	os, err := h.Store.GetOAuthState(ctx, state)
	if isNotFound(err) {
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", "state inválido ou já utilizado", nil)
		return
	}
	if err != nil {
		log.Printf("oauth state lookup: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}
	if time.Now().After(os.ExpiresAt) {
		_ = h.Store.DeleteOAuthState(ctx, state)
		respond.Error(w, http.StatusBadRequest, "BAD_REQUEST", "state expirado — reinicie a conexão", nil)
		return
	}

	oa, err := h.Registry.OAuth(os.Platform)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Plataforma não suportada", nil)
		return
	}

	tok, err := oa.ExchangeCode(ctx, code, os.CodeVerifier, os.RedirectURI)
	if err != nil {
		log.Printf("oauth exchange (%s): %v", os.Platform, err)
		respond.Error(w, http.StatusBadGateway, "INTERNAL_ERROR", "Falha na troca do code com a plataforma",
			map[string]any{"error": err.Error()})
		return
	}

	profileID := os.ProfileID
	if profileID == "" {
		profileID, err = h.ensureProfile(ctx, os.TeamID)
		if err != nil {
			log.Printf("oauth ensure profile: %v", err)
			respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
			return
		}
	}

	encTok, _ := oauth.Encrypt([]byte(tok.AccessToken), h.TokenKey)
	encRefresh, _ := oauth.Encrypt([]byte(tok.RefreshToken), h.TokenKey)
	expiresAt := tok.ExpiresAt

	acct := &domain.SocialAccount{
		ID:                    mustID(domain.NewAccountID),
		ProfileID:             profileID,
		Platform:              os.Platform,
		TokenStatus:           domain.TokenStatusOK,
		EncryptedToken:        encTok,
		RefreshTokenEncrypted: encRefresh,
		ExpiresAt:             &expiresAt,
		ExternalIdentifier:    tok.ExternalID,
		TokenScope:            tok.Scope,
		ConnectedAt:           time.Now().UTC(),
	}
	if err := h.Store.UpsertAccount(ctx, acct); err != nil {
		log.Printf("oauth upsert account: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}
	_ = h.Store.DeleteOAuthState(ctx, state)

	log.Printf("oauth: conta %s conectada (%s, profile %s)", acct.ID, os.Platform, profileID)
	writeData(w, http.StatusOK, map[string]any{"status": "connected", "accountId": acct.ID})
}

// TelegramCredentials — POST /v1/connect/telegram/credentials (ADR-006 §4 /
// insumo §6.3): recebe bot token + chat_id, valida com getMe e salva criptografado.
func (h *Handlers) TelegramCredentials(w http.ResponseWriter, r *http.Request) {
	teamID := middleware.TeamIDFromContext(r.Context())

	var payload struct {
		BotToken  string `json:"botToken"`
		ChatID    string `json:"chatId"`
		ProfileID string `json:"profileId"`
	}
	if !decodeJSON(w, r, &payload) {
		return
	}
	if payload.BotToken == "" || payload.ChatID == "" {
		respond.Error(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Dados inválidos",
			map[string]any{"fields": map[string]string{"botToken": "obrigatório", "chatId": "obrigatório"}})
		return
	}

	tg, err := h.Registry.Publisher(domain.PlatformTelegram)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}

	creds := platform.Credentials{AccessToken: payload.BotToken, ExternalID: payload.ChatID}
	if err := tg.ValidateAccount(r.Context(), creds); err != nil {
		respond.Error(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Bot token inválido",
			map[string]any{"error": err.Error()})
		return
	}

	profileID := payload.ProfileID
	if profileID == "" {
		profileID, err = h.ensureProfile(r.Context(), teamID)
		if err != nil {
			respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
			return
		}
	}

	encTok, _ := oauth.Encrypt([]byte(payload.BotToken), h.TokenKey)
	acct := &domain.SocialAccount{
		ID:                 mustID(domain.NewAccountID),
		ProfileID:          profileID,
		Platform:           domain.PlatformTelegram,
		DisplayName:        "Telegram",
		TokenStatus:        domain.TokenStatusOK,
		EncryptedToken:     encTok,
		ExternalIdentifier: payload.ChatID,
		ConnectedAt:        time.Now().UTC(),
	}
	if err := h.Store.UpsertAccount(r.Context(), acct); err != nil {
		log.Printf("telegram upsert: %v", err)
		respond.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Erro interno — tente novamente", nil)
		return
	}

	log.Printf("oauth: conta telegram %s conectada (profile %s)", acct.ID, profileID)
	writeData(w, http.StatusOK, map[string]any{"status": "connected", "accountId": acct.ID})
}

// ensureProfile cria um profile padrão no team quando o destino não foi dado.
func (h *Handlers) ensureProfile(ctx context.Context, teamID string) (string, error) {
	id, err := domain.NewProfileID()
	if err != nil {
		return "", err
	}
	p := &domain.Profile{ID: id, TeamID: teamID, Name: "Default Profile"}
	if err := h.Store.CreateProfile(ctx, p); err != nil {
		return "", err
	}
	return id, nil
}

func oauthRedirectURI(h *Handlers, p domain.Platform) string {
	return h.BaseURL + "/v1/connect/" + string(p) + "/callback"
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func mustID(f func() (string, error)) string {
	id, _ := f()
	return id
}
