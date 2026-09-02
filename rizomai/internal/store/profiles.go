// Repositório de profiles (escopado por team).
package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/rizomai/rizomai/internal/domain"
)

// CreateProfile insere um profile no team (POST /v1/profiles).
func (s *Store) CreateProfile(ctx context.Context, p *domain.Profile) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO profiles (id, team_id, name) VALUES ($1, $2, $3)`,
		p.ID, p.TeamID, p.Name,
	)
	return err
}

// GetProfile busca um profile validando o escopo de team.
func (s *Store) GetProfile(ctx context.Context, teamID, id string) (*domain.Profile, error) {
	var p domain.Profile
	err := s.db.QueryRow(ctx,
		`SELECT id, team_id, name, created_at, updated_at
		   FROM profiles
		  WHERE id = $1 AND team_id = $2`,
		id, teamID,
	).Scan(&p.ID, &p.TeamID, &p.Name, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ListProfiles pagina os profiles do team (GET /v1/profiles).
// Retorna a lista e o total de registros (para o envelope de paginação).
func (s *Store) ListProfiles(ctx context.Context, teamID string, limit, offset int) ([]domain.Profile, int, error) {
	var total int
	if err := s.db.QueryRow(ctx,
		`SELECT count(*) FROM profiles WHERE team_id = $1`, teamID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.Query(ctx,
		`SELECT id, team_id, name, created_at, updated_at
		   FROM profiles
		  WHERE team_id = $1
		  ORDER BY created_at DESC
		  LIMIT $2 OFFSET $3`,
		teamID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	profiles := make([]domain.Profile, 0, limit)
	for rows.Next() {
		var p domain.Profile
		if err := rows.Scan(&p.ID, &p.TeamID, &p.Name, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, err
		}
		profiles = append(profiles, p)
	}
	return profiles, total, rows.Err()
}
