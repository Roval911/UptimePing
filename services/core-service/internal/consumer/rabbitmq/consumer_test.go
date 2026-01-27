package rabbitmq

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"UptimePingPlatform/pkg/logger"
	pkg_rabbitmq "UptimePingPlatform/pkg/rabbitmq"
)

// MockLogger мок для логгера
type MockLogger struct{}

func (m *MockLogger) Debug(msg string, fields ...logger.Field) {}
func (m *MockLogger) Info(msg string, fields ...logger.Field)  {}
func (m *MockLogger) Warn(msg string, fields ...logger.Field)  {}
func (m *MockLogger) Error(msg string, fields ...logger.Field) {}
func (m *MockLogger) With(fields ...logger.Field) logger.Logger {
	return m
}
func (m *MockLogger) Sync() error {
	return nil
}

// MockCheckService мок для CheckService
type MockCheckService struct{}

func (m *MockCheckService) ProcessTask(ctx context.Context, message []byte) error {
	return nil
}

func TestConsumer_NewConsumer(t *testing.T) {
	config := ConsumerConfig{
		QueueName:   "test_queue",
		ConsumerTag: "test_consumer",
	}

	log := &MockLogger{}
	mockService := &MockCheckService{}
	mockRabbitConn := &pkg_rabbitmq.Connection{}
	
	consumer, err := NewConsumer(config, log, mockService, mockRabbitConn)
	assert.NoError(t, err)
	assert.NotNil(t, consumer)
	assert.Equal(t, log, consumer.logger)
	assert.Equal(t, mockService, consumer.checkService)
	assert.Equal(t, "test_queue", consumer.queueName)
	assert.Equal(t, "test_consumer", consumer.consumerTag)

	// Очистка
	consumer.Close()
}

func TestConsumer_config(t *testing.T) {
	config := ConsumerConfig{
		QueueName:   "test_queue",
		ConsumerTag: "test_consumer",
	}
	
	assert.Equal(t, "test_queue", config.QueueName)
	assert.Equal(t, "test_consumer", config.ConsumerTag)
}
