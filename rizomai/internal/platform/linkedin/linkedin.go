// Package linkedin integra o LinkedIn (API v2). OAuth 2.0 standard; headers
// OBRIGATÓRIOS X-RestLi-Protocol-Version: 2.0.0 e LinkedIn-Version por endpoint
// (insumo §6.2 — versão errada = 400 misterioso). URN author da conta.
package linkedin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rizomai/rizomai/internal/domain"
	"github.com/rizomai/rizomai/internal/platform/types"
)

// Config são as credenciais de app (env LINKEDIN_CLIENT_ID/_SECRET).
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// Configured indica se o conector tem credenciais de app.
func (c Config) Configured() bool { return c.ClientID != "" }

const (
	authBaseURL    = "https://www.linkedin.com/oauth/v2/authorization"
	tokenURL       = "https://www.linkedin.com/oauth/v2/accessToken"
	apiBaseURL     = "https://api.linkedin.com"
	linkedInVer    = "202511" // versionamento por HEADER (não path) — insumo §6.2
	restliVersion  = "2.0.0"
	oauthScopes    = "w_member_social r_liteprofile r_emailaddress offline_access"
)

// Client é o conector do LinkedIn.
type Client struct {
	cfg     Config
	http    *http.Client
	apiBase string // base da API (override em testes)
	tokenURL string
	authBase string
}

// New cria o conector.
func New(cfg Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: 15 * time.Second}, apiBase: apiBaseURL, tokenURL: tokenURL, authBase: authBaseURL}
}

// Name implementa types.Publisher.
func (c *Client) Name() string { return "linkedin" }

// AuthURL monta a URL de autorização (code flow, sem PKCE).
func (c *Client) AuthURL(state string, _ string) (string, error) {
	if !c.cfg.Configured() {
		return "", fmt.Errorf("conector linkedin não configurado: defina LINKEDIN_CLIENT_ID e LINKEDIN_CLIENT_SECRET")
	}
	return c.authBase +
		"?response_type=code" +
		"&client_id=" + c.cfg.ClientID +
		"&redirect_uri=" + c.cfg.RedirectURI +
		"&scope=" + oauthScopes +
		"&state=" + state, nil
}

// ExchangeCode troca o code por tokens (access 60d + refresh 365d — insumo §6.2).
func (c *Client) ExchangeCode(ctx context.Context, code, _, redirectURI string) (*types.Token, error) {
	form := "grant_type=authorization_code&code=" + code +
		"&redirect_uri=" + redirectURI +
		"&client_id=" + c.cfg.ClientID +
		"&client_secret=" + c.cfg.ClientSecret

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, bytes.NewBufferString(form))
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
	if err := c.doJSON(req, &out); err != nil {
		return nil, err
	}

	tok := &types.Token{
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(out.ExpiresIn) * time.Second),
		Scope:        out.Scope,
	}

	// author URN via userinfo (sub já é urn:li:person:<id>).
	author, err := c.fetchAuthorURN(ctx, out.AccessToken)
	if err != nil {
		return nil, err
	}
	tok.ExternalID = author
	return tok, nil
}

// fetchAuthorURN busca a URN da pessoa autenticada (GET /v2/userinfo).
func (c *Client) fetchAuthorURN(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBase+"/v2/userinfo", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("LinkedIn-Version", linkedInVer)

	var out struct {
		Sub string `json:"sub"`
	}
	if err := c.doJSON(req, &out); err != nil {
		return "", err
	}
	if out.Sub == "" {
		return "", &types.Error{Platform: "linkedin", Code: "no_author", Message: "userinfo sem sub (URN do autor)"}
	}
	return out.Sub, nil
}

// Publish publica via POST /v2/ugcPosts (insumo §6.2 — body UGC padrão).
func (c *Client) Publish(ctx context.Context, content string, _ *domain.PostTarget, creds types.Credentials) (*types.PublishResult, error) {
	if !c.cfg.Configured() {
		return nil, &types.Error{Platform: "linkedin", Code: "not_configured",
			Message: "conector linkedin não configurado: defina LINKEDIN_CLIENT_ID e LINKEDIN_CLIENT_SECRET"}
	}
	author := creds.ExternalID
	if author == "" {
		return nil, &types.Error{Platform: "linkedin", Code: "no_author", Message: "conta linkedin sem author URN"}
	}
	if creds.AccessToken == "" {
		return nil, &types.Error{Platform: "linkedin", Code: "no_credentials", Message: "conta linkedin sem token"}
	}

	body := map[string]any{
		"author":           author,
		"lifecycleState":   "PUBLISHED",
		"specificContent": map[string]any{
			"com.linkedin.ugc.ShareContent": map[string]any{
				"shareCommentary":   map[string]any{"text": content},
				"shareMediaCategory": "NONE",
			},
		},
		"visibility": map[string]any{
			"com.linkedin.ugc.MemberNetworkVisibility": "PUBLIC",
		},
	}
	payload, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBase+"/v2/ugcPosts", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-RestLi-Protocol-Version", restliVersion)
	req.Header.Set("LinkedIn-Version", linkedInVer)

	var out struct {
		ID string `json:"id"`
	}
	if err := c.doJSON(req, &out); err != nil {
		return nil, err
	}
	if out.ID == "" {
		return nil, &types.Error{Platform: "linkedin", Code: "api_error", Message: "resposta sem id"}
	}
	return &types.PublishResult{
		ExternalID:   out.ID,
		PublishedURL: "https://www.linkedin.com/feed/update/" + out.ID,
	}, nil
}

// ValidateAccount valida o token com GET /v2/userinfo.
func (c *Client) ValidateAccount(ctx context.Context, creds types.Credentials) error {
	if creds.AccessToken == "" {
		return &types.Error{Platform: "linkedin", Code: "no_credentials", Message: "token ausente"}
	}
	_, err := c.fetchAuthorURN(ctx, creds.AccessToken)
	return err
}

// doJSON executa o request e decodifica; mapeia status HTTP em types.Error.
func (c *Client) doJSON(req *http.Request, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return json.NewDecoder(resp.Body).Decode(out)
	}

	var e struct {
		Message string `json:"message"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&e)

	switch resp.StatusCode {
	case http.StatusTooManyRequests, http.StatusInternalServerError,
		http.StatusBadGateway, http.StatusServiceUnavailable:
		return &types.Error{Platform: "linkedin", Code: "retryable", Message: e.Message, HTTPStatus: resp.StatusCode, Retryable: true}
	default:
		return &types.Error{Platform: "linkedin", Code: "api_error", Message: e.Message, HTTPStatus: resp.StatusCode}
	}
}
