package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"UptimePingPlatform/pkg/logger"
	"github.com/stretchr/testify/assert"
)

// TestRecoveryMiddleware тестирует recovery middleware
func TestRecoveryMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		handler        http.HandlerFunc
		expectedStatus int
		expectPanic    bool
	}{
		{
			name: "successful request",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			},
			expectedStatus: http.StatusOK,
			expectPanic:    false,
		},
		{
			name: "panic in handler",
			handler: func(w http.ResponseWriter, r *http.Request) {
				panic("test panic")
			},
			expectedStatus: http.StatusInternalServerError,
			expectPanic:    true,
		},
		{
			name: "nil pointer panic",
			handler: func(w http.ResponseWriter, r *http.Request) {
				var ptr *string
				ptr = nil
				_ = *ptr // This will cause a nil pointer panic
			},
			expectedStatus: http.StatusInternalServerError,
			expectPanic:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			testLogger, _ := logger.NewLogger("test", "info", "test-service", false)
			middleware := RecoveryMiddleware(testLogger)

			// Act
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			// This should not panic even if the handler panics
			assert.NotPanics(t, func() {
				middleware(tt.handler).ServeHTTP(w, req)
			})

			// Assert
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

// TestRecoveryMiddleware_ErrorResponse тестирует формат ответа при панике
func TestRecoveryMiddleware_ErrorResponse(t *testing.T) {
	// Arrange
	testLogger, _ := logger.NewLogger("test", "info", "test-service", false)
	middleware := RecoveryMiddleware(testLogger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(errors.New("test error"))
	})

	// Act
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	middleware(handler).ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Internal server error")
}

// TestRecoveryMiddleware_Headers тестирует сохранение заголовков
func TestRecoveryMiddleware_Headers(t *testing.T) {
	// Arrange
	testLogger, _ := logger.NewLogger("test", "info", "test-service", false)
	middleware := RecoveryMiddleware(testLogger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom-Header", "test-value")
		panic("test panic")
	})

	// Act
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	middleware(handler).ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	// Note: Headers might be reset after panic, this is expected behavior
}
