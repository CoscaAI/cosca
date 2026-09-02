// Entidades de billing (ADR-010).
package domain

import "time"

// BillingPlan é um plano do catálogo (free/growth/escala/agencia).
type BillingPlan struct {
	Codename                 string         `json:"codename"` // free|growth|escala|agencia
	Name                     string         `json:"name"`
	ContasIncluidas          int            `json:"contasIncluidas"`
	PrecoPorContaExtraCents  int64          `json:"precoPorContaExtraCents"`
	PrecoFixoCents           int64          `json:"precoFixoCents"`
	StripePriceID            string         `json:"-"`
	Features                 map[string]any `json:"features"`
}

// TeamPlan é o plano atual de um team.
type TeamPlan struct {
	TeamID             string     `json:"teamId"`
	Plan               BillingPlan `json:"plan"`
	Status             string     `json:"status"` // active|trialing|past_due|canceled
	BillingAnchorDay   int        `json:"billingAnchorDay"`
	CurrentPeriodStart time.Time  `json:"currentPeriodStart"`
	CurrentPeriodEnd   *time.Time `json:"currentPeriodEnd,omitempty"`
}

// UsageInfo é o uso do período atual (GET /v1/billing/usage — ADR-010 §1.2).
type UsageInfo struct {
	PlanName         string `json:"planName"`
	ContasConectadas int    `json:"contasConectadas"`
	ContasLimite     int    `json:"contasLimite"`
	PeriodStart      string `json:"periodStart"`
	PeriodEnd        string `json:"periodEnd,omitempty"`
	PeriodValorCents int64  `json:"periodValorCents"`
}

// PostAnalytics é o snapshot de métricas de um post em uma plataforma
// (post_analytics — agregado por dia).
type PostAnalytics struct {
	PostID     string `json:"postId"`
	Platform   string `json:"platform"`
	ExternalID string `json:"externalId,omitempty"`
	Views      int64  `json:"views"`
	Likes      int64  `json:"likes"`
	Comments   int64  `json:"comments"`
	Shares     int64  `json:"shares"`
	CapturedAt string `json:"capturedAt"` // YYYY-MM-DD
}

// ProfileAnalytics agrega as métricas de um profile (por plataforma).
type ProfileAnalytics struct {
	Platform     string `json:"platform"`
	Views        int64  `json:"views"`
	Likes        int64  `json:"likes"`
	Comments     int64  `json:"comments"`
	Shares       int64  `json:"shares"`
	PostsCounted int64  `json:"postsCounted"`
}

// InboxMessage é uma mensagem da caixa unificada (dm|comment|mention).
type InboxMessage struct {
	ID          string    `json:"id"`
	ProfileID   string    `json:"profileId"`
	Platform    string    `json:"platform"`
	ExternalID  string    `json:"externalId,omitempty"`
	Sender      string    `json:"sender"`
	Text        string    `json:"text"`
	MessageType string    `json:"messageType"` // dm | comment | mention
	IsRead      bool      `json:"isRead"`
	ReceivedAt  time.Time `json:"receivedAt"`
}
