// Billing: metering account-day e limites de plano (ADR-010).
//
// Metering: cada conta conectada e ativa conta 1 unidade por dia
// (usage_account_days). A agregação é IDEMPOTENTE (unique team+account+day —
// ADR-010 §1.2). O limite do plano (contas_conectadas) é checado na conexão de
// contas (connect/callback/credentials → 402 PLAN_LIMIT_EXCEEDED).
package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/rizomai/rizomai/internal/domain"
)

// ErrPlanLimitExceeded indica que o team atingiu o limite de contas do plano.
var ErrPlanLimitExceeded = errors.New("store: plano excedeu o limite de contas conectadas")

// EnsureDefaultPlans garante o catálogo de planos (idempotente — o seed da
// migration já insere; aqui cobre o caso de plano ausente).
func (s *Store) EnsureDefaultPlans(ctx context.Context) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO plans (codename, name, contas_incluidas, preco_por_conta_extra_cents, preco_fixo_cents, features) VALUES
		   ('free', 'Free', 3, 0, 0, '{"postsIlimitados": true}'::jsonb),
		   ('growth', 'Growth', 3, 400, 0, '{"postsIlimitados": true}'::jsonb),
		   ('escala', 'Escala', 3, 200, 0, '{"postsIlimitados": true}'::jsonb),
		   ('agencia', 'Agência', 10, 0, 4900, '{"postsIlimitados": true, "whiteLabel": true}'::jsonb)
		   ON CONFLICT (codename) DO NOTHING`)
	return err
}

// GetPlan devolve um plano do catálogo pelo codename (ex.: growth).
func (s *Store) GetPlan(ctx context.Context, codename string) (*domain.BillingPlan, error) {
	var p domain.BillingPlan
	err := s.db.QueryRow(ctx,
		`SELECT codename, name, contas_incluidas, preco_por_conta_extra_cents, preco_fixo_cents, COALESCE(stripe_price_id, ''), features
		   FROM plans WHERE codename = $1`,
		codename,
	).Scan(&p.Codename, &p.Name, &p.ContasIncluidas, &p.PrecoPorContaExtraCents, &p.PrecoFixoCents, &p.StripePriceID, &p.Features)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// SetTeamPlan define o plano do team (default free) e o período corrente.
func (s *Store) SetTeamPlan(ctx context.Context, teamID, planID string, periodEnd *time.Time) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO team_plans (team_id, plan_id, current_period_start, current_period_end)
		   VALUES ($1, $2, now(), $3)
		   ON CONFLICT (team_id) DO UPDATE
		     SET plan_id = EXCLUDED.plan_id,
		         current_period_end = COALESCE(EXCLUDED.current_period_end, team_plans.current_period_end),
		         updated_at = now()`,
		teamID, planID, periodEnd,
	)
	return err
}

