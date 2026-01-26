package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"UptimePingPlatform/services/core-service/internal/domain"
	"UptimePingPlatform/services/core-service/internal/service/checker"
	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
)

// MockCheckerFactory мок для CheckerFactory
type MockCheckerFactory struct {
	mockChecker *MockChecker
}

func (m *MockCheckerFactory) CreateChecker(taskType domain.TaskType) (checker.Checker, error) {
	if m.mockChecker == nil {
		return nil, errors.New(errors.ErrInternal, "mock checker not set")
	}
	return m.mockChecker, nil
}

func (m *MockCheckerFactory) GetSupportedTypes() []domain.TaskType {
	return []domain.TaskType{
		domain.TaskTypeHTTP,
		domain.TaskTypeGRPC,
		domain.TaskTypeGraphQL,
		domain.TaskTypeTCP,
	}
}

// MockChecker мок для Checker
type MockChecker struct {
	mockResult *domain.CheckResult
	mockError  error
}

func (m *MockChecker) Execute(task *domain.Task) (*domain.CheckResult, error) {
	if m.mockError != nil {
		return nil, m.mockError
	}
	return m.mockResult, nil
}

func (m *MockChecker) GetType() domain.TaskType {
	return domain.TaskTypeHTTP
}

func (m *MockChecker) ValidateConfig(config map[string]interface{}) error {
	return nil
}

// MockLogger мок для Logger
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

func TestCheckService_NewCheckService(t *testing.T) {
	log := &MockLogger{}
	factory := &MockCheckerFactory{}
	
	service := NewCheckService(log, factory)
	
	assert.NotNil(t, service)
	assert.Equal(t, log, service.logger)
	assert.Equal(t, factory, service.checkerFactory)
}

func TestCheckService_ProcessTask_Success(t *testing.T) {
	log := &MockLogger{}
	mockChecker := &MockChecker{
		mockResult: &domain.CheckResult{
			CheckID:      "check-1",
			ExecutionID:  "exec-1",
			Success:      true,
			DurationMs:   100,
			StatusCode:   200,
			CheckedAt:    time.Now().UTC(),
			Metadata:     make(map[string]string),
		},
	}
	factory := &MockCheckerFactory{mockChecker: mockChecker}
	
	service := NewCheckService(log, factory)
	
	// Создание тестового сообщения
	message := TaskMessage{
		CheckID:     "check-1",
		ExecutionID: "exec-1",
		Target:      "https://example.com",
		Type:        "http",
		Config: map[string]interface{}{
			"method":         "GET",
			"url":            "https://example.com",
			"expected_status": float64(200),
		},
		ScheduledAt: time.Now(),
	}
	
	// Сериализация сообщения
	messageBytes, err := json.Marshal(message)
	require.NoError(t, err)
	
	// Обработка задачи
	err = service.ProcessTask(context.Background(), messageBytes)
	assert.NoError(t, err)
	
	// Проверка логов
	logs := log.GetLogs()
	assert.Contains(t, logs, "INFO: Starting task processing")
	assert.Contains(t, logs, "INFO: Task deserialized successfully")
	assert.Contains(t, logs, "INFO: Check executed successfully")
	assert.Contains(t, logs, "INFO: Task processing completed successfully")
}

func TestCheckService_ProcessTask_InvalidMessage(t *testing.T) {
	log := &MockLogger{}
	factory := &MockCheckerFactory{}
	
	service := NewCheckService(log, factory)
	
	// Некорректное сообщение
	invalidMessage := []byte("invalid json")
	
	// Обработка задачи
	err := service.ProcessTask(context.Background(), invalidMessage)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to deserialize message")
	
	// Проверка логов
	logs := log.GetLogs()
	assert.Contains(t, logs, "INFO: Starting task processing")
	assert.Contains(t, logs, "ERROR: Failed to deserialize message")
}

func TestCheckService_ProcessTask_MissingRequiredFields(t *testing.T) {
	log := &MockLogger{}
	factory := &MockCheckerFactory{}
	
	service := NewCheckService(log, factory)
	
	// Сообщение без обязательных полей
	message := TaskMessage{
		Config: map[string]interface{}{
			"method": "GET",
		},
	}
	
	messageBytes, err := json.Marshal(message)
	require.NoError(t, err)
	
	// Обработка задачи
	err = service.ProcessTask(context.Background(), messageBytes)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "check_id is required")
}

func TestCheckService_ProcessTask_CheckerCreationFailed(t *testing.T) {
	log := &MockLogger{}
	factory := &MockCheckerFactory{} // mockChecker не установлен
	
	service := NewCheckService(log, factory)
	
	message := TaskMessage{
		CheckID:     "check-1",
		ExecutionID: "exec-1",
		Target:      "https://example.com",
		Type:        "unknown_type", // неизвестный тип
		Config:       map[string]interface{}{},
		ScheduledAt:  time.Now(),
	}
	
	messageBytes, err := json.Marshal(message)
	require.NoError(t, err)
	
	// Обработка задачи
	err = service.ProcessTask(context.Background(), messageBytes)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create checker")
}

func TestCheckService_ProcessTask_CheckExecutionFailed(t *testing.T) {
	log := &MockLogger{}
	mockChecker := &MockChecker{
		mockError: errors.New(errors.ErrInternal, "check execution failed"),
	}
	factory := &MockCheckerFactory{mockChecker: mockChecker}
	
	service := NewCheckService(log, factory)
	
	message := TaskMessage{
		CheckID:     "check-1",
		ExecutionID: "exec-1",
		Target:      "https://example.com",
		Type:        "http",
		Config:       map[string]interface{}{},
		ScheduledAt:  time.Now(),
	}
	
	messageBytes, err := json.Marshal(message)
	require.NoError(t, err)
	
	// Обработка задачи
	err = service.ProcessTask(context.Background(), messageBytes)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "check execution failed")
}

