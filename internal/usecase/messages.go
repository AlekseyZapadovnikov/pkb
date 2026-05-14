package usecase

import "pkb/internal/usecase/domain"

// SubmitMessageInput описывает входные данные сценария сохранения исходного сообщения.
type SubmitMessageInput struct {
	SourceType domain.SourceType
	RawText    string
}
