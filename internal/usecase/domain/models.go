package domain

import (
	"encoding/json"
	"time"
)

type SourceType string

const (
	SourceTypeWebUI    SourceType = "web_ui"
	SourceTypeMcp      SourceType = "mcp"
	SourceTypeTelegram SourceType = "telegram"
	SourceTypeAPI      SourceType = "api"
)

type JobType string

const (
	JobTypeClassifyMessage JobType = "classify_message"
)

type JobStatus string

type SourceMessage struct {
	ID int64
	// Internal ID of the source message.

	SourceType SourceType
	// Message source: web_ui, telegram, api.

	RawText string
	// Original user text, preserved unchanged.

	CreatedAt time.Time
	// When the message was created in the system.
}

const (
	JobStatusPending JobStatus = "pending"
	JobStatusRunning JobStatus = "running"
	JobStatusDone    JobStatus = "done"
	JobStatusFailed  JobStatus = "failed"
)

func (status JobStatus) Valid() bool {
	switch status {
	case JobStatusPending, JobStatusRunning, JobStatusDone, JobStatusFailed:
		return true
	default:
		return false
	}
}

type Job struct {
	ID int64
	// Internal ID of the processing job.

	JobType JobType
	// Job type.

	SourceMessage *SourceMessage
	// ID of the source message to process.

	Status JobStatus
	// Current job status: pending, running, done, failed.

	Attempts int
	// Number of processing attempts.

	Error *string
	// Text of the most recent error.
	// nil if there is no error.

	StartedAt *time.Time
	// When processing of the job started.
	// nil if the job has not started yet.

	FinishedAt *time.Time
	// When the job finished.
	// nil if the job has not finished yet.

	CreatedAt time.Time
	// When the job was created.

	UpdatedAt time.Time
	// When the job was last updated.
}

type Topic struct {
	ID int64
	// Internal ID of the topic.

	Slug string
	// Stable machine-readable identifier of the topic.

	Name string
	// Human-readable name of the topic.

	Description string
	// Description of the topic scope for the user and classifier.
}

type KnowledgeItemBody struct {
	SourceMessageID int64
	// ID of the source message from which this item was created.

	Title string
	// Short title of the item.

	Body string
	// Normalized description.
	// Processed text rather than raw input.

	Topics []*Topic
	// Topics associated with the item; an empty list if none match.

	Confidence *float64
	// Topic classification confidence from 0.0 to 1.0; nil if unknown.

	DataJSON json.RawMessage
	// Additional structured data in JSON.
	// nil if there is no additional data.
	//
	// For example:
	// {
	//   "deadline": "2026-05-15",
	//   "tags": ["java", "sockets"],
	//   "project": "university"
	// }

	RawOutputJSON json.RawMessage
	// Full raw response from the classifier.
}

type KnowledgeItem struct {
	ID int64
	// Internal ID of the structured knowledge item.

	Body *KnowledgeItemBody
	// Body of the knowledge item.

	SourceMessage *SourceMessage
	// Source message from which the item was created.

	CreatedAt time.Time
	// When the item was created.

	UpdatedAt time.Time
	// When the item was last updated.
}

type ShortTopic struct {
	Name        string `json:"slug"`
	Description string `json:"description"`
}

// ClassificationResultPart contains the classification result for a single item.
type ClassificationResultPart struct {
	Title string
	// Title of the KnowledgeItem to be created.

	Body string
	// Description of the KnowledgeItem to be created.

	TopicSlugs []string
	// Slugs of the topics selected by the classifier.

	Confidence *float64
	// Topic classification confidence from 0.0 to 1.0; nil if unknown.

	DataJSON json.RawMessage
	// Additional data for the KnowledgeItem.
	// nil if there is no additional data.

}

type ClassificationResult struct {
	SourceMessageID int64
	// ID of the source message from which this item was created.

	Result []ClassificationResultPart
	// Classification result, which may consist of multiple parts.
	// For example, a large message can be split into several logical blocks
	// associated with different topics. The classifier may then return multiple parts,
	// each assigned to its own topic.
}
