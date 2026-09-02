// Repositório de teams e social accounts.
// Teams não têm rota pública (a API key pertence a um team); o seed cria o
// team inicial de dev. Social accounts nascem do OAuth (Fase 3); o seed cria
// contas fictícias para exercitar o pipeline de posts.
package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/rizomai/rizomai/internal/domain"
)

// ErrMultipleProfiles indica que os accounts de um post pertencem a profiles
// diferentes (um post pertence a UM profile — ADR-007).
var ErrMultipleProfiles = errors.New("store: accounts pertencem a profiles diferentes")

// EnsureTeam cria o team se não existir (idempotente — bootstrap/dev).
func (s *Store) EnsureTeam(ctx context.Context, id, name string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO teams (id, name) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING`,
		id, name,
	)
	return err
}

// CreateAccount insere uma conta conectada (OAuth broker na Fase 3).
func (s *Store) CreateAccount(ctx context.Context, a *domain.SocialAccount) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO social_accounts
		     (id, profile_id, platform, display_name, platform_user_id, token_status, settings)
		   VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		a.ID, a.ProfileID, a.Platform, a.DisplayName, a.PlatformUserID, a.TokenStatus,
		jsonObject(a.Settings),
	)
	return err
}

// ValidateAccountsForPost verifica que todos os accountIDs existem no team e
// pertencem ao MESMO profile; devolve o profileID resultante.
//
//	- algum account inexistente/outro tenant → ErrNotFound
//	- accounts de profiles diferentes → ErrMultipleProfiles
func (s *Store) ValidateAccountsForPost(ctx context.Context, teamID string, accountIDs []string) (string, error) {
	rows, err := s.db.Query(ctx,
		`SELECT a.id, a.profile_id
		   FROM social_accounts a JOIN profiles pr ON pr.id = a.profile_id
		  WHERE a.id = ANY($1) AND pr.team_id = $2`,
		accountIDs, teamID,
	)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	profileByAccount := make(map[string]string, len(accountIDs))
	for rows.Next() {
		var accountID, profileID string
		if err := rows.Scan(&accountID, &profileID); err != nil {
			return "", err
		}
		profileByAccount[accountID] = profileID
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	if len(profileByAccount) != len(accountIDs) {
		return "", ErrNotFound // algum account não existe no team
	}

	profileID := ""
	for _, pid := range profileByAccount {
		if profileID == "" {
			profileID = pid
		} else if pid != profileID {
			return "", ErrMultipleProfiles
		}
	}
	if profileID == "" {
		return "", pgx.ErrNoRows
	}
	return profileID, nil
}
