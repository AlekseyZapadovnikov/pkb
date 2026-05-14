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

	"pkb/internal/config"
	"pkb/internal/web"
)

// App объединяет конфигурацию, логгер и HTTP-сервер приложения.
type App struct {
	cfg    config.Config
	logger *slog.Logger
	server *http.Server
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

	webServer, err := web.NewServer(web.ServerConfig{}, logger, nil)
	if err != nil {
		return nil, fmt.Errorf("create web server: %w", err)
	}

	return &App{
		cfg:    cfg,
		logger: logger,
		server: &http.Server{
			Addr:              cfg.Addr,
			Handler:           webServer.Routes(),
			ReadHeaderTimeout: 5 * time.Second,
		},
	}, nil
}

// Run запускает HTTP-сервер и корректно останавливает его при отмене контекста.
func (a *App) Run(ctx context.Context) error {
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
