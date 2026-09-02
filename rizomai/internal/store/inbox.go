// Inbox: caixa unificada de DMs/comentários/menções (Fase 6).
package store

import (
	"context"
	"strconv"

	"github.com/rizomai/rizomai/internal/domain"
)

// InsertMessage registra uma mensagem do inbox.
func (s *Store) InsertMessage(ctx context.Context, m *domain.InboxMessage) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO inbox_messages (id, profile_id, platform, external_id, sender, text, message_type, is_read, received_at)
		   VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		m.ID, m.ProfileID, m.Platform, nullableString(m.ExternalID),
		m.Sender, m.Text, m.MessageType, m.IsRead, m.ReceivedAt,
	)
	return err
}

// ListMessages lista as mensagens do team (filtros opcionais por plataforma e
// status de leitura). Escopo: profiles do team.
func (s *Store) ListMessages(ctx context.Context, teamID, platform string, isRead *bool, limit int) ([]domain.InboxMessage, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	// Monta a query com filtros dinâmicos (evita injeção — valores por args).
	q := `SELECT m.id, m.profile_id, m.platform, m.external_id, m.sender, m.text,
	             m.message_type, m.is_read, m.received_at
	        FROM inbox_messages m JOIN profiles pr ON pr.id = m.profile_id
	       WHERE pr.team_id = $1`
	args := []any{teamID}
	n := 2
	if platform != "" {
		q += " AND m.platform = $" + strconv.Itoa(n)
		args = append(args, platform)
		n++
	}
	if isRead != nil {
		q += " AND m.is_read = $" + strconv.Itoa(n)
		args = append(args, *isRead)
		n++
	}
	q += " ORDER BY m.received_at DESC LIMIT $" + strconv.Itoa(n)
	args = append(args, limit)

	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.InboxMessage
	for rows.Next() {
		var m domain.InboxMessage
		if err := rows.Scan(&m.ID, &m.ProfileID, &m.Platform, &m.ExternalID, &m.Sender,
			&m.Text, &m.MessageType, &m.IsRead, &m.ReceivedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MarkRead marca uma mensagem como lida (escopo do team).
func (s *Store) MarkRead(ctx context.Context, teamID, messageID string) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE inbox_messages m SET is_read = TRUE
		   FROM profiles pr
		  WHERE m.id = $1 AND pr.id = m.profile_id AND pr.team_id = $2`,
		messageID, teamID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
