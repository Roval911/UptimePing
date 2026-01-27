package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"UptimePingPlatform/pkg/logger"
	"github.com/stretchr/testify/assert"
)

// TestCORSMiddleware тестирует CORS middleware
func TestCORSMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		origin         string
		allowedOrigins []string
		expectedStatus int
		expectCORS     bool
	}{
		{
			name:           "GET request with allowed origin",
			method:         "GET",
			origin:         "https://example.com",
			allowedOrigins: []string{"https://example.com", "https://api.example.com"},
			expectedStatus: http.StatusOK,
			expectCORS:     true,
		},
		{
			name:           "GET request with disallowed origin",
			method:         "GET",
			origin:         "https://malicious.com",
			allowedOrigins: []string{"https://example.com"},
			expectedStatus: http.StatusOK,
			expectCORS:     false,
		},
		{
			name:           "GET request with wildcard origin",
			method:         "GET",
			origin:         "https://any.com",
			allowedOrigins: []string{"*"},
			expectedStatus: http.StatusOK,
			expectCORS:     true,
		},
		{
			name:           "OPTIONS request",
			method:         "OPTIONS",
			origin:         "https://example.com",
			allowedOrigins: []string{"https://example.com"},
			expectedStatus: http.StatusOK,
			expectCORS:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			})

			// Создаем тестовый logger
			testLogger, _ := logger.NewLogger("test", "info", "test-service", false)
			middleware := CORSMiddleware(tt.allowedOrigins, testLogger)(handler)

			// Act
			req := httptest.NewRequest(tt.method, "/test", nil)
			req.Header.Set("Origin", tt.origin)
			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectCORS {
				assert.Equal(t, tt.origin, w.Header().Get("Access-Control-Allow-Origin"))
			} else {
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

// TestCORSMiddleware_NoOrigin тестирует запрос без Origin
func TestCORSMiddleware_NoOrigin(t *testing.T) {
	// Arrange
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Создаем тестовый logger
	testLogger, _ := logger.NewLogger("test", "info", "test-service", false)
	middleware := CORSMiddleware([]string{"https://example.com"}, testLogger)(handler)

	// Act
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	// Если origin пустой, то CORS заголовок не устанавливается
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

// TestCORSMiddleware_Headers тестирует установку CORS headers
func TestCORSMiddleware_Headers(t *testing.T) {
	// Arrange
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Создаем тестовый logger
	testLogger, _ := logger.NewLogger("test", "info", "test-service", false)

	middleware := CORSMiddleware([]string{"*"}, testLogger)(handler)

	// Act
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
	// Проверяем что заголовок содержит ожидаемые значения
	headers := w.Header().Get("Access-Control-Allow-Headers")
	assert.Contains(t, headers, "Content-Type")
	assert.Contains(t, headers, "Authorization")
}
