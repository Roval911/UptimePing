package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestNewMetrics проверяет создание системы метрик
func TestNewMetrics(t *testing.T) {
	m := NewMetrics("test-service")
	
	if m == nil {
		t.Fatal("Expected metrics, got nil")
	}
	
	if m.RequestCount == nil {
		t.Error("Expected RequestCount, got nil")
	}
	
	if m.RequestDuration == nil {
		t.Error("Expected RequestDuration, got nil")
	}
	
	if m.ErrorsCount == nil {
		t.Error("Expected ErrorsCount, got nil")
	}
	
	if m.Tracer == nil {
		t.Error("Expected Tracer, got nil")
	}
}

// TestGetHandler проверяет обработчик метрик
func TestGetHandler(t *testing.T) {
	m := NewMetrics("test-service")
	handler := m.GetHandler()
	
	// Создаем тестовый запрос
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	
	// Выполняем запрос
	handler.ServeHTTP(w, req)
	
	// Проверяем ответ
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}
	
	// Проверяем, что Content-Type содержит ожидаемое значение
	if !strings.HasPrefix(w.Header().Get("Content-Type"), "text/plain; version=0.0.4") {
		t.Errorf("Expected Content-Type to start with 'text/plain; version=0.0.4', got %s", w.Header().Get("Content-Type"))
	}
}

// TestMiddleware проверяет работу middleware
func TestMiddleware(t *testing.T) {
	m := NewMetrics("test-service")
	
	// Создаем тестовый обработчик
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	
	// Создаем тестовый запрос
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	
	// Выполняем запрос
	handler.ServeHTTP(w, req)
	
	// Проверяем ответ
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}
	
	if w.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got %s", w.Body.String())
	}
}

// TestMiddlewareWithError проверяет работу middleware с ошибкой
func TestMiddlewareWithError(t *testing.T) {
	m := NewMetrics("test-service")
	
	// Создаем тестовый обработчик, который возвращает ошибку
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	
	// Создаем тестовый запрос
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	
	// Выполняем запрос
	handler.ServeHTTP(w, req)
	
	// Проверяем ответ
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, w.Code)
	}
}