package http

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"UptimePingPlatform/pkg/logger"
	grpcBase "UptimePingPlatform/pkg/grpc"
)

// TestIsAuthenticated тестирует функцию аутентификации
func TestIsAuthenticated(t *testing.T) {
	log, _ := logger.NewLogger("test", "dev", "debug", false)
	baseHandler := grpcBase.NewBaseHandler(log)
	
	handler := &Handler{
		baseHandler: baseHandler,
	}

	tests := []struct {
		name           string
		headers        map[string]string
		expectedResult bool
	}{
		{
			name: "Missing Authorization header",
			headers: map[string]string{},
			expectedResult: false,
		},
		{
			name: "Valid JWT Bearer token",
			headers: map[string]string{
				"Authorization": "Bearer header.payload.signature",
			},
			expectedResult: true,
		},
		{
			name: "Invalid JWT format - missing parts",
			headers: map[string]string{
				"Authorization": "Bearer invalid_token",
			},
			expectedResult: false,
		},
		{
			name: "Empty Bearer token",
			headers: map[string]string{
				"Authorization": "Bearer ",
			},
			expectedResult: false,
		},
		{
			name: "Valid API key in Authorization header",
			headers: map[string]string{
				"Authorization": "Api-Key 1234567890123456",
			},
			expectedResult: true,
		},
		{
			name: "Too short API key in Authorization header",
			headers: map[string]string{
				"Authorization": "Api-Key short",
			},
			expectedResult: false,
		},
		{
			name: "Empty API key in Authorization header",
			headers: map[string]string{
				"Authorization": "Api-Key ",
			},
			expectedResult: false,
		},
		{
			name: "Valid API key in X-API-Key header",
			headers: map[string]string{
				"X-API-Key": "1234567890123456",
			},
			expectedResult: true,
		},
		{
			name: "Too short API key in X-API-Key header",
			headers: map[string]string{
				"X-API-Key": "short",
			},
			expectedResult: false,
		},
		{
			name: "Unsupported auth format",
			headers: map[string]string{
				"Authorization": "Basic dGVzdA==",
			},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			
			// Устанавливаем заголовки
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			result := handler.isAuthenticated(req)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
