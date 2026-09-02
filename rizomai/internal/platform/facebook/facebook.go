// Package facebook integra o Facebook (Graph API) — páginas. OAuth via
// Facebook Login; publicação em `POST /{page-id}/feed`. A página de destino
// vem de creds.ExternalID ou platformSpecificData.pageId (insumo §2.1).
package facebook

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

// Config são as credenciais de app (env FACEBOOK_CLIENT_ID/_SECRET).
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// Configured indica se o conector tem credenciais de app.
func (c Config) Configured() bool { return c.ClientID != "" }

const (
	graphURL   = "https://graph.facebook.com/v20.0"
	oauthScopes = "pages_manage_posts pages_show_list pages_read_engagement"
)

// Client é o conector do Facebook.
type Client struct {
	cfg      Config
	http     *http.Client
	apiBase  string
	tokenURL string
	authBase string
}

// New cria o conector.
func New(cfg Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: 20 * time.Second}, apiBase: graphURL, tokenURL: graphURL + "/oauth/access_token", authBase: "https://www.facebook.com/v20.0/dialog/oauth"}
}

// Name implementa types.Publisher.
func (c *Client) Name() string { return "facebook" }

// AuthURL monta a URL de autorização (páginas).
func (c *Client) AuthURL(state string, _ string) (string, error) {
	if !c.cfg.Configured() {
		return "", fmt.Errorf("conector facebook não configurado: defina FACEBOOK_CLIENT_ID e FACEBOOK_CLIENT_SECRET")
	}
	return c.authBase +
		"?client_id=" + c.cfg.ClientID +
		"&redirect_uri=" + c.cfg.RedirectURI +
		"&scope=" + oauthScopes +
		"&state=" + state +
		"&response_type=code", nil
}

// ExchangeCode troca code por token de usuário (a seleção de página é Fase 2;
// ExternalID = user id — o Publish usa pageId do PSD ou ExternalID).
func (c *Client) ExchangeCode(ctx context.Context, code, _, redirectURI string) (*types.Token, error) {
	form := "client_id=" + c.cfg.ClientID + "&client_secret=" + c.cfg.ClientSecret +
		"&code=" + code + "&redirect_uri=" + redirectURI

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := types.DoJSON(c.http, req, "facebook", &out); err != nil {
		return nil, err
	}

	// user id via /me (para debug/health).
	meReq, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		c.apiBase+"/me?fields=id&access_token="+out.AccessToken, nil)
	var me struct{ ID string `json:"id"` }
	_ = types.DoJSON(c.http, meReq, "facebook", &me)

	return &types.Token{
		AccessToken: out.AccessToken,
		ExpiresAt:   time.Now().Add(time.Duration(out.ExpiresIn) * time.Second),
		ExternalID:  me.ID,
		Scope:       "pages_manage_posts",
	}, nil
}

// Publish publica na página: POST /{page-id}/feed (insumo §2.1).
func (c *Client) Publish(ctx context.Context, content string, target *domain.PostTarget, creds types.Credentials) (*types.PublishResult, error) {
	if !c.cfg.Configured() {
		return nil, &types.Error{Platform: "facebook", Code: "not_configured",
			Message: "conector facebook não configurado: defina FACEBOOK_CLIENT_ID e FACEBOOK_CLIENT_SECRET"}
	}
	if creds.AccessToken == "" {
		return nil, &types.Error{Platform: "facebook", Code: "no_credentials", Message: "conta facebook sem token"}
	}
	pageID := pageIDFromTarget(target, creds)
	if pageID == "" {
		return nil, &types.Error{Platform: "facebook", Code: "no_page",
			Message: "defina pageId no platformSpecificData do target ou selecione a página na conta"}
	}

	body, _ := json.Marshal(map[string]any{
		"message": content, "access_token": creds.AccessToken,
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBase+"/"+pageID+"/feed", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")

	var out struct {
		ID string `json:"id"`
	}
	if err := types.DoJSON(c.http, req, "facebook", &out); err != nil {
		return nil, err
	}
	if out.ID == "" {
		return nil, &types.Error{Platform: "facebook", Code: "api_error", Message: "resposta sem id"}
	}
	postID := out.ID
	slug := postID
	if i := strings.IndexByte(postID, '_'); i >= 0 {
		slug = postID[i+1:]
	}
	return &types.PublishResult{
		ExternalID:   postID,
		PublishedURL: "https://www.facebook.com/" + slug,
	}, nil
}

func pageIDFromTarget(t *domain.PostTarget, creds types.Credentials) string {
	if t != nil {
		if pid, ok := t.PlatformSpecificData["pageId"].(string); ok && pid != "" {
			return pid
		}
	}
	return creds.ExternalID
}

// ValidateAccount valida o token (GET /me).
func (c *Client) ValidateAccount(ctx context.Context, creds types.Credentials) error {
	if creds.AccessToken == "" {
		return &types.Error{Platform: "facebook", Code: "no_credentials", Message: "token ausente"}
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		c.apiBase+"/me?fields=id&access_token="+creds.AccessToken, nil)
	if err := types.DoJSON(c.http, req, "facebook", &struct{}{}); err != nil {
		if pe, ok := err.(*types.Error); ok && (pe.HTTPStatus == 400 || pe.HTTPStatus == 401) {
			return &types.Error{Platform: "facebook", Code: "invalid_token", Message: pe.Message, HTTPStatus: pe.HTTPStatus}
		}
		return err
	}
	return nil
}
