package web

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

type loggerContextKey struct{}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.With("method", r.Method, "path", r.URL.Path, "date", time.Now().Format(time.DateOnly))
		r = r.WithContext(context.WithValue(r.Context(), loggerContextKey{}, logger))
		logger.DebugContext(r.Context(), "HTTP request started")

		start := time.Now()
		writer := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(writer, r)
		status := writer.Status()
		if status == 0 {
			status = http.StatusOK
		}
		logger = logger.With("status", status, "duration", time.Since(start))

		switch {
		case status >= http.StatusInternalServerError:
			logger.ErrorContext(r.Context(), "HTTP request failed")
		case status >= http.StatusBadRequest:
			logger.WarnContext(r.Context(), "HTTP request rejected")
		default:
			logger.InfoContext(r.Context(), "HTTP request completed")
		}
	})
}
