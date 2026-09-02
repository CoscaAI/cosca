// Package instagram integra o Instagram Business (Graph API). OAuth via
// Facebook Login + ig_exchange_token (short-lived → long-lived 60d — insumo
// §1.3/§6.4). Publicação em 2 passos (criar container → media_publish) com
// verificação pós-erro do 2207051 ("bloqueado mas pode ter publicado").
package instagram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rizomai/rizomai/internal/domain"
	"github.com/rizomai/rizomai/internal/platform/types"
)

// Config são as credenciais de app (env INSTAGRAM_CLIENT_ID/_SECRET).
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// Configured indica se o conector tem credenciais de app.
func (c Config) Configured() bool { return c.ClientID != "" }

const (
	graphBaseURL = "https://graph.instagram.com" // media e ig_exchange_token
	fbOAuthURL   = "https://www.facebook.com/v20.0/dialog/oauth"
	fbTokenURL   = "https://graph.facebook.com/v20.0/oauth/access_token"
	oauthScopes  = "instagram_business_basic instagram_business_content_publish instagram_business_manage_comments"
)

// Client é o conector do Instagram.
type Client struct {
	cfg      Config
	http     *http.Client
	apiBase  string // graph.instagram.com (media)
	tokenURL string // graph.facebook.com oauth
	authBase string // facebook dialog
}

// New cria o conector.
func New(cfg Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: 20 * time.Second}, apiBase: graphBaseURL, tokenURL: fbTokenURL, authBase: fbOAuthURL}
}

// Name implementa types.Publisher.
func (c *Client) Name() string { return "instagram" }

// AuthURL monta a URL de autorização (Facebook Login p/ IG Business).
func (c *Client) AuthURL(state string, _ string) (string, error) {
	if !c.cfg.Configured() {
		return "", fmt.Errorf("conector instagram não configurado: defina INSTAGRAM_CLIENT_ID e INSTAGRAM_CLIENT_SECRET")
	}
	return c.authBase +
		"?client_id=" + c.cfg.ClientID +
		"&redirect_uri=" + c.cfg.RedirectURI +
		"&scope=" + oauthScopes +
		"&state=" + state +
		"&response_type=code", nil
}

// ExchangeCode troca code por short-lived e depois ig_exchange_token
// (long-lived 60d — insumo §6.4); o retorno inclui o ig user_id.
func (c *Client) ExchangeCode(ctx context.Context, code, _, redirectURI string) (*types.Token, error) {
	form := "client_id=" + c.cfg.ClientID + "&client_secret=" + c.cfg.ClientSecret +
		"&code=" + code + "&redirect_uri=" + redirectURI

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var short struct {
		AccessToken string `json:"access_token"`
	}
	if err := types.DoJSON(c.http, req, "instagram", &short); err != nil {
		return nil, err
	}

	// Long-lived (60d) via ig_exchange_token.
	longReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.apiBase+"/access_token?grant_type=ig_exchange_token&client_secret="+c.cfg.ClientSecret+
			"&access_token="+short.AccessToken, nil)
	if err != nil {
		return nil, err
	}
	var long struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		UserID      int64  `json:"user_id"`
	}
	if err := types.DoJSON(c.http, longReq, "instagram", &long); err != nil {
		return nil, err
	}

	return &types.Token{
		AccessToken:  long.AccessToken,
		ExpiresAt:    time.Now().Add(time.Duration(long.ExpiresIn) * time.Second),
		ExternalID:   strconv.FormatInt(long.UserID, 10),
		Scope:        "instagram_business",
	}, nil
}