func TestCheckService_deserializeMessage_Success(t *testing.T) {
	log := &MockLogger{}
	factory := &MockCheckerFactory{}
	
	service := NewCheckService(log, factory)
	
	message := TaskMessage{
		CheckID:     "check-1",
		ExecutionID: "exec-1",
		Target:      "https://example.com",
		Type:        "http",
		Config:       map[string]interface{}{},
		ScheduledAt:  time.Now(),
	}
	
	messageBytes, err := json.Marshal(message)
	require.NoError(t, err)
	
	result, err := service.deserializeMessage(messageBytes)
	assert.NoError(t, err)
	assert.Equal(t, "check-1", result.CheckID)
	assert.Equal(t, "exec-1", result.ExecutionID)
	assert.Equal(t, "https://example.com", result.Target)
	assert.Equal(t, "http", result.Type)
}

func TestCheckService_deserializeMessage_InvalidJSON(t *testing.T) {
	log := &MockLogger{}
	factory := &MockCheckerFactory{}
	
	service := NewCheckService(log, factory)
	
	invalidMessage := []byte("invalid json")
	
	_, err := service.deserializeMessage(invalidMessage)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal task message")
}

func TestCheckService_deserializeMessage_MissingFields(t *testing.T) {
	log := &MockLogger{}
	factory := &MockCheckerFactory{}
	
	service := NewCheckService(log, factory)
	
	// Пустое сообщение
	emptyMessage := []byte("{}")
	
	_, err := service.deserializeMessage(emptyMessage)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "check_id is required")
}

func TestCheckService_createTask(t *testing.T) {
	log := &MockLogger{}
	factory := &MockCheckerFactory{}
	
	service := NewCheckService(log, factory)
	
	message := TaskMessage{
		CheckID:     "check-1",
		ExecutionID: "exec-1",
		Target:      "https://example.com",
		Type:        "http",
		Config:       map[string]interface{}{"method": "GET"},
		ScheduledAt:  time.Now(),
	}
	
	task := service.createTask(&message)
	
	assert.Equal(t, "check-1", task.CheckID)
	assert.Equal(t, "exec-1", task.ExecutionID)
	assert.Equal(t, "https://example.com", task.Target)
	assert.Equal(t, "http", task.Type)
	assert.Equal(t, map[string]interface{}{"method": "GET"}, task.Config)
}

func TestCheckService_executeCheck(t *testing.T) {
	log := &MockLogger{}
	mockChecker := &MockChecker{
		mockResult: &domain.CheckResult{
			CheckID:      "check-1",
			ExecutionID:  "exec-1",
			Success:      true,
			DurationMs:   100,
			StatusCode:   200,
			CheckedAt:    time.Now().UTC(),
			Metadata:     make(map[string]string),
		},
	}
	factory := &MockCheckerFactory{mockChecker: mockChecker}
	
	service := NewCheckService(log, factory)
	
	task := domain.NewTask(
		"check-1",
		"https://example.com",
		"http",
		"exec-1",
		time.Now(),
		map[string]interface{}{},
	)
	
	result, err := service.executeCheck(context.Background(), mockChecker, task)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "check-1", result.CheckID)
	assert.True(t, result.Success)
	assert.Contains(t, result.Metadata, "processed_at")
	assert.Contains(t, result.Metadata, "service")
}

func TestCheckService_saveResult(t *testing.T) {
	log := &MockLogger{}
	factory := &MockCheckerFactory{}
	
	service := NewCheckService(log, factory)
	
	result := &domain.CheckResult{
		CheckID:      "check-1",
		ExecutionID:  "exec-1",
		Success:      true,
		DurationMs:   100,
		StatusCode:   200,
		CheckedAt:    time.Now().UTC(),
		Metadata:     make(map[string]string),
	}
	
	err := service.saveResult(context.Background(), result)
	assert.NoError(t, err)
}

func TestCheckService_cacheResult(t *testing.T) {
	log := &MockLogger{}
	factory := &MockCheckerFactory{}
	
	service := NewCheckService(log, factory)
	
	result := &domain.CheckResult{
		CheckID:      "check-1",
		ExecutionID:  "exec-1",
		Success:      true,
		DurationMs:   100,
		StatusCode:   200,
		CheckedAt:    time.Now().UTC(),
		Metadata:     make(map[string]string),
	}
	
	err := service.cacheResult(context.Background(), result)
	assert.NoError(t, err)
}

func TestCheckService_sendToIncidentManager(t *testing.T) {
	log := &MockLogger{}
	factory := &MockCheckerFactory{}
	
	service := NewCheckService(log, factory)
	
	result := &domain.CheckResult{
		CheckID:      "check-1",
		ExecutionID:  "exec-1",
		Success:      false,
		DurationMs:   100,
		StatusCode:   500,
		CheckedAt:    time.Now().UTC(),
		Error:        "Connection failed",
		Metadata:     make(map[string]string),
	}
	
	err := service.sendToIncidentManager(context.Background(), result)
	assert.NoError(t, err)
}

func TestCheckService_GetCachedResult(t *testing.T) {
	log := &MockLogger{}
	factory := &MockCheckerFactory{}
	
	service := NewCheckService(log, factory)
	
	result, err := service.GetCachedResult(context.Background(), "check-1")
	assert.NoError(t, err)
	assert.Nil(t, result)
}
