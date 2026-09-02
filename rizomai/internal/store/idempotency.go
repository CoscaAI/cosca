// Idempotência — 1ª camada do ADR-005 §1.3:
// mesma Idempotency-Key (por team) dentro da janela de 24h devolve a MESMA
// resposta, sem reprocessar. A resposta é armazenada junto com o request hash.
package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// idempotencyTTL é a janela de replay (24h — ADR-005 §1.3).
const idempotencyTTL = 24 * time.Hour

// GetIdempotency retorna a resposta (status + body) previamente armazenada para
// (teamID, key), ou ErrNotFound se inexistente/fora da janela.
func (s *Store) GetIdempotency(ctx context.Context, teamID, key string) (int, []byte, error) {
	var (
		statusCode int
		body       []byte
		createdAt  time.Time
	)
	err := s.db.QueryRow(ctx,
		`SELECT status_code, response, created_at
		   FROM idempotency_keys
		  WHERE key = $1 AND team_id = $2`,
		key, teamID,
	).Scan(&statusCode, &body, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil, ErrNotFound
	}
	if err != nil {
		return 0, nil, err
	}

	if time.Since(createdAt) > idempotencyTTL {
		_, _ = s.db.Exec(ctx,
			`DELETE FROM idempotency_keys WHERE key = $1 AND team_id = $2`, key, teamID)
		return 0, nil, ErrNotFound
	}
	return statusCode, body, nil
}

// SaveIdempotency armazena a resposta. ON CONFLICT DO NOTHING: em corrida
// concorrente com a mesma key, a primeira resposta vence (a segunda retorna a
// resposta existente via GetIdempotency).
func (s *Store) SaveIdempotency(ctx context.Context, teamID, key, requestHash string, statusCode int, body []byte) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO idempotency_keys (key, team_id, request_hash, status_code, response)
		   VALUES ($1, $2, $3, $4, $5)
		   ON CONFLICT (key, team_id) DO NOTHING`,
		key, teamID, requestHash, statusCode, body,
	)
	return err
}
