// Repositório de teams e social accounts (Fase 3: credenciais criptografadas).
package store

import (
	"context"
	"errors"
	"fmt"

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

// CreateAccount insere uma conta conectada (OAuth broker / seed).
func (s *Store) CreateAccount(ctx context.Context, a *domain.SocialAccount) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO social_accounts
		     (id, profile_id, platform, display_name, platform_user_id, token_status, settings,
		      encrypted_token, refresh_token_encrypted, expires_at, external_identifier, token_scope)
		   VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		a.ID, a.ProfileID, a.Platform, nullableString(a.DisplayName), nullableString(a.PlatformUserID),
		a.TokenStatus, jsonObject(a.Settings),
		a.EncryptedToken, a.RefreshTokenEncrypted, a.ExpiresAt,
		nullableString(a.ExternalIdentifier), nullableString(a.TokenScope),
	)
	return err
}

// UpsertAccount cria a conta OU atualiza a existente da mesma plataforma no
// mesmo profile (multi-contas por plataforma são suportadas — ADR-006 §4).
func (s *Store) UpsertAccount(ctx context.Context, a *domain.SocialAccount) error {
	existingID, err := s.accountIDByPlatform(ctx, a.ProfileID, a.Platform)
	if err == nil && existingID != "" {
		_, err := s.db.Exec(ctx,
			`UPDATE social_accounts
			    SET display_name = $2, platform_user_id = $3, token_status = $4,
			        encrypted_token = $5, refresh_token_encrypted = $6, expires_at = $7,
			        external_identifier = $8, token_scope = $9, updated_at = now()
			  WHERE id = $1`,
			existingID, nullableString(a.DisplayName), nullableString(a.PlatformUserID), a.TokenStatus,
			a.EncryptedToken, a.RefreshTokenEncrypted, a.ExpiresAt,
			nullableString(a.ExternalIdentifier), nullableString(a.TokenScope),
		)
		a.ID = existingID
		return err
	}
	return s.CreateAccount(ctx, a)
}

func (s *Store) accountIDByPlatform(ctx context.Context, profileID string, p domain.Platform) (string, error) {
	var id string
	err := s.db.QueryRow(ctx,
		`SELECT id FROM social_accounts WHERE profile_id = $1 AND platform = $2 ORDER BY created_at LIMIT 1`,
		profileID, p,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, err
}

// ListAccountsByTeam pagina as contas do team (GET /v1/accounts — spec), com
// filtros opcionais por profileId e platform. Tokens NUNCA são selecionados:
// o contrato expõe apenas tokenStatus e metadados (ADR-006 §1.2).
func (s *Store) ListAccountsByTeam(ctx context.Context, teamID, profileID, platform string, limit, offset int) ([]domain.SocialAccount, int, error) {
	where := `pr.team_id = $1`
	args := []any{teamID}
	if profileID != "" {
		args = append(args, profileID)
		where += fmt.Sprintf(" AND a.profile_id = $%d", len(args))
	}
	if platform != "" {
		args = append(args, platform)
		where += fmt.Sprintf(" AND a.platform = $%d", len(args))
	}

	var total int
	if err := s.db.QueryRow(ctx,
		`SELECT count(*) FROM social_accounts a JOIN profiles pr ON pr.id = a.profile_id WHERE `+where,
		args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.Query(ctx,
		`SELECT a.id, a.profile_id, a.platform, a.display_name, a.platform_user_id,
		        a.token_status, a.settings, a.connected_at, a.created_at, a.updated_at,
		        a.expires_at, a.external_identifier, a.token_scope
		   FROM social_accounts a
		   JOIN profiles pr ON pr.id = a.profile_id
		  WHERE `+where+`
		  ORDER BY a.connected_at DESC
		  LIMIT $`+fmt.Sprintf("%d", len(args)+1)+` OFFSET $`+fmt.Sprintf("%d", len(args)+2),
		append(append([]any{}, args...), limit, offset)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	accounts := make([]domain.SocialAccount, 0, limit)
	for rows.Next() {
		var a domain.SocialAccount
		var displayName, platformUser, extIdent, tokenScope *string
		if err := rows.Scan(&a.ID, &a.ProfileID, &a.Platform, &displayName, &platformUser,
			&a.TokenStatus, &a.Settings, &a.ConnectedAt, &a.CreatedAt, &a.UpdatedAt,
			&a.ExpiresAt, &extIdent, &tokenScope); err != nil {
			return nil, 0, err
		}
		if displayName != nil {
			a.DisplayName = *displayName
		}
		if platformUser != nil {
			a.PlatformUserID = *platformUser
		}
		if extIdent != nil {
			a.ExternalIdentifier = *extIdent
		}
		if tokenScope != nil {
			a.TokenScope = *tokenScope
		}
		accounts = append(accounts, a)
	}
	return accounts, total, rows.Err()
}

// GetAccountByID busca uma conta com as credenciais criptografadas
// (escopo: team — usada pelo worker de publicação e health).
func (s *Store) GetAccountByID(ctx context.Context, teamID, accountID string) (*domain.SocialAccount, error) {
	var a domain.SocialAccount
	err := s.db.QueryRow(ctx,
		`SELECT a.id, a.profile_id, a.platform, a.display_name, a.platform_user_id,
		        a.token_status, a.settings, a.connected_at, a.created_at, a.updated_at,
		        a.encrypted_token, a.refresh_token_encrypted, a.expires_at,
		        a.external_identifier, a.token_scope
		   FROM social_accounts a
		   JOIN profiles pr ON pr.id = a.profile_id
		  WHERE a.id = $1 AND pr.team_id = $2`,
		accountID, teamID,
	).Scan(&a.ID, &a.ProfileID, &a.Platform, &a.DisplayName, &a.PlatformUserID,
		&a.TokenStatus, &a.Settings, &a.ConnectedAt, &a.CreatedAt, &a.UpdatedAt,
		&a.EncryptedToken, &a.RefreshTokenEncrypted, &a.ExpiresAt,
		&a.ExternalIdentifier, &a.TokenScope)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}
