// Package reddit integra o Reddit (API v1). OAuth script flow (app-only com
// password grant — insumo §6.5): sem browser. User-Agent DESCRITIVO
// obrigatório (insumo §4.4). Publicação: POST /api/submit (self post).
package reddit

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/rizomai/rizomai/internal/domain"
	"github.com/rizomai/rizomai/internal/platform/types"
)

// Config são as credenciais (env REDDIT_CLIENT_ID/_SECRET/_USERNAME/_PASSWORD/
// _USER_AGENT). Fluxo script: sem browser OAuth — credenciais diretas.
type Config struct {
	ClientID     string
	ClientSecret string
	Username     string
	Password     string
	UserAgent    string
}

// Configured indica se o conector tem credenciais de app.
func (c Config) Configured() bool { return c.ClientID != "" }

const (
	authURL   = "https://www.reddit.com/api/v1/access_token"
	apiBaseURL = "https://oauth.reddit.com"
	defaultUA  = "rizomai/0.1 (bot de publicação multi-rede)"
)

// Client é o conector do Reddit.
type Client struct {
	cfg      Config
	http     *http.Client
	apiBase  string
	authURL  string
}

// New cria o conector.
func New(cfg Config) *Client {
	if cfg.UserAgent == "" {
		cfg.UserAgent = defaultUA
	}
	return &Client{cfg: cfg, http: &http.Client{Timeout: 20 * time.Second}, apiBase: apiBaseURL, authURL: authURL}
}

// Name implementa types.Publisher.
func (c *Client) Name() string { return "reddit" }

// accessToken obtém token via password grant (script flow).
func (c *Client) accessToken(ctx context.Context) (string, error) {
	form := "grant_type=password&username=" + c.cfg.Username + "&password=" + c.cfg.Password
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.authURL, strings.NewReader(form))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+basicAuth(c.cfg.ClientID, c.cfg.ClientSecret))
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	var out struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := types.DoJSON(c.http, req, "reddit", &out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", &types.Error{Platform: "reddit", Code: "invalid_token", Message: "login reddit falhou: " + out.Error}
	}
	return out.AccessToken, nil
}

// Publish envia um self post: POST /api/submit (insumo §2.1 — UA obrigatório).
func (c *Client) Publish(ctx context.Context, content string, target *domain.PostTarget, _ types.Credentials) (*types.PublishResult, error) {
	if !c.cfg.Configured() {
		return nil, &types.Error{Platform: "reddit", Code: "not_configured",
			Message: "conector reddit não configurado: defina REDDIT_CLIENT_ID/_SECRET/_USERNAME/_PASSWORD"}
	}
	subreddit, _ := target.PlatformSpecificData["subreddit"].(string)
	if subreddit == "" {
		return nil, &types.Error{Platform: "reddit", Code: "no_subreddit",
			Message: "defina subreddit no platformSpecificData do target (ex.: {subreddit: tecnologia})"}
	}
	title, _ := target.PlatformSpecificData["title"].(string)
	if title == "" {
		title = content
		if len(title) > 200 {
			title = title[:200]
		}
	}

	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}

	form := "sr=" + subreddit + "&title=" + title + "&text=" + content + "&kind=self&api_type=json"
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBase+"/api/submit", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	var out struct {
		JSON struct {
			Errors [][]string `json:"errors"`
			Data   struct {
				ID string `json:"id"`
			} `json:"data"`
		} `json:"json"`
	}
	if err := types.DoJSON(c.http, req, "reddit", &out); err != nil {
		return nil, err
	}
	if len(out.JSON.Errors) > 0 {
		// ex.: RATELIMIT (retryable) ou NO_LINKS/forbidden (definitivo — insumo §4.4).
		code := out.JSON.Errors[0][0]
		msg := out.JSON.Errors[0][1]
		rr := strings.EqualFold(code, "ratelimit") || strings.Contains(strings.ToLower(msg), "slow down")
		return nil, &types.Error{Platform: "reddit", Code: code, Message: msg, Retryable: rr}
	}
	if out.JSON.Data.ID == "" {
		return nil, &types.Error{Platform: "reddit", Code: "api_error", Message: "submit sem id"}
	}
	return &types.PublishResult{
		ExternalID:   out.JSON.Data.ID,
		PublishedURL: "https://www.reddit.com/r/" + subreddit + "/comments/" + out.JSON.Data.ID,
	}, nil
}

// ValidateAccount valida o login (GET /api/v1/me).
func (c *Client) ValidateAccount(ctx context.Context, _ types.Credentials) error {
	token, err := c.accessToken(ctx)
	if err != nil {
		return err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBase+"/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	return types.DoJSON(c.http, req, "reddit", &struct{}{})
}

func basicAuth(id, secret string) string {
	return base64.StdEncoding.EncodeToString([]byte(id + ":" + secret))
}
