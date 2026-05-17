-- +goose Up
CREATE TABLE source_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_type TEXT NOT NULL,
    raw_text TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE processing_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_type TEXT NOT NULL,
    source_message_id INTEGER,
    status TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    error TEXT,
    started_at TEXT,
    finished_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (source_message_id) REFERENCES source_messages(id)
);

CREATE TABLE knowledge_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_message_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    confidence REAL NOT NULL,
    data_json TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (source_message_id) REFERENCES source_messages(id)
);

CREATE TABLE topics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE knowledge_item_topics (
    knowledge_item_id INTEGER NOT NULL,
    topic_id INTEGER NOT NULL,
    PRIMARY KEY (knowledge_item_id, topic_id),
    FOREIGN KEY (knowledge_item_id) REFERENCES knowledge_items(id) ON DELETE CASCADE,
    FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE CASCADE
);

CREATE INDEX idx_processing_jobs_status ON processing_jobs(status);
CREATE INDEX idx_processing_jobs_source_message_id ON processing_jobs(source_message_id);
CREATE INDEX idx_knowledge_items_source_message_id ON knowledge_items(source_message_id);
CREATE INDEX idx_topics_slug ON topics(slug);
CREATE INDEX idx_knowledge_item_topics_topic_id ON knowledge_item_topics(topic_id);
CREATE INDEX idx_knowledge_item_topics_item_id ON knowledge_item_topics(knowledge_item_id);

-- +goose Down
DROP TABLE IF EXISTS knowledge_item_topics;
DROP TABLE IF EXISTS topics;
DROP TABLE IF EXISTS knowledge_items;
DROP TABLE IF EXISTS processing_jobs;
DROP TABLE IF EXISTS source_messages;
