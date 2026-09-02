// Entrega de webhooks (ADR-009): assinatura HMAC-SHA256 e envio com timeout
// de 5s, registrando o delivery log. Compartilhado entre o worker River
// (retry com mesmo event id) e o SimulatedQueue (entrega síncrona p/ demo).
package queue

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/rizomai/rizomai/internal/domain"
	"github.com/rizomai/rizomai/internal/oauth"
	"github.com/rizomai/rizomai/internal/store"
)

func newDeliveryID() string {
	id, _ := domain.NewDeliveryID()
	return id
}

// SignPayload assina o body bruto com HMAC-SHA256 e devolve o hex
// (header X-Rizomai-Signature: sha256=<hex> — ADR-009 §1.2).
func SignPayload(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// DeliverWebhook entrega UM evento de forma síncrona: monta o payload padrão
// {id, event, timestamp, data}, assina, envia com timeout de 5s (ADR-009 §1.3)
// e registra o delivery log. Devolve erro em falha (rede ou não-2xx) para o
// chamador decidir retry (o event id NUNCA muda — dedup do consumidor).
func DeliverWebhook(ctx context.Context, st *store.Store, wh *domain.Webhook, eventID, eventType string, data map[string]any, tokenKey []byte, logger *log.Logger) error {
	if logger == nil {
		logger = log.Default()
	}

	secret, err := oauth.Decrypt(wh.SecretEncrypted, tokenKey)
	if err != nil {
		return fmt.Errorf("decrypt secret do webhook %s: %w", wh.ID, err)
	}

	payload := map[string]any{
		"id":        eventID,
		"event":     eventType,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"data":      data,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wh.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Rizomai-Signature", "sha256="+SignPayload(secret, body))
	for k, v := range wh.CustomHeaders {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 5 * time.Second} // ADR-009 §1.3
	resp, err := client.Do(req)

	delivery := &domain.WebhookDelivery{
		ID:        newDeliveryID(),
		WebhookID: wh.ID,
		EventID:   eventID,
		EventType: eventType,
		Payload:   body,
		Attempts:  1,
	}

	if err != nil {
		delivery.Status = "failed"
		delivery.Error = err.Error()
		_ = st.RecordDelivery(ctx, delivery)
		logger.Printf("webhook.deliver %s: erro de rede: %v", wh.ID, err)
		return err
	}
	defer resp.Body.Close()

	delivery.HTTPStatus = resp.StatusCode
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		delivery.Status = "success"
		_ = st.RecordDelivery(ctx, delivery)
		logger.Printf("webhook.deliver %s: %s entregue (%d)", wh.ID, eventType, resp.StatusCode)
		return nil
	}

	delivery.Status = "failed"
	delivery.Error = "HTTP " + resp.Status
	_ = st.RecordDelivery(ctx, delivery)
	logger.Printf("webhook.deliver %s: não-2xx %d", wh.ID, resp.StatusCode)
	return fmt.Errorf("webhook deliver: HTTP %d", resp.StatusCode)
}
