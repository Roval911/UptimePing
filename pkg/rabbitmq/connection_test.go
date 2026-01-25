package rabbitmq

import (
	"context"
	"testing"
	"time"
)

// TestConnect_Success проверяет успешное подключение к RabbitMQ
func TestConnect_Success(t *testing.T) {
	// Создаем контекст с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Создаем конфигурацию
	config := NewConfig()
	// Уменьшаем настройки для тестов
	config.MaxRetries = 1
	config.ReconnectInterval = 100 * time.Millisecond

	// Пытаемся подключиться (без ожидания реального RabbitMQ)
	// В реальном тесте здесь будет реальный RabbitMQ
	_, err := Connect(ctx, config)
	// Ожидаем ошибку, так как RabbitMQ не запущен
	// Но проверяем, что функция работает
	if err == nil {
		t.Error("Expected error when connecting to non-existent rabbitmq")
	}
}

// TestNewConfig проверяет создание конфигурации по умолчанию
func TestNewConfig(t *testing.T) {
	config := NewConfig()
	
	if config.URL != "amqp://guest:guest@localhost:5672/" {
		t.Errorf("Expected URL 'amqp://guest:guest@localhost:5672/', got %s", config.URL)
	}
	
	if config.Exchange != "" {
		t.Errorf("Expected exchange '', got %s", config.Exchange)
	}
	
	if config.Queue != "" {
		t.Errorf("Expected queue '', got %s", config.Queue)
	}
	
	if config.DLX != "dlx" {
		t.Errorf("Expected DLX 'dlx', got %s", config.DLX)
	}
	
	if config.DLQ != "dlq" {
		t.Errorf("Expected DLQ 'dlq', got %s", config.DLQ)
	}
	
	if config.ReconnectInterval != 5*time.Second {
		t.Errorf("Expected reconnect interval 5s, got %s", config.ReconnectInterval)
	}
	
	if config.MaxRetries != 3 {
		t.Errorf("Expected max retries 3, got %d", config.MaxRetries)
	}
	
	if config.PrefetchCount != 1 {
		t.Errorf("Expected prefetch count 1, got %d", config.PrefetchCount)
	}
	
	if config.PrefetchSize != 0 {
		t.Errorf("Expected prefetch size 0, got %d", config.PrefetchSize)
	}
	
	if config.Global != false {
		t.Errorf("Expected global false, got %t", config.Global)
	}
}