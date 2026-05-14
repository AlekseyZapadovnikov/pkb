package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenAndMigrate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	database, err := Open(ctx, filepath.Join(t.TempDir(), "pkb.sqlite3"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	if err := Migrate(ctx, database); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := Migrate(ctx, database); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	wantTables := []string{
		"goose_db_version",
		"source_messages",
		"processing_jobs",
		"knowledge_items",
		"unknown_items",
		"topics",
		"knowledge_item_topics",
	}
	for _, table := range wantTables {
		if !tableExists(ctx, t, database, table) {
			t.Fatalf("table %q does not exist", table)
		}
	}

	var foreignKeysEnabled int
	if err := database.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeysEnabled); err != nil {
		t.Fatalf("query foreign_keys pragma: %v", err)
	}
	if foreignKeysEnabled != 1 {
		t.Fatalf("foreign_keys pragma = %d, want 1", foreignKeysEnabled)
	}

	_, err = database.ExecContext(ctx, `
		INSERT INTO knowledge_items (source_message_id, category, title, body, confidence)
		VALUES (999, 'article', 'Title', 'Body', 0.9)
	`)
	if err == nil {
		t.Fatalf("insert knowledge_item with missing source_message_id succeeded, want foreign key error")
	}
}

func tableExists(ctx context.Context, t *testing.T, database *sql.DB, table string) bool {
	t.Helper()

	var count int
	err := database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = 'table' AND name = ?
	`, table).Scan(&count)
	if err != nil {
		t.Fatalf("query sqlite_master for table %q: %v", table, err)
	}

	return count == 1
}
