package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"UptimePingPlatform/services/scheduler-service/internal/domain"
	"UptimePingPlatform/services/scheduler-service/internal/mocks"
)

// setupTestTaskService создает тестовый сервис задач
func setupTestTaskService() (*TaskService, *mocks.MockCheckRepository, *mocks.MockTaskRepository, *mocks.MockLockRepository, *mocks.MockSchedulerRepository, *mocks.MockLogger) {
	mockCheckRepo := &mocks.MockCheckRepository{}
	mockTaskRepo := &mocks.MockTaskRepository{}
	mockLockRepo := &mocks.MockLockRepository{}
	mockSchedulerRepo := &mocks.MockSchedulerRepository{}
	mockLogger := &mocks.MockLogger{}
	mockProducer := &mocks.MockProducer{}

	// Настраиваем моки для логирования
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Maybe()
	mockLogger.On("Debug", mock.AnythingOfType("string"), mock.Anything).Maybe()
	mockLogger.On("Error", mock.AnythingOfType("string"), mock.Anything).Maybe()
	mockLogger.On("Warn", mock.AnythingOfType("string"), mock.Anything).Maybe()

	taskService := NewTaskService(mockCheckRepo, mockTaskRepo, mockLockRepo, mockSchedulerRepo, mockProducer, mockLogger)

	return taskService, mockCheckRepo, mockTaskRepo, mockLockRepo, mockSchedulerRepo, mockLogger
}

func TestTaskService_LoadActiveChecksOnStartup_Success(t *testing.T) {
	ctx := context.Background()
	taskService, mockCheckRepo, _, _, _, _ := setupTestTaskService()

	// Тестовые данные
	checks := []*domain.Check{
		{
			ID:     "check-123",
			Name:   "Test Check",
			Target: "https://example.com",
		},
	}

	// Настраиваем моки
	mockCheckRepo.On("GetActiveChecks", ctx).Return(checks, nil)

	// Выполняем операцию
	err := taskService.LoadActiveChecksOnStartup(ctx)

	// Проверяем результат
	assert.NoError(t, err)

	// Проверяем вызовы моков
	mockCheckRepo.AssertExpectations(t)
}

func TestTaskService_LoadActiveChecksOnStartup_Error(t *testing.T) {
	ctx := context.Background()
	taskService, mockCheckRepo, _, _, _, _ := setupTestTaskService()

	// Настраиваем моки
	mockCheckRepo.On("GetActiveChecks", ctx).Return([]*domain.Check{}, assert.AnError)

	// Выполняем операцию
	err := taskService.LoadActiveChecksOnStartup(ctx)

	// Проверяем результат
	assert.Error(t, err)

	// Проверяем вызовы моков
	mockCheckRepo.AssertExpectations(t)
}

func TestTaskService_GetStats(t *testing.T) {
	taskService, _, _, _, _, _ := setupTestTaskService()

	// Выполняем операцию
	stats := taskService.GetStats()

	// Проверяем результат
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "service")
	assert.Contains(t, stats, "worker_id")
}

func TestTaskService_StartStop(t *testing.T) {
	taskService, _, _, _, _, _ := setupTestTaskService()

	// Тестируем Start и Stop
	assert.NotPanics(t, taskService.Start)
	assert.NotPanics(t, taskService.Stop)
}
