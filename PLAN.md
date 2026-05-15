# План разработки

Цель плана — постепенно собрать MVP локальной базы знаний без преждевременного усложнения архитектуры.

Основной фокус первой версии:

```text
web input -> SQLite -> processing job -> classifier -> knowledge/unknown
```

## Допущения

- Приложение пишется на Go.
- Приложение запускается локально.
- SQLite используется как единственный обязательный storage.
- На первом этапе используется mock classifier.
- Реальный LLM provider подключается после того, как весь pipeline уже работает.
- Telegram, Obsidian, vector search и агенты откладываются до MVP.

## Этап 1. Skeleton приложения

Сделать минимальное Go web-приложение.

Нужно:

- запуск локального HTTP-сервера;
- простая главная страница;
- health endpoint;
- конфиг приложения;
- директория для SQLite-файла.

Результат:

```text
приложение запускается локально и открывается в браузере.
```

## Этап 2. SQLite и миграции

Подключить SQLite как основной storage.

Создать таблицы:

- `source_messages`;
- `processing_jobs`;
- `knowledge_items`;
- `unknown_items`;
- `topics`;
- `knowledge_item_topics`.

Результат:

```text
приложение умеет создавать и использовать локальный SQLite-файл.
```

## Этап 3. Raw text ingest

Сделать форму ввода текста.

При submit:

- сохранить текст в `source_messages`;
- поставить `source_messages.status = received`;
- создать `processing_jobs` запись с `job_type = classify_message`;
- поставить `processing_jobs.status = pending`.

Результат:

```text
пользователь может ввести текст, и оно сохраняется как raw message.
```

## Этап 4. Source messages list

Добавить страницу просмотра входящих сообщений.

Показывать:

- text;
- source;
- status;
- created_at;
- error.

Результат:

```text
можно видеть, какие сообщения попали в систему и в каком они статусе.
```

## Этап 5. Worker обработки jobs

Сделать простой worker внутри приложения.

Worker должен:

- искать pending jobs;
- брать одну job в обработку;
- менять job status на `running`;
- менять source message status на `processing`;
- запускать обработчик;
- завершать job как `done`, `retry`, `failed` или `dead`.

На этом этапе можно использовать mock classifier.

Результат:

```text
processing_jobs реально выполняются.
```

## Этап 6. Mock AI classifier

Перед реальным LLM сделать mock classifier.

Пример начальных правил:

```text
если текст содержит "почитать" -> article
если текст содержит "фильм" -> movie
если текст содержит "надо" -> task
иначе -> unknown
```

Mock classifier должен возвращать тот же структурированный результат, что и будущий AI classifier.

Результат:

```text
можно проверить весь pipeline без реального AI.
```

## Этап 7. Knowledge items flow

После уверенной классификации:

- создать `knowledge_item`;
- создать или найти topics;
- связать item с topics через `knowledge_item_topics`;
- обновить `source_messages.status = processed`;
- обновить `processing_jobs.status = done`.

Результат:

```text
raw message превращается в structured knowledge item.
```

## Этап 8. Unknown flow

Если классификатор не уверен или результат невалидный:

- создать `unknown_item`;
- сохранить причину;
- сохранить confidence;
- сохранить suggested type/topics;
- обновить `source_messages.status = unknown`;
- обновить `processing_jobs.status = done`, если технической ошибки не было.

Результат:

```text
сомнительные сообщения не теряются и попадают в unknown.
```

## Этап 9. Knowledge и unknown pages

Добавить страницы просмотра:

- всех knowledge items;
- knowledge item details;
- всех unknown items;
- unknown item details.

Результат:

```text
пользователь видит, что система сохранила и что не смогла разобрать.
```

## Этап 10. Реальный AI classifier

Заменить mock classifier на реальный LLM provider через общий интерфейс.

Важно:

- общий интерфейс classifier должен остаться;
- mock classifier не удалять;
- реальный classifier должен возвращать JSON;
- backend должен валидировать JSON;
- при невалидном результате сохранять `unknown_item`;
- при технической ошибке переводить job в retry/failed.

Результат:

```text
система реально классифицирует сообщения через AI.
```

## Этап 11. Confidence threshold

Добавить настройку:

```text
classification_confidence_threshold
```

Использовать её при обработке результата classifier.

Результат:

```text
можно управлять строгостью классификации.
```

## Этап 12. Простая фильтрация и поиск

В web UI добавить:

- фильтр knowledge items по type;
- фильтр по topic;
- простой текстовый поиск по title/body/summary.

Результат:

```text
базой уже можно пользоваться.
```

## Definition of Done для MVP

MVP готов, когда:

- приложение запускается локально;
- пользователь может открыть web UI;
- пользователь может ввести текст;
- текст сохраняется в `source_messages`;
- создаётся processing job;
- worker обрабатывает job;
- mock или AI classifier классифицирует текст;
- уверенный результат попадает в `knowledge_items`;
- неуверенный результат попадает в `unknown_items`;
- можно посмотреть source messages;
- можно посмотреть knowledge items;
- можно посмотреть unknown items;
- все важные данные лежат в SQLite.

## После MVP

После рабочего pipeline добавлять по очереди:

- Markdown export;
- Telegram input;
- unknown manual resolve;
- full-text search;
- embeddings;
- semantic search;
- attachments;
- digest/reflection agent;
- specialized agents.

