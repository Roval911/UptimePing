package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// SimpleMockRateLimiter простой мок для rate limiter
type SimpleMockRateLimiter struct {
	shouldAllow bool
}

func (m *SimpleMockRateLimiter) CheckRateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	return m.shouldAllow, nil
}

// TestRateLimitMiddleware_Allowed тестирует разрешенный запрос
func TestRateLimitMiddleware_Allowed(t *testing.T) {
	// Arrange
	limiter := &SimpleMockRateLimiter{shouldAllow: true}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := RateLimitMiddleware(limiter, 10, time.Minute)(handler)

	// Act
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

// TestRateLimitMiddleware_RateLimited тестирует заблокированный запрос
func TestRateLimitMiddleware_RateLimited(t *testing.T) {
	// Arrange
	limiter := &SimpleMockRateLimiter{shouldAllow: false}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Should not reach here"))
	})

	middleware := RateLimitMiddleware(limiter, 10, time.Minute)(handler)

	// Act
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Contains(t, w.Body.String(), "TOO_MANY_REQUESTS")
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

// TestRateLimitMiddleware_Error тестирует ошибку rate limiter
type ErrorMockRateLimiter struct{}

func (m *ErrorMockRateLimiter) CheckRateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	return false, assert.AnError
}

func TestRateLimitMiddleware_Error(t *testing.T) {
	// Arrange
	limiter := &ErrorMockRateLimiter{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Should not reach here"))
	})

	middleware := RateLimitMiddleware(limiter, 10, time.Minute)(handler)

	// Act
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Rate limit service unavailable")
}

// TestRateLimitMiddleware_DifferentMethods тестирует разные HTTP методы
func TestRateLimitMiddleware_DifferentMethods(t *testing.T) {
	// Arrange
	limiter := &SimpleMockRateLimiter{shouldAllow: true}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := RateLimitMiddleware(limiter, 10, time.Minute)(handler)

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

// TestRateLimitMiddleware_IPExtraction тестирует извлечение IP
func TestRateLimitMiddleware_IPExtraction(t *testing.T) {
	// Arrange
	limiter := &SimpleMockRateLimiter{shouldAllow: true}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := RateLimitMiddleware(limiter, 10, time.Minute)(handler)

	// Act & Assert - X-Forwarded-For
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1")
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Act & Assert - X-Real-IP
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Real-IP", "192.168.1.2")
	w = httptest.NewRecorder()
	middleware.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Act & Assert - RemoteAddr (по умолчанию)
	req = httptest.NewRequest("GET", "/test", nil)
	w = httptest.NewRecorder()
	middleware.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