// Publish publica imagem/vídeo via Graph: cria container e media_publish.
func (c *Client) Publish(ctx context.Context, content string, target *domain.PostTarget, creds types.Credentials) (*types.PublishResult, error) {
	if !c.cfg.Configured() {
		return nil, &types.Error{Platform: "instagram", Code: "not_configured",
			Message: "conector instagram não configurado: defina INSTAGRAM_CLIENT_ID e INSTAGRAM_CLIENT_SECRET"}
	}
	if creds.AccessToken == "" {
		return nil, &types.Error{Platform: "instagram", Code: "no_credentials", Message: "conta instagram sem token"}
	}
	igID := creds.ExternalID
	if igID == "" {
		return nil, &types.Error{Platform: "instagram", Code: "no_account", Message: "conta instagram sem ig user id"}
	}
	media := types.MediaURLsFromTarget(target)
	if len(media) == 0 {
		return nil, &types.Error{Platform: "instagram", Code: "no_media",
			Message: "Instagram exige imagem/vídeo — envie mediaUrls no platformSpecificData do target"}
	}

	// Passo 1: criar media container (insumo §6.4 — IDs 17+ dígitos como string).
	createBody, _ := json.Marshal(map[string]any{
		"image_url":   media[0],
		"caption":     content,
		"access_token": creds.AccessToken,
	})
	createReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBase+"/"+igID+"/media", strings.NewReader(string(createBody)))
	createReq.Header.Set("Content-Type", "application/json")
	var created struct {
		ID string `json:"id"`
	}
	if err := types.DoJSON(c.http, createReq, "instagram", &created); err != nil {
		return nil, err
	}

	// Passo 2: publicar.
	pubBody, _ := json.Marshal(map[string]any{"creation_id": created.ID, "access_token": creds.AccessToken})
	pubReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBase+"/"+igID+"/media_publish", strings.NewReader(string(pubBody)))
	pubReq.Header.Set("Content-Type", "application/json")
	var published struct {
		ID string `json:"id"`
	}
	if err := types.DoJSON(c.http, pubReq, "instagram", &published); err != nil {
		// 2207051: "bloqueado mas pode ter publicado" (insumo §4.4) — verificar
		// antes de marcar falha (anti-duplicado no retry).
		if isAntiSpamErr(err) {
			if c.mediaExists(ctx, igID, creds.AccessToken) {
				return &types.PublishResult{ExternalID: "ig-sim-" + created.ID,
					PublishedURL: "https://www.instagram.com/p/" + created.ID}, nil
			}
		}
		return nil, err
	}

	return &types.PublishResult{
		ExternalID:   published.ID,
		PublishedURL: "https://www.instagram.com/p/" + published.ID,
	}, nil
}

// mediaExists verifica se algo foi publicado após erro 2207051.
func (c *Client) mediaExists(ctx context.Context, igID, token string) bool {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		c.apiBase+"/"+igID+"/media?fields=id&limit=1&access_token="+token, nil)
	var out struct {
		Data []struct{ ID string `json:"id"` } `json:"data"`
	}
	return types.DoJSON(c.http, req, "instagram", &out) == nil && len(out.Data) > 0
}

func isAntiSpamErr(err error) bool {
	if err == nil {
		return false
	}
	var pe *types.Error
	if errors.As(err, &pe) {
		return pe.NativeCode == 2207051 || strings.Contains(pe.Message, "2207051")
	}
	return strings.Contains(err.Error(), "2207051")
}

// ValidateAccount valida o token (alarme precoce — ADR-006 §1.3).
func (c *Client) ValidateAccount(ctx context.Context, creds types.Credentials) error {
	if creds.AccessToken == "" {
		return &types.Error{Platform: "instagram", Code: "no_credentials", Message: "token ausente"}
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		c.apiBase+"/me?fields=id,username&access_token="+creds.AccessToken, nil)
	if err := types.DoJSON(c.http, req, "instagram", &struct{}{}); err != nil {
		if pe, ok := err.(*types.Error); ok && (pe.HTTPStatus == 400 || pe.HTTPStatus == 401) {
			return &types.Error{Platform: "instagram", Code: "invalid_token", Message: pe.Message, HTTPStatus: pe.HTTPStatus}
		}
		return err
	}
	return nil
}
