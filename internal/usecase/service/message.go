package service

import (
	"context"
	"log/slog"
	"pkb/internal/usecase/domain"
)

type Repository interface {
	SaveMessage(context.Context, *domain.SourceMessage) (messageID int64, err error)
}

type Classifier interface {
	AddMessageToQueue(context.Context, *domain.SourceMessage) (jobID int64, err error)
}

// Messager is a service that handling eatch type of messages
type Messager struct {
	log        *slog.Logger
	repo       Repository
	classifier Classifier
}

// ProcessMessage processes a source message and saves it to the repository
func (s *Messager) ProcessMessage(ctx context.Context, msg *domain.SourceMessage) (int64, error) {

	// HERE WE NEED TRANSACTION START
	messageID, err := s.repo.SaveMessage(ctx, msg)
	if err != nil {
		s.log.Error("failed to save message", "error", err)
		return -1, err
	}
	msg.ID = messageID

	// AddMessageToQueue added message and create job returning jobID? we need jobID to track status of processing
	jobID, err := s.classifier.AddMessageToQueue(ctx, msg)
	if err != nil {
		s.log.Error("failed to add message to classifier queue", "jobID", jobID, "error", err)
		return -1, err
	}
	// HERE WE NEED TRANSACTION COMMIT

	return jobID, nil
}
