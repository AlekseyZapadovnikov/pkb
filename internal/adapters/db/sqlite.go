package db

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"

	"github.com/jmoiron/sqlx"
	"github.com/mattn/go-sqlite3"

	"pkb/internal/usecase/service/config"
	"pkb/internal/usecase/service/valid"
)

var driverSeq atomic.Uint64

// Open opens SQLite, loads sqlite-vector and sqlite-memory,
// and configures the local embedding model.
func Open(
	ctx context.Context,
	path string,
	memoryPaths config.MemoryPaths,
) (*sqlx.DB, error) {
	if err := valid.Vl.Struct(memoryPaths); err != nil {
		return nil, fmt.Errorf("invalid memory paths: %w", err)
	}
	driverName := fmt.Sprintf(
		"sqlite3_sqlite_memory_%d",
		driverSeq.Add(1),
	)

	sql.Register(driverName, &sqlite3.SQLiteDriver{
		Extensions: []string{
			memoryPaths.Vector,
			memoryPaths.Memory,
		},
	})

	database, err := sqlx.Open(driverName, path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	database.SetMaxOpenConns(1)

	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}

	if err := configure(ctx, database, memoryPaths.Model); err != nil {
		_ = database.Close()
		return nil, err
	}
	if err := checkMemoryEnabled(ctx, database); err != nil {
		_ = database.Close()
		return nil, err
	}

	return database, nil
}

func checkMemoryEnabled(ctx context.Context, database *sqlx.DB) error {
	var enabled bool
	if err := database.GetContext(ctx, &enabled, `SELECT memory_is_enabled()`); err != nil {
		return fmt.Errorf("check knowledge search index: %w", err)
	}
	if !enabled {
		return fmt.Errorf("sqlite-memory schema is unavailable")
	}
	return nil
}

func configure(
	ctx context.Context,
	database *sqlx.DB,
	modelPath string,
) error {
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

	// Check that the FTS5 module is available.
	var hasFTS5 bool
	if err := database.QueryRowContext(ctx, `
			  SELECT EXISTS (
				  SELECT 1 as alias FROM pragma_module_list WHERE name = 'fts5'
			  ) as E
		  `).Scan(&hasFTS5); err != nil {
		return fmt.Errorf("check FTS5 availability: %w", err)
	}
	if !hasFTS5 {
		return fmt.Errorf("FTS5 is unavailable: rebuild with make build")
	}

	// Load the local GGUF embedding model.
	// The configuration is stored in the database, but repeating the call at startup is safe.
	if _, err := database.ExecContext(
		ctx,
		`SELECT memory_set_model('local', ?) as msm`,
		modelPath,
	); err != nil {
		return fmt.Errorf("configure sqlite-memory model: %w", err)
	}

	// knowledge_items will be passed as virtual files:
	// knowledge-items/123.md, knowledge-items/124.md, etc.
	if _, err := database.ExecContext(
		ctx,
		`SELECT memory_set_option('preserve_duplicate_paths', 1) as mso`,
	); err != nil {
		return fmt.Errorf("configure sqlite-memory paths: %w", err)
	}

	return nil
}
