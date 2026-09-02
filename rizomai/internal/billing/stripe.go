// Package billing integra o gateway de pagamento (ADR-010 §1.3): Stripe como
// gateway primário (cartão + PIX via Stripe Brasil — configurável por env).
//
// Sem STRIPE_SECRET_KEY configurada, as operações retornam STRIPE_NOT_CONFIGURED
// (nunca panic) — o checkout só funciona em produção/test com chave real.
package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	stripe "github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/webhook"
)

// ErrNotConfigured indica Stripe sem chave (STRIPE_NOT_CONFIGURED).
var ErrNotConfigured = errors.New("stripe não configurado: defina STRIPE_SECRET_KEY e STRIPE_WEBHOOK_SECRET")

// Client encapsula o gateway Stripe.
type Client struct {
	apiKey        string
	webhookSecret string
	currency      string
	paymentMethods []string // ex.: ["card", "pix"]
	successURL    string
	cancelURL     string
	logger        *log.Logger
}

// Config carrega o Stripe via env (modo TEST por padrão).
type Config struct {
	SecretKey       string // STRIPE_SECRET_KEY (sk_test_...)
	WebhookSecret   string // STRIPE_WEBHOOK_SECRET
	Currency        string // STRIPE_CURRENCY (default BRL)
	PaymentMethods  []string // STRIPE_PAYMENT_METHODS (default card,pix)
	SuccessURL      string // PUBLIC_BASE_URL/billing/success
	CancelURL       string // PUBLIC_BASE_URL/billing/cancel
	Logger          *log.Logger
}

// NewClient cria o cliente (não conecta nada; a validação acontece no uso).
func NewClient(cfg Config) *Client {
	if cfg.Currency == "" {
		cfg.Currency = "brl"
	}
	if len(cfg.PaymentMethods) == 0 {
		cfg.PaymentMethods = []string{"card", "pix"} // PIX via Stripe Brasil (ADR-010 §1.3)
	}
	if cfg.SuccessURL == "" {
		cfg.SuccessURL = "http://localhost:8080/billing/success"
	}
	if cfg.CancelURL == "" {
		cfg.CancelURL = "http://localhost:8080/billing/cancel"
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	return &Client{
		apiKey:         cfg.SecretKey,
		webhookSecret:  cfg.WebhookSecret,
		currency:       cfg.Currency,
		paymentMethods: cfg.PaymentMethods,
		successURL:     cfg.SuccessURL,
		cancelURL:      cfg.CancelURL,
		logger:         cfg.Logger,
	}
}

// FromEnv monta o Config a partir das env vars.
func FromEnv() Config {
	return Config{
		SecretKey:      os.Getenv("STRIPE_SECRET_KEY"),
		WebhookSecret:  os.Getenv("STRIPE_WEBHOOK_SECRET"),
		Currency:       envOr("STRIPE_CURRENCY", "brl"),
		PaymentMethods: splitCSV(os.Getenv("STRIPE_PAYMENT_METHODS"), "card,pix"),
	}
}

// Configured indica se as chaves do Stripe estão presentes.
func (c *Client) Configured() bool { return c.apiKey != "" && c.webhookSecret != "" }

// CreateCheckoutSession cria uma sessão de checkout (modo subscription) para o
// plano do team. priceID vem do plano (STRIPE_PRICE_* no seed).
func (c *Client) CreateCheckoutSession(ctx context.Context, teamID, planID, priceID string) (string, error) {
	if !c.Configured() {
		return "", ErrNotConfigured
	}
	if priceID == "" {
		return "", fmt.Errorf("plano %s sem stripe_price_id (configure STRIPE_PRICE_%s)", planID, planID)
	}

	stripe.Key = c.apiKey
	params := &stripe.CheckoutSessionParams{
		Params: stripe.Params{Context: ctx},
		Mode:   stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL:         stripe.String(c.successURL),
		CancelURL:          stripe.String(c.cancelURL),
		ClientReferenceID:  stripe.String(teamID),
		PaymentMethodTypes: stripe.StringSlice(c.paymentMethods),
		Metadata: map[string]string{
			"team_id": teamID,
			"plan":    planID,
		},
	}
	sess, err := session.New(params)
	if err != nil {
		return "", fmt.Errorf("stripe checkout: %w", err)
	}
	return sess.URL, nil
}

// WebhookEvent é o resultado processado de um evento de billing.
type WebhookEvent struct {
	Type   string `json:"type"`
	TeamID string `json:"teamId,omitempty"`
	PlanID string `json:"planId,omitempty"`
}

// HandleWebhook valida a assinatura (HMAC) e processa os eventos de billing.
// Em checkout.session.completed devolve TeamID/PlanID para o handler ativar o
// plano (metadados da sessão).
func (c *Client) HandleWebhook(ctx context.Context, payload []byte, sigHeader string) (*WebhookEvent, error) {
	if c.webhookSecret == "" {
		return nil, ErrNotConfigured
	}
	event, err := webhook.ConstructEventWithOptions(payload, sigHeader, c.webhookSecret,
		webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true})
	if err != nil {
		return nil, fmt.Errorf("stripe webhook: assinatura inválida: %w", err)
	}

	out := &WebhookEvent{Type: string(event.Type)}

	switch event.Type {
	case "checkout.session.completed":
		var cs stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &cs); err != nil {
			return out, err
		}
		out.TeamID = valueOr(cs.ClientReferenceID, "")
		out.PlanID = cs.Metadata["plan"]
		subID := ""
		if cs.Subscription != nil {
			subID = cs.Subscription.ID
		}
		c.logger.Printf("billing: checkout completado team=%s plan=%s (subscription %s)",
			out.TeamID, out.PlanID, subID)

	case "invoice.paid", "invoice.payment_failed":
		var inv stripe.Invoice
		_ = json.Unmarshal(event.Data.Raw, &inv)
		c.logger.Printf("billing: %s (status=%s)", event.Type, valueOr(string(inv.Status), "?"))

	case "customer.subscription.updated", "customer.subscription.deleted":
		var sub stripe.Subscription
		_ = json.Unmarshal(event.Data.Raw, &sub)
		c.logger.Printf("billing: %s (status=%s)", event.Type, string(sub.Status))
	}

	return out, nil
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func splitCSV(v, def string) []string {
	if strings.TrimSpace(v) == "" {
		return []string{def}
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func valueOr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
