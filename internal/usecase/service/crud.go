package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode"

	"pkb/internal/usecase/domain"
)

type EntityRepository interface {
	SaveTopic(context.Context, *domain.Topic) (int64, error)
	GetAllTopics(context.Context) ([]*domain.Topic, error)
	UpdateTopicDescription(ctx context.Context, slug string, description string) error
	DeleteTopic(ctx context.Context, slug string) error
	DeleteKnowledge(ctx context.Context, id int64) error
	UpdateKnowledge(ctx context.Context, id int64, item *domain.KnowledgeItem) error
}

type EntityManager struct {
	repo EntityRepository
}

func NewEntityManager(repo EntityRepository) *EntityManager {
	return &EntityManager{repo: repo}
}

// CreateTopic creates a new topic and saves it to the repository
func (m *EntityManager) CreateTopic(ctx context.Context, topic *domain.Topic) (int64, error) {
	topic.Slug = topicSlug(topic.Name)
	if topic.Slug == "" {
		slog.Error("create topic failed: slug is empty", "name", topic.Name)
		return 0, errors.New("topic slug is empty")
	}

	id, err := m.repo.SaveTopic(ctx, topic)
	if err != nil {
		slog.Error("save topic failed", "slug", topic.Slug, "name", topic.Name, "error", err)
		return 0, fmt.Errorf("save topic err: %w", err)
	}

	slog.Info("topic created", "id", id, "slug", topic.Slug, "name", topic.Name)
	return id, nil
}

func (m *EntityManager) DeleteTopic(ctx context.Context, name string) error {

	slug := topicSlug(name)
	if slug == "" {
		slog.Error("delete topic failed: slug is empty", "name", name)
		return errors.New("topic slug is empty")
	}

	if err := m.repo.DeleteTopic(ctx, slug); err != nil {
		slog.Error("delete topic failed", "slug", slug, "error", err)
		return err
	}

	slog.Info("topic deleted", "slug", slug)
	return nil
}

func (m *EntityManager) UpdateTopicDescription(ctx context.Context, name string, description string) error {
	slug := topicSlug(name)
	if slug == "" {
		slog.Error("update topic description failed: slug is empty", "name", name)
		return errors.New("topic slug is empty")
	}

	if err := m.repo.UpdateTopicDescription(ctx, slug, description); err != nil {
		slog.Error("update topic description failed", "slug", slug, "error", err)
		return err
	}

	slog.Info("topic description updated", "slug", slug)
	return nil
}

func (m *EntityManager) GetTopics(ctx context.Context) ([]*domain.Topic, error) {
	topics, err := m.repo.GetAllTopics(ctx)
	if err != nil {
		slog.Error("get topics failed", "error", err)
		return nil, err
	}

	slog.Debug("topics loaded", "count", len(topics))
	return topics, nil
}

func topicSlug(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))

	var b strings.Builder
	lastDash := false

	for _, r := range name {
		if unicode.IsSpace(r) || r == '_' {
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
			continue
		}

		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}

		if r == '-' && !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func (m *EntityManager) DeleteKnowledge(ctx context.Context, id int64) error {
	if err := m.repo.DeleteKnowledge(ctx, id); err != nil {
		return fmt.Errorf("delete knowledge item %d: %w", id, err)
	}

	return nil
}

func (m *EntityManager) UpdateKnowledge(ctx context.Context, id int64, item *domain.KnowledgeItem) error {
	if err := m.repo.UpdateKnowledge(ctx, id, item); err != nil {
		return fmt.Errorf("update knowledge item %d: %w", id, err)
	}

	return nil
}
