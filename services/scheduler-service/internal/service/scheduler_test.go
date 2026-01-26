package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"UptimePingPlatform/services/scheduler-service/internal/mocks"
)

// MockTaskService - мок для TaskService
type MockTaskService struct {
	mock.Mock
}

func (m *MockTaskService) LoadActiveChecksOnStartup(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockTaskService) ExecuteCronTask(ctx context.Context, checkID string) error {
	args := m.Called(ctx, checkID)
	return args.Error(0)
}

func (m *MockTaskService) GetStats() map[string]interface{} {
	args := m.Called()
	return args.Get(0).(map[string]interface{})
}

func (m *MockTaskService) Start() {
	m.Called()
}

func (m *MockTaskService) Stop() {
	m.Called()
}

// setupTestScheduler создает тестовый планировщик
func setupTestScheduler() (*Scheduler, *MockTaskService, *mocks.MockLogger) {
	mockTaskService := &MockTaskService{}
	mockLogger := &mocks.MockLogger{}
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Maybe()
	mockLogger.On("Debug", mock.AnythingOfType("string"), mock.Anything).Maybe()
	mockLogger.On("Error", mock.AnythingOfType("string"), mock.Anything).Maybe()
	mockLogger.On("Warn", mock.AnythingOfType("string"), mock.Anything).Maybe()

	scheduler := NewScheduler(mockTaskService, mockLogger)

	return scheduler, mockTaskService, mockLogger
}

func TestScheduler_Start(t *testing.T) {
	ctx := context.Background()
	scheduler, mockTaskService, mockLogger := setupTestScheduler()

	// Настройка моков
	mockTaskService.On("LoadActiveChecksOnStartup", ctx).Return(nil)
	mockTaskService.On("Start")
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything)

	// Act
	err := scheduler.Start(ctx)

	// Assert
	assert.NoError(t, err)
	assert.True(t, scheduler.IsRunning())
	mockTaskService.AssertExpectations(t)
}

func TestScheduler_Start_LoadError(t *testing.T) {
	ctx := context.Background()
	scheduler, mockTaskService, mockLogger := setupTestScheduler()

	// Настройка моков
	expectedError := assert.AnError
	mockTaskService.On("LoadActiveChecksOnStartup", ctx).Return(expectedError)
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything)
	mockLogger.On("Error", mock.AnythingOfType("string"), mock.Anything)

	// Act
	err := scheduler.Start(ctx)

	// Assert
	assert.Error(t, err)
	assert.False(t, scheduler.IsRunning())
	mockTaskService.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestScheduler_Stop(t *testing.T) {
	ctx := context.Background()
	scheduler, mockTaskService, mockLogger := setupTestScheduler()

	// Сначала запускаем планировщик
	mockTaskService.On("LoadActiveChecksOnStartup", ctx).Return(nil)
	mockTaskService.On("Start")
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything)
	err := scheduler.Start(ctx)
	assert.NoError(t, err)

	// Настройка моков для остановки
	mockTaskService.On("Stop")
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything)

	// Act
	err = scheduler.Stop(ctx)

	// Assert
	assert.NoError(t, err)
	assert.False(t, scheduler.IsRunning())
	mockTaskService.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestScheduler_AddCheck(t *testing.T) {
	ctx := context.Background()
	scheduler, mockTaskService, mockLogger := setupTestScheduler()

	// Настройка моков для запуска планировщика
	mockTaskService.On("LoadActiveChecksOnStartup", ctx).Return(nil)
	mockTaskService.On("Start")
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Maybe()

	// Запускаем планировщик
	err := scheduler.Start(ctx)
	assert.NoError(t, err)

	checkID := "test-check-123"
	nextRun := time.Now().Add(5 * time.Minute)

	// Act
	err = scheduler.AddCheck(ctx, checkID, nextRun)

	// Assert
	assert.NoError(t, err)
}

func TestScheduler_RemoveCheck(t *testing.T) {
	ctx := context.Background()
	scheduler, _, _ := setupTestScheduler()

	checkID := "test-check-123"

	// Act
	err := scheduler.RemoveCheck(ctx, checkID)

	// Assert
	assert.NoError(t, err)
}

func TestScheduler_UpdateCheck(t *testing.T) {
	ctx := context.Background()
	scheduler, _, _ := setupTestScheduler()

	checkID := "test-check-123"
	nextRun := time.Now().Add(10 * time.Minute)

	// Act
	err := scheduler.UpdateCheck(ctx, checkID, nextRun)

	// Assert
	assert.NoError(t, err)
}

func TestScheduler_GetStats(t *testing.T) {
	scheduler, _, _ := setupTestScheduler()

	// Act
	stats := scheduler.GetStats()

	// Assert
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "is_running")
	assert.Contains(t, stats, "cron_entries")
	assert.IsType(t, false, stats["is_running"])
}

func TestScheduler_formatTimeToCron(t *testing.T) {
	scheduler, _, _ := setupTestScheduler()

	testTime := time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)

	// Act
	cronExpr := scheduler.formatTimeToCron(testTime)

	// Assert
	assert.Equal(t, "45 30 14 15 1 *", cronExpr)
}
