package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"text/template"
	"time"

	"pkb/internal/usecase/domain"
	"pkb/prompts"
)

type JobTopicRepository interface {
	SaveJob(context.Context, *domain.Job) error
	SetJobStatus(context.Context, int64, domain.JobStatus) error
	GetAllTopics(ctx context.Context) ([]*domain.Topic, error)
	SaveKnowledgeItem(context.Context, *domain.KnowledgeItem) (int64, error)
}

type ModelClient interface {
	GenerateJSON(ctx context.Context, req GenerateRequest) (json.RawMessage, error)
}

type GenerateRequest struct {
	SystemText string
	UserText   string
}

type Classifier struct {
	jr JobTopicRepository
	mc ModelClient

	jobID int64
	jobQ  chan domain.Job
}

func NewClassifier(jr JobTopicRepository, mc ModelClient) *Classifier {
	return &Classifier{
		jr:   jr,
		mc:   mc,
		jobQ: make(chan domain.Job, 10),
	}
}

func (c *Classifier) AddMessageToQueue(ctx context.Context, msg *domain.SourceMessage) (jobID int64, err error) {
	var job domain.Job

	var sourceMessageID int64
	var sourceType domain.SourceType
	if msg != nil {
		sourceMessageID = msg.ID
		sourceType = msg.SourceType
	}
	slog.Debug("classification message received", "source_message_id", sourceMessageID, "source_type", sourceType)

	job.ID = c.jobID
	c.jobID++

	job.JobType = domain.JobTypeClassifyMessage
	job.SourceMessage = msg
	job.Status = domain.JobStatusPending
	job.CreatedAt = time.Now()
	if err := c.jr.SaveJob(ctx, &job); err != nil {
		return 0, fmt.Errorf("failed to save job: %w", err)
	}

	c.jobQ <- job
	slog.Debug("classification job queued", "job_id", job.ID, "source_message_id", sourceMessageID, "status", job.Status)
	return job.ID, nil
}

func (c *Classifier) ProcessJobWorker(ctx context.Context) {
	for {
		select {
		case job := <-c.jobQ:
			sourceMessageID := int64(0)
			if job.SourceMessage != nil {
				sourceMessageID = job.SourceMessage.ID
			}
			slog.Info("classification job received", "job_id", job.ID, "source_message_id", sourceMessageID, "status", job.Status)

			if err := c.jr.SetJobStatus(ctx, job.ID, domain.JobStatusRunning); err != nil {
				slog.Error("failed to set job status", "id", job.ID, "error", err)
			} else {
				slog.Info("classification job started", "job_id", job.ID, "source_message_id", sourceMessageID, "status", domain.JobStatusRunning)
			}

			if err := c.ProcessClassification(ctx, job); err != nil {
				slog.Error("failed to process job", "id", job.ID, "error", err)
				if err := c.jr.SetJobStatus(ctx, job.ID, domain.JobStatusFailed); err != nil {
					slog.Error("failed to set job status", "id", job.ID, "error", err)
				} else {
					slog.Info("classification job failed", "job_id", job.ID, "source_message_id", sourceMessageID, "status", domain.JobStatusFailed)
				}
				continue
			}

			if err := c.jr.SetJobStatus(ctx, job.ID, domain.JobStatusDone); err != nil {
				slog.Error("failed to set job status", "id", job.ID, "error", err)
				continue
			}
			slog.Info("classification job finished", "job_id", job.ID, "source_message_id", sourceMessageID, "status", domain.JobStatusDone)
		case <-ctx.Done():
			// The last job needs to be handled properly.
			slog.Info("classifier worker stopped")
			return
		}
	}
}

