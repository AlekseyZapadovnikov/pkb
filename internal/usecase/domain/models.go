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

const (
	JobStatusPending JobStatus = "pending"
	JobStatusRunning JobStatus = "running"
	JobStatusDone    JobStatus = "done"
	JobStatusFailed  JobStatus = "failed"
)

type Job struct {
	ID int64
	// Внутренний ID задачи обработки.

	JobType JobType
	// Тип задачи.

	SourceMessage *SourceMessage
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

type Topic struct {
	ID int64
	// Внутренний ID топика.

	Slug string
	// Стабильный машинный идентификатор топика.

	Name string
	// Человекочитаемое название топика.

	Description string
	// Описание границ топика для пользователя и классификатора.
}

type KnowledgeItemBody struct {
	SourceMessageID int64
	// ID исходного сообщения, из которого была создана эта запись.

	Title string
	// Короткий заголовок записи.

	Body string
	// Нормализованное описание.
	// Это уже обработанный текст, не raw input.

	Topics []*Topic
	// Топики, к которым относится запись.

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
}

type KnowledgeItem struct {
	ID int64
	// Внутренний ID структурированной записи знания.

	Body *KnowledgeItemBody
	// Тело записи знания.

	CreatedAt time.Time
	// Когда запись была создана.

	UpdatedAt time.Time
	// Когда запись последний раз обновлялась.
}

type ShortTopic struct {
	Name        string `json:"slug"`
	Description string `json:"description"`
}

type UnknownKnowledgeItemBody struct {
	Reason string
	// Причина, по которой классификатор не смог отнести сообщение ни к одному топику.

	RawOutputJSON json.RawMessage
	// Полный сырой ответ классификатора, если он есть.
	// Может помочь в будущем для анализа ошибок классификации.

	SugestTopics []ShortTopic
	// Моделька предложет несколько названий для топиков их описание, а пользователь сможет выбрать 1 из них

}
type UnknownKnowledgeItem struct {
	ID int64
	// Внутренний ID записи с неизвестным знанием.

	SourceMessageID int64
	// ID исходного сообщения, из которого была создана эта запись.

	CreatedAt time.Time
	// Когда запись была создана.

	UpdatedAt time.Time
	// Когда запись последний раз обновлялась.
}

// тут важно, что это результат классификации именно одной записи.
type ClassificationResultPart struct {
	Kind ClassificationKind
	// Общий результат классификации:
	// knowledge или unknown.

	Title string
	// Заголовок будущего KnowledgeItem.

	Body string
	// Описание будущего KnowledgeItem.

	TopicSlugs []string
	// Slug-и топиков, выбранных классификатором.

	Confidence float64
	// Уверенность классификатора от 0.0 до 1.0.

	DataJSON json.RawMessage
	// Дополнительные данные для KnowledgeItem.
	// nil, если дополнительных данных нет.

	Reason string
	// Причина, если Kind == unknown

	RawOutputJSON json.RawMessage
	// Полный сырой ответ классификатора, если он есть.
}

type ClassificationResult struct {
	SourceMessageID int64
	// ID исходного сообщения, из которого была создана эта запись.

	Result []ClassificationResultPart
	// Результат классификации, который может состоять из нескольких частей.
	// Например, из одного большого сообщения можно выделить несколько логических блоков,
	// которые относятся к разным топикам. В этом случае классификатор может вернуть несколько частей,
	// каждая из которых будет отнесена к своему топику.
}
