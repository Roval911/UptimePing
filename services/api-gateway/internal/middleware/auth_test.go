package middleware

import (
	"UptimePingPlatform/pkg/errors"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockAuthClient мок для AuthClient
type MockAuthClient struct {
	validToken  bool
	validAPIKey bool
}

func (m *MockAuthClient) ValidateToken(ctx context.Context, token string) (*TokenClaims, error) {
	if m.validToken && token == "valid-jwt-token" {
		return &TokenClaims{
			UserID:   "user123",
			TenantID: "tenant456",
			IsAdmin:  false,
		}, nil
	}
	return nil, errors.New(errors.ErrUnauthorized, "Invalid token")
}

func (m *MockAuthClient) ValidateAPIKey(ctx context.Context, key, secret string) (*APIKeyClaims, error) {
	if m.validAPIKey && key == "valid-api-key" && secret == "valid-secret" {
		return &APIKeyClaims{
			TenantID: "tenant456",
			KeyID:    "key789",
		}, nil
	}
	return nil, errors.New(errors.ErrUnauthorized, "Invalid API key")
}

// TestAuthMiddleware тестирует authentication middleware
func TestAuthMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		expectUserID   bool
	}{
		{
			name:           "valid Bearer token",
			authHeader:     "Bearer valid-jwt-token",
			expectedStatus: http.StatusOK,
			expectUserID:   true,
		},
		{
			name:           "missing auth header",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectUserID:   false,
		},
		{
			name:           "invalid Bearer token",
			authHeader:     "Bearer invalid-token",
			expectedStatus: http.StatusUnauthorized,
			expectUserID:   false,
		},
		{
			name:           "invalid auth header format",
			authHeader:     "InvalidFormat token",
			expectedStatus: http.StatusUnauthorized,
			expectUserID:   false,
		},
		{
			name:           "malformed token",
			authHeader:     "Bearer malformed.token.here",
			expectedStatus: http.StatusUnauthorized,
			expectUserID:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			authClient := &MockAuthClient{validToken: tt.authHeader == "Bearer valid-jwt-token"}
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Проверяем что user_id был добавлен в контекст
				userID := r.Context().Value("user_id")
				if tt.expectUserID {
					assert.NotNil(t, userID)
				} else {
					assert.Nil(t, userID)
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			})

			middleware := AuthMiddleware(authClient)(handler)

			// Act
			req := httptest.NewRequest("GET", "/protected", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

// TestAuthMiddleware_APIKeyAuth тестирует аутентификацию через API ключ
func TestAuthMiddleware_APIKeyAuth(t *testing.T) {
	// Arrange
	authClient := &MockAuthClient{validAPIKey: true}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Context().Value("tenant_id")
		assert.NotNil(t, tenantID)
		assert.Equal(t, "tenant456", tenantID)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("API Key auth successful"))
	})

	middleware := AuthMiddleware(authClient)(handler)

	// Act
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "APIKey valid-api-key:valid-secret")
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestAuthMiddleware_ContextValues тестирует передачу значений в контекст
func TestAuthMiddleware_ContextValues(t *testing.T) {
	// Arrange
	authClient := &MockAuthClient{validToken: true}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value("user_id")
		tenantID := r.Context().Value("tenant_id")
		assert.NotNil(t, userID)
		assert.Equal(t, "user123", userID)
		assert.NotNil(t, tenantID)
		assert.Equal(t, "tenant456", tenantID)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Context values test successful"))
	})

	middleware := AuthMiddleware(authClient)(handler)

	// Act
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer valid-jwt-token")
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestAuthMiddleware_ErrorResponse тестирует формат ответа при ошибке
func TestAuthMiddleware_ErrorResponse(t *testing.T) {
	// Arrange
	authClient := &MockAuthClient{validToken: false}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Should not reach here"))
	})

	middleware := AuthMiddleware(authClient)(handler)

	// Act
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestAuthMiddleware_DifferentMethods тестирует разные HTTP методы
func TestAuthMiddleware_DifferentMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run("method "+method, func(t *testing.T) {
			// Arrange
			authClient := &MockAuthClient{validToken: true}
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			})

			middleware := AuthMiddleware(authClient)(handler)

			// Act
			req := httptest.NewRequest(method, "/protected", nil)
			req.Header.Set("Authorization", "Bearer valid-jwt-token")
			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)

			// Assert
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}
