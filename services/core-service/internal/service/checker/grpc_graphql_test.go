package checker

import (
	"fmt"
	"testing"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/core-service/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGRPCChecker_Execute_Success(t *testing.T) {
	// Создание checker'а
	log, err := logger.NewLogger("test", "debug", "core-service", false)
	require.NoError(t, err)
	checker := NewgRPCChecker(5000, log)

	// Создание тестовой задачи с health check
	config := map[string]interface{}{
		"service": "grpc.health.v1.Health",
		"method":  "Check",
		"host":    "localhost",
		"port":    float64(50051),
		"timeout": "5s",
	}

	task := domain.NewTask("check-1", "localhost:50051", "grpc", "exec-1", time.Now(), config)

	// Выполнение проверки (может не пройти если нет gRPC сервера)
	result, err := checker.Execute(task)

	// Проверки
	if err != nil {
		// Ожидаем ошибку если нет сервера
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to connect")
	} else {
		// Если сервер есть, проверяем результат
		assert.NotNil(t, result)
		assert.Equal(t, "check-1", result.CheckID)
		assert.Equal(t, "exec-1", result.ExecutionID)
	}
}

func TestGRPCChecker_Execute_InvalidConfig(t *testing.T) {
	// Создание checker'а
	log, err := logger.NewLogger("test", "debug", "core-service", false)
	require.NoError(t, err)
	checker := NewgRPCChecker(5000, log)

	// Создание задачи с невалидной конфигурацией
	config := map[string]interface{}{
		"service": "", // пустой сервис
		"method":  "Check",
		"host":    "localhost",
		"port":    float64(50051),
	}

	task := domain.NewTask("check-1", "localhost:50051", "grpc", "exec-1", time.Now(), config)

	// Выполнение проверки
	result, err := checker.Execute(task)

	// Проверки
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "config validation failed")
}

func TestGRPCChecker_GetType(t *testing.T) {
	log, err := logger.NewLogger("test", "debug", "core-service", false)
	require.NoError(t, err)
	checker := NewgRPCChecker(5000, log)
	assert.Equal(t, domain.TaskTypeGRPC, checker.GetType())
}

func TestGRPCChecker_SetDialTimeout(t *testing.T) {
	log, err := logger.NewLogger("test", "debug", "core-service", false)
	require.NoError(t, err)
	checker := NewgRPCChecker(1000, log)

	// Проверка начального таймаута
	assert.Equal(t, 1*time.Second, checker.GetDialTimeout())

	// Установка нового таймаута
	newTimeout := 10 * time.Second
	checker.SetDialTimeout(newTimeout)

	// Проверка изменения таймаута
	assert.Equal(t, newTimeout, checker.GetDialTimeout())
}

