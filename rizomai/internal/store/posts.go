// Repositório de posts + targets (ADR-007).
// Criação é TRANSACIONAL: Post + N PostTargets no mesmo commit (ADR-002/003).
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/rizomai/rizomai/internal/domain"
)

// GetPostIDByContentHash busca um post do team com o mesmo content_hash
// (2ª camada de idempotência — ADR-005 §1.3b).
func (s *Store) GetPostIDByContentHash(ctx context.Context, teamID, contentHash string) (string, error) {
	var id string
	err := s.db.QueryRow(ctx,
		`SELECT p.id
		   FROM posts p JOIN profiles pr ON pr.id = p.profile_id
		  WHERE p.content_hash = $1 AND pr.team_id = $2`,
		contentHash, teamID,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

// CreatePostWithTargets persiste Post + N PostTargets atomicamente.
// Se qualquer INSERT falhar, a transação é revertida (nada de post "órfão").
func (s *Store) CreatePostWithTargets(ctx context.Context, p *domain.Post, targets []domain.PostTarget) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) // no-op após Commit

	if _, err := tx.Exec(ctx,
		`INSERT INTO posts
		     (id, profile_id, content, media_urls, scheduled_for, timezone, status, content_hash, created_by)
		   VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		p.ID, p.ProfileID, p.Content,
		jsonBytes(p.MediaURLs), p.ScheduledFor, p.Timezone,
		p.Status, p.ContentHash, p.CreatedBy,
	); err != nil {
		return fmt.Errorf("insert post: %w", err)
	}

	for _, t := range targets {
		if _, err := tx.Exec(ctx,
			`INSERT INTO post_targets
			     (id, post_id, account_id, platform, status, platform_specific_data)
			   VALUES ($1, $2, $3, $4, $5, $6)`,
			t.ID, p.ID, t.AccountID, t.Platform, t.Status,
			jsonObject(t.PlatformSpecificData),
		); err != nil {
			return fmt.Errorf("insert target %s: %w", t.ID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// GetPost busca um post com seus targets, validando o escopo de team.
func (s *Store) GetPost(ctx context.Context, teamID, id string) (*domain.Post, error) {
	var p domain.Post
	var mediaJSON []byte

	err := s.db.QueryRow(ctx,
		`SELECT p.id, p.profile_id, p.content, p.media_urls,
		        p.scheduled_for, p.timezone, p.status, p.content_hash,
		        p.created_by, p.created_at, p.updated_at
		   FROM posts p
		   JOIN profiles pr ON pr.id = p.profile_id
		  WHERE p.id = $1 AND pr.team_id = $2`,
		id, teamID,
	).Scan(&p.ID, &p.ProfileID, &p.Content, &mediaJSON,
		&p.ScheduledFor, &p.Timezone, &p.Status, &p.ContentHash,
		&p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal(mediaJSON, &p.MediaURLs)

	targets, err := s.ListTargets(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	p.Platforms = targets
	return &p, nil
}

// ListTargets retorna os targets de um post (ordem de criação).
func (s *Store) ListTargets(ctx context.Context, postID string) ([]domain.PostTarget, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, post_id, account_id, platform, status,
		        platform_specific_data, published_url, external_post_id, last_error
		   FROM post_targets
		  WHERE post_id = $1
		  ORDER BY created_at`,
		postID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []domain.PostTarget
	for rows.Next() {
		var t domain.PostTarget
		var spdJSON []byte
		var lastErrJSON []byte
		var publishedURL, externalID *string
		if err := rows.Scan(&t.ID, &t.PostID, &t.AccountID, &t.Platform, &t.Status,
			&spdJSON, &publishedURL, &externalID, &lastErrJSON); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(spdJSON, &t.PlatformSpecificData)
		if publishedURL != nil {
			t.PublishedURL = *publishedURL
		}
		if externalID != nil {
			t.ExternalPostID = *externalID
		}
		if lastErrJSON != nil {
			var te domain.TargetError
			if json.Unmarshal(lastErrJSON, &te) == nil {
				t.LastError = &te
			}
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

// ListPosts pagina posts do team (GET /v1/posts) com total.
// Busca targets de todos os posts numa única query (= ANY) para evitar N+1.
func (s *Store) ListPosts(ctx context.Context, teamID string, limit, offset int) ([]domain.Post, int, error) {
	var total int
	if err := s.db.QueryRow(ctx,
		`SELECT count(*)
		   FROM posts p JOIN profiles pr ON pr.id = p.profile_id
		  WHERE pr.team_id = $1`, teamID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.Query(ctx,
		`SELECT p.id, p.profile_id, p.content, p.media_urls,
		        p.scheduled_for, p.timezone, p.status, p.content_hash,
		        p.created_by, p.created_at, p.updated_at
		   FROM posts p JOIN profiles pr ON pr.id = p.profile_id
		  WHERE pr.team_id = $1
		  ORDER BY p.created_at DESC
		  LIMIT $2 OFFSET $3`,
		teamID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	posts := make([]domain.Post, 0, limit)
	postIndex := map[string]int{}
	for rows.Next() {
		var p domain.Post
		var mediaJSON []byte
		if err := rows.Scan(&p.ID, &p.ProfileID, &p.Content, &mediaJSON,
			&p.ScheduledFor, &p.Timezone, &p.Status, &p.ContentHash,
			&p.CreatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal(mediaJSON, &p.MediaURLs)
		postIndex[p.ID] = len(posts)
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if len(posts) == 0 {
		return posts, total, nil
	}

	ids := make([]string, 0, len(posts))
	for i := range posts {
		ids = append(ids, posts[i].ID)
	}

	trows, err := s.db.Query(ctx,
		`SELECT id, post_id, account_id, platform, status,
		        platform_specific_data, published_url, external_post_id, last_error
		   FROM post_targets
		  WHERE post_id = ANY($1)
		  ORDER BY created_at`,
		ids,
	)
	if err != nil {
		return nil, 0, err
	}
	defer trows.Close()

	for trows.Next() {
		var t domain.PostTarget
		var spdJSON []byte
		var lastErrJSON []byte
		var publishedURL, externalID *string
		if err := trows.Scan(&t.ID, &t.PostID, &t.AccountID, &t.Platform, &t.Status,
			&spdJSON, &publishedURL, &externalID, &lastErrJSON); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal(spdJSON, &t.PlatformSpecificData)
		if publishedURL != nil {
			t.PublishedURL = *publishedURL
		}
		if externalID != nil {
			t.ExternalPostID = *externalID
		}
		if lastErrJSON != nil {
			var te domain.TargetError
			if json.Unmarshal(lastErrJSON, &te) == nil {
				t.LastError = &te
			}
		}
		if idx, ok := postIndex[t.PostID]; ok {
			posts[idx].Platforms = append(posts[idx].Platforms, t)
		}
	}
	return posts, total, trows.Err()
}

// PublishContext reúne o que o worker de publicação precisa para UM target
// (ADR-007): o target, o conteúdo do post e a conta com credenciais.
type PublishContext struct {
	PostID  string
	Content string
	Target  domain.PostTarget
	Account domain.SocialAccount
}

// GetPublishContext busca o target + conteúdo do post + conta (com credenciais
// criptografadas) em uma única query.
func (s *Store) GetPublishContext(ctx context.Context, targetID string) (*PublishContext, error) {
	var (
		pc            PublishContext
		spdJSON       []byte
		lastErrJSON   []byte
		publishedURL  *string
		externalID    *string
		displayName   *string
		platformUser  *string
		extIdent      *string
		tokenScope    *string
	)

	err := s.db.QueryRow(ctx,
		`SELECT t.id, t.post_id, t.account_id, t.platform, t.status,
		        t.platform_specific_data, t.published_url, t.external_post_id, t.last_error,
		        p.content,
		        a.id, a.profile_id, a.platform, a.display_name, a.platform_user_id,
		        a.token_status, a.encrypted_token, a.refresh_token_encrypted, a.expires_at,
		        a.external_identifier, a.token_scope
		   FROM post_targets t
		   JOIN posts p ON p.id = t.post_id
		   JOIN social_accounts a ON a.id = t.account_id
		  WHERE t.id = $1`,
		targetID,
	).Scan(&pc.Target.ID, &pc.Target.PostID, &pc.Target.AccountID, &pc.Target.Platform, &pc.Target.Status,
		&spdJSON, &publishedURL, &externalID, &lastErrJSON,
		&pc.Content,
		&pc.Account.ID, &pc.Account.ProfileID, &pc.Account.Platform, &displayName, &platformUser,
		&pc.Account.TokenStatus, &pc.Account.EncryptedToken, &pc.Account.RefreshTokenEncrypted, &pc.Account.ExpiresAt,
		&extIdent, &tokenScope)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	pc.PostID = pc.Target.PostID
	_ = json.Unmarshal(spdJSON, &pc.Target.PlatformSpecificData)
	if publishedURL != nil {
		pc.Target.PublishedURL = *publishedURL
	}
	if externalID != nil {
		pc.Target.ExternalPostID = *externalID
	}
	if lastErrJSON != nil {
		var te domain.TargetError
		if json.Unmarshal(lastErrJSON, &te) == nil {
			pc.Target.LastError = &te
		}
	}
	if displayName != nil {
		pc.Account.DisplayName = *displayName
	}
	if platformUser != nil {
		pc.Account.PlatformUserID = *platformUser
	}
	if extIdent != nil {
		pc.Account.ExternalIdentifier = *extIdent
	}
	if tokenScope != nil {
		pc.Account.TokenScope = *tokenScope
	}
	return &pc, nil
}

// MarkTargetFailed marca um target como failed com o erro tipado (ADR-007 §1).
func (s *Store) MarkTargetFailed(ctx context.Context, targetID, postID, code, message string) error {
	lastErr := jsonObject(map[string]string{"code": code, "error": message})
	_, err := s.db.Exec(ctx,
		`UPDATE post_targets
		    SET status = 'failed', last_error = $3, updated_at = now()
		  WHERE id = $1 AND post_id = $2`,
		targetID, postID, lastErr,
	)
	return err
}

// ListTargetStatuses devolve os status dos targets de um post (para derivar o
// status agregado — ADR-007 §1).
func (s *Store) ListTargetStatuses(ctx context.Context, postID string) ([]domain.TargetStatus, error) {
	rows, err := s.db.Query(ctx,
		`SELECT status FROM post_targets WHERE post_id = $1 ORDER BY created_at`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.TargetStatus
	for rows.Next() {
		var st domain.TargetStatus
		if err := rows.Scan(&st); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// MarkTargetPublished simula o sucesso de publicação de um target.
// Fase 3: chamado pelo conector real após publicar na rede (ADR-007).
func (s *Store) MarkTargetPublished(ctx context.Context, targetID, postID, publishedURL, externalPostID string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE post_targets
		    SET status = 'published', published_url = $2, external_post_id = $3,
		        updated_at = now()
		  WHERE id = $1 AND post_id = $4`,
		targetID, publishedURL, externalPostID, postID,
	)
	return err
}

// UpdatePostStatus materializa o status agregado derivado dos targets
// (ADR-007 §1: o agregado é função dos targets; recomputado após o fan-out).
func (s *Store) UpdatePostStatus(ctx context.Context, postID string, status domain.PostStatus) error {
	_, err := s.db.Exec(ctx,
		`UPDATE posts SET status = $2, updated_at = now() WHERE id = $1`,
		postID, status,
	)
	return err
}

// RecordAttempt registra uma tentativa de publicação (log append-only — ADR-007).
func (s *Store) RecordAttempt(ctx context.Context, a *domain.PublishAttempt) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO publish_attempts
		     (id, target_id, attempt, started_at, finished_at, outcome, error, http_status, request_id)
		   VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		a.ID, a.TargetID, a.Attempt, a.StartedAt, a.FinishedAt, a.Outcome,
		jsonObject(a.Error), a.HTTPStatus, a.RequestID,
	)
	return err
}
