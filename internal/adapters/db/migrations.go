package db

import (
	"context"
	"fmt"
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"

	"pkb/migrations"
)

var migrateMu sync.Mutex

// Migrate applies embedded SQL migrations to the SQLite database.
func Migrate(ctx context.Context, database *sqlx.DB) error {
	if database == nil {
		return fmt.Errorf("database is nil")
	}

	migrateMu.Lock()
	defer migrateMu.Unlock()

	goose.SetBaseFS(migrations.Migrations)
	defer goose.SetBaseFS(nil)

	goose.SetLogger(goose.NopLogger())

	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	if err := goose.UpContext(ctx, database.DB, "."); err != nil {
		return fmt.Errorf("apply sqlite migrations: %w", err)
	}

	return nil
}
