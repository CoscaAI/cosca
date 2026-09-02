// Package snapchat integra o Snapchat (Public Story API). OAuth 2.0
// allowlist-only (insumo §1.2/§2.1 — aprovação manual da equipe Snapchat);
// publicação de story com mídia (public profile).
package snapchat

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

// Config são as credenciais de app (env SNAPCHAT_CLIENT_ID/_SECRET).
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// Configured indica se o conector tem credenciais de app.
func (c Config) Configured() bool { return c.ClientID != "" }

const (
	apiBaseURL  = "https://story-api.snapchat.com"
	oauthScopes = "snapchat-profile-api"
)

// Client é o conector do Snapchat.
type Client struct {
	cfg      Config
	http     *http.Client
	apiBase  string
	tokenURL string
	authBase string
}

// New cria o conector.
func New(cfg Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: 20 * time.Second}, apiBase: apiBaseURL, tokenURL: apiBaseURL + "/v1/oauth2/token", authBase: "https://accounts.snapchat.com/accounts/oauth2/auth"}
}

// Name implementa types.Publisher.
func (c *Client) Name() string { return "snapchat" }

// AuthURL monta a URL de autorização (allowlist — insumo §1.2).
func (c *Client) AuthURL(state string, _ string) (string, error) {
	if !c.cfg.Configured() {
		return "", fmt.Errorf("conector snapchat não configurado: defina SNAPCHAT_CLIENT_ID e SNAPCHAT_CLIENT_SECRET")
	}
	return c.authBase +
		"?client_id=" + c.cfg.ClientID +
		"&redirect_uri=" + c.cfg.RedirectURI +
		"&scope=" + oauthScopes +
		"&state=" + state +
		"&response_type=code", nil
}

// ExchangeCode troca code por tokens.
func (c *Client) ExchangeCode(ctx context.Context, code, _, redirectURI string) (*types.Token, error) {
	form := "grant_type=authorization_code&client_id=" + c.cfg.ClientID +
		"&client_secret=" + c.cfg.ClientSecret + "&code=" + code + "&redirect_uri=" + redirectURI
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := types.DoJSON(c.http, req, "snapchat", &out); err != nil {
		return nil, err
	}
	return &types.Token{
		AccessToken: out.AccessToken,
		ExpiresAt:   time.Now().Add(time.Duration(out.ExpiresIn) * time.Second),
		Scope:       "snapchat-profile-api",
	}, nil
}

// Publish publica um story público (mídia obrigatória — public profile).
func (c *Client) Publish(ctx context.Context, content string, target *domain.PostTarget, creds types.Credentials) (*types.PublishResult, error) {
	if !c.cfg.Configured() {
		return nil, &types.Error{Platform: "snapchat", Code: "not_configured",
			Message: "conector snapchat não configurado: defina SNAPCHAT_CLIENT_ID e SNAPCHAT_CLIENT_SECRET"}
	}
	if creds.AccessToken == "" {
		return nil, &types.Error{Platform: "snapchat", Code: "no_credentials", Message: "conta snapchat sem token"}
	}
	media := types.MediaURLsFromTarget(target)
	if len(media) == 0 {
		return nil, &types.Error{Platform: "snapchat", Code: "no_media",
			Message: "Snapchat exige mídia — envie mediaUrls no platformSpecificData do target"}
	}

	body, _ := json.Marshal(map[string]any{
		"media": map[string]any{"type": "video", "url": media[0]},
		"text":  content,
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBase+"/v1/stories", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)

	var out struct {
		ID string `json:"id"`
	}
	if err := types.DoJSON(c.http, req, "snapchat", &out); err != nil {
		return nil, err
	}
	if out.ID == "" {
		return nil, &types.Error{Platform: "snapchat", Code: "api_error", Message: "resposta sem id"}
	}
	return &types.PublishResult{
		ExternalID:   out.ID,
		PublishedURL: "https://story.snapchat.com/s/" + out.ID,
	}, nil
}

// ValidateAccount valida o token.
func (c *Client) ValidateAccount(ctx context.Context, creds types.Credentials) error {
	if creds.AccessToken == "" {
		return &types.Error{Platform: "snapchat", Code: "no_credentials", Message: "token ausente"}
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBase+"/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	if err := types.DoJSON(c.http, req, "snapchat", &struct{}{}); err != nil {
		if pe, ok := err.(*types.Error); ok && (pe.HTTPStatus == 401 || pe.HTTPStatus == 403) {
			return &types.Error{Platform: "snapchat", Code: "invalid_token", Message: pe.Message, HTTPStatus: pe.HTTPStatus}
		}
		return err
	}
	return nil
}
