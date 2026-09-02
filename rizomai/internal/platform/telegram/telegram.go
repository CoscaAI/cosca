// Package telegram integra o Telegram (Bot API). SEM OAuth clássico (insumo
// §6.3): credencial = bot token `BOT_ID:SECRET` da CONTA, validado via getMe;
// destino = chat_id (bot deve ser admin do canal). Parse mode: subconjunto HTML.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rizomai/rizomai/internal/domain"
	"github.com/rizomai/rizomai/internal/platform/types"
)

// Config do conector (parse mode default; o token vem da conta — ADR-006).
type Config struct {
	ParseMode string // HTML (default) | MarkdownV2 | ""
}

// Configured: telegram não exige credencial de app global.
func (c Config) Configured() bool { return true }

const apiBaseURL = "https://api.telegram.org"

// Client é o conector do Telegram.
type Client struct {
	cfg  Config
	http *http.Client
}

// New cria o conector.
func New(cfg Config) *Client {
	if cfg.ParseMode == "" {
		cfg.ParseMode = "HTML"
	}
	return &Client{cfg: cfg, http: &http.Client{Timeout: 15 * time.Second}}
}

// Name implementa types.Publisher.
func (c *Client) Name() string { return "telegram" }

// Publish envia mensagem via sendMessage (chat_id da conta).
func (c *Client) Publish(ctx context.Context, content string, _ *domain.PostTarget, creds types.Credentials) (*types.PublishResult, error) {
	token := creds.AccessToken // bot token da conta (nunca em client)
	if token == "" {
		return nil, &types.Error{Platform: "telegram", Code: "no_credentials", Message: "conta telegram sem bot token"}
	}
	chatID := creds.ExternalID
	if chatID == "" {
		return nil, &types.Error{Platform: "telegram", Code: "no_chat", Message: "conta telegram sem chat_id"}
	}

	body, _ := json.Marshal(map[string]any{
		"chat_id":    chatID,
		"text":       content,
		"parse_mode": c.cfg.ParseMode,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		apiBaseURL+"/bot"+token+"/sendMessage", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	var out struct {
		OK     bool `json:"ok"`
		Result struct {
			MessageID int `json:"message_id"`
		} `json:"result"`
		Description string `json:"description"`
	}
	if err := c.doJSON(req, &out); err != nil {
		return nil, err
	}
	if !out.OK {
		return nil, telegramError(out.Description)
	}
	mid := out.Result.MessageID
	return &types.PublishResult{
		ExternalID:   fmt.Sprintf("%d", mid),
		PublishedURL: "https://t.me/c/" + strings.TrimPrefix(chatID, "-100") + "/" + fmt.Sprintf("%d", mid),
	}, nil
}

// ValidateAccount valida o bot token com getMe (ADR-006 §1.3 / insumo §6.3).
func (c *Client) ValidateAccount(ctx context.Context, creds types.Credentials) error {
	token := creds.AccessToken
	if token == "" {
		return &types.Error{Platform: "telegram", Code: "no_credentials", Message: "bot token ausente"}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		apiBaseURL+"/bot"+token+"/getMe", nil)
	if err != nil {
		return err
	}

	var out struct {
		OK          bool `json:"ok"`
		Description string `json:"description"`
	}
	if err := c.doJSON(req, &out); err != nil {
		return err
	}
	if !out.OK {
		return telegramError(out.Description)
	}
	return nil
}

// doJSON executa o request e decodifica a resposta JSON.
func (c *Client) doJSON(req *http.Request, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return json.NewDecoder(resp.Body).Decode(out)
	}

	switch resp.StatusCode {
	case http.StatusTooManyRequests, http.StatusInternalServerError,
		http.StatusBadGateway, http.StatusServiceUnavailable:
		return &types.Error{Platform: "telegram", Code: "retryable", Message: "HTTP " + resp.Status, HTTPStatus: resp.StatusCode, Retryable: true}
	default:
		return &types.Error{Platform: "telegram", Code: "api_error", Message: "HTTP " + resp.Status, HTTPStatus: resp.StatusCode}
	}
}

// telegramError mapeia códigos nativos do Telegram (insumo §4.4):
// message too long, chat not found, bot was blocked — nenhum é retryable.
func telegramError(desc string) *types.Error {
	code := "api_error"
	msg := desc
	switch {
	case strings.Contains(desc, "message is too long"):
		code = "message_too_long"
	case strings.Contains(desc, "chat not found"):
		code = "chat_not_found"
	case strings.Contains(desc, "was blocked"):
		code = "bot_blocked"
	}
	return &types.Error{Platform: "telegram", Code: code, Message: msg}
}
