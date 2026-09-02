// Package pinterest integra o Pinterest (API v5). OAuth v2; publicação de pins
// com imagem (media_source image_url — insumo §2.1/§3). Board via
// platformSpecificData.boardId (obrigatório).
package pinterest

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

// Config são as credenciais de app (env PINTEREST_CLIENT_ID/_SECRET).
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// Configured indica se o conector tem credenciais de app.
func (c Config) Configured() bool { return c.ClientID != "" }

const (
	apiBaseURL  = "https://api.pinterest.com/v5"
	oauthScopes = "pins:read,pins:write,boards:read,user_accounts:read"
)

// Client é o conector do Pinterest.
type Client struct {
	cfg      Config
	http     *http.Client
	apiBase  string
	tokenURL string
	authBase string
}

// New cria o conector.
func New(cfg Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: 20 * time.Second}, apiBase: apiBaseURL, tokenURL: apiBaseURL + "/oauth/token", authBase: "https://www.pinterest.com/oauth/"}
}

// Name implementa types.Publisher.
func (c *Client) Name() string { return "pinterest" }

// AuthURL monta a URL de autorização.
func (c *Client) AuthURL(state string, _ string) (string, error) {
	if !c.cfg.Configured() {
		return "", fmt.Errorf("conector pinterest não configurado: defina PINTEREST_CLIENT_ID e PINTEREST_CLIENT_SECRET")
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
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := types.DoJSON(c.http, req, "pinterest", &out); err != nil {
		return nil, err
	}
	return &types.Token{
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(out.ExpiresIn) * time.Second),
		Scope:        out.Scope,
	}, nil
}

// Publish cria um pin (imagem obrigatória — insumo §2.1: cover p/ vídeo).
func (c *Client) Publish(ctx context.Context, content string, target *domain.PostTarget, creds types.Credentials) (*types.PublishResult, error) {
	if !c.cfg.Configured() {
		return nil, &types.Error{Platform: "pinterest", Code: "not_configured",
			Message: "conector pinterest não configurado: defina PINTEREST_CLIENT_ID e PINTEREST_CLIENT_SECRET"}
	}
	if creds.AccessToken == "" {
		return nil, &types.Error{Platform: "pinterest", Code: "no_credentials", Message: "conta pinterest sem token"}
	}
	boardID, _ := target.PlatformSpecificData["boardId"].(string)
	if boardID == "" {
		return nil, &types.Error{Platform: "pinterest", Code: "no_board",
			Message: "defina boardId no platformSpecificData do target"}
	}
	media := types.MediaURLsFromTarget(target)
	if len(media) == 0 {
		return nil, &types.Error{Platform: "pinterest", Code: "no_media",
			Message: "Pinterest exige imagem — envie mediaUrls no platformSpecificData do target"}
	}
	title, _ := target.PlatformSpecificData["title"].(string)

	body, _ := json.Marshal(map[string]any{
		"board_id":    boardID,
		"title":       title,
		"description": content,
		"media_source": map[string]any{"source_type": "image_url", "url": media[0]},
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBase+"/pins", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)

	var out struct {
		ID  string `json:"id"`
		URL string `json:"link"`
	}
	if err := types.DoJSON(c.http, req, "pinterest", &out); err != nil {
		return nil, err
	}
	if out.ID == "" {
		return nil, &types.Error{Platform: "pinterest", Code: "api_error", Message: "resposta sem id"}
	}
	return &types.PublishResult{
		ExternalID:   out.ID,
		PublishedURL: "https://www.pinterest.com/pin/" + out.ID,
	}, nil
}

// ValidateAccount valida o token (GET /v5/user_account).
func (c *Client) ValidateAccount(ctx context.Context, creds types.Credentials) error {
	if creds.AccessToken == "" {
		return &types.Error{Platform: "pinterest", Code: "no_credentials", Message: "token ausente"}
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBase+"/user_account", nil)
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	if err := types.DoJSON(c.http, req, "pinterest", &struct{}{}); err != nil {
		if pe, ok := err.(*types.Error); ok && (pe.HTTPStatus == 401 || pe.HTTPStatus == 403) {
			return &types.Error{Platform: "pinterest", Code: "invalid_token", Message: pe.Message, HTTPStatus: pe.HTTPStatus}
		}
		return err
	}
	return nil
}
