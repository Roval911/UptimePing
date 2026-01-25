package rabbitmq

import (
	"context"
	"testing"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

// TestProducer_Publish проверяет публикацию сообщения
func TestProducer_Publish(t *testing.T) {
	// Создаем фейковое подключение
	// В реальном тесте здесь будет мок или реальный RabbitMQ
	conn := &Connection{}
	config := NewConfig()
	producer := NewProducer(conn, config)

	// Пытаемся опубликовать сообщение
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Так как у нас нет реального соединения, ожидаем ошибку
	err := producer.Publish(ctx, []byte("test message"))
	if err == nil {
		t.Error("Expected error when publishing to non-existent connection")
	}
}

// TestProducer_PublishWithRetry проверяет публикацию с retry
func TestProducer_PublishWithRetry(t *testing.T) {
	// Создаем фейковое подключение
	conn := &Connection{}
	config := NewConfig()
	producer := NewProducer(conn, config)

	// Пытаемся опубликовать сообщение с retry
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Так как у нас нет реального соединения, ожидаем ошибку
	err := producer.PublishWithRetry(ctx, []byte("test message"), 2, 10*time.Millisecond)
	if err == nil {
		t.Error("Expected error when publishing with retry to non-existent connection")
	}
}

// TestPublishOptions проверяет опции публикации
func TestPublishOptions(t *testing.T) {
	// Тестируем функции опций
	opts := &PublishOptions{}

	// Тестируем WithExchange
	WithExchange("test-exchange")(opts)
	if opts.Exchange != "test-exchange" {
		t.Errorf("Expected exchange 'test-exchange', got %s", opts.Exchange)
	}

	// Сбрасываем
	opts = &PublishOptions{}

	// Тестируем WithRoutingKey
	WithRoutingKey("test-routing-key")(opts)
	if opts.RoutingKey != "test-routing-key" {
		t.Errorf("Expected routing key 'test-routing-key', got %s", opts.RoutingKey)
	}

	// Сбрасываем
	opts = &PublishOptions{}

	// Тестируем WithMandatory
	WithMandatory(true)(opts)
	if !opts.Mandatory {
		t.Error("Expected mandatory true")
	}

	// Сбрасываем
	opts = &PublishOptions{}

	// Тестируем WithImmediate
	WithImmediate(true)(opts)
	if !opts.Immediate {
		t.Error("Expected immediate true")
	}

	// Сбрасываем
	opts = &PublishOptions{}

	// Тестируем WithHeaders
	headers := amqp091.Table{"test": "value"}
	WithHeaders(headers)(opts)
	if opts.Headers["test"] != "value" {
		t.Errorf("Expected header 'test' with value 'value', got %v", opts.Headers["test"])
	}
}
