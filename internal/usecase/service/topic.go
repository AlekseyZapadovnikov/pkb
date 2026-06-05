package service

import (
	"context"
	"errors"
	"strings"
	"unicode"

	"pkb/internal/usecase/domain"
)

type TopicRepository interface {
	SaveTopic(context.Context, *domain.Topic) (int64, error)
	DeleteTopic(ctx context.Context, slug string) error
}

type TopicManager struct {
	repo TopicRepository
}

func NewTopicManager(repo TopicRepository) *TopicManager {
	return &TopicManager{
		repo: repo,
	}
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
	return m.repo.DeleteTopic(ctx, slug)
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
