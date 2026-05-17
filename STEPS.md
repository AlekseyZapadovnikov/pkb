# Пошаговый план реализации MVP классификации по топикам

---

## 2. Обновить доменную модель

Файл:

```text
internal/usecase/domain/models.go
```

### 2.1. Добавить `Topic`

```go
type Topic struct {
	ID          int64
	Slug        string
	Name        string
	Description string
}
```

Если в проекте уже везде используются `CreatedAt` и `UpdatedAt`, можно сразу добавить их:

```go
CreatedAt time.Time
UpdatedAt time.Time
```

Но для минимального MVP они не обязательны.

### 2.2. Изменить `KnowledgeItem`

Убрать:

```go
Category string
```

Добавить:

```go
Topics []*Topic
```

Итоговая модель:

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

### 2.3. Изменить `ClassificationResult`

Убрать:

```go
Category string
```

Добавить:

```go
TopicSlugs []string
```

Итоговая модель:

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

### 2.4. Оставить `ClassificationKind`

```go
type ClassificationKind string

const (
	ClassificationKindKnowledge ClassificationKind = "knowledge"
	ClassificationKindUnknown   ClassificationKind = "unknown"
)
```

### Результат этапа

Код доменной модели должен отражать новую схему:

```text
SourceMessage -> ClassificationResult -> KnowledgeItem -> Topics
```

`Category` больше не должен использоваться.

---

## 3. Обновить SQLite-схему

### 3.1. Таблица `topics`

Проверить, что есть таблица:

```sql
CREATE TABLE topics (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	slug TEXT NOT NULL UNIQUE,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT ''
);
```

### 3.2. Таблица `knowledge_items`

Убрать поле:

```sql
category
```

Минимальная схема:

```sql
CREATE TABLE knowledge_items (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	source_message_id INTEGER NOT NULL,
	title TEXT NOT NULL,
	body TEXT NOT NULL,
	confidence REAL NOT NULL,
	data_json TEXT,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,

	FOREIGN KEY (source_message_id) REFERENCES source_messages(id)
);
```

### 3.3. Таблица many-to-many

Добавить или оставить таблицу:

```sql
CREATE TABLE knowledge_item_topics (
	knowledge_item_id INTEGER NOT NULL,
	topic_id INTEGER NOT NULL,

	PRIMARY KEY (knowledge_item_id, topic_id),

	FOREIGN KEY (knowledge_item_id) REFERENCES knowledge_items(id),
	FOREIGN KEY (topic_id) REFERENCES topics(id)
);
```

### 3.4. Индексы

Добавить индексы:

```sql
CREATE INDEX idx_topics_slug ON topics(slug);
CREATE INDEX idx_knowledge_item_topics_topic_id ON knowledge_item_topics(topic_id);
CREATE INDEX idx_knowledge_item_topics_item_id ON knowledge_item_topics(knowledge_item_id);
```

### 3.5. `unknown_items`

Для MVP не использовать.

Если таблица уже есть и удалять её неудобно, можно временно оставить её в схеме, но код не должен на неё опираться.

### Результат этапа

В SQLite должна быть структура:

```text
topics
knowledge_items
knowledge_item_topics
```

А поля `category` в `knowledge_items` быть не должно.

---

## 4. Гарантировать наличие topic `unknown`

### 4.1. Добавить startup-логику

При старте приложения нужно гарантировать наличие технического topic:

```text
slug = "unknown"
name = "unknown"
description = "Messages that could not be confidently assigned to existing topics."
```

Это лучше делать не только миграцией, а именно кодом при старте приложения.

### 4.2. Поведение

Если topic уже есть — ничего не делать.

Если topic отсутствует — создать.

### Результат этапа

После запуска приложения в базе всегда есть topic:

```text
unknown
```

---

## 5. Добавить `TopicRepository`

### 5.1. Интерфейс

Добавить минимальный интерфейс:

```go
type TopicRepository interface {
	EnsureUnknownTopic(ctx context.Context) (*domain.Topic, error)
	ListTopics(ctx context.Context) ([]*domain.Topic, error)
	FindTopicsBySlugs(ctx context.Context, slugs []string) ([]*domain.Topic, error)
}
```

### 5.2. `EnsureUnknownTopic`

Должен:

