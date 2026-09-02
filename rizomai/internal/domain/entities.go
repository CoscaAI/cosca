// Package domain contém as entidades centrais e regras do RIZOMAI (ADR-004).
//
// Modelo de publicação (ADR-007) — o coração do produto:
//
//	Post ──1:N──> PostTarget ──1:N──> PublishAttempt
//
// O status agregado de Post é uma FUNÇÃO DERIVADA do status dos targets
// (materializado para leitura, atualizado transacionalmente — nunca editado
// à mão). Retry e unpublish são SEMPRE por target, nunca do post inteiro.
package domain

import "time"

// Platform é a enumeração de redes suportadas.
// MVP (ADR-006 §1): x, linkedin, telegram. Expansão: +10 redes (13 no total).
type Platform string

const (
	PlatformX        Platform = "x"
	PlatformLinkedIn Platform = "linkedin"
	PlatformTelegram Platform = "telegram"

	PlatformInstagram      Platform = "instagram"
	PlatformFacebook       Platform = "facebook"
	PlatformThreads        Platform = "threads"
	PlatformYouTube        Platform = "youtube"
	PlatformTikTok         Platform = "tiktok"
	PlatformBluesky        Platform = "bluesky"
	PlatformReddit         Platform = "reddit"
	PlatformPinterest      Platform = "pinterest"
	PlatformSnapchat       Platform = "snapchat"
	PlatformGoogleBusiness Platform = "googlebusiness"
)

// PostStatus é o status AGREGADO de um Post (ADR-007 §1).
type PostStatus string

const (
	PostStatusScheduled  PostStatus = "scheduled"  // agendado, ainda não due
	PostStatusPublishing PostStatus = "publishing" // fan-out em andamento
	PostStatusPublished  PostStatus = "published"  // TODOS os targets publicados
	PostStatusPartial    PostStatus = "partial"    // alguns publicados, outros falharam (o caso mais comum)
	PostStatusFailed     PostStatus = "failed"     // todos falharam
	PostStatusCancelled  PostStatus = "cancelled"  // cancelado antes da publicação
)

// TargetStatus é o status individual de um PostTarget.
type TargetStatus string

const (
	TargetStatusPending    TargetStatus = "pending"
	TargetStatusScheduled  TargetStatus = "scheduled"
	TargetStatusPublishing TargetStatus = "publishing"
	TargetStatusPublished  TargetStatus = "published"
	TargetStatusFailed     TargetStatus = "failed"
	TargetStatusSkipped    TargetStatus = "skipped"
)

// PublishOutcome é o resultado de uma tentativa de publicação (PublishAttempt).
// Base ADR-007 (success|failed|timeout) + rate_limited (429 é retryable —
// ADR-003) e skipped (target não processado, ex.: bloqueio por regra).
type PublishOutcome string

const (
	OutcomeSuccess     PublishOutcome = "success"
	OutcomeFailed      PublishOutcome = "failed"
	OutcomeTimeout     PublishOutcome = "timeout"
	OutcomeRateLimited PublishOutcome = "rate_limited"
	OutcomeSkipped     PublishOutcome = "skipped"
)

// TokenStatus expõe APENAS o estado do token de rede social na API.
// O token em si nunca é serializado (ADR-006 §1.2).
type TokenStatus string

const (
	TokenStatusOK             TokenStatus = "ok"
	TokenStatusExpired        TokenStatus = "expired"
	TokenStatusRevoked        TokenStatus = "revoked"
	TokenStatusNeedsAttention TokenStatus = "needs_attention"
)

