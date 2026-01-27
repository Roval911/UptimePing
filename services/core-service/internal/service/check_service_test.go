package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"UptimePingPlatform/services/core-service/internal/domain"
	"UptimePingPlatform/services/core-service/internal/repository"
	"UptimePingPlatform/services/core-service/internal/service/checker"
	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	pkg_redis "UptimePingPlatform/pkg/redis"
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

// MockCheckResultRepository мок для CheckResultRepository
type MockCheckResultRepository struct{}

func (m *MockCheckResultRepository) Save(ctx context.Context, result *domain.CheckResult) error {
	return nil
}

func (m *MockCheckResultRepository) GetByID(ctx context.Context, id string) (*domain.CheckResult, error) {
	return nil, nil
}

func (m *MockCheckResultRepository) GetByCheckID(ctx context.Context, checkID string, limit int) ([]*domain.CheckResult, error) {
	return nil, nil
}

func (m *MockCheckResultRepository) GetByTimeRange(ctx context.Context, startTime, endTime time.Time, limit int) ([]*domain.CheckResult, error) {
	return nil, nil
}

func (m *MockCheckResultRepository) DeleteOldResults(ctx context.Context, before time.Time) error {
	return nil
}

func (m *MockCheckResultRepository) GetFailedChecks(ctx context.Context, startTime, endTime time.Time, limit int) ([]*domain.CheckResult, error) {
	return nil, nil
}

func (m *MockCheckResultRepository) GetLatestByCheckID(ctx context.Context, checkID string) (*domain.CheckResult, error) {
	return nil, nil
}

func (m *MockCheckResultRepository) GetStats(ctx context.Context, startTime, endTime time.Time) (*repository.ResultStats, error) {
	return &repository.ResultStats{
		TotalChecks:     0,
		SuccessfulChecks: 0,
		FailedChecks:   0,
		AvgResponseTime: 0,
	}, nil
}

// MockIncidentManager мок для IncidentManager
type MockIncidentManager struct{}

func (m *MockIncidentManager) ProcessCheckResult(ctx context.Context, result *domain.CheckResult) error {
	return nil
}

func (m *MockIncidentManager) CreateIncident(ctx context.Context, incident *Incident) (*Incident, error) {
	return incident, nil
}

func (m *MockIncidentManager) UpdateIncident(ctx context.Context, incidentID string, updates *IncidentUpdates) (*Incident, error) {
	return &Incident{ID: incidentID}, nil
}

func (m *MockIncidentManager) ResolveIncident(ctx context.Context, incidentID string) error {
	return nil
}

func (m *MockIncidentManager) GetActiveIncidents(ctx context.Context, tenantID string) ([]*Incident, error) {
	return []*Incident{}, nil
}

func (m *MockIncidentManager) GetIncident(ctx context.Context, incidentID string) (*Incident, error) {
	return &Incident{ID: incidentID}, nil
}

func (m *MockIncidentManager) ListIncidents(ctx context.Context, filters *IncidentFilters) ([]*Incident, error) {
	return []*Incident{}, nil
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
	mockRepo := &MockCheckResultRepository{}
	mockRedis := (*pkg_redis.Client)(nil) // Передаем nil, чтобы избежать использования Redis в тесте
	mockIncidentManager := &MockIncidentManager{}
	
	service := NewCheckService(log, factory, mockRepo, mockRedis, mockIncidentManager)
	
	assert.NotNil(t, service)
	// Проверяем, что сервис создан, без прямого доступа к приватным полям
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
	mockRepo := &MockCheckResultRepository{}
	mockRedis := (*pkg_redis.Client)(nil) // Передаем nil, чтобы избежать использования Redis в тесте
	mockIncidentManager := &MockIncidentManager{}
	
	service := NewCheckService(log, factory, mockRepo, mockRedis, mockIncidentManager)
	
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
	mockRepo := &MockCheckResultRepository{}
	mockRedis := (*pkg_redis.Client)(nil) // Передаем nil, чтобы избежать использования Redis в тесте
	mockIncidentManager := &MockIncidentManager{}
	
	service := NewCheckService(log, factory, mockRepo, mockRedis, mockIncidentManager)
	
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
	mockRepo := &MockCheckResultRepository{}
	mockRedis := (*pkg_redis.Client)(nil) // Передаем nil, чтобы избежать использования Redis в тесте
	mockIncidentManager := &MockIncidentManager{}
	
	service := NewCheckService(log, factory, mockRepo, mockRedis, mockIncidentManager)
	
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
	mockRepo := &MockCheckResultRepository{}
	mockRedis := (*pkg_redis.Client)(nil) // Передаем nil, чтобы избежать использования Redis в тесте
	mockIncidentManager := &MockIncidentManager{}
	
	service := NewCheckService(log, factory, mockRepo, mockRedis, mockIncidentManager)
	
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
	mockRepo := &MockCheckResultRepository{}
	mockRedis := (*pkg_redis.Client)(nil) // Передаем nil, чтобы избежать использования Redis в тесте
	mockIncidentManager := &MockIncidentManager{}
	
	service := NewCheckService(log, factory, mockRepo, mockRedis, mockIncidentManager)
	
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

func TestCheckService_GetCachedResult(t *testing.T) {
	log := &MockLogger{}
	factory := &MockCheckerFactory{}
	mockRepo := &MockCheckResultRepository{}
	mockRedis := (*pkg_redis.Client)(nil) // Передаем nil, чтобы избежать использования Redis в тесте
	mockIncidentManager := &MockIncidentManager{}
	
	service := NewCheckService(log, factory, mockRepo, mockRedis, mockIncidentManager)
	
	result, err := service.GetCachedResult(context.Background(), "check-1")
	assert.NoError(t, err)
	assert.Nil(t, result)
}
