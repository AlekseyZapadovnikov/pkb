package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"pkb/internal/usecase/domain"
	"time"
)

type JobTopicRepository interface {
	SaveJob(context.Context, *domain.Job) error
	SetJobStatus(context.Context, int64, domain.JobStatus) error
	GetAllTopics(ctx context.Context) ([]*domain.Topic, error)
}

type Classifier struct {
	jr JobTopicRepository

	jobID int64
	jobQ  chan domain.Job
}

func (c *Classifier) AddMessageToQueue(ctx context.Context, msg *domain.SourceMessage) (jobID int64, err error) {
	var job domain.Job

	job.ID = c.jobID
	c.jobID++

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
	dataStr := string(data)

	// как-то объединить системный промпт для классификации и туда подмешать все топики, которые мы вытащили
	// отправить в LLM и получить ответ в формате JSON
	//решать, что делать с этим JSON
	return nil
}
