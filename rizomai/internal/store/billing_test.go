// Testes do metering billing com pgxmock (ADR-010 §1.2 — sem banco real).
package store

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/rizomai/rizomai/internal/domain"
)

func TestRecordAccountDayUpsert(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	s := &Store{db: mock}

	day := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	mock.ExpectExec(`INSERT INTO usage_account_days`).
		WithArgs("team_1", "account_1", "2026-09-02").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	if err := s.RecordAccountDay(context.Background(), "team_1", "account_1", day); err != nil {
		t.Fatalf("RecordAccountDay: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectativas não cumpridas: %v", err)
	}
}

func TestCanConnectAccountExistingAllows(t *testing.T) {
	// Conta já existe na plataforma (upsert de token) → sempre permitido.
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	s := &Store{db: mock}

	mock.ExpectQuery(`SELECT id FROM social_accounts`).
		WithArgs("profile_1", domain.PlatformX).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("account_1"))

	ok, _, _, err := s.CanConnectAccount(context.Background(), "team_1", "profile_1", domain.PlatformX)
	if err != nil {
		t.Fatalf("CanConnectAccount: %v", err)
	}
	if !ok {
		t.Error("atualização de conta existente deveria ser permitida (sem re-checar limite)")
	}
}

func TestCanConnectAccountLimitExceeded(t *testing.T) {
	// Nova conta; plano free (limite 3) com 3 contas ativas → bloqueado.
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	s := &Store{db: mock}

	mock.ExpectQuery(`SELECT id FROM social_accounts`).
		WithArgs("profile_1", domain.PlatformX).
		WillReturnRows(pgxmock.NewRows([]string{"id"})) // nenhuma linha → conta nova

	// GetTeamPlan: sem registro → default Free (3 incluídas).
	mock.ExpectQuery(`SELECT tp.team_id`).
		WithArgs("team_1").
		WillReturnError(pgx.ErrNoRows)

	// CountActiveAccounts → 3 (atingiu o limite).
	mock.ExpectQuery(`SELECT count\(\*\) FROM social_accounts`).
		WithArgs("team_1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(3))

	ok, current, limit, err := s.CanConnectAccount(context.Background(), "team_1", "profile_1", domain.PlatformX)
	if err != nil {
		t.Fatalf("CanConnectAccount: %v", err)
	}
	if ok {
		t.Error("3 de 3 contas → deveria bloquear (402 PLAN_LIMIT_EXCEEDED)")
	}
	if current != 3 || limit != 3 {
		t.Errorf("current=%d limit=%d, esperado 3/3", current, limit)
	}
}

func TestComputePeriodUsage(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	s := &Store{db: mock}

	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`SELECT count\(\*\) FROM usage_account_days`).
		WithArgs("team_1", "2026-09-01", "2026-09-30").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(42))

	n, err := s.ComputePeriodUsage(context.Background(), "team_1", start, end)
	if err != nil {
		t.Fatalf("ComputePeriodUsage: %v", err)
	}
	if n != 42 {
		t.Errorf("account-days = %d, esperado 42", n)
	}
}
