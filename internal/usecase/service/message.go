package service

import (
	"context"
	"log/slog"
	"pkb/internal/usecase/domain"
)

type Repository interface {
	SaveMessage(context.Context, *domain.SourceMessage) (messageID int64, err error)
}

type Transactor interface {
	WithTx(context.Context, func(context.Context) error) error
}

// Messager is a service that handling eatch type of messages
type Messager struct {
	log        *slog.Logger
	repo       Repository
	tx         Transactor
	classifier Classifier
}

func NewMessanger(log *slog.Logger, repo Repository, tx Transactor, classifier Classifier) *Messager {
	return &Messager{
		log:        log,
		repo:       repo,
		tx:         tx,
		classifier: classifier,
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

	// AddMessageToQueue added message and create job returning jobID? we need jobID to track status of processing
	jobID, err = s.classifier.AddMessageToQueue(ctx, msg)
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
