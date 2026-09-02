// Testes do billing Stripe sem chave real: caminho STRIPE_NOT_CONFIGURED,
// validação de webhook com payload fabricado e ativação do plano.
package billing

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v81/webhook"
)

func testClient(secret, webhookSecret string) *Client {
	return NewClient(Config{
		SecretKey:      secret,
		WebhookSecret:  webhookSecret,
		Logger:         log.Default(),
	})
}

func TestConfiguredFalseWithoutKeys(t *testing.T) {
	c := testClient("", "")
	if c.Configured() {
		t.Error("sem chaves o client não deveria estar configured")
	}
}

func TestCreateCheckoutSessionNotConfigured(t *testing.T) {
	c := testClient("", "")
	_, err := c.CreateCheckoutSession(context.Background(), "team_1", "growth", "price_123")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("esperava ErrNotConfigured, veio %v", err)
	}
}

func TestHandleWebhookNotConfigured(t *testing.T) {
	c := testClient("sk_test_x", "")
	_, err := c.HandleWebhook(context.Background(), []byte("{}"), "sig")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("esperava ErrNotConfigured, veio %v", err)
	}
}

func TestHandleWebhookInvalidSignature(t *testing.T) {
	c := testClient("sk_test_x", "whsec_test")
	_, err := c.HandleWebhook(context.Background(), []byte(`{"type":"x"}`), "assinatura-invalida")
	if err == nil {
		t.Fatal("assinatura inválida deveria falhar")
	}
}

func TestHandleWebhookCheckoutCompleted(t *testing.T) {
	c := testClient("sk_test_x", "whsec_test")

	payload := []byte(`{
		"id": "evt_1",
		"object": "event",
		"type": "checkout.session.completed",
		"data": {
			"object": {
				"id": "cs_1",
				"client_reference_id": "team_dev",
				"metadata": {"team_id": "team_dev", "plan": "growth"},
				"subscription": "sub_123"
			}
		}
	}`)

	// Assinatura HMAC fabricada (stripe-go expõe ComputeSignature); o timestamp
	// precisa estar na tolerância de ±5min da validação.
	ts := time.Now()
	sig := webhook.ComputeSignature(ts, payload, "whsec_test")
	header := fmt.Sprintf("t=%d,v1=%s", ts.Unix(), hex.EncodeToString(sig))

	ev, err := c.HandleWebhook(context.Background(), payload, header)
	if err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if ev.Type != "checkout.session.completed" {
		t.Errorf("type = %q", ev.Type)
	}
	if ev.TeamID != "team_dev" {
		t.Errorf("teamID = %q", ev.TeamID)
	}
	if ev.PlanID != "growth" {
		t.Errorf("planID = %q", ev.PlanID)
	}
}
