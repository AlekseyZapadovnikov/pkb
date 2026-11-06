package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
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
	GetClassificationJobsByStatus(ctx context.Context, status domain.JobStatus) ([]*domain.Job, error)
	ResetRunningJobs(ctx context.Context, activeIDs []int64) error
	ClaimJob(ctx context.Context, jobID int64) (bool, error)
	ResetFailedJob(ctx context.Context, jobID int64) (bool, error)
	GetJobByID(ctx context.Context, jobID int64) (*domain.Job, error)
	FailJob(ctx context.Context, jobID int64, message string) error
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
	tx Transactor

	mu         sync.Mutex
	activeJobs map[int64]struct{}

	jobQ *domain.JobQueue
}

func NewClassifier(jr JobTopicRepository, tx Transactor, mc ModelClient) *Classifier {
	return &Classifier{
		jr:         jr,
		mc:         mc,
		tx:         tx,
		activeJobs: make(map[int64]struct{}),
		jobQ:       domain.NewJobQueue(),
	}
}

func (c *Classifier) CreateJob(ctx context.Context, msg *domain.SourceMessage) (job domain.Job, err error) {
	var sourceMessageID int64
	var sourceType domain.SourceType
	if msg != nil {
		sourceMessageID = msg.ID
		sourceType = msg.SourceType
	}
	slog.Debug("classification message received", "source_message_id", sourceMessageID, "source_type", sourceType)

	job.JobType = domain.JobTypeClassifyMessage
	job.SourceMessage = msg
	job.Status = domain.JobStatusPending
	job.CreatedAt = time.Now()
	if err := c.jr.SaveJob(ctx, &job); err != nil {
		return domain.Job{}, fmt.Errorf("failed to save job: %w", err)
	}
	return job, nil
}

// RecoverPendingJobs resets inactive running jobs and queues pending classification jobs.
// Recovery is coordinated with workers sharing this Classifier; failed jobs are untouched.
func (c *Classifier) RecoverPendingJobs(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	activeIDs := make([]int64, 0, len(c.activeJobs))
	for id := range c.activeJobs {
		activeIDs = append(activeIDs, id)
	}
	if err := c.jr.ResetRunningJobs(ctx, activeIDs); err != nil {
		return fmt.Errorf("reset interrupted jobs: %w", err)
	}

	pendingJobs, err := c.jr.GetClassificationJobsByStatus(ctx, domain.JobStatusPending)
	if err != nil {
		return err
	}
	for _, j := range pendingJobs {
		c.jobQ.Add(*j)
	}
	return nil
}

// RetryFailedJob resets a failed classification job and queues it after commit.
// Active, missing and non-failed jobs are ignored.
func (c *Classifier) RetryFailedJob(ctx context.Context, id int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, active := c.activeJobs[id]; active {
		return nil
	}
	var job *domain.Job
	err := c.tx.WithTx(ctx, func(ctx context.Context) error {
		reset, err := c.jr.ResetFailedJob(ctx, id)
		if err != nil {
			return err
		}
		if !reset {
			return nil
		}
		job, err = c.jr.GetJobByID(ctx, id)
		return err
	})
	if err != nil {
		return fmt.Errorf("retry failed job: %w", err)
	}
	if job != nil {
		c.jobQ.Add(*job)
	}
	return nil
}

func (c *Classifier) ProcessJobWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			slog.Info("classifier worker stopped")
			return
		default:
		}

		job, ok := c.jobQ.Pop()
		if !ok {
			select {
			case <-ctx.Done():
				slog.Info("classifier worker stopped")
				return
			case <-c.jobQ.Notify:
				// wait for a new job to be added to the queue
				continue
			}
		}

		c.processJob(ctx, job)
	}
}

