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

	storage "pkb/internal/db"
	"pkb/internal/usecase/service"
	"pkb/internal/usecase/service/config"
	"pkb/internal/web"
)

// App объединяет конфигурацию, логгер и HTTP-сервер приложения.
type App struct {
	cfg      config.Config
	logger   *slog.Logger
	server   *http.Server
	database *sqlx.DB
}

// New подготавливает директории приложения и собирает HTTP-сервер.
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

	database, err := storage.Open(setupCtx, cfg.SQLitePath)
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
	topics := service.NewTopicManager(repository)

	webServer, err := web.NewServer(web.ServerConfig{}, logger, nil, topics)
	if err != nil {
		if closeErr := database.Close(); closeErr != nil {
			logger.Warn("close sqlite database after web server error", "error", closeErr)
		}
		return nil, fmt.Errorf("create web server: %w", err)
	}

	return &App{
		cfg:      cfg,
		logger:   logger,
		database: database,
		server: &http.Server{
			Addr:              cfg.Addr,
			Handler:           webServer.Routes(),
			ReadHeaderTimeout: 5 * time.Second,
		},
	}, nil
}

// Run запускает HTTP-сервер и корректно останавливает его при отмене контекста.
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

// ensureDir создаёт директорию, если путь указывает на реальную папку.
func ensureDir(path string) error {
	if path == "" || path == "." {
		return nil
	}
	return os.MkdirAll(path, 0o755)
}
