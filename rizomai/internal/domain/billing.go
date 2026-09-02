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