// processJob claims a job and saves its results and done status atomically.
// Errors are saved as failed; context cancellation leaves the job running for recovery.
func (c *Classifier) processJob(ctx context.Context, job domain.Job) {
	sourceMessageID := int64(0)
	if job.SourceMessage != nil {
		sourceMessageID = job.SourceMessage.ID
	}

	slog.Info("classification job received", "job_id", job.ID, "source_message_id", sourceMessageID, "status", job.Status)

	c.mu.Lock()
	if _, ok := c.activeJobs[job.ID]; ok {
		c.mu.Unlock()
		return
	}
	claimed, err := c.jr.ClaimJob(ctx, job.ID)
	if err == nil && claimed {
		c.activeJobs[job.ID] = struct{}{}
	}
	c.mu.Unlock()
	if err != nil {
		slog.Error("failed to set job status", "id", job.ID, "error", err)
		return
	}
	if !claimed {
		return
	}
	defer func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		defer delete(c.activeJobs, job.ID)
		if err != nil && ctx.Err() == nil {
			if saveErr := c.jr.FailJob(ctx, job.ID, err.Error()); saveErr != nil {
				slog.Error("failed to save job error", "job_id", job.ID, "error", saveErr)
			}
		}
	}()

	slog.Info("classification job started", "job_id", job.ID, "source_message_id", sourceMessageID, "status", domain.JobStatusRunning)

	knowledgeItems, err := c.ProcessClassification(ctx, job)
	if err != nil {
		slog.Error("failed to classify job", "id", job.ID, "error", err)
		return
	}

	err = c.tx.WithTx(ctx, func(ctx context.Context) error {
		for _, item := range knowledgeItems {
			if _, err := c.jr.SaveKnowledgeItem(ctx, item); err != nil {
				return fmt.Errorf("save knowledge item: %w", err)
			}
		}

		if err := c.jr.SetJobStatus(ctx, job.ID, domain.JobStatusDone); err != nil {
			slog.Error("failed to set job status", "id", job.ID, "error", err)
			return err
		}
		return nil
	})

	if err != nil {
		slog.Error("failed to process job", "id", job.ID, "error", err)
	} else {
		slog.Info("classification job finished", "job_id", job.ID, "source_message_id", sourceMessageID, "status", domain.JobStatusDone)
	}

}

type ShortTopic struct {
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

func (c *Classifier) ProcessClassification(ctx context.Context, job domain.Job) ([]*domain.KnowledgeItem, error) {
	sourceMessageID := job.SourceMessage.ID

	slog.Debug("classification started", "job_id", job.ID, "source_message_id", sourceMessageID)

	topics, err := c.jr.GetAllTopics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get topics: %w", err)
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
		return nil, fmt.Errorf("failed to marshal topics: %w", err)
	}

	prompt, err := BuildClassifyPrompt(data, job.SourceMessage.RawText)
	if err != nil {
		return nil, fmt.Errorf("failed to build classify prompt: %w", err)
	}
	slog.Debug("classification request sent to model", "job_id", job.ID, "source_message_id", sourceMessageID, "prompt_bytes", len(prompt))

	ans, err := c.mc.GenerateJSON(ctx, GenerateRequest{
		SystemText: "",
		UserText:   prompt,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to generate JSON: %w", err)
	}
	slog.Debug("classification response received", "job_id", job.ID, "source_message_id", sourceMessageID, "response_bytes", len(ans))

	var res domain.ClassificationResult
	if err := json.Unmarshal(ans, &res); err != nil {
		slog.Error("failed to unmarshal classification result", "job_id", job.ID, "source_message_id", sourceMessageID, "error", err, "response", string(ans))
		return nil, fmt.Errorf("unmarshal classification result: %w", err)
	}

	if res.Result == nil {
		return nil, fmt.Errorf("classification result must contain a Result array")
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
			return nil, fmt.Errorf("classification part %d must contain title and body", i)
		}
		if part.TopicSlugs == nil {
			return nil, fmt.Errorf("classification part %d must contain a TopicSlugs array", i)
		}
		if len(part.TopicSlugs) == 0 {
			if part.Confidence != nil {
				return nil, fmt.Errorf("classification part %d without topics must have null confidence", i)
			}
		} else if part.Confidence == nil || *part.Confidence < 0 || *part.Confidence > 1 {
			return nil, fmt.Errorf("classification part %d with topics must have confidence between 0 and 1", i)
		}

		itemTopics := make([]*domain.Topic, 0, len(part.TopicSlugs))
		seen := make(map[string]bool, len(part.TopicSlugs))
		for _, slug := range part.TopicSlugs {
			topic, ok := topicBySlug[slug]
			if !ok {
				return nil, fmt.Errorf("classification returned unknown topic slug %q", slug)
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
	return items, nil
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
	var job domain.Job

	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		messageID, err := s.repo.SaveMessage(ctx, msg)
		if err != nil {
			return err
		}

		msg.ID = messageID

		job, err = s.class.CreateJob(ctx, msg)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		s.log.Error("failed to process message", "error", err)
		return -1, err
	}

	s.class.jobQ.Add(job)
	slog.Debug("classification job queued", "job_id", job.ID, "source_message_id", msg.ID, "status", job.Status)
	return job.ID, nil
}
