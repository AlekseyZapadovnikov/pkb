package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"

	codexadapter "pkb/internal/adapters/codex"
	storage "pkb/internal/adapters/db"
	"pkb/internal/adapters/mcp"
	"pkb/internal/adapters/web"
	"pkb/internal/usecase/service"
	"pkb/internal/usecase/service/config"
)

// App holds the application configuration, logger, and HTTP server.
type App struct {
	cfg        config.Config
	logger     *slog.Logger
	server     *http.Server
	database   *sqlx.DB
	classifier *service.Classifier
}

func InitLogger(dataDir string) (*os.File, error) {
	if err := ensureDir(dataDir); err != nil {
		return nil, fmt.Errorf("ensure log directory: %w", err)
	}

	logFile, err := os.OpenFile(filepath.Join(dataDir, "pkb.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
	return logFile, nil
}

// New prepares the application directories and initializes the HTTP server.
func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	if logger == nil {
		logger = slog.Default()
	}

	if err := ensureDir(cfg.DataDir); err != nil {
		return nil, fmt.Errorf("ensure data dir: %w", err)
	}
	if err := ensureDir(filepath.Dir(cfg.SQLitePath)); err != nil {
		return nil, fmt.Errorf("ensure sqlite dir: %w", err)
	}

	setupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	database, err := storage.Open(setupCtx, cfg.SQLitePath, cfg.MemoryPaths)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	if err := storage.Migrate(setupCtx, database); err != nil {
		if closeErr := database.Close(); closeErr != nil {
			logger.Warn("close sqlite database after migration error", "error", closeErr)
		}
		return nil, fmt.Errorf("migrate sqlite database: %w", err)
	}

	repository := storage.NewRepository(database)
	transactor := storage.NewTransactor(database)

	var modelClient service.ModelClient = codexadapter.NewClient(cfg.ClassifierProvider)
	classifier := service.NewClassifier(repository, modelClient)
	messages := service.NewMessanger(logger, repository, transactor, classifier)
	topics := service.NewEntityManager(repository)

	knowledge := service.NewKnowledgeService(repository)
	mcpServer := mcp.NewServer(knowledge)
	webServer, err := web.NewServer(logger, messages, topics, repository, repository, knowledge)
	if err != nil {
		if closeErr := database.Close(); closeErr != nil {
			logger.Warn("close sqlite database after web server error", "error", closeErr)
		}
		return nil, fmt.Errorf("create web server: %w", err)
	}

	return &App{
		cfg:        cfg,
		logger:     logger,
		database:   database,
		classifier: classifier,
		server: &http.Server{
			Addr:              cfg.Addr,
			Handler:           webServer.Routes(mcpServer.Handler()),
			ReadHeaderTimeout: 5 * time.Second,
		},
	}, nil
}

// Run starts the HTTP server and shuts it down gracefully when the context is canceled.
func (a *App) Run(ctx context.Context) error {
	defer func() {
		if a.database == nil {
			return
		}
		if err := a.database.Close(); err != nil {
			a.logger.Warn("close sqlite database", "error", err)
		}
	}()

	errCh := make(chan error, 1)

	go a.classifier.ProcessJobWorker(ctx)

	go func() {
		a.logger.Info("starting http server", "addr", a.cfg.Addr, "sqlite_path", a.cfg.SQLitePath)
		errCh <- a.server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.ShutdownTimeout)
		defer cancel()

		a.logger.Info("shutting down http server")
		if err := a.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown http server: %w", err)
		}

		if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server stopped: %w", err)
		}
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("listen and serve: %w", err)
	}
}

// ensureDir creates a directory if the path identifies an actual folder.
func ensureDir(path string) error {
	if path == "" || path == "." {
		return nil
	}
	return os.MkdirAll(path, 0o755)
}
