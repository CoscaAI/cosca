// States OAuth do broker server-side (ADR-006 §1.1).
package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/rizomai/rizomai/internal/domain"
)

// CreateOAuthState persiste o state (com code_verifier PKCE e destino).
func (s *Store) CreateOAuthState(ctx context.Context, st *domain.OAuthState) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO oauth_states (id, team_id, platform, state, code_verifier, redirect_uri, profile_id, expires_at)
		   VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		st.ID, st.TeamID, st.Platform, st.State, st.CodeVerifier, st.RedirectURI, nullableString(st.ProfileID), st.ExpiresAt,
	)
	return err
}

// GetOAuthState busca o state pelo valor (UNIQUE).
func (s *Store) GetOAuthState(ctx context.Context, state string) (*domain.OAuthState, error) {
	var st domain.OAuthState
	err := s.db.QueryRow(ctx,
		`SELECT id, team_id, platform, state, code_verifier, redirect_uri, profile_id, created_at, expires_at
		   FROM oauth_states WHERE state = $1`,
		state,
	).Scan(&st.ID, &st.TeamID, &st.Platform, &st.State, &st.CodeVerifier, &st.RedirectURI,
		&st.ProfileID, &st.CreatedAt, &st.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// DeleteOAuthState remove o state (uso único — callback só roda uma vez).
func (s *Store) DeleteOAuthState(ctx context.Context, state string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM oauth_states WHERE state = $1`, state)
	return err
}

// nullableString converte string vazia em nil (coluna TEXT nullable).
func nullableString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
