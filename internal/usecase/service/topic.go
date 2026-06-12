package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"pkb/internal/usecase/domain"
)

type TopicRepository interface {
	SaveTopic(context.Context, *domain.Topic) (int64, error)
	GetAllTopics(context.Context) ([]*domain.Topic, error)
	UpdateTopicDescription(ctx context.Context, slug string, description string) error
	DeleteTopic(ctx context.Context, slug string) error
}

type TopicManager struct {
	repo TopicRepository
}

func NewTopicManager(repo TopicRepository) (*TopicManager, error) {

	var tm = &TopicManager{
		repo: repo,
	}

	_, err := tm.CreateTopic(context.TODO(), &domain.Topic{
		Name:        "unknown",
		Description: "Это системный топик сюда нужно складывать сообщения, которые не подходят ни под один из существующих топиков.",
	})

	if err != nil {
		return nil, fmt.Errorf("create default topic: %w", err)
	}

	return tm, nil
}

// CreateTopic creates a new topic and saves it to the repository
func (m *TopicManager) CreateTopic(ctx context.Context, topic *domain.Topic) (int64, error) {
	topic.Slug = topicSlug(topic.Name)
	if topic.Slug == "" {
		return 0, errors.New("topic slug is empty")
	}

	return m.repo.SaveTopic(ctx, topic)
}

func (m *TopicManager) DeleteTopic(ctx context.Context, name string) error {

	slug := topicSlug(name)
	if slug == "" {
		return errors.New("topic slug is empty")
	}
	if slug == topicSlug("unknown") {
		return errors.New("cannot delete topic 'unknown'")
	}

	return m.repo.DeleteTopic(ctx, slug)
}

func (m *TopicManager) UpdateTopicDescription(ctx context.Context, name string, description string) error {
	slug := topicSlug(name)
	if slug == "" {
		return errors.New("topic slug is empty")
	}

	return m.repo.UpdateTopicDescription(ctx, slug, description)
}

func (m *TopicManager) GetTopics(ctx context.Context) ([]*domain.Topic, error) {
	return m.repo.GetAllTopics(ctx)
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
