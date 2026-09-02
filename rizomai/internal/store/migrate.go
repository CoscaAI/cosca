// Migrations versionadas com golang-migrate, EMBEDDED no binário
// (single-binary — ADR-001). Aplicadas automaticamente no boot do rizomai-api.
package store

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registra o driver pgx5
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"embed"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate aplica as migrations pendentes (idempotente — já aplicadas pulam).
// Usa o driver pgx5 do golang-migrate, então aceita DATABASE_URL nos formatos
// postgres:// ou pgx5:// (converte o scheme).
func Migrate(databaseURL string) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("iofs migrations: %w", err)
	}

	db, err := migrate.NewWithSourceInstance("iofs", src, migrateURL(databaseURL))
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	defer db.Close()

	if err := db.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

func migrateURL(databaseURL string) string {
	switch {
	case strings.HasPrefix(databaseURL, "postgres://"):
		return "pgx5://" + strings.TrimPrefix(databaseURL, "postgres://")
	case strings.HasPrefix(databaseURL, "postgresql://"):
		return "pgx5://" + strings.TrimPrefix(databaseURL, "postgresql://")
	default:
		return databaseURL
	}
}
