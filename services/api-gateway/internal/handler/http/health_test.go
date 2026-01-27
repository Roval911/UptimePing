package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"UptimePingPlatform/pkg/health"
	"UptimePingPlatform/pkg/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHealthHandler тестирует health endpoint
func TestHealthHandler(t *testing.T) {
	// Arrange
	checker := &health.SimpleHealthChecker{}
	testLogger, _ := logger.NewLogger("test", "info", "test-service", false)
	handler := NewHealthHandler(checker, testLogger)

	// Act
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler.HealthCheck(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response["status"])
	assert.Contains(t, response, "timestamp")
}

// TestHealthHandler_Methods тестирует разные HTTP методы
func TestHealthHandler_Methods(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		expectedStatus int
	}{
		{
			name:           "GET method",
			method:         "GET",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST method",
			method:         "POST",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "PUT method",
			method:         "PUT",
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			checker := &health.SimpleHealthChecker{}
			testLogger, _ := logger.NewLogger("test", "info", "test-service", false)
			handler := NewHealthHandler(checker, testLogger)

			// Act
			req := httptest.NewRequest(tt.method, "/health", nil)
			w := httptest.NewRecorder()
			handler.HealthCheck(w, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

// TestHealthHandler_ResponseFormat тестирует формат ответа
func TestHealthHandler_ResponseFormat(t *testing.T) {
	// Arrange
	checker := &health.SimpleHealthChecker{}
	testLogger, _ := logger.NewLogger("test", "info", "test-service", false)
	handler := NewHealthHandler(checker, testLogger)

	// Act
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler.HealthCheck(w, req)

	// Assert
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Проверяем обязательные поля
	assert.Contains(t, response, "status")
	assert.Contains(t, response, "timestamp")

	// SimpleHealthChecker не включает version и services, проверяем только базовые поля
	assert.IsType(t, "healthy", response["status"])
	assert.IsType(t, "string", response["timestamp"])
}
