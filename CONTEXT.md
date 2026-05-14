# Контекст проекта

Мы строим локальное веб-приложение на Go для личной базы знаний пользователя.

Главная идея: пользователь вводит текстовое сообщение через локальный web UI, приложение сохраняет оригинал без изменений, создаёт задачу обработки, а затем классификатор превращает сообщение в структурированную запись базы знаний. Проектировать архитетуру необходимо так, чтобы в дальнейшем пользователь мог вводить это сообщение через другие ресурсы, например, телеграмм бота или через любой другой сторонний ресурс.

На текущем этапе SQLite является единственным обязательным хранилищем и главным источником правды. Все важные данные должны храниться в SQLite.

Markdown/Obsidian и vector index в будущем рассматриваются только как производные представления. Их можно пересобрать из SQLite, поэтому они не должны становиться основным хранилищем данных.

## Цель первой версии

Сделать надёжное ядро памяти:

```text
raw text -> processing -> classification -> structured knowledge
```
(в дальнейшем будет ещё и Obsidian data после того, как ролизошла классификация сообщения, будет запускаться еще 1 агент, который будет обновлять данные в Obsidian)
Первая версия считается готовой, когда пользователь может:

- открыть локальный web UI;
- ввести текст;
- сохранить raw message в SQLite;
- автоматически создать processing job;
- обработать job worker'ом;
- получить либо `knowledge_item`, либо `unknown_item`;
- посмотреть source messages, knowledge items и unknown items через web UI.

## Текущий поток

```text
Пользователь открывает localhost web UI
        ↓
вводит текстовое сообщение
        ↓
backend сохраняет raw message в SQLite
        ↓
создаётся processing job
        ↓
worker берёт job
        ↓
AI/mock classifier классифицирует сообщение
        ↓
если уверенность высокая — создаётся knowledge item
        ↓
если уверенность низкая — создаётся unknown item
```

## Что делаем сейчас

В первой итерации строим простое локальное web-приложение:

- HTTP-сервер на Go;
- простая главная страница с textarea и submit;
- SQLite storage;
- миграции;
- сохранение исходных сообщений;
- очередь processing jobs в SQLite;
- встроенный worker;
- mock classifier до подключения реального LLM;
- сохранение уверенных результатов в `knowledge_items`;
- сохранение неуверенных или невалидных результатов в `unknown_items`;
- страницы просмотра source messages, knowledge items и unknown items.

## Что не делаем в первой версии

Пока не добавляем:

- Telegram input;
- Obsidian/Markdown export как обязательную часть;
- vector database;
- semantic search;
- voice messages;
- PDF/documents;
- background scheduler;
- calendar integration;
- finance agent;
- multi-agent system;
- сложные графовые связи;
- полноценную систему пользователей;
- синхронизацию между устройствами;
- Postgres;
- Kafka/Redis;
- Docker как обязательную часть разработки.

## Главные правила разработки

- Не усложнять архитектуру раньше времени.
- Использовать SQLite как единственную обязательную базу данных.
- Все важные данные хранить в SQLite.
- Сначала реализовать простой web input, а не Telegram.
- AI не должен напрямую писать в базу.
- AI возвращает JSON, backend валидирует его и только потом пишет данные.
- Все исходные сообщения сохранять без изменений.
- Любую обработку должно быть возможно перезапустить.
- Если классификация невалидная, неполная или confidence низкий, сообщение должно попасть в `unknown`.
- `unknown` — это не ошибка, а inbox для неразобранных сообщений.
- Не делать approve-flow на первом этапе: уверенные записи сохраняются автоматически.
- Код должен оставаться простым, читаемым и расширяемым.

## Основные сущности

### `source_messages`

Хранит исходные сообщения пользователя.

Примерные поля:

```text
id
source
text
status
error
raw_payload
created_at
updated_at
```

`source` на первом этапе:

```text
web
```

Возможные будущие значения:

```text
telegram
cli
api
```

`status`:

```text
received
queued
processing
processed
unknown
failed
```

Смысл таблицы:

- сохранить оригинал сообщения;
- не потерять данные при сбое;
- понимать, что уже обработано;
- иметь возможность переобработать сообщение позже;
- дебажить работу классификатора.

### `processing_jobs`

Хранит задачи обработки.

На текущем этапе нужен только job type:

```text
classify_message
```

Примерные поля:

```text
id
job_type
source_message_id
status
attempts
max_attempts
available_at
started_at
finished_at
error
payload
created_at
updated_at
```

`status`:

