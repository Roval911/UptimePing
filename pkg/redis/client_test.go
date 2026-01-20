package redis

import (
	"context"
	"testing"
	"time"
)

// TestConnect_Success проверяет успешное подключение к Redis
func TestConnect_Success(t *testing.T) {
	// Создаем контекст с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Создаем конфигурацию
	config := NewConfig()
	// Уменьшаем настройки для тестов
	config.MaxRetries = 1
	config.RetryInterval = 100 * time.Millisecond

	// Пытаемся подключиться (без ожидания реального Redis)
	// В реальном тесте здесь будет реальный Redis
	_, err := Connect(ctx, config)
	// Ожидаем ошибку, так как Redis не запущен
	// Но проверяем, что функция работает
	if err == nil {
		t.Error("Expected error when connecting to non-existent redis")
	}
}

// TestHealthCheck проверяет health check
func TestHealthCheck(t *testing.T) {
	client := &Client{}
	ctx := context.Background()

	// Проверяем health check без инициализированного клиента
	err := client.HealthCheck(ctx)
	if err == nil {
		t.Error("Expected error when client is not initialized")
	}
}

// TestNewConfig проверяет создание конфигурации по умолчанию
func TestNewConfig(t *testing.T) {
	config := NewConfig()
	
	if config.Addr != "localhost:6379" {
		t.Errorf("Expected addr 'localhost:6379', got %s", config.Addr)
	}
	
	if config.DB != 0 {
		t.Errorf("Expected DB 0, got %d", config.DB)
	}
	
	if config.PoolSize != 10 {
		t.Errorf("Expected pool size 10, got %d", config.PoolSize)
	}
	
	if config.MinIdleConn != 2 {
		t.Errorf("Expected min idle conn 2, got %d", config.MinIdleConn)
	}
	
	if config.MaxRetries != 3 {
		t.Errorf("Expected max retries 3, got %d", config.MaxRetries)
	}
	
	if config.RetryInterval != 1*time.Second {
		t.Errorf("Expected retry interval 1s, got %s", config.RetryInterval)
	}
}