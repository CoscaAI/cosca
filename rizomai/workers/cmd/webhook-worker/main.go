// Command webhook-worker entrega webhooks com retry e mesmo event id
// (ADR-009) — binário 3 do monorepo (ADR-004).
//
// Fase 1: esqueleto. Implementação na Fase 2 como worker de delivery via River:
//   - timeout de 5s por entrega (consumidor lento = retry-storm — R6)
//   - assinatura HMAC-SHA256 (header X-Rizomai-Signature) — verificação em
//     tempo constante (hmac.Equal)
//   - retry 5s×2^n, janela de 24h, MESMO event id em todas as tentativas
//     (dedup do consumidor — ADR-009 §1.4)
//   - delivery logs + redelivery manual (GET /v1/webhooks/logs, POST .../redeliver)
package main

import "log"

// DeliverWebhookJob é a unidade de trabalho da entrega (ADR-009 §1.4).
// EventID NUNCA muda entre retries: é o ponto de dedup do consumidor.
type DeliverWebhookJob struct {
	DeliveryID string `json:"deliveryId"`
	WebhookID  string `json:"webhookId"`
	EventID    string `json:"eventId"`
	EventType  string `json:"eventType"`
}

// Kind identifica o tipo do job no River.
func (DeliverWebhookJob) Kind() string { return "webhook.deliver" }

func main() {
	log.Printf("webhook-worker: scaffold ativo — delivery chega na Fase 2")
}
