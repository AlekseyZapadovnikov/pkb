package web

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"pkb/internal/usecase/domain"
	"pkb/internal/usecase/service/valid"
)

//go:embed templates/*.html static/*
var content embed.FS

const (
	defaultSearchKnowledgeLimit = 10
)

// MessageSubmitter defines the use case that accepts a source message from the HTTP layer.
type MessageSubmitter interface {
	ProcessMessage(context.Context, *domain.SourceMessage) (jobID int64, err error)
}

type EntityManager interface {
	CreateTopic(context.Context, *domain.Topic) (topicID int64, err error)
	DeleteTopic(context.Context, string) error
	UpdateTopicDescription(context.Context, string, string) error
	GetTopics(context.Context) ([]*domain.Topic, error)
	UpdateKnowledge(context.Context, int64, *domain.KnowledgeItem) error
}

type JobReader interface {
	GetJobByID(context.Context, int64) (*domain.Job, error)
}

type KnowledgeReader interface {
	ListKnowledgeItems(context.Context, int64, int) ([]*domain.KnowledgeItem, error)
}

type KnowledgeService interface {
	GetEntry(ctx context.Context, id int) (domain.KnowledgeItem, error)
	SearchKnowledge(ctx context.Context, query string, limit int) ([]domain.KnowledgeItem, error)
}

// Server holds HTTP layer dependencies: templates, static files, logger, and use cases.
type Server struct {
	logger    *slog.Logger
	staticFS  fs.FS
	templates *template.Template

	messages         MessageSubmitter
	topics           EntityManager
	jobs             JobReader
	knowledge        KnowledgeReader
	knowledgeService KnowledgeService
}

// HomePageData defines the data passed to the home page template.
type HomePageData struct {
	Title string
}

type TopicsPageData struct {
	Title   string
	Topics  []*domain.Topic
	Created bool
}

type messageResponse struct {
	JobID  int64            `json:"job_id"`
	Status domain.JobStatus `json:"status"`
}

