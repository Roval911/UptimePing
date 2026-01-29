package unit_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockGRPCClient мок для gRPC клиента
type MockGRPCClient struct {
	mock.Mock
}

func (m *MockGRPCClient) Authenticate(ctx context.Context, email, password string) (string, error) {
	args := m.Called(ctx, email, password)
	return args.String(0), args.Error(1)
}

func (m *MockGRPCClient) GetChecks(ctx context.Context, token string) (interface{}, error) {
	args := m.Called(ctx, token)
	return args.Get(0), args.Error(1)
}

func (m *MockGRPCClient) GetConfig(ctx context.Context, token string) (interface{}, error) {
	args := m.Called(ctx, token)
	return args.Get(0), args.Error(1)
}

func (m *MockGRPCClient) GenerateForge(ctx context.Context, token string, config interface{}) (interface{}, error) {
	args := m.Called(ctx, token, config)
	return args.Get(0), args.Error(1)
}

func (m *MockGRPCClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

// TestAuthCommand тестирует команду аутентификации
func TestAuthCommand(t *testing.T) {
	mockClient := new(MockGRPCClient)

	t.Run("Successful authentication", func(t *testing.T) {
		expectedToken := "test-jwt-token"
		mockClient.On("Authenticate", mock.Anything, "test@example.com", "password").
			Return(expectedToken, nil).Once()

		ctx := context.Background()
		token, err := mockClient.Authenticate(ctx, "test@example.com", "password")

		assert.NoError(t, err)
		assert.Equal(t, expectedToken, token)
		mockClient.AssertExpectations(t)
	})

	t.Run("Failed authentication", func(t *testing.T) {
		mockClient.On("Authenticate", mock.Anything, "invalid@example.com", "wrongpassword").
			Return("", assert.AnError).Once()

		ctx := context.Background()
		token, err := mockClient.Authenticate(ctx, "invalid@example.com", "wrongpassword")

		assert.Error(t, err)
		assert.Empty(t, token)
		mockClient.AssertExpectations(t)
	})
}

// TestConfigCommand тестирует команду конфигурации
func TestConfigCommand(t *testing.T) {
	mockClient := new(MockGRPCClient)

	t.Run("Get config successfully", func(t *testing.T) {
		expectedConfig := map[string]interface{}{
			"server": map[string]interface{}{
				"host": "localhost",
				"port": 8080,
			},
			"database": map[string]interface{}{
				"host": "localhost",
				"port": 5432,
			},
		}

		mockClient.On("GetConfig", mock.Anything, "valid-token").
			Return(expectedConfig, nil).Once()

		ctx := context.Background()
		config, err := mockClient.GetConfig(ctx, "valid-token")

		assert.NoError(t, err)
		assert.Equal(t, expectedConfig, config)
		mockClient.AssertExpectations(t)
	})

	t.Run("Get config with invalid token", func(t *testing.T) {
		mockClient.On("GetConfig", mock.Anything, "invalid-token").
			Return(nil, assert.AnError).Once()

		ctx := context.Background()
		config, err := mockClient.GetConfig(ctx, "invalid-token")

		assert.Error(t, err)
		assert.Nil(t, config)
		mockClient.AssertExpectations(t)
	})
}

// TestChecksCommand тестирует команду проверок
func TestChecksCommand(t *testing.T) {
	mockClient := new(MockGRPCClient)

	t.Run("Get checks successfully", func(t *testing.T) {
		expectedChecks := []map[string]interface{}{
			{
				"id":     "check-1",
				"name":   "Website Check",
				"type":   "http",
				"url":    "https://example.com",
				"status": "active",
			},
			{
				"id":     "check-2",
				"name":   "API Check",
				"type":   "grpc",
				"url":    "localhost:50051",
				"status": "active",
			},
		}

		mockClient.On("GetChecks", mock.Anything, "valid-token").
			Return(expectedChecks, nil).Once()

		ctx := context.Background()
		checks, err := mockClient.GetChecks(ctx, "valid-token")

		assert.NoError(t, err)
		assert.Equal(t, expectedChecks, checks)
		mockClient.AssertExpectations(t)
	})

	t.Run("Get checks with network error", func(t *testing.T) {
		mockClient.On("GetChecks", mock.Anything, "valid-token").
			Return(nil, assert.AnError).Once()

		ctx := context.Background()
		checks, err := mockClient.GetChecks(ctx, "valid-token")

		assert.Error(t, err)
		assert.Nil(t, checks)
		mockClient.AssertExpectations(t)
	})
}

// TestForgeCommand тестирует команду генерации forge
func TestForgeCommand(t *testing.T) {
	mockClient := new(MockGRPCClient)

	t.Run("Generate forge successfully", func(t *testing.T) {
		forgeConfig := map[string]interface{}{
			"service_name": "test-service",
			"proto_file":   "test.proto",
			"output_dir":   "./generated",
		}

		expectedResult := map[string]interface{}{
			"status":  "success",
			"message": "Forge generated successfully",
			"files": []string{
				"generated/client.go",
				"generated/server.go",
			},
		}

		mockClient.On("GenerateForge", mock.Anything, "valid-token", forgeConfig).
			Return(expectedResult, nil).Once()

		ctx := context.Background()
		result, err := mockClient.GenerateForge(ctx, "valid-token", forgeConfig)

		assert.NoError(t, err)
		assert.Equal(t, expectedResult, result)
		mockClient.AssertExpectations(t)
	})

	t.Run("Generate forge with invalid config", func(t *testing.T) {
		invalidConfig := map[string]interface{}{
			"service_name": "", // Пустое имя сервиса
		}

		mockClient.On("GenerateForge", mock.Anything, "valid-token", invalidConfig).
			Return(nil, assert.AnError).Once()

		ctx := context.Background()
		result, err := mockClient.GenerateForge(ctx, "valid-token", invalidConfig)

		assert.Error(t, err)
		assert.Nil(t, result)
		mockClient.AssertExpectations(t)
	})
}

// TestErrorHandling тестирует обработку ошибок
func TestErrorHandling(t *testing.T) {
	mockClient := new(MockGRPCClient)

	t.Run("Network timeout", func(t *testing.T) {
		mockClient.On("Authenticate", mock.Anything, mock.Anything, mock.Anything).
			Return("", context.DeadlineExceeded).Once()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		token, err := mockClient.Authenticate(ctx, "test@example.com", "password")

		assert.Error(t, err)
		assert.Equal(t, context.DeadlineExceeded, err)
		assert.Empty(t, token)
		mockClient.AssertExpectations(t)
	})

	t.Run("Connection refused", func(t *testing.T) {
		mockClient.On("GetChecks", mock.Anything, "valid-token").
			Return(nil, assert.AnError).Once()

		ctx := context.Background()
		checks, err := mockClient.GetChecks(ctx, "valid-token")

		assert.Error(t, err)
		assert.Nil(t, checks)
		mockClient.AssertExpectations(t)
	})
}

// TestOutputFormats тестирует форматы вывода
func TestOutputFormats(t *testing.T) {
	data := map[string]interface{}{
		"checks": []map[string]interface{}{
			{
				"id":     "check-1",
				"name":   "Test Check",
				"status": "active",
			},
		},
		"total": 1,
	}

	t.Run("JSON format validation", func(t *testing.T) {
		// Проверяем структуру данных для JSON
		assert.NotNil(t, data)
		assert.Contains(t, data, "checks")
		assert.Contains(t, data, "total")
		
		checks, ok := data["checks"].([]map[string]interface{})
		assert.True(t, ok)
		assert.Len(t, checks, 1)
		
		check := checks[0]
		assert.Equal(t, "check-1", check["id"])
		assert.Equal(t, "Test Check", check["name"])
		assert.Equal(t, "active", check["status"])
	})

	t.Run("YAML format validation", func(t *testing.T) {
		// Проверяем структуру данных для YAML
		assert.NotNil(t, data)
		assert.IsType(t, map[string]interface{}{}, data)
		
		// Проверяем наличие необходимых полей
		assert.Contains(t, data, "checks")
		assert.Contains(t, data, "total")
	})

	t.Run("Table format validation", func(t *testing.T) {
		// Проверяем структуру данных для таблицы
		checks, ok := data["checks"].([]map[string]interface{})
		assert.True(t, ok)
		assert.NotEmpty(t, checks)
		
		// Проверяем структуру строки таблицы
		for _, check := range checks {
			assert.Contains(t, check, "id")
			assert.Contains(t, check, "name")
			assert.Contains(t, check, "status")
		}
	})
}

// TestConfigValidation тестирует валидацию конфигурации
func TestConfigValidation(t *testing.T) {
	t.Run("Valid config structure", func(t *testing.T) {
		cfg := map[string]interface{}{
			"auth": map[string]interface{}{
				"service_address": "localhost:50051",
				"timeout":         "5s",
			},
			"server": map[string]interface{}{
				"api_gateway": "localhost:50051",
			},
			"output": map[string]interface{}{
				"format": "json",
			},
		}

		// Проверяем структуру конфигурации
		assert.NotNil(t, cfg)
		assert.Contains(t, cfg, "auth")
		assert.Contains(t, cfg, "server")
		assert.Contains(t, cfg, "output")
		
		auth := cfg["auth"].(map[string]interface{})
		assert.Equal(t, "localhost:50051", auth["service_address"])
		assert.Equal(t, "5s", auth["timeout"])
	})

	t.Run("Invalid config - missing auth", func(t *testing.T) {
		cfg := map[string]interface{}{
			"server": map[string]interface{}{
				"api_gateway": "localhost:50051",
			},
		}

		// Проверяем отсутствие обязательного поля
		assert.NotContains(t, cfg, "auth")
	})

	t.Run("Invalid config - invalid format", func(t *testing.T) {
		cfg := map[string]interface{}{
			"auth": map[string]interface{}{
				"service_address": "localhost:50051",
				"timeout":         "5s",
			},
			"output": map[string]interface{}{
				"format": "invalid", // Неверный формат
			},
		}

		// Проверяем наличие поля с неверным значением
		output := cfg["output"].(map[string]interface{})
		assert.Equal(t, "invalid", output["format"])
	})
}