// Profile é o tenant lógico que agrupa contas conectadas
// (hierarquia: Team → Profile → Account — ADR-002).
// TeamID não é exposto no contrato (o team é inferido da API key).
type Profile struct {
	ID        string    `json:"id"`
	TeamID    string    `json:"-"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// SocialAccount é uma conexão OAuth ativa de um Profile (ADR-006).
// Apenas tokenStatus e metadados de escopo são expostos via API.
// Tokens ficam CRIPTOGRAFADOS (AES-256-GCM) nas colunas Encrypted* — nunca
// serializados em JSON (json:"-").
type SocialAccount struct {
	ID             string         `json:"id"`
	ProfileID      string         `json:"profileId"`
	Platform       Platform       `json:"platform"`
	DisplayName    string         `json:"displayName,omitempty"`
	PlatformUserID string         `json:"platformUserId,omitempty"`
	TokenStatus    TokenStatus    `json:"tokenStatus"`
	Settings       map[string]any `json:"settings,omitempty"` // JSONB validado na API (ADR-002/005)
	ConnectedAt    time.Time      `json:"connectedAt"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`

	EncryptedToken        []byte     `json:"-"` // access token AES-256-GCM
	RefreshTokenEncrypted []byte     `json:"-"` // refresh token AES-256-GCM
	ExpiresAt             *time.Time `json:"expiresAt,omitempty"`
	ExternalIdentifier    string     `json:"externalIdentifier,omitempty"` // chat_id / author URN / user id
	TokenScope            string     `json:"tokenScope,omitempty"`
}

// OAuthState é o state do OAuth server-side (ADR-006 §1.1): guarda o
// code_verifier PKCE e o destino (profile) para o callback validar.
type OAuthState struct {
	ID           string    `json:"id"`
	TeamID       string    `json:"teamId"`
	Platform     Platform  `json:"platform"`
	State        string    `json:"state"`
	CodeVerifier string    `json:"-"`
	RedirectURI  string    `json:"-"`
	ProfileID    string    `json:"profileId,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

// WebhookDelivery é o log append-only de cada tentativa de entrega (ADR-009 §1.5).
type WebhookDelivery struct {
	ID         string    `json:"id"`
	WebhookID  string    `json:"webhookId"`
	EventID    string    `json:"eventId"` // MESMO id em todos os retries
	EventType  string    `json:"eventType"`
	Payload    []byte    `json:"-"`
	Status     string    `json:"status"` // success | failed
	HTTPStatus int       `json:"httpStatus,omitempty"`
	Error      string    `json:"error,omitempty"`
	Attempts   int       `json:"attempts"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Post é a unidade de conteúdo que sofre fan-out (ADR-007).
type Post struct {
	ID           string        `json:"id"`
	ProfileID    string        `json:"profileId"`
	Content      string        `json:"content"`
	MediaURLs    []string      `json:"mediaUrls,omitempty"`
	Platforms    []PostTarget  `json:"platforms"`
	ScheduledFor *time.Time    `json:"scheduledFor,omitempty"`
	Timezone     string        `json:"timezone,omitempty"`
	Status       PostStatus    `json:"status"`
	ContentHash  string        `json:"contentHash,omitempty"` // idempotência content-hash (ADR-005 §1.3)
	CreatedBy    string        `json:"createdBy,omitempty"`
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    time.Time     `json:"updatedAt"`
}

// PostTarget é 1 linha por (post, accountId, platform) com status próprio (ADR-007).
// platformSpecificData é a union tipada por plataforma (ADR-005 §1.5), JSONB no banco.
type PostTarget struct {
	ID                   string         `json:"id,omitempty"`
	PostID               string         `json:"postId,omitempty"`
	Platform             Platform       `json:"platform"`
	AccountID            string         `json:"accountId"`
	Status               TargetStatus   `json:"status"`
	PlatformSpecificData map[string]any `json:"platformSpecificData,omitempty"`
	PublishedURL         string         `json:"publishedUrl,omitempty"`
	ExternalPostID       string         `json:"externalPostId,omitempty"`
	LastError            *TargetError   `json:"lastError,omitempty"`
}

// TargetError é o erro tipado de um target (code + mensagem legível).
type TargetError struct {
	Code  string `json:"code"`
	Error string `json:"error"`
}

// PublishAttempt é o log APPEND-ONLY de cada tentativa (ADR-007 §1) — fonte da
// auditoria em GET /posts/{id}/logs.
type PublishAttempt struct {
	ID         string         `json:"id,omitempty"`
	TargetID   string         `json:"targetId"`
	Attempt    int            `json:"attempt"`
	StartedAt  time.Time      `json:"startedAt"`
	FinishedAt *time.Time     `json:"finishedAt,omitempty"`
	Outcome    PublishOutcome `json:"outcome"`
	Error      *TargetError   `json:"error,omitempty"`
	HTTPStatus int            `json:"httpStatus,omitempty"`
	RequestID  string         `json:"requestId,omitempty"`
}

// APIKey é a credencial do cliente (ADR-006). Somente o HASH é armazenado;
// a chave pura (sk_live_...) é retornada UMA única vez no momento da criação.
type APIKey struct {
	ID         string     `json:"id"`
	TeamID     string     `json:"teamId"`
	Name       string     `json:"name"`
	KeyHash    string     `json:"-"`
	KeyPrefix  string     `json:"keyPrefix"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	RevokedAt  *time.Time `json:"revokedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}

// Media é um arquivo armazenado (ADR-008): upload direto (≤25MB, retenção 7d)
// ou presign (até 5GB, permanente). storage_path é interno (nunca exposto).
type Media struct {
	ID            string     `json:"id"`
	ProfileID     string     `json:"profileId"`
	Filename      string     `json:"filename"`
	ContentType   string     `json:"contentType"`
	SizeBytes     int64      `json:"sizeBytes"`
	StoragePath   string     `json:"-"`
	PublicURL     string     `json:"publicUrl"`
	RetentionDays *int       `json:"retentionDays,omitempty"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
}

// Webhook é uma configuração de entrega de eventos (ADR-009).
type Webhook struct {
	ID              string            `json:"id"`
	ProfileID       string            `json:"profileId"`
	Name            string            `json:"name"`
	URL             string            `json:"url"`
	SecretHash      string            `json:"-"`
	SecretEncrypted []byte            `json:"-"` // AES-256-GCM — para ASSINAR payloads
	Events          []string          `json:"events"`
	IsActive        bool              `json:"isActive"`
	CustomHeaders   map[string]string `json:"customHeaders,omitempty"`
	CreatedAt       time.Time         `json:"createdAt"`
	UpdatedAt       time.Time         `json:"updatedAt"`
}