type jobResponse struct {
	ID              int64            `json:"id"`
	JobType         domain.JobType   `json:"job_type"`
	SourceMessageID int64            `json:"source_message_id"`
	Status          domain.JobStatus `json:"status"`
	Attempts        int              `json:"attempts"`
	Error           *string          `json:"error"`
	StartedAt       *time.Time       `json:"started_at"`
	FinishedAt      *time.Time       `json:"finished_at"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

type KnowledgePageData struct {
	Query        string
	SearchError  string
	Title        string
	Items        []*domain.KnowledgeItem
	Limit        int
	BeforeID     int64
	HasMore      bool
	NextBeforeID int64
}

const (
	defaultKnowledgeLimit = 50
	maxKnowledgeLimit     = 100
)

// NewServer prepares templates and static files and returns an HTTP server wrapper.
func NewServer(logger *slog.Logger, messages MessageSubmitter, topics EntityManager, jobs JobReader, knowledge KnowledgeReader, knowledgeService KnowledgeService) (*Server, error) {
	if logger == nil {
		logger = slog.Default()
	}

	staticFS, err := fs.Sub(content, "static")
	if err != nil {
		return nil, fmt.Errorf("load static fs: %w", err)
	}

	templates, err := template.New("").Funcs(template.FuncMap{
		"formatConfidence": func(value *float64) string {
			return fmt.Sprintf("%.2f", *value)
		},
	}).ParseFS(content, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	return &Server{
		logger:           logger,
		staticFS:         staticFS,
		templates:        templates,
		messages:         messages,
		topics:           topics,
		jobs:             jobs,
		knowledge:        knowledge,
		knowledgeService: knowledgeService,
	}, nil
}

// Routes registers the application HTTP routes.
func (s *Server) Routes(mcpHandler http.Handler) http.Handler {
	router := chi.NewRouter()
	router.Use(s.loggingMiddleware)
	router.Handle("/mcp", mcpHandler)
	router.Get("/", s.handleHome)
	router.Get("/knowledge", s.handleKnowledge)
	router.Get("/api/knowledge/{id}", s.handleGetKnowledge)
	router.Put("/api/knowledge/{id}", s.handleUpdateKnowledge)
	router.Get("/api/search", s.handleSearchKnowledge)
	router.Post("/message", s.handleTextMessage)
	router.Get("/jobs/{id}", s.handleGetJob)
	router.Get("/topics", s.handleGetTopics)
	router.Get("/topics/manage", s.handleTopics)
	router.Post("/topics", s.handleCreateTopic)
	router.Patch("/topics/{slug}/description", s.handleUpdateTopicDescription)
	router.Delete("/topics/{slug}", s.handleDeleteTopic)
	router.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(s.staticFS))))
	return router
}

// handleHome renders the page for adding a note.
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "home.html", HomePageData{Title: "Add note"}); err != nil {
		s.logger.Error("render home page", "error", err)
	}
}

func (s *Server) handleTopics(w http.ResponseWriter, r *http.Request) {
	topics, err := s.topics.GetTopics(r.Context())
	if err != nil {
		s.logger.Error("get topics for manage page", "error", err)
		http.Error(w, "failed to load topics", http.StatusInternalServerError)
		return
	}

	data := TopicsPageData{
		Title:   "Topics",
		Topics:  topics,
		Created: r.URL.Query().Get("created") == "1",
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "topics.html", data); err != nil {
		s.logger.Error("render topics page", "error", err)
	}
}

// handleKnowledge renders the knowledge list html page.
func (s *Server) handleKnowledge(w http.ResponseWriter, r *http.Request) {
	data := KnowledgePageData{Title: "Knowledge", Query: r.URL.Query().Get("query")}
	if strings.TrimSpace(data.Query) != "" {
		data.Limit = defaultSearchKnowledgeLimit
		items, err := s.knowledgeService.SearchKnowledge(r.Context(), data.Query, data.Limit)
		if err != nil {
			s.logger.Error("search knowledge for page", "error", err)
			data.SearchError = "Failed to search knowledge. Please try again."
		} else {
			for i := range items {
				data.Items = append(data.Items, &items[i])
			}
		}
	} else {
		limit := defaultKnowledgeLimit
		if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
			parsedLimit, err := strconv.Atoi(rawLimit)
			if err != nil || parsedLimit <= 0 {
				http.Error(w, "invalid limit", http.StatusBadRequest)
				return
			}
			limit = parsedLimit
			if limit > maxKnowledgeLimit {
				limit = maxKnowledgeLimit
			}
		}

		var beforeID int64
		if rawBeforeID := r.URL.Query().Get("before_id"); rawBeforeID != "" {
			parsedBeforeID, err := strconv.ParseInt(rawBeforeID, 10, 64)
			if err != nil || parsedBeforeID <= 0 {
				http.Error(w, "invalid before_id", http.StatusBadRequest)
				return
			}
			beforeID = parsedBeforeID
		}

		items, err := s.knowledge.ListKnowledgeItems(r.Context(), beforeID, limit+1)
		if err != nil {
			s.logger.Error("get knowledge items", "error", err, "before_id", beforeID, "limit", limit)
			http.Error(w, "failed to load knowledge items", http.StatusInternalServerError)
			return
		}

		hasMore := len(items) > limit
		if hasMore {
			items = items[:limit]
		}

		var nextBeforeID int64
		if hasMore && len(items) > 0 {
			nextBeforeID = items[len(items)-1].ID
		}

		data = KnowledgePageData{
			Title:        "Knowledge",
			Items:        items,
			Limit:        limit,
			BeforeID:     beforeID,
			HasMore:      hasMore,
			NextBeforeID: nextBeforeID,
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if data.SearchError != "" {
		w.WriteHeader(http.StatusInternalServerError)
	}
	if err := s.templates.ExecuteTemplate(w, "knowledge.html", data); err != nil {
		s.logger.Error("render knowledge page", "error", err)
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

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(messageResponse{
		JobID:  jobID,
		Status: domain.JobStatusPending,
	}); err != nil {
		s.logger.Error("write message response", "error", err, "job_id", jobID)
	}
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || jobID <= 0 {
		http.Error(w, "invalid job id", http.StatusBadRequest)
		return
	}

	job, err := s.jobs.GetJobByID(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "job not found", http.StatusNotFound)
			return
		}
		s.logger.Error("get job", "error", err, "job_id", jobID)
		http.Error(w, "failed to get job", http.StatusInternalServerError)
		return
	}

	sourceMessageID := int64(0)
	if job.SourceMessage != nil {
		sourceMessageID = job.SourceMessage.ID
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(jobResponse{
		ID:              job.ID,
		JobType:         job.JobType,
		SourceMessageID: sourceMessageID,
		Status:          job.Status,
		Attempts:        job.Attempts,
		Error:           job.Error,
		StartedAt:       job.StartedAt,
		FinishedAt:      job.FinishedAt,
		CreatedAt:       job.CreatedAt,
		UpdatedAt:       job.UpdatedAt,
	}); err != nil {
		s.logger.Error("write job response", "error", err, "job_id", jobID)
	}
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

	http.Redirect(w, r, "/topics/manage?created=1", http.StatusSeeOther)
}

func (s *Server) handleUpdateTopicDescription(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		http.Error(w, "slug is required", http.StatusBadRequest)
		return
	}
	slug, err := url.PathUnescape(slug)
	if err != nil {
		s.logger.Error("decode topic slug", "error", err, "slug", slug)
		http.Error(w, "invalid slug", http.StatusBadRequest)
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
	slug, err := url.PathUnescape(slug)
	if err != nil {
		s.logger.Error("decode topic slug", "error", err, "slug", slug)
		http.Error(w, "invalid slug", http.StatusBadRequest)
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

// Операция UpdateKnowledge не является потокобезопасной в смысле защиты от потери
// изменений при сохранении устаревшей полной формы.
// Например, две вкладки открыли запись с title="Go". Первая сохранила
// title="Изучение Go", а вторая изменила body и отправила форму
// со старым title="Go", перезаписав заголовок из первой вкладки.
// Принято поведение: последнее успешное сохранение определяет содержимое.
// Это логическая потеря обновления, а не data race в Go;
// запись, связи топиков и индекс сохраняются атомарно в транзакции.

type updateKnowledgeRequest struct {
	Title    string          `json:"title" validate:"required"`
	Body     string          `json:"body" validate:"required"`
	Topics   []int64         `json:"topics" validate:"required,dive,gt=0"`
	DataJSON json.RawMessage `json:"data_json" validate:"required"`
}

func (s *Server) handleUpdateKnowledge(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid knowledge id", http.StatusBadRequest)
		return
	}

	var input updateKnowledgeRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		http.Error(w, "invalid knowledge JSON", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "expected a single JSON object", http.StatusBadRequest)
		return
	}
	if err := valid.Vl.Struct(input); err != nil || strings.TrimSpace(input.Title) == "" || strings.TrimSpace(input.Body) == "" || !json.Valid(input.DataJSON) {
		http.Error(w, "title, body, topics and data_json must be valid and present", http.StatusBadRequest)
		return
	}

	topics := make([]*domain.Topic, 0, len(input.Topics))
	for _, topicID := range input.Topics {
		topics = append(topics, &domain.Topic{ID: topicID})
	}
	if strings.TrimSpace(string(input.DataJSON)) == "null" {
		input.DataJSON = nil
	}
	item := &domain.KnowledgeItem{Body: &domain.KnowledgeItemBody{
		Title: input.Title, Body: input.Body, Topics: topics, DataJSON: input.DataJSON,
	}}
	if err := s.topics.UpdateKnowledge(r.Context(), id, item); err != nil {
		switch {
		case errors.Is(err, domain.ErrKnowledgeNotFound):
			http.Error(w, "knowledge item not found", http.StatusNotFound)
		case errors.Is(err, domain.ErrTopicNotFound):
			http.Error(w, "selected topic does not exist", http.StatusBadRequest)
		default:
			s.logger.Error("update knowledge item", "error", err, "knowledge_id", id)
			http.Error(w, "failed to update knowledge item", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/knowledge/{id}
func (s *Server) handleGetKnowledge(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid knowledge id", http.StatusBadRequest)
		return
	}

	item, err := s.knowledgeService.GetEntry(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "knowledge item not found", http.StatusNotFound)
			return
		}
		s.logger.Error("get knowledge item", "error", err, "knowledge_id", id)
		http.Error(w, "failed to get knowledge item", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(item); err != nil {
		s.logger.Error("write knowledge response", "error", err, "knowledge_id", id)
	}
}

// GET /api/search?query=...&limit=5
func (s *Server) handleSearchKnowledge(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if strings.TrimSpace(query) == "" {
		http.Error(w, "query must be non empty", http.StatusBadRequest)
		return
	}

	limit := defaultSearchKnowledgeLimit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit <= 0 {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		limit = parsedLimit
	}

	items, err := s.knowledgeService.SearchKnowledge(r.Context(), query, limit)
	if err != nil {
		s.logger.Error("search knowledge", "error", err, "limit", limit)
		http.Error(w, "failed to search knowledge", http.StatusInternalServerError)
		return
	}
	if items == nil {
		items = []domain.KnowledgeItem{}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(items); err != nil {
		s.logger.Error("write knowledge search response", "error", err)
	}
}
