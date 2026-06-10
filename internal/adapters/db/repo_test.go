package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"pkb/internal/usecase/domain"
)

func TestRepositorySaveKnowledgeItem(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepository(t, ctx)

	messageID := saveTestMessage(t, ctx, repo)
	topicID, err := repo.SaveTopic(ctx, &domain.Topic{
		Slug:        "go",
		Name:        "Go",
		Description: "Go notes",
	})
	if err != nil {
		t.Fatalf("save topic: %v", err)
	}

	item := &domain.KnowledgeItem{
		Body: &domain.KnowledgeItemBody{
			SourceMessageID: messageID,
			Title:           "Interfaces",
			Body:            "Small interfaces are easier to reuse.",
			Topics: []*domain.Topic{
				{Slug: "go"},
			},
			Confidence: 0.9,
			DataJSON:   json.RawMessage(`{"tag":"go"}`),
		},
	}

	itemID, err := repo.SaveKnowledgeItem(ctx, item)
	if err != nil {
		t.Fatalf("save knowledge item: %v", err)
	}
	if item.ID != itemID {
		t.Fatalf("expected item ID to be set to %d, got %d", itemID, item.ID)
	}

	var got struct {
		SourceMessageID int64          `db:"source_message_id"`
		Title           string         `db:"title"`
		Body            string         `db:"body"`
		Confidence      float64        `db:"confidence"`
		DataJSON        sql.NullString `db:"data_json"`
	}
	if err := repo.db.QueryRowxContext(ctx, `
		SELECT source_message_id, title, body, confidence, data_json
		FROM knowledge_items
		WHERE id = ?
	`, itemID).StructScan(&got); err != nil {
		t.Fatalf("select knowledge item: %v", err)
	}

	if got.SourceMessageID != messageID {
		t.Fatalf("expected source_message_id %d, got %d", messageID, got.SourceMessageID)
	}
	if got.Title != "Interfaces" || got.Body != "Small interfaces are easier to reuse." {
		t.Fatalf("unexpected knowledge item body: %#v", got)
	}
	if got.Confidence != 0.9 {
		t.Fatalf("expected confidence 0.9, got %f", got.Confidence)
	}
	if !got.DataJSON.Valid || got.DataJSON.String != `{"tag":"go"}` {
		t.Fatalf("expected data_json to be stored, got %#v", got.DataJSON)
	}

	var gotTopicID int64
	if err := repo.db.QueryRowxContext(ctx, `
		SELECT topic_id
		FROM knowledge_item_topics
		WHERE knowledge_item_id = ?
	`, itemID).Scan(&gotTopicID); err != nil {
		t.Fatalf("select knowledge item topic: %v", err)
	}
	if gotTopicID != topicID {
		t.Fatalf("expected topic_id %d, got %d", topicID, gotTopicID)
	}
}

func TestRepositorySaveUnknownKnowledgeItem(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepository(t, ctx)

	messageID := saveTestMessage(t, ctx, repo)
	item := &domain.UnknownKnowledgeItem{
		SourceMessageID: messageID,
	}
	body := &domain.UnknownKnowledgeItemBody{
		Reason:        "No matching topic",
		RawOutputJSON: json.RawMessage(`{"kind":"unknown"}`),
		SugestTopics: []domain.ShortTopic{
			{
				Name:        "maybe-go",
				Description: "Possible Go notes",
			},
		},
	}

	itemID, err := repo.SaveUnknownKnowledgeItem(ctx, item, body)
	if err != nil {
		t.Fatalf("save unknown knowledge item: %v", err)
	}
	if item.ID != itemID {
		t.Fatalf("expected item ID to be set to %d, got %d", itemID, item.ID)
	}

	var got struct {
		SourceMessageID   int64          `db:"source_message_id"`
		Reason            string         `db:"reason"`
		RawOutputJSON     sql.NullString `db:"raw_output_json"`
		SuggestTopicsJSON sql.NullString `db:"suggest_topics_json"`
	}
	if err := repo.db.QueryRowxContext(ctx, `
		SELECT source_message_id, reason, raw_output_json, suggest_topics_json
		FROM unknown_knowledge_items
		WHERE id = ?
	`, itemID).StructScan(&got); err != nil {
		t.Fatalf("select unknown knowledge item: %v", err)
	}

	if got.SourceMessageID != messageID {
		t.Fatalf("expected source_message_id %d, got %d", messageID, got.SourceMessageID)
	}
	if got.Reason != "No matching topic" {
		t.Fatalf("expected reason to be stored, got %q", got.Reason)
	}
	if !got.RawOutputJSON.Valid || got.RawOutputJSON.String != `{"kind":"unknown"}` {
		t.Fatalf("expected raw_output_json to be stored, got %#v", got.RawOutputJSON)
	}
	if !got.SuggestTopicsJSON.Valid || got.SuggestTopicsJSON.String != `[{"slug":"maybe-go","description":"Possible Go notes"}]` {
		t.Fatalf("expected suggest_topics_json to be stored, got %#v", got.SuggestTopicsJSON)
	}
}

func newTestRepository(t *testing.T, ctx context.Context) *Repository {
	t.Helper()

	database, err := Open(ctx, filepath.Join(t.TempDir(), "test.sqlite3"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("close test database: %v", err)
		}
	})

	if err := Migrate(ctx, database); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	return NewRepository(database)
}

func saveTestMessage(t *testing.T, ctx context.Context, repo *Repository) int64 {
	t.Helper()

	messageID, err := repo.SaveMessage(ctx, &domain.SourceMessage{
		SourceType: domain.SourceTypeWebUI,
		RawText:    "raw text",
	})
	if err != nil {
		t.Fatalf("save message: %v", err)
	}

	return messageID
}
