package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

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

func (r *Repository) CreateClassificationJob(ctx context.Context, sourceMessageID int64) (int64, error) {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO processing_jobs (job_type, source_message_id, status)
		VALUES (?, ?, ?)
	`, domain.JobTypeClassifyMessage, sourceMessageID, domain.JobStatusPending)
	if err != nil {
		return 0, fmt.Errorf("insert classification job: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read inserted classification job id: %w", err)
	}

	return id, nil
}

func (r *Repository) GetPendingClassificationJob(ctx context.Context) (*domain.Job, error) {
	var row struct {
		JobID                  int64             `db:"job_id"`
		JobType                domain.JobType    `db:"job_type"`
		JobStatus              domain.JobStatus  `db:"job_status"`
		JobAttempts            int               `db:"attempts"`
		JobError               sql.NullString    `db:"error"`
		JobStartedAt           sql.NullString    `db:"started_at"`
		JobFinishedAt          sql.NullString    `db:"finished_at"`
		JobCreatedAt           string            `db:"job_created_at"`
		JobUpdatedAt           string            `db:"job_updated_at"`
		SourceMessageID        int64             `db:"source_message_id"`
		SourceMessageType      domain.SourceType `db:"source_message_type"`
		SourceMessageRawText   string            `db:"raw_text"`
		SourceMessageCreatedAt string            `db:"message_created_at"`
	}

	err := r.db.QueryRowxContext(ctx, `
		SELECT
			j.id AS job_id,
			j.job_type,
			j.status AS job_status,
			j.attempts,
			j.error,
			j.started_at,
			j.finished_at,
			j.created_at AS job_created_at,
			j.updated_at AS job_updated_at,
			m.id AS source_message_id,
			m.source_type AS source_message_type,
			m.raw_text,
			m.created_at AS message_created_at
		FROM processing_jobs j
		JOIN source_messages m ON m.id = j.source_message_id
		WHERE j.job_type = ? AND j.status = ?
		ORDER BY j.created_at, j.id
		LIMIT 1
	`, domain.JobTypeClassifyMessage, domain.JobStatusPending).StructScan(&row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("select pending classification job: %w", err)
	}

	jobCreatedAt, err := parseSQLiteTime(row.JobCreatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse job created_at: %w", err)
	}
	jobUpdatedAt, err := parseSQLiteTime(row.JobUpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse job updated_at: %w", err)
	}
	messageCreatedAt, err := parseSQLiteTime(row.SourceMessageCreatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse source message created_at: %w", err)
	}

	var jobError *string
	if row.JobError.Valid {
		jobError = &row.JobError.String
	}

	var startedAt *time.Time
	if row.JobStartedAt.Valid {
		parsed, err := parseSQLiteTime(row.JobStartedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse job started_at: %w", err)
		}
		startedAt = &parsed
	}

	var finishedAt *time.Time
	if row.JobFinishedAt.Valid {
		parsed, err := parseSQLiteTime(row.JobFinishedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse job finished_at: %w", err)
		}
		finishedAt = &parsed
	}

	return &domain.Job{
		ID:      row.JobID,
		JobType: row.JobType,
		SourceMessage: &domain.SourceMessage{
			ID:         row.SourceMessageID,
			SourceType: row.SourceMessageType,
			RawText:    row.SourceMessageRawText,
			CreatedAt:  messageCreatedAt,
		},
		Status:     row.JobStatus,
		Attempts:   row.JobAttempts,
		Error:      jobError,
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		CreatedAt:  jobCreatedAt,
		UpdatedAt:  jobUpdatedAt,
	}, nil
}

func (r *Repository) SetJobStatus(ctx context.Context, jobID int64, status domain.JobStatus) error {
	if !isValidJobStatus(status) {
		return fmt.Errorf("set job status: unknown status %q", status)
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE processing_jobs
		SET status = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, jobID)
	if err != nil {
		return fmt.Errorf("set job status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read set job status rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("set job status: job %d does not exist", jobID)
	}

	return nil
}

func parseSQLiteTime(value string) (time.Time, error) { // TODO проверить, есть ли у этой функции готовая реализация в sqlx, если что поменять эту функцию
	if value == "" {
		return time.Time{}, errors.New("empty time value")
	}

	layouts := []string{
		"2006-01-02 15:04:05",
		time.RFC3339Nano,
	}

	var lastErr error
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
		lastErr = err
	}

	return time.Time{}, lastErr
}

func isValidJobStatus(status domain.JobStatus) bool {
	switch status {
	case domain.JobStatusPending,
		domain.JobStatusRunning,
		domain.JobStatusDone,
		domain.JobStatusFailed:
		return true
	default:
		return false
	}
}
