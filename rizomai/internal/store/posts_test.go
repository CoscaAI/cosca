// Testes do repositório de posts com pgxmock (valida o SQL SEM banco real).
package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/rizomai/rizomai/internal/domain"
)

func testPost() (*domain.Post, []domain.PostTarget) {
	now := time.Now().UTC()
	post := &domain.Post{
		ID:        "post_1",
		ProfileID: "profile_1",
		Content:   "olá mundo",
		MediaURLs: []string{"https://cdn.rizomai.app/media_1"},
		Timezone:  "UTC",
		Status:    domain.PostStatusScheduled,
		CreatedAt: now,
		UpdatedAt: now,
	}
	targets := []domain.PostTarget{
		{ID: "target_1", PostID: "post_1", Platform: domain.PlatformX, AccountID: "account_1", Status: domain.TargetStatusPending},
		{ID: "target_2", PostID: "post_1", Platform: domain.PlatformLinkedIn, AccountID: "account_2", Status: domain.TargetStatusPending},
	}
	return post, targets
}

func TestCreatePostWithTargets(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	s := &Store{db: mock}

	post, targets := testPost()

	// Nota pgxmock v4: args chegam crus — jsonb serializado vira []byte e
	// ponteiros nulos viram (*time.Time)(nil), não nil puro.
	mediaJSON := []byte(`["https://cdn.rizomai.app/media_1"]`)
	emptyObj := []byte(`{}`)
	nilTime := (*time.Time)(nil)

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO posts`).
		WithArgs("post_1", "profile_1", "olá mundo", mediaJSON, nilTime, "UTC",
			domain.PostStatusScheduled, "", "").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec(`INSERT INTO post_targets`).
		WithArgs("target_1", "post_1", "account_1", domain.PlatformX,
			domain.TargetStatusPending, emptyObj).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec(`INSERT INTO post_targets`).
		WithArgs("target_2", "post_1", "account_2", domain.PlatformLinkedIn,
			domain.TargetStatusPending, emptyObj).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	if err := s.CreatePostWithTargets(context.Background(), post, targets); err != nil {
		t.Fatalf("CreatePostWithTargets: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectativas não cumpridas: %v", err)
	}
}

func TestCreatePostWithTargetsRollbackOnTargetError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	s := &Store{db: mock}

	post, targets := testPost()
	mediaJSON := []byte(`["https://cdn.rizomai.app/media_1"]`)
	emptyObj := []byte(`{}`)
	nilTime := (*time.Time)(nil)

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO posts`).
		WithArgs("post_1", "profile_1", "olá mundo", mediaJSON, nilTime, "UTC",
			domain.PostStatusScheduled, "", "").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec(`INSERT INTO post_targets`).
		WithArgs("target_1", "post_1", "account_1", domain.PlatformX,
			domain.TargetStatusPending, emptyObj).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	// 2º target falha (ex.: unique violation) → transação deve reverter TUDO.
	mock.ExpectExec(`INSERT INTO post_targets`).
		WithArgs("target_2", "post_1", "account_2", domain.PlatformLinkedIn,
			domain.TargetStatusPending, emptyObj).
		WillReturnError(errors.New("duplicate key value violates unique constraint"))
	mock.ExpectRollback()

	if err := s.CreatePostWithTargets(context.Background(), post, targets); err == nil {
		t.Fatal("esperava erro com falha no INSERT de target")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectativas não cumpridas: %v", err)
	}
}
