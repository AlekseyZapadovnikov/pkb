package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"pkb/internal/usecase/domain"
)

// avgChunksPerRecord is the experimentally determined average number of chunks per record.
const avgChunksPerRecord = 10

// Repository stores source messages in SQLite.
type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
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

	topics := make([]*domain.Topic, 0)
	err := store.SelectContext(ctx, &topics, `
		SELECT id, slug, name, description
		FROM topics
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("select topics: %w", err)
	}

	return topics, nil
}

func (r *Repository) DeleteTopic(ctx context.Context, slug string) error {
	store := r.store(ctx)

	result, err := store.ExecContext(ctx, `
		DELETE FROM topics
		WHERE slug = ?
	`, slug)
	if err != nil {
		return fmt.Errorf("delete topic: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted topics count: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("topic not found: %s", slug)
	}

	return nil
}

func (r *Repository) UpdateTopicDescription(ctx context.Context, slug string, description string) error {
	store := r.store(ctx)

	result, err := store.ExecContext(ctx, `
		UPDATE topics
		SET description = ?
		WHERE slug = ?
	`, description, slug)
	if err != nil {
		return fmt.Errorf("update topic description: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read updated topics count: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("topic not found: %s", slug)
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
		INSERT INTO knowledge_items (source_message_id, title, body, confidence, data_json, raw_output_json)
		VALUES (?, ?, ?, ?, ?, ?)
	`, body.SourceMessageID, body.Title, body.Body, body.Confidence, sqliteJSON(body.DataJSON), sqliteJSON(body.RawOutputJSON))
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

	path := fmt.Sprintf("knowledge_%d", id)
	content := fmt.Sprintf("%s\n\n%s", body.Title, body.Body)
	if len(body.Topics) > 0 {
		topicNames := make([]string, 0, len(body.Topics))
		for _, topic := range body.Topics {
			topicNames = append(topicNames, topic.Name)
		}
		content += "\n\nТопики: " + strings.Join(topicNames, ", ")
	}

	if _, err := store.ExecContext(ctx, ` 
        SELECT memory_add_content(?, ?, ?) 
  `, path, content, "knowledge"); err != nil {
		return 0, fmt.Errorf("index knowledge item: %w", err)
	}

	item.ID = id

	return id, nil
}

// ListKnowledgeItems returns knowledge_items with their source messages and topics,
// optionally restricted to IDs below beforeID  and capped at limit.
func (r *Repository) ListKnowledgeItems(ctx context.Context, beforeID int64, limit int) ([]*domain.KnowledgeItem, error) {
	if limit <= 0 {
		return []*domain.KnowledgeItem{}, nil
	}

	store := r.store(ctx)
	query := `
		SELECT
			ki.id,
			ki.source_message_id,
			ki.title,
			ki.body,
			ki.confidence,
			ki.data_json,
			ki.raw_output_json,
			ki.created_at,
			ki.updated_at,
			m.source_type,
			m.raw_text,
			m.created_at AS source_message_created_at
		FROM knowledge_items ki
		JOIN source_messages m ON m.id = ki.source_message_id
	`
	args := make([]any, 0, 2)
	if beforeID > 0 {
		query += " WHERE ki.id < ?"
		args = append(args, beforeID)
	}
	query += " ORDER BY ki.id DESC LIMIT ?"
	args = append(args, limit)

	var rows []knowledgeItemRow
	if err := store.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("select knowledge items: %w", err)
	}

	items := make([]*domain.KnowledgeItem, 0, len(rows))
	itemsByID := make(map[int64]*domain.KnowledgeItem, len(rows))
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		item := row.toDomain()
		items = append(items, item)
		itemsByID[item.ID] = item
		ids = append(ids, item.ID)
	}

	if len(ids) == 0 {
		return items, nil
	}

	topicQuery, topicArgs, err := sqlx.In(`
		SELECT
			kit.knowledge_item_id,
			t.id,
			t.slug,
			t.name,
			t.description
		FROM knowledge_item_topics kit
		JOIN topics t ON t.id = kit.topic_id
		WHERE kit.knowledge_item_id IN (?)
		ORDER BY kit.knowledge_item_id DESC, t.name
	`, ids)
	if err != nil {
		return nil, fmt.Errorf("build knowledge item topics query: %w", err)
	}

	var topicRows []knowledgeItemTopicRow
	if err := store.SelectContext(ctx, &topicRows, topicQuery, topicArgs...); err != nil {
		return nil, fmt.Errorf("select knowledge item topics: %w", err)
	}

	for _, row := range topicRows {
		item := itemsByID[row.KnowledgeItemID]
		if item == nil || item.Body == nil {
			continue
		}
		item.Body.Topics = append(item.Body.Topics, row.toDomain())
	}

	return items, nil
}

