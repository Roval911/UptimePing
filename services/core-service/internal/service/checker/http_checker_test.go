package checker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"UptimePingPlatform/services/core-service/internal/domain"
	"UptimePingPlatform/pkg/logger"
)

func TestHTTPChecker_Execute_Success(t *testing.T) {
	// Создание реального logger для тестов
	log, err := logger.NewLogger("test", "debug", "core-service", false)
	require.NoError(t, err)
	
	// Создание checker'а
	checker := NewHTTPChecker(5000, log)
	
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
	// Создание реального logger для тестов
	log, err := logger.NewLogger("test", "debug", "core-service", false)
	require.NoError(t, err)
	
	// Создание checker'а
	checker := NewHTTPChecker(5000, log)
	
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
	log, err := logger.NewLogger("test", "debug", "core-service", false)
	require.NoError(t, err)
	
	checker := NewHTTPChecker(5000, log)
	assert.Equal(t, domain.TaskTypeHTTP, checker.GetType())
}

func TestHTTPChecker_SetTimeout(t *testing.T) {
	log, err := logger.NewLogger("test", "debug", "core-service", false)
	require.NoError(t, err)
	
	checker := NewHTTPChecker(5000, log)
	newTimeout := 10000 * time.Millisecond
	
	checker.SetTimeout(newTimeout)
	assert.Equal(t, newTimeout, checker.GetClient().Timeout)
}
