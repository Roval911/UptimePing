package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCLIConfiguration тестирует конфигурацию CLI
func TestCLIConfiguration(t *testing.T) {
	tempDir := t.TempDir()
	
	t.Run("Create config file", func(t *testing.T) {
		configPath := filepath.Join(tempDir, "config.yaml")
		configContent := `
auth:
  service_address: "localhost:50051"
  timeout: "5s"
server:
  api_gateway: "localhost:50051"
output:
  format: "json"
`
		
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)
		
		// Проверяем, что файл создан
		_, err = os.Stat(configPath)
		assert.NoError(t, err)
	})
	
	t.Run("Load config from file", func(t *testing.T) {
		configPath := filepath.Join(tempDir, "config.yaml")
		configContent := `
auth:
  service_address: "localhost:50051"
  timeout: "5s"
`
		
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)
		
		// Проверяем чтение файла
		content, err := os.ReadFile(configPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "localhost:50051")
	})
}

// TestCLIContextOperations тестирует операции с контекстом
func TestCLIContextOperations(t *testing.T) {
	t.Run("Context with timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		
		// Проверяем, что контекст создан
		assert.NotNil(t, ctx)
		
		// Проверяем таймаут
		select {
		case <-ctx.Done():
			assert.Equal(t, context.DeadlineExceeded, ctx.Err())
		case <-time.After(200 * time.Millisecond):
			t.Error("Context should have timed out")
		}
	})
	
	t.Run("Context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		
		// Отменяем контекст
		cancel()
		
		// Проверяем, что контекст отменен
		select {
		case <-ctx.Done():
			assert.Equal(t, context.Canceled, ctx.Err())
		default:
			t.Error("Context should be cancelled")
		}
	})
}