// GetTeamPlan devolve o plano do team (com fallback free se nunca definido).
func (s *Store) GetTeamPlan(ctx context.Context, teamID string) (*domain.TeamPlan, error) {
	var tp domain.TeamPlan
	err := s.db.QueryRow(ctx,
		`SELECT tp.team_id, tp.plan_id, p.name, p.contas_incluidas,
		        p.preco_por_conta_extra_cents, p.preco_fixo_cents, p.stripe_price_id, p.features,
		        tp.status, tp.billing_anchor_day, tp.current_period_start, tp.current_period_end
		   FROM team_plans tp JOIN plans p ON p.codename = tp.plan_id
		  WHERE tp.team_id = $1`,
		teamID,
	).Scan(&tp.TeamID, &tp.Plan.Codename, &tp.Plan.Name, &tp.Plan.ContasIncluidas,
		&tp.Plan.PrecoPorContaExtraCents, &tp.Plan.PrecoFixoCents, &tp.Plan.StripePriceID, &tp.Plan.Features,
		&tp.Status, &tp.BillingAnchorDay, &tp.CurrentPeriodStart, &tp.CurrentPeriodEnd)
	if errors.Is(err, pgx.ErrNoRows) {
		// Sem registro: default Free.
		return &domain.TeamPlan{
			TeamID: teamID,
			Plan: domain.BillingPlan{
				Codename: "free", Name: "Free", ContasIncluidas: 3,
				Features: map[string]any{"postsIlimitados": true},
			},
			Status:           "active",
			BillingAnchorDay: 1,
			CurrentPeriodStart: time.Now().UTC().Truncate(24 * time.Hour),
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return &tp, nil
}

// RecordAccountDay registra o uso account-day (upsert idempotente — ADR-010 §1.2).
func (s *Store) RecordAccountDay(ctx context.Context, teamID, accountID string, day time.Time) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO usage_account_days (team_id, account_id, day, active)
		   VALUES ($1, $2, $3, TRUE)
		   ON CONFLICT (team_id, account_id, day) DO UPDATE SET active = TRUE`,
		teamID, accountID, day.UTC().Format("2006-01-02"),
	)
	return err
}

// CountActiveAccounts conta as contas conectadas ATIVAS do team
// (token_status != revoked) — base do limite do plano.
func (s *Store) CountActiveAccounts(ctx context.Context, teamID string) (int, error) {
	var n int
	err := s.db.QueryRow(ctx,
		`SELECT count(*) FROM social_accounts a JOIN profiles pr ON pr.id = a.profile_id
		  WHERE pr.team_id = $1 AND a.token_status <> 'revoked'`,
		teamID,
	).Scan(&n)
	return n, err
}

// CanConnectAccount valida o limite do plano ao conectar uma conta.
// Se a conta já existe (mesma plataforma no mesmo profile), é atualização de
// token e o limite NÃO é re-checado (upsert). Retorna (ok, current, limit).
func (s *Store) CanConnectAccount(ctx context.Context, teamID, profileID string, p domain.Platform) (bool, int, int, error) {
	// Atualização de conta existente → sempre permitido.
	existing, err := s.accountIDByPlatform(ctx, profileID, p)
	if err != nil {
		return false, 0, 0, err
	}
	if existing != "" {
		return true, 0, 0, nil
	}

	plan, err := s.GetTeamPlan(ctx, teamID)
	if err != nil {
		return false, 0, 0, err
	}
	current, err := s.CountActiveAccounts(ctx, teamID)
	if err != nil {
		return false, 0, 0, err
	}
	return current < plan.Plan.ContasIncluidas, current, plan.Plan.ContasIncluidas, nil
}

// ComputePeriodUsage soma os account-days ativos do team no período
// (para fatura — ADR-010 §1.2).
func (s *Store) ComputePeriodUsage(ctx context.Context, teamID string, periodStart, periodEnd time.Time) (int, error) {
	var n int
	err := s.db.QueryRow(ctx,
		`SELECT count(*) FROM usage_account_days
		  WHERE team_id = $1 AND active = TRUE AND day >= $2 AND day <= $3`,
		teamID, periodStart.UTC().Format("2006-01-02"), periodEnd.UTC().Format("2006-01-02"),
	).Scan(&n)
	return n, err
}

// RecordMeteringEvent grava o snapshot diário (idempotente — ADR-010 §1.2).
func (s *Store) RecordMeteringEvent(ctx context.Context, teamID string, day time.Time, contasAtivas int, valorCents int64) error {
	id, err := domain.NewMeteringID()
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx,
		`INSERT INTO metering_events (id, team_id, day, contas_ativas, valor_cents)
		   VALUES ($1, $2, $3, $4, $5)
		   ON CONFLICT (team_id, day) DO UPDATE
		     SET contas_ativas = EXCLUDED.contas_ativas, valor_cents = EXCLUDED.valor_cents`,
		id, teamID, day.UTC().Format("2006-01-02"), contasAtivas, valorCents,
	)
	return err
}
