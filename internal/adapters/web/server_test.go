package web

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"pkb/internal/usecase/domain"
)

type fakeTopicManager struct {
	topics  []*domain.Topic
	created *domain.Topic
	deleted string
}

func (m *fakeTopicManager) CreateTopic(_ context.Context, topic *domain.Topic) (int64, error) {
	m.created = topic
	return 1, nil
}

func (m *fakeTopicManager) DeleteTopic(_ context.Context, slug string) error {
	m.deleted = slug
	return nil
}

func (m *fakeTopicManager) GetTopics(context.Context) ([]*domain.Topic, error) {
	return m.topics, nil
}

func TestHomeShowsTopics(t *testing.T) {
	manager := &fakeTopicManager{
		topics: []*domain.Topic{
			{
				ID:          1,
				Slug:        "golang",
				Name:        "Golang",
				Description: "Go notes",
			},
		},
	}
	server := newTestServer(t, manager)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	server.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	body := recorder.Body.String()
	for _, want := range []string{"Golang", "golang", "Go notes", "data-show-topics", "/static/topics.js"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected home page to contain %q", want)
		}
	}
}

func TestGetTopicsReturnsJSON(t *testing.T) {
	manager := &fakeTopicManager{
		topics: []*domain.Topic{
			{
				ID:          1,
				Slug:        "golang",
				Name:        "Golang",
				Description: "Go notes",
			},
		},
	}
	server := newTestServer(t, manager)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/topics", nil)
	server.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if !strings.Contains(recorder.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("expected json response, got %q", recorder.Header().Get("Content-Type"))
	}
	if !strings.Contains(recorder.Body.String(), `"Slug":"golang"`) {
		t.Fatalf("expected topics json, got %q", recorder.Body.String())
	}
}

func TestCreateTopicRedirectsAndCallsManager(t *testing.T) {
	manager := &fakeTopicManager{}
	server := newTestServer(t, manager)

	form := url.Values{
		"name":        {"Golang"},
		"description": {"Go notes"},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/topics", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	server.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, recorder.Code)
	}
	if recorder.Header().Get("Location") != "/" {
		t.Fatalf("expected redirect to /, got %q", recorder.Header().Get("Location"))
	}
	if manager.created == nil {
		t.Fatal("expected topic to be created")
	}
	if manager.created.Name != "Golang" || manager.created.Description != "Go notes" {
		t.Fatalf("unexpected created topic: %#v", manager.created)
	}
}

func TestDeleteTopicRedirectsAndCallsManager(t *testing.T) {
	manager := &fakeTopicManager{}
	server := newTestServer(t, manager)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/topics/golang/delete", nil)
	server.Routes().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, recorder.Code)
	}
	if recorder.Header().Get("Location") != "/" {
		t.Fatalf("expected redirect to /, got %q", recorder.Header().Get("Location"))
	}
	if manager.deleted != "golang" {
		t.Fatalf("expected deleted slug golang, got %q", manager.deleted)
	}
}

func newTestServer(t *testing.T, topics TopicManager) *Server {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server, err := NewServer(logger, nil, topics)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	return server
}
