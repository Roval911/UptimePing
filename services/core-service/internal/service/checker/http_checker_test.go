package checker

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"UptimePingPlatform/services/core-service/internal/domain"
)

func TestHTTPChecker_Execute_Success(t *testing.T) {
	// Создание checker'а
	checker := NewHTTPChecker(5000)
	
	// Создание тестовой задачи
	config := map[string]interface{}{
		"method":         "GET",
		"url":            "https://httpbin.org/status/200",
		"expected_status": float64(200),
		"timeout":        "5s",
	}
	
	task := domain.NewTask("check-1", "https://httpbin.org/status/200", "http", "exec-1", time.Now(), config)
	
	// Выполнение проверки
	result, err := checker.Execute(task)
	
	// Проверки
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "check-1", result.CheckID)
	assert.Equal(t, "exec-1", result.ExecutionID)
	assert.True(t, result.DurationMs > 0)
}

func TestHTTPChecker_Execute_InvalidConfig(t *testing.T) {
	// Создание checker'а
	checker := NewHTTPChecker(5000)
	
	// Создание задачи с невалидной конфигурацией
	config := map[string]interface{}{
		"method": "", // пустой метод
		"url":    "https://example.com",
	}
	
	task := domain.NewTask("check-1", "https://example.com", "http", "exec-1", time.Now(), config)
	
	// Выполнение проверки
	result, err := checker.Execute(task)
	
	// Проверки
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "config validation failed")
}

func TestHTTPChecker_GetType(t *testing.T) {
	checker := NewHTTPChecker(5000)
	assert.Equal(t, domain.TaskTypeHTTP, checker.GetType())
}

func TestHTTPChecker_SetTimeout(t *testing.T) {
	checker := NewHTTPChecker(1000)
	
	// Проверка начального таймаута
	assert.Equal(t, int64(1000), checker.GetTimeout())
	
	// Установка нового таймаута
	newTimeout := 5 * time.Second
	checker.SetTimeout(newTimeout)
	
	// Проверка изменения таймаута
	assert.Equal(t, int64(5000), checker.GetTimeout())
	assert.Equal(t, newTimeout, checker.GetClient().Timeout)
}

