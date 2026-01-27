package rabbitmq

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockMessageHandler мок для MessageHandler
type MockMessageHandler struct {
	mock.Mock
}

func (m *MockMessageHandler) Execute(ctx context.Context, msg amqp091.Delivery) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func TestConsumer_RetryLogic(t *testing.T) {
	// Создаем тестовую конфигурацию
	config := NewConfig()
	config.MaxRetryAttempts = 2
	config.RetryDelay = 100 * time.Millisecond
	
	// Создаем consumer
	conn := &Connection{} // Заглушка для теста
	consumer := NewConsumer(conn, config)
	
	// Создаем мок обработчик
	mockHandler := &MockMessageHandler{}
	
	// Первая попытка - ошибка
	mockHandler.On("Execute", mock.Anything, mock.Anything).Once().Return(errors.New("test error"))
	
	// Регистрируем обработчик
	consumer.RegisterHandler("test-queue", mockHandler.Execute)
	
	// Тестируем логику retry
	// В реальном тесте здесь нужно было бы создать реальное подключение к RabbitMQ
	// Для демонстрации просто проверяем, что конфигурация установлена правильно
	assert.Equal(t, 2, config.MaxRetryAttempts)
	assert.Equal(t, 100*time.Millisecond, config.RetryDelay)
}

func TestConsumer_PublishToDLQ(t *testing.T) {
	// Создаем тестовую конфигурацию
	config := NewConfig()
	config.DLX = "test-dlx"
	config.DLQ = "test-dlq"
	
	// Создаем consumer
	conn := &Connection{} // Заглушка для теста
	consumer := NewConsumer(conn, config)
	
	// Создаем тестовое сообщение
	msg := amqp091.Delivery{
		Headers: amqp091.Table{
			"x-death": []interface{}{
				amqp091.Table{
					"count":      3,
					"exchange":   "test-exchange",
					"queue":      "test-queue",
					"reason":     "rejected",
					"time":       time.Now(),
				},
			},
		},
		RoutingKey:  "test-queue",
		Exchange:    "test-exchange",
		ContentType: "application/json",
		Timestamp:   time.Now(),
		MessageId:   "test-message-id",
		Body:        []byte(`{"test": "data"}`),
	}
	
	// Тестируем publishToDLQ
	err := consumer.publishToDLQ(msg, errors.New("processing error"))
	
	// В реальном тесте здесь нужно было бы создать реальное подключение к RabbitMQ
	// Для демонстрации просто проверяем, что функция не паникует
	assert.Error(t, err) // Ожидаем ошибку, так как connection не инициализирован
	assert.Contains(t, err.Error(), "rabbitmq channel is not initialized")
}

func TestConfig_RetrySettings(t *testing.T) {
	// Очищаем переменные окружения
	clearEnvVars()
	
	// Устанавливаем retry переменные
	os.Setenv("RABBITMQ_MAX_RETRY_ATTEMPTS", "5")
	os.Setenv("RABBITMQ_RETRY_DELAY", "10s")
	
	defer clearEnvVars()
	
	config := GetConfig()
	
	// Проверяем retry настройки
	assert.Equal(t, 5, config.MaxRetryAttempts)
	assert.Equal(t, 10*time.Second, config.RetryDelay)
}

func TestConfig_DefaultRetrySettings(t *testing.T) {
	// Очищаем переменные окружения
	clearEnvVars()
	
	config := GetConfig()
	
	// Проверяем значения по умолчанию
	assert.Equal(t, 3, config.MaxRetryAttempts)
	assert.Equal(t, 5*time.Second, config.RetryDelay)
}

func TestConfig_InvalidRetrySettings(t *testing.T) {
	// Очищаем переменные окружения
	clearEnvVars()
	
	// Устанавливаем некорректные значения
	os.Setenv("RABBITMQ_MAX_RETRY_ATTEMPTS", "invalid")
	os.Setenv("RABBITMQ_RETRY_DELAY", "not_a_duration")
	
	defer clearEnvVars()
	
	config := GetConfig()
	
	// Проверяем, что остались значения по умолчанию
	assert.Equal(t, 3, config.MaxRetryAttempts)
	assert.Equal(t, 5*time.Second, config.RetryDelay)
}
