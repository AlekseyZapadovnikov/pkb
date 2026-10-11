package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"pkb/internal/app"
	"pkb/internal/usecase/service/config"
)

// main loads the configuration, initializes the application, and starts the HTTP server.
func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	logFile, err := app.InitLogger(cfg.DataDir)
	if err != nil {
		slog.Error("initialize logger", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := logFile.Close(); err != nil {
			fmt.Fprintln(os.Stderr, "close log file:", err)
		}
	}()
	slog.Debug("debug message enabled")

	application, err := app.New(cfg, slog.Default())
	if err != nil {
		slog.Error("create app", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := application.Run(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("run app", "error", err)
		os.Exit(1)
	}
}