1. искать topic по slug `unknown`;
2. если найден — вернуть его;
3. если не найден — создать;
4. вернуть созданный topic.

### 5.3. `ListTopics`

Должен возвращать список topics.

Для классификации лучше использовать список без `unknown`, чтобы AI выбирала только нормальные topics.

Можно сделать отдельный метод позже:

```go
ListClassifiableTopics(ctx context.Context) ([]*domain.Topic, error)
```

Но для MVP можно фильтровать `unknown` в usecase/worker.

### 5.4. `FindTopicsBySlugs`

Должен принимать slugs, которые вернул AI, и возвращать найденные topics.

### Результат этапа

Worker сможет:

```text
- загрузить доступные topics
- найти topics по AI topic_slugs
- получить technical unknown topic
```

---

## 6. Обновить `KnowledgeRepository`

### 6.1. Интерфейс

Минимальный вариант:

```go
type KnowledgeRepository interface {
	SaveKnowledgeItem(ctx context.Context, item *domain.KnowledgeItem) error
	SaveKnowledgeItemTopics(ctx context.Context, itemID int64, topics []*domain.Topic) error
}
```

### 6.2. Лучше сделать транзакционный метод

Если удобно, лучше сразу сделать один метод:

```go
SaveKnowledgeItemWithTopics(ctx context.Context, item *domain.KnowledgeItem) error
```

Он должен внутри одной транзакции:

1. вставить `knowledge_items`;
2. получить `item.ID`;
3. вставить связи в `knowledge_item_topics`.

### 6.3. Правило атомарности

Нельзя допустить ситуацию:

```text
KnowledgeItem сохранился,
а связи с topics не сохранились.
```

Поэтому сохранение item и связей лучше выполнять в одной транзакции.

### Результат этапа

Можно сохранить полноценную запись знания вместе с topics.

---

## 7. Переименовать LLM-классификатор

Если в проекте уже есть service с названием `Classifier`, который отвечает за очередь/worker, не использовать то же имя для LLM-классификатора.

### 7.1. Добавить интерфейс

```go
type MessageClassifier interface {
	Classify(
		ctx context.Context,
		msg *domain.SourceMessage,
		topics []*domain.Topic,
	) (*domain.ClassificationResult, error)
}
```

### 7.2. Назначение

`MessageClassifier` отвечает только за:

```text
SourceMessage + topics -> ClassificationResult
```

Он не должен:

- писать в SQLite;
- создавать topics;
- изменять jobs;
- сохранять KnowledgeItem;
- работать с Markdown export.

### Результат этапа

LLM/mock-классификатор отделён от worker/usecase-логики.

---

## 8. Обновить prompt/JSON-контракт AI

### 8.1. AI должен возвращать строгий JSON

Пример результата для знания:

```json
{
  "kind": "knowledge",
  "title": "Использовать SQLite вместо Postgres",
  "body": "В проекте локальной базы знаний нужно использовать SQLite, чтобы пользователю не приходилось отдельно устанавливать Postgres.",
  "topic_slugs": ["storage", "architecture"],
  "confidence": 0.94,
  "data": {
    "database": "sqlite",
    "replaces": "postgres"
  }
}
```

Пример результата для unknown:

```json
{
  "kind": "unknown",
  "title": "Неразобранное сообщение",
  "body": "Исходное сообщение не удалось уверенно классифицировать.",
  "topic_slugs": [],
  "confidence": 0.2,
  "reason": "Message does not contain stable knowledge."
}
```

### 8.2. AI не должен создавать topics

В prompt нужно явно указать:

```text
Ты можешь выбирать только из переданного списка topic_slugs.
Нельзя придумывать новые topic_slugs.
Если подходящего topic нет, верни kind = "unknown".
```

### 8.3. Входные данные для AI

В классификатор передавать:

```text
- raw text сообщения;
- список доступных topics;
- инструкцию вернуть только JSON.
```

### Результат этапа

AI возвращает результат, который backend может безопасно распарсить.

---

## 9. Обновить worker flow

### 9.1. Новый pipeline

Worker должен выполнять шаги:

```text
1. Взять pending job.
2. Получить SourceMessage.
3. Загрузить topics.
4. Исключить topic unknown из списка для AI.
5. Вызвать MessageClassifier.
6. Получить ClassificationResult.
7. Провалидировать ClassificationResult.
8. Resolve topic_slugs в Topic.
9. Создать KnowledgeItem.
10. Сохранить KnowledgeItem.
11. Сохранить связи knowledge_item_topics.
12. Пометить Job как done.
```

### 9.2. Правила обработки результата AI

#### Сценарий A: нормальное знание

Условия:

```text
kind == knowledge
confidence >= threshold
topic_slugs не пустой
все topic_slugs существуют
```

Действие:

```text
сохранить KnowledgeItem с найденными topics
```

#### Сценарий B: AI вернула unknown

Условия:

```text
kind == unknown
```

Действие:

```text
сохранить KnowledgeItem с topic unknown
```

#### Сценарий C: низкая уверенность

Условия:

```text
confidence < threshold
```

Действие:

```text
сохранить KnowledgeItem с topic unknown
```

#### Сценарий D: AI вернула неизвестный slug

Условия:

```text
topic_slugs содержит slug, которого нет в базе
```

Действие для MVP:

```text
заменить весь набор topics на unknown
```

AI не должен автоматически создавать topic.

#### Сценарий E: невалидный JSON или ошибка LLM

Условия:

```text
MessageClassifier вернул ошибку
или JSON не распарсился
```

Действие:

```text
увеличить attempts;
сохранить error в Job;
если попытки ещё есть — вернуть Job в pending;
если попытки закончились — JobStatusFailed.
```

### Результат этапа

Worker стабильно обрабатывает все основные варианты результата классификации.

---

## 10. Добавить threshold для confidence

### 10.1. Минимальный вариант

Можно временно захардкодить значение в worker/usecase:

```go
const minClassificationConfidence = 0.7
```

Если в проекте принято не добавлять лишние константы заранее, можно оставить inline-значение в месте проверки.

### 10.2. Правило

```text
confidence < threshold => topic unknown
```

### Результат этапа

Низкоуверенные результаты AI не попадают в обычные topics.

---

## 11. Обновить search/answer поведение

### 11.1. Обычный поиск

Обычный поиск по базе знаний должен исключать topic `unknown`:

```sql
WHERE topics.slug != 'unknown'
```

### 11.2. Обычный answer pipeline

Когда AI отвечает на вопрос пользователя по базе знаний, записи из `unknown` не должны попадать в контекст.

### 11.3. Режим просмотра unknown

Для ручного разбора сделать отдельный query/метод:

```sql
WHERE topics.slug = 'unknown'
```

### Результат этапа

Технический мусор не участвует в обычных ответах базы знаний.

---

## 12. Добавить базовые тесты

### 12.1. Unit-тесты для topic repository

Проверить:

- `EnsureUnknownTopic` создаёт topic, если его нет;
- `EnsureUnknownTopic` возвращает существующий topic, если он уже есть;
- `FindTopicsBySlugs` возвращает корректные topics.

### 12.2. Unit/integration-тесты для knowledge repository

Проверить:

- сохранение `KnowledgeItem`;
- сохранение связей с topics;
- сохранение item и topic links в одной транзакции.

### 12.3. Тесты worker flow

Проверить сценарии:

- `kind == knowledge`, известный topic;
- `kind == unknown`;
- низкий confidence;
- неизвестный topic slug;
- ошибка классификатора;
- невалидный результат классификатора.

### 12.4. Тесты search behavior

Проверить:

- обычный поиск исключает `unknown`;
- явный режим просмотра unknown возвращает только `unknown`.

### Результат этапа

Основной pipeline покрыт тестами.

---

## 13. Прогнать проверки проекта

Выполнить:

```sh
gofmt -w <changed-go-files>
go test ./...
go vet ./...
```

Если в проекте используются дополнительные проверки, запустить и их.

### Результат этапа

Проект собирается, тесты проходят, форматирование применено.

---

## 14. Ручная проверка MVP

Проверить руками следующие сценарии.

### 14.1. Сообщение попадает в известный topic

Вход:

```text
Нужно использовать SQLite вместо Postgres для локального приложения.
```

Ожидаемый результат:

```text
KnowledgeItem создан.
Связан с topic storage/architecture.
JobStatus = done.
```

### 14.2. Сообщение попадает в unknown

Вход:

```text
ага, потом посмотрим
```

Ожидаемый результат:

```text
KnowledgeItem создан.
Связан только с topic unknown.
JobStatus = done.
```

### 14.3. AI вернула неизвестный slug

Входной результат AI:

```json
{
  "kind": "knowledge",
  "title": "Test",
  "body": "Test",
  "topic_slugs": ["not-existing-topic"],
  "confidence": 0.9
}
```

Ожидаемый результат:

```text
KnowledgeItem создан.
Связан с topic unknown.
Новый topic не создан.
```

### 14.4. Обычный поиск не видит unknown

Ожидаемый результат:

```text
Записи из topic unknown не возвращаются.
```

### 14.5. Явный просмотр unknown работает

Ожидаемый результат:

```text
Можно получить список записей из topic unknown отдельным запросом.
```

---

## 15. Definition of Done

Реализацию можно считать законченной, когда выполнены все пункты:

- [ ] В доменной модели есть `Topic`.
- [ ] В `KnowledgeItem` больше нет `Category`.
- [ ] В `KnowledgeItem` есть `Topics []*Topic`.
- [ ] В `ClassificationResult` больше нет `Category`.
- [ ] В `ClassificationResult` есть `TopicSlugs []string`.
- [ ] В SQLite есть таблица `topics`.
- [ ] В SQLite есть таблица `knowledge_item_topics`.
- [ ] В `knowledge_items` нет поля `category`.
- [ ] При старте приложения гарантируется наличие topic `unknown`.
- [ ] AI получает список доступных topics без `unknown`.
- [ ] AI возвращает только JSON.
- [ ] Backend валидирует результат AI.
- [ ] Backend сохраняет `KnowledgeItem`.
- [ ] Backend сохраняет связи `KnowledgeItem` с topics.
- [ ] `kind == unknown` сохраняется в topic `unknown`.
- [ ] Низкий confidence сохраняется в topic `unknown`.
- [ ] Неизвестные AI slugs не создают новые topics.
- [ ] Обычный search исключает `unknown`.
- [ ] Явный режим просмотра unknown работает.
- [ ] `go test ./...` проходит.
- [ ] `go vet ./...` проходит.

---

## 16. Рекомендуемый порядок коммитов

### Commit 1

```text
domain: add topics to knowledge model
```

Содержит:

- `Topic`;
- обновлённый `KnowledgeItem`;
- обновлённый `ClassificationResult`.

### Commit 2

```text
storage: add topics schema
```

Содержит:

- миграции;
- `topics`;
- `knowledge_item_topics`;
- индексы;
- удаление/игнорирование `category`.

### Commit 3

```text
storage: add topic repository
```

Содержит:

- `EnsureUnknownTopic`;
- `ListTopics`;
- `FindTopicsBySlugs`.

### Commit 4

```text
storage: save knowledge items with topics
```

Содержит:

- сохранение `KnowledgeItem`;
- сохранение связей с topics;
- транзакционность.

### Commit 5

```text
ai: introduce message classifier result with topic slugs
```

Содержит:

- интерфейс `MessageClassifier`;
- обновление mock/LLM-классификатора;
- обновление prompt JSON-контракта.

### Commit 6

```text
worker: classify messages into topics
```

Содержит:

- новый worker flow;
- обработку unknown;
- обработку low confidence;
- обработку unknown slugs.

### Commit 7

```text
search: exclude unknown topic by default
```

Содержит:

- обычный search без `unknown`;
- отдельный режим просмотра `unknown`.

### Commit 8

```text
test: cover topic classification mvp
```

Содержит:

- repository tests;
- worker tests;
- search behavior tests.

---

## 17. Короткая инструкция для AI-агента

Если этот план будет передан AI-агенту, выполнять строго по порядку:

```text
1. Сначала изменить доменные модели.
2. Потом изменить SQLite-схему.
3. Потом добавить репозитории topics.
4. Потом обновить сохранение knowledge items.
5. Потом обновить classifier contract.
6. Потом обновить worker.
7. Потом обновить search behavior.
8. Потом добавить тесты.
9. Потом прогнать gofmt, go test, go vet.
```

Не добавлять:

```text
- MCP;
- agents framework;
- embeddings;
- vector search;
- Markdown sync;
- новые сущности без необходимости.
```