func TestHTTPChecker_ValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
		errorField  string
	}{
		{
			name: "valid config",
			config: map[string]interface{}{
				"method":         "GET",
				"url":            "https://example.com",
				"expected_status": float64(200),
				"timeout":        "5s",
			},
			expectError: false,
		},
		{
			name: "missing method",
			config: map[string]interface{}{
				"url":            "https://example.com",
				"expected_status": float64(200),
			},
			expectError: true,
			errorField:  "method",
		},
		{
			name: "invalid method",
			config: map[string]interface{}{
				"method":         "INVALID",
				"url":            "https://example.com",
				"expected_status": float64(200),
			},
			expectError: true,
			errorField:  "method",
		},
		{
			name: "missing url",
			config: map[string]interface{}{
				"method":         "GET",
				"expected_status": float64(200),
			},
			expectError: true,
			errorField:  "url",
		},
		{
			name: "invalid url scheme",
			config: map[string]interface{}{
				"method":         "GET",
				"url":            "ftp://example.com",
				"expected_status": float64(200),
			},
			expectError: true,
			errorField:  "url",
		},
		{
			name: "invalid status code",
			config: map[string]interface{}{
				"method":         "GET",
				"url":            "https://example.com",
				"expected_status": float64(999),
			},
			expectError: true,
			errorField:  "expected_status",
		},
		{
			name: "invalid timeout",
			config: map[string]interface{}{
				"method":         "GET",
				"url":            "https://example.com",
				"expected_status": float64(200),
				"timeout":        "invalid",
			},
			expectError: true,
			errorField:  "timeout",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewHTTPChecker(5000)
			err := checker.ValidateConfig(tt.config)
			
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorField != "" {
					assert.Contains(t, err.Error(), tt.errorField)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHTTPChecker_createHTTPRequest(t *testing.T) {
	checker := NewHTTPChecker(5000)
	
	tests := []struct {
		name     string
		config   *domain.HTTPConfig
		expected string
	}{
		{
			name: "GET request",
			config: &domain.HTTPConfig{
				Method: "GET",
				URL:    "https://example.com",
				Headers: map[string]string{"Accept": "application/json"},
			},
			expected: "GET",
		},
		{
			name: "POST request with body",
			config: &domain.HTTPConfig{
				Method: "POST",
				URL:    "https://example.com/api",
				Body:   `{"test": "data"}`,
				Headers: map[string]string{"Content-Type": "application/json"},
			},
			expected: "POST",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := checker.createHTTPRequest(tt.config)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, req.Method)
			assert.Equal(t, tt.config.URL, req.URL.String())
		})
	}
}

func TestHTTPChecker_validateJSONPath(t *testing.T) {
	checker := NewHTTPChecker(5000)
	
	tests := []struct {
		name        string
		body        string
		rule        map[string]interface{}
		expectError bool
	}{
		{
			name: "valid json path",
			body: `{"status": "ok", "data": {"value": 42}}`,
			rule: map[string]interface{}{
				"type":     "json_path",
				"path":     "$.status",
				"expected": "ok",
				"operator": "equals",
			},
			expectError: false,
		},
		{
			name: "invalid json",
			body: `invalid json`,
			rule: map[string]interface{}{
				"type":     "json_path",
				"path":     "$.status",
				"expected": "ok",
				"operator": "equals",
			},
			expectError: true,
		},
		{
			name: "invalid json path",
			body: `{"status": "ok"}`,
			rule: map[string]interface{}{
				"type":     "json_path",
				"path":     "$.invalid.path",
				"expected": "value",
				"operator": "equals",
			},
			expectError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := checker.validateJSONPath(tt.body, tt.rule)
			
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHTTPChecker_validateRegex(t *testing.T) {
	checker := NewHTTPChecker(5000)
	
	tests := []struct {
		name        string
		body        string
		rule        map[string]interface{}
		expectError bool
	}{
		{
			name: "regex match found",
			body: "Response: SUCCESS",
			rule: map[string]interface{}{
				"type":     "regex",
				"path":     `(?i)success`,
				"operator": "contains",
			},
			expectError: false,
		},
		{
			name: "regex match not found",
			body: "Response: FAILURE",
			rule: map[string]interface{}{
				"type":     "regex",
				"path":     `(?i)success`,
				"operator": "contains",
			},
			expectError: false,
		},
		{
			name: "invalid regex pattern",
			body: "test",
			rule: map[string]interface{}{
				"type":     "regex",
				"path":     `[invalid regex`,
				"operator": "contains",
			},
			expectError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := checker.validateRegex(tt.body, tt.rule)
			
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHTTPChecker_compareValues(t *testing.T) {
	checker := NewHTTPChecker(5000)
	
	tests := []struct {
		name     string
		actual   interface{}
		expected interface{}
		operator string
		result   bool
	}{
		{
			name:     "equals true",
			actual:   "test",
			expected: "test",
			operator: "equals",
			result:   true,
		},
		{
			name:     "equals false",
			actual:   "test",
			expected: "other",
			operator: "equals",
			result:   false,
		},
		{
			name:     "contains true",
			actual:   "hello world",
			expected: "world",
			operator: "contains",
			result:   true,
		},
		{
			name:     "contains false",
			actual:   "hello",
			expected: "world",
			operator: "contains",
			result:   false,
		},
		{
			name:     "not_empty true",
			actual:   "value",
			expected: nil,
			operator: "not_empty",
			result:   true,
		},
		{
			name:     "not_empty false",
			actual:   "",
			expected: nil,
			operator: "not_empty",
			result:   false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.compareValues(tt.actual, tt.expected, tt.operator)
			assert.Equal(t, tt.result, result)
		})
	}
}

func TestHTTPChecker_extractHeaders(t *testing.T) {
	checker := NewHTTPChecker(5000)
	
	headers := http.Header{
		"Content-Type":  []string{"application/json"},
		"X-Custom-Header": []string{"value1", "value2"},
		"Set-Cookie":    []string{"session=abc123"},
	}
	
	result := checker.extractHeaders(headers)
	
	assert.Equal(t, "application/json", result["Content-Type"])
	assert.Equal(t, "value1", result["X-Custom-Header"])
	assert.Equal(t, "session=abc123", result["Set-Cookie"])
}
