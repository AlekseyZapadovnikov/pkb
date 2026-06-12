package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"pkb/internal/usecase/domain"
)

//go:embed templates/*.html static/*
var content embed.FS

// MessageSubmitter описывает use case, который принимает исходное сообщение из HTTP-слоя.
type MessageSubmitter interface {
	ProcessMessage(context.Context, *domain.SourceMessage) (jobID int64, err error)
}

type TopicManager interface {
	CreateTopic(context.Context, *domain.Topic) (topicID int64, err error)
	DeleteTopic(context.Context, string) error
	UpdateTopicDescription(context.Context, string, string) error
	GetTopics(context.Context) ([]*domain.Topic, error)
}

// Server хранит зависимости HTTP-слоя: шаблоны, статические файлы, логгер, use case и состояние запуска.
type Server struct {
	logger    *slog.Logger
	staticFS  fs.FS
	templates *template.Template
	startedAt time.Time

	messages MessageSubmitter
	topics   TopicManager
}

// HomePageData описывает данные, которые передаются в шаблон главной страницы.
type HomePageData struct {
	Title     string
	StartedAt string
	Topics    []*domain.Topic
}

// NewServer подготавливает шаблоны, статические файлы и возвращает обёртку HTTP-сервера.
func NewServer(logger *slog.Logger, messages MessageSubmitter, topics TopicManager) (*Server, error) {
	if logger == nil {
		logger = slog.Default()
	}

	staticFS, err := fs.Sub(content, "static")
	if err != nil {
		return nil, fmt.Errorf("load static fs: %w", err)
	}

	templates, err := template.ParseFS(content, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	return &Server{
		logger:    logger,
		staticFS:  staticFS,
		templates: templates,
		startedAt: time.Now(),
		messages:  messages,
		topics:    topics,
	}, nil
}

// Routes собирает HTTP-маршруты приложения.
func (s *Server) Routes() http.Handler {
	router := chi.NewRouter()
	router.Get("/", s.handleHome)
	router.Post("/message", s.handleTextMessage)
	router.Get("/topics", s.handleGetTopics)
	router.Post("/topics", s.handleCreateTopic)
	router.Patch("/topics/{slug}/description", s.handleUpdateTopicDescription)
	router.Delete("/topics/{slug}", s.handleDeleteTopic)
	router.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(s.staticFS))))
	return router
}

// handleHome рендерит главную страницу каркаса приложения.
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	topics, err := s.topics.GetTopics(r.Context())
	if err != nil {
		s.logger.Error("get topics for home page", "error", err)
		http.Error(w, "failed to load topics", http.StatusInternalServerError)
		return
	}

	data := HomePageData{
		Title:     "Personal KB",
		StartedAt: s.startedAt.Format(time.RFC3339),
		Topics:    topics,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "home.html", data); err != nil {
		s.logger.Error("render home page", "error", err)
	}
}

// POST ./message
func (s *Server) handleTextMessage(w http.ResponseWriter, r *http.Request) {

	if err := r.ParseForm(); err != nil {
		s.logger.Error("parse form", "error", err)
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	RawMessage := r.FormValue("raw_text")

	if RawMessage == "" {
		http.Error(w, "raw_text is required", http.StatusBadRequest)
		return
	}

	input := domain.SourceMessage{
		SourceType: domain.SourceTypeWebUI,
		RawText:    RawMessage,
		CreatedAt:  time.Now(),
	}

	jobID, err := s.messages.ProcessMessage(r.Context(), &input)
	if err != nil {
		s.logger.Error("process message", "error", err)
		http.Error(w, "failed to process message", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(fmt.Sprintf("jobID: %d", jobID)))
}

func (s *Server) handleCreateTopic(w http.ResponseWriter, r *http.Request) {

	if err := r.ParseForm(); err != nil {
		s.logger.Error("parse topic form", "error", err)
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	input := domain.Topic{
		Name:        name,
		Description: r.FormValue("description"),
	}

	if _, err := s.topics.CreateTopic(r.Context(), &input); err != nil {
		s.logger.Error("create topic", "error", err)
		http.Error(w, "failed to create topic", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleUpdateTopicDescription(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		http.Error(w, "slug is required", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		s.logger.Error("parse topic description form", "error", err)
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	if err := s.topics.UpdateTopicDescription(r.Context(), slug, r.FormValue("description")); err != nil {
		s.logger.Error("update topic description", "error", err, "slug", slug)
		http.Error(w, "failed to update topic description", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteTopic(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		http.Error(w, "slug is required", http.StatusBadRequest)
		return
	}

	if err := s.topics.DeleteTopic(r.Context(), slug); err != nil {
		s.logger.Error("delete topic", "error", err, "slug", slug)
		http.Error(w, "failed to delete topic", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetTopics(w http.ResponseWriter, r *http.Request) {
	topics, err := s.topics.GetTopics(r.Context())
	if err != nil {
		s.logger.Error("get topics", "error", err)
		http.Error(w, "failed to get topics", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(topics); err != nil {
		s.logger.Error("write topics response", "error", err)
	}
}
