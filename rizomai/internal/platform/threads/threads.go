// Package threads integra o Threads (API oficial da Meta). OAuth no padrão
// Instagram (2-pass: short-lived → th_exchange_token 60d — insumo §1.3).
// Publicação em 2 passos: criar thread (TEXT/IMAGE/VIDEO) → publicar.
package threads

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rizomai/rizomai/internal/domain"
	"github.com/rizomai/rizomai/internal/platform/types"
)

// Config são as credenciais de app (env THREADS_CLIENT_ID/_SECRET).
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// Configured indica se o conector tem credenciais de app.
func (c Config) Configured() bool { return c.ClientID != "" }

const (
	graphURL  = "https://graph.threads.net"
	authURL   = "https://threads.net/oauth/authorize"
	oauthScopes = "threads_basic threads_content_publish"
)

// Client é o conector do Threads.
type Client struct {
	cfg      Config
	http     *http.Client
	apiBase  string
	tokenURL string
	authBase string
}

// New cria o conector.
func New(cfg Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: 20 * time.Second}, apiBase: graphURL, tokenURL: graphURL + "/access_token", authBase: authURL}
}

// Name implementa types.Publisher.
func (c *Client) Name() string { return "threads" }

// AuthURL monta a URL de autorização.
func (c *Client) AuthURL(state string, _ string) (string, error) {
	if !c.cfg.Configured() {
		return "", fmt.Errorf("conector threads não configurado: defina THREADS_CLIENT_ID e THREADS_CLIENT_SECRET")
	}
	return c.authBase +
		"?client_id=" + c.cfg.ClientID +
		"&redirect_uri=" + c.cfg.RedirectURI +
		"&scope=" + oauthScopes +
		"&state=" + state +
		"&response_type=code", nil
}

// ExchangeCode troca code por token (short-lived) e depois long-lived 60d
// (th_exchange_token — insumo §1.3); ExternalID = threads user id.
func (c *Client) ExchangeCode(ctx context.Context, code, _, redirectURI string) (*types.Token, error) {
	form := "client_id=" + c.cfg.ClientID + "&client_secret=" + c.cfg.ClientSecret +
		"&code=" + code + "&redirect_uri=" + redirectURI + "&grant_type=authorization_code"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var short struct {
		AccessToken string `json:"access_token"`
		UserID      string `json:"user_id"`
	}
	if err := types.DoJSON(c.http, req, "threads", &short); err != nil {
		return nil, err
	}

	// Long-lived 60d.
	longReq, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		c.apiBase+"/access_token?grant_type=th_exchange_token&client_secret="+c.cfg.ClientSecret+
			"&access_token="+short.AccessToken, nil)
	var long struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := types.DoJSON(c.http, longReq, "threads", &long); err != nil {
		return nil, err
	}

	return &types.Token{
		AccessToken: long.AccessToken,
		ExpiresAt:   time.Now().Add(time.Duration(long.ExpiresIn) * time.Second),
		ExternalID:  short.UserID,
		Scope:       "threads_content_publish",
	}, nil
}

// Publish publica um thread de texto (media_type TEXT — insumo §2.1).
func (c *Client) Publish(ctx context.Context, content string, target *domain.PostTarget, creds types.Credentials) (*types.PublishResult, error) {
	if !c.cfg.Configured() {
		return nil, &types.Error{Platform: "threads", Code: "not_configured",
			Message: "conector threads não configurado: defina THREADS_CLIENT_ID e THREADS_CLIENT_SECRET"}
	}
	if creds.AccessToken == "" {
		return nil, &types.Error{Platform: "threads", Code: "no_credentials", Message: "conta threads sem token"}
	}
	userID := creds.ExternalID
	if userID == "" {
		return nil, &types.Error{Platform: "threads", Code: "no_account", Message: "conta threads sem user id"}
	}

	createBody, _ := json.Marshal(map[string]any{"media_type": "TEXT", "text": content})
	createReq, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		c.apiBase+"/"+userID+"/threads?access_token="+creds.AccessToken, strings.NewReader(string(createBody)))
	createReq.Header.Set("Content-Type", "application/json")
	var created struct {
		ID string `json:"id"`
	}
	if err := types.DoJSON(c.http, createReq, "threads", &created); err != nil {
		return nil, err
	}

	pubReq, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		c.apiBase+"/"+created.ID+"/publish?access_token="+creds.AccessToken, nil)
	var published struct {
		ID string `json:"id"`
	}
	if err := types.DoJSON(c.http, pubReq, "threads", &published); err != nil {
		return nil, err
	}

	return &types.PublishResult{
		ExternalID:   published.ID,
		PublishedURL: "https://www.threads.net/@threads/post/" + published.ID,
	}, nil
}

// ValidateAccount valida o token (GET /me — ADR-006 §1.3).
func (c *Client) ValidateAccount(ctx context.Context, creds types.Credentials) error {
	if creds.AccessToken == "" {
		return &types.Error{Platform: "threads", Code: "no_credentials", Message: "token ausente"}
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		c.apiBase+"/me?fields=id,username&access_token="+creds.AccessToken, nil)
	if err := types.DoJSON(c.http, req, "threads", &struct{}{}); err != nil {
		if pe, ok := err.(*types.Error); ok && (pe.HTTPStatus == 400 || pe.HTTPStatus == 401) {
			return &types.Error{Platform: "threads", Code: "invalid_token", Message: pe.Message, HTTPStatus: pe.HTTPStatus}
		}
		return err
	}
	return nil
}
