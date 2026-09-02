// Package tiktok integra o TikTok (Content Posting API). OAuth Open Platform;
// publicação em 2 passos: video/init (PULL_FROM_URL) → video publish, com
// privacyLevel e disclosure (insumo §2.1/§6.4 — app não auditada posta privado).
package tiktok

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

// Config são as credenciais de app (env TIKTOK_CLIENT_KEY/_SECRET).
type Config struct {
	ClientKey    string // client_key (TikTok chama assim)
	ClientSecret string
	RedirectURI  string
}

// Configured indica se o conector tem credenciais de app.
func (c Config) Configured() bool { return c.ClientKey != "" }

const (
	apiBaseURL = "https://open-api.tiktok.com"
	oauthScopes = "user.info.basic video.publish video.upload"
)

// Client é o conector do TikTok.
type Client struct {
	cfg      Config
	http     *http.Client
	apiBase  string
	tokenURL string
	authBase string
}

// New cria o conector.
func New(cfg Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: 30 * time.Second}, apiBase: apiBaseURL, tokenURL: apiBaseURL + "/v2/oauth/token/", authBase: "https://www.tiktok.com/v2/auth/authorize/"}
}

// Name implementa types.Publisher.
func (c *Client) Name() string { return "tiktok" }

// AuthURL monta a URL de autorização.
func (c *Client) AuthURL(state string, _ string) (string, error) {
	if !c.cfg.Configured() {
		return "", fmt.Errorf("conector tiktok não configurado: defina TIKTOK_CLIENT_KEY e TIKTOK_CLIENT_SECRET")
	}
	return c.authBase +
		"?client_key=" + c.cfg.ClientKey +
		"&redirect_uri=" + c.cfg.RedirectURI +
		"&scope=" + oauthScopes +
		"&state=" + state +
		"&response_type=code", nil
}

// ExchangeCode troca code por tokens.
func (c *Client) ExchangeCode(ctx context.Context, code, _, redirectURI string) (*types.Token, error) {
	body, _ := json.Marshal(map[string]any{
		"client_key": c.cfg.ClientKey, "client_secret": c.cfg.ClientSecret,
		"code": code, "redirect_uri": redirectURI, "grant_type": "authorization_code",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	var out struct {
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int    `json:"expires_in"`
			OpenID       string `json:"open_id"`
			Scope        string `json:"scope"`
		} `json:"data"`
	}
	if err := types.DoJSON(c.http, req, "tiktok", &out); err != nil {
		return nil, err
	}
	return &types.Token{
		AccessToken:  out.Data.AccessToken,
		RefreshToken: out.Data.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(out.Data.ExpiresIn) * time.Second),
		ExternalID:   out.Data.OpenID,
		Scope:        out.Data.Scope,
	}, nil
}

// Publish publica vídeo via init → publish (PULL_FROM_URL; streaming server-side).
func (c *Client) Publish(ctx context.Context, content string, target *domain.PostTarget, creds types.Credentials) (*types.PublishResult, error) {
	if !c.cfg.Configured() {
		return nil, &types.Error{Platform: "tiktok", Code: "not_configured",
			Message: "conector tiktok não configurado: defina TIKTOK_CLIENT_KEY e TIKTOK_CLIENT_SECRET"}
	}
	if creds.AccessToken == "" {
		return nil, &types.Error{Platform: "tiktok", Code: "no_credentials", Message: "conta tiktok sem token"}
	}
	media := types.MediaURLsFromTarget(target)
	if len(media) == 0 {
		return nil, &types.Error{Platform: "tiktok", Code: "no_media",
			Message: "TikTok exige vídeo — envie mediaUrls no platformSpecificData do target"}
	}

	privacy := "PUBLIC_TO_EVERYONE" // app auditada
	if v, ok := target.PlatformSpecificData["privacyLevel"].(string); ok && v != "" {
		privacy = v
	}

	// Passo 1: init.
	initBody, _ := json.Marshal(map[string]any{
		"post_info": map[string]any{
			"title":         content,
			"privacy_level": privacy,
			"disable_duet":  false, "disable_comment": false, "disable_stitch": false,
		},
		"source_info": map[string]any{"source": "PULL_FROM_URL", "video_url": media[0]},
	})
	initReq, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		c.apiBase+"/v2/post/publish/video/init/", strings.NewReader(string(initBody)))
	initReq.Header.Set("Content-Type", "application/json")
	initReq.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	var initRes struct {
		Data struct {
			PublishID string `json:"publish_id"`
			UploadURL string `json:"upload_url"`
		} `json:"data"`
	}
	if err := types.DoJSON(c.http, initReq, "tiktok", &initRes); err != nil {
		return nil, err
	}
	if initRes.Data.PublishID == "" {
		return nil, &types.Error{Platform: "tiktok", Code: "api_error", Message: "init sem publish_id"}
	}

	// Passo 2: publish.
	pubBody, _ := json.Marshal(map[string]any{
		"publish_id": initRes.Data.PublishID,
		"post_info": map[string]any{
			"title": content, "privacy_level": privacy,
			"disable_duet": false, "disable_comment": false, "disable_stitch": false,
		},
	})
	pubReq, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		c.apiBase+"/v2/post/publish/video/", strings.NewReader(string(pubBody)))
	pubReq.Header.Set("Content-Type", "application/json")
	pubReq.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	var pubRes struct {
		Data struct {
			PublishID string `json:"publish_id"`
			Status    string `json:"status"`
		} `json:"data"`
	}
	if err := types.DoJSON(c.http, pubReq, "tiktok", &pubRes); err != nil {
		return nil, err
	}

	return &types.PublishResult{
		ExternalID:   pubRes.Data.PublishID,
		PublishedURL: "https://www.tiktok.com/@" + creds.ExternalID + "/video/" + pubRes.Data.PublishID,
	}, nil
}

// ValidateAccount valida o token (GET /v2/user/info/).
func (c *Client) ValidateAccount(ctx context.Context, creds types.Credentials) error {
	if creds.AccessToken == "" {
		return &types.Error{Platform: "tiktok", Code: "no_credentials", Message: "token ausente"}
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		c.apiBase+"/v2/user/info/?fields=open_id&access_token="+creds.AccessToken, nil)
	if err := types.DoJSON(c.http, req, "tiktok", &struct{}{}); err != nil {
		if pe, ok := err.(*types.Error); ok && (pe.HTTPStatus == 401 || pe.HTTPStatus == 403) {
			return &types.Error{Platform: "tiktok", Code: "invalid_token", Message: pe.Message, HTTPStatus: pe.HTTPStatus}
		}
		return err
	}
	return nil
}
