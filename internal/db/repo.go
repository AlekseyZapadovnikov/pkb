package db

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"pkb/internal/usecase/domain"
)

// Repository хранит исходные сообщения в SQLite.
type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) SaveMessage(ctx context.Context, msg *domain.SourceMessage) (id int64, err error) {

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

func (r *Repository) SaveTopic(ctx context.Context, topic *domain.Topic) (int64, error) {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO topics (slug, name, description)
		VALUES (?, ?, ?)
	`, topic.Slug, topic.Name, topic.Description)
	if err != nil {
		return 0, fmt.Errorf("insert topic: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read inserted topic id: %w", err)
	}

	return id, nil
}

func (r *Repository) GetAllTopics(ctx context.Context) ([]*domain.Topic, error) {
	rows, err := r.db.QueryxContext(ctx, `
		SELECT id, slug, name, description
		FROM topics
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("select topics: %w", err)
	}
	defer rows.Close()

	topics := make([]*domain.Topic, 0)
	for rows.Next() {
		var topic domain.Topic
		if err := rows.StructScan(&topic); err != nil {
			return nil, fmt.Errorf("scan topic: %w", err)
		}
		topics = append(topics, &topic)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate topics: %w", err)
	}

	return topics, nil
}

func (r *Repository) DeleteTopic(ctx context.Context, slug string) error {
	if _, err := r.db.ExecContext(ctx, `
		DELETE FROM topics
		WHERE slug = ?
	`, slug); err != nil {
		return fmt.Errorf("delete topic: %w", err)
	}

	return nil
}
