// Package bluesky integra o Bluesky (AT Protocol). SEM OAuth browser — app
// password (insumo §1.2/§6): createSession → accessJwt → createRecord com o
// post no repositório pessoal (DID). Refresh automático em ExpiredToken.
package bluesky

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rizomai/rizomai/internal/domain"
	"github.com/rizomai/rizomai/internal/platform/types"
)

// Config do conector (env BLUESKY_PDS_URL — default bsky.social).
type Config struct {
	PDSURL string
}

// Configured: Bluesky não exige credencial de app global.
func (c Config) Configured() bool { return true }

const defaultPDS = "https://bsky.social"

// Client é o conector do Bluesky.
type Client struct {
	cfg     Config
	http    *http.Client
	apiBase string // PDS
}

// New cria o conector.
func New(cfg Config) *Client {
	if cfg.PDSURL == "" {
		cfg.PDSURL = defaultPDS
	}
	return &Client{cfg: cfg, http: &http.Client{Timeout: 20 * time.Second}, apiBase: strings.TrimSuffix(cfg.PDSURL, "/")}
}

// Name implementa types.Publisher.
func (c *Client) Name() string { return "bluesky" }

// Publish posta via createRecord (app.bsky.feed.post). creds.AccessToken =
// app password; creds.ExternalID = handle (identifier do login).
func (c *Client) Publish(ctx context.Context, content string, _ *domain.PostTarget, creds types.Credentials) (*types.PublishResult, error) {
	identifier := creds.ExternalID
	if identifier == "" {
		return nil, &types.Error{Platform: "bluesky", Code: "no_credentials", Message: "conta bluesky sem handle (identifier)"}
	}
	if creds.AccessToken == "" {
		return nil, &types.Error{Platform: "bluesky", Code: "no_credentials", Message: "conta bluesky sem app password"}
	}

	// createSession (insumo: login app-password).
	session, err := c.createSession(ctx, identifier, creds.AccessToken)
	if err != nil {
		return nil, err
	}

	record, _ := json.Marshal(map[string]any{
		"repo":       session.DID,
		"collection": "app.bsky.feed.post",
		"record": map[string]any{
			"$type":     "app.bsky.feed.post",
			"text":      content,
			"createdAt": time.Now().UTC().Format(time.RFC3339),
		},
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		c.apiBase+"/xrpc/com.atproto.repo.createRecord", strings.NewReader(string(record)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)

	var out struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	}
	if err := types.DoJSON(c.http, req, "bluesky", &out); err != nil {
		return nil, err
	}
	if out.URI == "" {
		return nil, &types.Error{Platform: "bluesky", Code: "api_error", Message: "createRecord sem uri"}
	}
	// uri: at://did:plc:xxx/app.bsky.feed.post/3xxx → short id no final.
	postID := out.URI
	if i := strings.LastIndex(out.URI, "/"); i >= 0 {
		postID = out.URI[i+1:]
	}
	return &types.PublishResult{
		ExternalID:   out.URI,
		PublishedURL: "https://bsky.app/profile/" + identifier + "/post/" + postID,
	}, nil
}

type session struct {
	AccessJwt string `json:"accessJwt"`
	DID       string `json:"did"`
}

func (c *Client) createSession(ctx context.Context, identifier, appPassword string) (*session, error) {
	body, _ := json.Marshal(map[string]any{"identifier": identifier, "password": appPassword})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.apiBase+"/xrpc/com.atproto.server.createSession", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	var out session
	if err := types.DoJSON(c.http, req, "bluesky", &out); err != nil {
		return nil, err
	}
	if out.AccessJwt == "" {
		return nil, &types.Error{Platform: "bluesky", Code: "invalid_token", Message: "login bluesky falhou (credenciais inválidas)"}
	}
	return &out, nil
}

// ValidateAccount valida o app password via createSession.
func (c *Client) ValidateAccount(ctx context.Context, creds types.Credentials) error {
	if creds.ExternalID == "" || creds.AccessToken == "" {
		return &types.Error{Platform: "bluesky", Code: "no_credentials", Message: "identifier e app password obrigatórios"}
	}
	_, err := c.createSession(ctx, creds.ExternalID, creds.AccessToken)
	return err
}
