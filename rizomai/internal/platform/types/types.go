// Package types define os tipos base da camada de plataforma (sem dependência
// dos conectores) — quebra o ciclo platform → conector → platform.
//
// Os conectores (x, linkedin, telegram) importam ESTE pacote; o pacote
// internal/platform re-exporta os tipos para o resto do código.
package types

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/rizomai/rizomai/internal/domain"
)

// Credentials são as credenciais da CONTA (descriptografadas do banco —
// ADR-006 §1.2: tokens nunca trafegam em claro pela API).
type Credentials struct {
	AccessToken  string
	RefreshToken string
	ExternalID   string // chat_id (telegram), author URN (linkedin), user id (x)
	ExpiresAt    *time.Time
}

// PublishResult é o resultado de uma publicação bem-sucedida.
type PublishResult struct {
	ExternalID   string // id nativo do post na plataforma
	PublishedURL string
}

// Publisher publica conteúdo e valida contas.
type Publisher interface {
	// Publish publica o post do target e devolve o id externo.
	Publish(ctx context.Context, content string, target *domain.PostTarget, creds Credentials) (*PublishResult, error)
	// ValidateAccount valida as credenciais da conta (ex.: getMe do Telegram).
	ValidateAccount(ctx context.Context, creds Credentials) error
	// Name devolve o nome da plataforma (x|linkedin|telegram).
	Name() string
}

// Token é o resultado da troca de code por token no OAuth.
type Token struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	ExternalID   string // user id (x) / author URN (linkedin)
	Scope        string
}

// OAuthProvider é a interface dos conectores com OAuth server-side (ADR-006).
// Telegram (bot token) não implementa — usa Credentials direto.
type OAuthProvider interface {
	Publisher
	// AuthURL monta a URL de autorização. codeVerifier só para PKCE (X).
	AuthURL(state string, codeVerifier string) (string, error)
	// ExchangeCode troca o code (do callback) por tokens.
	ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI string) (*Token, error)
}

// Error é um erro de plataforma com código tipado (ADR-007 §1.3 / insumo §4.4).
type Error struct {
	Platform   string `json:"platform"`
	Code       string `json:"code"` // invalid_grant, duplicate, message_too_long, rate_limited...
	Message    string `json:"message"`
	HTTPStatus int    `json:"httpStatus,omitempty"`
	Retryable  bool   `json:"-"` // 429/500/502/503 → retry com backoff (ADR-003)
	NativeCode int    `json:"nativeCode,omitempty"` // code numérico nativo (ex.: 2207051 do Instagram)
}

func (e *Error) Error() string {
	return "platform " + e.Platform + ": [" + e.Code + "] " + e.Message
}

// DoJSON executa o request e decodifica a resposta JSON, mapeando status HTTP
// para *Error (429/500/502/503 → Retryable — ADR-003). Compartilhado pelos
// conectores (mesmo comportamento do doJSON de X/LinkedIn/Telegram).
func DoJSON(httpClient *http.Client, req *http.Request, platformName string, out any) error {
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return json.NewDecoder(resp.Body).Decode(out)
	}

	var e struct {
		Message string `json:"message"`
		Detail  string `json:"detail"`
		Title   string `json:"title"`
		Err     struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&e)
	msg := e.Message
	if msg == "" {
		msg = e.Detail
	}
	if msg == "" {
		msg = e.Err.Message
	}
	if msg == "" {
		msg = e.Title
	}
	if msg == "" {
		msg = "HTTP " + resp.Status
	}

	switch resp.StatusCode {
	case http.StatusTooManyRequests, http.StatusInternalServerError,
		http.StatusBadGateway, http.StatusServiceUnavailable:
		return &Error{Platform: platformName, Code: "retryable", Message: msg, HTTPStatus: resp.StatusCode, Retryable: true, NativeCode: e.Err.Code}
	default:
		return &Error{Platform: platformName, Code: "api_error", Message: msg, HTTPStatus: resp.StatusCode, NativeCode: e.Err.Code}
	}
}

// MediaURLsFromTarget lê mediaUrls do platformSpecificData do target
// (as plataformas de mídia exigem um URL de imagem/vídeo — insumo §3).
func MediaURLsFromTarget(t *domain.PostTarget) []string {
	if t == nil {
		return nil
	}
	raw, ok := t.PlatformSpecificData["mediaUrls"].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}
