package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRoutesHealthz проверяет ответ endpoint'а состояния сервера.
func TestRoutesHealthz(t *testing.T) {
	server, err := NewServer(ServerConfig{}, nil, nil)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /healthz status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"status":"ok"}` {
		t.Fatalf("GET /healthz body = %q", body)
	}
}

// TestRoutesHome проверяет рендеринг главной страницы.
func TestRoutesHome(t *testing.T) {
	server, err := NewServer(ServerConfig{}, nil, nil)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "Personal KB") {
		t.Fatalf("GET / body does not contain app title")
	}
	if strings.Contains(rec.Body.String(), "SQLite path") {
		t.Fatalf("GET / body contains storage details")
	}
}

// TestRoutesRejectUnsupportedMethod проверяет, что chi отклоняет неподдержанный метод.
func TestRoutesRejectUnsupportedMethod(t *testing.T) {
	server, err := NewServer(ServerConfig{}, nil, nil)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /healthz status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
