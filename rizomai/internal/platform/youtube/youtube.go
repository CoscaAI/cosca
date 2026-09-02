// Package youtube integra o YouTube (Data API v3). OAuth Google com
// access_type=offline&prompt=consent (insumo §6.4 — senão não vem refresh).
// Publicação: upload RESUMABLE (metadata → PUT streaming dos bytes), Shorts
// auto-detect ≤3min, visibility via platformSpecificData.
package youtube

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

// Config são as credenciais de app (env YOUTUBE_CLIENT_ID/_SECRET).
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// Configured indica se o conector tem credenciais de app.
func (c Config) Configured() bool { return c.ClientID != "" }

const (
	apiBaseURL = "https://www.googleapis.com"
	tokenURL   = "https://oauth2.googleapis.com/token"
	authBaseURL = "https://accounts.google.com/o/oauth2/v2/auth"
	uploadScopes = "https://www.googleapis.com/auth/youtube.upload"
)

// Client é o conector do YouTube.
type Client struct {
	cfg      Config
	http     *http.Client
	apiBase  string
	tokenURL string
	authBase string
}

// New cria o conector.
func New(cfg Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: 60 * time.Second}, apiBase: apiBaseURL, tokenURL: tokenURL, authBase: authBaseURL}
}

// Name implementa types.Publisher.
func (c *Client) Name() string { return "youtube" }

// AuthURL monta a URL de autorização (offline + consent — insumo §6.4).
func (c *Client) AuthURL(state string, _ string) (string, error) {
	if !c.cfg.Configured() {
		return "", fmt.Errorf("conector youtube não configurado: defina YOUTUBE_CLIENT_ID e YOUTUBE_CLIENT_SECRET")
	}
	return c.authBase +
		"?client_id=" + c.cfg.ClientID +
		"&redirect_uri=" + c.cfg.RedirectURI +
		"&scope=" + uploadScopes +
		"&state=" + state +
		"&response_type=code&access_type=offline&prompt=consent", nil
}

// ExchangeCode troca code por tokens (access + refresh).
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
		Scope        string `json:"scope"`
	}
	if err := types.DoJSON(c.http, req, "youtube", &out); err != nil {
		return nil, err
	}
	return &types.Token{
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(out.ExpiresIn) * time.Second),
		Scope:        out.Scope,
	}, nil
}

// Publish faz upload resumable de um vídeo (insumo §3/§6.4): passo 1 metadata
// → Location; passo 2 PUT streaming dos bytes da mídia (nunca ReadAll).
func (c *Client) Publish(ctx context.Context, content string, target *domain.PostTarget, creds types.Credentials) (*types.PublishResult, error) {
	if !c.cfg.Configured() {
		return nil, &types.Error{Platform: "youtube", Code: "not_configured",
			Message: "conector youtube não configurado: defina YOUTUBE_CLIENT_ID e YOUTUBE_CLIENT_SECRET"}
	}
	if creds.AccessToken == "" {
		return nil, &types.Error{Platform: "youtube", Code: "no_credentials", Message: "conta youtube sem token"}
	}
	media := types.MediaURLsFromTarget(target)
	if len(media) == 0 {
		return nil, &types.Error{Platform: "youtube", Code: "no_media",
			Message: "YouTube exige vídeo — envie mediaUrls no platformSpecificData do target"}
	}

	title, _ := target.PlatformSpecificData["title"].(string)
	if title == "" {
		title = content
		if len(title) > 90 {
			title = title[:90]
		}
	}
	privacy := "public"
	if v, ok := target.PlatformSpecificData["visibility"].(string); ok && v != "" {
		privacy = v
	}

	// Passo 1: iniciar sessão de upload resumable (metadata).
	meta, _ := json.Marshal(map[string]any{
		"snippet": map[string]any{"title": title, "description": content},
		"status":  map[string]any{"privacyStatus": privacy},
	})
	initReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.apiBase+"/upload/youtube/v3/videos?uploadType=resumable&part=snippet,status", strings.NewReader(string(meta)))
	if err != nil {
		return nil, err
	}
	initReq.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	initReq.Header.Set("Content-Type", "application/json; charset=UTF-8")

	resp, err := c.http.Do(initReq)
	if err != nil {
		return nil, err
	}
	location := resp.Header.Get("Location")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || location == "" {
		return nil, &types.Error{Platform: "youtube", Code: "api_error", Message: "upload init falhou (HTTP " + resp.Status + ")", HTTPStatus: resp.StatusCode}
	}

	// Passo 2: PUT dos bytes (streaming do mediaURL para o upload — ADR-008 §1.2).
	mediaResp, err := http.Get(media[0])
	if err != nil {
		return nil, err
	}
	defer mediaResp.Body.Close()
	if mediaResp.StatusCode != http.StatusOK {
		return nil, &types.Error{Platform: "youtube", Code: "media_fetch_failed", Message: "falha ao baixar mídia " + media[0]}
	}

	upReq, err := http.NewRequestWithContext(ctx, http.MethodPut, location, mediaResp.Body)
	if err != nil {
		return nil, err
	}
	upReq.Header.Set("Content-Type", mediaResp.Header.Get("Content-Type"))

	upResp, err := c.http.Do(upReq)
	if err != nil {
		return nil, err
	}
	defer upResp.Body.Close()
	if upResp.StatusCode < 200 || upResp.StatusCode >= 300 {
		return nil, &types.Error{Platform: "youtube", Code: "upload_failed", Message: "upload falhou (HTTP " + upResp.Status + ")", HTTPStatus: upResp.StatusCode}
	}

	var out struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(upResp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.ID == "" {
		return nil, &types.Error{Platform: "youtube", Code: "api_error", Message: "resposta sem id"}
	}
	return &types.PublishResult{
		ExternalID:   out.ID,
		PublishedURL: "https://youtu.be/" + out.ID,
	}, nil
}

// ValidateAccount valida o token (GET /youtube/v3/channels?part=id&mine=true).
func (c *Client) ValidateAccount(ctx context.Context, creds types.Credentials) error {
	if creds.AccessToken == "" {
		return &types.Error{Platform: "youtube", Code: "no_credentials", Message: "token ausente"}
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		c.apiBase+"/youtube/v3/channels?part=id&mine=true", nil)
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	if err := types.DoJSON(c.http, req, "youtube", &struct{}{}); err != nil {
		if pe, ok := err.(*types.Error); ok && (pe.HTTPStatus == 401 || pe.HTTPStatus == 403) {
			return &types.Error{Platform: "youtube", Code: "invalid_token", Message: pe.Message, HTTPStatus: pe.HTTPStatus}
		}
		return err
	}
	return nil
}
