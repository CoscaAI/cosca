// Validação de payload na borda da API (ADR-002 §1 / ADR-005 §1.5):
// o schema OpenAPI (openapi/rizomai.yaml) é a fonte da verdade do formato;
// esta validação manual espelha o contrato v0.1 e produz erros no formato
// VALIDATION_ERROR + details.fields (422).
package domain

import (
	"fmt"
	"strings"
	"time"
)

// ValidationError agrega erros por campo → 422 VALIDATION_ERROR, details.fields.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string { return "validation error" }

// Add registra um erro para o campo field.
func (e *ValidationError) Add(field, msg string) {
	if e.Fields == nil {
		e.Fields = map[string]string{}
	}
	e.Fields[field] = msg
}

// Empty indica se não houve nenhum erro.
func (e *ValidationError) Empty() bool { return len(e.Fields) == 0 }

// PlatformFromString valida e normaliza o valor de plataforma (enum da spec).
func PlatformFromString(s string) (Platform, error) {
	switch Platform(s) {
	case PlatformX, PlatformLinkedIn, PlatformTelegram,
		PlatformInstagram, PlatformFacebook, PlatformThreads, PlatformYouTube,
		PlatformTikTok, PlatformBluesky, PlatformReddit, PlatformPinterest,
		PlatformSnapchat, PlatformGoogleBusiness:
		return Platform(s), nil
	default:
		return "", fmt.Errorf("plataforma não suportada: %q", s)
	}
}

// isPrefixedID valida o padrão de ID prefixado (ex.: account_abc123).
func isPrefixedID(v, prefix string) bool {
	return len(v) > len(prefix) && strings.HasPrefix(v, prefix)
}

// --- Payloads de criação (espelham ProfileCreate / PostCreate da spec) ------

// ProfileCreatePayload é o corpo de POST /v1/profiles.
type ProfileCreatePayload struct {
	Name string `json:"name"`
}

// Validate aplica as regras do contrato (name: 1..100).
func (p *ProfileCreatePayload) Validate() *ValidationError {
	v := &ValidationError{}
	if strings.TrimSpace(p.Name) == "" {
		v.Add("name", "obrigatório")
	} else if len(p.Name) > 100 {
		v.Add("name", "máximo 100 caracteres")
	}
	if v.Empty() {
		return nil
	}
	return v
}

// PostTargetCreatePayload é 1 item de platforms[] em POST /v1/posts.
type PostTargetCreatePayload struct {
	Platform             string         `json:"platform"`
	AccountID            string         `json:"accountId"`
	PlatformSpecificData map[string]any `json:"platformSpecificData"`
}

// PostCreatePayload é o corpo de POST /v1/posts (espelha PostCreate da spec).
type PostCreatePayload struct {
	Content      string                     `json:"content"`
	Platforms    []PostTargetCreatePayload  `json:"platforms"`
	MediaURLs    []string                   `json:"mediaUrls"`
	ScheduledFor *time.Time                 `json:"scheduledFor"`
	Timezone     string                     `json:"timezone"`
}

// Validate aplica as regras do contrato v0.1:
//   - content 1..4000; mediaUrls opcional
//   - platforms 1..50, cada um com platform ∈ enum e accountId com prefixo
//   - scheduledFor opcional (date-time); timezone IANA quando presente
func (p *PostCreatePayload) Validate() *ValidationError {
	v := &ValidationError{}

	if strings.TrimSpace(p.Content) == "" {
		v.Add("content", "obrigatório")
	} else if len(p.Content) > 4000 {
		v.Add("content", "máximo 4000 caracteres")
	}

	if len(p.Platforms) == 0 {
		v.Add("platforms", "mínimo de 1 plataforma")
	} else if len(p.Platforms) > 50 {
		v.Add("platforms", "máximo de 50 plataformas")
	}
	for i, t := range p.Platforms {
		if _, err := PlatformFromString(t.Platform); err != nil {
			v.Add(fmt.Sprintf("platforms[%d].platform", i), err.Error())
		}
		if !isPrefixedID(t.AccountID, "account_") {
			v.Add(fmt.Sprintf("platforms[%d].accountId", i), "deve ter prefixo account_")
		}
	}

	if p.Timezone != "" {
		if _, err := time.LoadLocation(p.Timezone); err != nil {
			v.Add("timezone", "fuso horário IANA inválido")
		}
	}

	if v.Empty() {
		return nil
	}
	return v
}
