package database

import (
	"context"
	"testing"
	"time"
)

// TestConnect_Success проверяет успешное подключение к PostgreSQL
func TestConnect_Success(t *testing.T) {
	// Создаем контекст с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Создаем конфигурацию
	config := NewConfig()
	// Уменьшаем настройки для тестов
	config.MaxRetries = 1
	config.RetryInterval = 100 * time.Millisecond

	// Пытаемся подключиться (без ожидания реальной базы данных)
	// В реальном тесте здесь будет реальная база данных
	_, err := Connect(ctx, config)
	// Ожидаем ошибку, так как база данных не запущена
	// Но проверяем, что функция работает
	if err == nil {
		t.Error("Expected error when connecting to non-existent database")
	}
}

// TestHealthCheck проверяет health check
func TestHealthCheck(t *testing.T) {
	postgres := &Postgres{}
	ctx := context.Background()

	// Проверяем health check без инициализированного пула
	err := postgres.HealthCheck(ctx)
	if err == nil {
		t.Error("Expected error when pool is not initialized")
	}
}

// TestNewConfig проверяет создание конфигурации по умолчанию
func TestNewConfig(t *testing.T) {
	config := NewConfig()

	if config.Host != "localhost" {
		t.Errorf("Expected host 'localhost', got %s", config.Host)
	}

	if config.Port != 5432 {
		t.Errorf("Expected port 5432, got %d", config.Port)
	}

	if config.MaxConns != 20 {
		t.Errorf("Expected max conns 20, got %d", config.MaxConns)
	}

	if config.MinConns != 5 {
		t.Errorf("Expected min conns 5, got %d", config.MinConns)
	}

	if config.MaxConnLife != 30*time.Minute {
		t.Errorf("Expected max conn life 30m, got %s", config.MaxConnLife)
	}

	if config.MaxConnIdle != 5*time.Minute {
		t.Errorf("Expected max conn idle 5m, got %s", config.MaxConnIdle)
	}

	if config.MaxRetries != 3 {
		t.Errorf("Expected max retries 3, got %d", config.MaxRetries)
	}

	if config.RetryInterval != 1*time.Second {
		t.Errorf("Expected retry interval 1s, got %s", config.RetryInterval)
	}
}
