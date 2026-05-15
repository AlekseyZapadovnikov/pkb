package db

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"pkb/internal/usecase/domain"
)

// MessageRepository хранит исходные сообщения в SQLite.
type MessageRepository struct {
	db *sqlx.DB
}

func NewMessageRepository(db *sqlx.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) SaveMessage(ctx context.Context, msg *domain.SourceMessage) (id int64, err error) {

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO source_messages (source_type, raw_text)
		VALUES (?, ?)
	`, msg.SourceType, msg.RawText,
	)
	if err != nil {
		return 0, fmt.Errorf("insert source message: %w", err)
	}

	id, err = result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read inserted source message id: %w", err)
	}
	msg.ID = id

	return id, nil
}