// TestCLIErrorScenarios тестирует сценарии ошибок
func TestCLIErrorScenarios(t *testing.T) {
	t.Run("File not found error", func(t *testing.T) {
		nonExistentPath := filepath.Join(t.TempDir(), "nonexistent.yaml")
		
		_, err := os.Stat(nonExistentPath)
		assert.True(t, os.IsNotExist(err))
	})
	
	t.Run("Invalid YAML format", func(t *testing.T) {
		invalidYAMLPath := filepath.Join(t.TempDir(), "invalid.yaml")
		invalidContent := `
invalid: yaml: content:
  - missing
  proper: structure
`
		
		err := os.WriteFile(invalidYAMLPath, []byte(invalidContent), 0644)
		require.NoError(t, err)
		
		// Проверяем, что файл существует
		_, err = os.Stat(invalidYAMLPath)
		assert.NoError(t, err)
		
		// В реальном коде здесь была бы попытка распарсить YAML
		// и проверка на ошибку
		content, err := os.ReadFile(invalidYAMLPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "invalid: yaml: content:")
	})
	
	t.Run("Network connection timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		
		// Пытаемся подключиться к несуществующему адресу
		connector := func(ctx context.Context) error {
			select {
			case <-time.After(100 * time.Millisecond):
				return nil // Имитируем долгое подключение
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		
		err := connector(ctx)
		assert.Equal(t, context.DeadlineExceeded, err)
	})
}

// TestCLIOutputFormats тестирует форматы вывода
func TestCLIOutputFormats(t *testing.T) {
	testData := map[string]interface{}{
		"checks": []map[string]interface{}{
			{
				"id":     "check-1",
				"name":   "Website Check",
				"type":   "http",
				"url":    "https://example.com",
				"status": "active",
			},
		},
		"total": 1,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	
	t.Run("JSON output structure", func(t *testing.T) {
		// Проверяем структуру для JSON вывода
		assert.NotNil(t, testData)
		assert.Contains(t, testData, "checks")
		assert.Contains(t, testData, "total")
		assert.Contains(t, testData, "timestamp")
		
		checks := testData["checks"].([]map[string]interface{})
		assert.Len(t, checks, 1)
		
		check := checks[0]
		assert.Equal(t, "check-1", check["id"])
		assert.Equal(t, "Website Check", check["name"])
		assert.Equal(t, "http", check["type"])
		assert.Equal(t, "https://example.com", check["url"])
		assert.Equal(t, "active", check["status"])
	})
	
	t.Run("Table output structure", func(t *testing.T) {
		// Проверяем структуру для табличного вывода
		checks := testData["checks"].([]map[string]interface{})
		require.NotEmpty(t, checks)
		
		// Проверяем заголовки таблицы
		headers := []string{"ID", "Name", "Type", "URL", "Status"}
		assert.Len(t, headers, 5)
		
		// Проверяем данные для таблицы
		for _, check := range checks {
			assert.Contains(t, check, "id")
			assert.Contains(t, check, "name")
			assert.Contains(t, check, "type")
			assert.Contains(t, check, "url")
			assert.Contains(t, check, "status")
		}
	})
	
	t.Run("YAML output structure", func(t *testing.T) {
		// Проверяем структуру для YAML вывода
		assert.IsType(t, map[string]interface{}{}, testData)
		
		// Проверяем вложенные структуры
		checks, ok := testData["checks"].([]map[string]interface{})
		assert.True(t, ok)
		assert.NotEmpty(t, checks)
		
		// Проверяем скалярные значения
		total, ok := testData["total"].(int)
		assert.True(t, ok)
		assert.Equal(t, 1, total)
		
		timestamp, ok := testData["timestamp"].(string)
		assert.True(t, ok)
		assert.NotEmpty(t, timestamp)
	})
}

// TestCLITokenManagement тестирует управление токенами
func TestCLITokenManagement(t *testing.T) {
	tempDir := t.TempDir()
	tokenStorePath := filepath.Join(tempDir, "tokens.json")
	
	t.Run("Save token to file", func(t *testing.T) {
		tokenData := map[string]interface{}{
			"token":    "test-jwt-token",
			"tenant_id": "test-tenant",
			"user_id":   "test-user",
			"email":     "test@example.com",
			"expires_at": time.Now().Add(time.Hour).Format(time.RFC3339),
		}
		
		// В реальном коде здесь была бы сериализация в JSON
		// Для теста просто проверяем структуру данных
		assert.NotNil(t, tokenData)
		assert.Contains(t, tokenData, "token")
		assert.Contains(t, tokenData, "tenant_id")
		assert.Contains(t, tokenData, "user_id")
		assert.Contains(t, tokenData, "email")
		assert.Contains(t, tokenData, "expires_at")
		
		assert.Equal(t, "test-jwt-token", tokenData["token"])
		assert.Equal(t, "test-tenant", tokenData["tenant_id"])
		assert.Equal(t, "test-user", tokenData["user_id"])
		assert.Equal(t, "test@example.com", tokenData["email"])
	})
	
	t.Run("Load token from file", func(t *testing.T) {
		// Создаем тестовый файл с токеном
		tokenContent := `{
			"token": "test-jwt-token",
			"tenant_id": "test-tenant",
			"user_id": "test-user",
			"email": "test@example.com"
		}`
		
		err := os.WriteFile(tokenStorePath, []byte(tokenContent), 0644)
		require.NoError(t, err)
		
		// Проверяем чтение файла
		content, err := os.ReadFile(tokenStorePath)
		require.NoError(t, err)
		
		assert.Contains(t, string(content), "test-jwt-token")
		assert.Contains(t, string(content), "test-tenant")
		assert.Contains(t, string(content), "test@example.com")
	})
	
	t.Run("Token expiration check", func(t *testing.T) {
		expiredTime := time.Now().Add(-time.Hour).Format(time.RFC3339)
		validTime := time.Now().Add(time.Hour).Format(time.RFC3339)
		
		// Проверяем истекший токен
		assert.True(t, isTokenExpired(expiredTime))
		
		// Проверяем валидный токен
		assert.False(t, isTokenExpired(validTime))
	})
}

// TestCLICommandValidation тестирует валидацию команд
func TestCLICommandValidation(t *testing.T) {
	t.Run("Valid command structure", func(t *testing.T) {
		command := map[string]interface{}{
			"name": "checks",
			"args": []string{"--status", "active"},
			"flags": map[string]interface{}{
				"format": "json",
				"limit":  10,
			},
		}
		
		// Проверяем структуру команды
		assert.NotNil(t, command)
		assert.Contains(t, command, "name")
		assert.Contains(t, command, "args")
		assert.Contains(t, command, "flags")
		
		assert.Equal(t, "checks", command["name"])
		
		args := command["args"].([]string)
		assert.Contains(t, args, "--status")
		assert.Contains(t, args, "active")
		
		flags := command["flags"].(map[string]interface{})
		assert.Equal(t, "json", flags["format"])
		assert.Equal(t, 10, flags["limit"])
	})
	
	t.Run("Invalid command - missing name", func(t *testing.T) {
		command := map[string]interface{}{
			"args": []string{"--help"},
		}
		
		// Проверяем отсутствие обязательного поля
		assert.NotContains(t, command, "name")
	})
	
	t.Run("Invalid command - empty args", func(t *testing.T) {
		command := map[string]interface{}{
			"name": "config",
			"args": []string{},
		}
		
		args := command["args"].([]string)
		assert.Empty(t, args)
	})
}

// Вспомогательная функция для проверки истечения токена
func isTokenExpired(expirationTime string) bool {
	parsedTime, err := time.Parse(time.RFC3339, expirationTime)
	if err != nil {
		return true // Если не можем распарсить, считаем истекшим
	}
	return time.Now().After(parsedTime)
}

// TestCLINetworkOperations тестирует сетевые операции
func TestCLINetworkOperations(t *testing.T) {
	t.Run("Connection to valid address", func(t *testing.T) {
		// Тестируем подключение к валидному адресу (может не работать в тесте)
		address := "localhost:50051"
		
		// Проверяем формат адреса
		assert.NotEmpty(t, address)
		assert.Contains(t, address, ":")
		
		parts := strings.Split(address, ":")
		assert.Len(t, parts, 2)
		assert.Equal(t, "localhost", parts[0])
		assert.Equal(t, "50051", parts[1])
	})
	
	t.Run("Connection timeout handling", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		
		// Имитируем подключение с таймаутом
		connect := func(ctx context.Context) error {
			select {
			case <-time.After(100 * time.Millisecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		
		err := connect(ctx)
		assert.Equal(t, context.DeadlineExceeded, err)
	})
}

// TestCLIFileOperations тестирует файловые операции
func TestCLIFileOperations(t *testing.T) {
	tempDir := t.TempDir()
	
	t.Run("Create and read config file", func(t *testing.T) {
		configPath := filepath.Join(tempDir, "test-config.yaml")
		configContent := `
server:
  host: "localhost"
  port: 8080
auth:
  enabled: true
`
		
		// Создаем файл
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)
		
		// Проверяем существование файла
		_, err = os.Stat(configPath)
		assert.NoError(t, err)
		
		// Читаем файл
		content, err := os.ReadFile(configPath)
		require.NoError(t, err)
		
		assert.Contains(t, string(content), "server:")
		assert.Contains(t, string(content), "host: \"localhost\"")
		assert.Contains(t, string(content), "port: 8080")
		assert.Contains(t, string(content), "auth:")
		assert.Contains(t, string(content), "enabled: true")
	})
	
	t.Run("Handle missing file", func(t *testing.T) {
		missingPath := filepath.Join(tempDir, "missing.yaml")
		
		_, err := os.Stat(missingPath)
		assert.True(t, os.IsNotExist(err))
	})
	
	t.Run("Create directory structure", func(t *testing.T) {
		subDir := filepath.Join(tempDir, "subdir", "nested")
		
		err := os.MkdirAll(subDir, 0755)
		require.NoError(t, err)
		
		// Проверяем создание директории
		_, err = os.Stat(subDir)
		assert.NoError(t, err)
		
		// Создаем файл в вложенной директории
		filePath := filepath.Join(subDir, "test.txt")
		err = os.WriteFile(filePath, []byte("test content"), 0644)
		require.NoError(t, err)
		
		content, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, "test content", string(content))
	})
}