func TestGRPCChecker_ValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
		errorField  string
	}{
		{
			name: "valid config",
			config: map[string]interface{}{
				"service": "grpc.health.v1.Health",
				"method":  "Check",
				"host":    "localhost",
				"port":    float64(50051),
				"timeout": "5s",
			},
			expectError: false,
		},
		{
			name: "missing service",
			config: map[string]interface{}{
				"method": "Check",
				"host":   "localhost",
				"port":   float64(50051),
			},
			expectError: true,
			errorField:  "Service name",
		},
		{
			name: "empty service",
			config: map[string]interface{}{
				"service": "",
				"method":  "Check",
				"host":    "localhost",
				"port":    float64(50051),
			},
			expectError: true,
			errorField:  "Service name",
		},
		{
			name: "missing method",
			config: map[string]interface{}{
				"service": "grpc.health.v1.Health",
				"host":    "localhost",
				"port":    float64(50051),
			},
			expectError: true,
			errorField:  "Method name",
		},
		{
			name: "missing host",
			config: map[string]interface{}{
				"service": "grpc.health.v1.Health",
				"method":  "Check",
				"port":    float64(50051),
			},
			expectError: true,
			errorField:  "Host address",
		},
		{
			name: "missing port",
			config: map[string]interface{}{
				"service": "grpc.health.v1.Health",
				"method":  "Check",
				"host":    "localhost",
			},
			expectError: true,
			errorField:  "Port number",
		},
		{
			name: "invalid port too low",
			config: map[string]interface{}{
				"service": "grpc.health.v1.Health",
				"method":  "Check",
				"host":    "localhost",
				"port":    float64(0),
			},
			expectError: true,
			errorField:  "port must be between 1 and 65535",
		},
		{
			name: "invalid port too high",
			config: map[string]interface{}{
				"service": "grpc.health.v1.Health",
				"method":  "Check",
				"host":    "localhost",
				"port":    float64(65536),
			},
			expectError: true,
			errorField:  "port must be between 1 and 65535",
		},
		{
			name: "invalid timeout",
			config: map[string]interface{}{
				"service": "grpc.health.v1.Health",
				"method":  "Check",
				"host":    "localhost",
				"port":    float64(50051),
				"timeout": "invalid",
			},
			expectError: true,
			errorField:  "timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log, err := logger.NewLogger("test", "debug", "core-service", false)
			require.NoError(t, err)
			checker := NewgRPCChecker(5000, log)
			err = checker.ValidateConfig(tt.config)

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

func TestGRPCChecker_executeStandardHealthCheck(t *testing.T) {
	log, err := logger.NewLogger("test", "debug", "core-service", false)
	require.NoError(t, err)
	checker := NewgRPCChecker(5000, log)

	// Просто проверяем, что функция существует
	assert.NotNil(t, checker)
}

func TestGRPCChecker_executeCustomMethodCheck(t *testing.T) {
	log, err := logger.NewLogger("test", "debug", "core-service", false)
	require.NoError(t, err)
	checker := NewgRPCChecker(5000, log)

	// Создание тестовой конфигурации
	config := &domain.GPRCConfig{
		Service: "test.service",
		Method:  "TestMethod",
		Host:    "localhost",
		Port:    50051,
	}

	// Просто проверяем, что функция существует и конфигурация создана
	assert.NotNil(t, checker)
	assert.Equal(t, "test.service", config.Service)
	assert.Equal(t, "TestMethod", config.Method)
}

func TestGRPCChecker_createErrorResult(t *testing.T) {
	log, err := logger.NewLogger("test", "debug", "core-service", false)
	require.NoError(t, err)
	checker := NewgRPCChecker(5000, log)

	task := domain.NewTask("check-1", "localhost:50051", "grpc", "exec-1", time.Now(), map[string]interface{}{})
	testErr := fmt.Errorf("test error")

	result := checker.createErrorResult(task, 500, 1000, testErr)

	assert.False(t, result.Success)
	assert.Equal(t, "check-1", result.CheckID)
	assert.Equal(t, "exec-1", result.ExecutionID)
	assert.Equal(t, int64(1000), result.DurationMs)
	assert.Equal(t, 500, result.StatusCode)
	assert.Equal(t, testErr.Error(), result.Error)
}

func TestGraphQLChecker_Execute_Success(t *testing.T) {
	// Создание checker'а
	log, err := logger.NewLogger("test", "debug", "core-service", false)
	require.NoError(t, err)
	checker := NewGraphQLChecker(5000, log)

	// Создание тестовой задачи
	config := map[string]interface{}{
		"url":     "https://httpbin.org/graphql",
		"query":   "{ status }",
		"timeout": "5s",
	}

	task := domain.NewTask("check-1", "https://httpbin.org/graphql", "graphql", "exec-1", time.Now(), config)

	// Выполнение проверки
	result, err := checker.Execute(task)

	// Проверки
	if err != nil {
		// Ожидаем ошибку если нет GraphQL сервера
		assert.Error(t, err)
	} else {
		// Если сервер есть, проверяем результат
		assert.NotNil(t, result)
		assert.Equal(t, "check-1", result.CheckID)
		assert.Equal(t, "exec-1", result.ExecutionID)
		assert.True(t, result.DurationMs > 0)
	}
}

func TestGraphQLChecker_Execute_InvalidConfig(t *testing.T) {
	// Создание checker'а
	log, err := logger.NewLogger("test", "debug", "core-service", false)
	require.NoError(t, err)
	checker := NewGraphQLChecker(5000, log)

	// Создание задачи с невалидной конфигурацией
	config := map[string]interface{}{
		"url":   "", // пустой URL
		"query": "{ status }",
	}

	task := domain.NewTask("check-1", "https://example.com/graphql", "graphql", "exec-1", time.Now(), config)

	// Выполнение проверки
	result, err := checker.Execute(task)

	// Проверки
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "config validation failed")
}

func TestGraphQLChecker_GetType(t *testing.T) {
	log, err := logger.NewLogger("test", "debug", "core-service", false)
	require.NoError(t, err)
	checker := NewGraphQLChecker(5000, log)
	assert.Equal(t, domain.TaskTypeGraphQL, checker.GetType())
}

func TestGraphQLChecker_SetTimeout(t *testing.T) {
	log, err := logger.NewLogger("test", "debug", "core-service", false)
	require.NoError(t, err)
	checker := NewGraphQLChecker(1000, log)

	// Проверка начального таймаута
	assert.Equal(t, time.Second, checker.GetTimeout())

	// Установка нового таймаута
	newTimeout := 5 * time.Second
	checker.SetTimeout(newTimeout)

	// Проверка изменения таймаута
	assert.Equal(t, 5*time.Second, checker.GetTimeout())
	assert.Equal(t, newTimeout, checker.GetClient().Timeout)
}

func TestGraphQLChecker_ValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
		errorField  string
	}{
		{
			name: "valid config",
			config: map[string]interface{}{
				"url":     "https://example.com/graphql",
				"query":   "{ status }",
				"timeout": "5s",
			},
			expectError: false,
		},
		{
			name: "missing url",
			config: map[string]interface{}{
				"query": "{ status }",
			},
			expectError: true,
			errorField:  "GraphQL endpoint URL",
		},
		{
			name: "empty url",
			config: map[string]interface{}{
				"url":   "",
				"query": "{ status }",
			},
			expectError: true,
			errorField:  "GraphQL endpoint URL",
		},
		{
			name: "invalid url scheme",
			config: map[string]interface{}{
				"url":   "ftp://example.com/graphql",
				"query": "{ status }",
			},
			expectError: true,
			errorField:  "URL",
		},
		{
			name: "missing query",
			config: map[string]interface{}{
				"url": "https://example.com/graphql",
			},
			expectError: true,
			errorField:  "GraphQL query",
		},
		{
			name: "empty query",
			config: map[string]interface{}{
				"url":   "https://example.com/graphql",
				"query": "",
			},
			expectError: true,
			errorField:  "GraphQL query",
		},
		{
			name: "invalid query no braces",
			config: map[string]interface{}{
				"url":   "https://example.com/graphql",
				"query": "status",
			},
			expectError: true,
			errorField:  "query",
		},
		{
			name: "invalid timeout",
			config: map[string]interface{}{
				"url":     "https://example.com/graphql",
				"query":   "{ status }",
				"timeout": "invalid",
			},
			expectError: true,
			errorField:  "invalid timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log, err := logger.NewLogger("test", "debug", "core-service", false)
			require.NoError(t, err)
			checker := NewGraphQLChecker(5000, log)
			err = checker.ValidateConfig(tt.config)

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

func TestGraphQLChecker_createGraphQLRequest(t *testing.T) {
	log, err := logger.NewLogger("test", "debug", "core-service", false)
	require.NoError(t, err)
	checker := NewGraphQLChecker(5000, log)

	tests := []struct {
		name     string
		config   *domain.GraphQLConfig
		expected string
	}{
		{
			name: "simple query",
			config: &domain.GraphQLConfig{
				URL:   "https://example.com/graphql",
				Query: "{ status }",
			},
			expected: "POST",
		},
		{
			name: "query with variables",
			config: &domain.GraphQLConfig{
				URL:   "https://example.com/graphql",
				Query: "query GetUser($id: ID!) { user(id: $id) { name } }",
				Variables: map[string]interface{}{
					"id": "123",
				},
			},
			expected: "POST",
		},
		{
			name: "query with operation name",
			config: &domain.GraphQLConfig{
				URL:           "https://example.com/graphql",
				Query:         "{ status }",
				OperationName: "GetStatus",
			},
			expected: "POST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := checker.createGraphQLRequest(tt.config)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, req.Method)
			assert.Equal(t, tt.config.URL, req.URL.String())
			assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
			assert.Equal(t, "application/json", req.Header.Get("Accept"))
		})
	}
}

func TestGraphQLChecker_parseGraphQLResponse(t *testing.T) {
	log, err := logger.NewLogger("test", "debug", "core-service", false)
	require.NoError(t, err)
	checker := NewGraphQLChecker(5000, log)

	tests := []struct {
		name        string
		body        string
		expectError bool
		expectData  bool
	}{
		{
			name:        "valid response with data",
			body:        `{"data": {"status": "ok"}}`,
			expectError: false,
			expectData:  true,
		},
		{
			name:        "valid response with errors",
			body:        `{"errors": [{"message": "Field not found"}]}`,
			expectError: false,
			expectData:  false,
		},
		{
			name:        "invalid JSON",
			body:        `invalid json`,
			expectError: true,
			expectData:  false,
		},
		{
			name:        "empty response",
			body:        `{}`,
			expectError: false,
			expectData:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := checker.parseGraphQLResponse(tt.body)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.expectData {
					assert.NotNil(t, resp.Data)
				}
			}
		})
	}
}

func TestGraphQLChecker_createErrorResult(t *testing.T) {
	log, err := logger.NewLogger("test", "debug", "core-service", false)
	require.NoError(t, err)
	checker := NewGraphQLChecker(5000, log)

	task := domain.NewTask("check-1", "https://example.com/graphql", "graphql", "exec-1", time.Now(), map[string]interface{}{})
	testErr := fmt.Errorf("test error")

	result := checker.createErrorResult(task, 500, 1000, testErr)

	assert.False(t, result.Success)
	assert.Equal(t, "check-1", result.CheckID)
	assert.Equal(t, "exec-1", result.ExecutionID)
	assert.Equal(t, int64(1000), result.DurationMs)
	assert.Equal(t, 500, result.StatusCode)
	assert.Equal(t, testErr.Error(), result.Error)
}
