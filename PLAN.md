# План MVP для классификации по топикам

## Архитектурное решение

1. SQLite — источник истины.
2. Markdown на этом этапе не участвует в хранении данных.
   - Prompt-файлы лежат отдельно.
   - Markdown export можно добавить позже как производное представление.
3. AI не пишет напрямую ни в SQLite, ни в Markdown.
4. AI возвращает только строгий JSON.
5. MCP, agents framework, embeddings и vector search сейчас не добавляем.
6. `Category` убираем.
7. `Topic` добавляем как отдельную доменную сущность.
8. `KnowledgeItem` связывается с `Topic` через many-to-many.
9. `unknown_items` в этом MVP не используем.
10. `unknown` реализуем как обычный технический topic со slug `unknown`.

Важное правило: topic `unknown` не должен участвовать в обычных ответах базы знаний и обычном поиске по умолчанию. Это техническая зона для сообщений, которые нужно потом разобрать вручную или повторно классифицировать.

Обычные search/answer запросы должны исключать:

```sql
topic.slug = 'unknown'
```

Если пользователь явно хочет посмотреть неразобранные сообщения, тогда запрашиваем `unknown` напрямую.

## Изменения доменной модели

В `internal/usecase/domain/models.go`:

- добавить `Topic`;
- убрать `Category` из `KnowledgeItem`;
- добавить `Topics []*Topic` в `KnowledgeItem`;
- убрать `Category` из `ClassificationResult`;
- добавить `TopicSlugs []string` в `ClassificationResult`;
- оставить `Kind` со значениями `knowledge` и `unknown`.

Минимальная модель topic:

```go
type Topic struct {
	ID          int64
	Slug        string
	Name        string
	Description string
}
```

`KnowledgeItem` должен хранить нормализованный контент и уже найденные топики:

```go
type KnowledgeItem struct {
	ID              int64
	SourceMessageID int64

	Title string
	Body  string

	Topics []*Topic

	Confidence float64
	DataJSON   json.RawMessage

	CreatedAt time.Time
	UpdatedAt time.Time
}
```

`ClassificationResult` должен оставаться черновиком от AI, а не полноценной сохранённой сущностью:

```go
type ClassificationResult struct {
	Kind ClassificationKind

	Title string
	Body  string

	TopicSlugs []string

	Confidence float64
	DataJSON   json.RawMessage

	Reason        string
	RawOutputJSON json.RawMessage
}
```

## Изменения SQLite

Если текущая база содержит только одноразовые dev-данные, можно обновить `00001_init.sql`.

Если существующие данные нужно сохранить, нужно добавить новую миграцию `00002`.

Изменения схемы:

- убрать `category` из `knowledge_items`;
- добавить `description` в `topics`;
- оставить `slug`;
- оставить `knowledge_item_topics`;
- убрать `unknown_items` из MVP-схемы или оставить неиспользуемой, если миграционный шум пока не нужен;
- оставить уникальность для `topics.slug`;
- добавить индексы для `topics.slug` и `knowledge_item_topics.topic_id`.

## Дефолтный topic unknown

При старте приложения нужно гарантировать, что технический topic существует:

```text
slug = "unknown"
name = "unknown"
description = "Messages that could not be confidently assigned to existing topics."
```

Это нужно делать в startup-коде, а не только в миграции, чтобы приложение могло восстановить обязательный технический topic при необходимости.

## Изменения репозиториев

Добавить минимальное поведение для topic repository:

```go
type TopicRepository interface {
	EnsureUnknownTopic(ctx context.Context) (*domain.Topic, error)
	ListTopics(ctx context.Context) ([]*domain.Topic, error)
	FindTopicsBySlugs(ctx context.Context, slugs []string) ([]*domain.Topic, error)
}
```

Добавить минимальное поведение для knowledge repository:

```go
type KnowledgeRepository interface {
	SaveKnowledgeItem(ctx context.Context, item *domain.KnowledgeItem) error
	SaveKnowledgeItemTopics(ctx context.Context, itemID int64, topics []*domain.Topic) error
}
```

Позже эти методы можно объединить в один транзакционный save-метод, если так лучше ляжет на реализацию.

## Интерфейс классификатора

Текущий service type с именем `Classifier` уже отвечает за очередь и worker. LLM/mock classifier лучше назвать иначе, например:

```go
type MessageClassifier interface {
	Classify(ctx context.Context, msg *domain.SourceMessage, topics []*domain.Topic) (*domain.ClassificationResult, error)
}
```

## Worker flow

Поток обработки:

```text
SourceMessage
-> Job
-> загрузить topics кроме unknown
-> MessageClassifier
-> ClassificationResult
-> валидация
-> resolve topics
-> сохранить KnowledgeItem
-> сохранить knowledge_item_topics
-> пометить Job как done
```

Правила:

- если `kind == knowledge`, confidence достаточно высокий и все slugs существуют: сохраняем с найденными topics;
- если `kind == unknown`: сохраняем с topic `unknown`;
- если confidence ниже threshold: сохраняем с topic `unknown`;
- если AI вернула неизвестные topic slugs: для MVP заменяем весь набор topics на `unknown`;
- AI никогда не создаёт topics напрямую.

## Search и answer behavior

Обычный поиск по базе знаний и обычные answer-запросы должны исключать технический topic:

```sql
WHERE topics.slug != 'unknown'
```

Режим ручного разбора unknown должен явно включать только:

```sql
WHERE topics.slug = 'unknown'
```

## Проверка после реализации

Запустить:

```sh
gofmt -w <changed-go-files>
go test ./...
go vet ./...
```

Ручные сценарии для проверки:

- классификация в известный topic;
- `kind == unknown`;
- классификация с низким confidence;
- AI вернула неизвестный topic slug;
- обычный поиск исключает `unknown`;
- явный режим просмотра unknown включает `unknown`.

## Риски

Главный риск — стратегия миграций.

Если локальная SQLite база содержит только одноразовые dev-данные, проще обновить `00001_init.sql`.

Если данные нужно сохранить, нужно добавить `00002` и мигрировать аккуратно.
