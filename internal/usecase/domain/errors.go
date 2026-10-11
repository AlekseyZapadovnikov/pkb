package domain

import "errors"

var (
	ErrKnowledgeNotFound = errors.New("knowledge item not found")
	ErrTopicNotFound     = errors.New("topic not found")
)
