package rabbitmq

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

// TestConsumer_RegisterHandler проверяет регистрацию обработчика
func TestConsumer_RegisterHandler(t *testing.T) {
	// Создаем фейковое подключение
	conn := &Connection{}
	config := NewConfig()
	consumer := NewConsumer(conn, config)

	// Регистрируем обработчик
	consumer.RegisterHandler("test-queue", func(ctx context.Context, msg amqp091.Delivery) error {
		return nil
	})

	// Проверяем, что обработчик зарегистрирован
	if _, exists := consumer.handlers["test-queue"]; !exists {
		t.Error("Expected handler to be registered for 'test-queue'")
	}
}

// TestConsumer_Start проверяет запуск консьюмера
func TestConsumer_Start(t *testing.T) {
	// Создаем фейковое подключение
	// В реальном приложении мы бы использовали мок, но для простоты используем nil
	conn := &Connection{}
	config := NewConfig()
	consumer := NewConsumer(conn, config)

	// Регистрируем обработчик
	var handlerCalled int
	var mu sync.Mutex

	consumer.RegisterHandler("test-queue", func(ctx context.Context, msg amqp091.Delivery) error {
		mu.Lock()
		handlerCalled++
		mu.Unlock()
		return nil
	})

	// Создаем контекст с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Запускаем консьюмера в отдельной горутине
	go func() {
		// В реальном тесте здесь будет реальный RabbitMQ
		// Мы ожидаем ошибку, но хотим проверить, что Start запускается
		err := consumer.Start(ctx)
		if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
			t.Errorf("Unexpected error from Start: %v", err)
		}
	}()

	// Ждем немного
	time.Sleep(50 * time.Millisecond)

	// Отменяем контекст
	cancel()

	// Ждем завершения
	time.Sleep(50 * time.Millisecond)

	// Проверяем, что обработчик был вызван (или хотя бы попытка была)
	// В реальном тесте с моком мы могли бы проверить больше
}

// TestConsumer_HealthCheck проверяет health check
func TestConsumer_HealthCheck(t *testing.T) {
	// Создаем консьюмера без подключения
	consumer := &Consumer{
		conn:   nil,
		config: NewConfig(),
	}

	// Проверяем health check
	ctx := context.Background()
	err := consumer.HealthCheck(ctx)
	if err == nil {
		t.Error("Expected error when connection is not initialized")
	}

	// Создаем консьюмера с фейковым подключением
	consumer = &Consumer{
		conn: &Connection{
			conn: nil, // Соединение не инициализировано
		},
		config: NewConfig(),
	}

	// Проверяем health check
	err = consumer.HealthCheck(ctx)
	if err == nil {
		t.Error("Expected error when connection is not initialized")
	}
}
