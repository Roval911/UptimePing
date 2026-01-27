package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"UptimePingPlatform/pkg/health"
	"UptimePingPlatform/pkg/logger"
)

// TestReadyCheckEnhanced тестирует улучшенный ready check
func TestReadyCheckEnhanced(t *testing.T) {
	log, _ := logger.NewLogger("test", "dev", "debug", false)
	checker := health.NewSimpleHealthChecker("1.0.0")
	handler := NewHealthHandler(checker, log)

	tests := []struct {
		name           string
		expectedStatus int
		expectedFields []string
	}{
		{
			name:           "Ready check success",
			expectedStatus: http.StatusOK,
			expectedFields: []string{"status", "timestamp", "checks"},
		},
		{
			name:           "Ready check with database",
			expectedStatus: http.StatusOK,
			expectedFields: []string{"checks"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/ready", nil)
			w := httptest.NewRecorder()

			handler.ReadyCheck(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			// Проверяем наличие обязательных полей
			for _, field := range tt.expectedFields {
				assert.Contains(t, response, field)
			}

			// Проверяем структуру checks
			if checks, ok := response["checks"].(map[string]interface{}); ok {
				assert.Contains(t, checks, "database")
				assert.Contains(t, checks, "redis")
				assert.Contains(t, checks, "services")
				
				// Проверяем структуру вложенных полей
				if db, ok := checks["database"].(map[string]interface{}); ok {
					assert.Contains(t, db, "ready")
					assert.Contains(t, db, "message")
					assert.Contains(t, db, "latency")
				}
				
				if redis, ok := checks["redis"].(map[string]interface{}); ok {
					assert.Contains(t, redis, "ready")
					assert.Contains(t, redis, "message")
					assert.Contains(t, redis, "latency")
				}
				
				if services, ok := checks["services"].(map[string]interface{}); ok {
					assert.Contains(t, services, "ready")
					assert.Contains(t, services, "message")
					assert.Contains(t, services, "details")
				}
			}
		})
	}
}

// TestLiveCheckEnhanced тестирует улучшенный live check
func TestLiveCheckEnhanced(t *testing.T) {
	log, _ := logger.NewLogger("test", "dev", "debug", false)
	checker := health.NewSimpleHealthChecker("1.0.0")
	handler := NewHealthHandler(checker, log)

	tests := []struct {
		name           string
		expectedStatus int
		expectedFields []string
	}{
		{
			name:           "Live check success",
			expectedStatus: http.StatusOK,
			expectedFields: []string{"status", "timestamp", "uptime", "version", "checks"},
		},
		{
			name:           "Live check with system info",
			expectedStatus: http.StatusOK,
			expectedFields: []string{"checks"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/live", nil)
			w := httptest.NewRecorder()

			handler.LiveCheck(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			// Проверяем наличие обязательных полей
			for _, field := range tt.expectedFields {
				assert.Contains(t, response, field)
			}

			// Проверяем структуру checks
			if checks, ok := response["checks"].(map[string]interface{}); ok {
				assert.Contains(t, checks, "memory")
				assert.Contains(t, checks, "goroutines")

				// Проверяем структуру memory
				if memory, ok := checks["memory"].(map[string]interface{}); ok {
					assert.Contains(t, memory, "used_mb")
					assert.Contains(t, memory, "total_mb")
					assert.Contains(t, memory, "percent")
					assert.Contains(t, memory, "status")
				}

				// Проверяем структуру goroutines
				if goroutines, ok := checks["goroutines"].(map[string]interface{}); ok {
					assert.Contains(t, goroutines, "count")
					assert.Contains(t, goroutines, "status")
				}
			}
		})
	}
}

// TestHealthCheckMethods тестирует все health check методы
func TestHealthCheckMethods(t *testing.T) {
	log, _ := logger.NewLogger("test", "dev", "debug", false)
	checker := health.NewSimpleHealthChecker("1.0.0")
	handler := NewHealthHandler(checker, log)

	tests := []struct {
		name           string
		method         string
		path           string
		handlerFunc    func(http.ResponseWriter, *http.Request)
		expectedStatus int
	}{
		{
			name:           "Health check GET",
			method:         "GET",
			path:           "/health",
			handlerFunc:    handler.HealthCheck,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Health check POST",
			method:         "POST",
			path:           "/health",
			handlerFunc:    handler.HealthCheck,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "Ready check GET",
			method:         "GET",
			path:           "/ready",
			handlerFunc:    handler.ReadyCheck,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Ready check POST",
			method:         "POST",
			path:           "/ready",
			handlerFunc:    handler.ReadyCheck,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "Live check GET",
			method:         "GET",
			path:           "/live",
			handlerFunc:    handler.LiveCheck,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Live check POST",
			method:         "POST",
			path:           "/live",
			handlerFunc:    handler.LiveCheck,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			tt.handlerFunc(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			// Для успешных ответов проверяем Content-Type
			if tt.expectedStatus == http.StatusOK {
				assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
			}
		})
	}
}

// TestHealthCheckResponseFormat тестирует формат ответов
func TestHealthCheckResponseFormat(t *testing.T) {
	log, _ := logger.NewLogger("test", "dev", "debug", false)
	checker := health.NewSimpleHealthChecker("1.0.0")
	handler := NewHealthHandler(checker, log)

	t.Run("Health check response format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		handler.HealthCheck(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Contains(t, response, "status")
		assert.Contains(t, response, "timestamp")
		// SimpleHealthChecker не возвращает services, только status, timestamp и version
		assert.Contains(t, response, "version")
	})

	t.Run("Ready check response format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/ready", nil)
		w := httptest.NewRecorder()

		handler.ReadyCheck(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Contains(t, response, "status")
		assert.Contains(t, response, "timestamp")
		assert.Contains(t, response, "checks")
	})

	t.Run("Live check response format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/live", nil)
		w := httptest.NewRecorder()

		handler.LiveCheck(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Contains(t, response, "status")
		assert.Contains(t, response, "timestamp")
		assert.Contains(t, response, "uptime")
		assert.Contains(t, response, "version")
		assert.Contains(t, response, "checks")
	})
}
