// Analytics: post_analytics (Fase 6 — agregação de métricas por post/plataforma).
package store

import (
	"context"
	"time"

	"github.com/rizomai/rizomai/internal/domain"
)

// UpsertPostAnalytics grava o snapshot diário (idempotente por post+platform+day).
func (s *Store) UpsertPostAnalytics(ctx context.Context, a *domain.PostAnalytics) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO post_analytics (post_id, platform, external_id, views, likes, comments, shares, captured_at)
		   VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		   ON CONFLICT (post_id, platform, captured_at) DO UPDATE
		     SET views = EXCLUDED.views, likes = EXCLUDED.likes,
		         comments = EXCLUDED.comments, shares = EXCLUDED.shares`,
		a.PostID, a.Platform, nullableString(a.ExternalID),
		a.Views, a.Likes, a.Comments, a.Shares, a.CapturedAt,
	)
	return err
}

// GetPostAnalytics devolve as métricas agregadas de um post (por plataforma),
// validando o escopo de team.
func (s *Store) GetPostAnalytics(ctx context.Context, teamID, postID string) ([]domain.PostAnalytics, error) {
	rows, err := s.db.Query(ctx,
		`SELECT pa.post_id, pa.platform, pa.external_id,
		        sum(pa.views), sum(pa.likes), sum(pa.comments), sum(pa.shares), max(pa.captured_at)
		   FROM post_analytics pa
		   JOIN posts po ON po.id = pa.post_id
		   JOIN profiles pr ON pr.id = po.profile_id
		  WHERE pa.post_id = $1 AND pr.team_id = $2
		  GROUP BY pa.post_id, pa.platform, pa.external_id`,
		postID, teamID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.PostAnalytics
	for rows.Next() {
		var a domain.PostAnalytics
		var captured time.Time
		if err := rows.Scan(&a.PostID, &a.Platform, &a.ExternalID,
			&a.Views, &a.Likes, &a.Comments, &a.Shares, &captured); err != nil {
			return nil, err
		}
		a.CapturedAt = captured.Format("2006-01-02")
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetProfileAnalytics agrega as métricas de um profile nos últimos `days`
// dias, por plataforma (dados de post_analytics).
func (s *Store) GetProfileAnalytics(ctx context.Context, teamID, profileID string, days int) ([]domain.ProfileAnalytics, error) {
	if days <= 0 {
		days = 30
	}
	since := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02")

	rows, err := s.db.Query(ctx,
		`SELECT pa.platform, sum(pa.views), sum(pa.likes), sum(pa.comments), sum(pa.shares), count(DISTINCT pa.post_id)
		   FROM post_analytics pa
		   JOIN posts po ON po.id = pa.post_id
		   JOIN profiles pr ON pr.id = po.profile_id
		  WHERE po.profile_id = $1 AND pr.team_id = $2 AND pa.captured_at >= $3
		  GROUP BY pa.platform
		  ORDER BY sum(pa.views) DESC`,
		profileID, teamID, since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.ProfileAnalytics
	for rows.Next() {
		var a domain.ProfileAnalytics
		if err := rows.Scan(&a.Platform, &a.Views, &a.Likes, &a.Comments, &a.Shares, &a.PostsCounted); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// SeedPostAnalytics insere snapshots fictícios para o post demo (modo demo).
func (s *Store) SeedPostAnalytics(ctx context.Context, postID string, platforms []domain.Platform) error {
	day := time.Now().UTC().Format("2006-01-02")
	samples := []domain.PostAnalytics{
		{Views: 1250, Likes: 87, Comments: 12, Shares: 5},
		{Views: 980, Likes: 64, Comments: 8, Shares: 3},
		{Views: 2200, Likes: 143, Comments: 21, Shares: 11},
	}
	for i, p := range platforms {
		sp := samples[i%len(samples)]
		sp.PostID = postID
		sp.Platform = string(p)
		sp.ExternalID = "sim_" + postID
		sp.CapturedAt = day
		if err := s.UpsertPostAnalytics(ctx, &sp); err != nil {
			return err
		}
	}
	return nil
}