type ShortTopic struct {
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

func (c *Classifier) ProcessClassification(ctx context.Context, job domain.Job) error {
	sourceMessageID := job.SourceMessage.ID

	slog.Debug("classification started", "job_id", job.ID, "source_message_id", sourceMessageID)

	topics, err := c.jr.GetAllTopics(ctx)
	if err != nil {
		return fmt.Errorf("failed to get topics: %w", err)
	}
	slog.Debug("classification topics loaded", "job_id", job.ID, "source_message_id", sourceMessageID, "topics_count", len(topics))

	shortTopicSlice := make([]ShortTopic, len(topics))
	for i, topic := range topics {
		shortTopicSlice[i] = ShortTopic{
			Slug:        topic.Slug,
			Description: topic.Description,
		}
	}

	data, err := json.MarshalIndent(shortTopicSlice, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal topics: %w", err)
	}

	prompt, err := BuildClassifyPrompt(data, job.SourceMessage.RawText)
	if err != nil {
		return fmt.Errorf("failed to build classify prompt: %w", err)
	}
	slog.Debug("classification request sent to model", "job_id", job.ID, "source_message_id", sourceMessageID, "prompt_bytes", len(prompt))

	ans, err := c.mc.GenerateJSON(ctx, GenerateRequest{
		SystemText: "",
		UserText:   prompt,
	})

	if err != nil {
		return fmt.Errorf("failed to generate JSON: %w", err)
	}
	slog.Debug("classification response received", "job_id", job.ID, "source_message_id", sourceMessageID, "response_bytes", len(ans))

	var res domain.ClassificationResult
	if err := json.Unmarshal(ans, &res); err != nil {
		slog.Error("failed to unmarshal classification result", "job_id", job.ID, "source_message_id", sourceMessageID, "error", err, "response", string(ans))
		return fmt.Errorf("unmarshal classification result: %w", err)
	}

	if res.Result == nil {
		return fmt.Errorf("classification result must contain a Result array")
	}

	res.SourceMessageID = sourceMessageID
	slog.Debug("classification response parsed", "job_id", job.ID, "source_message_id", sourceMessageID, "parts_count", len(res.Result))

	topicBySlug := make(map[string]*domain.Topic, len(topics))
	for _, topic := range topics {
		topicBySlug[topic.Slug] = topic
	}

	items := make([]*domain.KnowledgeItem, 0, len(res.Result))
	for i, part := range res.Result {
		if strings.TrimSpace(part.Title) == "" || strings.TrimSpace(part.Body) == "" {
			return fmt.Errorf("classification part %d must contain title and body", i)
		}
		if part.TopicSlugs == nil {
			return fmt.Errorf("classification part %d must contain a TopicSlugs array", i)
		}
		if len(part.TopicSlugs) == 0 {
			if part.Confidence != nil {
				return fmt.Errorf("classification part %d without topics must have null confidence", i)
			}
		} else if part.Confidence == nil || *part.Confidence < 0 || *part.Confidence > 1 {
			return fmt.Errorf("classification part %d with topics must have confidence between 0 and 1", i)
		}

		itemTopics := make([]*domain.Topic, 0, len(part.TopicSlugs))
		seen := make(map[string]bool, len(part.TopicSlugs))
		for _, slug := range part.TopicSlugs {
			topic, ok := topicBySlug[slug]
			if !ok {
				return fmt.Errorf("classification returned unknown topic slug %q", slug)
			}
			if seen[slug] {
				continue
			}
			seen[slug] = true
			itemTopics = append(itemTopics, topic)
		}

		items = append(items, &domain.KnowledgeItem{
			Body: &domain.KnowledgeItemBody{
				SourceMessageID: res.SourceMessageID,
				Title:           part.Title,
				Body:            part.Body,
				Topics:          itemTopics,
				Confidence:      part.Confidence,
				DataJSON:        part.DataJSON,
				RawOutputJSON:   ans,
			},
		})
	}

	for _, item := range items {
		if _, err := c.jr.SaveKnowledgeItem(ctx, item); err != nil {
			return fmt.Errorf("save knowledge item: %w", err)
		}
	}

	return nil
}

func BuildClassifyPrompt(topicsJSON []byte, inputText string) (string, error) {
	tmpl, err := template.New("classify").
		Option("missingkey=error").
		Parse(prompts.Classify)
	if err != nil {
		return "", fmt.Errorf("parse classify prompt template: %w", err)
	}

	data := map[string]any{
		"TopicsJSON": string(topicsJSON),
		"InputText":  inputText,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute classify prompt template: %w", err)
	}

	return buf.String(), nil
}

type Repository interface {
	SaveMessage(context.Context, *domain.SourceMessage) (messageID int64, err error)
}

type Transactor interface {
	WithTx(context.Context, func(context.Context) error) error
}

// Messager is a service that handling eatch type of messages
type Messager struct {
	log   *slog.Logger
	repo  Repository
	tx    Transactor
	class *Classifier
}

func NewMessanger(log *slog.Logger, repo Repository, tx Transactor, class *Classifier) *Messager {
	return &Messager{
		log:   log,
		repo:  repo,
		tx:    tx,
		class: class,
	}
}

// ProcessMessage processes a source message and saves it to the repository
func (s *Messager) ProcessMessage(ctx context.Context, msg *domain.SourceMessage) (int64, error) {
	var jobID int64

	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		messageID, err := s.repo.SaveMessage(ctx, msg)
		if err != nil {
			return err
		}

		msg.ID = messageID

		jobID, err = s.class.AddMessageToQueue(ctx, msg)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		s.log.Error("failed to process message", "error", err)
		return -1, err
	}

	return jobID, nil
}
