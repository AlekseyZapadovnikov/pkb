package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sync"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

var migrateMu sync.Mutex

// Migrate применяет встроенные SQL-миграции к SQLite database.
func Migrate(ctx context.Context, database *sql.DB) error {
	if database == nil {
		return fmt.Errorf("database is nil")
	}

	migrateMu.Lock()
	defer migrateMu.Unlock()

	goose.SetBaseFS(migrationsFS)
	defer goose.SetBaseFS(nil)

	goose.SetLogger(goose.NopLogger())

	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	if err := goose.UpContext(ctx, database, "migrations"); err != nil {
		return fmt.Errorf("apply sqlite migrations: %w", err)
	}

	return nil
}
