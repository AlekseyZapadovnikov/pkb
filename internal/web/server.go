package web

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"pkb/internal/usecase"
	"pkb/internal/usecase/domain"
)

//go:embed templates/*.html static/*
var content embed.FS

// ServerConfig содержит настройки, необходимые HTTP-слою.
type ServerConfig struct{}

// MessageSubmitter описывает use case, который принимает исходное сообщение из HTTP-слоя.
type MessageSubmitter interface {
	SubmitMessage(ctx context.Context, input usecase.SubmitMessageInput) (domain.SourceMessage, error)
}

// Server хранит зависимости HTTP-слоя: шаблоны, статические файлы, логгер, use case и состояние запуска.
type Server struct {
	logger    *slog.Logger
	staticFS  fs.FS
	templates *template.Template
	startedAt time.Time

	messages MessageSubmitter
}

// HomePageData описывает данные, которые передаются в шаблон главной страницы.
type HomePageData struct {
	Title     string
	StartedAt string
}

// NewServer подготавливает шаблоны, статические файлы и возвращает обёртку HTTP-сервера.
func NewServer(_ ServerConfig, logger *slog.Logger, messages MessageSubmitter) (*Server, error) {
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
	}, nil
}

// Routes собирает HTTP-маршруты приложения.
func (s *Server) Routes() http.Handler {
	router := chi.NewRouter()
	router.Get("/", s.handleHome)
	router.Get("/healthz", s.handleHealthz)
	router.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(s.staticFS))))
	return router
}

// handleHome рендерит главную страницу каркаса приложения.
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	data := HomePageData{
		Title:     "Personal KB",
		StartedAt: s.startedAt.Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "home.html", data); err != nil {
		s.logger.Error("render home page", "error", err)
	}
}

// handleHealthz отдаёт простую проверку состояния сервера.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := w.Write([]byte("{\"status\":\"ok\"}\n")); err != nil {
		s.logger.Warn("write health response", "error", err)
	}
}