// topicID returns the provided topic ID or looks it up in the database by slug if no ID is set.
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
	err := store.GetContext(ctx, &id, `
		SELECT id
		FROM topics
		WHERE slug = ?
	`, topic.Slug)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("topic %q does not exist", topic.Slug)
	}
	if err != nil {
		return 0, fmt.Errorf("select topic id: %w", err)
	}

	return id, nil
}

func (r *Repository) UpdateKnowledge(ctx context.Context, id int64, item *domain.KnowledgeItem) error {
	if item == nil || item.Body == nil {
		return errors.New("knowledge item body is nil")
	}

	return NewTransactor(r.db).WithTx(ctx, func(ctx context.Context) error {
		store := r.store(ctx)
		body := item.Body
		topics := make([]*domain.Topic, 0, len(body.Topics))
		seen := make(map[int64]bool, len(body.Topics))
		for _, selected := range body.Topics {
			if selected == nil {
				return errors.New("topic is nil")
			}
			if seen[selected.ID] {
				continue
			}
			seen[selected.ID] = true
			var topic domain.Topic
			if err := store.GetContext(ctx, &topic, `SELECT id, slug, name, description FROM topics WHERE id = ?`, selected.ID); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return fmt.Errorf("topic %d: %w", selected.ID, domain.ErrTopicNotFound)
				}
				return fmt.Errorf("select knowledge topic: %w", err)
			}
			topics = append(topics, &topic)
		}

		result, err := store.ExecContext(ctx, `
			UPDATE knowledge_items
			SET title = ?, body = ?, data_json = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, body.Title, body.Body, sqliteJSON(body.DataJSON), id)
		if err != nil {
			return fmt.Errorf("update knowledge item: %w", err)
		}
		count, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read updated knowledge count: %w", err)
		}
		if count == 0 {
			return domain.ErrKnowledgeNotFound
		}

		if _, err := store.ExecContext(ctx, `DELETE FROM knowledge_item_topics WHERE knowledge_item_id = ?`, id); err != nil {
			return fmt.Errorf("delete knowledge item topics: %w", err)
		}
		topicNames := make([]string, 0, len(topics))
		for _, topic := range topics {
			if _, err := store.ExecContext(ctx, `INSERT INTO knowledge_item_topics (knowledge_item_id, topic_id) VALUES (?, ?)`, id, topic.ID); err != nil {
				return fmt.Errorf("insert knowledge item topic: %w", err)
			}
			topicNames = append(topicNames, topic.Name)
		}

		content := fmt.Sprintf("%s\n\n%s", body.Title, body.Body)
		if len(topicNames) > 0 {
			content += "\n\nТопики: " + strings.Join(topicNames, ", ")
		}
		if _, err := store.ExecContext(ctx, `SELECT memory_add_content(?, ?, ?)`, fmt.Sprintf("knowledge_%d", id), content, "knowledge"); err != nil {
			return fmt.Errorf("index updated knowledge item: %w", err)
		}
		return nil
	})
}

func (r *Repository) DeleteKnowledge(ctx context.Context, id int64) error {
	return NewTransactor(r.db).WithTx(ctx, func(ctx context.Context) error {
		store := r.store(ctx)
		result, err := store.ExecContext(ctx, `DELETE FROM knowledge_items WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("delete knowledge item: %w", err)
		}
		count, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read deleted knowledge count: %w", err)
		}
		if count == 0 {
			return domain.ErrKnowledgeNotFound
		}
		if _, err := store.ExecContext(ctx, `SELECT memory_delete_file(?)`, fmt.Sprintf("knowledge_%d", id)); err != nil {
			return fmt.Errorf("delete knowledge index: %w", err)
		}
		return nil
	})
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
	if !job.Status.Valid() {
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
		job.Error,
		job.StartedAt,
		job.FinishedAt,
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

	var row jobRow

	err := store.GetContext(ctx, &row, `
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
	`, domain.JobTypeClassifyMessage, domain.JobStatusPending)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("select pending classification job: %w", err)
	}

	return row.toDomain(), nil
}

