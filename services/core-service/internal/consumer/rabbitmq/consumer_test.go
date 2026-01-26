package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"UptimePingPlatform/services/core-service/internal/service"
	"UptimePingPlatform/pkg/logger"
)

// MockCheckService мок для CheckService
type MockCheckService struct {
	shouldError bool
	errorMsg    string
}

func (m *MockCheckService) ProcessTask(ctx context.Context, message []byte) error {
	if m.shouldError {
		return fmt.Errorf("mock error: %s", m.errorMsg)
	}
	return nil
}

func TestConsumer_NewConsumer(t *testing.T) {
	config := ConsumerConfig{
		QueueName:   "test_queue",
		ConsumerTag: "test_consumer",
	}
	
	log := &MockLogger{}
	mockService := &MockCheckService{}
	
	consumer, err := NewConsumer(config, log, mockService)
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

func TestConsumer_Start(t *testing.T) {
	config := ConsumerConfig{
		QueueName:   "test_queue",
		ConsumerTag: "test_consumer",
	}
	
	log := &MockLogger{}
	mockService := &MockCheckService{}
	
	consumer, err := NewConsumer(config, log, mockService)
	require.NoError(t, err)
	
	ctx := context.Background()
	err = consumer.Start(ctx)
	assert.NoError(t, err)
	
	consumer.Close()
}

func TestConsumer_ProcessMessage_Success(t *testing.T) {
	config := ConsumerConfig{
		QueueName:   "test_queue",
		ConsumerTag: "test_consumer",
	}
	
	log := &MockLogger{}
	mockService := &MockCheckService{}
	
	consumer, err := NewConsumer(config, log, mockService)
	require.NoError(t, err)
	
	// Создание тестового сообщения
	message := service.TaskMessage{
		CheckID:     "check-1",
		ExecutionID: "exec-1",
		Target:      "https://example.com",
		Type:        "http",
		Config:       map[string]interface{}{"method": "GET"},
		ScheduledAt:  time.Now(),
	}
	
	messageBytes, err := json.Marshal(message)
	require.NoError(t, err)
	
	// Обработка сообщения
	ctx := context.Background()
	err = consumer.ProcessMessage(ctx, messageBytes)
	assert.NoError(t, err)
	
	// Проверка логов
	logs := log.GetLogs()
	assert.Contains(t, logs, "INFO: Processing message")
	assert.Contains(t, logs, "INFO: Message processed successfully")
	
	consumer.Close()
}

func TestConsumer_ProcessMessage_Error(t *testing.T) {
	config := ConsumerConfig{
		QueueName:   "test_queue",
		ConsumerTag: "test_consumer",
	}
	
	log := &MockLogger{}
	mockService := &MockCheckService{
		shouldError: true,
		errorMsg:    "processing failed",
	}
	
	consumer, err := NewConsumer(config, log, mockService)
	require.NoError(t, err)
	
	// Создание тестового сообщения
	message := service.TaskMessage{
		CheckID:     "check-1",
		ExecutionID: "exec-1",
		Target:      "https://example.com",
		Type:        "http",
		Config:       map[string]interface{}{},
		ScheduledAt:  time.Now(),
	}
	
	messageBytes, err := json.Marshal(message)
	require.NoError(t, err)
	
	// Обработка сообщения
	ctx := context.Background()
	err = consumer.ProcessMessage(ctx, messageBytes)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mock error: processing failed")
	
	// Проверка логов
	logs := log.GetLogs()
	assert.Contains(t, logs, "INFO: Processing message")
	assert.Contains(t, logs, "ERROR: Failed to process message")
	
	consumer.Close()
}

func TestConsumer_GetStats(t *testing.T) {
	config := ConsumerConfig{
		QueueName:   "test_queue",
		ConsumerTag: "test_consumer",
	}
	
	log := &MockLogger{}
	mockService := &MockCheckService{}
	
	consumer, err := NewConsumer(config, log, mockService)
	require.NoError(t, err)
	
	stats := consumer.GetStats()
	assert.Equal(t, "test_queue", stats["queue_name"])
	assert.Equal(t, "test_consumer", stats["consumer_tag"])
	assert.Equal(t, false, stats["closed"])
	assert.Equal(t, "running", stats["status"])
	
	consumer.Close()
	
	// После закрытия
	stats = consumer.GetStats()
	assert.Equal(t, true, stats["closed"])
	assert.Equal(t, "closed", stats["status"])
}

func TestConsumer_Close(t *testing.T) {
	config := ConsumerConfig{
		QueueName:   "test_queue",
		ConsumerTag: "test_consumer",
	}
	
	log := &MockLogger{}
	mockService := &MockCheckService{}
	
	consumer, err := NewConsumer(config, log, mockService)
	require.NoError(t, err)
	
	// Проверка логов перед закрытием
	log.ClearLogs()
	
	err = consumer.Close()
	assert.NoError(t, err)
	
	// Проверка логов после закрытия
	logs := log.GetLogs()
	assert.Contains(t, logs, "INFO: Closing consumer")
	assert.Contains(t, logs, "INFO: Consumer closed")
}

// MockLogger мок для тестов
type MockLogger struct {
	logs []string
}

func (m *MockLogger) Debug(msg string, fields ...logger.Field) {
	m.logs = append(m.logs, "DEBUG: "+msg)
}

func (m *MockLogger) Info(msg string, fields ...logger.Field) {
	m.logs = append(m.logs, "INFO: "+msg)
}

func (m *MockLogger) Warn(msg string, fields ...logger.Field) {
	m.logs = append(m.logs, "WARN: "+msg)
}

func (m *MockLogger) Error(msg string, fields ...logger.Field) {
	m.logs = append(m.logs, "ERROR: "+msg)
}

func (m *MockLogger) With(fields ...logger.Field) logger.Logger {
	return m
}

func (m *MockLogger) Sync() error {
	return nil
}

func (m *MockLogger) GetLogs() []string {
	return m.logs
}

func (m *MockLogger) ClearLogs() {
	m.logs = []string{}
}
