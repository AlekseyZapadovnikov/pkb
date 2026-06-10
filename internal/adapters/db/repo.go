package db

import (
	"context"
	"database/sql"
	"encoding/json"
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

type sqlStore interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryxContext(context.Context, string, ...any) (*sqlx.Rows, error)
	QueryRowxContext(context.Context, string, ...any) *sqlx.Row
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) store(ctx context.Context) sqlStore {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return r.db
}

func (r *Repository) SaveMessage(ctx context.Context, msg *domain.SourceMessage) (id int64, err error) {
	store := r.store(ctx)

	result, err := store.ExecContext(ctx, `
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
	store := r.store(ctx)

	result, err := store.ExecContext(ctx, `
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
	store := r.store(ctx)

	rows, err := store.QueryxContext(ctx, `
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
	store := r.store(ctx)

	if _, err := store.ExecContext(ctx, `
		DELETE FROM topics
		WHERE slug = ?
	`, slug); err != nil {
		return fmt.Errorf("delete topic: %w", err)
	}

	return nil
}

func (r *Repository) SaveKnowledgeItem(ctx context.Context, item *domain.KnowledgeItem) (int64, error) {
	if item == nil {
		return 0, errors.New("knowledge item is nil")
	}
	if item.Body == nil {
		return 0, errors.New("knowledge item body is nil")
	}

	var id int64
	err := NewTransactor(r.db).WithTx(ctx, func(ctx context.Context) error {
		var err error
		id, err = r.saveKnowledgeItem(ctx, item)
		return err
	})
	if err != nil {
		return 0, err
	}

	return id, nil
}

func (r *Repository) saveKnowledgeItem(ctx context.Context, item *domain.KnowledgeItem) (int64, error) {
	store := r.store(ctx)
	body := item.Body

	result, err := store.ExecContext(ctx, `
		INSERT INTO knowledge_items (source_message_id, title, body, confidence, data_json)
		VALUES (?, ?, ?, ?, ?)
	`, body.SourceMessageID, body.Title, body.Body, body.Confidence, rawJSONValue(body.DataJSON))
	if err != nil {
		return 0, fmt.Errorf("insert knowledge item: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read inserted knowledge item id: %w", err)
	}

	for _, topic := range body.Topics {
		topicID, err := r.topicID(ctx, store, topic)
		if err != nil {
			return 0, err
		}

		if _, err := store.ExecContext(ctx, `
			INSERT INTO knowledge_item_topics (knowledge_item_id, topic_id)
			VALUES (?, ?)
		`, id, topicID); err != nil {
			return 0, fmt.Errorf("insert knowledge item topic: %w", err)
		}
	}

	item.ID = id

	return id, nil
}

func (r *Repository) SaveUnknownKnowledgeItem(ctx context.Context, item *domain.UnknownKnowledgeItem, body *domain.UnknownKnowledgeItemBody) (int64, error) {
	if item == nil {
		return 0, errors.New("unknown knowledge item is nil")
	}
	if body == nil {
		return 0, errors.New("unknown knowledge item body is nil")
	}

	var suggestTopics any
	if body.SugestTopics != nil {
		data, err := json.Marshal(body.SugestTopics)
		if err != nil {
			return 0, fmt.Errorf("marshal suggest topics: %w", err)
		}
		suggestTopics = string(data)
	}

	store := r.store(ctx)
	result, err := store.ExecContext(ctx, `
		INSERT INTO unknown_knowledge_items (source_message_id, reason, raw_output_json, suggest_topics_json)
		VALUES (?, ?, ?, ?)
	`, item.SourceMessageID, body.Reason, rawJSONValue(body.RawOutputJSON), suggestTopics)
	if err != nil {
		return 0, fmt.Errorf("insert unknown knowledge item: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read inserted unknown knowledge item id: %w", err)
	}

	item.ID = id

	return id, nil
}

func (r *Repository) topicID(ctx context.Context, store sqlStore, topic *domain.Topic) (int64, error) {
	if topic == nil {
		return 0, errors.New("topic is nil")
	}
	if topic.ID != 0 {
		return topic.ID, nil
	}
	if topic.Slug == "" {
		return 0, errors.New("topic id and slug are empty")
	}

	var id int64
	err := store.QueryRowxContext(ctx, `
		SELECT id
		FROM topics
		WHERE slug = ?
	`, topic.Slug).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("topic %q does not exist", topic.Slug)
	}
	if err != nil {
		return 0, fmt.Errorf("select topic id: %w", err)
	}

	return id, nil
}

func (r *Repository) CreateClassificationJob(ctx context.Context, sourceMessageID int64) (int64, error) {
	store := r.store(ctx)

	result, err := store.ExecContext(ctx, `
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

func (r *Repository) SaveJob(ctx context.Context, job *domain.Job) error {
	if job == nil {
		return errors.New("job is nil")
	}
	if job.SourceMessage == nil {
		return errors.New("job source message is nil")
	}
	if job.JobType == "" {
		return errors.New("job type is empty")
	}
	if !isValidJobStatus(job.Status) {
		return fmt.Errorf("save job: unknown status %q", job.Status)
	}

	store := r.store(ctx)
	result, err := store.ExecContext(ctx, `
		INSERT INTO processing_jobs (
			job_type,
			source_message_id,
			status,
			attempts,
			error,
			started_at,
			finished_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, job.JobType,
		job.SourceMessage.ID,
		job.Status,
		job.Attempts,
		stringPointerValue(job.Error),
		timePointerValue(job.StartedAt),
		timePointerValue(job.FinishedAt),
	)
	if err != nil {
		return fmt.Errorf("insert job: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("read inserted job id: %w", err)
	}

	job.ID = id

	return nil
}

func (r *Repository) GetPendingClassificationJob(ctx context.Context) (*domain.Job, error) {
	store := r.store(ctx)

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

	err := store.QueryRowxContext(ctx, `
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

	store := r.store(ctx)

	result, err := store.ExecContext(ctx, `
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

func rawJSONValue(data json.RawMessage) any {
	if len(data) == 0 {
		return nil
	}
	return string(data)
}

func stringPointerValue(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func timePointerValue(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format("2006-01-02 15:04:05")
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
