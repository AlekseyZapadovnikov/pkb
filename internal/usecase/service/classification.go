package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"text/template"
	"time"

	"pkb/prompts"

	"pkb/internal/usecase/domain"
)

type JobTopicRepository interface {
	SaveJob(context.Context, *domain.Job) error
	SetJobStatus(context.Context, int64, domain.JobStatus) error
	GetAllTopics(ctx context.Context) ([]*domain.Topic, error)
}

type ModelClient interface {
	GenerateJSON(ctx context.Context, req GenerateRequest) (json.RawMessage, error)
}

type GenerateRequest struct {
	SystemText string
	UserText   string
	WantJSON   bool
}

type Classifier struct {
	jr JobTopicRepository
	mc ModelClient

	jobID int64
	jobQ  chan domain.Job
}

func (c *Classifier) AddMessageToQueue(ctx context.Context, msg *domain.SourceMessage) (jobID int64, err error) {
	var job domain.Job

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
	slog.Debug("job added to q", "id", job.ID)
	return job.ID, nil
}

func (c *Classifier) ProcessJobWorker(ctx context.Context) {
	for {
		select {
		case job := <-c.jobQ:
			c.jr.SetJobStatus(ctx, job.ID, domain.JobStatusRunning)
			slog.Debug("processing job", "id", job.ID)
			if err := c.ProcessClassification(ctx, job); err != nil {
				slog.Error("failed to process job", "id", job.ID, "error", err)
				if err := c.jr.SetJobStatus(ctx, job.ID, domain.JobStatusFailed); err != nil {
					slog.Error("failed to set job status", "id", job.ID, "error", err)
				}
			}
		case <-ctx.Done():
			// нужно нормально обрабатывать последнюю джобу
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
	// сходить в БД и достать все топики
	topics, err := c.jr.GetAllTopics(ctx)
	if err != nil {
		return fmt.Errorf("failed to get topics: %w", err)
	}

	shortTopicSlice := make([]ShortTopic, len(topics))
	for i, topic := range topics {
		shortTopicSlice[i] = ShortTopic{
			Slug:        topic.Slug,
			Description: topic.Description,
		}
	}

	data, err := json.MarshalIndent(shortTopicSlice, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal topics: %w", err)
	}

	prompt, err := BuildClassifyPrompt(data, job.SourceMessage.RawText)
	if err != nil {
		return fmt.Errorf("failed to build classify prompt: %w", err)
	}
	_ = prompt

	// отправить в LLM и получить ответ в формате JSON
	//решать, что делать с этим JSON
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