```text
pending
running
done
failed
retry
dead
```

Смысл таблицы:

- отделить факт получения сообщения от его обработки;
- дать возможность повторять обработку;
- позже добавить другие job types;
- не держать обработку внутри HTTP request.

### `knowledge_items`

Хранит структурированные знания после классификации.

Примерные поля:

```text
id
source_message_id
type
title
body
summary
confidence
metadata
created_at
updated_at
```

`type` на старте:

```text
about_user
task
reminder
article
book
movie
video
project_note
study_note
idea
finance_note
code_snippet
raw_note
```

`metadata` хранится как JSON-строка, потому что разные типы знаний могут иметь разные дополнительные поля.

### `unknown_items`

Хранит сообщения, которые классификатор не смог уверенно разобрать.

Примерные поля:

```text
id
source_message_id
text
reason
confidence
suggested_type
suggested_topics
status
created_at
resolved_at
```

`status`:

```text
open
resolved
ignored
```

Сообщение должно попасть в `unknown`, если:

- confidence ниже threshold;
- classifier вернул невалидный JSON;
- type неизвестен;
- title пустой;
- body пустой;
- сообщение слишком неоднозначное;
- classifier сам выбрал `unknown`;
- произошла ошибка классификации, но raw message уже сохранён.

### `topics`

Хранит топики.

Примерные поля:

```text
id
name
slug
created_at
```

Примеры:

```text
golang
sqlite
personal-kb
ai
rag
finance
university
movies
health
```

### `knowledge_item_topics`

Связь many-to-many между `knowledge_items` и `topics`.

Поля:

```text
knowledge_item_id
topic_id
```

## Тип `about_user`

`about_user` — это обычный `knowledge_item`, но с особым смыслом.

Туда попадают устойчивые знания о пользователе:

- предпочтения;
- цели;
- ограничения;
- окружение;
- текущие проекты;
- долгосрочные планы;
- стиль работы;
- навыки.

Для `about_user` использовать `metadata`:

```json
{
  "memory_kind": "preference",
  "stability": "long_term",
  "source": "explicit"
}
```

Возможные `memory_kind`:

```text
profile_fact
preference
goal
constraint
environment
project_context
skill
```

## Контракт классификатора

Классификатор получает исходный текст и возвращает строго структурированный JSON:

```json
{
  "type": "article",
  "title": "Почитать про embeddings для локальной базы знаний",
  "body": "Пользователь хочет изучить embeddings в контексте локальной personal knowledge base.",
  "summary": "Материал для чтения про embeddings.",
  "topics": ["ai", "embeddings", "personal-kb"],
  "confidence": 0.87,
  "metadata": {
    "status": "to_read"
  },
  "reason_if_unknown": ""
}
```

Если сообщение непонятное:

```json
{
  "type": "unknown",
  "title": "",
  "body": "посмотреть это потом",
  "summary": "",
  "topics": ["unknown"],
  "confidence": 0.25,
  "metadata": {},
  "reason_if_unknown": "Неясно, что именно нужно посмотреть."
}
```

Backend обязан валидировать результат классификации.

## Confidence threshold

В конфиге должен быть threshold:

```text
classification_confidence_threshold = 0.75
```

Логика:

```text
если confidence >= threshold и результат валиден
    создать knowledge_item
иначе
    создать unknown_item
```

Даже при высокой confidence сообщение должно попасть в `unknown`, если результат невалидный.

## Web UI первой версии

Нужные страницы:

- Home / input page;
- Source messages list;
- Knowledge items list;
- Unknown items list;
- Knowledge item details;
- Unknown item details.

На главной странице:

- textarea для ввода текста;
- кнопка Submit;
- статус после отправки.

На странице source messages:

- raw text;
- source;
- status;
- created_at;
- error, если есть.

На странице knowledge items:

- список сохранённых знаний;
- фильтр по type;
- фильтр по topic;
- ссылка на detail page.

На странице unknown items:

- список unknown items;
- причина попадания в unknown;
- confidence;
- suggested type/topics.

Пока не делаем сложное редактирование. На первом этапе достаточно просмотра.

## Правильная модель хранения

```text
SQLite = настоящая база данных.
Markdown = будущий экспорт для человека и Obsidian.
Vector index = будущий индекс для semantic search.
```

Нельзя хранить уникальные важные данные только в Markdown или только в vector index.

Если Markdown удалили — восстановить из SQLite.
Если vector index удалили — пересобрать из SQLite.
Если SQLite удалили — данные потеряны.

Поэтому SQLite — source of truth.

