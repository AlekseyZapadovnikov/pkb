package domain

import (
	"encoding/json"
	"time"
)

type SourceType string

const (
	SourceTypeWebUI    SourceType = "web_ui"
	SourceTypeTelegram SourceType = "telegram"
	SourceTypeAPI      SourceType = "api"
)

type JobType string

const (
	JobTypeClassifyMessage JobType = "classify_message"
)

type JobStatus string

const (
	JobStatusPending JobStatus = "pending"
	JobStatusRunning JobStatus = "running"
	JobStatusDone    JobStatus = "done"
	JobStatusFailed  JobStatus = "failed"
)

type ClassificationKind string

const (
	ClassificationKindKnowledge ClassificationKind = "knowledge"
	ClassificationKindUnknown   ClassificationKind = "unknown"
)

type SourceMessage struct {
	ID int64
	// Внутренний ID исходного сообщения.

	SourceType SourceType
	// Источник сообщения: web_ui, telegram, api.

	RawText string
	// Оригинальный текст пользователя без изменений.

	CreatedAt time.Time
	// Когда сообщение было создано в системе.
}

type ProcessingJob struct {
	ID int64
	// Внутренний ID задачи обработки.

	JobType JobType
	// Тип задачи.

	SourceMessageID *int64
	// ID исходного сообщения, которое нужно обработать.

	Status JobStatus
	// Текущий статус задачи: pending, running, done, failed.

	Attempts int
	// Количество попыток обработки.

	Error *string
	// Текст последней ошибки.
	// nil, если ошибки нет.

	StartedAt *time.Time
	// Когда задача была взята в обработку.
	// nil, если задача ещё не запускалась.

	FinishedAt *time.Time
	// Когда задача завершилась.
	// nil, если задача ещё не завершалась.

	CreatedAt time.Time
	// Когда задача была создана.

	UpdatedAt time.Time
	// Когда задача последний раз обновлялась.
}

type KnowledgeItem struct {
	ID int64
	// Внутренний ID структурированной записи знания.

	SourceMessageID int64
	// ID исходного сообщения, из которого была создана эта запись.

	Category string
	// Пользовательская категория знания.

	Title string
	// Короткий заголовок записи.

	Body string
	// Нормализованное описание.
	// Это уже обработанный текст, не raw input.

	Confidence float64
	// Уверенность классификатора от 0.0 до 1.0.

	DataJSON json.RawMessage
	// Дополнительные структурированные данные в JSON.
	// nil, если дополнительных данных нет.
	//
	// Например:
	// {
	//   "deadline": "2026-05-15",
	//   "tags": ["java", "sockets"],
	//   "project": "university"
	// }

	CreatedAt time.Time
	// Когда запись была создана.

	UpdatedAt time.Time
	// Когда запись последний раз обновлялась.
}

type ClassificationResult struct {
	Kind ClassificationKind
	// Общий результат классификации:
	// knowledge или unknown.

	Category string
	// Пользовательская категория будущего KnowledgeItem.
	//
	// Используется только если Kind == knowledge.
	// Это свободная строка, не enum.

	Title string
	// Заголовок будущего KnowledgeItem.

	Body string
	// Нормализованное описание будущего KnowledgeItem.

	Confidence float64
	// Уверенность классификатора от 0.0 до 1.0.

	DataJSON json.RawMessage
	// Дополнительные данные для KnowledgeItem.
	// nil, если дополнительных данных нет.

	Reason string
	// Причина, если Kind == unknown.

	RawOutputJSON json.RawMessage
	// Полный сырой ответ классификатора, если он есть.
}
