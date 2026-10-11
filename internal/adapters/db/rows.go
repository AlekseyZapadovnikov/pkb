package db

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"pkb/internal/usecase/domain"

	"github.com/mattn/go-sqlite3"
)

type jobRow struct {
	JobID                  int64             `db:"job_id"`
	JobType                domain.JobType    `db:"job_type"`
	JobStatus              domain.JobStatus  `db:"job_status"`
	JobAttempts            int               `db:"attempts"`
	JobError               *string           `db:"error"`
	JobStartedAt           *sqliteTime       `db:"started_at"`
	JobFinishedAt          *sqliteTime       `db:"finished_at"`
	JobCreatedAt           sqliteTime        `db:"job_created_at"`
	JobUpdatedAt           sqliteTime        `db:"job_updated_at"`
	SourceMessageID        int64             `db:"source_message_id"`
	SourceMessageType      domain.SourceType `db:"source_message_type"`
	SourceMessageRawText   string            `db:"raw_text"`
	SourceMessageCreatedAt sqliteTime        `db:"message_created_at"`
}

func (row jobRow) toDomain() *domain.Job {
	return &domain.Job{
		ID:      row.JobID,
		JobType: row.JobType,
		SourceMessage: &domain.SourceMessage{
			ID:         row.SourceMessageID,
			SourceType: row.SourceMessageType,
			RawText:    row.SourceMessageRawText,
			CreatedAt:  row.SourceMessageCreatedAt.Time,
		},
		Status:     row.JobStatus,
		Attempts:   row.JobAttempts,
		Error:      row.JobError,
		StartedAt:  row.JobStartedAt.toTime(),
		FinishedAt: row.JobFinishedAt.toTime(),
		CreatedAt:  row.JobCreatedAt.Time,
		UpdatedAt:  row.JobUpdatedAt.Time,
	}
}

type knowledgeItemRow struct {
	ID                     int64             `db:"id"`
	SourceMessageID        int64             `db:"source_message_id"`
	Title                  string            `db:"title"`
	Body                   string            `db:"body"`
	Confidence             *float64          `db:"confidence"`
	DataJSON               sqliteJSON        `db:"data_json"`
	RawOutputJSON          sqliteJSON        `db:"raw_output_json"`
	CreatedAt              sqliteTime        `db:"created_at"`
	UpdatedAt              sqliteTime        `db:"updated_at"`
	SourceType             domain.SourceType `db:"source_type"`
	RawText                string            `db:"raw_text"`
	SourceMessageCreatedAt sqliteTime        `db:"source_message_created_at"`
}

func (row knowledgeItemRow) toDomain() *domain.KnowledgeItem {
	return &domain.KnowledgeItem{
		ID: row.ID,
		Body: &domain.KnowledgeItemBody{
			SourceMessageID: row.SourceMessageID,
			Title:           row.Title,
			Body:            row.Body,
			Topics:          []*domain.Topic{},
			Confidence:      row.Confidence,
			DataJSON:        json.RawMessage(row.DataJSON),
			RawOutputJSON:   json.RawMessage(row.RawOutputJSON),
		},
		SourceMessage: &domain.SourceMessage{
			ID:         row.SourceMessageID,
			SourceType: row.SourceType,
			RawText:    row.RawText,
			CreatedAt:  row.SourceMessageCreatedAt.Time,
		},
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}

type knowledgeItemTopicRow struct {
	KnowledgeItemID int64  `db:"knowledge_item_id"`
	ID              int64  `db:"id"`
	Slug            string `db:"slug"`
	Name            string `db:"name"`
	Description     string `db:"description"`
}

func (row knowledgeItemTopicRow) toDomain() *domain.Topic {
	return &domain.Topic{
		ID:          row.ID,
		Slug:        row.Slug,
		Name:        row.Name,
		Description: row.Description,
	}
}

// sqliteTime reads timestamps from the existing TEXT columns.
type sqliteTime struct {
	time.Time
}

func (t *sqliteTime) Scan(value any) error {
	var text string
	switch value := value.(type) {
	case time.Time:
		t.Time = value
		return nil
	case string:
		text = value
	case []byte:
		text = string(value)
	default:
		return fmt.Errorf("cannot scan SQLite time from %T", value)
	}

	text = strings.TrimSuffix(text, "Z")
	for _, layout := range sqlite3.SQLiteTimestampFormats {
		parsed, err := time.ParseInLocation(layout, text, time.UTC)
		if err == nil {
			t.Time = parsed
			return nil
		}
	}
	return fmt.Errorf("invalid SQLite time %q", text)
}

func (t *sqliteTime) toTime() *time.Time {
	if t == nil {
		return nil
	}
	return &t.Time
}

// sqliteJSON preserves TEXT storage and maps empty JSON to SQL NULL.
type sqliteJSON []byte

func (j sqliteJSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return string(j), nil
}

func (j *sqliteJSON) Scan(value any) error {
	switch value := value.(type) {
	case nil:
		*j = nil
	case string:
		*j = sqliteJSON(value)
	case []byte:
		*j = append(sqliteJSON{}, value...)
	default:
		return fmt.Errorf("cannot scan SQLite JSON from %T", value)
	}
	return nil
}
