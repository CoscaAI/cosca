// Package googlebusiness integra o Google Business Profile (My Business API).
// OAuth Google; publicação de posts na localização (POST localPosts com
// summary + callToAction — insumo §2.1/§6). accountId/locationId vêm do
// external_identifier da conta ("accounts/123/locations/456").
package googlebusiness

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

// Config são as credenciais de app (env GOOGLEBUSINESS_CLIENT_ID/_SECRET).
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// Configured indica se o conector tem credenciais de app.
func (c Config) Configured() bool { return c.ClientID != "" }

const (
	apiBaseURL  = "https://mybusiness.googleapis.com"
	tokenURL    = "https://oauth2.googleapis.com/token"
	authBaseURL = "https://accounts.google.com/o/oauth2/v2/auth"
	oauthScopes = "https://www.googleapis.com/auth/business.manage"
)

// Client é o conector do Google Business.
type Client struct {
	cfg      Config
	http     *http.Client
	apiBase  string
	tokenURL string
	authBase string
}

// New cria o conector.
func New(cfg Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: 20 * time.Second}, apiBase: apiBaseURL, tokenURL: tokenURL, authBase: authBaseURL}
}

// Name implementa types.Publisher.
func (c *Client) Name() string { return "googlebusiness" }

// AuthURL monta a URL de autorização (offline p/ refresh — insumo §6.4).
func (c *Client) AuthURL(state string, _ string) (string, error) {
	if !c.cfg.Configured() {
		return "", fmt.Errorf("conector googlebusiness não configurado: defina GOOGLEBUSINESS_CLIENT_ID e GOOGLEBUSINESS_CLIENT_SECRET")
	}
	return c.authBase +
		"?client_id=" + c.cfg.ClientID +
		"&redirect_uri=" + c.cfg.RedirectURI +
		"&scope=" + oauthScopes +
		"&state=" + state +
		"&response_type=code&access_type=offline&prompt=consent", nil
}

// ExchangeCode troca code por tokens.
func (c *Client) ExchangeCode(ctx context.Context, code, _, redirectURI string) (*types.Token, error) {
	form := "client_id=" + c.cfg.ClientID + "&client_secret=" + c.cfg.ClientSecret +
		"&code=" + code + "&redirect_uri=" + redirectURI + "&grant_type=authorization_code"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := types.DoJSON(c.http, req, "googlebusiness", &out); err != nil {
		return nil, err
	}
	return &types.Token{
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(out.ExpiresIn) * time.Second),
		Scope:        "business.manage",
	}, nil
}

// Publish publica um post na localização (POST localPosts).
func (c *Client) Publish(ctx context.Context, content string, target *domain.PostTarget, creds types.Credentials) (*types.PublishResult, error) {
	if !c.cfg.Configured() {
		return nil, &types.Error{Platform: "googlebusiness", Code: "not_configured",
			Message: "conector googlebusiness não configurado: defina GOOGLEBUSINESS_CLIENT_ID e GOOGLEBUSINESS_CLIENT_SECRET"}
	}
	if creds.AccessToken == "" {
		return nil, &types.Error{Platform: "googlebusiness", Code: "no_credentials", Message: "conta googlebusiness sem token"}
	}
	// external_identifier: "accounts/{id}/locations/{id}" — insumo §2.1.
	location := creds.ExternalID
	if location == "" {
		location, _ = target.PlatformSpecificData["location"].(string)
	}
	if location == "" {
		return nil, &types.Error{Platform: "googlebusiness", Code: "no_location",
			Message: "defina external_identifier da conta como accounts/{id}/locations/{id}"}
	}

	body := map[string]any{"summary": content}
	if cta, ok := target.PlatformSpecificData["callToAction"].(map[string]any); ok {
		body["callToAction"] = cta
	}
	payload, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		c.apiBase+"/v4/"+location+"/localPosts", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)

	var out struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	}
	if err := types.DoJSON(c.http, req, "googlebusiness", &out); err != nil {
		return nil, err
	}
	if out.Name == "" && out.ID == "" {
		return nil, &types.Error{Platform: "googlebusiness", Code: "api_error", Message: "resposta sem name"}
	}
	id := out.ID
	if id == "" {
		id = out.Name
	}
	return &types.PublishResult{
		ExternalID:   out.Name,
		PublishedURL: "https://business.google.com/posts/" + id,
	}, nil
}

// ValidateAccount valida o token (GET /v4/accounts — lista contas).
func (c *Client) ValidateAccount(ctx context.Context, creds types.Credentials) error {
	if creds.AccessToken == "" {
		return &types.Error{Platform: "googlebusiness", Code: "no_credentials", Message: "token ausente"}
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBase+"/v4/accounts", nil)
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	if err := types.DoJSON(c.http, req, "googlebusiness", &struct{}{}); err != nil {
		if pe, ok := err.(*types.Error); ok && (pe.HTTPStatus == 401 || pe.HTTPStatus == 403) {
			return &types.Error{Platform: "googlebusiness", Code: "invalid_token", Message: pe.Message, HTTPStatus: pe.HTTPStatus}
		}
		return err
	}
	return nil
}