func (r *Repository) GetJobByID(ctx context.Context, jobID int64) (*domain.Job, error) {
	store := r.store(ctx)

	var row jobRow

	err := store.GetContext(ctx, &row, `
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
		WHERE j.id = ?
		LIMIT 1
	`, jobID)
	if err != nil {
		return nil, fmt.Errorf("select job by id: %w", err)
	}

	return row.toDomain(), nil
}

func (r *Repository) SetJobStatus(ctx context.Context, jobID int64, status domain.JobStatus) error {
	if !status.Valid() {
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

func (r *Repository) GetKnowledge(ctx context.Context, id int) (domain.KnowledgeItem, error) {
	store := r.store(ctx)
	var row knowledgeItemRow

	err := store.GetContext(ctx, &row, `
		SELECT
			ki.id,
			ki.source_message_id,
			ki.title,
			ki.body,
			ki.confidence,
			ki.data_json,
			ki.raw_output_json,
			ki.created_at,
			ki.updated_at,
			m.source_type,
			m.raw_text,
			m.created_at AS source_message_created_at
		FROM knowledge_items ki
		JOIN source_messages m ON m.id = ki.source_message_id
		WHERE ki.id = ?
	`, id)
	if err != nil {
		return domain.KnowledgeItem{}, fmt.Errorf("select knowledge item: %w", err)
	}

	item := row.toDomain()
	err = store.SelectContext(ctx, &item.Body.Topics, `
		SELECT t.id, t.slug, t.name, t.description
		FROM knowledge_item_topics kit
		JOIN topics t ON t.id = kit.topic_id
		WHERE kit.knowledge_item_id = ?
		ORDER BY t.name
	`, id)
	if err != nil {
		return domain.KnowledgeItem{}, fmt.Errorf("select knowledge item topics: %w", err)
	}
	return *item, nil
}

func (r *Repository) SearchKnowledge(ctx context.Context, query string, limit int) ([]domain.KnowledgeItem, error) {
	items := make([]domain.KnowledgeItem, 0, limit)
	store := r.store(ctx)

	var hasItems bool
	if err := store.GetContext(ctx, &hasItems, `SELECT EXISTS (SELECT 1 FROM knowledge_items)`); err != nil {
		return nil, fmt.Errorf("check knowledge items for search: %w", err)
	}
	if !hasItems {
		return items, nil
	}

	var ids []int
	err := store.SelectContext(ctx, &ids, `
		SELECT ki.id
		FROM memory_search ms
		JOIN knowledge_items ki ON ms.path = 'knowledge_' || ki.id
		WHERE ms.query = ? AND ms.max_entries = ? AND ms.context = 'knowledge'
		GROUP BY ki.id
		ORDER BY MAX(ms.ranking) DESC, ki.id DESC
		LIMIT ?
	`, query, limit*avgChunksPerRecord, limit)
	if err != nil {
		return nil, fmt.Errorf("search knowledge index: %w", err)
	}

	for _, id := range ids {
		item, err := r.GetKnowledge(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("load knowledge search result %d: %w", id, err)
		}
		items = append(items, item)
	}
	return items, nil
}
