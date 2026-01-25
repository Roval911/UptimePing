package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"UptimePingPlatform/pkg/logger"

	"github.com/stretchr/testify/assert"
)

// TestLoggingMiddleware тестирует logging middleware
func TestLoggingMiddleware(t *testing.T) {
	// Arrange
	logger, _ := logger.NewLogger("test", "debug", "test-service", false)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := LoggingMiddleware(logger)(handler)

	// Act
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

// TestLoggingMiddleware_DifferentMethods тестирует разные HTTP методы
func TestLoggingMiddleware_DifferentMethods(t *testing.T) {
	// Arrange
	logger, _ := logger.NewLogger("test", "debug", "test-service", false)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := LoggingMiddleware(logger)(handler)
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run("method_"+method, func(t *testing.T) {
			// Act
			req := httptest.NewRequest(method, "/test", nil)
			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)

			// Assert
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

// TestLoggingMiddleware_ErrorStatus тестирует логирование ошибок
func TestLoggingMiddleware_ErrorStatus(t *testing.T) {
	// Arrange
	logger, _ := logger.NewLogger("test", "debug", "test-service", false)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	})

	middleware := LoggingMiddleware(logger)(handler)

	// Act
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "Internal Server Error", w.Body.String())
}

// TestLoggingMiddleware_RequestHeaders тестирует заголовки запроса
func TestLoggingMiddleware_RequestHeaders(t *testing.T) {
	// Arrange
	logger, _ := logger.NewLogger("test", "debug", "test-service", false)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := LoggingMiddleware(logger)(handler)

	// Act
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}
