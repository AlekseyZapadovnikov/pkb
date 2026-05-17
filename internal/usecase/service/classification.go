package service

import (
	"context"
	"fmt"
	"log/slog"
	"pkb/internal/usecase/domain"
	"time"
)

type JobRepository interface {
	SaveJob(context.Context, *domain.Job) error
	SetJobStatus(context.Context, int64, domain.JobStatus) error
}

type Classifier struct {
	jr JobRepository

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
			if err := ProcessClassification(ctx, job); err != nil {
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

func ProcessClassification(ctx context.Context, job domain.Job) error {
	// логика процессинга классификации
	return nil
}
