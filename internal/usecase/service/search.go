package service

import (
	"context"
	"fmt"

	"pkb/internal/usecase/domain"
)

type KnowledgeRepository interface {
	GetKnowledge(ctx context.Context, id int) (domain.KnowledgeItem, error)
	SearchKnowledge(ctx context.Context, query string, limit int) ([]domain.KnowledgeItem, error)
}

type KnowledgeService struct {
	repo KnowledgeRepository
}

func NewKnowledgeService(repo KnowledgeRepository) *KnowledgeService {
	return &KnowledgeService{repo: repo}
}

func (s *KnowledgeService) GetEntry(ctx context.Context, id int) (domain.KnowledgeItem, error) {
	item, err := s.repo.GetKnowledge(ctx, id)
	if err != nil {
		return domain.KnowledgeItem{}, fmt.Errorf("get knowledge item %d: %w", id, err)
	}
	return item, nil
}

func (s *KnowledgeService) SearchKnowledge(ctx context.Context, query string, limit int) ([]domain.KnowledgeItem, error) {
	items, err := s.repo.SearchKnowledge(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search knowledge: %w", err)
	}
	return items, nil
}
