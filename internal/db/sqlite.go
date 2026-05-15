package db

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

const driverName = "sqlite"

// Open открывает SQLite через sqlx и применяет настройки для локального storage.
func Open(ctx context.Context, path string) (*sqlx.DB, error) {
	if path == "" {
		return nil, fmt.Errorf("sqlite path is empty")
	}

	database, err := sqlx.Open(driverName, path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	database.SetMaxOpenConns(1)

	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}

	if err := configure(ctx, database); err != nil {
		_ = database.Close()
		return nil, err
	}

	return database, nil
}

func configure(ctx context.Context, database *sqlx.DB) error {
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
	}

	for _, pragma := range pragmas {
		if _, err := database.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("apply sqlite pragma %q: %w", pragma, err)
		}
	}

	return nil
}
