// Repositório de webhooks (ADR-009).
package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/rizomai/rizomai/internal/domain"
)

// CreateWebhook registra um webhook (secret criptografado para assinatura).
func (s *Store) CreateWebhook(ctx context.Context, w *domain.Webhook) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO webhooks
		     (id, profile_id, name, url, secret_hash, secret_encrypted, events, is_active, custom_headers)
		   VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		w.ID, w.ProfileID, w.Name, w.URL, w.SecretHash, w.SecretEncrypted,
		jsonBytes(w.Events), w.IsActive, jsonObject(w.CustomHeaders),
	)
	return err
}

// ListWebhooksByProfile devolve os webhooks ativos de um profile
// (usado para enfileirar eventos post.* após a publicação).
func (s *Store) ListWebhooksByProfile(ctx context.Context, profileID string) ([]domain.Webhook, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, profile_id, name, url, secret_hash, secret_encrypted, events, is_active, custom_headers, created_at, updated_at
		   FROM webhooks
		  WHERE profile_id = $1 AND is_active = TRUE
		  ORDER BY created_at`,
		profileID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Webhook
	for rows.Next() {
		var w domain.Webhook
		var eventsJSON, headersJSON []byte
		if err := rows.Scan(&w.ID, &w.ProfileID, &w.Name, &w.URL, &w.SecretHash, &w.SecretEncrypted,
			&eventsJSON, &w.IsActive, &headersJSON, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(eventsJSON, &w.Events)
		_ = json.Unmarshal(headersJSON, &w.CustomHeaders)
		out = append(out, w)
	}
	return out, rows.Err()
}

// GetWebhook busca um webhook pelo id (com secret criptografado).
func (s *Store) GetWebhook(ctx context.Context, id string) (*domain.Webhook, error) {
	var w domain.Webhook
	var eventsJSON, headersJSON []byte
	err := s.db.QueryRow(ctx,
		`SELECT id, profile_id, name, url, secret_hash, secret_encrypted, events, is_active, custom_headers, created_at, updated_at
		   FROM webhooks WHERE id = $1`,
		id,
	).Scan(&w.ID, &w.ProfileID, &w.Name, &w.URL, &w.SecretHash, &w.SecretEncrypted,
		&eventsJSON, &w.IsActive, &headersJSON, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(eventsJSON, &w.Events)
	_ = json.Unmarshal(headersJSON, &w.CustomHeaders)
	return &w, nil
}

// RecordDelivery registra uma tentativa de entrega (log append-only — ADR-009 §1.5).
func (s *Store) RecordDelivery(ctx context.Context, d *domain.WebhookDelivery) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO webhook_deliveries
		     (id, webhook_id, event_id, event_type, payload, status, http_status, error, attempts)
		   VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		d.ID, d.WebhookID, d.EventID, d.EventType, jsonBytes(d.Payload),
		d.Status, nullableInt(d.HTTPStatus), nullableString(d.Error), d.Attempts,
	)
	return err
}

func nullableInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}
