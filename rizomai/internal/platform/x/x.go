// Package x integra o Twitter/X (API v2). OAuth 2.0 PKCE S256 (insumo §6.1):
// verifier embutido no state (`<state>-cv_<verifier>`), access ~2h + refresh.
package x

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/rizomai/rizomai/internal/domain"
	"github.com/rizomai/rizomai/internal/platform/types"
)

// Config são as credenciais de app (env X_CLIENT_ID/X_CLIENT_SECRET).
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// Configured indica se o conector tem credenciais de app.
func (c Config) Configured() bool { return c.ClientID != "" }

const (
	authBaseURL  = "https://twitter.com/i/oauth2/authorize"
	tokenURL     = "https://api.twitter.com/2/oauth2/token"
	apiBaseURL   = "https://api.twitter.com"
	oauthScopes  = "tweet.read tweet.write users.read offline.access media.write"
)

// Client é o conector do X.
type Client struct {
	cfg   Config
	http  *http.Client
	apiBase string // base da API v2 (override em testes)
	tokenURL string
	authBase string
}

// New cria o conector com as credenciais de app.
func New(cfg Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: 15 * time.Second}, apiBase: apiBaseURL, tokenURL: tokenURL, authBase: authBaseURL}
}

// Name implementa types.Publisher.
func (c *Client) Name() string { return "x" }

// AuthURL monta a URL de autorização com PKCE S256 (ADR-006 §1.1 / insumo §6.1).
// O code_verifier vai embutido no state para o callback: <state>-cv_<verifier>.
func (c *Client) AuthURL(state string, codeVerifier string) (string, error) {
	if !c.cfg.Configured() {
		return "", fmt.Errorf("conector x não configurado: defina X_CLIENT_ID e X_CLIENT_SECRET")
	}
	if codeVerifier == "" {
		return "", fmt.Errorf("x: code_verifier ausente (o broker deve gerar via NewCodeVerifier)")
	}
	challenge := pkceChallenge(codeVerifier)

	u := c.authBase +
		"?response_type=code" +
		"&client_id=" + c.cfg.ClientID +
		"&redirect_uri=" + c.cfg.RedirectURI +
		"&scope=" + oauthScopes +
		"&state=" + state + "-cv_" + codeVerifier +
		"&code_challenge=" + challenge +
		"&code_challenge_method=S256"
	return u, nil
}

// ExchangeCode troca o code por tokens (grant authorization_code + PKCE).
func (c *Client) ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI string) (*types.Token, error) {
	if codeVerifier == "" {
		return nil, fmt.Errorf("x: code_verifier ausente (state inválido ou expirado)")
	}

	form := "grant_type=authorization_code&code=" + code +
		"&redirect_uri=" + redirectURI + "&code_verifier=" + codeVerifier

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, bytes.NewBufferString(form))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+basicAuth(c.cfg.ClientID, c.cfg.ClientSecret))

	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := c.doJSON(req, &out); err != nil {
		return nil, err
	}

	return &types.Token{
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(out.ExpiresIn) * time.Second),
		Scope:        out.Scope,
	}, nil
}

// Publish publica um tweet via POST /2/tweets (Bearer = access token do usuário).
func (c *Client) Publish(ctx context.Context, content string, target *domain.PostTarget, creds types.Credentials) (*types.PublishResult, error) {
	if !c.cfg.Configured() {
		return nil, &types.Error{Platform: "x", Code: "not_configured",
			Message: "conector x não configurado: defina X_CLIENT_ID e X_CLIENT_SECRET"}
	}
	if creds.AccessToken == "" {
		return nil, &types.Error{Platform: "x", Code: "no_credentials", Message: "conta x sem token"}
	}

	body, _ := json.Marshal(map[string]any{"text": content})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBase+"/2/tweets", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	var out struct {
		Data struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"data"`
		Title string `json:"title"`
		Detail string `json:"detail"`
	}
	if err := c.doJSON(req, &out); err != nil {
		return nil, err
	}
	if out.Data.ID == "" {
		// erro tipado: invalid_grant (revogado → reconectar), duplicate (sem retry)
		return nil, &types.Error{Platform: "x", Code: "api_error", Message: out.Detail}
	}
	return &types.PublishResult{
		ExternalID:   out.Data.ID,
		PublishedURL: "https://x.com/i/status/" + out.Data.ID,
	}, nil
}

// ValidateAccount valida o token com GET /2/users/me (Alarme precoce — ADR-006 §1.3).
func (c *Client) ValidateAccount(ctx context.Context, creds types.Credentials) error {
	if creds.AccessToken == "" {
		return &types.Error{Platform: "x", Code: "no_credentials", Message: "token ausente"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBase+"/2/users/me", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)

	var out struct {
		Data struct {
			ID       string `json:"id"`
			Username string `json:"username"`
		} `json:"data"`
		Detail string `json:"detail"`
	}
	if err := c.doJSON(req, &out); err != nil {
		// 401/403 = token revogado/insuficiente → reconectar (alarme precoce — ADR-006 §1.3)
		if pe, ok := err.(*types.Error); ok &&
			(pe.HTTPStatus == http.StatusUnauthorized || pe.HTTPStatus == http.StatusForbidden) {
			return &types.Error{Platform: "x", Code: "invalid_token", Message: pe.Message, HTTPStatus: pe.HTTPStatus}
		}
		return err
	}
	if out.Data.ID == "" {
		return &types.Error{Platform: "x", Code: "invalid_token", Message: out.Detail, Retryable: false}
	}
	return nil
}

// doJSON executa o request e decodifica a resposta; mapeia status HTTP em
// types.Error (429/5xx → Retryable).
func (c *Client) doJSON(req *http.Request, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return json.NewDecoder(resp.Body).Decode(out)
	}

	code := strconv.Itoa(resp.StatusCode)
	switch resp.StatusCode {
	case http.StatusTooManyRequests, http.StatusInternalServerError,
		http.StatusBadGateway, http.StatusServiceUnavailable:
		return &types.Error{Platform: "x", Code: "retryable_" + code, Message: "HTTP " + code, HTTPStatus: resp.StatusCode, Retryable: true}
	default:
		return &types.Error{Platform: "x", Code: "api_error_" + code, Message: "HTTP " + code, HTTPStatus: resp.StatusCode}
	}
}

// NewCodeVerifier gera um code_verifier PKCE (43–128 chars, alta entropia).
func NewCodeVerifier() (string, error) {
	b := make([]byte, 48)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// pkceChallenge computa o code_challenge S256.
func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func basicAuth(id, secret string) string {
	return base64.StdEncoding.EncodeToString([]byte(id + ":" + secret))
}
